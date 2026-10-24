package agent

import (
	"log"
	"path/filepath"

	"github.com/Autumn-27/norma/agentcore"
	"github.com/Autumn-27/norma/noaadapter"
)

// noaWarn returns a diagnostics sink tagging non-fatal noa messages with the
// session, routed through the package logger (agents have no per-instance one).
func noaWarn(session string) func(string) {
	return func(msg string) { log.Printf("[noa] %s: %s", session, msg) }
}

// noa is the "model-driven context compaction" mechanism introduced in norma v0.4.0 and
// is toggled by the user in system settings as a platform experimental feature. It is mutually exclusive with
// built-in compaction: noaadapter.Enable is the only entry point, attaching the context
// taker-over (Compactor), the Compress tool, and three persistent prompt segments in one call; not calling Enable means it is off
// (built-in compaction works as usual). The toggle is resolved by the noaEnabledFn injected into each agent and read once
// per run, so switching only affects runs started afterward and no agent rebuild is needed.

// enableNoa wires noa into opts when the resolver reports it enabled. archiveRoot is the base directory where compacted original text is persisted
// (it takes the global workDir; all agents uniformly land under <workDir>/noa rather than being scattered per task/intent directory), and sessionID
// names the archive subdirectory beneath it (globally unique, so there are no collisions within the same base directory).
//
// noa is an experimental feature: a failed hookup must not interrupt a real task. On error it is reported via onWarn and falls back to built-in compaction.
// On successful enable, opts.Compaction is cleared to avoid agentcore warning that "two context managers are set at the same time".
func enableNoa(opts *agentcore.Options, enabled func() bool, archiveRoot, sessionID string, onWarn func(string)) {
	if enabled == nil || !enabled() {
		return
	}
	if opts.OnWarn == nil {
		opts.OnWarn = onWarn
	}
	if err := noaadapter.Enable(opts, noaadapter.Options{
		ArchiveBaseDir: filepath.Join(archiveRoot, "noa"),
		SessionID:      sessionID,
		OnWarn:         onWarn,
	}); err != nil {
		if onWarn != nil {
			onWarn("noa compaction enable failed, falling back to built-in compaction: " + err.Error())
		}
		return
	}
	// Compactor overrides Compaction, but when both coexist agentcore warns every time; clear it explicitly.
	opts.Compaction = nil
}
