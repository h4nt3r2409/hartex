# Long side-question conversations and the context budget

English · [한국어](CONTEXT_BUDGET.ko.md) · [中文](CONTEXT_BUDGET.zh.md)

Revised 2026-09-11. The previous implementation treated the character count of the request JSON as the token count and inherited the main task's 32K output reservation as-is, so it rejected legitimate side questions in advance when there was a lot of HTML, JS, or tool output.

> This document is the English translation of the original Chinese document (`CONTEXT_BUDGET.zh.md`). The original is preserved as-is so that upstream changes are easy to compare.

## Comparison with open-source implementations

- [Grok CLI side-question context](https://github.com/superagent-ai/grok-cli/blob/fb97af83f06dca873281d60168430f06c8de6324/src/agent/agent.ts#L739): It extracts some fragments from the most recent user and assistant text, with a character budget of about 2000 and a cap of 400 characters per item. This path does not maintain a history of consecutive side-question Q&A.
- [Grok CLI independent request](https://github.com/superagent-ai/grok-cli/blob/fb97af83f06dca873281d60168430f06c8de6324/src/utils/side-question.ts): It uses a separate cancellation signal. When the model supports it, the output limit is 2048 tokens. No tools are provided.
- [Grok CLI main session compaction](https://github.com/superagent-ai/grok-cli/blob/fb97af83f06dca873281d60168430f06c8de6324/src/agent/compaction.ts): It estimates tokens, keeps recent content, folds new content into the previous summary, and handles truncation across turn boundaries.
- [OpenCode session compaction](https://github.com/anomalyco/opencode/blob/b3f1a96c6dd7adeb28b36dd11add1998fc84d67b/packages/core/src/session/compaction.ts): It estimates the whole request, reserves output and buffer, adds a rolling summary to the recent content, and sends a tool-free summary request. This implementation uses a default recent budget of 8000 and a summary output limit of 4096 tokens.
- [OpenCode overflow recovery](https://github.com/anomalyco/opencode/blob/b3f1a96c6dd7adeb28b36dd11add1998fc84d67b/packages/core/src/session/runner/llm.ts): It attempts overflow recovery only when assistant output has not yet started, and calls made after recovery do not re-enter the same overflow-recovery path.

ARTEX adopted the approach of an independent output budget, recent content plus a rolling summary, and bounded recovery. It retains norma v0.3.6's structured messages and tool pairing as-is, but does not adopt Grok's text-excerpt approach verbatim, and does not write OpenCode's main-session compaction events to the ARTEX main transcript.

## Request budget and execution

- Messages use norma's method as-is: estimate UTF-8 bytes per content block and add a 4/3 margin. The system prompt, tool schema, and message-wrapping overhead are counted additionally. This estimate is not the model's exact token count.
- Side-question output is at most 8192 tokens by default, and does not exceed the output limit already set in the main configuration. The service environment variable `ARTEX_BTW_MAX_OUTPUT_TOKENS` can set a limit in the range 256–32768, without changing the product's default model or the main task parameters.
- The input budget is the context window minus the output limit and a safety margin; when the window size is unknown, the platform default of 200K is used. The safety margin is 5% of the window, with a minimum of 128 tokens and a maximum of 8192 tokens.
- Successful Q&A is loaded in increasing ordinal order, up to 20 exchanges at a time. Verbatim text is retained for up to 20 exchanges, and its token budget is at most 1/4 of the input budget and no more than 16K.
- Q&A that exceeds the budget is folded into a rolling summary. The summary contains the source record and the context time point. Past assistant answers are not the same as new tool evidence, so the latest main snapshot takes precedence when they conflict.
- If the main context is still too long, only the older messages of the copy are summarized, retaining recent content up to 8K tokens. The cut point does not split a tool call from its result. If a single exchange is too large, the entire exchange goes into the summary.
- Summary input is split safely on a UTF-8 basis to fit the actually remaining window size, and the output limit is 2048 tokens. Empty summaries, truncated responses, tool calls, and over-budget summaries are all not written to the cache. A single side question makes at most 12 summary calls, all subject to the same 120-second timeout. When the limit is reached, it ends with an explicit failure instead of looping forever.
- If the model returns a context-overflow error for the first time and no text or tool call has been emitted yet, it reduces the context further and retries at most once. If the estimated size does not decrease, recovery stops immediately. Other model errors or partially streamed output do not trigger this recovery.
- All usage already accumulated, including summarization, failed attempts, and usage at the time of cancellation, is added to the same side-question request. If the Provider does not return usage, only 0 is recorded, and estimates are not presented as if they were actual usage.

## Persistence and UI

`side_question_sessions.memory` stores summaries of older Q&A, the reflected ordinal, and main-context summaries cached per snapshot identity. `side_question_requests.context_info` stores the preparation phase, the number of exchanges actually re-included, summary usage details, and budget estimates.

A summary is saved only when the original request is still running and the cleanup version matches. Clear also wipes the cache, and a late-arriving write does not revive already-cleared data. A new snapshot does not reuse the previous snapshot's summary. Summary fields are saved together with the v3 task archive; when restoring an older v3 that lacks the fields, they are filled with an empty object, and v1 and v2 remain compatible.

POST accepts and returns the request first, then runs preparation and compaction in the background without holding the admission lock or a database transaction. SSE and the history screen show the preparation, Q&A consolidation, copy compaction, and response phases. A compaction failure is stored as the failed terminal state of that side-question request. The frontend keeps the error, recovers the draft of the question that just failed, and uses a floating notification that does not cover the input box. History polling also no longer clears submission errors.

## Validation record

- Replay of 19, 20, 21, and 50 exchanges, preservation of older conclusions beyond 20 exchanges, and reuse of the summary cache after restart: automated tests pass.
- Very long Chinese answers and code context, request budget splitting, tool pairing, snapshot immutability, and cache invalidation on a new snapshot: automated tests pass.
- Summary failure, cancellation, truncation, over-length, and tool return; clear races, the call limit, exactly one overflow recovery, and no retry on a partial stream: automated tests pass.
- Pagination on an independent PostgreSQL, restart, v1/v2/v3 archives, archive restoration of the summary cache and budget metadata, an older v3 without the new fields, and 20 parent sessions sharing four concurrency slots: pass.
- Go candidate service build, frontend TypeScript check, Biome check of the modified components, and a Next.js production build in an independent directory: pass.
- Using the bundled browser with an independent UI fixture, at 1280×720 and 390×844 resolutions, verified the Q&A consolidation phase, the summary scope notice, draft recovery after failure, the absence of a floating error notification, the absence of horizontal overflow, and the absence of console errors. The temporary fixture was removed.
- Replaying an already-present local Worker snapshot read-only passed the new budget check. For example, Worker #3's 293,085-character snapshot is no longer incorrectly rejected by the local character-count calculation. This item involved no external model calls.
- The real-conversation test that intended to send a private Worker snapshot to Grok was rejected by the automated approval review and did not run, so it is not counted as a passing item.
- At 00:37 on 2026-09-11, the local backend was restarted at the user's request, using the original database, data directory, and login configuration unchanged. The SHA-256 of the executable matched the candidate binary, and `/api/health` returned healthy for both the backend and the frontend proxy.

Validation commands (using only the independent test DB):

```sh
go test -race ./sidequestion ./db ./server -run 'TestSide|TestCheckpoint|TestSnapshot|TestBuildRequest|TestService|TestMainSide|TestTaskArchive' -count=1
go build ./cmd/artex
npx tsc --noEmit
npm run build -- --webpack
```

The frontend production build used an independent copy so that it would not overwrite the current preview's `.next`. The candidate service is at `/private/tmp/artex-btw-budget-candidate`, and was copied to `/private/tmp/artex-btw-preview/artex` and run. The original binary was backed up as `artex.before-context-budget` in the same directory.
