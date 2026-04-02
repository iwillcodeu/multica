#!/usr/bin/env bash
# 2.2) Mac / Linux: migrate + build Go API + run (foreground). Uses repo .env.
# Postgres must match DATABASE_URL. Optional: SKIP_LOCAL_PG_CHECK=1 if DB is Docker-only.

set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ ! -f .env ]]; then
  echo "Missing .env (copy from .env.example)" >&2
  exit 1
fi

if [[ "${SKIP_LOCAL_PG_CHECK:-0}" != "1" ]]; then
  bash scripts/require-local-postgres.sh .env
fi

set -a
# shellcheck disable=SC1090
source .env
set +a

cd "$ROOT/server"
go run ./cmd/migrate up
go build -o bin/server ./cmd/server
echo "==> Starting API (PORT=${PORT:-8080})"
exec ./bin/server
