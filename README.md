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

JSON-RPC errors are returned with **HTTP 200**; the error lives in the response
body (`{"jsonrpc":"2.0","error":{"code":...,"message":...},"id":...}`). Codes:
`-32700` parse error, `-32001` unauthorized, and `-32002` forbidden come from the
auth middleware; business errors raised by a method (e.g. insufficient balance,
account not found) come back as `-32000` (the gorilla json2 server-error code)
with the message in the body.

### Roles in one line

- **member** — read their own account and **spend** from it (default for every new registration). Members cannot credit points.
- **admin** — read/adjust *any* account, **credit** points (earn / process), and run batch ingestion.

See [SOLUTION.md](./SOLUTION.md#access-control) for how a user becomes an admin.

---

## Endpoints

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
{ "jsonrpc": "2.0", "error": { "code": -32000, "message": "insufficient balance" }, "id": 1 }
```

### `Wallet.ProcessTransaction` — admin only

The general earn/spend method; `kind` is `"earn"` or `"spend"`. `EarnPoints` and
`SpendPoints` are thin wrappers that fix `kind` for you. Because it can credit,
it is operator-only — members spend via `Wallet.SpendPoints`. Same
request/response shape as above plus a `"kind"` field in params.

### `Account.GetByID` — member (own) / admin (any)

```bash
curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{
  "jsonrpc": "2.0", "method": "Account.GetByID",
  "params": [{ "account_id": "a91b..." }], "id": 1
}'
```

```json
{ "jsonrpc": "2.0", "result": { "id": "a91b...", "user_id": "0c5f...", "name": "Primary Wallet", "balance": 110, "created_at": "..." }, "id": 1 }
```

A member requesting an account they do not own gets `"account not found"` — a
non-owner cannot tell a foreign account from a missing one (no existence leak).

### `Account.GetAccountBalance` — member (own) / admin (any)

```json
// params: [{ "account_id": "a91b..." }]
// result: { "account_id": "a91b...", "balance": 110 }
```

### `Account.UpdateAccountName` — member (own) / admin (any)

```json
// params: [{ "account_id": "a91b...", "name": "Holiday Wallet" }]
// result: full account object (as GetByID)
```

### `Account.UpdateAccountBalance` — admin only

A raw signed-delta adjustment that bypasses the ledger (operator correction).
The overdraft floor still applies. A member token is rejected.

```json
// params: [{ "account_id": "a91b...", "delta": -25 }]
// result: { "account_id": "a91b...", "balance": 85 }
```

### `Wallet.ProcessTransactionBatch` — admin only

Applies an ordered batch in one request and returns per-element outcomes plus
summary tallies. This is the method the CLI's `ingest` command calls — see
below.

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
to the server's `audit_log` table with its reason and timestamp.

---

## Repository layout

| Path                 | What lives there                                                            |
| -------------------- | --------------------------------------------------------------------------- |
| `cmd/app`            | Server entrypoint, config, dependency wiring, RPC setup                     |
| `cmd/cli`            | `loyalty-cli` — thin JSON-RPC client (notably `ingest`)                     |
| `pkg/<domain>`       | Ports: interfaces, DTOs, `Validate()`, JSON-RPC adaptors                    |
| `internal/pkg/<domain>` | PostgreSQL implementations of those ports                                |
| `pkg/postgres/migrations` | Embedded SQL schema, applied in lexical order on startup               |
| `tests/`             | Black-box HTTP/JSON-RPC integration tests                                   |

See [SOLUTION.md](./SOLUTION.md) for the architecture and the reasoning behind
it.