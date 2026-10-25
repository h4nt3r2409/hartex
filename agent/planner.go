package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/intercept"
	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/llm"
	"github.com/Autumn-27/norma/permission"
	actool "github.com/Autumn-27/norma/tool"
	"github.com/Autumn-27/norma/transcript"
)

// Planner is the event-driven LLM planner (docs §4.3): each time the asset or
// exploration graph changes (debounced), it reads the exploration route, queries
// assets, judges whether the task goal is met, and emits 0..N exploration intents
// into the frontier. It is the sole intent generator.
type Planner struct {
	findingRecorder   FindingRecorder
	prov              llm.Provider
	model             string
	tx                *transcript.Store                      // raw LLM conversation persistence (nil = off)
	window            int                                    // context window in tokens (for compaction)
	windowFn          func() int                             // optional dynamic task-chain minimum
	maxTurns          int                                    // max agent turns per run (0 = unlimited)
	killWork          func(intentID int64) error             // engine callback to terminate a running work (nil = off)
	steerWork         func(intentID int64, msg string) error // engine callback to steer a running work mid-run (nil = off)
	proxyAddr         string                                 // recording proxy for WebFetch (empty = direct)
	proxyCACert       string                                 // recording proxy's CA cert path (HTTPS verify)
	webSearch         WebSearchOpts                          // web_search tool backend selection (off by default)
	workDir           string                                 // shared work dir (surfaced in prompt as artifact-output target)
	injectConstraints func() bool                            // resolver: inject task operation constraints into system prompt? (nil = yes)
	nonStreamingFn    func() bool                            // resolver: use non-streaming (Complete) path? (nil = streaming)
	noaEnabledFn      func() bool                            // resolver: use experimental noa compaction? (nil = off)
	maxTokensFn       func() int                             // resolver: per-reply output cap (nil/0 = send no cap)
	compactor         *Compactor                             // cold-node compaction (§7); nil = disabled

	// todos keeps ONE plan-scratchpad per task (keyed by exploration id) so the
	// planner's multi-step plan survives across wake-ups — each Plan() is a fresh
	// session, but the shared store lets it record a serial exploit chain once and
	// dispatch it step-by-step over rounds instead of front-loading it in parallel.
	todoMu sync.Mutex
	todos  map[int64]*actool.TodoStore
}

func NewPlanner(prov llm.Provider, model, workDir string, tx *transcript.Store, window, maxTurns int) *Planner {
	return &Planner{prov: prov, model: model, workDir: workDir, tx: tx, window: window, maxTurns: maxTurns, todos: map[int64]*actool.TodoStore{}}
}

func (p *Planner) SetCompactionWindowResolver(fn func() int) { p.windowFn = fn }

// SetCompactor wires the cold-node compactor (cold-digest §7). Called each
// planner wake-up to advance the round counter, maintain cold stamps, and
// (off the hot path) fold cold nodes into digests. nil = feature disabled.
func (p *Planner) SetCompactor(c *Compactor) { p.compactor = c }

// SetNonStreaming wires a resolver deciding whether runs use the non-streaming
// model path (true = non-streaming). nil/unset = streaming (default).
func (p *Planner) SetNonStreaming(fn func() bool) { p.nonStreamingFn = fn }

func (p *Planner) nonStreaming() bool { return p.nonStreamingFn != nil && p.nonStreamingFn() }

// SetNoaEnabled wires a resolver deciding whether runs use the experimental noa
// context-compression mechanism. nil/unset = off (built-in compaction). Read per
// run so the settings toggle takes effect without rebuilding the agent.
func (p *Planner) SetNoaEnabled(fn func() bool) { p.noaEnabledFn = fn }

// SetMaxTokens wires a resolver for the per-reply output cap. nil/unset or 0 =
// send no cap and let the endpoint decide. Read per run, like nonStreaming.
func (p *Planner) SetMaxTokens(fn func() int) { p.maxTokensFn = fn }

func (p *Planner) maxTokens() int {
	if p.maxTokensFn == nil {
		return 0
	}
	return p.maxTokensFn()
}

func (p *Planner) compactionWindow() int {
	if p.windowFn != nil {
		return p.windowFn()
	}
	return p.window
}

// SetProxy points the planner's WebFetch at the recording proxy plus the CA cert
// it trusts to verify HTTPS through it (empty addr = direct).
func (p *Planner) SetProxy(addr, caCert string) { p.proxyAddr, p.proxyCACert = addr, caCert }

// SetWebSearch selects the web_search backend for the planner (off by default).
func (p *Planner) SetWebSearch(o WebSearchOpts) { p.webSearch = o }

// SetConstraintInject wires a resolver deciding whether this task's operation
// constraints get injected into the planner system prompt. Read per round so the
// settings toggle takes effect without rebuilding the agent. nil = inject (default).
func (p *Planner) SetConstraintInject(fn func() bool) { p.injectConstraints = fn }

// wantConstraints reports whether constraint injection is enabled (default yes).
func (p *Planner) wantConstraints() bool { return p.injectConstraints == nil || p.injectConstraints() }

// todoFor returns the task's persistent planning todo store, creating it on first
// use. Shared across all of this task's planner wake-ups.
func (p *Planner) todoFor(expID int64) *actool.TodoStore {
	p.todoMu.Lock()
	defer p.todoMu.Unlock()
	s := p.todos[expID]
	if s == nil {
		s = actool.NewTodoStore()
		p.todos[expID] = s
	}
	return s
}

// SetKillWork wires the engine's per-work terminate callback so the planner's
// kill_work tool can stop a single running worker.
func (p *Planner) SetKillWork(fn func(intentID int64) error) { p.killWork = fn }

// SetSteerWork wires the engine's per-work steering callback so the planner's
// steer_work tool can inject a mid-run course-correction into a running worker.
func (p *Planner) SetSteerWork(fn func(intentID int64, msg string) error) { p.steerWork = fn }

// renderPlannerTodos formats the persistent planning todo for injection into the
// wake-up prompt (empty when there are no todos yet — first wake-up).
func renderPlannerTodos(items []actool.Todo) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Your planning todo list (persisted across wake-ups, written by you last round)]:\n")
	for _, it := range items {
		mark := map[actool.TodoStatus]string{actool.TodoPending: "☐", actool.TodoInProgress: "▶", actool.TodoCompleted: "✔"}[it.Status]
		if mark == "" {
			mark = "☐"
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", mark, it.Content))
	}
	b.WriteString("Proceed accordingly: only dispatch intents for the next step whose [prerequisite steps are complete / the facts it depends on already exist]; use TodoWrite to update the list (mark steps already satisfied by facts as completed). Do not re-dispatch steps already pending/in_progress in the list.")
	return b.String()
}

// TriggerEvent describes what concretely caused this planning round to fire, so
// the planner looks first at the actual change instead of re-scanning the whole
// overview. Kind:
//
//	"done"    — a worker finished intent IntentID (its output conclusion is fetched).
//	"finding" — a worker reported a finding on intent IntentID (Detail = summary).
//	"goal"    — the human (via the main agent's set_goals) added one OR MORE goals in a
//	            single call (Goals = the newly added goal texts, 1+ items; set_goals supports batches).
//	"goal_deleted" — the human deleted a goal from the overview's goal management (Detail = deleted goal text).
//	"goal_edited"  — the human edited a goal from the overview's goal management (OldGoal→NewGoal texts).
//	"cancelled" — the human deleted intent IntentID (Detail = deletion reason). The intent is
//	            stopped (not deleted) and the reason is attached to it as a fact.
type TriggerEvent struct {
	Kind     string
	IntentID int64
	Detail   string
	Summary  string   // Kind=="cancelled" only: intent summary captured before deletion (after a real deletion the node no longer exists, so it cannot be looked up again)
	Goals    []string // Kind=="goal" only: goal texts newly added by this set_goals call (one or more)
	OldGoal  string   // Kind=="goal_edited" only: goal text before the edit
	NewGoal  string   // Kind=="goal_edited" only: goal text after the edit
	Hints    []string // Kind=="hint" only: hint texts newly added by this add_hint call (one or more)
}

// renderTriggers spells out the change(s) that fired this round: for a finished
// worker — which intent + its output conclusion; for a finding — which intent +
// what was found. Empty for time/heartbeat wakes. Reads the store (best-effort;
// a blank field never blocks the round).
func renderTriggers(ts *db.ExplorationStore, evs []TriggerEvent) string {
	if len(evs) == 0 || ts == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Actual changes that triggered this round (look here first, then decide whether to add directions)]:")
	for _, ev := range evs {
		switch ev.Kind {
		case "goal":
			if len(ev.Goals) == 1 {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added one goal: %s — a new goal to achieve; add exploration directions accordingly (if no corresponding intent exists yet).", ev.Goals[0]))
			} else {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added %d goals: %s — all are new goals to achieve; add exploration directions for each goal that has no corresponding intent yet.", len(ev.Goals), strings.Join(ev.Goals, "; ")))
			}
		case "hint":
			if len(ev.Hints) == 1 {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added one strategic hint: %s — it has been attached to the exploration graph; adjust/add exploration directions accordingly (if no corresponding intent exists yet).", ev.Hints[0]))
			} else {
				b.WriteString(fmt.Sprintf("\n- The human (main agent) added %d strategic hints: %s — all have been attached to the exploration graph; adjust/add exploration directions based on each.", len(ev.Hints), strings.Join(ev.Hints, "; ")))
			}
		case "goal_deleted":
			b.WriteString(fmt.Sprintf("\n- The human deleted the goal: %s — the goal has been removed; re-evaluate the remaining goals/directions accordingly (no need to dispatch intents for it anymore).", ev.Detail))
		case "goal_edited":
			b.WriteString(fmt.Sprintf("\n- The human edited the goal from \"%s\" to \"%s\" — adjust exploration directions to the new goal (stop dispatching the old direction if it no longer applies).", ev.OldGoal, ev.NewGoal))
		case "finding":
			b.WriteString(fmt.Sprintf("\n- The worker for intent #%d (%s) reported a finding: %s", ev.IntentID, intentSummary(ts, ev.IntentID), ev.Detail))
		case "cancelled":
			// Prefer the Summary captured at deletion time for the intent content (after a real deletion the node no longer exists, so intentSummary cannot find it).
			sm := ev.Summary
			if sm == "" {
				sm = intentSummary(ts, ev.IntentID)
			}
			b.WriteString(fmt.Sprintf("\n- Intent #%d was deleted by the user; intent content: %s; deletion reason: %s. The intent has been deleted (it will not run); re-plan accordingly.", ev.IntentID, sm, ev.Detail))
		default: // "done"
			b.WriteString(fmt.Sprintf("\n- The worker for intent #%d (%s) finished; output conclusion: %s", ev.IntentID, intentSummary(ts, ev.IntentID), workerOutput(ts, ev.IntentID)))
			if fids := factIDsYielded(ts, ev.IntentID); fids != "" {
				b.WriteString(fmt.Sprintf("; fact ids newly produced by this intent: %s ", fids))
			}
		}
	}
	b.WriteString("\n(For full details, query node_detail / get_worker_output / list_findings again.)")
	return b.String()
}

// factIDsYielded lists the fact ids an intent produced this run as "#12, #15", so the
// planner can jump straight to the round's incremental facts. Empty (best-effort) when
// the intent yielded no facts or the lookup fails.
func factIDsYielded(ts *db.ExplorationStore, id int64) string {
	ids, err := ts.FactsYielded(id)
	if err != nil || len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, fid := range ids {
		parts[i] = fmt.Sprintf("#%d", fid)
	}
	return strings.Join(parts, ", ")
}

// intentSummary reads an intent node's one-line summary (best-effort, "?" on miss).
func intentSummary(ts *db.ExplorationStore, id int64) string {
	n, err := ts.GetNode(id)
	if err != nil || n == nil {
		return "?"
	}
	var p map[string]any
	if json.Unmarshal(n.Payload, &p) == nil {
		if s, ok := p["summary"].(string); ok && s != "" {
			return s
		}
	}
	return "?"
}

// workerOutput returns the finished worker's conclusion for an intent — the last
// 'result' (else 'text') activity's full detail, truncated. Same source get_worker_output uses.
func workerOutput(ts *db.ExplorationStore, id int64) string {
	acts, _, err := ts.ActivityList(&id, 0, 1000)
	if err != nil {
		return "(failed to fetch output)"
	}
	var pick *db.Activity
	for i := range acts {
		if acts[i].Kind == "result" {
			pick = &acts[i]
		} else if acts[i].Kind == "text" && pick == nil {
			pick = &acts[i]
		}
	}
	if pick == nil {
		return "(this work has no output records yet)"
	}
	out, _ := ts.ActivityDetail(pick.ID)
	if out == "" {
		out = pick.Summary
	}
	return truncOutput(out, 800)
}

// truncOutput caps a worker-output blob so the trigger context doesn't bloat the
// system prompt every round; full text is one get_worker_output call away.
func truncOutput(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + " …(truncated; see get_worker_output for the full text)"
}

// renderGraphOverview folds the pre-computed graph_overview snapshot into the
// wake-up prompt so the planner starts each round with the full situation in
// hand — saving the round-trip it would otherwise spend calling the tool. It is
// the exact same JSON graph_overview would return; deeper detail is still one
// tool call away (node_detail / list_facts / …).
func renderGraphOverview(data map[string]any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return "" // fall back to the model calling graph_overview itself
	}
	return "\n\n[Current situation (graph_overview pre-fetch, identical to what calling that tool returns; call node_detail/list_facts etc. as needed for details)]:\n" + string(b)
}

// plannerDefaultTmpl is the built-in EDITABLE body (section [A]) of the planner prompt,
// seeded into agent_prompts. Goal is a {{.Goal}} template var; the intermediate-artifact
// output-spec tail is code-owned (artifactSpec) and appended by plannerSystem after rendering.
const plannerDefaultTmpl = `You are the "planner" of a cybersecurity platform's authorized penetration-testing system, and you are woken frequently (every time the graph changes). Responsibilities: read the situation → judge the goal → **only when a genuinely uncovered new direction exists** add exploration intents. You are the planner, not the worker: in this round every output may only be [generate/articulate intents] or [judge the goal]; never do the actual work in the plan.

Task goal: {{.Goal}}

**How many intents to produce this round (think this through first)**:
- **Hard floor (highest priority)**: whenever [the goal is not met] AND [there are currently no open or running intents] (frontier_open=0 and running_intents is empty), this round **must** produce at least one intent that advances the goal — with no running work to wait for and no queued direction, producing 0 intents = the task stalls; even if all known directions are only in recent_done, you must open a new one or continue one per the done/exhausted/blocked judgment below.
- Beyond the hard floor, **producing 0 intents is a normal outcome, but it must have a legitimate reason** (not the "dispatch fewer is safer" default): ① **already covered** — every direction you thought of is already handled by an intent still in open/running (reproducing an existing intent with different wording is a serious error); ② **waiting on a dependency** — the next step depends on the output of currently running work that has not come out yet (forcing a dispatch here would leave the downstream without its prerequisite and spinning, so wait for the next wake-up after the graph updates before dispatching).
- Conversely: if a genuinely [uncovered and not dependent on running work] new direction exists, or the goal is not met and untested surface remains in scope, you should dispatch — do not treat 0 intents as the lazy default.

**Decision process for every wake-up**:

1. **The full situation is attached below this prompt** (it is exactly the graph_overview return; no need to call it again): task (original title + goal/root node), asset counts, goals+status, open/running/recent_done intents, sites_without_endpoints (sites without endpoints, hinting at directions possibly still to explore), facts (number of exploration facts, a separate category from vulnerabilities), recent_facts ({id,summary,confidence?}).
   - **Scope**: exploration nodes (goals/intents/facts/findings) contain only this task; **the asset graph is globally shared** (one copy across tasks; asset counts are global within scope and not unique to this task) — ignore assets unrelated to this task when they appear.
   - **Lineage**: each intent carries parents (upstream: which facts/intents it was derived from) and yields (downstream: which facts/findings it produced), and each recent_facts entry carries from_intent; use this to understand "which facts came from which direction, and whether a new direction can be synthesized".
   - **Negative/inconclusive observations** (e.g. "port closed / not injectable" in recent_facts) are worker observations, not verdicts: before accepting one, call node_detail(id) to inspect the evidence — only when the evidence is solid, confidence=observed, and the means have been exhausted should the direction be considered temporarily closed; if the evidence is missing, it merely "looks like / was probed only once", or confidence=inferred, treat it as [not yet determined], and if it is in scope and uncovered by other intents, dispatch one revalidation intent by default to confirm or refute it (**the same negative direction is revalidated at most once**; if it remains negative after revalidation and the evidence is reasonable, respect the conclusion and do not dispatch again).
   - **Call these on demand only when deeper detail is needed**: list_facts (paginated, newest first, 20 by default, supports q filtering and before paging, returns total/has_more), list_findings (all vulnerabilities), node_detail(id) (full evidence/details; lists/recent_facts only give summaries), list_assets (pull: q search, type/company_id/task_id filtering, pagination, or direct fetch by id/ids), asset_neighbors. Assets are globally shared; do not pull everything by default.

2. **Judge the goal (core responsibility)**: the goals field already contains goals and status; for an unmet goal that has already been proven by some finding/fact, call prove_goal(goal_id, evidence_id, reason) to mark it met. **When the one you mark is exactly the last unfinished goal, the system automatically judges the entire task complete** — completion is driven only by proving goals one by one; there is no other "one-click complete" mechanism.
   - ⚠️ **Quantitative acceptance check (stamping early is strictly forbidden)**: when the goal contains quantifiable conditions (coverage reaches X%, capture N flags, obtain a certain privilege), before prove_goal you **must** check the measured values in the graph_overview above (coverage.pct, findings_total counts, etc.): if the target is not reached, prove_goal is **forbidden** — dispatch intents to close the gap instead; do not mark met early on the grounds of "mostly achieved / the core is captured". Example: coverage of 100% is required but the measured coverage.pct=40% → not met, keep dispatching supplementary-test intents.

3. **(Optional, start-of-task only, extremely lightweight) probing for understanding**: only when the graph has almost no facts yet (recent_facts essentially empty, task just started) and the situation alone is not enough to make the initial intent concrete, use Bash etc. to perform a very small amount of read-only probing against the target (e.g. 1–2 curl calls to look at the homepage/fingerprint). **The only legitimate output is a more precise intent description** — never vulnerability discovery/verification/exploitation, and never the results of enumerating endpoints/directories/parameters (those are the worker's job; write them as intents and dispatch them). Three hard boundaries:
   - If the graph already has facts produced by workers (facts>0 / recent_facts non-empty) → **forbidden** to probe on your own; base every judgment on existing facts, and this round's output may only be "dispatch new intents" or "end"; to dig deeper into a lead → dispatch an intent and let the worker investigate, do not curl yourself.
   - Even at the start, probe at most ≤3 times and stop; this is only to make the initial intent clear. As soon as you find yourself "investigating deeply" rather than "quickly determining direction" (enumerating endpoints/directories one by one, trying ids one by one, decoding chains, repeatedly probing the same interface, any injection/privilege-escalation/vulnerability testing and verification — all heavy work for workers), stop immediately and write it as an intent.
   - Anything judgable from existing facts/situation does not need probing at all.

4. **Decide which new directions to add**: **"restraint" here means only [do not duplicate existing intents], not "dispatch as few as possible"** — when the goal is not met, the default question is "to close in on the goal, what deeper, harder, still-uncovered approaches remain", not "can we wrap up". Intents are [open exploration directions] (not fixed types/menus); combine known facts, assets, and the goal to judge directions yourself, comparing each against open + running + recent_done:
   - Already covered by open/running → do not generate (it is being handled).
   - Appeared in recent_done → **first look at that intent's state (each entry carries it) to tell how it stopped, then decide**:
     · **done (ran to completion normally)**: covered → do not re-dispatch as-is; whether it is a dead end depends on the fact conclusions it yields, not on state; re-dispatch only when a [materially new mechanism] appears (new fact/asset/parameter/clearly different approach), and state in the summary how it differs from last time; rewording or "trying once more might work" does not count — retrying is forbidden.
     · **exhausted (budget exhausted, cut off midway through exploration, only partial results written back) / blocked (model or network failure, essentially no exploration done)**: both ended badly midway with incomplete information — first use get_worker_trace / get_worker_output to see how far it actually got and where it got stuck, then choose from the following: cut off by budget close to a breakthrough → dispatch "continue from last progress"; pure external failure with no run (blocked often is) → directly re-dispatch the same direction; stuck at the same place each time → change approach/direction. The basis is always the real progress in the trace, never the state itself.
   - A brand-new direction with no intent covering it at all → generate.
   - All known directions are covered by intents still in open/running → generate nothing and end directly (there is running/queued work; wait for it to advance); but if only recent_done covers them, there is no open/running, and the goal is not met → per the hard floor at the top you must open a new one or continue one.
   - **Depth takes priority over coverage**: coverage is a lower bound/acceptance item, not the exploration goal itself; after finding a high-value entry point (possibly leading to RCE/privilege escalation/data exfiltration), prioritize dispatching intents to [drive that path deep through], rather than broadening to level out coverage and shallow-testing each asset one by one.
   - **Keep routes diverse; do not converge too early**: when the goal is not met, if existing intents all crowd onto the same route/entry point while an [essentially different] uncovered direction exists (another entry surface/another asset class/another exploitation chain), prioritize adding that divergent direction rather than adding synonymous intents on the same line (judge substantive difference, not wording); if that divergent direction is already covered by existing intents, still do not generate. The ideal is 2–3 routes with different mechanisms coexisting (e.g. "attack via the upload chain" and "attack via auth bypass"); only after one produces evidence of [progress toward the goal] should resources be concentrated there. **But diversity always defers to the [operation constraints] at the top**: entry surfaces/ports/hosts/operations excluded by constraints must never be turned into intents, even if essentially different.

   **Serial exploitation chains: dispatch step by step, do not split into parallel.** For strongly dependent serial chains (①→②→③, where each later step depends on the previous step's actual output): do not dispatch them all at once in parallel (downstream steps cannot get prerequisites that do not exist yet and will only repeat/spin); use TodoWrite to record the whole chain as todos (one per step), dispatch only the step whose "prerequisites are satisfied" this round (usually the first step), and after it produces a fact, on the next wake-up (the prompt will include the todo list) dispatch the next step and mark the satisfied ones completed. Do not split "the same thing" into two entries ("confirm the trigger point" and "trigger the trigger point" are the same step); use multiple parallel intents only for [parallel, mutually independent] dimensions (e.g. enumerating several unrelated endpoints).

5. **Submit**: use [one] add_intent call to batch-submit the selected new directions (intents array, at most 4 of the highest value; do not call once per item):
   - **summary**: one natural-language sentence describing the direction (full address of the test target + what to do + why), with no fixed classification; deduplication relies mainly on comparing it against existing intents.
   - **asset_ids**: target asset ids that this direction will test/attack (pass whenever possible; 0/1/multiple, from list_assets) — whenever the direction revolves around concrete assets (site/endpoint/parameter/host) you must pass them, for coverage deduplication and linking into the asset graph; pass all of them when spanning multiple assets; leave empty only for pure global reconnaissance with no concrete asset.
   - **parent_ids**: which upstream nodes this direction was synthesized from (optional, 0/1/multiple) — when several facts combine to produce one intent, pass them all; when derived from an upstream intent/finding, pass its id too; leave empty for a brand-new top-level direction.

Do not duplicate, do not force it; but when the goal is not met and deeper uncovered approaches exist, dispatch when you should. Be concise, focused, efficient.`

func plannerSystem(goal, dataDir, workDir string) string {
	body := renderSystem("planner", plannerDefaultTmpl, PlannerVars{Goal: goal, DataDir: dataDir, Now: nowStr()})
	return body + artifactSpec(workDir) + outputDirective()
}

// Plan runs one planning round. emit, if non-nil, receives the planner's execution
// steps (so users can see how it reads the situation and judges goals — the
// planner is the intent generator and was previously a black box). Returns whether
// the planner judged the goal met.
// triggers carries the concrete change(s) that fired this round — worker(s) done
// and/or finding(s) reported (may be several — the engine debounces a burst; empty
// for time/heartbeat wakes). They are spelled out at the top of the prompt so the
// planner looks first at the actual change (which intent, its output/finding).
func (p *Planner) Plan(ctx context.Context, taskID int64, as *db.AssetStore, ts *db.ExplorationStore, goal string, triggers []TriggerEvent, emit func(db.Activity)) (met bool, reason string, err error) {
	// cold-digest §2.3/§7: advance this task's planner-round counter, maintain the
	// cold_since_round stamps, and (if a threshold is hit) kick off background
	// compaction. Synchronous part is cheap (a few queries); the LLM compaction
	// runs in a detached goroutine so it never adds latency to this round.
	p.compactor.OnPlannerRound(ctx, ts)
	tsx := NewToolSet(ts, "planner")
	tsx.SetFindingRecorder(p.findingRecorder)
	if as != nil {
		tsx.SetAssetStore(as, as.Companies())
	}
	tsx.SetTaskID(taskID)
	tsx.SetCoverageEnabled(as == nil || as.CoverageEnabled(taskID))
	tsx.killWork = p.killWork   // enable kill_work tool (nil = unavailable)
	tsx.steerWork = p.steerWork // enable steer_work tool (nil = unavailable)
	if origin, _ := ts.OriginFactID(); origin > 0 {
		tsx.SetOwnerNode(origin) // planner-side anchors default to the task root (origin fact)
	}
	// Domain tools + basic default toolset (Read/Write/Edit/MultiEdit/LS/Glob/Grep/Bash)
	// When the asset coverage feature is off, drop add_task_scope/list_untested_assets (not included in the prompt).
	base := append(tsx.DropCoverageTools(tsx.PlannerTools()), actool.DefaultTools()...)
	ctx = WithRunInfo(ctx, RunInfo{TaskID: taskID, ExplorationID: explorationID(ts)})
	tools, def, cleanup := AugmentTools(ctx, "planner", base)
	defer cleanup()
	// Key situation (just-finished intents + pre-fetched full graph) is moved into [this round's user input] (see input below); system
	// keeps only the static planning body. Moving it out keeps the system prompt stable each round, which is better for caching; the cost is that if a single round
	// grows long, the situation may be compressed by compaction (planner rounds are usually short, so the risk is low). situational is concatenated into input below.
	situational := renderTriggers(ts, triggers) + renderGraphOverview(tsx.graphOverviewData())
	// Task-level deadline / final-round mode (injected via ctx, see taskclock.go). The final round appends the task-timeout
	// planner wrap-up wording as [this round's operation instruction] to this round's user input (along with situational), so it only does the final
	// goal judgment and produces no new intents.
	tc := taskClockFrom(ctx)
	if tc.Final {
		situational += "\n\n[Task final wrap-up (special instruction for this round, overrides the normal planning flow above)]:" + resolveTaskTimeoutWrapup("planner")
	}
	// This task's work directory <workDir>/tasks/<taskID>; create it first.
	taskDir := ensureRunDir(p.workDir, taskID, 0)
	ctx = intercept.WithReviewContext(ctx, taskDir, intercept.ReviewBackground{})
	sysBody := plannerSystem(goal, p.workDir, taskDir)
	if p.wantConstraints() {
		sysBody += constraintBlock(ts) // operation constraints (if any) injected into the system prompt to frame the exploration boundary
	}
	system, boundary := deferredSystem(sysBody, def)
	// The planner has no wall-clock budget of its own; when there is a deadline, clamp MaxDuration to the remaining time so a running planning round enters wrap-up
	// when the task hits its deadline (timeout → task-timeout wording; step count → per-run wording).
	maxDur, clamped := clampMaxDuration(tc.DeadlineUnix, 0)
	settle := wrapupSettlement("planner", nil)
	if tc.DeadlineUnix > 0 {
		settle = wrapupSettlementForTask("planner", nil, clamped)
	}
	opts := agentcore.Options{
		Provider:        p.prov,
		SystemPrompt:    system,
		DynamicBoundary: boundary,
		Tools:           tools,
		DeferredTools:   def.Deferred,
		UnlockSet:       def.Unlock,
		PermissionMode:  permission.ModeBypass,
		EnableWebFetch:  true, // route through the recording proxy for audit; load the proxy CA to verify HTTPS certs re-signed by MITM
		WebFetchProxy:   p.proxyAddr,
		WebFetchCACert:  p.proxyCACert,
		// Web search (optional). ddgs needs no key; brave-free needs BraveKey; tavily needs TavilyKey.
		// WebSearchProxy is a separate egress proxy (http/https/socks5), unrelated to the traffic-recording MITM proxy; empty = direct connection.
		EnableWebSearch:       p.webSearch.Enabled,
		WebSearchBackend:      p.webSearch.Backend,
		BraveSearchAPIKey:     p.webSearch.BraveKey,
		TavilySearchAPIKey:    p.webSearch.TavilyKey,
		DeepSeekSearchBaseURL: p.webSearch.DeepSeekBaseURL,
		DeepSeekSearchAPIKey:  p.webSearch.DeepSeekAPIKey,
		DeepSeekSearchModel:   p.webSearch.DeepSeekModel,
		WebSearchProxy:        p.webSearch.Proxy,
		BashEnv:               proxyEnv(p.proxyAddr, p.proxyCACert), // Bash subcommands go through the proxy and trust the CA by default
		WorkingDir:            taskDir,                              // this task's work directory <workDir>/tasks/<taskID>
		ToolOutputDir:         cmdOutDir(taskDir),
		MaxTurns:              p.maxTurns, // 0 = unlimited (configurable in agent management)
		MaxDuration:           maxDur,     // 0=unlimited; with a deadline=remaining time until deadline
		Compaction:            compactionConfig(p.compactionWindow()),
		// Planning todos shared across wake-ups: preserve serial chains across rounds (the session is new; the store is not).
		Todos: p.todoFor(ts.ID()),
		// Hitting [this round's] step budget → the SDK runs wrap-up: land the conclusions already thought through this round (the add_intent calls that should be dispatched,
		// the prove_goal calls that can be proven, the serial chain recorded via TodoWrite), rather than stopping planning — the planner will still be woken repeatedly afterward.
		// When clamped (pinned by the task deadline), switch to PromptByReason (see wrapupSettlementForTask).
		Settlement:   settle,
		NonStreaming: p.nonStreaming(), // use Provider.Complete when this profile selects non-streaming
		MaxTokens:    p.maxTokens(),    // 0 = send no cap, let the server default decide
	}
	if p.tx != nil { // persist raw LLM conversation; one accumulating file per task's planner
		opts.Transcript = p.tx
		opts.SessionID = fmt.Sprintf("exp%d-planner", ts.ID())
	}
	// Experimental feature: when enabled, noa takes over context compaction (archives are centralized under <workDir>/noa/<SessionID>, persistent).
	noaSession := fmt.Sprintf("exp%d-planner", ts.ID())
	enableNoa(&opts, p.noaEnabledFn, p.workDir, noaSession, noaWarn(noaSession))
	// The situation (just-finished intents + full graph) is now concatenated into this round's user input (see input below). The user input also contains
	// instructions + cross-wake-up todos (todos are the model's own planning scratchpad and regenerable, so putting them in user is fine).
	// The opening line has two variants depending on [whether this round has concrete changes]: changes → point to the [actual changes] block below; no changes
	// (heartbeat periodic check / hint / resume, etc.) → do not falsely claim "the graph changed"; instead prompt it to also review running intents.
	lead := "There were just concrete changes (see [Actual changes that triggered this round] below); plan the next step accordingly:"
	if len(triggers) == 0 {
		lead = "This round is a wake-up for **periodic check (heartbeat fired) / no concrete change signal** — the graph does not necessarily have new changes. While you are at it, review running intents: use steer_work to correct ones with no progress for a long time or that have gone off track, and kill_work to stop the bleeding on ones whose direction is entirely wrong; then judge the goal and decide whether to add directions:"
		// On heartbeat/no-change wake-ups, if the whole graph has no open or running intents → exploration has stalled (no worker running,
		// and no queued direction). Explicitly tell the planner and force it to add new directions this round; do not let it just review running intents and spin for a round.
		if active, err := ts.HasActiveIntent(); err == nil && !active {
			lead = "This round is a **periodic check (heartbeat fired)** wake-up, and currently **there are no open or running intents at all** — no worker is running and no direction is queued, so exploration has stalled. You **must** produce one or more new intents this round that advance the goal and **do not duplicate** existing intents in the graph (producing 0 intents is not allowed); first judge from the situation below whether the goal is already met, and if not, add directions immediately:"
		}
	}
	input := lead + situational + "\n\nBased on the situation above, judge the goal. When the goal is [truly met] (the target result has been obtained / the target vulnerability has been confirmed), mark it with prove_goal one by one. **Hard floor: whenever the goal is not yet met and there are currently no open or running intents (frontier_open=0 and running_intents is empty), this round must produce at least one intent that advances the goal — there is no running work to wait for and no queued direction, so producing 0 intents = the task stalls. Only when open/running intents are already advancing, or the goal is met, may this round produce no new intents.**" +
		renderPlannerTodos(opts.Todos.List())
	// MaxDuration now interrupts running tools when the wall clock expires and enters wrap-up in place (on the live ctx); a single stuck round no longer
	// bypasses wrap-up, so no external hard ctx fallback is needed. ctx only carries pause / kill / shutdown.
	_, _, err = captureRun(ctx, opts, input,
		func(r db.Activity) {
			if emit != nil {
				r.Worker = "planner" // planner activity has no intent_id (it generates them)
				emit(r)
			}
		})
	return tsx.GoalMet, tsx.Reason, err
}
