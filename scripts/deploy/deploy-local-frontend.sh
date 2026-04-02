#!/usr/bin/env bash
# 2.1) Mac / Linux: install deps, production build, run Next on 127.0.0.1 (separate terminal from backend).

set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

pnpm install --frozen-lockfile 2>/dev/null || pnpm install
export NEXT_TELEMETRY_DISABLED=1
pnpm build

PORT="${FRONTEND_PORT:-3000}"
echo "==> Starting Next on http://127.0.0.1:${PORT}"
exec pnpm --filter @multica/web start -- -H 127.0.0.1 -p "$PORT"
