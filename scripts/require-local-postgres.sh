#!/usr/bin/env bash
# Fail fast if local Postgres is not accepting connections.
# Does not start Docker or install Postgres — you manage the server yourself.
set -euo pipefail

ENV_FILE="${1:-.env}"

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing env file: $ENV_FILE"
  exit 1
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

POSTGRES_PORT="${POSTGRES_PORT:-5432}"
# Match typical .env: host is 127.0.0.1 or localhost
PGHOST="${PGHOST:-127.0.0.1}"

if ! command -v pg_isready >/dev/null 2>&1; then
  echo "==> pg_isready not in PATH; skipping local Postgres check."
  echo "    (Install PostgreSQL client tools or add their bin dir to PATH for a preflight check.)"
  exit 0
fi

if pg_isready -q -h "$PGHOST" -p "$POSTGRES_PORT"; then
  echo "✓ Local Postgres accepting connections on ${PGHOST}:${POSTGRES_PORT}"
  exit 0
fi

echo "Local Postgres is not accepting connections on ${PGHOST}:${POSTGRES_PORT}"
echo "Start your server (e.g. brew services start postgresql@17) and ensure POSTGRES_PORT in $(basename "$ENV_FILE") matches."
exit 1
