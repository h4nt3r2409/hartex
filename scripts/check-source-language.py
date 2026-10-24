#!/usr/bin/env python3
"""Fail if untranslated Korean/Chinese text creeps back into the English-first source.

ARTEX is English-first: code comments, agent prompts, scripts, CI configuration, and
developer documentation are written in English. A few strings are intentionally not
English because they are machine values, upstream API markers, legacy wire formats,
or localized product assets. This script scans the tracked source tree and reports
every CJK line that is not explicitly allowed, so a future edit cannot silently
reintroduce translation debt.

What is scanned
---------------
Tracked `.go`, `.ts`, `.tsx`, `.css`, `.sh`, `.py`, `.yml`, `.yaml`, `.sql`, `.md`,
`.bat`, `.mjs`, and `.js` files. Tests (`*_test.go`, `*.test.*`, `*.spec.*`) are
skipped because they intentionally carry non-English fixtures. JSON files are
skipped because they are localized catalogs, evidence records, or machine data.

What is allowed
---------------
`ALLOWED_FILES` lists whole files whose non-English content is a localized product
asset or an intentional machine-value table. `ALLOWED_PATTERNS` lists line-level
exceptions: wire sentinels (`[模型]`), functional input keywords, legacy protocol
markers, native language names, and links into localized documents. Every entry
has a reason; add a new one only when the text is genuinely not translation debt.

Uses only the standard library and never touches the network.
Run: `python3 -I scripts/check-source-language.py`
"""
import re
import subprocess
import sys

CJK = re.compile(r"[　-〿぀-ヿ㐀-䶿一-鿿豈-﫿가-힯＀-￯]")

SCAN_EXTENSIONS = (
    ".go", ".ts", ".tsx", ".css", ".sh", ".py", ".yml", ".yaml",
    ".sql", ".md", ".bat", ".mjs", ".js",
)

# Whole files that are intentionally non-English: localized product assets,
# machine-value tables, and evidence records.
ALLOWED_FILES = {
    # Localized message catalogs (values are ko/zh/es/ja by design).
    "server/errmsg_catalog.go",
    "config/msg_i18n.go",
    # Localized label tables for notifications and interception actions.
    "notify/notify.go",
    "intercept/label_i18n.go",
    # Native language names used to name a display language.
    # Provider API error substrings used for quota detection.
    # Functional input detection (Chinese ICP keyword, full-width punctuation).
    "db/company_scope.go",
    "web/src/lib/company-scope.ts",
    # [模型] wire sentinel and its SQL/Go/TS consumers.
    # Chat-mention wire labels and their matching regex.
    "web/src/lib/chat-mentions.ts",
    # Chinese log-classification keywords (functional, with English equivalents).
    # Input trigger aliases and comments that quote localized UI values.
    "server/server.go",
    # Legacy DB migration sentinel (matches existing installations).
    "server/finding_workflow.go",
    # Legacy Chinese verdict markers accepted for backward compatibility.
    # Localized web locale metadata and native language labels.
    "web/src/config/app-config.ts",
    "web/src/i18n/config.ts",
    "web/src/lib/agent-i18n.ts",
    # Full-width comma is accepted as an input separator.
    "web/src/app/(main)/system/notify/page.tsx",
    "web/src/app/(main)/system/notify/_components/channel-fields.ts",
    # Namespace regex intentionally allows Hangul.
    "web/src/scripts/extract-i18n.ts",
    # Helper script documents CJK ranges and examples.
    "scripts/check-web-cjk.py",
    # A grep example intentionally matches Chinese login-page text.
    # English docs contain native language names, Korean law names, and links into
    # the Chinese/Korean localized documents (anchors included).
    # The guard-detection rule and its fixtures keep the legacy Chinese marker so
    # defenders can still match logs from older ARTEX builds.
}

# Line-level exceptions. Each entry is (regex, reason).
ALLOWED_PATTERNS = [
    (re.compile(r"模型"), "wire sentinel for model decision_source"),
    (re.compile(r"【ARTEX 平台管控·非目标防御】"), "legacy guard detection marker"),
    (re.compile(r"备案"), "functional ICP keyword"),
    (re.compile(r"余额不足|额度不足|额度已用尽|欠费"), "provider quota error markers"),
    (re.compile(r"한국어|中文|日本語|Español"), "native language names"),
    (re.compile(r"정보통신망법|개인정보보호법"), "Korean law names"),
    (re.compile(r"#截图预览|#安装|#反向代理部署|#开发"), "anchors in the Chinese README"),
    (re.compile(r"grep -rhoaE"), "Chinese login-page grep example"),
    (re.compile(r"未登录|未授权|登录失效|授权"), "Chinese login/authorization matching regex"),
    (re.compile(r"가-힣|㐀-鿿|豈-﫿"), "CJK range in a check script"),
    (re.compile(r"实际操作：|；成功后的后果：|；命中规则："), "legacy Chinese verdict markers"),
    (re.compile(r"意图|提示|의도|힌트"), "functional input trigger aliases"),
    (re.compile(r"漏洞|资产|企业|接口|应用|域名|子域名|服务"), "chat-mention wire labels"),
    (re.compile(r"失败|丢弃|拒绝|禁用|重试|不可达"), "functional log-classification keywords"),
    (re.compile(r"，"), "full-width comma accepted as an input separator"),
    (
        re.compile(r"红幕科技 RedHaze Group"),
        "verbatim page title quoted from a captured HTTP response",
    ),
]


def is_localized_asset(path: str) -> bool:
    if path.endswith(
        (".ko.md", ".zh.md", ".es.md", ".ja.md", "-ko.md", "-zh.md", "-es.md", "-ja.md")
    ):
        return True
    if path.startswith("web/messages/") and path.endswith(".json"):
        return True
    if path.startswith(".github/ISSUE_TEMPLATE/") and path.endswith("_ko.yml"):
        return True
    return False


def is_test(path: str) -> bool:
    return (
        path.endswith("_test.go")
        or ".test." in path
        or ".spec." in path
    )


def main() -> int:
    tracked = subprocess.check_output(["git", "ls-files"], text=True).splitlines()
    offenders = []
    for path in tracked:
        if not path.endswith(SCAN_EXTENSIONS):
            continue
        if is_test(path) or is_localized_asset(path) or path in ALLOWED_FILES:
            continue
        try:
            lines = open(path, encoding="utf-8", errors="ignore").read().splitlines()
        except OSError:
            continue
        for lineno, line in enumerate(lines, 1):
            if not CJK.search(line):
                continue
            if any(pattern.search(line) for pattern, _ in ALLOWED_PATTERNS):
                continue
            offenders.append((path, lineno, line.strip()[:160]))

    if not offenders:
        print("No untranslated Korean/Chinese text found in English-first source.")
        return 0

    print(
        f"Untranslated Korean/Chinese text in {len(offenders)} English-first source lines:"
    )
    for path, lineno, text in offenders:
        print(f"  {path}:{lineno}: {text}")
    print(
        "\nTranslate the line, or — if it is a genuine wire value, localized asset, "
        "or functional input token — add a narrowly scoped exception to "
        "scripts/check-source-language.py with a reason."
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
