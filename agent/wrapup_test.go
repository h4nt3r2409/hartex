package agent

import (
	"strings"
	"testing"
	"unicode"
)

// Wrap-up / settlement prompts in English.
//
// These constants are injected during settlement when a run or task hits a step
// or time budget, and they directly instruct the final summary the user sees.
// They must therefore (1) be written in English, (2) contain no CJK leftovers,
// and (3) preserve tool names and instructions such as "one sentence of pure
// text".
//
// The DB seed leaves wrapup_prompt / task_timeout_wrapup_prompt empty (the
// built-in agent INSERT in db/db.go does not fill those columns); when empty,
// these constants are the fallback. They are the single source of truth.
func TestWrapupPromptsAreEnglish(t *testing.T) {
	all := map[string]string{
		"settleWrapUpPrompt":        settleWrapUpPrompt,
		"plannerWrapUpDefault":      plannerWrapUpDefault,
		"mainAgentWrapUpDefault":    mainAgentWrapUpDefault,
		"genericWrapUpDefault":      genericWrapUpDefault,
		"workerTaskTimeoutDefault":  workerTaskTimeoutDefault,
		"plannerTaskTimeoutDefault": plannerTaskTimeoutDefault,
	}

	hasScript := func(s string, table *unicode.RangeTable) bool {
		for _, r := range s {
			if unicode.Is(table, r) {
				return true
			}
		}
		return false
	}

	for name, p := range all {
		if !hasScript(p, unicode.Latin) {
			t.Errorf("%s: no Latin text found; the prompt is not English", name)
		}
		if hasScript(p, unicode.Hangul) || hasScript(p, unicode.Han) {
			t.Errorf("%s: CJK leftovers remain: %q", name, p)
		}
	}

	// Tool names are identifiers and must survive verbatim. The worker prompts
	// (per-run and task-timeout) must keep record_fact for conclusions,
	// report_finding for findings, and the final one-sentence pure-text summary.
	mustContain := func(name, p string, subs ...string) {
		for _, s := range subs {
			if !strings.Contains(p, s) {
				t.Errorf("%s: required instruction %q is missing", name, s)
			}
		}
	}
	mustContain("settleWrapUpPrompt", settleWrapUpPrompt,
		"insert_assets", "record_fact", "report_finding", "single sentence", "pure text")
	mustContain("workerTaskTimeoutDefault", workerTaskTimeoutDefault,
		"insert_assets", "record_fact", "report_finding", "single sentence", "pure text")
	mustContain("genericWrapUpDefault", genericWrapUpDefault, "single sentence", "pure text")
	mustContain("mainAgentWrapUpDefault", mainAgentWrapUpDefault, "single sentence", "pure text")
	// The planner does not emit a summary sentence (it only judges and exits) and
	// keeps the intent/goal/todo tools.
	mustContain("plannerWrapUpDefault", plannerWrapUpDefault, "add_intent", "prove_goal", "TodoWrite")
	mustContain("plannerTaskTimeoutDefault", plannerTaskTimeoutDefault, "prove_goal")
}

// Per-run and task-timeout have different meanings (especially for the planner):
// per-run means "only this round ends", task-timeout means "the whole task ends".
// A routing change must not mix them up.
func TestWrapupDefaultsRouting(t *testing.T) {
	if WrapupDefault("worker") != settleWrapUpPrompt {
		t.Error("worker per-run default is not settleWrapUpPrompt")
	}
	if WrapupDefault("planner") != plannerWrapUpDefault {
		t.Error("planner per-run default is not plannerWrapUpDefault")
	}
	if WrapupDefault("mainagent") != mainAgentWrapUpDefault {
		t.Error("mainagent per-run default is not mainAgentWrapUpDefault")
	}
	// An unregistered key (custom agent) falls back to generic.
	if WrapupDefault("unknown-agent") != genericWrapUpDefault {
		t.Error("an unregistered key did not fall back to genericWrapUpDefault")
	}
	// task-timeout exists only for worker/planner; others return an empty string
	// (the caller falls back to the per-run prompt).
	if TaskTimeoutWrapupDefault("worker") != workerTaskTimeoutDefault {
		t.Error("worker task-timeout default is not workerTaskTimeoutDefault")
	}
	if TaskTimeoutWrapupDefault("planner") != plannerTaskTimeoutDefault {
		t.Error("planner task-timeout default is not plannerTaskTimeoutDefault")
	}
	if TaskTimeoutWrapupDefault("mainagent") != "" {
		t.Error("mainagent must have no task-timeout wording (empty string)")
	}
	// Identical per-run and task-timeout wording would lose the semantic split.
	if workerTaskTimeoutDefault == settleWrapUpPrompt {
		t.Error("worker per-run and task-timeout wording are identical")
	}
	if plannerTaskTimeoutDefault == plannerWrapUpDefault {
		t.Error("planner per-run and task-timeout wording are identical")
	}
}
