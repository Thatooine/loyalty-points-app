# Loyalty Points Wallet

A small backend service for a loyalty-points wallet: member accounts, earning
and spending of points, role-based access control, batch CSV ingestion, and an
audit trail. Built in Go, exposed as a **JSON-RPC 2.0** API over HTTP, backed by
**PostgreSQL**.

The design rationale and trade-offs live in [SOLUTION.md](./SOLUTION.md). This
file is the operator's guide: how to run it and how to call it.

---

## Requirements

- Go 1.23+
- Docker (for the bundled PostgreSQL) — or any reachable PostgreSQL instance

## Run it locally

**One-click:** start Postgres, seed an admin, and run the server in one command:

```bash
./scripts/start-stack.sh
```

It runs `docker compose up -d`, waits for Postgres, runs `cmd/bootstrap` (which
**wipes all data tables** and recreates the `system@mail.com` admin — see below),
then runs the server in the foreground on `:8080`. Press Ctrl-C to stop the
server; `docker compose down` stops Postgres.

Or do it by hand:

```bash
# 1. Start PostgreSQL (credentials match the app's baked-in defaults)
docker compose up -d

# 2. Run the server. It auto-applies migrations on startup and listens on :8080.
go run ./cmd/app
```

The server connects to the docker-compose database out of the box — the default
DSN and a development JWT signing key are baked into `cmd/app/config.go`. Both
can be overridden by environment variable:

| Variable              | Default                                                                     | Purpose                          |
| --------------------- | --------------------------------------------------------------------------- | -------------------------------- |
| `POSTGRES_DSN`        | `postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable`  | Database connection string       |
| `JWT_PRIVATE_KEY_PEM` | a dev RSA key (PKCS#8 PEM)                                                   | Signs/verifies access tokens     |
| `PORT`                | `8080`                                                                      | HTTP listen port                 |

> The baked-in key is for local development only. Provide your own
> `JWT_PRIVATE_KEY_PEM` for any real deployment.

### Get an admin token

New registrations are always members, and crediting points / batch ingestion are
admin-only. For local development, bootstrap a well-known **admin** system user:

```bash
go run ./cmd/bootstrap   # resets the DB and (re)creates system@mail.com / systemUser123 as an admin
```

Then `EmailPasswordAuthenticator.Login` with those credentials to get an admin
token. (For an existing user, the production path is an operator promotion —
`UPDATE users SET role = 'admin' WHERE email = '...'` — see
[SOLUTION.md](./SOLUTION.md#access-control).)

> `go run ./cmd/bootstrap` **wipes every data table** before recreating the
> system user — it is a clean-slate dev tool, not for production.

## Build, vet, test

```bash
go build ./...
go vet ./...
go test ./...        # DB-backed tests SKIP when TEST_POSTGRES_DSN is unset

# Full suite against the bundled database:
export TEST_POSTGRES_DSN='postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable'
go test ./...
```

---

## The API

All services — public and protected — mount on a single endpoint:

```
POST http://localhost:8080/api
Content-Type: application/json
```

Requests follow JSON-RPC 2.0. The transport (`gorilla/rpc/v2/json2`) expects
`params` to be a **one-element array** whose element is the argument object:

```json
{ "jsonrpc": "2.0", "method": "Service.Method", "params": [ { ... } ], "id": 1 }
```

Protected methods require an access token in the `Authorization` header:

```
Authorization: Bearer <token>
```

### The error model

JSON-RPC errors are returned with **HTTP 200**; the error lives in the response
body. Every error envelope — whether written by the auth middleware before a
handler runs, or mapped from a domain error a handler returned — has the same
shape: a numeric `code`, a human-readable `message`, and a stable
machine-readable token under `data.reason` that clients should branch on instead
of the message.

```json
{ "jsonrpc": "2.0", "error": { "code": -32005, "message": "insufficient balance", "data": { "reason": "insufficient_balance" } }, "id": 1 }
```

Codes are defined once in `pkg/jsonrpc/error.go` and shared by both the codec
mapper (`MapError`) and the middleware, so the same condition always yields the
same code:

| Code     | `data.reason`        | Meaning                                                              |
| -------- | -------------------- | -------------------------------------------------------------------- |
| `-32700` | —                    | Parse error (malformed JSON)                                         |
| `-32602` | `invalid_argument`   | Validation failed; per-field reasons under `data.fields`             |
| `-32603` | `internal`           | Internal error (the unmapped default hides the underlying message)   |
| `-32001` | `unauthorized`       | Missing/invalid/expired token, or token revoked by logout            |
| `-32002` | `forbidden`          | Authenticated but the role lacks permission for the method           |
| `-32003` | `not_found`          | Resource missing — also returned for a foreign resource (no leak)    |
| `-32004` | `already_exists`     | Uniqueness conflict (e.g. duplicate email on register)               |
| `-32005` | `insufficient_balance` | A spend/adjustment would drive the balance below zero              |

Validation failures (`-32602`) additionally carry the offending fields:

```json
{ "jsonrpc": "2.0", "error": { "code": -32602, "message": "...", "data": { "reason": "invalid_argument", "fields": { "points": "must be positive" } } }, "id": 1 }
```

### Roles in one line

- **member** — read their own account and **spend** from it (default for every new registration). Members cannot credit points.
- **admin** — read/adjust *any* account, **credit** points (earn / process), and run batch ingestion.

See [SOLUTION.md](./SOLUTION.md#access-control) for how a user becomes an admin.

---

## Endpoints

The full method surface at a glance (detail follows):

| Method                          | Access                       | Purpose                                     |
| ------------------------------- | ---------------------------- | ------------------------------------------- |
| `UserRegistrationService.Register` | public                    | Create user + first account, return a token |
| `EmailPasswordAuthenticator.Login` | public                    | Exchange credentials for a token            |
| `Session.Logout`                | any authenticated            | Revoke all of the caller's tokens           |
| `AccountOpener.OpenAccount`     | any authenticated            | Open an additional wallet for the caller    |
| `AccountService.GetAccountByID`               | member (own) / admin (any)   | Read an account                             |
| `AccountService.GetAccountBalance`     | member (own) / admin (any)   | Read a balance                              |
| `AccountService.UpdateAccountName`     | member (own) / admin (any)   | Rename an account                           |
| `AccountService.UpdateAccountBalance`  | admin only                   | Raw signed-delta correction (off-ledger)    |
| `Wallet.SpendPoints`            | member (own) / admin (any)   | Debit points                                |
| `Wallet.EarnPoints`             | admin only                   | Credit points                               |
| `Wallet.ProcessTransaction`     | admin only                   | Generic earn/spend (caller picks `kind`)    |
| `Wallet.ProcessTransactionBatch`| admin only                   | Apply an ordered batch                      |
| `AuditService.ListByTransactionRef`    | member (own) / admin (any)   | List processing attempts for a `ref`        |

### `UserRegistrationService.Register` — public

Creates a user and opens their first wallet account in one atomic step, then
returns an access token (you are logged in immediately). New users are always
members.

**Request**

```bash
curl -s http://localhost:8080/api -H 'Content-Type: application/json' -d '{
  "jsonrpc": "2.0",
  "method": "UserRegistrationService.Register",
  "params": [{
    "email": "rina@example.com",
    "password": "s3cret-pass",
    "name": "Rina",
    "accountName": "Primary Wallet"
  }],
  "id": 1
}'
```

**Response**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "token": "eyJhbGci...",
    "userID": "0c5f...",
    "accountID": "a91b...",
    "email": "rina@example.com"
  },
  "id": 1
}
```

### `EmailPasswordAuthenticator.Login` — public

Exchanges credentials for a fresh access token.

**Request**

```bash
curl -s http://localhost:8080/api -H 'Content-Type: application/json' -d '{
  "jsonrpc": "2.0",
  "method": "EmailPasswordAuthenticator.Login",
  "params": [{ "email": "rina@example.com", "password": "s3cret-pass" }],
  "id": 1
}'
```

**Response**

```json
{ "jsonrpc": "2.0", "result": { "token": "eyJhbGci...", "userID": "0c5f...", "email": "rina@example.com" }, "id": 1 }
```

Tokens expire after **1 hour**. There is no refresh flow — log in again for a
fresh token.

### `Session.Logout` — any authenticated user

Revokes the caller's tokens. The acting user is taken from the token, never the
body, so an empty params object is fine. Logout bumps the user's session epoch,
which invalidates **every** token they currently hold — including other devices
("log out everywhere"). The same token used after logout is rejected as
unauthorized.

**Request**

```bash
curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{
  "jsonrpc": "2.0", "method": "Session.Logout", "params": [{}], "id": 1
}'
```

**Response**

```json
{ "jsonrpc": "2.0", "result": { "ok": true }, "id": 1 }
```

### `Wallet.EarnPoints` — admin only

Credits points. Crediting is an operator action — a member token is rejected, so
a member cannot mint points into their own account. `ref` is the idempotency
key — re-submitting the same `ref` returns the original outcome with
`"duplicate": true` and applies no new effect. `occurred_at` is optional; the
server stamps it when omitted.

**Request**

```bash
curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{
  "jsonrpc": "2.0",
  "method": "Wallet.EarnPoints",
  "params": [{
    "ref": "tx-001",
    "account_id": "a91b...",
    "points": 150,
    "occurred_at": "2024-06-01T10:00:00Z"
  }],
  "id": 1
}'
```

**Response**

```json
{
  "jsonrpc": "2.0",
  "result": {
    "ref": "tx-001",
    "account_id": "a91b...",
    "kind": "earn",
    "points": 150,
    "occurred_at": "2024-06-01T10:00:00Z",
    "recorded_at": "2026-06-16T08:30:01.123456789Z",
    "balance": 150,
    "duplicate": false
  },
  "id": 1
}
```

### `Wallet.SpendPoints` — member (own account) / admin (any)

Debits points. Rejected if it would drive the balance below zero.

**Request**

```bash
curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{
  "jsonrpc": "2.0",
  "method": "Wallet.SpendPoints",
  "params": [{ "ref": "tx-002", "account_id": "a91b...", "points": 40 }],
  "id": 1
}'
```

**Response (accepted)**

```json
{ "jsonrpc": "2.0", "result": { "ref": "tx-002", "account_id": "a91b...", "kind": "spend", "points": 40, "balance": 110, "duplicate": false, "occurred_at": "...", "recorded_at": "..." }, "id": 1 }
```

**Response (overdraft rejected)**

```json
{ "jsonrpc": "2.0", "error": { "code": -32005, "message": "insufficient balance", "data": { "reason": "insufficient_balance" } }, "id": 1 }
```

### `Wallet.ProcessTransaction` — admin only

The general earn/spend method; `kind` is `"earn"` or `"spend"`. `EarnPoints` and
`SpendPoints` are thin wrappers that fix `kind` for you. Because it can credit,
it is operator-only — members spend via `Wallet.SpendPoints`. Same
request/response shape as above plus a `"kind"` field in params.

### `AccountService.GetAccountByID` — member (own) / admin (any)

```bash
curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{
  "jsonrpc": "2.0", "method": "AccountService.GetAccountByID",
  "params": [{ "account_id": "a91b..." }], "id": 1
}'
```

```json
{ "jsonrpc": "2.0", "result": { "id": "a91b...", "user_id": "0c5f...", "name": "Primary Wallet", "balance": 110, "created_at": "..." }, "id": 1 }
```

A member requesting an account they do not own gets `"account not found"` — a
non-owner cannot tell a foreign account from a missing one (no existence leak).

### `AccountService.GetAccountBalance` — member (own) / admin (any)

```json
// params: [{ "account_id": "a91b..." }]
// result: { "account_id": "a91b...", "balance": 110 }
```

### `AccountService.UpdateAccountName` — member (own) / admin (any)

```json
// params: [{ "account_id": "a91b...", "name": "Holiday Wallet" }]
// result: full account object (as GetAccountByID)
```

### `AccountService.UpdateAccountBalance` — admin only

A raw signed-delta adjustment that bypasses the ledger (operator correction).
The overdraft floor still applies. A member token is rejected.

```json
// params: [{ "account_id": "a91b...", "delta": -25 }]
// result: { "account_id": "a91b...", "balance": 85 }
```

### `AccountOpener.OpenAccount` — any authenticated user

Opens an additional wallet account for the calling user. The owner is **never on
the wire** — it is pinned to the user in the token, so a caller can only open an
account for themselves. `name` is optional; the service defaults it when blank.

```bash
curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{
  "jsonrpc": "2.0", "method": "AccountOpener.OpenAccount",
  "params": [{ "name": "Holiday Wallet" }], "id": 1
}'
```

```json
{ "jsonrpc": "2.0", "result": { "account_id": "b72c...", "name": "Holiday Wallet", "owner_id": "0c5f...", "balance": 0, "created_at": "..." }, "id": 1 }
```

### `Wallet.ProcessTransactionBatch` — admin only

Applies an ordered batch in one request and returns per-element outcomes plus
summary tallies. The server applies the transactions in slice order, so the
overdraft floor sees them in the order given. This is the method the CLI's
`ingest` command calls — see below.

```json
// params: [{ "transactions": [
//   { "ref": "tx-001", "account_id": "a91b...", "kind": "earn",  "points": 150 },
//   { "ref": "tx-002", "account_id": "a91b...", "kind": "spend", "points": 40 }
// ] }]
// result:
// {
//   "results": [
//     { "ref": "tx-001", "status": "accepted", "balance": 150 },
//     { "ref": "tx-002", "status": "accepted", "balance": 110 }
//   ],
//   "summary": { "accepted": 2, "duplicate": 0, "rejected": 0 }
// }
```

A rejected element carries its `status` (`"rejected"`) and a `reason`; the rest
of the batch still applies. Every attempt — accepted, duplicate, or rejected — is
written to the `audit_entries` table.

### `AuditService.ListByTransactionRef` — member (own) / admin (any)

Returns every recorded processing attempt for a transaction `ref`, oldest first.
Unlike the ledger, the same `ref` can appear multiple times here — one row per
attempt (accepted, duplicate, or rejected), each with its `reason`. The listing
is scoped to the caller: a member sees only attempts against their own accounts;
an admin holding `audit:read:all` sees every owner's. A `ref` the caller may not
see returns an **empty `entries` array**, not an error.

```bash
curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{
  "jsonrpc": "2.0", "method": "AuditService.ListByTransactionRef",
  "params": [{ "transaction_ref": "tx-001" }], "id": 1
}'
```

```json
{
  "jsonrpc": "2.0",
  "result": {
    "transaction_ref": "tx-001",
    "entries": [
      {
        "id": 42,
        "user_id": "0c5f...",
        "transaction_ref": "tx-001",
        "account_id": "a91b...",
        "owner_id": "0c5f...",
        "kind": "earn",
        "points": 150,
        "outcome": "accepted",
        "reason": "ok",
        "created_at": "..."
      }
    ]
  },
  "id": 1
}
```

---

## CLI: batch CSV ingestion

`loyalty-cli ingest` reads a CSV, sorts rows by `occurred_at` (then line), and
sends the whole ordered batch as one admin-only JSON-RPC request, then prints a
summary. The server applies them sequentially in that order, so the overdraft
floor sees transactions in true chronology.

**CSV format** (header is mandatory and exact):

```csv
ref,account_id,kind,points,occurred_at
tx-001,a91b...,earn,150,2024-06-01T10:00:00Z
tx-002,a91b...,spend,40,2024-06-01T11:00:00Z
```

`occurred_at` may be blank (the server stamps it). A non-blank, non-RFC3339
value is a local error and is never sent.

**Run it** (requires an admin token):

```bash
go run ./cmd/cli ingest \
  --file ./batch.csv \
  --url http://localhost:8080/api \
  --token "$ADMIN_TOKEN"

# Preview without sending anything:
go run ./cmd/cli ingest --file ./batch.csv --dry-run
```

**Summary output**

```
file:         batch.csv
processed:    2
accepted:     2
duplicates:   0
rejected:     0
```

Rejected rows (including the reason and the originating line) are listed beneath
the tallies. Every attempt — accepted, duplicate, or rejected — is also written
to the server's `audit_entries` table with its reason and timestamp, and is
queryable afterwards via `AuditService.ListByTransactionRef`.

---

## Repository layout

| Path                 | What lives there                                                            |
| -------------------- | --------------------------------------------------------------------------- |
| `cmd/app`            | Server entrypoint, config, dependency wiring, RPC setup                     |
| `cmd/cli`            | `loyalty-cli` — thin JSON-RPC client (notably `ingest`)                     |
| `cmd/bootstrap`      | Clean-slate dev tool: wipes data tables and (re)creates the admin user      |
| `pkg/<domain>`       | Ports: interfaces, DTOs, `Validate()`, JSON-RPC adaptors                    |
| `internal/pkg/<domain>` | PostgreSQL implementations of those ports                                |
| `pkg/postgres/migrations` | Embedded SQL schema, applied in lexical order on startup               |
| `tests/`             | Black-box HTTP/JSON-RPC integration tests                                   |
| `api/`               | Postman collection (`loyalty-points.postman_collection.json`)               |
| `scripts/`           | `start-stack.sh` — one-command local stack                                  |

A ready-to-import **Postman collection** lives at
[`api/loyalty-points.postman_collection.json`](./api/loyalty-points.postman_collection.json)
with every method above pre-built.

See [SOLUTION.md](./SOLUTION.md) for the architecture and the reasoning behind
it.