#!/usr/bin/env python3
"""Check that external links (http/https) referenced by tracked Markdown are still alive.

Its sibling `check-doc-links.py` checks only internal links and anchors and, by
design, skips external URLs (they depend on the network and would make a merge
gate flaky). This script performs that separate check: the external links that the
top-level README, the defense guide, and the detections READMEs tell visitors and
defenders to follow (incident-reporting portals, CISA KEV, OWASP, SigmaHQ,
Suricata, MITRE, MISP, the original demo, GitHub badges, and so on). If any of them
rots, moves, or is closed, it would otherwise stay silently broken; this script
catches that periodically and manually.

How the URL set is chosen
- Scan every tracked `.md`, skipping URLs inside fenced code blocks (``` or ~~~)
  and inline code spans (`...`). Those are command examples, config values, or
  quoted external assets (for example `id.redhaze.top` in a validation document),
  not reference links a reader is expected to follow.
- Collect http/https addresses from Markdown links and images (`[text](url)`,
  `![alt](url)`), autolinks (`<url>`), and bare URLs remaining in prose.
- Exclude reserved/placeholder hosts: localhost, private/loopback IPs (127.,
  10., 192.168., 169.254., 172.16–31., 0.0.0.0, ::1), RFC 2606/6761 reserved
  names (example.com/org/net/edu, `*.example.*`, `.test`, `.invalid`, `.local`,
  `.tld`), and names without a dot that are not FQDNs (for example `target`).

How liveness is checked (the lesson from MAINTAINING.md §8.2 in code)
Many public and security-agency sites reject HEAD requests or the default
User-Agent, or redirect several times, so a naive check mistakes healthy links
for broken ones. This script therefore checks with a **browser User-Agent, using
GET, following redirects**, and classifies the result three ways:
- OK: final status is 2xx/3xx. The link is valid.
- RESTRICTED: 401/403/405/429. The host is alive; only the checking method is
  blocked by the server's access policy (bot blocking, method rejection, rate
  limiting). It is not a broken link. Report it but do not count it as failure.
- DOWN: 404/410/5xx (after retries), or DNS/connection/timeout/SSL errors (after
  retries). The link is likely actually broken.

Network errors, 5xx, and 429 are retried with a small delay to separate a
transient blip from a real outage. Uses only the standard library. This script
sends GET requests only to reference URLs that the repository's **public
documentation already points at**, to confirm they are alive; it never scans or
probes any target.

Known exceptions (allowlist)
If a URL listed in `scripts/external-links-allowlist.txt` comes back DOWN, it is
classified separately as "ALLOWED" and excluded from the strict exit code. This
transparently records dead links inherited by files we do not own and cannot fix
(for example the upstream `CHANGELOG.zh.md`), so a periodic strict check does not
stay red forever because of that one link and turns red **only when a new link
breaks**.

Running
- Default (report, exit 0): `python3 -I scripts/check-external-links.py`
- Periodic/release check, not a merge gate (red when a newly broken link exists):
  `--strict` (exit 1 if any DOWN is not in the allowlist). RESTRICTED and ALLOWED
  are not failures even under strict.
- Preview the extracted set without the network: `--list` (prints the checked URLs
  and their sources without calling them).
"""
import argparse
import http.client
import os
import re
import socket
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.request
from collections import defaultdict
from urllib.parse import urlsplit

# [text](url) body links (not images) and ![alt](url) images. The path runs to the
# first space or closing parenthesis.
MD_LINK = re.compile(r"(?<!\!)\[[^\]]*\]\(([^)\s]+)")
MD_IMAGE = re.compile(r"!\[[^\]]*\]\(([^)\s]+)")
# <https://...> autolinks.
AUTOLINK = re.compile(r"<(https?://[^>\s]+)>")
# Bare URLs in prose. Trailing punctuation is stripped below.
BARE_URL = re.compile(r"https?://[^\s)>\]\"'`]+")
# Inline code spans `...` (single backtick). Removed before extraction.
INLINE_CODE = re.compile(r"`[^`]*`")
# Punctuation commonly attached to the end of a prose URL.
TRAILING_PUNCT = ".,;:!?\"'»)]}>"

# IPv4 private/loopback/link-local ranges (172.16–31. is a separate pattern).
PRIVATE_IPV4 = re.compile(r"^(127\.|10\.|192\.168\.|169\.254\.|0\.0\.0\.0$)")
PRIVATE_IPV4_172 = re.compile(r"^172\.(1[6-9]|2\d|3[01])\.")

BROWSER_UA = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36"
)

# Status codes where the host is alive but the checking method is blocked by the
# server's policy (not a broken link).
RESTRICTED_CODES = {401, 403, 405, 429}

ALLOWLIST_PATH = os.path.join("scripts", "external-links-allowlist.txt")


def load_allowlist(root: str) -> set:
    """Read the known, unfixable DOWN URLs (empty set when the file is absent).

    One URL per line. Text after `#` is a comment; blank and comment-only lines
    are ignored.
    """
    path = os.path.join(root, ALLOWLIST_PATH)
    allow = set()
    if not os.path.exists(path):
        return allow
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.split("#", 1)[0].strip()
            if line:
                allow.add(line)
    return allow


def strip_code(line: str) -> str:
    """Remove inline code spans from one non-fenced line (URLs inside are not references)."""
    return INLINE_CODE.sub(" ", line)


def is_checkable(url: str) -> bool:
    """Filter out reserved/placeholder hosts, leaving external URLs worth checking."""
    parts = urlsplit(url)
    if parts.scheme not in ("http", "https"):
        return False
    host = parts.hostname
    if not host:
        return False
    host = host.lower()
    if host == "localhost" or host.endswith(".localhost"):
        return False
    if host == "::1":
        return False
    if PRIVATE_IPV4.match(host) or PRIVATE_IPV4_172.match(host):
        return False
    if host in ("example.com", "example.org", "example.net", "example.edu"):
        return False
    if host.endswith((".example.com", ".example.org", ".example.net", ".example")):
        return False
    if host.endswith((".test", ".invalid", ".local", ".localdomain", ".tld")):
        return False
    if "." not in host:  # a bare non-FQDN name (for example target)
        return False
    return True


def extract(root: str, md_files: list) -> dict:
    """Build a map of external URL → [(file, line), ...] from tracked Markdown."""
    found = defaultdict(list)
    for md in md_files:
        with open(os.path.join(root, md), encoding="utf-8", errors="ignore") as fh:
            lines = fh.readlines()
        in_fence = False
        for lineno, raw in enumerate(lines, 1):
            stripped = raw.lstrip()
            if stripped.startswith("```") or stripped.startswith("~~~"):
                in_fence = not in_fence
                continue
            if in_fence:
                continue
            line = strip_code(raw)
            candidates = []
            for pattern in (MD_LINK, MD_IMAGE, AUTOLINK):
                candidates.extend(m.group(1) for m in pattern.finditer(line))
            for m in BARE_URL.finditer(line):
                candidates.append(m.group(0).rstrip(TRAILING_PUNCT))
            for url in candidates:
                url = url.strip().rstrip(TRAILING_PUNCT)
                if is_checkable(url):
                    where = (md, lineno)
                    if where not in found[url]:
                        found[url].append(where)
    return found


def probe(url: str, timeout: float, retries: int, delay: float):
    """GET the URL with a browser UA and return (classification, detail).

    The classification is "OK", "RESTRICTED", or "DOWN". Network errors, 5xx, and
    429 are retried.
    """
    req = urllib.request.Request(
        url,
        method="GET",
        headers={
            "User-Agent": BROWSER_UA,
            "Accept": "*/*",
            "Accept-Language": "en,ko;q=0.8",
        },
    )
    last = ""
    for attempt in range(retries + 1):
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                code = resp.getcode()
                if code in RESTRICTED_CODES:
                    return "RESTRICTED", f"HTTP {code}"
                return "OK", f"HTTP {code}"
        except urllib.error.HTTPError as exc:
            code = exc.code
            if code in RESTRICTED_CODES:
                return "RESTRICTED", f"HTTP {code}"
            if code >= 500 or code == 408:
                last = f"HTTP {code}"  # possibly transient; retry
            else:
                return "DOWN", f"HTTP {code}"  # 404/410 etc. are definitive; no retry
        except (
            urllib.error.URLError,
            http.client.HTTPException,
            ssl.SSLError,
            socket.timeout,
            ConnectionError,
            TimeoutError,
            OSError,
        ) as exc:
            reason = getattr(exc, "reason", exc)
            last = f"{type(exc).__name__}: {reason}"
        if attempt < retries:
            time.sleep(delay * (attempt + 1))
    return "DOWN", last


def main() -> int:
    ap = argparse.ArgumentParser(description="Check liveness of external links in tracked Markdown")
    ap.add_argument("--strict", action="store_true",
                    help="exit 1 if any DOWN exists (periodic/release check)")
    ap.add_argument("--list", action="store_true",
                    help="print the checked URLs and their sources without network calls")
    ap.add_argument("--timeout", type=float, default=15.0, help="request timeout in seconds")
    ap.add_argument("--retries", type=int, default=2, help="retries for network errors, 5xx, and 429")
    ap.add_argument("--delay", type=float, default=0.5, help="base delay between calls and retries (seconds)")
    args = ap.parse_args()

    root = subprocess.check_output(
        ["git", "rev-parse", "--show-toplevel"], text=True
    ).strip()
    tracked = subprocess.check_output(["git", "ls-files"], cwd=root, text=True).splitlines()
    md_files = [f for f in tracked if f.endswith(".md")]

    allow = load_allowlist(root)
    found = extract(root, md_files)
    urls = sorted(found)
    print(f"Collected {len(urls)} external URLs to check from {len(md_files)} tracked Markdown files")

    if args.list:
        for url in urls:
            where = ", ".join(f"{f}:{ln}" for f, ln in found[url])
            tag = " [allowlist]" if url in allow else ""
            print(f"  {url}{tag}  ({where})")
        return 0

    results = {"OK": [], "RESTRICTED": [], "ALLOWED": [], "DOWN": []}
    for i, url in enumerate(urls):
        if i:
            time.sleep(args.delay)  # small delay between calls (avoids sandbox throttling)
        verdict, detail = probe(url, args.timeout, args.retries, args.delay)
        if verdict == "DOWN" and url in allow:
            verdict = "ALLOWED"  # a known upstream-inherited DOWN we cannot fix
        results[verdict].append((url, detail))
        print(f"  [{verdict:10}] {url} — {detail}")

    print(
        f"\nSummary: OK {len(results['OK'])} · RESTRICTED {len(results['RESTRICTED'])} · "
        f"ALLOWED {len(results['ALLOWED'])} · DOWN {len(results['DOWN'])}"
    )
    if results["RESTRICTED"]:
        print("RESTRICTED (host alive; checking method blocked, not a broken link):")
        for url, detail in results["RESTRICTED"]:
            print(f"  {url} — {detail}")
    if results["ALLOWED"]:
        print("ALLOWED (known DOWN in the allowlist — not fixable here, excluded from strict):")
        for url, detail in results["ALLOWED"]:
            where = ", ".join(f"{f}:{ln}" for f, ln in found[url])
            print(f"  {url} — {detail}  (referenced by: {where})")
    if results["DOWN"]:
        print("DOWN (likely broken — needs attention):")
        for url, detail in results["DOWN"]:
            where = ", ".join(f"{f}:{ln}" for f, ln in found[url])
            print(f"  {url} — {detail}  (referenced by: {where})")

    if args.strict and results["DOWN"]:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
