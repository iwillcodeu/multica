# Multica

AI-native project management — like Linear, but with AI agents as first-class team members.

Multica lets you manage tasks and collaborate with AI agents the same way you work with human teammates. Agents can be assigned issues, post comments, update statuses, and execute work autonomously on your local machine.

## Features

- **AI agents as teammates** — assign issues to agents, mention them in comments, and let them do the work
- **Local agent runtime** — agents run on your machine using Claude Code or Codex, with full access to your codebase
- **Real-time collaboration** — WebSocket-powered live updates across the board
- **Multi-workspace** — organize work across teams with workspace-level isolation
- **Familiar UX** — if you've used Linear, you'll feel right at home

## Getting Started

### Use Multica Cloud

The fastest way to get started: [multica.ai](https://multica.ai)

### Self-Host

More detail: [SELF_HOSTING.md](SELF_HOSTING.md), [AGENTS.md](AGENTS.md). Deployment is split into **three** flows; each has dedicated scripts under `scripts/deploy/`.

#### 1. Docker deployment

Docker runs PostgreSQL; this machine runs the Go API and Next dev server (frontend hot reload).

```bash
cp .env.example .env   # set JWT_SECRET etc.
./scripts/deploy/deploy-docker.sh
# equivalent: make deploy-docker
```

Requires: Docker, Go, pnpm, `.env`.

#### 2. Local deployment (e.g. Mac + Homebrew Postgres)

Use **two terminals**: backend first, then frontend.

- **2.1 Frontend** — `pnpm install`, production `pnpm build`, `next start` on `127.0.0.1`: `./scripts/deploy/deploy-local-frontend.sh` (or `make deploy-local-frontend`).
- **2.2 Backend** — DB check, `migrate up`, `go build`, run `./bin/server`: `./scripts/deploy/deploy-local-backend.sh` (or `make deploy-local-backend`).

Configure `.env` (`DATABASE_URL`, `JWT_SECRET`). If Postgres is only reachable via Docker on this Mac, run backend with `SKIP_LOCAL_PG_CHECK=1 ./scripts/deploy/deploy-local-backend.sh`.

#### 3. Remote deployment (Ubuntu, e.g. `chandao`)

**First time on the server (as root):** install tree under `/opt/multica` (do **not** copy your laptop `.env` over production). Free port 80 if needed: `./scripts/deploy/disable-docker-on-server.sh`. Bootstrap stack: `./scripts/deploy/ubuntu-noble-native.sh` (see file header). Set public URLs: `./scripts/deploy/apply-public-url-on-server.sh`. DNS + TLS (e.g. `certbot --nginx`) as usual.

**Day-to-day from your Mac:**

- **3.1 Frontend** — Mac standalone build, rsync, restart `multica-web`: `./scripts/deploy/deploy-remote-frontend.sh` (or `make deploy-remote-frontend`).
- **3.2 Backend** — Linux cross-compile, rsync `server/` + binaries, `migrate up`, restart `multica-server`: `./scripts/deploy/deploy-remote-backend.sh` (or `make deploy-remote-backend`).

Environment: `DEPLOY_HOST` (default `chandao`), `DEPLOY_REMOTE_DIR` (default `/opt/multica`), `NO_MIGRATE=1` to skip migrations. Frontend build env: `scripts/deploy/pmo.chandao.web.env` or `.env.chandao.deploy` — see `scripts/deploy/env.chandao.web.example`.

#### Backend logs

- **Local:** logs in the terminal running the API; optional `LOG_LEVEL=debug|info|warn|error` in `.env`.
- **systemd:** `journalctl -u multica-server -f` (API), `journalctl -u multica-web -f` (Next).
- **Nginx:** `/var/log/nginx/access.log`, `error.log`.

## CLI

The `multica` CLI connects your local machine to Multica — authenticate, manage workspaces, and run the agent daemon.

```bash
# Install
brew tap multica-ai/tap
brew install multica

# Authenticate and start
multica login
multica daemon start
```

The daemon auto-detects available agent CLIs (`claude`, `codex`) on your PATH. When an agent is assigned a task, the daemon creates an isolated environment, runs the agent, and reports results back.

See the [CLI and Daemon Guide](CLI_AND_DAEMON.md) for the full command reference, daemon configuration, and advanced usage.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────────┐
│   Next.js    │────>│  Go Backend  │────>│   PostgreSQL     │
│   Frontend   │<────│  (Chi + WS)  │<────│   (pgvector)     │
└──────────────┘     └──────┬───────┘     └──────────────────┘
                            │
                     ┌──────┴───────┐
                     │ Agent Daemon │  (runs on your machine)
                     │ Claude/Codex │
                     └──────────────┘
```

- **Frontend**: Next.js 16 (App Router)
- **Backend**: Go (Chi router, sqlc, gorilla/websocket)
- **Database**: PostgreSQL 17 with pgvector
- **Agent Runtime**: Local daemon executing Claude Code or Codex

## Development

For contributors working on the Multica codebase, see the [Contributing Guide](CONTRIBUTING.md).

### Prerequisites

- [Node.js](https://nodejs.org/) (v20+)
- [pnpm](https://pnpm.io/) (v10.28+)
- [Go](https://go.dev/) (v1.26+)
- [Docker](https://www.docker.com/)

### Quick Start

```bash
pnpm install
cp .env.example .env
make setup
make start
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development workflow, worktree support, testing, and troubleshooting.

## License

See [LICENSE](LICENSE) for details.
