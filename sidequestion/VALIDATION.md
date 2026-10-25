# `/btw` validation record

Date: 2026-09-10. Branch: `codex/btw-side-question`. Baseline commit: `8dae851b9b622f2ff2631f332fde9719d0b16fba`.

An independent PostgreSQL test DB and data directory were used. Real model credentials were injected only into the isolated test environment and were not written into code or this record, and the product's default model was not changed either. Go 1.26.3, norma v0.3.6, Next.js 16.2.9. No API credentials were recorded.

## Engineering checks

All of the following passed; the evidence test is given in parentheses.

- Deep copy of structured messages and tool arguments: pass (evidence `TestCheckpointDeepCopyAndBoundaries`).
- Summary and compaction requests do not overwrite; completed responses and terminal states are published; half-finished responses during generation are excluded: pass (evidence `TestCheckpointDeepCopyAndBoundaries`, `TestSnapshotExcludesPartialStreamAndSelectsPoolMember`).
- Actual model pool member identity: pass (evidence `TestSnapshotExcludesPartialStreamAndSelectsPoolMember`).
- Tool pairing, replay of 20 exchanges, budget reduction, and over-budget errors: pass (evidence `TestBuildRequestCompactionToolPairingAndBudget`).
- Parallel execution of main and side questions, and bidirectional cancellation isolation: pass (evidence: blocking Provider, `TestMainSideConcurrencyAndIndependentCancellation`).
- No tool execution, streaming and non-streaming, existing usage at the time of failure: pass (evidence `TestServiceNoToolsAndUsageOnFailure`).
- Real norma ChatAgent with the local Read tool, isolation of the main transcript and activity: pass (evidence: the streaming and non-streaming subcases of `TestSideActualChatCheckpointToolResultAndTranscriptIsolation`).
- Persistence, pagination, idempotency, and preservation of partial responses after restart: pass (evidence `TestSideHistoryIdempotencyPagingAndRecovery`).
- Clear versus late-write races, parent resource deletion, and version comparison: pass (evidence `TestSideClearLateWritersAndDeletedParent`).
- MainAgent and Worker archive and restore (v1, v2, v3): pass (evidence `TestSideTaskArchiveVersions`).
- Three parent interfaces, authentication, resource attribution, and Worker logical deletion: pass (evidence `TestSideHTTPGlobalLimitTaskWorkerAndDeletion`, `TestSideCheckpointPersistsBeforeAdmissionAndRestart`).
- Side questions available even on a busy main session, independent SSE reconnect and disconnect, cancel, and clear: pass (evidence `TestSideHTTPBusyIsolationClearAndReconnect`).
- 1 concurrent run per parent session and 4 globally: pass (evidence: the two `TestSideHTTP…` cases).
- Snapshot saved before submission, follow-up questions after restart, and old sessions unable to forge a snapshot: pass (evidence `TestSideCheckpointPersistsBeforeAdmissionAndRestart`).
- Rejects continuation when the cached configuration is deleted or the model changes: pass (evidence `TestSideRejectsDeletedOrChangedCachedProfile`).
- Cancels before archive and waits for the final response and usage to be saved: pass (evidence `TestSideTaskDrainPersistsBeforeArchive`).
- Records usage once and attributes it to the side question even if the streaming consumer cancels early: pass (evidence `TestSideUsageRecordedOnceOnConsumerCancellation`).
- Worker and deadline execution contexts automatically restored after restart keep publishing new snapshots: pass (evidence `TestSideRestoredWorkerRuntimePublishesNewCheckpoint`).
- Race check of the related packages: pass (evidence: the command below).
- TypeScript and production build: pass (evidence `npx tsc --noEmit`, `npm run build`).
- Biome check of the newly added frontend modules: pass (evidence `biome check`, 3 new modules).

Automated checks can be reproduced by setting `ARTEX_PG_DSN` to a disposable database (do not point it at a production DB):

```sh
go test -race ./agent ./db ./server ./sidequestion ./llmrec ./llmpool \
  -run 'Test(Side|Checkpoint|Snapshot|BuildRequest|Service|MainSide|CaptureRun|TaskArchive|CompleteForwards|StopIntent|CancelIntent)' -count=1
cd web
npx tsc --noEmit
npx biome check src/lib/side-questions.ts src/hooks/use-side-questions.ts src/components/side-question-workspace.tsx
npm run build
```

The full Go regression suite is not all green. Two existing tests in the `server` package fail during temporary-directory cleanup, and both report `TempDir RemoveAll … directory not empty`:

- `TestInheritedActivityDetailAndRelationDeletion`
- `TestTaskMetadataPatchReturnsRenameAndPin`

Downloading the source at the unmodified baseline commit above and re-running the `server` package in the same isolated environment reproduces both cleanup failures identically. In the baseline-commit run, an assertion failure about the target node count in `TestCoreTaskLifecyclePG` also appeared separately, but that assertion failure did not appear in the `server` regression of the final revision. The other packages passed, and the side-question cases and race checks also passed. The baseline commit's problems were not marked as passing in this review, and existing assertions were not changed to hide problems.

The Next.js build prints existing warnings about multiple lockfiles and workspace root inference, but the build completes and all pages are generated successfully.

## Browser checks

Using Codex's built-in browser, connected to an independent local Go service and a Next.js development server. On desktop and a narrow 390 × 844 screen, performed the following manual and automated interactions and checked screenshots and browser logs:

- While regular chat is running, typing `/btw` shows the main content and the side question at the same time, and the desktop sidebar works correctly.
- Then asked a follow-up question. Stopping the side question keeps the part already generated, and the main flow continues.
- Closing the panel does not stop the request; reopening it recovers the completed response. After refreshing the page and typing only `/btw` with no content, the history is recovered.
- In the narrow-screen Drawer, input, buttons, history, and close interactions work correctly, with no horizontal overflow.
- Clear shows a confirmation popup; after clearing, the history disappears but the main transcript and snapshot remain intact.
- When asking the task's MainAgent and two Workers separately and switching between them, the agent tabs and histories did not mix.
- With a Worker kept running by a blocking local model fixture, submitted `/btw` from the Worker's main input. Even after stopping the side question, the Worker still displayed its live run state and its own pause button, and the side question saved the partial response.
- Browser error and warning logs are empty.

The controllable fixture was used to verify concurrency timing precisely, without depending on the real model's output speed. During debugging, two Worker-run checks did not produce a valid concurrency window (the task had already finished or the response had ended early); the fixture was fixed and the run repeated, and it passed. These initial attempts are not counted as valid passes.

## Real model conversations

First `grok-4.6` was detected. The OpenAI-compatible interface is `http://127.0.0.1:12580/tingly/openai`. Detection returned HTTP 200 along with the model name `grok-4.6` and `READY`, taking 2.82 seconds. Because the first priority was available, the fallback chain of Tingly's `glm` and Zhipu's `glm-5.3` was not enabled, and these two fallback services were not validated this time.

- Asked about assets, goals, and markers while the main session was running: returned `redhaze.top`, first-page reading, a goal summary, and `BTW-REAL-0910`, and the side question completed (16.97 seconds).
- After the main session finished reading the first page, asked about tool evidence: WebFetch 200, curl's 301 → 302 → 200 redirects, and quoted the page title accurately (7.24 seconds).
- The side question asked to create a test file with Bash: it refused execution, and the target file was not created (7.74 seconds).
- A side question after completion did not change the main context: the main transcript's SHA-256 and the main activity log matched exactly, and the side question's tool execution count was 0.
- Actually stopping and restarting the Go service and then asking a follow-up: preserved 3 previous side-question records, answered assets, markers, and title directly from the persisted snapshot, and did not re-run the main agent.
- Used the Grok non-streaming configuration in a new session: answered the asset and `ATOMIC-0910` accurately, and returned and stored usage (input 11734, output 138, cache_read 11520).

For the asset case, the main session read the public first page with WebFetch and Bash/curl; the landing page was `https://id.redhaze.top/home`, and the model returned the page's portal title verbatim. Bash only saved the response temporarily to a local test file and performed no remote write. This fact was confirmed separately from the point that "the side question did not execute tools."

Main transcript verification value: `e7e61f135a4a120954b539f357e8c4205d7d5cd7460dcaf3dc0fd066463e1d00`.

**Usage limitation:** Tingly's Grok streaming response did not return usage. Sending `stream_options.include_usage=true` directly to verify returned HTTP 200, 12 data frames, and 0 usage frames. Therefore, the 0 in the streaming test means that the endpoint does not provide usage, and must not be interpreted as meaning there was no billing. Non-streaming usage and the fixture's failure and cancellation usage were all stored correctly.

## Qwen review

The review model was `qwen-flash`, the OpenAI-compatible interface `https://dashscope.aliyuncs.com/compatible-mode/v1`, HTTP 200. It was given the three real side-question conversations above, the main session's tool evidence, and the engineering assertions, and returned `verdict: accept` and `concerns: []`. It judged that the responses were consistent with the asset, marker, and page-reading evidence, and that the side question's refusal of tools complied with the constraints. Review usage: prompt 6625, completion 312, total 6937.

This Qwen review scope does not include the service restart and non-streaming tests added later. Qwen's generalization of "no writes" is too broad. The main session's curl did actually create a local response temp file, and this was clearly recorded above. Concurrency, zero tool executions, and transcript isolation are judged by engineering assertions; model review only assists in evaluating response quality.
