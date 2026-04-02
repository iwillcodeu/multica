#!/usr/bin/env bash
# Run on the Ubuntu host (as root) to align /opt/multica/.env with a public URL + WebSocket.
# Use after switching DNS to Multica or changing domain.
#
#   APP_PUBLIC_BASE=https://pmo.atuofuture.com bash apply-public-url-on-server.sh
#   APP_PUBLIC_BASE=http://pmo.atuofuture.com:9080 bash apply-public-url-on-server.sh /opt/multica/.env

set -euo pipefail

ENV_FILE="${1:-/opt/multica/.env}"
BASE="${APP_PUBLIC_BASE:-https://pmo.atuofuture.com}"
BASE="${BASE%/}"

if [[ "$BASE" =~ ^https://([^/:]+) ]]; then
  WS="wss://${BASH_REMATCH[1]}/ws"
elif [[ "$BASE" =~ ^http://([^/:]+) ]]; then
  WS="ws://${BASH_REMATCH[1]}/ws"
else
  echo "APP_PUBLIC_BASE must start with http:// or https://" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE" >&2
  exit 1
fi

set_kv() {
  local k="$1" v="$2"
  if grep -q "^${k}=" "$ENV_FILE" 2>/dev/null; then
    sed -i.bak "s#^${k}=.*#${k}=${v}#" "$ENV_FILE"
  else
    echo "${k}=${v}" >>"$ENV_FILE"
  fi
}

set_kv MULTICA_APP_URL "$BASE"
set_kv FRONTEND_ORIGIN "$BASE"
set_kv CORS_ALLOWED_ORIGINS "$BASE"
set_kv MULTICA_SERVER_URL "$WS"
set_kv NEXT_PUBLIC_WS_URL "$WS"
set_kv GOOGLE_REDIRECT_URI "${BASE}/auth/callback"
rm -f "${ENV_FILE}.bak"

echo "Updated $ENV_FILE for public URL ${BASE} (WS ${WS}). Restart: systemctl restart multica-server multica-web"
