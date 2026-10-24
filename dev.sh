#!/usr/bin/env bash
# Development mode: runs the backend (127.0.0.1:8787) + traffic proxy (:8788) together with the frontend next dev (:5173).
# Frontend /api is reverse-proxied to the backend. Ctrl-C stops both.
#
# For the single-binary (embedded frontend) approach, see the "Single binary" section of the README. This script is not used for it.
set -euo pipefail
cd "$(dirname "$0")"

# On exit, terminate every child process in this process group (backend + frontend).
cleanup() { kill 0 2>/dev/null || true; }
trap cleanup EXIT INT TERM

# Backend (plain go run, no embedded frontend). The number of concurrent work agents is set in "System settings".
go run ./cmd/artex -addr 127.0.0.1:8787 -proxy 127.0.0.1:8788 &

# Frontend hot reload (Vite/Next dev server; /api is reverse-proxied to :8787).
( cd web && npm run dev ) &

echo "[dev] backend :8787 / proxy :8788 / frontend http://localhost:5173  (Ctrl-C to stop)"
wait
