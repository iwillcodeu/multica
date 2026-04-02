#!/usr/bin/env bash
# 1) Docker Postgres + migrate + API + Next dev (hot reload). Run from repo root.
# Requires: Docker, Go, pnpm, .env (see .env.example).

set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "Created .env — set JWT_SECRET and save, then run this script again."
  exit 1
fi

pnpm install --frozen-lockfile 2>/dev/null || pnpm install
bash scripts/ensure-postgres.sh .env

set -a
# shellcheck disable=SC1090
source .env
set +a

(cd server && go run ./cmd/migrate up)

echo "==> API :${PORT:-8080} · Web :${FRONTEND_PORT:-3000} (Ctrl+C stops both)"
trap 'kill 0' EXIT
(cd server && go run ./cmd/server) &
pnpm dev:web &
wait
