# Maintaining the English-only edition

This document is the maintainer runbook for the **English-only global edition** of Hartex. It covers
upstream tracking and the checks that keep the repository English-only. Contribution scope, legal
responsibility, and the English-only policy live in [CONTRIBUTING.md](CONTRIBUTING.md); user-facing
guidance lives in [README.md](README.md).

The goal of this edition can be summed up in one sentence: **one language in the product, the
source tree, the documentation, and the shipped assets.** English is the only UI and output
language, the agent prompts and developer documentation are English, and the translated documents
and catalogs have been removed. The checks below keep it that way.

---

## 1. The English-only structure at a glance

This repository **forks** upstream ARTEX and holds the English-only adaptation on top of its
history. The historical base is preserved in this repository's history (see section 3).

- **One UI catalog.** `web/messages/en.json` is the only message catalog; the Korean, Chinese,
  Spanish, and Japanese catalogs, the language switcher, and the client-side locale store are gone.
- **English backend literals.** The Go server has no output-language setting, catalog, resolver, or
  API field. Backend messages are English literals.
- **A fixed English output directive.** `outputDirective()` in `agent/prompt.go` (backed by
  `textpolicy.EnglishOutputDirective`) is appended after the rendered body for the six user-facing
  agent roles and always to the wrap-up prompt. It is code-owned, so a database-edited prompt cannot
  remove it. The same English-only rule applies to side-question answers and the intercept judge's
  comment.
- **English documentation and assets.** The core documents, detection READMEs, guides, screenshots,
  and release assets are English. The translated Markdown, non-English issue templates, and
  Chinese-UI screenshots were deleted.
- **A small functional allowlist.** A documented set of non-English machine values remains because
  removing it would break persisted data, old conversations, provider error detection, DB upgrades,
  detection rules, or the guards. The list is in
  [CONTRIBUTING.md § Functional allowlist](CONTRIBUTING.md#functional-allowlist-design-7).

---

## 2. English assets and verbatim values

The agents' reasoning prompts (the "brain" bodies in `agent/promptcatalog.go` and the DB seed
`agent_prompts`), source comments, scripts, CI configuration, and developer documentation are in
English. When an upstream change adds or edits Chinese prompt text, **translate it into English as
part of the upstream sync**; do not reintroduce Chinese into the agent brain, comments, or developer
docs.

The following stay verbatim and must not be translated:

- **Command, payload, code, URL, identifier, path, and log originals.** These are the exact inputs a
  human needs to reproduce a finding, so they are preserved byte-for-byte. This includes tool names,
  JSON keys, placeholders such as `{{.Goal}}`, and `fmt` verbs.
- **Wire, database, and protocol values.** Enum values, status strings, sentinel tokens, channel
  kinds, and API field names are machine values; leave them unchanged.
- **The functional allowlist in [CONTRIBUTING.md](CONTRIBUTING.md#functional-allowlist-design-7).**
  These are data, wire values, upstream API text, detection patterns, or encoding support, not
  translation. Keep them as-is and keep their explanatory comments.

---

## 3. The upstream tracking base

Upstream `Autumn-27/ARTEX` has been removed from GitHub (404, checked 2026-10-24), so it can no
longer be fetched. **There is nothing to sync until a reachable mirror appears.** The historical
upstream base is preserved in this repository's own history and on the
`backup/pre-squash-2026-10-24` branch. If you obtain a reachable mirror, add it under the `upstream`
name; otherwise sections 4–7 apply only to the historical base below.

```bash
# Optional: only if you have a reachable mirror of upstream.
git remote add upstream <mirror-url>
git remote -v
```

The upstream base commit at which the English-only adaptation started is:

- **Base = `d003372`** (upstream `main`, 2026-10-03, merge of PR #189 `fix/sse-same-origin`).

This value means "every upstream change up to this commit is already folded into this repository."
Each time you apply a new upstream change, update this base using the method in section 7.

---

## 4. The procedure if a reachable mirror appears

### 4.1 Fetch upstream and inspect the difference

```bash
git fetch upstream
git rev-list --count d003372..upstream/main        # number of unapplied upstream commits
git log --oneline d003372..upstream/main           # list of unapplied commits
```

`git fetch` only updates upstream's remote-tracking branch, so it leaves the working tree and `HEAD`
untouched. If the count of unapplied commits is 0, you are in sync with upstream and there is nothing
more to do.

### 4.2 Classify the changed files

See which files the unapplied commits touched, and split them into English verbatim values, English
translation targets, and detection indicators.

```bash
git log --name-status --oneline d003372..upstream/main
```

The classification criteria are as follows.

- If `agent/promptcatalog.go` / the `agent_prompts` seed, or any other prompt or documentation file,
  changed and contains Chinese prose → **translate the prose into English**. Commands, payloads,
  identifiers, wire values, and the functional allowlist stay verbatim.
- If Go backend logic (`db/`, `llmrec/`, `server/`, and so on) changed → apply the logic as is. Write
  any new user-facing strings as English literals.
- If the UI (`web/src/**`) changed and introduced **new screen strings** → do not hard-code them;
  add them under the same key to `web/messages/en.json`.
- If **upstream indicators pinned by the detection rules** changed (the prober User-Agent in
  `enrich/enrich.go`, the self-update User-Agent in `selfupdate/`, the audit marker in
  `guard/guard.go`, the destructive-command deny list in `db/db.go`, the default listen and
  recording-proxy ports in `cmd/artex/main.go`) → bring the Sigma/Suricata rules and the ATT&CK layer
  in `detections/`, as well as the values in `detections/indicators/artex_indicators.csv`, in line
  with the new values. These indicators are not translation targets but the **basis of detection**,
  so when upstream changes a value, the rules silently go stale. The indicator-match test in section
  5.3 catches that mismatch automatically.

### 4.3 Apply

Merge or cherry-pick feature by feature, translate any upstream Chinese prompt text into English,
and add new UI strings to `web/messages/en.json`. Always run the checks in section 5 right after
applying.

> **There is no upstream data to sync today.** Upstream is unreachable, so this section is a
> contingency procedure, not a routine task. The historical base and backup branch above are the
> reference if a mirror appears.

---

## 5. English-only and drift checks

After applying an upstream change, run the checks below.

### 5.1 Source language

`scripts/check-source-language.py` is the English-only source guard. It scans the tracked source
tree (code, scripts, CI, tests, and developer documentation) and fails on non-English text; only
generated `graphify-out/` and change-process records under `docs/superpowers/` are skipped. Run it
before merging:

```bash
python3 -I scripts/check-source-language.py
```

The functional allowlist in [CONTRIBUTING.md](CONTRIBUTING.md#functional-allowlist-design-7) is the
only content exception. Do not add broad exemptions to make a change pass.

### 5.2 Static export

After exporting the UI statically, any CJK in the prerendered HTML means non-English text leaked into
the product. CI enforces this with `scripts/check-web-cjk.py`, which detects Han, Hangul, and Kana.

```bash
cd web && npm ci && NEXT_EXPORT=1 npm run build   # generates out/
python3 -I ../scripts/check-web-cjk.py
```

### 5.3 Make sure the detection indicators still match the upstream source

The rules in `detections/` are based on the strings that upstream actually emits (prober User-Agent,
self-update User-Agent, audit marker, destructive-command deny list). When an upstream re-sync changes
these values, the English-only checks still pass while only the shipped rules silently stop matching. The
test below verifies bidirectionally that each indicator is still present in both the upstream source
and the rules, so run it after a re-sync.

```bash
detections/tests/indicators/run.sh   # runs in isolation under Docker; RESULT: PASS means a match
```

On failure it prints which indicator is out of sync and in which direction (whether the upstream
source changed or the rule changed), so bring the rules and the layer in line with the new values per
the last classification criterion of 4.2. This test also runs automatically in the repository CI
([`.github/workflows/detections.yml`](.github/workflows/detections.yml)) on every push/PR that changes
the rule tree or the upstream source files above, catching re-sync drift at the merge gate.

If you add a new indicator and in doing so **pin a new upstream source file** (as when adding the port
indicator from `cmd/artex/main.go`, for example), you must also add that file to the `push` and
`pull_request` `paths` filters of the workflow above. If you miss it, a PR that changes only that
source will not trigger the indicator test, and the drift will silently pass the merge gate. The
indicator test checks this synchronization itself (its fifth check, "CI triggers this test when any
pinned source changes"): if any non-`detections/` source the test reads is not listed in both `paths`
blocks, the test fails, so source pinning and CI trigger conditions cannot be merged out of sync.

### 5.4 When bumping the detection-test tool pins

The detection tests run `sigma-cli`, the SigmaHQ validator plugin (`pySigma-validators-sigmahq`), and
the Suricata image at fixed versions (the defaults in each `run.sh`, overridable via environment
variables). Bumping these pins can introduce **tool-side drift** rather than upstream-source drift. In
particular, the SigmaHQ validator adds new convention checks with each release, so
`detections/tests/sigma_lint/run.sh` may surface new issues in red. When that happens, bring the rules
in line with the new convention, or — if the convention does not fit a standalone rule set — record
the reason and add it to the exclusion list in
[`detections/tests/sigma_lint/validators.yml`](detections/tests/sigma_lint/validators.yml). If a
backend plugin changes its support, the `sigma_backends` test gives the same signal.

---

## 6. Finishing with build and test verification

After applying an upstream change, verify the backend and frontend per the
[development environment procedure in CONTRIBUTING.md](CONTRIBUTING.md#development-environment).
If you do not have Go locally, you can run the same thing under Docker.

```bash
docker run --rm -v "$PWD":/src -w /src \
  -v artexko-gomod:/go/pkg/mod -v artexko-gocache:/root/.cache/go-build \
  golang:1.26 sh -c 'go build ./... && go vet ./... && go test ./... -count=1'
```

---

## 7. Base-update record

Once you have applied an upstream change and finished verifying it, **update the base commit value in
section 3 of this document to the new upstream commit** and include that change in the same commit or
a following one. Doing so lets the next maintainer confirm "how far things have been applied" in this
one place.

Commit messages follow the [commit-message rules in CONTRIBUTING.md](CONTRIBUTING.md#commit-messages)
and are written in English. For example, an upstream-sync commit is written like this:

```
chore(upstream): sync upstream d003372..b55ceb1 (intercept token accounting) + translate new prompt text
```

---

## 8. Review and verification rules of thumb (common pitfalls)

Here are two pitfalls maintainers repeatedly fall into when checking upstream applies and
documentation improvements. Both are cases where "the checking method itself is wrong, so you
mistake something healthy for broken," so they are fixed here as rules of thumb to prevent unnecessary
reverts.

### 8.1 Check the repository's CI status by specifying the repository

This repository was forked from upstream ARTEX. If you still have an `upstream` remote registered
(see section 3), an unqualified `gh` command can **pick the upstream repository as the default** and
show you run results from a repository that does not have our workflows. You can then see upstream
CI green and **mistakenly think our CI passed**, or judge our workflows (`ci.yml`,
`detections.yml`) as "HTTP 404 ... not found" by mistake.

So when checking CI, always name the repository explicitly.

```bash
gh run list -R h4nt3r2409/hartex --workflow ci.yml --limit 5
gh run list -R h4nt3r2409/hartex --workflow detections.yml --limit 5
```

Once set, you can make `gh` default to our repository even when you omit `-R`. Note, however, that
this setting is a **local gh setting** and is not committed to the repository, so you must set it
again on a new machine or a new checkout.

```bash
gh repo set-default h4nt3r2409/hartex
gh repo set-default --view   # check that h4nt3r2409/hartex shows up
```

### 8.2 Check external links in documents with GET, like a browser

Section 7 of the defense guide ([`docs/defense-en.md`](docs/defense-en.md)) carries links to Korean official channels (boho.or.kr,
fsec.or.kr, pipc.go.kr). When checking whether these links are alive, using only `curl -I` (a HEAD
request) or the default User-Agent will **mistake a healthy link for a broken one**. Korean public and
security agency sites refuse a simple check for three reasons.

- **They reject HEAD requests.** For example, fsec.or.kr returns 400 to `curl -I` (HEAD).
- **They block the default `curl` User-Agent.** fsec.or.kr and pipc.go.kr return 400 even to GET
  requests sent with the default UA (they return 200 when sent with a browser UA).
- **They redirect to a different address.** pipc.go.kr redirects twice, from `www.pipc.go.kr` to
  `pipc.go.kr/np/`, so if you do not follow redirects you miss the final status.

So check links **with a browser User-Agent, with GET, following redirects**.

```bash
UA='Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36'
for u in https://www.boho.or.kr https://www.fsec.or.kr https://www.pipc.go.kr; do
  curl -sS -L -A "$UA" -o /dev/null -w "$u -> %{http_code} %{url_effective}\n" "$u"
done
```

A final status code of 200 means the link is valid. If the status code comes back 400 or 403, first
suspect that the link is not broken but that **your checking method was blocked by the server's access
policy**, and re-check by eliminating factors one at a time: HEAD, the default UA, and not following
redirects. (As of the 2026-10-06 check, all three links return 200 with the method above, with
pipc.go.kr returning 200 after two redirects.)

This manual procedure is automated as-is by `scripts/check-external-links.py`. It gathers the external
links outside code fences and inline code from every tracked `.md` (excluding reserved and placeholder
hosts), checks their status with a browser UA, GET, and redirect-following as above, and retries on
network errors, 5xx, and 429 to separate transient flakes from real outages. It sorts results into four
classes: OK (2xx/3xx) · RESTRICTED (401/403/405/429 — the host is alive, only the checking method is
blocked) · ALLOWED (a known upstream-inherited dead link we cannot fix, listed in
`scripts/external-links-allowlist.txt`) · DOWN (404/410/5xx/connection error — likely broken).

- Preview the targets without the network: `python3 -I scripts/check-external-links.py --list`
- Release/periodic check (exits non-zero on a newly broken link): `python3 -I scripts/check-external-links.py --strict`

External-link liveness is flaky, so it is **not a merge gate**. Instead the non-blocking
[`external-links`](.github/workflows/external-links.yml) workflow runs `--strict` every Monday and on
manual dispatch, turning red when a DOWN not on the allowlist newly appears. Dead links inherited by
upstream-preserved files (for example a vanished contributor account credited in the upstream release
history, now kept only in git history) are ones we cannot fix, so they go on the allowlist with a
reason and are excluded from the strict check.

---

## 9. The release pipeline

Pushing a version tag (`v*`) makes [`.github/workflows/release.yml`](.github/workflows/release.yml)
build binaries for five platforms and, when a condition is met, a multi-architecture Docker image.
This fork has never cut a release tag, so this workflow has never run. This section therefore records
what the pipeline assumes and produces, and whether those assumptions match the current repository
structure. Because pushing a tag creates a GitHub Release on the public repository, cut a release only
after the publishing decision is made.

### 9.1 How to cut a release

Pushing a tag that starts with `v` fires the workflow.

```bash
git tag v0.3.15
git push origin v0.3.15
```

### 9.2 What the pipeline does

The workflow is split into five jobs.

- **frontend.** Statically exports the frontend once (`web/out`) and uploads that output as the
  `web-dist` artifact. The binaries job below downloads and reuses this output per target.
- **binaries.** Cross-compiles five targets (linux amd64/arm64, darwin amd64/arm64, windows amd64) and
  packages a zip per target. On the linux amd64 binary it runs an `artex -h` smoke test to confirm the
  binary actually runs.
- **release.** Gathers all zips, generates a `SHA256SUMS` checksum file, and creates a GitHub Release
  with the zips and the checksum attached.
- **docker.** Takes the linux binaries cross-compiled by binaries, builds a multi-architecture
  (amd64/arm64) image, and pushes it to GitHub Container Registry as `ghcr.io/h4nt3r2409/hartex`
  with the release tag and `latest`. It logs in with the workflow's built-in `GITHUB_TOKEN` (job
  permission `packages: write`), so no extra secrets are needed.

### 9.3 Do the build assumptions match the repository structure

The pipeline runs on the following three assumptions, and all three were confirmed to match the
current repository structure by reproducing the binaries job locally.

- **Frontend embed.** The binaries job receives the `web-dist` (the contents of `web/out`) uploaded by
  the frontend job into `server/webui/dist`, and `//go:embed all:webui/dist` in `server/webui_embed.go`
  embeds that location into the binary. The binaries job therefore does not rebuild the frontend; it
  calls [`build.sh`](build.sh) with `ARTEX_SKIP_FRONTEND=1`.
- **Binary and package paths.** `build.sh --target <os>/<arch>` produces the `dist/artex-<os>-<arch>/artex`
  binary and a zip package under `dist/`. The zip contains the binary together with a start script
  (`start.sh` on Linux/macOS, `start.bat` on Windows), `skills/`, `config.example.json`, and
  `README.md`.
- **Copying the binary into the Docker image.** The binaries job uploads the linux binary separately as
  the `bin-linux-<arch>` artifact, and the docker job receives it as `dist/<arch>/artex`. The
  `COPY dist/${TARGETARCH}/artex` in [`Dockerfile`](Dockerfile) picks up that path via the `TARGETARCH`
  that buildx fills in per platform during a multi-architecture build. [`.dockerignore`](.dockerignore)
  does not exclude `dist/`, so the binary is included in the build context.

### 9.4 The Docker image namespace

Images are published to `ghcr.io/h4nt3r2409/hartex`, a namespace this repository owns (the repository
was renamed to `h4nt3r2409/hartex` on 2026-10-24), and `docker-compose.yml` pulls that image.
Upstream's `autumn27/artex` has been taken down from Docker Hub and this repository has no push access
to it, so it is no longer used. A GHCR package may be created as private on its first publish, so after
the first release check in the package settings that it is public; if it is private,
`docker compose pull` fails with an authentication error.

### 9.5 Verifying locally without a tag

To check only the pipeline assumptions without cutting a public release, reproduce the binaries job
locally. If you do not have Go locally, you can run the same thing under Docker.

```bash
# 1) Static frontend export (corresponds to the frontend job in release.yml)
cd web && npm ci && npm run build:static && cd ..
# 2) Place it where the binaries job receives the artifact
rm -rf server/webui/dist && mkdir -p server/webui/dist && cp -a web/out/. server/webui/dist/
# 3) Build one target with the same environment as the binaries job
docker run --rm -v "$PWD":/app -w /app \
  -e ARTEX_SKIP_FRONTEND=1 -e ARTEX_SKIP_NPM_CI=1 \
  -e ARTEX_COMPRESS=0 -e ARTEX_PACKAGE=1 -e ARTEX_PACKAGE_DIR=dist \
  -e ARTEX_BUILD_VERSION=v0.0.0-local \
  golang:1.26 bash -c 'apt-get update && apt-get install -y zip && ./build.sh --target linux/amd64'
# 4) Confirm the outputs: dist/artex-linux-amd64/artex · dist/*.zip · dist/SHA256SUMS
```

`dist/artex-linux-amd64/artex` is a statically linked ELF, and given `-h` it prints usage and exits
with code 0. This is the behavior the binaries job's smoke test confirms. Build outputs
(`dist/`, `server/webui/dist/`) are not committed to the repository (they are excluded by
`.gitignore`).

### 9.6 Building release archives

`build.sh --release` builds and embeds the frontend, strips debug information with the Go linker,
and packages the release files into zips. Release mode produces Linux amd64/arm64, macOS
amd64/arm64, and Windows amd64 by default:

```bash
./build.sh --release
# output: dist/artex-0.3.3-*.zip
```

UPX self-extracting binaries may be incompatible with some Linux kernels, virtualization
environments, or security policies, so UPX is not enabled by default. Override the targets with
`ARTEX_TARGETS`; once you have confirmed the target runtime is compatible, pass `--upx` explicitly
to shrink the binary further:

```bash
ARTEX_TARGETS=linux/amd64,windows/amd64 ./build.sh --release
./build.sh --target linux/amd64 --upx
```
