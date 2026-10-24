package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// smokeEnv makes a subprocess launched by the smoke test skip Bootstrap entirely.
//
// Strictly speaking not adding it would not cause problems either: the subprocess's os.Executable() is artex.new, and
// all derived paths carry the .new prefix, so it cannot touch the real upgrade files. But relying on such a coincidence is too fragile;
// an explicit short-circuit is obvious at a glance and also saves the subprocess a pointless disk probe.
const smokeEnv = "ARTEX_SELFUPDATE_SMOKE"

// Action is the instruction Bootstrap gives main.
type Action int

const (
	// Continue: start the server as usual.
	Continue Action = iota
	// Restart: exit immediately with ExitRestart so the supervisor script relaunches it.
	Restart
)

// State describes the upgrade state at this start, so /api/update/check can truthfully tell the frontend
// "whether the last upgrade succeeded or was rolled back".
type State struct {
	Pending     bool   // not yet confirmed stable after the swap
	RolledBack  bool   // automatic rollback was just performed on this start
	FailedStage bool   // staged file failed verification/smoke test and was discarded
	Detail      string // one-sentence explanation for the user
}

// Bootstrap runs at the very beginning of main and must be called before listening on any port or opening the database.
//
// Three situations:
//
//	① a staged file artex.new exists → verify + smoke; if it passes, swap and request restart; if not, discard and keep running the old version
//	② only the marker file remains     → means the swap just completed, accumulate one attempt; roll back after enough consecutive failures
//	③ nothing                          → normal start
func Bootstrap() (Action, State) {
	if os.Getenv(smokeEnv) != "" {
		return Continue, State{}
	}
	p, err := ResolvePaths()
	if err != nil {
		log.Printf("[update] Skipping bootstrap: %v", err)
		return Continue, State{}
	}

	if _, err := os.Stat(p.New); err == nil {
		return applyStaged(p)
	}

	m, ok := readMarker(p.Marker)
	if !ok {
		return Continue, State{}
	}
	return confirmOrRollback(p, m)
}

// applyStaged handles the "staged file exists" situation: swap if verification passes, discard if it fails.
//
// This is the only place in the entire upgrade chain that overwrites the executable, and also the last gate—the smoke test blocks
// problems such as corrupted downloads, wrong architecture, and missing dynamic links. Once a non-runnable binary is let through,
// the supervisor script will tirelessly relaunch it, and the Go code gets no chance to run, so automatic rollback cannot even begin.
func applyStaged(p Paths) (Action, State) {
	m, _ := readMarker(p.Marker)

	if err := verifyStaged(p); err != nil {
		log.Printf("[update] Staged new version failed verification; discarded, continuing to run the current version: %v", err)
		cleanStaged(p)
		_ = os.Remove(p.Marker)
		return Continue, State{FailedStage: true, Detail: "failed to verify the new version, so it was discarded: " + err.Error()}
	}

	if err := swap(p); err != nil {
		log.Printf("[update] Swap failed; continuing to run the current version: %v", err)
		cleanStaged(p)
		_ = os.Remove(p.Marker)
		return Continue, State{FailedStage: true, Detail: "failed to replace the version: " + err.Error()}
	}

	// Swap succeeded. Keep the marker for the next start (which runs the new version) to confirm stability.
	m.Attempts = 0
	if m.StagedAt == 0 {
		m.StagedAt = time.Now().Unix()
	}
	if err := writeMarker(p.Marker, m); err != nil {
		log.Printf("[update] Failed to write upgrade marker (losing automatic rollback capability): %v", err)
	}
	log.Printf("[update] Swapped to %s, exiting to restart (exit %d)", orUnknown(m.To), ExitRestart)
	return Restart, State{Pending: true}
}

// confirmOrRollback handles the "post-swap start": accumulate attempts, and swap the old version back if the limit is exceeded.
//
// The counter only increments once Go code is running, so it covers faults of the kind "can execute but crashes during initialization"
// (incompatible config, port in use, DB migration blew up); "cannot exec at all" is blocked by the pre-swap
// smoke test, and the two together are what makes it complete.
func confirmOrRollback(p Paths, m marker) (Action, State) {
	m.Attempts++
	if m.Attempts > maxAttempts {
		if err := rollback(p); err != nil {
			// If even rollback fails, don't restart again, or it will fall into an infinite restart loop. Clear the marker
			// and let the process start in its current state—if it won't start, at least the user can see the reason in the logs.
			log.Printf("[update] New version failed to start %d times in a row, and rollback also failed: %v", maxAttempts, err)
			_ = os.Remove(p.Marker)
			return Continue, State{Detail: "failed to start the new version and rollback also failed: " + err.Error()}
		}
		log.Printf("[update] New version failed to start %d times in a row; rolled back to %s, exiting to restart (exit %d)",
			maxAttempts, orUnknown(m.From), ExitRestart)
		_ = os.Remove(p.Marker)
		return Restart, State{RolledBack: true, Detail: fmt.Sprintf("failed to start the new version, so rolled back to the previous version (%s)", orUnknown(m.From))}
	}
	if err := writeMarker(p.Marker, m); err != nil {
		log.Printf("[update] Failed to update upgrade marker: %v", err)
	}
	log.Printf("[update] New version starting (attempt %d/%d); upgrade will be confirmed after it runs stably",
		m.Attempts, maxAttempts)
	return Continue, State{Pending: true}
}

// Settle confirms the new version is running stably and clears the upgrade marker.
//
// Called delayed by main after the HTTP listener comes up: only surviving this period counts; otherwise the marker stays in place,
// and the next start continues accumulating attempts until rollback is triggered.
func Settle() {
	p, err := ResolvePaths()
	if err != nil {
		return
	}
	settle(p)
}

func settle(p Paths) {
	if _, ok := readMarker(p.Marker); !ok {
		return // not a post-upgrade start, nothing to do
	}
	if err := os.Remove(p.Marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("[update] Failed to clear upgrade marker: %v", err)
		return
	}
	log.Printf("[update] New version is running stably; upgrade complete (previous version kept as %s)", p.Old)
}

// SettleDelay is the runtime required to judge that "the new version has survived".
const SettleDelay = 30 * time.Second

// verifyStaged verifies the staged file: first compare SHA256, then actually launch it once.
func verifyStaged(p Paths) error {
	want, err := os.ReadFile(p.Sum)
	if err != nil {
		return fmt.Errorf("failed to read checksum: %w", err)
	}
	got, err := fileSHA256(p.New)
	if err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(string(want)), got) {
		return errors.New("SHA256 does not match (the download is corrupted or tampered with)")
	}
	return smokeTest(p.New)
}

// smokeTest launches the new binary with -h to confirm it can actually execute on the current system.
// This blocks a large class of problems such as truncated downloads, wrong architecture (exec format error), and missing dependencies.
func smokeTest(bin string) error {
	if err := os.Chmod(bin, 0o755); err != nil {
		return fmt.Errorf("failed to grant execute permission: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-h")
	cmd.Env = append(os.Environ(), smokeEnv+"=1")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return errors.New("smoke test timed out (the new binary is not responding)")
	}
	if err != nil {
		snippet := strings.TrimSpace(string(out))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "…"
		}
		return fmt.Errorf("smoke test failed: %v: %s", err, snippet)
	}
	return nil
}

// swap replaces the current binary with the staged new version.
//
// Both Unix and Windows allow renaming a running executable (what Windows forbids is deleting and
// overwriting, not rename), so no platform split is needed here, and there is no need to stop ourselves first.
func swap(p Paths) error {
	// Windows rename won't overwrite an existing target, so the .old left by the previous upgrade must be removed first.
	if err := os.Remove(p.Old); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to clean up previous backup %s: %w", p.Old, err)
	}
	if err := os.Rename(p.Current, p.Old); err != nil {
		return fmt.Errorf("failed to back up current version: %w", err)
	}
	if err := os.Rename(p.New, p.Current); err != nil {
		// The swap failed but the current version has already been moved away; it must be put back as-is, otherwise there will be no executable on the next start.
		if rerr := os.Rename(p.Old, p.Current); rerr != nil {
			return fmt.Errorf("failed to install new version (%v), and restoring the current version also failed: %w", err, rerr)
		}
		return fmt.Errorf("failed to install new version: %w", err)
	}
	_ = os.Remove(p.Sum)
	return nil
}

// rollback swaps back the old version backed up by swap.
func rollback(p Paths) error {
	if _, err := os.Stat(p.Old); err != nil {
		return fmt.Errorf("no backup to roll back to (%s): %w", p.Old, err)
	}
	// Move the non-starting new version to .failed for investigation rather than deleting it directly.
	failed := p.Current + ".failed"
	_ = os.Remove(failed)
	if err := os.Rename(p.Current, failed); err != nil {
		return fmt.Errorf("failed to move failed version: %w", err)
	}
	if err := os.Rename(p.Old, p.Current); err != nil {
		return fmt.Errorf("failed to restore previous version: %w", err)
	}
	return nil
}

// Rollback is the implementation of /api/update/rollback: actively revert to the previous version.
// It only performs the swap; restarting is likewise left to the supervisor script (the caller then exits with ExitRestart).
func Rollback() error {
	p, err := ResolvePaths()
	if err != nil {
		return err
	}
	if _, err := os.Stat(p.Old); err != nil {
		return errors.New("no previous version to roll back to (" + p.Old + " missing)")
	}
	cleanStaged(p)
	if err := smokeTest(p.Old); err != nil {
		return fmt.Errorf("refusing to roll back because the previous version cannot be executed: %w", err)
	}
	// Swap current and backup: after rolling back, it can still be rolled back again.
	tmp := p.Current + ".swap"
	_ = os.Remove(tmp)
	if err := os.Rename(p.Current, tmp); err != nil {
		return fmt.Errorf("failed to move current version: %w", err)
	}
	if err := os.Rename(p.Old, p.Current); err != nil {
		_ = os.Rename(tmp, p.Current)
		return fmt.Errorf("failed to install previous version: %w", err)
	}
	if err := os.Rename(tmp, p.Old); err != nil {
		log.Printf("[update] Failed to tidy up backup after rollback (does not affect operation): %v", err)
	}
	_ = os.Remove(p.Marker)
	return nil
}

// HasBackup reports whether there is a previous version that can be rolled back to, so the frontend can decide whether to show the rollback button.
func HasBackup() bool {
	p, err := ResolvePaths()
	if err != nil {
		return false
	}
	_, err = os.Stat(p.Old)
	return err == nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Unknown version"
	}
	return s
}
