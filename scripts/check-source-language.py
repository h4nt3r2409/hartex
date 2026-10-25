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


def _u(*codepoints: int) -> str:
    """Return these Unicode code points as a string (keeps this file ASCII-only)."""
    return "".join(chr(cp) for cp in codepoints)


# Line-level exceptions for the functional allowlist in §7 of the English-only design.
# Non-English tokens are built from code points so this guard file is itself
# English-only. §7 of docs/superpowers/specs/2026-10-25-english-only-design.md lists the
# exact tokens. Each entry is (regex, reason) and is anchored to its functional site.
_CHAT_LABELS = "|".join([
    _u(0x6F0F, 0x6D1E),          # finding
    _u(0x8D44, 0x4EA7),          # asset
    _u(0x4F01, 0x4E1A),          # company
    _u(0x63A5, 0x53E3),          # endpoint
    "IP",                        # ip
    _u(0x5E94, 0x7528),          # app
    _u(0x57DF, 0x540D),          # root_domain
    _u(0x5B50, 0x57DF, 0x540D),  # subdomain
    _u(0x670D, 0x52A1),          # service
])
_MODEL = _u(0x6A21, 0x578B)
_COMMA = _u(0xFF0C)

ALLOWED_PATTERNS = [
    # §7 item 1: persisted model-vs-rule decision sentinel, plain in strings and
    # escaped in TypeScript regex literals; matched by SQL LIKE/strings.HasPrefix.
    (re.compile(r"\[" + _MODEL + r"\]|\\\[" + _MODEL + r"\\\]"), "model-vs-rule decision sentinel"),
    # §7 item 2: persisted @[kind#id label] chat-mention wire token.
    (re.compile(r"@\[(?:" + _CHAT_LABELS + r")#"), "persisted chat-mention wire token"),
    # §7 item 2: the Go/TypeScript regex that defines the wire token.
    (re.compile(r"chatMentionPattern = regexp\.MustCompile|matchAll\(/@\\\["), "chat-mention wire-token regex"),
    # §7 item 2: server-side kind -> wire-label table.
    (re.compile(r'"(?:' + _CHAT_LABELS + r')":\s*"'), "chat-mention label table"),
    # §7 item 2: web-side kind -> wire-label table.
    (re.compile(r'label: "(?:' + _CHAT_LABELS + r')"'), "chat-mention label table"),
    # §7 item 2: wire-label fallback and struct-literal fixtures.
    (re.compile(r'\?\? "' + _u(0x8D44, 0x4EA7) + r'"'), "chat-mention wire-label fallback"),
    (re.compile(r'chatMentionRef\{[^}]*(?:' + _CHAT_LABELS + r')"'), "chat-mention wire-label fixture"),
    # §7 item 2: fixtures that exercise wire-label search and @-trigger parsing.
    (re.compile(r'mentionSearch\("(?:' + _CHAT_LABELS + r')'), "chat-mention search fixture"),
    (re.compile(r'activeMention\("[^"]*@(?:' + _CHAT_LABELS + r')'), "chat-mention trigger fixture"),
    # §7 item 3: legacy Chinese verdict markers accepted from older prompts.
    (
        re.compile(
            _u(0x5B9E, 0x9645, 0x64CD, 0x4F5C, 0xFF1A)
            + "|"
            + _u(0xFF1B, 0x6210, 0x529F, 0x540E, 0x7684, 0x540E, 0x679C, 0xFF1A)
            + "|"
            + _u(0xFF1B, 0x547D, 0x4E2D, 0x89C4, 0x5219, 0xFF1A)
        ),
        "legacy Chinese verdict markers",
    ),
    # §7 item 4: literal upstream LLM-provider quota error text.
    (
        re.compile(
            _u(0x4F59, 0x989D, 0x4E0D, 0x8DB3)
            + "|"
            + _u(0x989D, 0x5EA6, 0x4E0D, 0x8DB3)
            + "|"
            + _u(0x989D, 0x5EA6, 0x5DF2, 0x7528, 0x5C3D)
            + "|"
            + _u(0x6B20, 0x8D39)
        ),
        "provider quota error markers",
    ),
    # §7 item 5: the exact keyword tables in server/logsink.go that classify logs
    # emitted by upstream tools.
    (re.compile(r'"unreachable", "' + _u(0x5931, 0x8D25) + r'"'), "log-classification status keywords"),
    (re.compile(r'"retrying", "' + _u(0x7981, 0x7528) + r'"'), "log-classification warning keywords"),
    # §7 item 6: functional ICP-filing detection in target/company data, including
    # the full-width and ideographic domain separators accepted by its normalizer.
    (re.compile(_u(0x5907, 0x6848) + r"|\." + _u(0xFF0E, 0x3002)), "functional ICP detection"),
    # §7 item 7: byte-exact legacy traffic_search description migration sentinel.
    (
        re.compile(
            _u(
                0x67E5, 0x8BE2, 0x8BB0, 0x5F55, 0x4EE3, 0x7406, 0x5DF2,
                0x6293, 0x53D6, 0x7684, 0x76EE, 0x6807, 0x6D41, 0x91CF,
            )
        ),
        "legacy traffic_search description sentinel",
    ),
    # §7 item 8: lets defenders match audit logs from older ARTEX builds.
    (
        re.compile(
            _u(0x3010)
            + "ARTEX "
            + _u(0x5E73, 0x53F0, 0x7BA1, 0x63A7, 0x00B7, 0x975E, 0x76EE, 0x6807, 0x9632, 0x5FA1)
            + _u(0x3011)
        ),
        "legacy guard detection marker",
    ),
    # §7 item 9: GBK zip-name decoding fixture (Chinese entry names are the data the
    # decoder exists to support).
    (re.compile(_u(0x4E2D, 0x6587, 0x6280, 0x80FD) + "|" + _u(0x53C2, 0x8003, 0x8D44, 0x6599)), "GBK zip-name fixture"),
    # §7 item 10: detection recipes for Chinese login/authorization pages.
    (re.compile(r"const NEGATIVE_RE = /"), "Chinese login/authorization regex (preload.js)"),
    (re.compile(r"grep -rhoaE"), "Chinese login/authorization grep example (reference.md)"),
    # §7 item 11: the guards cannot work without their CJK detector ranges.
    (re.compile(r"가-힣|㐀-鿿|豈-﫿"), "CJK range in a check script"),
    # §7 item 12: full-width comma accepted as an input separator (paste tolerance)
    # at its two call sites, plus the §7 table row that documents it.
    (re.compile(r'comma "' + _COMMA + r'"'), "full-width comma call-site comments"),
    (re.compile(r"\.split\(/\[[^\]]*" + _COMMA + r"[^\]]*\]\+/\)"), "full-width comma input separator"),
    (re.compile("Full-width comma `" + _COMMA + "`"), "full-width comma in the documentation table"),
    # §7 item 2: full-width brackets escape `[`/`]` inside a persisted chat-mention
    # wire label so they cannot terminate the label.
    (re.compile(r'"' + _u(0xFF08) + r'" : "' + _u(0xFF09) + r'"'), "chat-mention wire-label bracket escaping"),
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
