# English-only ARTEX global edition — design

- **Date:** 2026-10-25
- **Status:** draft for owner approval
- **Scope:** the whole repository and the shipped product
- **Decision owner:** repository maintainer
- **Author:** design session (Claude Code + maintainer)

---

## 1. Summary

The global edition becomes **English-only**: one language in the product, the source tree,
the documentation, and the shipped assets. This is not a new localization direction — it
removes the five-language output-localization layer that the global edition currently
carries and keeps English as the sole language.

The target state is:

- `web/messages/en.json` is the only UI message catalog. The Korean, Chinese, Spanish, and
  Japanese catalogs, the language switcher, and the client-side locale store are deleted.
- The Go server has no output-language setting, catalog, resolver, or API field. Backend
  messages are English literals.
- Agent prompts keep a fixed **English output directive** so findings, summaries, reports,
  and chat replies are written in English even when the target or source material is in
  another language.
- All translated Markdown, non-English issue templates, and Chinese-UI screenshots are
  removed. English documents that described the multilingual layer are rewritten.
- A small, documented allowlist of non-English **machine values** remains because removing
  it would break persisted data, old conversations, provider error detection, DB upgrades,
  detection rules, or the guards themselves. The allowlist is listed in §7.

The migration is a one-time refactor with a large but mostly mechanical diff. The product
behavior that matters — autonomous pentesting, detection coverage, data compatibility — does
not change.

---

## 2. Why

The maintainer's reason for the change is **future maintenance and development cost**.

Today the multilingual layer creates recurring work:

- Five UI catalogs must stay key-for-key in sync (2,996 leaves each). Every new UI string is
  five edits, and every catalog change needs the five-locale parity gate.
- The source tree carries a 209-entry Go error catalog, 33 config/log templates, per-language
  label tables in `notify` and `intercept`, and an output-language directive parameterized by
  language.
- 26 translated Markdown documents must be kept in step with their English counterparts;
  six detection READMEs and several English docs carry language navigation and translation
  banners that point at them.
- 51 `*_localized_test.go` files and a large number of mixed tests pin Korean/Chinese output
  and must be maintained whenever the wording changes.
- Two language gates (`check-source-language.py`, `check-web-cjk.py`) and a five-locale
  parity gate are part of the merge gate.

Collapsing all of that to one language removes the translation-drift class of bug entirely
and makes every future change a one-language change.

---

## 3. Goals, non-goals, anti-goals

### Goals

1. English is the only language the product shows and the only language the repository ships
   in prose, comments, scripts, CI, tests, and documentation.
2. One English UI catalog remains; there is no runtime language selection anywhere (web,
   server, config, environment, API).
3. Agent-generated user-facing text is forced to English by a code-owned directive that a
   DB-edited prompt cannot remove.
4. Functional behavior is preserved: persisted wire values, DB upgrades, provider quota
   detection, detection rules, and Unicode handling of non-English *target data* continue to
   work.
5. The English-only invariant is enforced by merge gates, so translation debt cannot return.
6. Operational content that exists only in a translated document is migrated to English
   before the translated document is deleted.

### Non-goals

- Translating historical user data — there is no production installation or accumulated run
  data yet; any developer database can be recreated. No data-content migration is required.
- Rewriting git history or changing the agent brain's reasoning behavior.
- Removing Unicode capabilities needed to handle non-English target data: IME composition,
  CJK full-text search, GBK zip-filename decoding, RFC 2047 email encoding, UTF-8 BOM, and
  non-ASCII skill identifiers.
- Changing detection semantics or the authorized-use/legal posture.

### Anti-goals (what would make this a failure even if it “works”)

- A half-migrated app that still offers a language setting, still reads `output_language`, or
  still accepts `ARTEX_LANG`.
- Deleting a functional Chinese/Korean token and breaking old conversations, DB upgrades,
  quota detection, or legacy-log detection.
- Deleting a translated document without first moving its unique operational content (for
  example the Nginx reverse-proxy example in `README.zh.md`).
- Leaving dead links to deleted files, or leaving the language guard exempting tests and
  localized assets so the old content can silently return.
- Replacing CJK test fixtures with English in tests whose actual subject is multibyte
  truncation, CJK full-text search, or non-ASCII identifiers, thereby losing the behavior
  under test.

---

## 4. Decisions

### 4.1 Owner decisions (already made)

1. The repository and product become English-only.
2. Translated READMEs and translated design documents are removed, not kept.
3. Keep `web/messages/en.json` as the single English catalog. Do not inline all 2,996
   strings into components.
4. Keep the documented functional allowlist in §7 rather than migrating wire formats and
   persisted data to English.

### 4.2 Builder calls (recorded; override only with a reason)

1. **Keep next-intl.** Keep `en.json`, the `useTranslations` call sites, and the existing
   provider. Hardcode the locale to `en`; delete every other catalog and all locale
   switching. Inlining 2,996 strings is out of scope.
2. **Replace `langDirective(code)` with a fixed English directive.** Append it to the same
   six user-facing agent roles and always to the wrap-up prompt. The directive keeps the
   “commands, payloads, code, URLs, and raw excerpts stay verbatim” and “do not mirror the
   target/material language” clauses. It also applies to the two user-visible LLM surfaces
   outside the agent roles: the side-question answer (`sidequestion/request.go:33`) and the
   intercept judge's `comment` (`intercept/prompt.go:36,124-127`). Use one shared exported
   constant, or per-package copies plus a test that asserts they are byte-identical, so the
   wording cannot drift. This is an English output invariant, not localization machinery.
3. **Remove `output_language` from the public API.** `GET /api/settings` no longer returns
   it and `PUT /api/settings` no longer accepts it. No “accepted but ignored” compatibility
   field. The bundled web client is the only first-party client; external clients must
   adapt.
4. **Remove `ARTEX_LANG` and the config-file `language` field.** They are simply unread; do
   not fail startup when present. Remove them from `docker-compose.yml`, `.env.example`, and
   documentation.
5. **Remove the non-English `intent`/`hint` input aliases** (`意图`/`提示`/`의도`/`힌트`).
   Keep the English `intent`/`hint` triggers. Old transcripts are not re-parsed, so this does
   not corrupt stored data.
6. **Keep the full-width comma `，` as an input separator.** It is paste tolerance, not
   display copy.
7. **Keep both language guards, tightened.** `check-source-language.py` becomes a
   no-other-language gate that also scans tests and no longer exempts localized assets.
   `check-web-cjk.py` stays as the static-export gate. The functional allowlist in §7 is the
   only content exception. Two non-product artifacts are explicitly skipped: generated
   `graphify-out/` (rebuilt output) and process records under `docs/superpowers/` (change
   designs/plans, which necessarily quote tokens under removal). Do not add any other
   directory-level exemption.
8. **Keep historical `CHANGELOG.md` entries** that describe the Korean/multilingual era.
   They are English text and are history. Delete `CHANGELOG.ko.md` and `CHANGELOG.zh.md`,
   remove the language navigation, and fix links that pointed at the deleted files.
9. **Delete the translated screenshots.** Keep `screenshots/en/`; delete the 13 Chinese-UI
   `screenshots/*.png` and `screenshots/wx.png`. Rewrite `screenshots/README.md`.
10. **Delete Korean/Chinese issue templates and both translation-issue templates.** Keep
    `bug_report_en.yml` and `feature_request_en.yml`.
11. **Use official English names only for Korean statutes.** Remove the Korean parentheticals
    in English prose. The English official names are sufficient for the legal notice.
12. **There is no historical DB content to preserve.** The project has not been deployed and
    has no accumulated runs; a developer database can be recreated. Only the obsolete
    `output_language` settings key is cleaned up (or simply ignored on a fresh database). No
    content migration or translation pass is needed.
13. **Delete `sidequestion/validation-2026-09-10.json` and the Korean/Chinese sidequestion
    documents.** Rewrite the English `sidequestion/VALIDATION.md` as English-only evidence,
    or delete it if it has no purpose after the Chinese original is gone.
14. **Rewrite the upstream-sync runbook for English-only.** Upstream `Autumn-27/ARTEX` is
    unreachable (404); the historical base commit is preserved in this repository's history
    and on `backup/pre-squash-2026-10-24`. The runbook should say there is nothing to sync
    until a mirror appears, and that any future upstream Chinese prompt text is translated
    to English during the sync.
15. **Commit messages become English** from this change forward. The existing history is not
    rewritten.

---

## 5. Current-state inventory

The inventory was produced by an eight-agent sweep of the repository (seven dimensions plus
an independent completeness critic). Counts below are from that sweep and were spot-checked
against the source.

### 5.1 Go backend

| Surface | Where | Detail |
|---|---|---|
| Language config API | `config/config.go:28-95` | `Config.Language`, `SupportedLanguages`, `DefaultLanguage`, `NormalizeLanguage`, `CurrentLanguage`, `ActiveLanguage` (hardcoded `ko` fallback), `Language()` (`ARTEX_LANG` > file > `en`) |
| Config/log catalog | `config/msg_i18n.go` | 4 config messages + 33 log templates in ko/zh/es/ja; `trCfg`, `T` |
| Server error catalog | `server/errmsg_catalog.go` | 209 English-keyed entries, each with ko/zh/es/ja values; generated, no generator in-repo |
| Server localization runtime | `server/errmsg_i18n.go` | `trMsg`, `logT`, `loadOutputLanguage`, `outputLanguage`; 63 non-test `trMsg` calls, 29–30 `logT` calls, 3 `config.T` calls |
| Server language state | `server/server.go:203-206,308-321` | atomic `outputLang` cache; wires `agent.OutputLanguage`, `config.CurrentLanguage`, `notify.Lang` |
| Settings API | `server/server.go:75-76,3402-3405,3506-3508,3533-3545` | `errOutputLanguageInvalid`, GET/PUT `output_language`, validation and cache update |
| Settings key | `server/manager.go:279-282` | `settingOutputLanguage = "output_language"` |
| Agent output language | `agent/outputlang.go` | `OutputLanguage`, `defaultOutputLang`, `SupportedOutputLangs`, `normalizeLang`, `resolveOutputLang`, native-name maps |
| Agent directive | `agent/prompt.go:57-85` | `langDirective(code)` parameterized by five languages |
| Directive call sites | `agent/chat.go:87`, `goals.go:82`, `mainagent.go:105`, `planner.go:344`, `worker.go:296`, `wrapup.go:107-126` | six roles; wrap-up omits the directive for English |
| Notification labels | `notify/notify.go:54-98` | `Lang`, `labelLang` (Korean fallback), five-language severity/status maps |
| Intercept labels | `intercept/label_i18n.go` | five-language allow/deny/ask labels; Korean fallback |
| Input aliases | `server/server.go:3947-3958` | Chinese/Korean `意图`/`提示`/`의도`/`힌트` alongside English |
| Env/config plumbing | `config.example.json:20-21`, `.env.example:30-35`, `docker-compose.yml:42-45` | `language`, `ARTEX_LANG` |

### 5.2 Next.js frontend

| Surface | Where | Detail |
|---|---|---|
| Message catalogs | `web/messages/{en,ko,zh,es,ja}.json` | 2,996 leaves each; `en.json` is CJK-free |
| Catalog consumers | 76 files, 188 `useTranslations` sites | mostly large components and page routes |
| Locale config | `web/src/i18n/config.ts` | `LOCALES`, `Locale`, `DEFAULT_LOCALE`, `LOCALE_LABELS`, cookie/storage keys, `isLocale`, `resolveLocale` (`NEXT_PUBLIC_LOCALE`) |
| Locale store | `web/src/i18n/locale-store.ts` | localStorage + cookie, `useActiveLocale`, `setActiveLocale`, cross-tab subscription |
| Request config | `web/src/i18n/request.ts` | five locale loaders |
| Runtime provider | `web/src/i18n/intl-runtime-provider.tsx` | bundles all five catalogs, follows `useActiveLocale`, sets `<html lang>` |
| Date locale | `web/src/i18n/datetime.ts` | maps five locales; 18 consumer files; 19 date sites, some `toLocaleString()` with no locale. Extra locale-sensitive sites: `chat/page.tsx:1276` (`localeCompare(..., "ko")`), `function/tasks/page.tsx:1616` (bare `toLocaleString()`), `ui/calendar.tsx:43,202` (`locale?.code`), and bare number formatting in `sessions-tab.tsx`, `chart.tsx`, `intercept/page.tsx`, `tasks/page.tsx`, and `traffic-evidence-viewer.tsx` |
| Metadata | `web/src/config/app-config.ts` | five-language title/description |
| Root layout | `web/src/app/layout.tsx` | `lang={locale}`, wraps `IntlRuntimeProvider` |
| Language selector | `web/src/app/(main)/system/settings/page.tsx:23-24,67,122-128,672` | native labels; writes both localStorage and server `output_language` |
| API type | `web/src/lib/types.ts:960` | `output_language?: string` |
| Catalog tooling | `web/src/scripts/{extract-i18n.ts,i18n-keys.ts,check-i18n-keys.ts}`, `web/src/scripts/i18n-keys.test.ts`, `web/src/i18n/config.test.ts` | five-locale parity and Chinese-string extraction |
| Built-in agent labels | `web/src/lib/agent-i18n.ts` | `builtinAgents` namespace; masks DB-seeded built-in names |
| Mock/demo data | `web/src/lib/mock/data.ts`, `handler.ts` | some Chinese fixtures and the `[模型]` sentinel |

### 5.3 Tests and fixtures

- **112 tracked test files contain CJK.**
- **51 `*_localized_test.go` files** plus localized tests in other packages pin Korean/
  Chinese output. These are deleted or rewritten.
- **61 non-localized test files** contain CJK fixtures, table names, or failure diagnostics.
  These are translated, except for the functional fixtures in §7.
- `server/testmain_test.go:33` and several tests set `ARTEX_LANG`; `config/config_localized_test.go`,
  `config/msg_i18n_test.go`, `server/logt_catalog_test.go`, `server/output_language_test.go`,
  and `server/server_localized_test.go` exist only for the localization layer.

### 5.4 Documentation and metadata

- **26 translated Markdown files:** `README.{ko,zh,es,ja}.md`, `CHANGELOG.{ko,zh}.md`,
  `CONTRIBUTING.ko.md`, `MAINTAINING.ko.md`, `SECURITY.ko.md`, `CODE_OF_CONDUCT.ko.md`,
  `detections/README.ko.md` plus six subdirectory `README.ko.md` files,
  `docs/defense-ko.md`, `docs/finding-traffic-evidence-{ko,zh}.md`, and six
  `sidequestion/*.{ko,zh}.md` files.
- **English documents with language navigation, translation banners, or links to those
  files:** `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `MAINTAINING.md`, `SECURITY.md`,
  `CODE_OF_CONDUCT.md`, the six `detections/*/README.md` files, `sidequestion/README.md`,
  `sidequestion/CONTEXT_BUDGET.md`, `sidequestion/VALIDATION.md`, `docs/defense-en.md`,
  `docs/finding-traffic-evidence-en.md`, `screenshots/README.md`.
- **Issue templates:** `bug_report_ko.yml`, `feature_request_ko.yml`,
  `translation_issue_en.yml`, `translation_issue_ko.yml`.
- **PR template:** a “Localization / translation (`i18n`)” change type and a five-file
  localization checklist.
- **Screenshots:** 13 Chinese-UI PNGs at the root plus `screenshots/wx.png`; English captures
  live under `screenshots/en/`.

### 5.5 Scripts, CI, hooks, detection harness

- `scripts/check-source-language.py`: scans source and developer docs, skips tests and JSON,
  exempts localized assets, and carries a line-level allowlist for functional tokens.
- `scripts/check-web-cjk.py`: scans `web/out/*.html` for Han/Hangul/Kana; `ALLOWED_CJK` is
  empty after the language switcher is removed.
- `scripts/check-external-links.py:192`: sends `Accept-Language: en,ko;q=0.8`.
- `scripts/external-links-allowlist.txt:12-25`: entries that exist only because
  `CHANGELOG.zh.md`/`README.zh.md` are preserved.
- `.github/workflows/ci.yml`: source-language check.
- `.github/workflows/web.yml`: five-locale parity (`npm run check:i18n`) and CJK guard.
- `.github/workflows/detections.yml:48,62`: path filters include `docs/defense-ko.md`.
- `.pre-commit-config.yaml:40`: detections hook watches `docs/defense-(ko|en).md`;
  `source-language` hook is always-on.
- `detections/tests/check-defense-template.py`: requires both defense guides and asserts the
  ko/en Sigma rule bodies are byte-identical.
- Sigma/Suricata/lint files link to `docs/defense-ko.md` in comments or `related` fields.

---

## 6. Target architecture

### 6.1 The single English catalog

```
web/messages/en.json                 # the only catalog (2,996 leaves)
web/src/i18n/config.ts               # LOCALE = "en"; no locale list, labels, or storage keys
web/src/i18n/request.ts              # loads en.json only
web/src/i18n/intl-runtime-provider.tsx  # NextIntlClientProvider locale="en", static messages
web/src/i18n/datetime.ts             # getDateLocale() returns "en-US"
web/src/config/app-config.ts         # English title/description only
web/src/app/layout.tsx               # lang="en", static metadata
```

- `useTranslations` and the `en.json` key structure stay as-is, so the 76 consumer files
  need no mechanical rewrite. `builtinAgents` stays in `en.json` so it continues to mask
  legacy DB-seeded built-in agent names.
- `web/src/i18n/locale-store.ts` and every import of `useActiveLocale`/`setActiveLocale` are
  deleted. The settings page loses its “Display language” section and no longer calls
  `setSettings({ output_language })`.
- `getDateLocale()` becomes a constant `"en-US"`. Every `toLocaleString()`/`Intl` site that
  relied on the active locale passes `"en-US"` explicitly, including the two
  `side-question-workspace` sites that currently pass no locale.
- `NEXT_PUBLIC_LOCALE`, `artex_locale` cookies/localStorage, and `LOCALE_LABELS` disappear.
  Stale browser values are harmless and are not actively cleared.
- The five-locale parity/extraction tooling (`extract-i18n.ts`, `i18n-keys.ts`,
  `check-i18n-keys.ts`, their tests, `package.json` `check:i18n`/`extract:i18n`, and the
  `web.yml` step) is removed. With one catalog there is no parity to check; the static-export
  CJK guard and the language guard cover the invariant. If a cheap single-catalog sanity
  test is useful, keep only a test that `en.json` parses and contains no CJK.

### 6.2 English output directive

`agent/outputlang.go` is deleted. `langDirective(code)` is replaced with a fixed
`englishOutputDirective()` returning the existing English-path text:

- every user-facing natural-language field is written in **English**;
- commands, payloads, code, paths, URLs, parameter names, and raw log/request/response
  excerpts stay verbatim;
- even when the target or reference material is in another language, display fields are not
  mirrored from it;
- visible replies are English from the first character, with no reasoning preamble.

The directive is appended after the rendered body/artifact/traffic tails for the six
user-facing roles and is **always** appended to the wrap-up prompt (this fixes the current
English-path omission at `agent/wrapup.go:121-125`). It remains code-owned so a DB-edited
prompt cannot remove it.

Two user-visible LLM surfaces sit outside the six roles: the side-question answer
(`sidequestion/request.go:33` `instruction`) and the intercept judge's `comment`
(`intercept/prompt.go:36,124-127`, persisted as `InitialReason` and shown in
`web/src/components/approval-records.tsx`). Both get the same English-only rule. The
side-question package does not import `agent`; the builder chooses one shared exported
constant, a tiny shared package, or per-package copies with an equality test. The intercept
judge keeps accepting the legacy Chinese verdict markers for compatibility (§7) while
requiring new comments to be English.

### 6.3 Backend English literals

- `config/msg_i18n.go` is deleted. The 4 `trCfg` and 3 `config.T` call sites use English
  literals.
- `server/errmsg_catalog.go` and `server/errmsg_i18n.go` are deleted. The 63 `trMsg` call
  sites pass their English key directly to `writeErr`/`writeJSON`; the 29–30 `logT` sites
  pass the English template directly to `log.Printf`.
- `server/server.go` loses `outputLang`, the resolver closure, the `agent.OutputLanguage`/
  `config.CurrentLanguage`/`notify.Lang` wiring, `settingOutputLanguage`, the settings API
  field, and `errOutputLanguageInvalid`.
- `notify` and `intercept` label tables collapse to English. The `[模型]` marker and the
  English enum keys remain.
- `config/config.go` loses the language API. `config.example.json`, `.env.example`, and
  `docker-compose.yml` lose the language field/env var.

---

## 7. Functional allowlist (must stay non-English)

These are data, wire values, upstream API text, detection patterns, or encoding support —
not translation. Each site gets a short comment explaining why it cannot be translated, and
the source-language guard permits exactly these.

| # | Token / mechanism | Paths | Why it stays |
|---|---|---|---|
| 1 | `[模型]` | `intercept/intercept.go:30-31,500-502`; `db/intercept.go:40,231,251`; `db/task_archives_restore.go:680`; `db/schema.sql:1096`; web approval/mock/types | Persisted model-vs-rule decision sentinel; matched by SQL `LIKE` and `strings.HasPrefix`; changing it breaks historical rows and `decision_source` backfills |
| 2 | Chinese chat-mention labels (`漏洞/资产/企业/接口/应用/IP/域名/子域名/服务`) | `server/chat_mentions.go:45-49`; `web/src/lib/chat-mentions.ts:2-10,46,67`; tests | Persisted `@[kind#id label]` conversation wire format, parsed by server and UI |
| 3 | Legacy verdict markers (`实际操作：`, `；成功后的后果：`, `；命中规则：`) | `intercept/prompt.go:203-210`; intercept tests | Accepted for verdicts produced by older prompts or already persisted |
| 4 | Provider quota strings (`余额不足`, `额度不足`, `额度已用尽`, `欠费`) | `agent/provider.go:309-315`; provider/quota tests | Literal upstream LLM-provider API error text; translating breaks quota detection/failover |
| 5 | Chinese log-classification keywords (`失败/丢弃/拒绝/不可达/禁用/重试`) | `server/logsink.go:217,222` | Classifies log lines emitted by upstream tools; not display text |
| 6 | `备案` | `db/company_scope.go:162`; `web/src/lib/company-scope.ts:208`; tests | Functional ICP-filing detection in target/company data |
| 7 | Legacy `traffic_search` description | `server/finding_workflow.go:37` | Byte-exact `WHERE description=$2` migration sentinel; changing it silently breaks upgrades |
| 8 | Sigma legacy guard marker (`【ARTEX 平台管控·非目标防御】`) | `detections/sigma/artex_guard_audit_framing.yml:26-27`; `detections/tests/sigma_match/events/artex_guard_audit_framing.json:8` | Lets defenders match audit logs from older ARTEX builds |
| 9 | GBK zip-filename decoding | `server/skill_zip.go:14,111-123`; `golang.org/x/text` in `go.mod:16`; skill-upload tests | Encoding support for Windows-written Korean/Chinese archive entry names |
| 10 | Chinese auth/login regex | `skills/api-recon/scripts/preload.js:192`; `skills/api-recon/reference.md:52` | Detection recipe for Chinese login/authorization pages on scan targets |
| 11 | CJK detector ranges and functional patterns | `scripts/check-source-language.py:33,82-102`; `scripts/check-web-cjk.py:62-72` | The guards cannot work without these ranges; the allowlist patterns are the functional tokens above |
| 12 | Full-width comma `，` | `web/src/app/(main)/system/notify/page.tsx:149`; `.../_components/channel-fields.ts:171-185`; tests | Input separator for pasted ID lists; paste tolerance, not display copy |

**Not in the allowlist** (remove or translate): the non-English `intent`/`hint` aliases, the
native language names in the switcher, the Korean statute parentheticals, the Chinese
`README.zh.md` anchors, the quoted Chinese page title in `sidequestion/VALIDATION.md`, and
every other Chinese/Korean/Japanese/Spanish prose or fixture.

**Unicode capabilities that stay but are not “non-English content”:** IME composition
handling (`web/src/lib/chat-send-mode.ts`), FTS5 trigram tokenizer for CJK target traffic
(`traffic/traffic.go`), UTF-8-safe byte truncation (`notify/render.go`), RFC 2047 email
Subject encoding (`notify/email.go`), UTF-8 BOM in CSV export (`report/findings.go`), and
acceptance of non-ASCII skill identifiers (`server/skill_zip.go`, skill-upload tests). Their
comments and fixtures become English/non-CJK where possible, but the mechanisms stay.

---

## 8. Workstreams

### 8.1 Go backend

1. **Config** (`config/config.go`): delete `Config.Language`, `SupportedLanguages`,
   `DefaultLanguage`, `NormalizeLanguage`, `CurrentLanguage`, `ActiveLanguage`, `Language()`;
   remove the `os` import if it becomes unused. Keep DSN/skill-dir behavior.
2. **Config catalogs** (`config/msg_i18n.go`): delete; inline English at the 4 `trCfg` and 3
   `config.T` call sites.
3. **Server catalogs** (`server/errmsg_catalog.go`, `server/errmsg_i18n.go`): delete; make
   `trMsg(x)` become `x` and `logT(x)` become `x` at every call site. Do not keep no-op
   wrappers.
4. **Server state/API** (`server/server.go`, `server/manager.go`): remove the output-language
   cache, resolver wiring, settings key, API request/response field, validation, and error
   constant. Remove the stale comments that describe the Korean default or the five-language
   set.
5. **Agent** (`agent/outputlang.go`, `agent/prompt.go`, `agent/*.go`, `agent/wrapup.go`):
   delete `outputlang.go`; replace the parameterized directive with the fixed English one;
   update the six call sites and always append to wrap-up. Update `agent/prompt_test.go` to
   assert the English directive is present and appended after the body, and that no
   per-language branch remains.
6. **Labels** (`notify/notify.go`, `intercept/label_i18n.go`, `intercept/intercept.go`):
   collapse to English maps/labels; remove `Lang`, `labelLang`, `judgeActionLang`, and the
   Korean fallbacks. Keep `[模型]`.
7. **Input aliases** (`server/server.go:3947-3958`): keep `intent`/`hint`; remove the
   Chinese/Korean aliases and their comments. Update `server/server_localized_test.go`.
8. **Env/config surface** (`config.example.json`, `.env.example`, `docker-compose.yml`):
   remove `language`/`_comment_language`/`ARTEX_LANG`. Do not add a deprecation warning
   dependency; the values are simply ignored.
9. **Comments:** rewrite stale comments that say “Korean”, “localized”, “ko fallback”, or
   “en/ko/zh/es (ja missing)” to describe the English-only behavior.
10. **Side-question and intercept judge prompts:** append the English-only rule to
    `sidequestion/request.go:33` `instruction` and to the intercept judge prompt
    (`intercept/prompt.go:36,124-127`), so answers and persisted `comment`/`InitialReason`
    values are English. Keep accepting the legacy Chinese verdict markers (§7).

### 8.2 Next.js frontend

1. **Catalogs:** keep `web/messages/en.json`; delete `ko.json`, `zh.json`, `es.json`,
   `ja.json` (and any local `zh.sources.json`).
2. **Config/store:** simplify `web/src/i18n/config.ts` to a single `LOCALE = "en"` (and the
   minimal type/guard it needs); delete `web/src/i18n/locale-store.ts`.
3. **Request/provider:** `request.ts` loads only `en.json`; `intl-runtime-provider.tsx`
   becomes a static `NextIntlClientProvider locale="en" messages={enMessages}` (keep the
   explicit `timeZone` for prerender). It no longer calls `useActiveLocale` or mutates
   `<html lang>`.
4. **Layout/metadata:** `layout.tsx` uses `lang="en"` and static metadata; `app-config.ts`
   keeps only the English title/description and drops `META_BY_LOCALE`.
5. **Settings:** remove the display-language section, `LOCALE_LABELS`, `useActiveLocale`,
   `setActiveLocale`, and the `setSettings({ output_language })` call from
   `web/src/app/(main)/system/settings/page.tsx`. Remove `output_language` from
   `web/src/lib/types.ts`.
6. **Dates:** `getDateLocale()` returns `"en-US"`; every locale-dependent formatting site
   (18 files, 19 sites, including the two no-locale `toLocaleString()` calls in
   `side-question-workspace.tsx`) is pinned to `en-US`.
7. **Tooling:** delete `extract-i18n.ts`, `i18n-keys.ts`, `check-i18n-keys.ts`,
   `i18n-keys.test.ts`, and `config.test.ts` (or rewrite the latter into a single-catalog
   sanity test). Remove `extract:i18n`/`check:i18n` from `web/package.json` and the
   `check:i18n` step from `.github/workflows/web.yml`.
8. **Built-in labels/mock data:** keep `web/src/lib/agent-i18n.ts` and the `builtinAgents`
   namespace in `en.json` (it masks legacy DB names). Translate non-functional Chinese mock
   fixtures in `web/src/lib/mock/data.ts`/`handler.ts`; keep the `[模型]` sentinel.
9. **Locale-sensitive formatting sweep:** beyond the date sites in §8.2.6, pin every
   locale-sensitive call to English. At minimum: `web/src/app/(main)/chat/page.tsx:1276`
   (`localeCompare(..., "ko")`), `web/src/app/(main)/function/tasks/page.tsx:1616` (bare
   `toLocaleString()`), `web/src/components/ui/calendar.tsx:43,202` (`locale?.code`, which is
   undefined from `date-range-picker.tsx` and therefore follows the browser), and the bare
   number `toLocaleString()` calls in `sessions-tab.tsx`, `chart.tsx`,
   `intercept/page.tsx`, `tasks/page.tsx`, and `traffic-evidence-viewer.tsx`. Also cover the
   two no-locale `side-question-workspace.tsx` sites. Centralize date formatting in
   `web/src/i18n/datetime.ts` and number/currency formatting in `web/src/lib/utils.ts` with
   an `en-US` default; pass `"en"` to `localeCompare` wherever a locale is supplied. The
   existing `formatCurrency` de-DE unit test tests the helper's locale parameter, not
   product localization; keep it only if the helper stays generic, otherwise switch it to
   `en-US`.
10. **Functional web tokens:** keep `web/src/lib/chat-mentions.ts`, `company-scope.ts`, the
    full-width comma, and the IME handling. Update their comments to say they are functional
    compatibility, not localization.

### 8.3 Tests and fixtures

1. **Delete** localization-only tests: `*_localized_test.go`, `config/config_localized_test.go`,
   `config/msg_i18n_test.go`, `server/logt_catalog_test.go`, `server/output_language_test.go`,
   the per-language branches in `server/engine_localized_test.go`,
   `server/intercept_archive_localized_test.go`, and web i18n tests that only assert locale
   lists/labels.
2. **Rewrite mixed tests** that incidentally assert language behavior (prompt directive,
   settings payload, notification labels, intercept labels) to assert English-only behavior.
3. **Translate non-functional CJK** in the 61 ordinary test files: table names, failure
   diagnostics, task goals, mock findings, prompts, and sample data. This is in scope because
   the owner asked for the whole codebase.
4. **Preserve functional fixtures:** keep the §7 tokens in tests where the test exists to
   prove compatibility. Add a comment naming the allowlist entry.
5. **Replace CJK-only multibyte fixtures** in truncation/token/byte-limit tests with another
   multibyte script (or update expectations) when the test's subject is multibyte handling,
   not Chinese specifically. Examples: `notify/render_test.go`, `agent/terminalreason_test.go`,
   `agent/tools_overview_test.go`, `db/task_context_unit_test.go`,
   `sidequestion/context_test.go`, `intercept/trace_test.go`, `server/task_metadata_test.go`,
   `server/task_templates_test.go`. The behavior under test must survive even if the fixture
   characters change.
6. **Replace non-ASCII skill-name fixtures** (`中文技能/端口扫描-x/日本語スキル`) with non-CJK
   Unicode identifiers (e.g. `café-skill`, `π-scan`) so the acceptance capability is still
   covered.
7. **Remove `ARTEX_LANG` setup** from `server/testmain_test.go`, `server/chat_llm_resolve_test.go`,
   and any other test that pinned the environment language.
8. **Add/keep English-only invariants:** the fixed directive is present in every user-facing
   role and wrap-up; `GET /api/settings` has no `output_language`; `PUT` rejects/ignores it by
   not accepting it; the source-language guard passes under the new allowlist.

### 8.4 Documentation

**Delete (26 files):**

- `README.ko.md`, `README.zh.md`, `README.es.md`, `README.ja.md`
- `CHANGELOG.ko.md`, `CHANGELOG.zh.md`
- `CONTRIBUTING.ko.md`, `MAINTAINING.ko.md`, `SECURITY.ko.md`, `CODE_OF_CONDUCT.ko.md`
- `detections/README.ko.md`, `detections/{attack,indicators,sigma,suricata,tests,triage}/README.ko.md`
- `docs/defense-ko.md`, `docs/finding-traffic-evidence-ko.md`,
  `docs/finding-traffic-evidence-zh.md`
- `sidequestion/{CONTEXT_BUDGET,README,VALIDATION}.{ko,zh}.md`
- `sidequestion/validation-2026-09-10.json`

**Before deleting, migrate unique operational content** from the translated docs into the
English docs. At minimum:

- the Nginx reverse-proxy example referenced by `README.md:180`;
- any development/verification instructions referenced by `README.md:385`;
- any installation/release details that only exist in `README.zh.md`;
- any detection-guide content present only in `docs/defense-ko.md` (the English guide is
  expected to be complete; verify section by section).

**Rewrite:**

- `README.md`: remove the language navigation and the “switchable in Settings” claims;
  remove the multilingual-edition rationale and the `README.zh.md` notes; remove Korean
  statute parentheticals (use official English names); remove `docs/defense-ko.md` and
  Chinese-UI screenshot references; keep the security/authorized-use warnings, English
  screenshots, installation, configuration, and architecture content. Update the
  configuration section so it no longer documents `ARTEX_LANG` or an output-language
  setting.
- `CONTRIBUTING.md`: replace the localization policy with the English-only policy and the
  §7 allowlist; remove the translation-issue link, the `i18n/` branch prefix, and the
  Korean commit-message rule (commit messages are English); update the contribution process
  to run the tightened language guard; update the design-doc reference that points at the
  Chinese original.
- `MAINTAINING.md`: rewrite as “Maintaining the English-only edition”. Keep the upstream base
  note and the detection-indicator check; remove the translation-symmetry/drift sections and
  the localized-asset guidance; document that upstream is unreachable and that any future
  upstream Chinese prompt text is translated to English during sync.
- `CHANGELOG.md`: remove the language navigation at line 3 and the `CHANGELOG.zh.md` links;
  keep historical entries (with their English descriptions of the multilingual era); fix the
  broken `README.en.md` reference; add an `[Unreleased]` entry for this migration.
- `SECURITY.md`, `CODE_OF_CONDUCT.md`: remove the Korean navigation line; content otherwise
  stays English.
- `docs/defense-en.md`: remove the `defense-ko` links and Korean-language references; keep
  the English guide and the Korea-specific English section; update the Sigma `related` links
  to point at `defense-en.md`.
- `docs/finding-traffic-evidence-en.md`: remove the zh/ko links and the “original preserved”
  banner.
- `sidequestion/README.md`, `CONTEXT_BUDGET.md`, `VALIDATION.md`: remove Korean/Chinese
  navigation and translation banners; rewrite `VALIDATION.md` as English-only evidence or
  delete it if it only validated the Chinese original.
- `screenshots/README.md`: describe only the English `en/` set; note that the Chinese-UI
  captures were removed.
- `detections/README.md` and the six subdirectory English READMEs: remove the Korean
  navigation, the “Korean translation” banner, and `defense-ko` links; update any
  cross-references to deleted Korean READMEs.
- `.github/PULL_REQUEST_TEMPLATE.md`: remove the localization change type and the five-file
  localization checklist; keep the English-only invariant in the verification checklist
  (run the language guard).
- `.github/ISSUE_TEMPLATE/config.yml`: no translation template link to remove, but confirm
  the remaining templates are English.

### 8.5 Scripts, CI, hooks, detection harness

1. **`scripts/check-source-language.py`**
   - Keep the CJK detector and the functional allowlist entries in §7.
   - Remove the `ALLOWED_FILES` entries that exist only for localized catalogs/labels
     (`server/errmsg_catalog.go`, `config/msg_i18n.go`, `notify/notify.go`,
     `intercept/label_i18n.go`, `web/src/config/app-config.ts`, `web/src/i18n/config.ts`,
     `web/src/lib/agent-i18n.ts`, `web/src/scripts/extract-i18n.ts` as applicable).
   - Remove `is_localized_asset()` exemptions (the assets are gone) and stop skipping tests;
     scan `*_test.go`, `*.test.*`, and `*.spec.*`.
   - Keep line-level allowlist patterns for the §7 tokens; remove the Korean-law-name and
     native-language-name patterns.
   - Skip generated `graphify-out/` and process records under `docs/superpowers/` with an
     explicit comment: the former is rebuilt output, and the latter are change-design
     records that may quote the tokens being removed. Neither is shipped product
     documentation. Do not add any other directory-level exemption.
   - Update the docstring and the success message (“English-only source”).
2. **`scripts/check-web-cjk.py`**: keep; update the docstring (no language switcher); keep
   `ALLOWED_CJK` empty. It continues to scan `web/out/*.html`.
3. **`scripts/check-external-links.py`**: change `Accept-Language: en,ko;q=0.8` to `en`;
   update the docstring examples that mention `CHANGELOG.zh.md`.
4. **`scripts/external-links-allowlist.txt`**: remove entries/examples that exist only for
   `README.zh.md`/`CHANGELOG.zh.md`; keep the upstream-contributor link if an English
   document still references it.
5. **`.github/workflows/ci.yml`**: keep the source-language step; update comments that
   mention Korean-localization regressions.
6. **`.github/workflows/web.yml`**: remove the five-locale parity step; keep unit tests,
   static build, CJK guard, and Biome; update comments.
7. **`.github/workflows/detections.yml`**: remove `docs/defense-ko.md` from both paths lists;
   keep `docs/defense-en.md`.
8. **`.github/workflows/docs.yml` / `external-links.yml`**: no structural change; the link
   checker will validate the rewritten docs. Update allowlist/external-link references.
9. **`.pre-commit-config.yaml`**: change the detections `files` regex from
   `docs/defense-(ko|en)\.md` to `docs/defense-en\.md`; keep the docs and source-language
   hooks.
10. **`detections/tests/check-defense-template.py`**: remove the KO path and the ko/en parity
    assertion; extract the YAML block from `docs/defense-en.md` only; keep the sigma check
    and conversion-matrix assertions. Update `check-defense-template.sh` and any comments
    that mount both docs.
11. **Detection files**: update `related`/comment links in `detections/sigma/*.yml`,
    `detections/sigma/correlation/*.yml`, `detections/suricata/artex.rules`, and
    `detections/sigma_lint/validators.yml` from `docs/defense-ko.md` to
    `docs/defense-en.md`. Keep the legacy Chinese guard marker and its fixture (§7).
12. **`graphify update .`** after the code changes, per the repository rule.

### 8.6 Persisted state and migration

1. **`output_language` settings row:** remove all readers/writers. On a fresh database the
   key never appears. For an existing developer database, an idempotent
   `DELETE FROM settings WHERE key = 'output_language'` is optional; do not add a data
   migration solely for it.
2. **Historical user content:** there is none to migrate — the project has not been deployed
   and has no accumulated run data. Developer databases may be recreated. The migration does
   not scan or translate existing rows.
3. **Seeded built-in agents:** the built-in seeds in this tree are English. `en.json`'s
   `builtinAgents` namespace remains useful only for a developer database seeded by an older
   upstream build; it requires no data migration.
4. **Browser locale state:** stale `artex_locale` localStorage/cookie values are ignored
   after the store is deleted. No active cleanup.
5. **API compatibility:** removing `output_language` from `/api/settings` is an intentional
   breaking change for external clients. The bundled UI is updated in the same change.

### 8.7 Screenshots and release assets

- Keep `screenshots/en/{dashboard,findings,chat}.png`; they are the delivered English UI.
- Delete the 13 root Chinese-UI PNGs and `screenshots/wx.png`.
- Rewrite `screenshots/README.md` to describe only the English set.
- `build.sh` copies `README.md` only; no packaging change. Verify `Dockerfile`/release
  workflow do not reference deleted docs or catalogs.

---

## 9. Deletion list (explicit)

**Go**
- `config/msg_i18n.go`
- `server/errmsg_catalog.go`
- `server/errmsg_i18n.go`
- `agent/outputlang.go`
- `intercept/label_i18n.go` (replace with an English label helper or inline labels)
- Localization-only tests and test helpers (§8.3)

**Web**
- `web/messages/{ko,zh,es,ja}.json`
- `web/src/i18n/locale-store.ts`
- `web/src/scripts/extract-i18n.ts`
- `web/src/scripts/i18n-keys.ts`
- `web/src/scripts/check-i18n-keys.ts`
- `web/src/scripts/i18n-keys.test.ts`
- `web/src/i18n/config.test.ts` (or rewrite as a single-catalog sanity test)

**Docs/assets**
- The 26 translated Markdown files in §8.4
- `sidequestion/validation-2026-09-10.json`
- `.github/ISSUE_TEMPLATE/{bug_report_ko.yml,feature_request_ko.yml,translation_issue_en.yml,translation_issue_ko.yml}`
- 13 root `screenshots/*.png` plus `screenshots/wx.png`

**Not deleted:** `web/messages/en.json`, next-intl, the `useTranslations` call sites,
`web/src/lib/chat-mentions.ts`, `web/src/lib/company-scope.ts`, IME support, FTS/CJK
capabilities, the legacy guard marker, the functional allowlist tokens, English docs and
English screenshots.

---

## 10. Verification

Run from the repository root (and `web/` where noted):

1. `go build ./...`
2. `go vet ./...`
3. `go test ./... -count=1` (DB integration packages need PostgreSQL; use the CI matrix)
4. `cd web && npm ci && npm run check && npm test -- --run && npm run build:static`
5. `python3 -I scripts/check-source-language.py`
6. `python3 -I scripts/check-web-cjk.py`
7. `python3 -I scripts/check-doc-links.py`
8. `detections/tests/run-all.sh` (Docker; includes the defense-template gate)
9. `graphify update .`
10. Grep gates (must return only §7 allowlist hits or nothing; exclude generated
    `graphify-out/` and process records under `docs/superpowers/`):
    - `git grep -nE 'README\.(ko|zh|es|ja)|CHANGELOG\.(ko|zh)|CONTRIBUTING\.ko|MAINTAINING\.ko|SECURITY\.ko|CODE_OF_CONDUCT\.ko|defense-ko|finding-traffic-evidence-(ko|zh)|web/messages/(ko|zh|es|ja)|LOCALE_LABELS|useActiveLocale|setActiveLocale|output_language|ARTEX_LANG' -- . ':!graphify-out' ':!docs/superpowers'`
    - `git grep -nE 'langDirective\(|SupportedLanguages|NormalizeLanguage|ActiveLanguage|SupportedOutputLangs|labelLang|judgeActionLang' -- . ':!graphify-out' ':!docs/superpowers'`
    - `git grep -nE 'localeCompare\([^,]+, "(ko|zh|ja|es)"|toLocale(String|DateString|TimeString)\(\s*\)' -- web/src` — production hits must be gone; test-only helper cases must be justified.
    - `git grep -nP '[\x{AC00}-\x{D7AF}\x{4E00}-\x{9FFF}\x{3040}-\x{30FF}]' -- . ':!graphify-out' ':!docs/superpowers'` — review every remaining hit against §7.
11. Manual smoke test:
    - Settings has no display-language control; the UI renders English; dates use `en-US`.
    - An agent run against a Chinese-language target still produces English findings,
      summaries, reports, and chat replies.
    - A historical `[模型]` row, an old chat mention, and an old verdict still parse/render.
    - `GET /api/settings` has no `output_language`; `PUT` with it is not accepted.
    - A legacy Sigma rule matches the Chinese guard marker fixture.
    - Side-question answers and intercept judge comments are English when the question or
      target is Chinese.
    - On a browser set to Korean/Chinese, dates, numbers, month names, and name sorting
      render in English (`en-US`).
    - No Korean/Chinese/Japanese text appears in `web/out/*.html`.

**Definition of done:** all of the above pass, the guard scripts have no localized-asset
escapes, no tracked file contains non-English prose outside §7, and no live document links
to a deleted file.

---

## 11. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Deleting translated docs loses unique operational content | Audit and migrate before deleting; the checks in §8.4 are mandatory |
| 61 ordinary test files with CJK create a large, error-prone translation diff | Do it in batches per package; keep functional fixtures; run each package's tests before moving on |
| Replacing CJK multibyte fixtures breaks the behavior under test | Replace with another multibyte script or update expectations; never silently reduce coverage |
| `detections/tests/check-defense-template.py` and Sigma links break when `defense-ko.md` is deleted | Update the test, workflow paths, and all `related` links in the same change |
| Removing `output_language` breaks an external client | Intentional breaking change; document it in the changelog; the bundled UI is updated together |
| A developer database still has rows from an older run | No production data exists; recreate the developer DB or ignore the rows. No translation pass is planned. |
| A stale language setting/env value changes behavior | Remove all readers; values are ignored; the UI no longer writes them |
| The source-language guard still passes because it skips tests/localized assets | Tighten the guard first, then make the tree pass; do not add broad exemptions |
| Static export or next-intl prerender fails under a single locale | Keep the explicit `timeZone`; run `build:static`; verify `lang="en"` in the output |
| `CHANGELOG.zh.md`/`README.zh.md` deletion breaks links | Fix all references in the same change; run `check-doc-links.py` |
| Side-question answers or intercept comments remain non-English | Append the fixed directive to both prompts; test with a Chinese question/target |
| Browser-locale formatting leaks non-English month names, digits, or sort order | Repo-wide locale-sensitive sweep; helper defaults to `en-US`; grep gate |
| The tightened guard fails on the design/plan docs or the generated graph | Explicitly skip `docs/superpowers/` and `graphify-out/` as non-product artifacts; keep the exemption documented |

**Rollout:** one branch. Recommended order: (1) tighten the guards; (2) backend removal;
(3) frontend single-catalog removal; (4) test translation/rewrite; (5) documentation
deletion/rewrite; (6) detection/CI cleanup; (7) full verification. Each phase should leave
the tree buildable where possible; the final phase is the only one that deletes the
translated docs, after all references are rewritten.

---

## 12. What is left to the builder

These are mechanical or low-judgment choices the plan can settle:

- The exact replacement text for each `trMsg`/`logT`/`config.T` call site and the order of
  edits per file.
- Whether `getDateLocale()` remains as a one-line constant helper or is inlined; either is
  fine as long as every formatting site uses `en-US`.
- Whether to keep a minimal `en.json` sanity test after deleting the parity tooling.
- The exact DB mechanism for removing the `output_language` row (schema statement vs one-time
  migration) and whether to leave the harmless row.
- The exact non-CJK replacement characters for multibyte test fixtures.
- Whether `sidequestion/VALIDATION.md` is rewritten or deleted after its Chinese evidence is
  removed.
- The `[Unreleased]` changelog wording and whether a version bump accompanies the change.
- The order of documentation-reference fixes so `check-doc-links.py` stays green.
- The mechanism for sharing the English directive with `sidequestion` and `intercept` (a
  small shared package, an exported constant, or per-package copies plus an equality test).
- The exact locale-pinning approach for the calendar and number formatters (default
  parameter vs a wrapper helper).

## 13. Owner confirmations still worth recording

These are the places where the design makes a judgment the owner may want to revisit; they
are not blockers for planning:

1. **No historical DB content exists.** The project has not been deployed and has no
   accumulated runs, so there is nothing to translate or preserve. A developer database may
   be recreated.
2. **Official Korean statute names are removed** from English prose in favor of their English
   names. If native citations should remain, they can be re-added as a narrow allowlist.
3. **Upstream provenance is lost from the working tree.** Deleting `README.zh.md`,
   `CHANGELOG.zh.md`, and `docs/finding-traffic-evidence-zh.md` removes the side-by-side
   upstream copies; the content remains in git history and on
   `backup/pre-squash-2026-10-24`.
4. **`sidequestion/validation-2026-09-10.json` is deleted** with the Chinese validation
   evidence. If it is needed as an audit artifact, it should be archived outside the repo
   first.
