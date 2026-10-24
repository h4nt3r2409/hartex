package agent

import (
	"strings"

	"github.com/Autumn-27/norma/harness"
)

// Wrap-up (settlement) prompt: when an agent is terminated because [its turn budget is
// exhausted (MaxTurns)] or [it timed out (run_seconds/MaxDuration)], the SDK's settlement
// phase injects this prompt so the agent first persists what it identified but has not yet
// written back, then outputs a one-sentence summary, avoiding an unfinished run.
//
// Each agent's wrap-up prompt can be overridden on demand in the admin UI (stored in
// agents.wrapup_prompt); leaving it empty uses the built-in default here. Only the
// [prompt body] is editable; which tools are disabled and how many turns the wrap-up
// itself gets are fixed code policy.

// WrapupOverride, if set, returns the stored wrap-up prompt for an agent key and
// whether a non-empty one exists. Wired by the server to the agents table (like
// PromptOverride for system prompts). nil / empty → the built-in default is used.
var WrapupOverride func(agentKey string) (string, bool)

// WrapupMaxTurnsOverride, if set, returns the admin-configured turn budget for the
// wrap-up phase of an agent and whether a positive one exists. Wired to the agents
// table. nil / ≤0 → the built-in per-agent default (wrapupTurnDefaults) is used.
var WrapupMaxTurnsOverride func(agentKey string) (int, bool)

// Built-in default wrap-up prompts, indexed by agent key. worker reuses the historically
// hard-coded settleWrapUpPrompt (defined in worker.go); planner/mainagent each have their
// own; unmatched agents (custom agents) fall back to the generic default.
var wrapupDefaults = map[string]string{
	"worker":    settleWrapUpPrompt,
	"planner":   plannerWrapUpDefault,
	"mainagent": mainAgentWrapUpDefault,
}

// wrapupTurnDefaults: built-in default turn budgets for each agent's wrap-up phase [itself]
// (overridable in the admin UI with a value > 0). All are 10 turns, ensuring the wrap-up phase
// has enough steps to persist results. Unmatched agents use genericWrapupTurns.
var wrapupTurnDefaults = map[string]int{
	"worker":    10,
	"planner":   10,
	"mainagent": 10,
}

const genericWrapupTurns = 10

const plannerWrapUpDefault = "This planning round's step budget is about to be exhausted. Note that only [this round] is ending; the system will wake you again later as the situation changes so you can continue planning. The task itself is not over, so there is no need to wrap up the entire plan here. Do reflect the conclusions you have already clearly reached this round so this round is not wasted, but [do not invent intents just to wrap up] (having zero intents this round is a completely normal result). (1) If you have already determined a [must-dispatch-now] exploration direction, submit it bundled in a single add_intent call (do not sit on something already decided — send it immediately). (2) Mark goals already proven achieved by some finding or fact as met with prove_goal (do not leave any out). (3) If you identified a serial exploitation chain that must be split into stages, record it with TodoWrite so you can continue dispatching it when you wake up next. When everything is done, end this round immediately and do not output summary text."

const mainAgentWrapUpDefault = "The step budget is about to be exhausted, so this interaction is ending. Do not start any new exploration or manipulation. **As pure text limited to a single sentence**, summarize for the user in the user-facing output language the current progress, the key conclusions, and the recommended next steps."

const genericWrapUpDefault = "You are about to end because the budget is exhausted. First record the results you completed but have not yet saved, then **as pure text limited to a single sentence** summarize in the user-facing output language what you did and what key conclusion you reached (this one sentence is displayed as the result of this run)."

// WrapupDefault returns the built-in default wrap-up prompt for an agent key —
// used by the admin UI as the "restore default" value and empty-field placeholder.
func WrapupDefault(agentKey string) string {
	if d, ok := wrapupDefaults[agentKey]; ok {
		return d
	}
	return genericWrapUpDefault
}

// WrapupTurnsDefault returns the built-in wrap-up turn budget for an agent key —
// used by the admin UI as the "0 = default N" hint.
func WrapupTurnsDefault(agentKey string) int {
	if n, ok := wrapupTurnDefaults[agentKey]; ok {
		return n
	}
	return genericWrapupTurns
}

// resolveWrapup returns the effective wrap-up prompt: the DB override (if set and
// non-empty) over the built-in default.
func resolveWrapup(agentKey string) string {
	if WrapupOverride != nil {
		if t, ok := WrapupOverride(agentKey); ok && strings.TrimSpace(t) != "" {
			return t
		}
	}
	return WrapupDefault(agentKey)
}

// resolveWrapupTurns returns the effective wrap-up turn budget: a positive DB
// override over the built-in per-agent default.
func resolveWrapupTurns(agentKey string) int {
	if WrapupMaxTurnsOverride != nil {
		if v, ok := WrapupMaxTurnsOverride(agentKey); ok && v > 0 {
			return v
		}
	}
	return WrapupTurnsDefault(agentKey)
}

// wrapupSettlement builds the settlement config for an agent's run. Prompt and the
// turn budget are admin-editable per agent; disabled tools are code-owned policy so
// a user can't edit away the "stop probing" guardrail. Resolved fresh each run
// (reads DB live), so edits apply on the next run without a restart.
func wrapupSettlement(agentKey string, disabledTools []string) *harness.Settlement {
	return &harness.Settlement{
		Prompt:        withWrapupLang(resolveWrapup(agentKey)),
		DisabledTools: disabledTools,
		MaxTurns:      resolveWrapupTurns(agentKey),
	}
}

// withWrapupLang appends the output-language directive (langDirective) after the wrap-up prompt.
// Unlike the system prompt, the wrap-up prompt does not automatically get the langDirective tail,
// so it must be appended explicitly here so that the final one-sentence summary shown to the user
// follows the selected display language.
// The wrap-up prose itself is an operator instruction the model reads (English,
// like the rest of the prompt), and this tail only forces the language of
// user-visible output with the highest priority. It is harmless for the planner
// family, which does not emit a summary. For the default language (English) the
// tail is omitted: the wrapping prompt is already English, and this keeps the
// default path's model input identical to the previous revision.
func withWrapupLang(prompt string) string {
	if strings.TrimSpace(prompt) == "" {
		return prompt
	}
	lang := resolveOutputLang()
	if lang == defaultOutputLang {
		return prompt
	}
	return prompt + langDirective(lang)
}

// ---------- Task-level timeout wrap-up wording (see the task-timeout and wrap-up design notes) ----------
//
// This is a separate set from the per-run wrap-up wording: per-run means "your budget for
// this one run is used up"; task timeout means "the whole task has reached its deadline and is
// about to end". Their semantics are often opposite (especially for planner: per-run says
// "don't stop, keep planning", while task timeout says "time's up, stop planning and make the
// final judgment"). Configured only for worker/planner.

// WrapupTaskTimeoutOverride / …TurnsOverride: DB overrides for the task-timeout wrap-up
// wording and turn count (wired to agents.task_timeout_wrapup_prompt / _max_turns,
// worker/planner only).
var (
	WrapupTaskTimeoutOverride      func(agentKey string) (string, bool)
	WrapupTaskTimeoutTurnsOverride func(agentKey string) (int, bool)
)

var taskTimeoutWrapupDefaults = map[string]string{
	"worker":  workerTaskTimeoutDefault,
	"planner": plannerTaskTimeoutDefault,
}

const workerTaskTimeoutDefault = "**The entire task has reached its time limit and is about to end** (this is not this run's budget — the whole exploration is ending). This is the last chance. (1) Save [all] content already identified but not yet recorded. Record new assets with insert_assets, exploration conclusions and facts with record_fact, and confirmed vulnerabilities with report_finding. (2) Do not start any new commands or probes. (3) **At the very end, as pure text limited to a single sentence**, summarize in the user-facing output language the key conclusion obtained from this intent."

const plannerTaskTimeoutDefault = "**The entire task has reached its time limit and is about to end** (this is not this round — the whole task is ending). Perform the final goal judgment based on [all] facts and findings so far. Mark goals proven achieved by evidence as met with prove_goal (do not leave any out). **Do not generate any new intents** (even if you dispatch one now, it will no longer be executed). When the judgment is complete, end immediately and do not output summary text."

// TaskTimeoutWrapupDefault returns an agent's built-in default task-timeout wrap-up wording
// (for the admin UI placeholder / restore default).
func TaskTimeoutWrapupDefault(agentKey string) string {
	return taskTimeoutWrapupDefaults[agentKey] // not configured (mainagent/chat) returns an empty string
}

// resolveTaskTimeoutWrapup: DB override (non-empty) > built-in default. An empty string means
// the agent has no task-timeout wording (not worker/planner), in which case the caller should
// fall back to the per-run wording.
func resolveTaskTimeoutWrapup(agentKey string) string {
	if WrapupTaskTimeoutOverride != nil {
		if t, ok := WrapupTaskTimeoutOverride(agentKey); ok && strings.TrimSpace(t) != "" {
			return t
		}
	}
	return TaskTimeoutWrapupDefault(agentKey)
}

func resolveTaskTimeoutTurns(agentKey string) int {
	if WrapupTaskTimeoutTurnsOverride != nil {
		if v, ok := WrapupTaskTimeoutTurnsOverride(agentKey); ok && v > 0 {
			return v
		}
	}
	return resolveWrapupTurns(agentKey) // by default reuse the per-run turn count
}

// wrapupSettlementForTask builds settlement for a worker/planner run that is aware
// of the task deadline. See §5 of the design doc:
//   - clamped=true  → this run is clamped by the task deadline: a Timeout wrap-up = task deadline
//     reached → task-timeout wording; a MaxTurns wrap-up = turns exhausted within the clamped
//     window while minutes remain in the task → fall back to the per-run wording.
//   - clamped=false → the task deadline is still far off: both reasons use the per-run wording
//     (i.e. degrade to wrapupSettlement).
//
// PromptByReason handed to the harness selects by the [actual] reason at wrap-up time, avoiding
// a mismatch at build time.
func wrapupSettlementForTask(agentKey string, disabledTools []string, clamped bool) *harness.Settlement {
	perRun := withWrapupLang(resolveWrapup(agentKey))
	st := &harness.Settlement{
		Prompt:        perRun, // fallback (also the value for both reasons when not clamped)
		DisabledTools: disabledTools,
		MaxTurns:      resolveWrapupTurns(agentKey),
	}
	if clamped {
		if tt := resolveTaskTimeoutWrapup(agentKey); tt != "" {
			st.PromptByReason = map[harness.TerminalReason]string{
				harness.ReasonTimeout:  withWrapupLang(tt), // task deadline reached
				harness.ReasonMaxTurns: perRun,             // turns exhausted first, task still has time
			}
			st.MaxTurns = resolveTaskTimeoutTurns(agentKey)
		}
	}
	return st
}
