#!/usr/bin/env bash
# Native (non-Docker) deploy on Ubuntu 24.04+ with PostgreSQL + pgvector, Go, Node 22, pnpm.
# Run as root. Idempotent for apt/toolchain; preserves existing ${DEPLOY_DIR}/.env if present.
#
# Optional:
#   DEPLOY_SKIP_CLONE=1   — skip git clone/pull (use after rsync or manual checkout)
#   USE_PREBUILT_GO=1     — no Go install; use server/bin/migrate + server/bin/server (Linux amd64)
#   SKIP_GO_INSTALL=1     — same as above for toolchain (set automatically with USE_PREBUILT_GO)
#   APP_PUBLIC_BASE       — browser-facing origin for new .env (default https://pmo.atuofuture.com)
#   MULTICA_SERVER_NAME   — nginx server_name (default "pmo.atuofuture.com 47.103.102.65")
#   GIT_REPO / GIT_REF / DEPLOY_DIR / GO_VERSION
#
# Do not rsync your Mac .env onto the server (local DATABASE_URL/POSTGRES_PORT will break Ubuntu Postgres).

set -euo pipefail

GIT_REPO="${GIT_REPO:-https://github.com/iwillcodeu/multica.git}"
GIT_REF="${GIT_REF:-main}"
DEPLOY_DIR="${DEPLOY_DIR:-/opt/multica}"
GO_VERSION="${GO_VERSION:-1.26.1}"
NODE_MAJOR="${NODE_MAJOR:-22}"

log() { echo "[deploy] $*"; }

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root (sudo -i or root SSH)." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
export GOTOOLCHAIN=auto
# Go/npm default mirrors: proxy.golang.org often times out from mainland China; override with GOPROXY / NPM_CONFIG_REGISTRY if needed.
export GOPROXY="${GOPROXY:-https://goproxy.cn,https://proxy.golang.org,direct}"
export GOSUMDB="${GOSUMDB:-sum.golang.google.cn}"
export NPM_CONFIG_REGISTRY="${NPM_CONFIG_REGISTRY:-https://registry.npmmirror.com}"

discover_public_ip() {
  local ip=""
  ip="$(curl -fsS --connect-timeout 2 http://100.100.100.200/latest/meta-data/eipv4 2>/dev/null || true)"
  if [[ -z "$ip" ]]; then
    ip="$(curl -fsS --connect-timeout 2 http://100.100.100.200/latest/meta-data/public-ipv4 2>/dev/null || true)"
  fi
  if [[ -z "$ip" ]]; then
    ip="$(curl -fsS --connect-timeout 3 https://ifconfig.me 2>/dev/null || true)"
  fi
  if [[ -z "$ip" ]]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  fi
  echo "$ip"
}

log "Installing OS packages (PostgreSQL 16 + pgvector, nginx, build tools)..."
apt-get update -qq
apt-get install -y -qq \
  ca-certificates curl git xz-utils \
  postgresql-16 postgresql-client-16 postgresql-16-pgvector \
  nginx

MULTICA_HTTP_PORT=80
if ss -tlnp 2>/dev/null | grep -qE ':(80)\s'; then
  MULTICA_HTTP_PORT=9080
  log "Port 80 is already in use; Multica will be exposed on TCP ${MULTICA_HTTP_PORT} (open it in the security group)."
fi

if [[ "${USE_PREBUILT_GO:-0}" == "1" ]]; then
  SKIP_GO_INSTALL=1
fi

if [[ "${SKIP_GO_INSTALL:-0}" != "1" ]]; then
  if [[ ! -x /usr/local/go/bin/go ]]; then
    log "Installing Go ${GO_VERSION}..."
    # Use dl.google.com — go.dev redirects here and is more reliable from some regions (e.g. CN).
    curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tgz
    rm -f /tmp/go.tgz
  fi
  export PATH="/usr/local/go/bin:${PATH}"
  go version
else
  log "SKIP_GO_INSTALL=1 — Go toolchain not installed on server."
fi

if ! command -v node >/dev/null 2>&1 || [[ "$(node -v 2>/dev/null || true)" != v${NODE_MAJOR}* ]]; then
  log "Installing Node.js ${NODE_MAJOR}.x (NodeSource)..."
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
  apt-get install -y -qq nodejs
fi
node -v
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
corepack enable
corepack prepare pnpm@10.28.2 --activate
command -v pnpm >/dev/null
pnpm -v

install -d -m 0755 "$DEPLOY_DIR"

if [[ "${DEPLOY_SKIP_CLONE:-0}" == "1" ]]; then
  log "Skipping git (DEPLOY_SKIP_CLONE=1); using tree at ${DEPLOY_DIR}"
  if [[ ! -f "${DEPLOY_DIR}/server/go.mod" ]]; then
    echo "No server/go.mod under ${DEPLOY_DIR}; clone or rsync the app first." >&2
    exit 1
  fi
else
  if [[ -d "${DEPLOY_DIR}/.git" ]]; then
    log "Updating repo in ${DEPLOY_DIR}..."
    git -C "$DEPLOY_DIR" fetch --all --prune
    git -C "$DEPLOY_DIR" checkout "$GIT_REF"
    git -C "$DEPLOY_DIR" pull --ff-only origin "$GIT_REF" || git -C "$DEPLOY_DIR" pull --ff-only
  else
    log "Cloning ${GIT_REPO} (${GIT_REF})..."
    git clone --branch "$GIT_REF" --depth 1 "$GIT_REPO" "$DEPLOY_DIR"
  fi
fi

PUBLIC_IP="$(discover_public_ip)"
log "Detected host/public IP: ${PUBLIC_IP}"

# Browser-facing URL for CORS, OAuth, emails (not nginx listen port). Override for other domains.
APP_PUBLIC_BASE="${APP_PUBLIC_BASE:-https://pmo.atuofuture.com}"
APP_PUBLIC_BASE="${APP_PUBLIC_BASE%/}"
if [[ "$APP_PUBLIC_BASE" =~ ^https://([^/:]+) ]]; then
  APP_BASE="$APP_PUBLIC_BASE"
  WS_BASE="wss://${BASH_REMATCH[1]}"
elif [[ "$APP_PUBLIC_BASE" =~ ^http://([^/:]+) ]]; then
  APP_BASE="$APP_PUBLIC_BASE"
  WS_BASE="ws://${BASH_REMATCH[1]}"
else
  echo "APP_PUBLIC_BASE must start with http:// or https:// (got: ${APP_PUBLIC_BASE})" >&2
  exit 1
fi
log "Using APP_PUBLIC_BASE=${APP_BASE} (WebSocket base ${WS_BASE})"

MULTICA_SERVER_NAME="${MULTICA_SERVER_NAME:-pmo.atuofuture.com 47.103.102.65}"

DB_PASS=""
if [[ -f "${DEPLOY_DIR}/.env" ]]; then
  log "Loading existing ${DEPLOY_DIR}/.env"
  # shellcheck disable=SC1090
  set -a && source "${DEPLOY_DIR}/.env" && set +a
  DB_PASS="${POSTGRES_PASSWORD:-}"
fi

if [[ -z "${DB_PASS}" ]]; then
  DB_PASS="$(openssl rand -hex 16)"
fi
JWT_SECRET="${JWT_SECRET:-$(openssl rand -hex 32)}"

if [[ ! -f "${DEPLOY_DIR}/.env" ]]; then
  log "Creating ${DEPLOY_DIR}/.env"
  umask 077
  cat >"${DEPLOY_DIR}/.env" <<EOF
# --- Multica production (generated by ubuntu-noble-native.sh) ---
APP_ENV=production
PORT=8080
FRONTEND_PORT=3000
DATABASE_URL=postgres://multica:${DB_PASS}@127.0.0.1:5432/multica?sslmode=disable
POSTGRES_DB=multica
POSTGRES_USER=multica
POSTGRES_PASSWORD=${DB_PASS}
POSTGRES_PORT=5432
JWT_SECRET=${JWT_SECRET}
MULTICA_SERVER_URL=${WS_BASE}/ws
MULTICA_APP_URL=${APP_BASE}
FRONTEND_ORIGIN=${APP_BASE}
CORS_ALLOWED_ORIGINS=${APP_BASE}
NEXT_PUBLIC_API_URL=
NEXT_PUBLIC_WS_URL=${WS_BASE}/ws
REMOTE_API_URL=http://127.0.0.1:8080
GOOGLE_REDIRECT_URI=${APP_BASE}/auth/callback
RESEND_API_KEY=
RESEND_FROM_EMAIL=noreply@multica.ai
S3_BUCKET=
S3_REGION=us-west-2
EOF
  chmod 600 "${DEPLOY_DIR}/.env"
  log "New database password (stored in .env): ${DB_PASS}"
else
  log "Keeping ${DEPLOY_DIR}/.env (sync DATABASE_URL with Postgres if you changed DB_PASS)"
fi

# shellcheck disable=SC1090
set -a && source "${DEPLOY_DIR}/.env" && set +a
DB_PASS="${POSTGRES_PASSWORD:-$DB_PASS}"
if [[ -z "$DB_PASS" ]]; then
  echo "POSTGRES_PASSWORD missing in .env" >&2
  exit 1
fi

log "Configuring PostgreSQL role and database..."
# Password is generated as hex; safe to embed. If you ever use special characters, switch to dollar-quoting.
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='multica'" | grep -q 1; then
  sudo -u postgres psql -v ON_ERROR_STOP=1 -c "CREATE ROLE multica LOGIN PASSWORD '${DB_PASS}'"
else
  sudo -u postgres psql -v ON_ERROR_STOP=1 -c "ALTER ROLE multica PASSWORD '${DB_PASS}'"
fi
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='multica'" | grep -q 1; then
  sudo -u postgres psql -v ON_ERROR_STOP=1 -c "CREATE DATABASE multica OWNER multica"
fi
sudo -u postgres psql -v ON_ERROR_STOP=1 -d multica -c "CREATE EXTENSION IF NOT EXISTS vector;"

(
  cd "${DEPLOY_DIR}/server"
  if [[ "${USE_PREBUILT_GO:-0}" == "1" ]]; then
    if [[ ! -x ./bin/migrate ]] || [[ ! -x ./bin/server ]]; then
      echo "USE_PREBUILT_GO=1 requires Linux amd64 binaries at ${DEPLOY_DIR}/server/bin/migrate and bin/server." >&2
      echo "On your Mac run: scripts/deploy/deploy-remote-backend.sh (uploads server/ + binaries)" >&2
      exit 1
    fi
    log "Running migrations (prebuilt ./bin/migrate)..."
    ./bin/migrate up
    log "Skipping go build (prebuilt ./bin/server)."
  else
    if ! command -v go >/dev/null 2>&1; then
      echo "go not found; install Go or upload prebuilt binaries and set USE_PREBUILT_GO=1." >&2
      exit 1
    fi
    log "Running migrations (go run)..."
    go run ./cmd/migrate up
    log "Building Go API..."
    go build -o bin/server ./cmd/server
  fi
)

log "Installing JS deps and building Next.js..."
export NEXT_TELEMETRY_DISABLED=1
(
  cd "${DEPLOY_DIR}"
  pnpm install --frozen-lockfile
  pnpm build
)

log "Installing web start helper (standalone node or pnpm fallback)..."
sed "s|__DEPLOY_DIR__|${DEPLOY_DIR}|g" "${DEPLOY_DIR}/scripts/deploy/multica-web-start.in" >/usr/local/bin/multica-web-start
chmod +x /usr/local/bin/multica-web-start

log "Installing nginx site..."
sed -e "s|/opt/multica|${DEPLOY_DIR}|g" \
  -e "s|__MULTICA_HTTP_PORT__|${MULTICA_HTTP_PORT}|g" \
  -e "s|__MULTICA_SERVER_NAME__|${MULTICA_SERVER_NAME}|g" \
  "${DEPLOY_DIR}/scripts/deploy/nginx-multica.conf" >/etc/nginx/sites-available/multica
ln -sf /etc/nginx/sites-available/multica /etc/nginx/sites-enabled/multica
rm -f /etc/nginx/sites-enabled/default
nginx -t
if systemctl is-active --quiet nginx 2>/dev/null; then
  systemctl reload nginx
else
  systemctl enable --now nginx
fi

log "Installing systemd units..."
sed "s|/opt/multica|${DEPLOY_DIR}|g" "${DEPLOY_DIR}/scripts/deploy/systemd/multica-server.service" >/etc/systemd/system/multica-server.service
sed "s|/opt/multica|${DEPLOY_DIR}|g" "${DEPLOY_DIR}/scripts/deploy/systemd/multica-web.service" >/etc/systemd/system/multica-web.service
systemctl daemon-reload
systemctl enable multica-server multica-web
systemctl restart multica-server multica-web || systemctl start multica-server multica-web

log "Done. Public app URL (env): ${APP_BASE}"
log "Nginx listens on TCP ${MULTICA_HTTP_PORT} for: ${MULTICA_SERVER_NAME}"
log "Open ECS security group for that port (and 443 after certbot). DNS A: pmo.atuofuture.com → ${PUBLIC_IP}"
