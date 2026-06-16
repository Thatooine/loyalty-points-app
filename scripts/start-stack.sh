#!/usr/bin/env bash
#
# One-click local stack: start PostgreSQL, seed an admin, run the server.
#
#   ./scripts/start-stack.sh
#
# Steps, in order:
#   1. docker compose up -d   — start the bundled Postgres
#   2. wait until Postgres is accepting connections
#   3. go run ./cmd/bootstrap — wipe data tables and (re)create the admin
#                               system@mail.com / systemUser123
#   4. go run ./cmd/app       — JSON-RPC server on :8080 (auto-applies migrations)
#
# The server runs in the foreground; press Ctrl-C to stop it. Postgres keeps
# running in the background — stop it with `docker compose down`.
#
set -euo pipefail

# Resolve the repo root from this script's location so it runs from anywhere.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

log() { printf '\n\033[1;34m==>\033[0m %s\n' "$1"; }

for cmd in docker go; do
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "error: '${cmd}' is required but not installed" >&2
    exit 1
  fi
done

log "Starting PostgreSQL (docker compose up -d)"
docker compose up -d

log "Waiting for PostgreSQL to be ready"
for attempt in $(seq 1 30); do
  if docker compose exec -T postgres pg_isready -U loyalty -d loyalty_points >/dev/null 2>&1; then
    echo "PostgreSQL is ready."
    break
  fi
  if [ "${attempt}" -eq 30 ]; then
    echo "error: PostgreSQL did not become ready in time" >&2
    exit 1
  fi
  sleep 1
done

log "Bootstrapping admin user (system@mail.com / systemUser123)"
echo "Note: this wipes all data tables before recreating the admin."
go run ./cmd/bootstrap

log "Starting JSON-RPC server on :8080 (Ctrl-C to stop)"
exec go run ./cmd/app
