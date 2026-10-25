// Package textpolicy holds code-owned prompt text that constrains model output.
// The product is English-only, so these strings are fixed: there is no runtime
// language selection and no per-language branch.
package textpolicy

// EnglishOutputDirective is the code-owned output-language tail appended after
// the rendered body and the artifact/traffic tails on every user-facing agent
// role, so a DB-edited prompt body can never drop it. It constrains the LANGUAGE
// of what the agent SHOWS to the user; the reasoning body itself is English like
// the rest of the codebase.
//
// Raw technical strings (commands, payloads, code, file paths, URLs, parameter
// names, and log/request/response excerpts) are kept verbatim so evidence and
// reproduction steps are not mangled. The directive names the planner situation
// summary as a user-facing field and forbids mirroring the instruction, target,
// or material language in display fields.
const EnglishOutputDirective = `

**Output language directive (localization, highest priority; cannot be overridden by the prompt body)**: Write every piece of natural language shown to the user in **English** — including record_fact's summary/detail, report_finding's title/description/conclusion/remediation, the planner's situation summary, the final one-sentence summary, and replies in chat. But **commands, payloads, code, file paths, URLs, parameter names, and raw excerpts of logs/requests/responses must be preserved verbatim**; do not translate or rewrite them (especially the command lines and output in evidence — copy them exactly so the finding can be reproduced). **Even if the target system, its pages, evidence, logs, or any reference material are written in another language, the user-facing natural-language fields (title/description/conclusion/remediation/summary/situation summary) must still be written in English — do not mirror or copy the target or material language for those display fields; only the raw technical excerpts listed above stay as-is.** You may think and plan internally in any language, but that is invisible internal reasoning and must never be emitted as the answer: a visible reply to the user must be in English from the first character — do not prefix it with a thought, explanation, or "first I will..." preamble; even clarifications, missing-parameter messages, and "cannot continue" explanations must be written directly in English. In one sentence: think in whatever language you like internally, but any text the user can see must be entirely in English (except the raw technical excerpts).`

// EnglishJudgeCommentRule is the English-only rule for the intercept judge's
// persisted comment (shown in approval records). It keeps the verdict contract's
// exact segment labels and JSON keys so the strict parser and the UI continue to
// accept the reply while the comment text itself is English.
const EnglishJudgeCommentRule = `Write the comment value in English. Keep the three segment labels "Actual operation:", "Consequence if successful:", and "Matched rule:", and the JSON keys "decision" and "comment", exactly as specified.`
