# English-only ARTEX Global Edition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the five-language localization layer and make the repository and product English-only, keeping the documented functional allowlist and all non-localization behavior.

**Architecture:** The Go backend loses its language config, message catalogs, per-language label tables, output-language setting, and API field; a fixed code-owned English directive is appended to every user-visible LLM prompt. The Next.js app keeps only `web/messages/en.json` and next-intl, hardcodes the locale to `en`, and pins locale-sensitive formatting to `en-US`. Translated docs, templates, and screenshots are removed; English docs that described them are rewritten. Two guard scripts enforce the invariant.

**Tech Stack:** Go 1.26, PostgreSQL, Next.js 16 / React 19 / next-intl 4.14.9, TypeScript, Vitest, Python 3 standard library, GitHub Actions, pre-commit, Docker-based detection tests.

**Spec:** `docs/superpowers/specs/2026-10-25-english-only-design.md` — executors must read both this plan and the spec.

## Global Constraints

- **English-only.** The only non-English content allowed in tracked files is the functional allowlist in spec §7. Everything else is deleted or translated to English.
- **One UI catalog.** Keep `web/messages/en.json` (2,996 leaves) and next-intl. Do not inline all strings into components.
- **No runtime language selection.** Remove `ARTEX_LANG`, the config-file `language` field, the `output_language` settings key, the `output_language` API field, `LOCALE_LABELS`, the locale store, and the language switcher.
- **Fixed English output directive.** The same code-owned English-only instruction applies to the six agent roles, the wrap-up prompt, side-question answers, and the intercept judge's `comment`.
- **No data migration.** The project has no production installation or accumulated run data. Developer databases may be recreated; only the obsolete `output_language` settings key is optionally cleaned up.
- **Keep functional compatibility.** `[模型]`, Chinese chat-mention tokens, legacy Chinese verdict markers, provider quota strings, Chinese log keywords, `备案`, the `traffic_search` migration sentinel, the legacy Sigma guard marker, GBK zip decoding, the api-recon Chinese auth regex, CJK detector ranges, and the full-width comma input separator stay exactly as they are.
- **Keep Unicode capabilities.** IME composition, FTS5 trigram search, UTF-8-safe truncation, RFC 2047 email encoding, UTF-8 BOM, and non-ASCII skill identifiers are not localization and must survive.
- **Do not change detection semantics.** The legacy Chinese Sigma marker and its fixture stay; the detection test suite must remain green.
- **Historical `CHANGELOG.md` entries stay** (they are English history). `CHANGELOG.ko.md` and `CHANGELOG.zh.md` are deleted.
- **Guard exclusions.** `check-source-language.py` skips generated `graphify-out/` and process records under `docs/superpowers/`; no other directory-level exemptions.
- **Commit messages are English.** Use `type(scope): description`.
- **Run `graphify update .`** after the code changes, per the repository rule.
- **Preserve git history.** Never rewrite commits; only add new commits.

## Review Focus

These are the input classes/failure modes the spec implies but that are easy to miss. Each is pinned by a test in the named task.

1. **A historical row still parses.** An old `[模型]` row, a Chinese chat-mention token, or a legacy Chinese verdict must still parse and render after the language layer is gone. Test in Task 2 and Task 3.
2. **A Chinese target still produces English display fields.** A run whose target/material is Chinese must still write findings, summaries, reports, side-question answers, and intercept comments in English. Test in Task 1.
3. **A non-English browser still sees English formatting.** Dates, numbers, month names, and name sorting must be English even when the browser locale is Korean/Chinese. Test in Task 5.
4. **Legacy detection still works.** The Sigma rule must still match `【ARTEX 平台管控·非目标防御】` and the detection suite must pass after `defense-ko.md` is removed. Test in Task 12.
5. **Deleting translated docs leaves no broken links.** The Nginx reverse-proxy example and other unique operational content must be migrated before `README.zh.md` is deleted. Test in Task 8/Task 9 via `check-doc-links.py`.

---

## File Structure

| Area | Files | Responsibility |
|---|---|---|
| English directive | `textpolicy/english.go` (new); `agent/prompt.go`, `agent/{chat,goals,mainagent,planner,worker,wrapup}.go`; `sidequestion/request.go`; `intercept/prompt.go` | One fixed English-only prompt tail and its consumers |
| Go labels | `notify/notify.go`; `intercept/label_i18n.go`; `intercept/intercept.go`; `server/server.go` | English-only severity/status/action labels; remove input aliases |
| Go catalogs/state | `config/config.go`; `config/msg_i18n.go` (delete); `server/errmsg_catalog.go` (delete); `server/errmsg_i18n.go` (delete); `server/server.go`; `server/manager.go`; `cmd/artex/main.go` | Remove language selection, catalogs, settings/API, `ARTEX_LANG` |
| Go tests | `config/*_test.go`, `server/*_test.go`, `agent/*_test.go`, `notify/*_test.go`, `intercept/*_test.go`, `db/*_test.go`, `sidequestion/*_test.go` | Delete localization tests; translate non-functional CJK; keep functional fixtures |
| Web locale | `web/messages/*`; `web/src/i18n/*`; `web/src/app/layout.tsx`; `web/src/config/app-config.ts`; `web/src/app/(main)/system/settings/page.tsx`; `web/src/lib/types.ts` | Single `en` catalog and no locale switching |
| Web formatting | `web/src/i18n/datetime.ts`; `web/src/lib/utils.ts`; the 18 date consumers; `ui/calendar.tsx`; `chat/page.tsx`; `function/tasks/page.tsx`; number-formatting sites | Pin locale-sensitive formatting to `en-US`/`en` |
| Web tooling/data | `web/src/scripts/{extract-i18n.ts,i18n-keys.ts,check-i18n-keys.ts,i18n-keys.test.ts}` (delete); `web/package.json`; `web/src/lib/mock/{data,handler}.ts`; `web/src/lib/agent-i18n.ts` | Remove five-locale tooling; translate non-functional mock data; keep built-in labels |
| Docs | `README.md`, `CONTRIBUTING.md`, `MAINTAINING.md`, `CHANGELOG.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`; `docs/defense-en.md`, `docs/finding-traffic-evidence-en.md`; `detections/**/README.md`; `sidequestion/*.md`; `screenshots/README.md`; `.github/PULL_REQUEST_TEMPLATE.md` | English-only docs and templates |
| Deletions | The 26 translated Markdown files, 4 non-English/translation issue templates, 13 root Chinese screenshots + `wx.png`, `sidequestion/validation-2026-09-10.json` | Remove translated assets |
| Detection/CI | `detections/tests/check-defense-template.py`, `.sh`; `detections/sigma/**`; `detections/suricata/artex.rules`; `detections/sigma_lint/validators.yml`; `.github/workflows/*`; `.pre-commit-config.yaml`; `scripts/*` | Remove Korean-doc dependencies; tighten English-only gates |

---

### Task 1: Fixed English output directive

**Files:**
- Create: `textpolicy/english.go`
- Modify: `agent/prompt.go:57-85`; `agent/chat.go:87`; `agent/goals.go:82`; `agent/mainagent.go:105`; `agent/planner.go:344`; `agent/worker.go:296`; `agent/wrapup.go:107-126`
- Modify: `sidequestion/request.go:33`
- Modify: `intercept/prompt.go:36,42-128` (the default judge prompt)
- Test: `agent/prompt_test.go`; `sidequestion/request_test.go`; `intercept/prompt_test.go`

**Interfaces:**
- `textpolicy.EnglishOutputDirective string` — the code-owned tail, containing the current English-path text of `langDirective("en")` with the “commands/payloads/code/URLs/raw excerpts stay verbatim” and “do not mirror the target/material language” clauses.
- `textpolicy.EnglishJudgeCommentRule string` — `Write the comment value in English. Keep the three segment labels "Actual operation:", "Consequence if successful:", and "Matched rule:", and the JSON keys "decision" and "comment", exactly as specified.`
- `func outputDirective() string` in `agent` returns `textpolicy.EnglishOutputDirective`.
- `sidequestion.instruction` includes `textpolicy.EnglishOutputDirective`.
- `intercept.DefaultJudgePrompt` includes `textpolicy.EnglishJudgeCommentRule`.

- [ ] **Step 1: Write the failing tests**

In `agent/prompt_test.go`, replace the per-language directive tests with:

```go
func TestOutputDirectiveIsEnglishOnly(t *testing.T) {
    got := outputDirective()
    if !strings.Contains(got, "English") {
        t.Fatalf("directive must require English: %q", got)
    }
    for _, forbidden := range []string{"Korean", "Chinese", "Spanish", "Japanese"} {
        if strings.Contains(got, forbidden) {
            t.Fatalf("directive must not name %s: %q", forbidden, got)
        }
    }
    if !strings.Contains(got, "another language") {
        t.Fatalf("directive must forbid mirroring the target/material language: %q", got)
    }
}

func TestWrapupAlwaysAppendsDirective(t *testing.T) {
    got := withWrapupLang("operator prompt")
    if !strings.HasSuffix(got, textpolicy.EnglishOutputDirective) {
        t.Fatalf("wrap-up must always append the English directive: %q", got)
    }
}
```

In `sidequestion/request_test.go`:

```go
func TestInstructionRequiresEnglishAnswer(t *testing.T) {
    if !strings.Contains(instruction, "English") {
        t.Fatalf("side-question instruction must require English: %q", instruction)
    }
}
```

In `intercept/prompt_test.go`:

```go
func TestJudgePromptRequiresEnglishComment(t *testing.T) {
    if !strings.Contains(DefaultJudgePrompt, "Write the comment value in English") {
        t.Fatalf("judge prompt must require an English comment")
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./agent ./sidequestion ./intercept -run 'TestOutputDirectiveIsEnglishOnly|TestWrapupAlwaysAppendsDirective|TestInstructionRequiresEnglishAnswer|TestJudgePromptRequiresEnglishComment' -count=1`

Expected: FAIL — `outputDirective`, the English-only text, or the required sentence does not exist yet.

- [ ] **Step 3: Create `textpolicy/english.go`**

Copy the current English-path directive text from `agent/prompt.go:77-82` (the `base` string with every `%s` replaced by `English`). Keep the clause that commands, payloads, code, paths, URLs, parameter names, and raw log/request/response excerpts stay verbatim, and the clause that display fields must not mirror the target/material language. Add `EnglishJudgeCommentRule` with the exact sentence above.

- [ ] **Step 4: Replace `langDirective(code)` with the fixed directive**

- In `agent/prompt.go`, delete `langDirective`, `langPhrase`, and `langBare`; add `func outputDirective() string { return textpolicy.EnglishOutputDirective }`.
- In the six call sites (`chat.go`, `goals.go`, `mainagent.go`, `planner.go`, `worker.go`, `wrapup.go`), call `outputDirective()` with no argument.
- In `wrapup.go`, always append the directive (remove the `if lang == defaultOutputLang { return prompt }` early return).
- In `sidequestion/request.go`, append `textpolicy.EnglishOutputDirective` to `instruction`.
- In `intercept/prompt.go`, append `textpolicy.EnglishJudgeCommentRule` to the judge system prompt.
- Leave `agent/outputlang.go` and the server wiring in place for this task; Task 3 removes them.

- [ ] **Step 5: Run the package tests**

Run: `go test ./agent ./sidequestion ./intercept -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add textpolicy/english.go agent sidequestion intercept
git commit -m "feat(agent): require English for all user-visible LLM output"
```

---

### Task 2: Remove label localization and non-English input aliases

**Files:**
- Modify: `notify/notify.go:54-100`
- Modify: `intercept/label_i18n.go`; `intercept/intercept.go` (label use)
- Modify: `server/server.go:3946-3962` (input aliases) and the `notify.Lang` assignment
- Test: `notify/notify_test.go`; `intercept/prompt_test.go`; `server/server_localized_test.go` (rewrite) or a new `server/server_english_test.go`

**Interfaces:**
- `notify.SeverityLabel(severity string) string` returns `🔴 Critical` / `🟠 High` / `🟡 Medium` / `🔵 Low` for known values, otherwise the input.
- `notify.StatusLabel(status string) string` returns the English labels (`Pending`, `In progress`, `Confirmed`, `Resolved`, `Fixed`, `False positive`, `Ignored`, `Duplicate`, `Risk accepted`).
- `intercept.judgeActionLabel(action string) string` returns `allow` / `deny` / `ask` for known values.
- No `notify.Lang`, `labelLang`, `judgeActionLang`, or Korean fallback remains.

- [ ] **Step 1: Write the failing tests**

`notify/notify_test.go`:

```go
func TestLabelsAreEnglish(t *testing.T) {
    if got := SeverityLabel("critical"); got != "🔴 Critical" {
        t.Fatalf("SeverityLabel(critical) = %q", got)
    }
    if got := StatusLabel("confirmed"); got != "Confirmed" {
        t.Fatalf("StatusLabel(confirmed) = %q", got)
    }
}
```

`intercept/prompt_test.go`:

```go
func TestJudgeActionLabelIsEnglish(t *testing.T) {
    if got := judgeActionLabel("deny"); got != "deny" {
        t.Fatalf("judgeActionLabel(deny) = %q", got)
    }
}
```

`server/server_english_test.go`:

```go
func TestFallbackCommandRejectsNonEnglishAliases(t *testing.T) {
    if cmd, _ := fallbackCommand("意图: 读取报告"); cmd == "intent" {
        t.Fatal("non-English intent alias must no longer be accepted")
    }
    if cmd, text := fallbackCommand("intent: read the report"); cmd != "intent" || text != "read the report" {
        t.Fatalf("English intent alias = %q, %q", cmd, text)
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./notify ./intercept ./server -run 'TestLabelsAreEnglish|TestJudgeActionLabelIsEnglish|TestFallbackCommandRejectsNonEnglishAliases' -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement**

- In `notify/notify.go`, delete `Lang`, `labelLang`, and the per-language maps. Keep one English map for severity and one for status; `SeverityLabel`/`StatusLabel` read it directly.
- In `intercept/label_i18n.go`, replace the five-language map with an English map or a direct switch; remove `judgeActionLang` and the Korean fallback.
- In `server/server.go`, remove the `notify.Lang = resolveLang` assignment and the Chinese/Korean `intent`/`hint` aliases at `3946-3962`; keep the English `intent`/`hint` prefixes.
- Keep the `[模型]` sentinel and all other functional tokens untouched.

- [ ] **Step 4: Run the package tests**

Run: `go test ./notify ./intercept ./server -count=1`

Expected: PASS (DB-backed server tests may skip without PostgreSQL; run the same command in the DB CI job).

- [ ] **Step 5: Commit**

```bash
git add notify intercept server
git commit -m "refactor(labels): make notification and interception labels English-only"
```

---

### Task 3: Remove Go message catalogs, output-language setting/API, and config language API

**Files:**
- Delete: `config/msg_i18n.go`; `server/errmsg_catalog.go`; `server/errmsg_i18n.go`; `agent/outputlang.go`
- Modify: `config/config.go:28-95`; `config/config.go:203-212`; `config.example.json:20-21`; `.env.example:30-35`; `docker-compose.yml:42-45`
- Modify: `server/server.go:75-76,203-206,308-321,3402-3405,3506-3508,3533-3545`; `server/manager.go:279-282`
- Modify: `cmd/artex/main.go` (config.T call sites); all `trMsg(` / `logT(` / `config.T(` / `trCfg(` call sites
- Test: `server/output_language_test.go` (rewrite); delete `config/config_localized_test.go`, `config/msg_i18n_test.go`, `server/logt_catalog_test.go`, `server/engine_localized_test.go`, `server/intercept_archive_localized_test.go`; update `server/testmain_test.go:33`, `server/chat_llm_resolve_test.go` (`ARTEX_LANG` setups)

**Interfaces:**
- No `config.Config.Language`, `config.SupportedLanguages`, `config.DefaultLanguage`, `config.NormalizeLanguage`, `config.CurrentLanguage`, `config.ActiveLanguage`, or `config.Language()`.
- No `trMsg`, `logT`, `config.T`, or `trCfg`.
- No `Server.outputLang`, `Server.loadOutputLanguage`, `Server.outputLanguage`, `settingOutputLanguage`, `errOutputLanguageInvalid`, or `output_language` in `GET`/`PUT /api/settings`.
- No `agent.OutputLanguage`, `agent.SupportedOutputLangs`, `agent.defaultOutputLang`, `agent.normalizeLang`, or `agent.resolveOutputLang`.

- [ ] **Step 1: Write the failing API test**

Replace `server/output_language_test.go` with a test that asserts the field is gone:

```go
func TestSettingsPayloadHasNoOutputLanguage(t *testing.T) {
    rec := do(http.MethodGet, "/api/settings", nil)
    if rec.Code != http.StatusOK {
        t.Fatalf("GET /api/settings = %d", rec.Code)
    }
    var body map[string]any
    if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
        t.Fatal(err)
    }
    if _, ok := body["output_language"]; ok {
        t.Fatal("GET /api/settings must not expose output_language")
    }
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./server -run TestSettingsPayloadHasNoOutputLanguage -count=1`

Expected: FAIL — the key is present.

- [ ] **Step 3: Remove the catalogs and unwrap the call sites**

- Delete `config/msg_i18n.go` and replace each `trCfg(x)` / `config.T(x)` with the English literal `x`.
- Delete `server/errmsg_catalog.go` and `server/errmsg_i18n.go`; replace each `trMsg(x)` / `logT(x)` with `x`.
- Use `git grep -n 'trMsg(\|logT(\|config\.T(\|trCfg(' -- '*.go'` to enumerate; the argument is the English source string.
- Remove the `config/config.go` language struct field and functions; remove the `os` import if it becomes unused. Remove the language fields from `config.example.json`, `.env.example`, and `docker-compose.yml`.
- Remove the server state, settings key, API field, validation, and error constant; remove the `agent.OutputLanguage`/`config.CurrentLanguage`/`notify.Lang` startup wiring (the `notify.Lang` assignment was already removed in Task 2; remove the rest).
- Delete `agent/outputlang.go` and its test references.

- [ ] **Step 4: Run the Go build and package tests**

Run: `go build ./...` and `go vet ./...`

Expected: clean.

Run: `go test ./config ./agent ./server -count=1` (DB-backed server tests may skip without PostgreSQL; run them in the DB CI job).

Expected: PASS. Confirm that the existing `[模型]`, chat-mention, and legacy-verdict compatibility tests still pass; do not delete them.

- [ ] **Step 5: Commit**

```bash
git add config server agent cmd config.example.json .env.example docker-compose.yml
git commit -m "refactor(server): remove the output-language setting and message catalogs"
```

---

### Task 4: Single English UI catalog and remove locale switching

**Files:**
- Delete: `web/messages/{ko,zh,es,ja}.json`
- Modify: `web/src/i18n/config.ts`; `web/src/i18n/request.ts`; `web/src/i18n/intl-runtime-provider.tsx`; `web/src/app/layout.tsx`; `web/src/config/app-config.ts`; `web/src/app/(main)/system/settings/page.tsx:23-24,67,122-128,672`; `web/src/lib/types.ts:960`
- Delete: `web/src/i18n/locale-store.ts`; `web/src/i18n/config.test.ts` (replace with a single-locale test)
- Test: new `web/src/i18n/config.test.ts`

**Interfaces:**
- `web/src/i18n/config.ts` exports `LOCALE = "en" as const` and `type Locale = typeof LOCALE`; no `LOCALES`, `DEFAULT_LOCALE`, `LOCALE_LABELS`, `LOCALE_COOKIE`, `LOCALE_STORAGE_KEY`, `isLocale`, or `resolveLocale` unless a consumer still needs a minimal guard.
- `web/src/i18n/request.ts` loads only `en.json`.
- `IntlRuntimeProvider` renders `NextIntlClientProvider locale="en" messages={enMessages} timeZone={TIME_ZONE}`.
- `layout.tsx` uses `lang="en"` and static English metadata.
- `app-config.ts` exports English `meta.title`/`meta.description` only.
- The settings page has no display-language section and no `setSettings({ output_language })` call.
- `web/src/lib/types.ts` has no `output_language`.

- [ ] **Step 1: Write the failing single-locale test**

```ts
import { LOCALE } from "./config";

test("the UI has exactly one locale", () => {
  expect(LOCALE).toBe("en");
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd web && npx vitest run src/i18n/config.test.ts`

Expected: FAIL — `LOCALE` is not exported.

- [ ] **Step 3: Implement**

- Delete the four non-English catalogs.
- Reduce `config.ts` to `LOCALE`/`Locale` (and a minimal `isLocale`/`resolveLocale` only if another module still imports it; prefer deleting the imports).
- Simplify `request.ts` to the single `en.json` import.
- Simplify `intl-runtime-provider.tsx` to the static provider; remove `useActiveLocale` and the `document.documentElement.lang` effect.
- Update `layout.tsx` to `lang="en"` and static metadata.
- Remove `META_BY_LOCALE` from `app-config.ts`.
- Remove the display-language UI, imports, and the `setSettings({ output_language })` call from the settings page.
- Remove `output_language` from `types.ts`.

- [ ] **Step 4: Run the web tests and static build**

Run: `cd web && npm test -- --run && npm run build:static`

Expected: PASS and a successful static export.

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "refactor(web): keep only the English message catalog"
```

---

### Task 5: Pin locale-sensitive formatting to English

**Files:**
- Modify: `web/src/i18n/datetime.ts`
- Modify: `web/src/lib/utils.ts` (add `formatNumber`)
- Modify the date consumers: `web/src/app/(main)/chat/page.tsx:995,1276`; `web/src/app/(main)/function/commands/page.tsx:22`; `.../findings/_components/findings-table.tsx:82`; `.../findings/detail/page.tsx:45`; `.../llm-records/page.tsx:41`; `.../tasks/detail/_tabs/broadcast-tab.tsx:160-161,248,403`; `.../findings-tab.tsx:114`; `.../overview-tab.tsx:907,921`; `.../traffic/page.tsx:52`; `.../workspace/page.tsx:38`; `.../notify/_components/delivery-list.tsx:187`; `.../skills/page.tsx:47`; `web/src/components/agent-editor.tsx:511,554`; `approval-records.tsx:45`; `finding-retest-panel.tsx:131`; `finding-traffic-panel.tsx:157`; `traffic-picker-dialog.tsx:221`; `transcript.tsx:139,143`; `side-question-workspace.tsx:89,124,125`
- Modify: `web/src/app/(main)/function/tasks/page.tsx:1616,1640,1958`; `web/src/app/(main)/function/tasks/detail/_tabs/sessions-tab.tsx:1699-1702,1889-1892`; `web/src/app/(main)/system/intercept/page.tsx:269`; `web/src/components/ui/calendar.tsx:43,202`; `web/src/components/ui/chart.tsx:258`; `web/src/components/traffic-evidence-viewer.tsx:95`
- Test: `web/src/i18n/datetime.test.ts`; `web/src/lib/utils.test.ts`

**Interfaces:**
- `getDateLocale(): string` returns `"en-US"` unconditionally.
- `formatNumber(value: number): string` returns `value.toLocaleString("en-US")`.
- Name sorting uses `localeCompare(other, "en")`; calendar formatting uses `"en-US"` when no locale is supplied.

- [ ] **Step 1: Write the failing tests**

`web/src/i18n/datetime.test.ts`:

```ts
import { getDateLocale } from "./datetime";

test("date formatting is pinned to English", () => {
  expect(getDateLocale()).toBe("en-US");
});
```

`web/src/lib/utils.test.ts`:

```ts
import { formatNumber } from "./utils";

test("numbers use English grouping", () => {
  expect(formatNumber(1234567)).toBe("1,234,567");
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest run src/i18n/datetime.test.ts src/lib/utils.test.ts`

Expected: FAIL — `getDateLocale` is locale-dependent and `formatNumber` does not exist.

- [ ] **Step 3: Implement**

- Replace `datetime.ts` with the constant `en-US` return.
- Add `formatNumber` to `web/src/lib/utils.ts`.
- Replace bare `toLocaleString()`/`toLocaleDateString()`/`toLocaleTimeString()`/`Intl.*`/`localeCompare` calls in the files above with the `en-US`/`en` form or the `formatNumber` helper.
- In `calendar.tsx`, default the locale to `"en-US"` when `locale?.code` is undefined (or pass `en-US` from `date-range-picker.tsx`).
- In `chat/page.tsx:1276`, change `"ko"` to `"en"`.

- [ ] **Step 4: Run the web tests and build**

Run: `cd web && npm test -- --run && npm run build:static`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "fix(web): pin locale-sensitive formatting to English"
```

---

### Task 6: Remove i18n tooling and translate remaining web CJK

**Files:**
- Delete: `web/src/scripts/extract-i18n.ts`; `web/src/scripts/i18n-keys.ts`; `web/src/scripts/check-i18n-keys.ts`; `web/src/scripts/i18n-keys.test.ts`
- Modify: `web/package.json` (remove `extract:i18n` and `check:i18n`)
- Modify: `web/src/lib/mock/data.ts`; `web/src/lib/mock/handler.ts`; `web/src/lib/agent-i18n.ts` (comments only); `web/src/i18n/config.ts` (comments); `web/src/scripts/extract-i18n.ts` (deleted)
- Test: `cd web && npm test -- --run`; `cd web && npm run build:static`; `python3 -I scripts/check-web-cjk.py`

**Interfaces:**
- No `extract-i18n` or `check-i18n-keys` scripts.
- `web/src/lib/agent-i18n.ts` keeps `useTranslations("builtinAgents")` and the `en.json` `builtinAgents` namespace.
- Mock data and non-functional fixtures are English; the `[模型]` sentinel, chat-mention tokens, and full-width comma remain.

- [ ] **Step 1: Delete the tooling and scripts**

Remove the four files and the two `package.json` scripts. Update any imports/tests that referenced them.

- [ ] **Step 2: Translate non-functional web CJK**

Use `git grep -lP '[\x{AC00}-\x{D7AF}\x{4E00}-\x{9FFF}\x{3040}-\x{30FF}]' -- web/src` and translate comments, mock data, test fixtures, and sample text. Keep the functional tokens listed in spec §7 and add a comment naming the allowlist entry at each kept site. Do not translate `web/src/lib/chat-mentions.ts`, `web/src/lib/company-scope.ts`, the full-width comma, or the `[模型]` sentinel.

- [ ] **Step 3: Run the web tests and static build**

Run: `cd web && npm test -- --run && npm run build:static`

Expected: PASS.

Run: `python3 -I scripts/check-web-cjk.py`

Expected: PASS; `web/out/*.html` contains no Han/Hangul/Kana.

- [ ] **Step 4: Commit**

```bash
git add web
git commit -m "chore(web): remove the five-locale tooling and translate mock data"
```

---

### Task 7: Translate and clean Go test fixtures

This task is split into three commits, one per package group, so each stays reviewable.

**Files (7a — server):** `server/*_test.go` with CJK; representative: `server/auth_setup_test.go`, `server/auth_harden_sync_test.go`, `server/core_test.go`, `server/finding_retests_test.go`, `server/findings_groups_test.go`, `server/intercept_judge_usage_test.go`, `server/mgmt_test.go`, `server/notify_api_test.go`, `server/task_metadata_test.go`, `server/task_templates_test.go`, `server/update_test.go`
**Files (7b — db/notify):** `db/*_test.go`, `notify/*_test.go`; representative: `db/activity_page_test.go`, `db/asset_intercept_match_test.go`, `db/assets_test.go`, `db/company_scope_consistency_test.go`, `db/config_test.go`, `db/exploration_test.go`, `db/finding_retests_test.go`, `db/intercept_seed_test.go`, `db/settings_insert_test.go`, `db/task_assets_test.go`, `notify/email_test.go`, `notify/filter_test.go`, `notify/http_test.go`, `notify/mask_test.go`, `notify/redact_test.go`, `notify/sign_test.go`, `notify/ssrf_test.go`, `notify/webhook_template_test.go`
**Files (7c — agent/intercept/config):** `agent/*_test.go`, `intercept/*_test.go`, `config/*_test.go`; representative: `agent/review_context_test.go`, `agent/prompt_now_test.go`, `agent/terminalreason_test.go`, `agent/provider_quota_test.go`, `agent/blackboard_inheritance_test.go`, `intercept/trace_test.go`, `intercept/review_context_test.go`

**Interfaces:** none (test-only).

**Rules:**
- Translate table names, `t.Fatalf` diagnostics, task goals, mock findings, prompts, and sample data to English.
- Keep the §7 functional fixtures and add a comment naming the allowlist entry.
- For multibyte/truncation/token tests, replace CJK with another multibyte script or update expectations; do not silently reduce coverage.
- Replace non-ASCII skill-name fixtures with non-CJK Unicode (e.g. `café-skill`, `π-scan`).

- [ ] **Step 1: Enumerate the CJK test files**

Run: `git grep -lP '[\x{AC00}-\x{D7AF}\x{4E00}-\x{9FFF}\x{3040}-\x{30FF}]' -- '*_test.go'`

Expected: the package groups above.

- [ ] **Step 2: Translate 7a (server) and run the package**

Run: `go test ./server -count=1`

Expected: PASS (DB tests may skip without PostgreSQL).

- [ ] **Step 3: Commit 7a**

```bash
git add server
git commit -m "test(server): translate non-functional fixtures to English"
```

- [ ] **Step 4: Translate 7b (db/notify) and run the packages**

Run: `go test ./db ./notify -count=1`

Expected: PASS.

- [ ] **Step 5: Commit 7b**

```bash
git add db notify
git commit -m "test(db,notify): translate non-functional fixtures to English"
```

- [ ] **Step 6: Translate 7c (agent/intercept/config) and run the packages**

Run: `go test ./agent ./intercept ./config -count=1`

Expected: PASS.

- [ ] **Step 7: Commit 7c**

```bash
git add agent intercept config
git commit -m "test(agent,intercept,config): translate non-functional fixtures to English"
```

---

### Task 8: Migrate unique content out of translated docs

**Files:**
- Modify: `README.md`; `docs/defense-en.md`; `docs/finding-traffic-evidence-en.md`; `MAINTAINING.md`; `CONTRIBUTING.md`
- Read/source: `README.zh.md`; `docs/defense-ko.md`; `docs/finding-traffic-evidence-zh.md`; `sidequestion/*.{ko,zh}.md`

**Interfaces:** none.

- [ ] **Step 1: Locate unique operational content**

Run: `git grep -nE 'Nginx|reverse proxy|反向代理|开发|Development|install|설치' -- README.zh.md docs/defense-ko.md docs/finding-traffic-evidence-zh.md`

Expected: a list of sections not present in the English docs. At minimum, the Nginx reverse-proxy example referenced by `README.md:180` and any development/verification content referenced by `README.md:385`.

- [ ] **Step 2: Move the content into English docs**

Add the Nginx example to `README.md` (or a dedicated English doc) and any unique development/installation content to the appropriate English document. Keep commands, code blocks, URLs, and identifiers byte-for-byte. Translate prose.

- [ ] **Step 3: Verify the English docs are self-contained**

Run: `python3 -I scripts/check-doc-links.py`

Expected: PASS for the current tree (the translated files still exist at this point).

- [ ] **Step 4: Commit**

```bash
git add README.md docs MAINTAINING.md CONTRIBUTING.md
git commit -m "docs: migrate unique operational content to English docs"
```

---

### Task 9: Delete translated docs, templates, and screenshots; fix references

**Files:**
- Delete: `README.ko.md`, `README.zh.md`, `README.es.md`, `README.ja.md`; `CHANGELOG.ko.md`, `CHANGELOG.zh.md`; `CONTRIBUTING.ko.md`, `MAINTAINING.ko.md`, `SECURITY.ko.md`, `CODE_OF_CONDUCT.ko.md`; `detections/README.ko.md` and `detections/{attack,indicators,sigma,suricata,tests,triage}/README.ko.md`; `docs/defense-ko.md`, `docs/finding-traffic-evidence-ko.md`, `docs/finding-traffic-evidence-zh.md`; `sidequestion/{CONTEXT_BUDGET,README,VALIDATION}.{ko,zh}.md`; `sidequestion/validation-2026-09-10.json`
- Delete: `.github/ISSUE_TEMPLATE/{bug_report_ko.yml,feature_request_ko.yml,translation_issue_en.yml,translation_issue_ko.yml}`
- Delete: the 13 root `screenshots/*.png` and `screenshots/wx.png`
- Modify: `screenshots/README.md`; every file found by the reference grep below

**Interfaces:** none.

- [ ] **Step 1: Find every reference to a file being deleted**

Run: `git grep -nE 'README\.(ko|zh|es|ja)|CHANGELOG\.(ko|zh)|CONTRIBUTING\.ko|MAINTAINING\.ko|SECURITY\.ko|CODE_OF_CONDUCT\.ko|defense-ko|finding-traffic-evidence-(ko|zh)|README\.ko|\.ko\.md|\.zh\.md|translation_issue|screenshots/(dashboard|tasks|sessions|graph|findings|assets|assets_test|traffic|chat|agents|llm|intercept|logs|wx)' -- . ':!graphify-out' ':!docs/superpowers'`

Expected: the reference list to fix.

- [ ] **Step 2: Delete the files**

Use `git rm` for each path above.

- [ ] **Step 3: Fix references**

Remove language navigation and translation banners from `README.md`, `CONTRIBUTING.md`, `MAINTAINING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `CHANGELOG.md`, the six `detections/*/README.md` files, `sidequestion/*.md`, `docs/defense-en.md`, `docs/finding-traffic-evidence-en.md`, and `screenshots/README.md`. Replace `defense-ko.md` links with `defense-en.md`; replace `README.zh.md` links with the English equivalent or remove them; remove the Chinese screenshot references.

- [ ] **Step 4: Run the doc link checker**

Run: `python3 -I scripts/check-doc-links.py`

Expected: PASS. If it reports a broken link, fix the reference (do not re-add the deleted file).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "docs: remove translated documents, templates, and Chinese screenshots"
```

---

### Task 10: Rewrite core English docs

**Files:**
- Modify: `README.md`; `CONTRIBUTING.md`; `MAINTAINING.md`; `CHANGELOG.md`; `SECURITY.md`; `CODE_OF_CONDUCT.md`

**Interfaces:** none.

- [ ] **Step 1: Rewrite `README.md`**

Remove the five-language navigation and “switchable in Settings” claims; remove the multilingual-edition rationale, the `README.zh.md` notes, the Chinese-UI screenshot note, and the `docs/defense-ko.md` link. Replace Korean statute parentheticals with the official English names. Keep the authorized-use warning, English screenshots, installation, configuration, architecture, and legal sections. Remove `ARTEX_LANG`/language-setting documentation.

- [ ] **Step 2: Rewrite `CONTRIBUTING.md`**

Replace the Localization policy with the English-only policy and the spec §7 allowlist. Remove the translation-issue link, the `i18n/` branch prefix, and the Korean commit-message rule (commit messages are English). Update the verification instructions to run the tightened language guard. Update the design-doc reference that points at the Chinese original.

- [ ] **Step 3: Rewrite `MAINTAINING.md`**

Rewrite as “Maintaining the English-only edition”. Keep the upstream base note and the detection-indicator check. Remove the translation-symmetry/drift sections and the localized-asset guidance. Document that upstream is unreachable and that future upstream Chinese prompt text is translated to English during sync.

- [ ] **Step 4: Rewrite `CHANGELOG.md`**

Remove the language navigation and `CHANGELOG.zh.md` links; keep historical entries. Convert every historical link to a deleted file (`README.ko.md`, `README.zh.md`, `docs/defense-ko.md`, etc.) into plain text so the prose survives but `check-doc-links.py` stays green. Fix the broken `README.en.md` reference; add an `[Unreleased]` entry describing the English-only migration (deleted catalogs/docs, single English catalog, fixed English directive, guard tightening).

- [ ] **Step 5: Update `SECURITY.md` and `CODE_OF_CONDUCT.md`**

Remove the Korean navigation line only; keep the English content.

- [ ] **Step 6: Run the doc link checker**

Run: `python3 -I scripts/check-doc-links.py`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add README.md CONTRIBUTING.md MAINTAINING.md CHANGELOG.md SECURITY.md CODE_OF_CONDUCT.md
git commit -m "docs: rewrite the core documentation for English-only"
```

---

### Task 11: Rewrite detection, side-question, and PR docs

**Files:**
- Modify: `detections/README.md`; `detections/{attack,indicators,sigma,suricata,tests,triage}/README.md`
- Modify: `docs/defense-en.md`; `docs/finding-traffic-evidence-en.md`
- Modify: `sidequestion/README.md`; `sidequestion/CONTEXT_BUDGET.md`; `sidequestion/VALIDATION.md` (or delete it)
- Modify: `.github/PULL_REQUEST_TEMPLATE.md`; `.github/ISSUE_TEMPLATE/config.yml` (verify)

**Interfaces:** none.

- [ ] **Step 1: Update the detection READMEs**

Remove the Korean navigation, the “Korean translation” banner, and `defense-ko` links from the root and six subdirectory READMEs. Replace cross-references to deleted Korean READMEs with the English equivalents.

- [ ] **Step 2: Update the English guides**

In `docs/defense-en.md`, remove the `defense-ko` links and Korean-language references; keep the English guide and the Korea-specific English section. In `docs/finding-traffic-evidence-en.md`, remove the zh/ko links and the “original preserved” banner.

- [ ] **Step 3: Update the side-question docs**

Remove Korean/Chinese navigation and translation banners. Rewrite `VALIDATION.md` as English-only evidence, or delete it if it only validated the Chinese original. Delete the Chinese validation JSON if it is not already gone (Task 9).

- [ ] **Step 4: Update the PR template**

Remove the “Localization / translation (`i18n`)” change type and the five-file localization checklist. Add a verification item: `python3 -I scripts/check-source-language.py` passes. Keep the authorized-use confirmation.

- [ ] **Step 5: Run the doc link checker**

Run: `python3 -I scripts/check-doc-links.py`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add detections docs sidequestion .github
git commit -m "docs: remove detection and side-question localization references"
```

---

### Task 12: Update the detection harness and rule links

**Files:**
- Modify: `detections/tests/check-defense-template.py`; `detections/tests/check-defense-template.sh`
- Modify: `.github/workflows/detections.yml:45-63`
- Modify: `detections/sigma/*.yml`; `detections/sigma/correlation/*.yml`; `detections/suricata/artex.rules`; `detections/sigma_lint/validators.yml`
- Modify: `.pre-commit-config.yaml:40`
- Test: `detections/tests/run-all.sh`

**Interfaces:**
- `check-defense-template.py` reads only `docs/defense-en.md`; no `KO` path, `ko_block`, or ko/en parity assertion.
- The sigma check and conversion-matrix assertions remain.

- [ ] **Step 1: Simplify `check-defense-template.py`**

Remove the `KO` path, the ko/en parity section, and the “both languages parse” loop. Extract the YAML block from `docs/defense-en.md` only. Keep the `sigma check` and splunk/eql/loki conversion assertions, and the lucene/kusto negative assertion.

- [ ] **Step 2: Update the shell wrapper and workflow**

Remove the `defense-ko.md` mount/argument from `check-defense-template.sh`. Remove `docs/defense-ko.md` from both `paths` lists in `.github/workflows/detections.yml`; keep `docs/defense-en.md`. Update the comments that say “ko/en rules”.

- [ ] **Step 3: Update links**

Change every `docs/defense-ko.md` reference in `detections/sigma/**`, `detections/suricata/artex.rules`, `detections/sigma_lint/validators.yml`, and `.pre-commit-config.yaml` to `docs/defense-en.md`. Keep the legacy Chinese guard marker and its fixture untouched.

- [ ] **Step 4: Run the detection suite**

Run: `detections/tests/run-all.sh`

Expected: PASS, including the defense-template gate. Docker must be available.

- [ ] **Step 5: Commit**

```bash
git add detections .github/workflows/detections.yml .pre-commit-config.yaml
git commit -m "test(detections): validate the English defense guide only"
```

---

### Task 13: Tighten guards, CI, hooks, and external links

**Files:**
- Modify: `scripts/check-source-language.py`; `scripts/check-web-cjk.py` (docstring only); `scripts/check-external-links.py:192`; `scripts/external-links-allowlist.txt`
- Modify: `.github/workflows/ci.yml`; `.github/workflows/web.yml`; `.github/workflows/docs.yml`; `.github/workflows/external-links.yml`; `.pre-commit-config.yaml`
- Test: `python3 -I scripts/check-source-language.py`; `python3 -I scripts/check-web-cjk.py`; `python3 -I scripts/check-doc-links.py`

**Interfaces:**
- `check-source-language.py` scans tests, skips `graphify-out/` and `docs/superpowers/`, and permits only the spec §7 allowlist.
- `check-web-cjk.py` keeps `ALLOWED_CJK` empty.
- `check-external-links.py` sends `Accept-Language: en`.

- [ ] **Step 1: Tighten `check-source-language.py`**

Remove `is_localized_asset()` and the localized-catalog/label entries from `ALLOWED_FILES`; stop skipping `_test.go`/`.test.*`/`.spec.*`; keep the §7 line patterns; remove the Korean-law-name and native-language-name patterns; add `graphify-out/` and `docs/superpowers/` to the skip list with a comment. Update the docstring and success message.

- [ ] **Step 2: Run the source guard**

Run: `python3 -I scripts/check-source-language.py`

Expected: PASS. If it reports a remaining non-functional CJK line, translate it (or add it to the §7 allowlist only if it is genuinely functional).

- [ ] **Step 3: Update the web guard and workflows**

Update `check-web-cjk.py`'s docstring to remove the language-switcher example. In `.github/workflows/web.yml`, remove the `check:i18n` step and update the unit-test comment. In `ci.yml`, update the “Korean-localization regressions” comment. Keep the docs/external-links checks.

- [ ] **Step 4: Update external links**

Change `Accept-Language: en,ko;q=0.8` to `Accept-Language: en`. Remove the allowlist entries/examples for `README.zh.md`/`CHANGELOG.zh.md`; keep the upstream-contributor link only if an English document still references it.

- [ ] **Step 5: Run the guards and link checks**

Run: `cd web && npm run build:static && python3 -I ../scripts/check-web-cjk.py`

Expected: PASS.

Run: `python3 -I scripts/check-source-language.py && python3 -I scripts/check-doc-links.py`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts .github .pre-commit-config.yaml
git commit -m "ci: enforce English-only in source, docs, and static export"
```

---

### Task 14: Final verification, graph rebuild, and grep gates

**Files:** none (verification only); `graphify-out/` is regenerated.

- [ ] **Step 1: Run the Go checks**

Run: `go build ./... && go vet ./... && go test ./... -count=1`

Expected: PASS (DB integration packages require PostgreSQL; run the CI matrix for those).

- [ ] **Step 2: Run the web checks**

Run: `cd web && npm ci && npm run check && npm test -- --run && npm run build:static`

Expected: PASS.

- [ ] **Step 3: Run the Python and detection gates**

Run: `python3 -I scripts/check-source-language.py`

Run: `python3 -I scripts/check-web-cjk.py`

Run: `python3 -I scripts/check-doc-links.py`

Run: `detections/tests/run-all.sh`

Expected: PASS.

- [ ] **Step 4: Rebuild the knowledge graph**

Run: `graphify update .`

Expected: the graph rebuild completes. A large `graphify-out/graph.json` diff is expected.

- [ ] **Step 5: Run the grep gates**

Run each and review every hit against spec §7:

```bash
git grep -nE 'README\.(ko|zh|es|ja)|CHANGELOG\.(ko|zh)|CONTRIBUTING\.ko|MAINTAINING\.ko|SECURITY\.ko|CODE_OF_CONDUCT\.ko|defense-ko|finding-traffic-evidence-(ko|zh)|web/messages/(ko|zh|es|ja)|LOCALE_LABELS|useActiveLocale|setActiveLocale|output_language|ARTEX_LANG' -- . ':!graphify-out' ':!docs/superpowers' ':!CHANGELOG.md'
git grep -nE 'langDirective\(|SupportedLanguages|NormalizeLanguage|ActiveLanguage|SupportedOutputLangs|labelLang|judgeActionLang' -- . ':!graphify-out' ':!docs/superpowers' ':!CHANGELOG.md'
git grep -nE 'localeCompare\([^,]+, "(ko|zh|ja|es)"|toLocale(String|DateString|TimeString)\(\s*\)' -- web/src
git grep -nP '[\x{AC00}-\x{D7AF}\x{4E00}-\x{9FFF}\x{3040}-\x{30FF}]' -- . ':!graphify-out' ':!docs/superpowers'
```

Expected: only the spec §7 allowlist hits remain; the `localeCompare`/bare-`toLocale*` gate has no production hits. `CHANGELOG.md` is excluded from the first two gates because its historical entries intentionally mention the old filenames/identifiers as plain text; its links are still checked by `check-doc-links.py`.

- [ ] **Step 6: Manual smoke test**

- Settings has no display-language control; the UI renders English; dates and numbers use `en-US`.
- An agent run against a Chinese-language target produces English findings, summaries, reports, and chat replies.
- A side-question answer is English when the question/context is Chinese.
- An intercept judge comment is English.
- A historical `[模型]` row, an old chat mention, and an old verdict still parse/render.
- `GET /api/settings` has no `output_language`; `PUT` with it is not accepted.
- A legacy Sigma rule matches the Chinese guard marker fixture.
- `web/out/*.html` contains no Han/Hangul/Kana.

- [ ] **Step 7: Commit the graph rebuild**

```bash
git add graphify-out
git commit -m "chore(graph): rebuild the knowledge graph for the English-only tree"
```

---

## Self-Review

- **Spec coverage:** Every section of the spec maps to a task: §6.1 → Tasks 4-6; §6.2 → Task 1; §6.3 → Tasks 2-3; §7 → Tasks 2, 7, 12, 13; §8.1 → Tasks 1-3; §8.2 → Tasks 4-6; §8.3 → Tasks 3, 7; §8.4 → Tasks 8-11; §8.5 → Tasks 12-13; §8.6 → Task 3; §8.7 → Task 9; §10 → Task 14.
- **Step scan:** Each step names a file, command, or exact assertion. Mechanical call-site work is expressed as a grep-and-replace rule, not a transcript.
- **Type consistency:** `textpolicy.EnglishOutputDirective`, `textpolicy.EnglishJudgeCommentRule`, `outputDirective()`, `LOCALE`, `getDateLocale()`, and `formatNumber()` are defined once and reused.
- **Review Focus:** Each of the five failure modes has a named test in Tasks 1, 2, 3, 5, and 12.
- **Proportion:** The plan is longer than the spec only in task mechanics; code bodies are limited to failing tests and interface signatures.
