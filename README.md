# Loyalty Points Wallet

A Go backend for a loyalty-points wallet — member accounts, earn/spend, role-based
access, batch CSV ingestion, and an audit trail. Exposed as a **JSON-RPC 2.0** API
over HTTP, backed by **PostgreSQL**, with **Redis**-backed rate limiting.

## Requirements

- Go 1.25+
- Docker (for the bundled PostgreSQL + Redis), or any reachable instances

## Run it

```bash
./scripts/start-stack.sh   # restart, keeps existing data
./scripts/reset-stack.sh   # fresh start: wipes data, reseeds the admin
```

Both bring up Postgres + Redis and run the server in the foreground on `:8080`
(migrations auto-apply on startup). `reset-stack.sh` also runs `cmd/bootstrap`,
which **wipes all data tables** and recreates the `system@mail.com` admin.

By hand:

```bash
docker compose up -d
go run ./cmd/app
```

The default DSN and a dev JWT key are baked into `cmd/app/config.go`, matching
docker-compose. Override via env:

| Variable              | Default                                                                    | Purpose                          |
| --------------------- | -------------------------------------------------------------------------- | -------------------------------- |
| `POSTGRES_DSN`        | `postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable` | Database connection string       |
| `JWT_PRIVATE_KEY_PEM` | a dev RSA key (PKCS#8 PEM)                                                  | Signs/verifies access tokens     |
| `REDIS_URI`           | `localhost:6379`                                                           | Rate limiter backend             |
| `APP_ENV`             | `local`                                                                    | Non-`local` requires secrets via env |
| `PORT`                | `8080`                                                                     | HTTP listen port                 |

The baked-in key and Redis fallback are local-only: outside `local`,
`JWT_PRIVATE_KEY_PEM` and `REDIS_URI` are required (fail closed).

### Admin token

New registrations are always members. To get an admin, bootstrap the well-known
system user, then log in as it:

```bash
go run ./cmd/bootstrap   # wipes data, (re)creates system@mail.com / admin-user-123 as admin
```

For an existing user the production path is an operator promotion — see
[SOLUTION.md](./SOLUTION.md#access-control).

## Build, vet, test

```bash
go build ./...
go vet ./...
go test ./...        # DB-backed tests SKIP without TEST_POSTGRES_DSN

export TEST_POSTGRES_DSN='postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable'
go test ./...        # full suite against the bundled database
```

## The API

All services mount on a single endpoint:

```
POST http://localhost:8080/api      Content-Type: application/json
```

Requests are JSON-RPC 2.0; `params` is a **one-element array** holding the
argument object. Protected methods need `Authorization: Bearer <token>`.

```json
{ "jsonrpc": "2.0", "method": "Service.Method", "params": [ { ... } ], "id": 1 }
```

A ready-to-import **Postman collection** with every method pre-built lives at
[`api/loyalty-points.postman_collection.json`](./api/loyalty-points.postman_collection.json).

### Errors

JSON-RPC errors return **HTTP 200**; the error is in the body. Every envelope has
a numeric `code`, a `message`, and a stable `data.reason` token clients should
branch on. Codes are defined once in `pkg/jsonrpc/error.go`.

| Code     | `data.reason`          | Meaning                                                            |
| -------- | ---------------------- | ----------------------------------------------------------------- |
| `-32700` | —                      | Parse error (malformed JSON)                                      |
| `-32602` | `invalid_argument`     | Validation failed; per-field reasons under `data.fields`          |
| `-32603` | `internal`             | Internal error (default hides the underlying message)             |
| `-32001` | `unauthorized`         | Missing/invalid/expired/revoked token                             |
| `-32002` | `forbidden`            | Authenticated but the role lacks permission                       |
| `-32003` | `not_found`            | Missing — also returned for a foreign resource (no leak)          |
| `-32004` | `already_exists`       | Uniqueness conflict (e.g. duplicate email)                        |
| `-32005` | `insufficient_balance` | A spend/adjustment would drive the balance below zero             |

### Roles

- **member** — read their own account and **earn/spend** on it (default for every registration).
- **admin** — read/adjust *any* account, plus the generic `ProcessTransaction` and batch ingestion.

### Methods

| Method                                     | Access                     | Purpose                                     |
| ------------------------------------------ | -------------------------- | ------------------------------------------- |
| `UserRegistrationService.Register`         | public                     | Create user + first account, return a token |
| `EmailPasswordAuthenticator.Login`         | public                     | Exchange credentials for a token            |
| `Session.Logout`                           | any authenticated          | Revoke all of the caller's tokens           |
| `AccountOpener.OpenAccount`                | any authenticated          | Open an additional wallet for the caller    |
| `AccountService.GetAccountByID`            | member (own) / admin (any) | Read an account                             |
| `AccountService.GetAccountBalance`         | member (own) / admin (any) | Read a balance                              |
| `AccountService.UpdateAccountName`         | member (own) / admin (any) | Rename an account                           |
| `AccountService.UpdateAccountBalance`      | admin only                 | Raw signed-delta correction (off-ledger)    |
| `Wallet.EarnPoints`                        | member (own) / admin (any) | Credit points                               |
| `Wallet.SpendPoints`                       | member (own) / admin (any) | Debit points (rejected below zero)          |
| `Wallet.ProcessTransaction`                | admin only                 | Generic earn/spend (caller picks `kind`)    |
| `Wallet.ProcessTransactionBatch`           | admin only                 | Apply an ordered batch                      |
| `AuditService.FetchTransactionAuditTrail`  | member (own) / admin (any) | List processing attempts for a `ref`        |

Notes that matter when calling the wallet methods:

- `ref` is the **idempotency key** — re-submitting the same `ref` returns the
  original outcome with `"duplicate": true` and applies no new effect.
- `occurred_at` is optional; the server stamps it when omitted.
- Account ownership is scoped to the caller, so a member hitting a foreign account
  reads as `not_found` (no existence leak).
- Tokens expire after **1 hour**; there is no refresh flow — log in again.
- Logout bumps the user's session epoch, invalidating **every** token they hold.

## CLI: batch CSV ingestion

`loyalty-cli ingest` reads a CSV, sorts rows by `occurred_at` (then line), and
sends the whole ordered batch as one admin-only request, then prints a summary.

CSV header is mandatory and exact:

```csv
ref,account_id,kind,points,occurred_at
tx-001,a91b...,earn,150,2024-06-01T10:00:00Z
tx-002,a91b...,spend,40,2024-06-01T11:00:00Z
```

`occurred_at` may be blank (server stamps it); a non-blank, non-RFC3339 value is a
local error and is never sent.

```bash
go run ./cmd/cli ingest --file ./batch.csv --url http://localhost:8080/api --token "$ADMIN_TOKEN"
go run ./cmd/cli ingest --file ./batch.csv --dry-run    # preview, sends nothing
```

Every attempt — accepted, duplicate, or rejected — is written to `audit_entries`
and queryable via `AuditService.FetchTransactionAuditTrail`.

## Repository layout

| Path                      | What lives there                                                  |
| ------------------------- | ----------------------------------------------------------------- |
| `cmd/app`                 | Server entrypoint, config, DI wiring, RPC setup                   |
| `cmd/cli`                 | `loyalty-cli` — thin JSON-RPC client (notably `ingest`)          |
| `cmd/bootstrap`           | Clean-slate dev tool: wipes data tables and recreates the admin   |
| `pkg/<domain>`            | Ports: interfaces, DTOs, `Validate()`, JSON-RPC adaptors          |
| `internal/pkg/<domain>`   | PostgreSQL implementations of those ports                         |
| `pkg/postgres/migrations` | Embedded SQL schema, applied in lexical order on startup          |
| `tests/`                  | Black-box HTTP/JSON-RPC integration tests                         |
| `api/`                    | Postman collection                                                |
| `scripts/`                | `start-stack.sh` (keeps data) / `reset-stack.sh` (wipes data)     |

See [SOLUTION.md](./SOLUTION.md) for the architecture and reasoning.
