// Package selfupdate implements ARTEX's one-click page update: pull a new
// binary from a GitHub Release, verify it, stage it, and atomically swap it in on next start.
//
// Division of labor (see start.sh / start.bat):
//
//	startup script = dumb supervisor loop, only responsible for "after the process exits, decide by exit code whether to relaunch"
//	this package   = all error-prone logic (download / SHA256 verification / smoke test / swap / failure rollback)
//
// The swap is in Go rather than the script because sha256 verification and the smoke test would have to be
// written twice for sh and bat (sha256sum / shasum / certutil), and this is exactly the link that can least afford errors—swap in a
// binary that won't run and the supervisor will faithfully relaunch it over and over; the user can only rescue things by hand on the machine.
//
// A complete upgrade goes through three process starts:
//
//	① old server receives /api/update/apply → download+verify → stage artex.new → exit 75
//	② script relaunches the old version → Bootstrap finds artex.new → verify+smoke → swap → exit 75
//	③ script relaunches, now the new version → Bootstrap records one attempt → clears the marker after a successful start
//
// Any failing step falls back to the old version: ② if verification fails, delete the staged file and keep running the old version; ③ if it fails to live long enough to
// clear the marker 3 times in a row (crashes as soon as it starts), automatically swap artex.old back.
package selfupdate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ExitRestart is the exit code meaning "please have the supervisor relaunch me" (EX_TEMPFAIL). When the startup script sees it
// it reruns immediately, without counting toward crash backoff. 0 means the user stopped normally (the script exits its loop); anything else is treated as a crash.
const ExitRestart = 75

// maxAttempts is the number of start attempts allowed after a swap. Each start of the new version increments the counter; if it survives
// settleDelay the marker is cleared; crashing maxAttempts times in a row means the new version simply won't start, so it rolls back automatically.
const maxAttempts = 3

// Paths covers all files involved in one upgrade, all under the **directory containing the executable**.
// CWD is deliberately not used: when running as a service the working directory may be / or arbitrary, and using CWD would put staged files
// elsewhere, making the swap logic fail outright.
type Paths struct {
	Dir     string // directory containing the executable
	Current string // currently running binary        artex      / artex.exe
	New     string // staged new version              artex.new  / artex.new.exe
	Sum     string // sha256 of the new version (hex) artex.new.sha256 / artex.new.exe.sha256
	Old     string // old version backed up before the swap artex.old  / artex.old.exe
	Marker  string // upgrade state marker            artex.upgrade.json
}

// ResolvePaths derives all upgrade paths from the current executable.
//
// On Windows .new/.old must also carry the .exe suffix, otherwise the smoke test and post-swap execution will fail,
// so strip the suffix first, then concatenate, so the naming is symmetric on both platforms.
func ResolvePaths() (Paths, error) {
	exe, err := os.Executable()
	if err != nil {
		return Paths{}, fmt.Errorf("failed to determine executable location: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	name := filepath.Base(exe)
	ext := filepath.Ext(name) // On Windows it is ".exe"; on Unix usually empty
	stem := strings.TrimSuffix(name, ext)

	join := func(suffix string) string { return filepath.Join(dir, stem+suffix+ext) }
	return Paths{
		Dir:     dir,
		Current: exe,
		New:     join(".new"),
		Sum:     join(".new") + ".sha256",
		Old:     join(".old"),
		Marker:  filepath.Join(dir, stem+".upgrade.json"),
	}, nil
}

// marker records the progress of a swap, used to trigger automatic rollback when the new version won't start.
type marker struct {
	From     string `json:"from"`     // version before the upgrade
	To       string `json:"to"`       // target version
	Attempts int    `json:"attempts"` // number of start attempts made after the swap
	StagedAt int64  `json:"staged_at"`
}

func readMarker(path string) (marker, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return marker{}, false
	}
	var m marker
	if json.Unmarshal(b, &m) != nil {
		return marker{}, false
	}
	return m, true
}

func writeMarker(path string, m marker) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// cleanStaged removes the staged files. A successful swap, failed verification, and user cancellation all go through it, preventing a leftover
// artex.new from being retried on the next start.
func cleanStaged(p Paths) {
	_ = os.Remove(p.New)
	_ = os.Remove(p.Sum)
}

// CompareVersions compares two version numbers, returning -1/0/1 (a<b / a==b / a>b).
// ok=false means at least one side is not a comparable version number (e.g. "dev" from a local development build, or
// "0.3.7-2-gabc1234-dirty" produced by git describe); callers should then disable one-click update,
// otherwise they would "upgrade" a development build to a release and overwrite uncommitted changes.
func CompareVersions(a, b string) (int, bool) {
	av, aok := parseVersion(a)
	bv, bok := parseVersion(b)
	if !aok || !bok {
		return 0, false
	}
	for i := range 3 {
		if av[i] != bv[i] {
			if av[i] < bv[i] {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

// parseVersion parses version strings in the form "v0.3.7" / "0.3.7" into [3]int.
//
// Only pure three-part versions are accepted: build.sh uses git describe for non-tag builds, producing
// versions with suffixes like "0.3.7-2-gabc1234"; they must be judged incomparable rather than treated as
// 0.3.7 — otherwise development builds would be misjudged as "already latest" or overwritten by a release.
func parseVersion(s string) ([3]int, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return [3]int{}, false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// InDocker reports whether the process runs inside a container. Under Docker the swap writes to the container's writable layer;
// rebuilding the container with `docker compose up -d` reverts to the version bundled in the image—this is expected behavior
// (at that point the user is already pulling a new image), but the frontend should be able to explain this clearly.
func InDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	b, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	s := string(b)
	return strings.Contains(s, "docker") || strings.Contains(s, "containerd")
}
