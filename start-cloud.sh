#!/usr/bin/env bash
set -euo pipefail

# Render assigns the PORT environment variable arbitrarily, so pass it as an argument so the app listens on that port
# Keep the default execution and skill/data paths
: "${PORT:?PORT environment variable must be set}"
echo "[*] Starting ARTEX Server on port :$PORT"
exec ./artex -addr ":$PORT"
