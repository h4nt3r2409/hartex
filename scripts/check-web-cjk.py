#!/usr/bin/env python3
"""Fail if Korean/Chinese/Japanese text leaks into the statically exported web UI (`web/out`).

This fork is English-first: the screen a user sees contains no untranslated Korean,
Chinese, or Japanese. Relying on manual inspection for that property means a
contributor can hard-code a string or miss a translation and the regression stays
silently broken until the next person happens to look. This script turns that
property into a merge gate.

How it checks
-------------
It scans the **raw content** of every `.html` file under `web/out` — not just the
rendered markup but also inline scripts and React Server Components flight data —
and exits with code 1 if it finds any Han, Hangul, or Kana character. Raw content matters because
user-visible text also travels in the flight data (JSON-shaped strings) embedded
in the HTML; stripping tags alone would miss leaks on that path.

Out of scope (honest limitations)
---------------------------------
This gate sees only the static export. It cannot see content the browser renders
at runtime in **MOCK mode (`NEXT_PUBLIC_MOCK=1`, used by the public demo)**.
`web/src/lib/mock/data.ts`, `handler.ts`, and the hard-coded samples on each page
are fetched by `mockHandle()` in a `useEffect` after hydration and rendered into
React state, so they live only in the runtime JS bundle, not in `out` HTML. A leak
on that path passes this gate (a public demo's `/system/logs` sample log once
carried Chinese that way and was fixed in backlog item G114).

That surface is guarded by source review instead. Chinese remaining in files that
can render (`web/src` `.tsx` files and client data) must be one of: a code
comment; an i18n fallback unreachable because `t.has(...)` guards it; a backend
wire-format token (chat-mention kind labels, the `[模型]` decision-source
sentinel); a branch literal shown only when the locale is `zh`; or a mirror of an
agent-brain prompt that is intentionally kept for reference. Anything else that a
user could actually see on screen is untranslated text and must be fixed. Backlog
item G115 audited this classification exhaustively.

Detection scope
---------------
- **Han, Hangul, and Kana are detected**: CJK Unified Ideographs and Extension A
  (㐀–鿿), CJK Compatibility Ideographs (豈–﫿), Supplementary Plane Extension B and
  above (𠀀–), Hangul syllables and jamo (가–힣, ᄀ–ᇿ, ㄰–㆏), and Kana (぀–ヿ).
  Full-width punctuation is not treated as a leak by itself, to avoid false
  positives from preserved code blocks or quoted evidence.

Intentional Han that must be allowed
------------------------------------
The native names in the language switcher (for example 中文) are intentional:
they let a user recognize their own language. Add such characters to
`ALLOWED_HAN` below with a comment explaining why they are kept, so the gate stays
strict and only narrowly-scope exceptions pass.

Uses only the standard library and never touches the network.
Run: `python3 -I scripts/check-web-cjk.py [out-directory]`
(The default is `web/out` at the repository root. CI runs it from the `web`
working directory as `python3 -I ../scripts/check-web-cjk.py`.)
"""
import os
import re
import subprocess
import sys

# Han, Hangul, and Kana blocks. Ranges: Extension A + Unified Ideographs
# (U+3400–U+9FFF = 㐀–鿿), Compatibility Ideographs (U+F900–U+FAFF = 豈–﫿),
# Supplementary Plane Extension B and above (U+20000–U+2FFFF), Hangul syllables
# and jamo (가–힣, ᄀ–ᇿ, ㄰–㆏), and Kana (぀–ヿ).
CJK = re.compile(r"[㐀-鿿豈-﫿\U00020000-\U0002ffff가-힣ᄀ-ᇿ㄰-㆏぀-ヿ]")

# Characters intentionally shown on screen (for example native names in the
# language switcher). Empty means every detected character trips the gate. When
# adding an exception, record both the character and why.
# Example: ALLOWED_CJK = {"中", "文"}  # language-switcher label 中文 (native name)
ALLOWED_CJK: set = set()


def find_out_dir() -> str:
    """Return the `out` directory to check.

    Uses the path argument when given; otherwise `web/out` under the git
    repository root.
    """
    if len(sys.argv) > 1:
        return os.path.abspath(sys.argv[1])
    root = subprocess.check_output(
        ["git", "rev-parse", "--show-toplevel"], text=True
    ).strip()
    return os.path.join(root, "web", "out")


def main() -> int:
    out_dir = find_out_dir()
    if not os.path.isdir(out_dir):
        print(
            f"Error: directory to check does not exist — {out_dir}\n"
            "Run `npm run build:static` to create the static export first."
        )
        return 2

    checked = 0
    offenders = []  # (relative path, Han count, character preview)
    for dirpath, _dirnames, filenames in os.walk(out_dir):
        for name in filenames:
            if not name.endswith(".html"):
                continue
            checked += 1
            path = os.path.join(dirpath, name)
            with open(path, encoding="utf-8", errors="ignore") as fh:
                text = fh.read()
            found = [ch for ch in CJK.findall(text) if ch not in ALLOWED_CJK]
            if found:
                rel = os.path.relpath(path, out_dir)
                kinds = "".join(sorted(set(found)))
                offenders.append((rel, len(found), kinds))

    print(f"Checked {checked} HTML files (target: {out_dir})")
    if offenders:
        print(
            f"CJK leak in {len(offenders)} files — user-visible HTML contains Korean/Chinese/Japanese characters:"
        )
        for rel, count, kinds in offenders:
            preview = kinds if len(kinds) <= 30 else kinds[:30] + "…"
            print(f"  out/{rel}: {count} CJK characters · kinds {preview}")
        print(
            "\nTranslate them to the selected language, or — if the character is an "
            "intentional native label that must match upstream — add it to ALLOWED_CJK "
            "in scripts/check-web-cjk.py with a reason."
        )
        return 1
    if checked == 0:
        print("Warning: no HTML files were checked — verify that the build produced output.")
        return 2
    print("No CJK leak — user-visible HTML contains no Korean/Chinese/Japanese characters.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
