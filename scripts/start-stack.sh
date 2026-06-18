#!/usr/bin/env bash
#
# Start the local stack WITHOUT wiping data — the everyday restart.
#
#   ./scripts/start-stack.sh
#
# Steps, in order:
#   1. docker compose up -d   — start the bundled Postgres and Redis
#   2. wait until Postgres and Redis are accepting connections
#   3. go run ./cmd/app       — JSON-RPC server on :8080 (auto-applies migrations)
#
# Existing data (users, accounts, ledger) is preserved across restarts. To wipe
# everything and reseed the admin instead, use ./scripts/reset-stack.sh.
#
# The server runs in the foreground; press Ctrl-C to stop it. Postgres and Redis
# keep running in the background — stop them with `docker compose down`.
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

log "Starting the stack (existing data is preserved)"
echo "First time running the stack? The database has no admin user yet — run"
echo "./scripts/reset-stack.sh instead, which seeds system@mail.com before starting."
sleep 2

log "Starting PostgreSQL + Redis (docker compose up -d)"
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

log "Waiting for Redis to be ready"
for attempt in $(seq 1 30); do
  if docker compose exec -T redis redis-cli ping >/dev/null 2>&1; then
    echo "Redis is ready."
    break
  fi
  if [ "${attempt}" -eq 30 ]; then
    echo "error: Redis did not become ready in time" >&2
    exit 1
  fi
  sleep 1
done

log "Starting JSON-RPC server on :8080 (Ctrl-C to stop)"
exec go run ./cmd/app
