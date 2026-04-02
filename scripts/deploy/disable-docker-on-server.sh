#!/usr/bin/env bash
# Run as root on Ubuntu: stop all containers, stop Docker, disable auto-start.
# Use when freeing port 80/443 for native nginx (e.g. Multica).

set -euo pipefail
if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root." >&2
  exit 1
fi

if command -v docker >/dev/null 2>&1; then
  docker stop "$(docker ps -q)" 2>/dev/null || true
fi

systemctl stop docker.socket 2>/dev/null || true
systemctl stop docker 2>/dev/null || true
systemctl disable docker.socket 2>/dev/null || true
systemctl disable docker 2>/dev/null || true

echo "Docker stopped and disabled (docker.service + docker.socket)."
echo "Re-enable later: systemctl enable --now docker"
