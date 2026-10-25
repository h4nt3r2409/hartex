#!/usr/bin/env python3
"""Fail if non-English text creeps back into the English-only source.

ARTEX is English-only: code comments, agent prompts, scripts, CI configuration, tests, and
developer documentation are written in English. A few strings are intentionally not
English because they are functional machine values, legacy wire formats, detection
patterns, or encoding support. This script scans the tracked source tree and reports every
CJK line that is not explicitly allowed, so a future edit cannot silently reintroduce
translation debt.

What is scanned
---------------
Tracked `.go`, `.ts`, `.tsx`, `.css`, `.sh`, `.py`, `.yml`, `.yaml`, `.sql`, `.md`,
`.bat`, `.mjs`, and `.js` files, including tests (`*_test.go`, `*.test.*`, `*.spec.*`).
JSON files are skipped because they are machine data or the English message catalog.

What is skipped
---------------
Two non-product trees are skipped explicitly: generated `graphify-out/` (rebuilt output)
and process records under `docs/superpowers/` (change designs and plans, which
necessarily quote tokens under removal). Neither is shipped product documentation. Do not
add any other directory-level exemption.

What is allowed
---------------
`ALLOWED_PATTERNS` lists line-level exceptions for the functional allowlist in the
English-only design (§7): wire sentinels, functional input keywords, legacy protocol
markers, detection patterns, and encoding support. Every entry has a reason and names the
§7 item it implements. Add a new one only when the text is genuinely functional and
listed in §7.

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

# Generated output and change-process records, not shipped product documentation.
# `graphify-out/` is rebuilt by `graphify update`; `docs/superpowers/` holds the design
# and plan records for in-flight changes, which necessarily quote the tokens being
# removed. Do not add any other directory-level exemption.
SKIP_PREFIXES = ("graphify-out/", "docs/superpowers/")

# Line-level exceptions for the functional allowlist in §7 of the English-only design.
# Each entry is (regex, reason).
ALLOWED_PATTERNS = [
    # §7 item 1: persisted model-vs-rule decision sentinel, matched by SQL LIKE and
    # strings.HasPrefix.
    (re.compile(r"模型"), "[模型] model-vs-rule decision sentinel"),
    # §7 item 2: persisted @[kind#id label] conversation wire format.
    (re.compile(r"漏洞|资产|企业|接口|应用|域名|子域名|服务"), "chat-mention wire labels"),
    # §7 item 3: legacy Chinese verdict markers accepted from older prompts.
    (re.compile(r"实际操作：|；成功后的后果：|；命中规则："), "legacy Chinese verdict markers"),
    # §7 item 4: literal upstream LLM-provider quota error text.
    (re.compile(r"余额不足|额度不足|额度已用尽|欠费"), "provider quota error markers"),
    # §7 item 5: classifies log lines emitted by upstream tools.
    (re.compile(r"失败|丢弃|拒绝|禁用|重试|不可达"), "functional log-classification keywords"),
    # §7 item 6: functional ICP-filing detection in target/company data, including
    # the full-width and ideographic domain separators accepted by its normalizer.
    (re.compile(r"备案|\.．。"), "functional ICP detection"),
    # §7 item 7: byte-exact legacy traffic_search description migration sentinel.
    (re.compile(r"查询记录代理已抓取的目标流量"), "legacy traffic_search description sentinel"),
    # §7 item 8: lets defenders match audit logs from older ARTEX builds.
    (re.compile(r"【ARTEX 平台管控·非目标防御】"), "legacy guard detection marker"),
    # §7 item 9: GBK zip-name decoding fixture (Chinese entry names are the data the
    # decoder exists to support).
    (re.compile(r"中文技能|参考资料"), "GBK zip-name fixture"),
    # §7 item 10: detection recipe for Chinese login/authorization pages on targets.
    (re.compile(r"未登录|未授权|登录失效|授权"), "Chinese login/authorization matching regex"),
    # §7 item 11: the guards cannot work without their CJK detector ranges.
    (re.compile(r"가-힣|㐀-鿿|豈-﫿"), "CJK range in a check script"),
    # §7 item 12: full-width comma accepted as an input separator (paste tolerance).
    (re.compile(r"，"), "full-width comma accepted as an input separator"),
    # §7 item 2: full-width brackets escape `[`/`]` inside a persisted chat-mention
    # wire label so they cannot terminate the label.
    (re.compile(r'"（" : "）"'), "chat-mention wire-label bracket escaping"),
]


def main() -> int:
    tracked = subprocess.check_output(["git", "ls-files"], text=True).splitlines()
    offenders = []
    for path in tracked:
        if not path.endswith(SCAN_EXTENSIONS):
            continue
        if path.startswith(SKIP_PREFIXES):
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
        print("English-only source: no untranslated Korean/Chinese/Japanese text found.")
        return 0

    print(
        f"Untranslated Korean/Chinese/Japanese text in {len(offenders)} English-only source lines:"
    )
    for path, lineno, text in offenders:
        print(f"  {path}:{lineno}: {text}")
    print(
        "\nTranslate the line, or — if it is a genuine functional token or wire value — "
        "add a narrowly scoped exception to ALLOWED_PATTERNS with a reason naming the "
        "§7 item."
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
