package agent

import (
	"bytes"
	"fmt"
	"text/template"
	"time"
)

// PromptOverride, if set, returns the stored system-prompt template for an agent
// key and whether one exists. The server wires it to the PG agent_prompts table.
// When nil or no override exists, agents use their built-in default prompt — so
// behavior is identical until a user edits a prompt in the UI.
var PromptOverride func(agentKey string) (string, bool)

// Prompt-variable structs — fields mirror each agent's catalog (docs §5a) so a
// user template referencing a catalog variable renders; referencing anything else
// fails template execution and falls back to the built-in default.
type PlannerVars struct{ Goal, Scope, AssetSummary, DataDir, Now string }
type WorkerVars struct{ ProxyAddr, WorkerName, DataDir, Now string }
type MainVars struct{ Goal, AssetSummary, FindingsSummary, DataDir, Now string }
type GoalsVars struct{ EngagementDescription, DataDir, Now string }

// nowStr is the server-local wall-clock string exposed as the universal {{.Now}}
// prompt variable. renderSystem runs on every agent turn/round, so this is fresh
// each run — a prompt can subtract it from a fixed start stamp to reason about
// elapsed time (e.g. a timed benchmark's "last N hours" window).
func nowStr() string { return time.Now().Format("2006-01-02 15:04:05 MST") }

// renderSystem returns the rendered system-prompt BODY (section [A]) for agentKey.
// Precedence: the DB-stored template (if any) over the built-in default template
// (def). BOTH are Go templates now — the built-in default is seeded into the DB
// verbatim, so the two paths render identically until a user edits the prompt.
// Rendering always runs (def used to be pre-substituted plain text; it is now a
// {{.Var}} template like the DB one). On any render error we fall back to the
// default template, then to the raw default string — an agent never starts with a
// half-rendered prompt. Callers append the code-owned tail (trafficTool / the
// intermediate-artifact output spec) AFTER this, so those cannot be edited away
// through the DB body.
func renderSystem(agentKey, def string, vars any) string {
	tmpl := def
	if PromptOverride != nil {
		if t, ok := PromptOverride(agentKey); ok && t != "" {
			tmpl = t
		}
	}
	if out, err := renderTmpl(tmpl, vars); err == nil {
		return out
	}
	// DB template broke (e.g. references an out-of-catalog var) → code default.
	if out, err := renderTmpl(def, vars); err == nil {
		return out
	}
	return def
}

// langDirective is the output-language tail: a code-owned segment appended
// AFTER the rendered body and the artifact/traffic tails on every user-facing
// agent role, so a DB-edited prompt body can never drop it — the same guarantee
// artifactSpec gives. It constrains the LANGUAGE of what the agent SHOWS to the
// user; the reasoning body itself is English like the rest of the codebase.
// Raw technical strings (commands, payloads, code, URLs, log/response excerpts)
// are kept verbatim so evidence and reproduction steps are not mangled.
//
// The directive names the planner situation summary as a user-facing field and
// forbids mirroring both the instruction language and the target/material
// language in display fields. It is written in English; the display language is
// a parameter ("en"/"ko"/"zh"/"es"/"ja") and the "write in <language>" token is
// swapped per target. When the display language IS English, the anti-English
// clause is omitted because English output is the goal.
func langDirective(code string) string {
	phrase := langPhrase[code]
	bare := langBare[code]
	if phrase == "" || bare == "" {
		phrase, bare = langPhrase[defaultOutputLang], langBare[defaultOutputLang]
	}
	base := `

**Output language directive (localization, highest priority; cannot be overridden by the prompt body)**: Write every piece of natural language shown to the user in **%s** — including record_fact's summary/detail, report_finding's title/description/conclusion/remediation, the planner's situation summary, the final one-sentence summary, and replies in chat. But **commands, payloads, code, file paths, URLs, parameter names, and raw excerpts of logs/requests/responses must be preserved verbatim**; do not translate or rewrite them (especially the command lines and output in evidence — copy them exactly so the finding can be reproduced). **Even if the target system, its pages, evidence, logs, or any reference material are written in another language, the user-facing natural-language fields (title/description/conclusion/remediation/summary/situation summary) must still be written in %s — do not mirror or copy the target or material language for those display fields; only the raw technical excerpts listed above stay as-is.** You may think and plan internally in any language, but that is invisible internal reasoning and must never be emitted as the answer: a visible reply to the user must be in %s from the first character — do not prefix it with a thought, explanation, or "first I will..." preamble; even clarifications, missing-parameter messages, and "cannot continue" explanations must be written directly in %s. In one sentence: think in whatever language you like internally, but any text the user can see must be entirely in %s (except the raw technical excerpts).`
	if code == "en" {
		return fmt.Sprintf(base, phrase, bare, bare, bare, bare)
	}
	extra := ` **Even if the system/role instructions above are written in English, never emit English to the user — the user-facing display language is %s only, and no English sentence may appear in user-visible text.** **You may think and plan internally in English, but that is invisible internal reasoning and must never be emitted as the answer; even clarifications, missing-parameter messages, and "cannot continue" explanations must be written directly in %s.**`
	return fmt.Sprintf(base+extra, phrase, bare, bare, bare, bare, bare, bare, bare)
}

func renderTmpl(tmpl string, vars any) (string, error) {
	t, err := template.New("p").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, vars); err != nil {
		return "", err
	}
	return b.String(), nil
}
