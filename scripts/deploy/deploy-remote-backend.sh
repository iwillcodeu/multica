#!/usr/bin/env bash
# 3.2) On your Mac: cross-compile Linux API + migrate, rsync server tree + binaries to Ubuntu, migrate, restart.
#
# Env: DEPLOY_HOST (default chandao), DEPLOY_REMOTE_DIR (default /opt/multica), NO_MIGRATE=1

set -euo pipefail
HOST="${DEPLOY_HOST:-chandao}"
REMOTE="${DEPLOY_REMOTE_DIR:-/opt/multica}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

cd "$ROOT/server"
mkdir -p bin/linux-amd64
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=auto
echo "==> Building migrate + server (linux/amd64)..."
go build -trimpath -o bin/linux-amd64/migrate ./cmd/migrate
go build -trimpath -o bin/linux-amd64/server ./cmd/server

echo "==> Rsync server/ → ${HOST}:${REMOTE}/server/ (excluding bin/)"
rsync -az --delete --exclude 'bin/' "$ROOT/server/" "${HOST}:${REMOTE}/server/"

echo "==> Upload binaries"
rsync -az \
  "$ROOT/server/bin/linux-amd64/server" \
  "$ROOT/server/bin/linux-amd64/migrate" \
  "${HOST}:${REMOTE}/server/bin/"
ssh "$HOST" "chmod +x ${REMOTE}/server/bin/server ${REMOTE}/server/bin/migrate"

if [[ "${NO_MIGRATE:-0}" != "1" ]]; then
  echo "==> migrate up on ${HOST}"
  ssh "$HOST" "set -a && source ${REMOTE}/.env && set +a && ${REMOTE}/server/bin/migrate up"
else
  echo "==> Skipping migrate (NO_MIGRATE=1)"
fi

echo "==> restart multica-server"
ssh "$HOST" "systemctl restart multica-server 2>/dev/null || true"
echo "Done."
