#!/usr/bin/env bash
# 3.1) On your Mac: Next standalone build + rsync to Ubuntu + restart multica-web.
#
# Env: DEPLOY_HOST, DEPLOY_REMOTE_DIR, DEPLOY_WEB_ENV_FILE
# Env file: scripts/deploy/pmo.chandao.web.env or repo .env.chandao.deploy

set -euo pipefail

HOST="${DEPLOY_HOST:-chandao}"
REMOTE="${DEPLOY_REMOTE_DIR:-/opt/multica}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WEB="${ROOT}/apps/web"
STAND="${WEB}/.next/standalone"
DEFAULT_ENV="${ROOT}/scripts/deploy/pmo.chandao.web.env"
ENV_FILE="${DEPLOY_WEB_ENV_FILE:-}"

cd "$ROOT"

if [[ -z "$ENV_FILE" ]]; then
  if [[ -f "${ROOT}/.env.chandao.deploy" ]]; then
    ENV_FILE="${ROOT}/.env.chandao.deploy"
  elif [[ -f "$DEFAULT_ENV" ]]; then
    ENV_FILE="$DEFAULT_ENV"
  fi
fi

if [[ -z "$ENV_FILE" || ! -f "$ENV_FILE" ]]; then
  echo "No web env file. Use scripts/deploy/pmo.chandao.web.env or .env.chandao.deploy (see env.chandao.web.example)." >&2
  exit 1
fi

echo "==> Build env: ${ENV_FILE}"
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

export MULTICA_STANDALONE_DEPLOY=1
export NEXT_TELEMETRY_DISABLED=1

echo "==> pnpm install (sync lockfile → node_modules; avoids missing TipTap etc. after pull)"
pnpm install --frozen-lockfile

echo "==> pnpm build (standalone)"
pnpm build

if [[ ! -f "${STAND}/apps/web/server.js" ]]; then
  echo "Missing ${STAND}/apps/web/server.js" >&2
  exit 1
fi

echo "==> standalone: public + static"
rm -rf "${STAND}/apps/web/public"
cp -R "${WEB}/public" "${STAND}/apps/web/public"
mkdir -p "${STAND}/apps/web/.next"
rm -rf "${STAND}/apps/web/.next/static"
cp -R "${WEB}/.next/static" "${STAND}/apps/web/.next/static"

echo "==> prune macOS sharp"
shopt -s nullglob 2>/dev/null || true
for d in "${STAND}/node_modules/.pnpm/"@img+sharp-darwin* \
         "${STAND}/node_modules/.pnpm/"@img+sharp-libvips-darwin* \
         "${STAND}/node_modules/.pnpm/sharp@*"; do
  [[ -e "$d" ]] && rm -rf "$d"
done

echo "==> rsync → ${HOST}:${REMOTE}/apps/web/.next/standalone/"
ssh "$HOST" "mkdir -p ${REMOTE}/apps/web/.next"
rsync -avz --delete "${STAND}/" "${HOST}:${REMOTE}/apps/web/.next/standalone/"

sed "s|__DEPLOY_DIR__|${REMOTE}|g" "${ROOT}/scripts/deploy/multica-web-start.in" | ssh "$HOST" "cat > /usr/local/bin/multica-web-start && chmod +x /usr/local/bin/multica-web-start"

ssh "$HOST" "systemctl restart multica-web 2>/dev/null || true"
echo "Done."
