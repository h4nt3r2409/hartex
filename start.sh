#!/bin/sh
# ARTEX daemon start script (Linux / macOS / Docker ENTRYPOINT)
#
# Usage:
#   ./start.sh                       Run in the foreground (Ctrl-C to stop)
#   nohup ./start.sh >artex.log 2>&1 &   Run in the background
#   ./start.sh -addr :9000           Additional arguments are passed through to artex
#
# This script does one thing: it runs artex, and when the process exits it decides whether to relaunch based on the exit code.
#
#   0      Normal user stop       → exit the loop
#   75     Program requests restart  → relaunch immediately (the user clicked "one-click update" or "rollback" in the UI)
#   other  Abnormal exit          → back off and relaunch (1→2→4… up to 60 seconds)
#
# Download, SHA256 checksum verification, and version swapping are deliberately not done here. That logic would have to be written twice, in sh and bat,
# and it is precisely the part that must never be wrong. If the binary is ever replaced with one that will not run, this script will faithfully
# keep relaunching it, and the user has no choice but to log in to the machine and recover manually. So verification and swapping all live in Go (the selfupdate package),
# letting artex handle them on startup, while keeping the script simple.
set -u

cd "$(dirname "$0")" || exit 1

BIN=./artex
[ -x "$BIN" ] || { echo "[artex] Executable not found: $BIN" >&2; exit 1; }

RESTART_CODE=75
MAX_DELAY=60

child=0
stopping=0

# Forward the stop signal to the artex process itself.
#
# This is essential under Docker. docker stop sends SIGTERM only to PID 1 (this script)
# and not to child processes. Without forwarding, artex never receives the signal, cannot shut down gracefully,
# and is force-killed with SIGKILL after 10 seconds, interrupting in-progress work.
forward() {
	stopping=1
	if [ "$child" -ne 0 ]; then
		kill -TERM "$child" 2>/dev/null || true
	fi
}
trap forward INT TERM

delay=1
while :; do
	"$BIN" "$@" &
	child=$!

	# A signal interrupts wait and makes it return a value greater than 128. At that point the child process is still shutting down gracefully,
	# so wait once more to get the real exit code.
	wait "$child"
	code=$?
	if [ "$code" -gt 128 ]; then
		wait "$child"
		code=$?
	fi
	child=0

	if [ "$stopping" -eq 1 ]; then
		echo "[artex] Stopped"
		exit 0
	fi

	case "$code" in
		0)
			echo "[artex] Exited normally"
			exit 0
			;;
		"$RESTART_CODE")
			# An update/rollback is ready. Relaunching lets artex complete the version swap on startup (see selfupdate.Bootstrap).
			echo "[artex] Restart requested (applying new version)…"
			delay=1
			;;
		*)
			echo "[artex] Abnormal exit (code=$code), restarting in ${delay}s" >&2
			sleep "$delay"
			delay=$((delay * 2))
			[ "$delay" -gt "$MAX_DELAY" ] && delay=$MAX_DELAY
			;;
	esac
done
