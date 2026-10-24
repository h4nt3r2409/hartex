#!/usr/bin/env bash
# ARTEX update script: 1) Docker update (pull the new image and recreate)  2) local build update (rebuild the binary)
# Pairs with install.sh. install handles the initial installation, update handles upgrading to a new version.
# DB migrations do not need to be run manually. artex re-runs schema.sql idempotently on every startup (including ADD COLUMN/CREATE
# INDEX IF NOT EXISTS), so "a restart is the migration". Data (pgdata volume, ./data, ./skills) is unaffected.
set -euo pipefail
cd "$(cd "$(dirname "$0")" && pwd)"

info(){ printf '\033[36m[*]\033[0m %s\n' "$*"; }
ok(){   printf '\033[32m[+]\033[0m %s\n' "$*"; }
warn(){ printf '\033[33m[!]\033[0m %s\n' "$*"; }
die(){  printf '\033[31m[x]\033[0m %s\n' "$*" >&2; exit 1; }
ask(){  local p="$1" d="${2:-}" a; read -rp "$p${d:+ [$d]}: " a; echo "${a:-$d}"; }

# -- Optional: sync the repository to the latest code (compose, scripts, and local build sources all get updated by this) ------
sync_repo(){
  [ -d .git ] && command -v git >/dev/null 2>&1 || { warn "Not a git working copy, skipping git pull"; return; }
  [ "$(ask 'Fetch the latest code (git pull --ff-only)? (y/n)' y)" = y ] || return
  if ! git pull --ff-only; then
    warn "git pull could not fast-forward (you have local changes or the branch has diverged). Resolve it manually and try again. Using the current code as-is this time"
  fi
}

# -- 1) Docker update ------------------------------
update_docker(){
  command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1 \
    || die "docker / docker compose not found. Install and deploy with ./install.sh first"
  [ -f .env ] || die ".env not found. Complete the initial deployment with ./install.sh first"

  # Optional: upgrade to a specific version tag (leave empty to follow ARTEX_TAG in .env; the default is latest)
  local tag; tag="$(ask 'Target image tag (press Enter to use .env / latest)' '')"
  if [ -n "$tag" ]; then
    if grep -q '^ARTEX_TAG=' .env; then
      sed -i.bak "s|^ARTEX_TAG=.*|ARTEX_TAG=${tag}|" .env && rm -f .env.bak
    else
      printf '\nARTEX_TAG=%s\n' "$tag" >> .env
    fi
    ok "Set ARTEX_TAG to ${tag}"
  fi

  # Only touches artex. postgres is pinned to 16-alpine and does not need upgrading (pulling it would just waste bandwidth,
  # and a major version change carries compatibility risk). artex declares depends_on postgres, so bringing the service up
  # by name starts pg automatically when it is not running, and leaves it as-is without recreating it when it is already up.
  info "Pulling the new image (artex only)..."
  if ! docker compose pull artex; then
    warn "Failed to pull image ghcr.io/h4nt3r2409/hartex (check that the tag is a published release tag and that the network works)"
    die "Image update failed. You can also update via \"2) Local update (recompile with go)\""
  fi
  info "Recreating and starting (artex migrates the schema automatically on restart)..."
  docker compose up -d artex
  ok "Update complete -> http://localhost:8787"
  info "View logs: docker compose logs -f artex"
  info "Prune old images (optional): docker image prune -f"
}

# -- 2) Local build update ------------------------------
update_local(){
  command -v go >/dev/null 2>&1 || die "Go not found (>=1.26): https://go.dev/dl/"
  [ -f config.json ] || warn "config.json not found. If this is the initial deployment, use ./install.sh"
  ok "Go: $(go version)"

  if command -v npm >/dev/null 2>&1; then
    info "Rebuilding the frontend static output..."
    ( cd web && npm ci && npm run build:static )
    rm -rf server/webui/dist && cp -r web/out server/webui/dist
    info "Recompiling the single binary with the embedded frontend..."
    CGO_ENABLED=0 go build -tags embedui -trimpath -o artex ./cmd/artex
  else
    warn "npm not found. Compiling the backend only without an embedded frontend (run the frontend separately with npm run dev)"
    CGO_ENABLED=0 go build -o artex ./cmd/artex
  fi
  ok "Build complete -> ./artex"
  warn "Restart the running artex process to apply the changes (the schema is migrated automatically on restart)"
}

echo "=============================="
echo "  ARTEX update"
echo "  1) Docker update (pull the new image and recreate)"
echo "  2) Local update (recompile with go)"
echo "=============================="
case "$(ask 'Select' 1)" in
  1) sync_repo; update_docker ;;
  2) sync_repo; update_local ;;
  *) die "Invalid selection" ;;
esac
