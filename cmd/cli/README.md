# loyalty-cli

A thin command-line client for the loyalty-points JSON-RPC API. It holds **no
business logic** — idempotency, the overdraft floor, and the audit trail all
live on the server and are reached through the same transaction core the
single-transaction API uses. The CLI's only real job is to turn a CSV file into
one correctly *ordered* batch request.

## Build / run

```bash
# Run straight from source
go run ./cmd/cli --help

# Or build a binary
go build -o loyalty-cli ./cmd/cli
./loyalty-cli --help
```

## Global flags

These are available on every subcommand:

| Flag      | Default                        | Purpose                                      |
| --------- | ------------------------------ | -------------------------------------------- |
| `--url`   | `http://localhost:8080/api`    | JSON-RPC endpoint URL                        |
| `--token` | _(empty)_                      | Bearer access token for authenticated calls  |

## Getting an admin token

Batch ingestion is **admin-only**, so you need an admin bearer token. The
bundled `cmd/bootstrap` tool creates a well-known admin principal:

- email: `system@mail.com`
- password: `admin-user-123`

```bash
# 1. Bring up Postgres and the server
docker compose up -d
go run ./cmd/app &                      # JSON-RPC server on :8080/api

# 2. (Optional) reset the DB and (re)create the admin user
go run ./cmd/bootstrap

# 3. Log in and capture the token
ADMIN_TOKEN=$(curl -s http://localhost:8080/api \
  -H 'Content-Type: application/json' \
  -d '{
        "jsonrpc": "2.0",
        "method": "EmailPasswordAuthenticator.Login",
        "params": [{"email": "system@mail.com", "password": "admin-user-123"}],
        "id": 1
      }' | jq -r '.result.token')
```

## `ingest` — batch CSV ingestion

Reads a CSV file, **sorts the rows by `occurred_at` (then original line)**, and
sends the whole ordered batch as a single JSON-RPC request to
`Wallet.ProcessTransactionBatch`. The server applies the transactions
sequentially in exactly that order, so the order-dependent overdraft floor sees
each account's transactions in their true chronology (e.g. an earn before the
spend it funds), regardless of the file's row order.

> Why one ordered request instead of a JSON-RPC batch array? The JSON-RPC 2.0
> spec lets a server process a batch in any order and concurrently, and the
> gorilla json2 codec does not decode batch arrays at all. Since every write is
> order-dependent, the whole batch travels as one request whose slice order is
> authoritative.

### CSV format

The header is **mandatory and exact**:

```csv
ref,account_id,kind,points,occurred_at
tx-001,11111111-1111-1111-1111-111111111111,earn,200,2026-06-15T09:00:00Z
tx-002,11111111-1111-1111-1111-111111111111,spend,150,2026-06-15T12:00:00Z
tx-003,33333333-3333-3333-3333-333333333333,earn,50,
```

| Column        | Notes                                                                          |
| ------------- | ------------------------------------------------------------------------------ |
| `ref`         | Idempotency key. A repeat (in-file or already stored) is reported a duplicate. |
| `account_id`  | UUID of the target account.                                                    |
| `kind`        | `earn` or `spend`. `spend` subtracts; everything else adds.                    |
| `points`      | Integer point amount.                                                          |
| `occurred_at` | RFC 3339, or **blank** to let the server stamp it at processing time.          |

Local validation runs before anything is sent: a non-numeric `points` or a
non-blank, non-RFC3339 `occurred_at` is a **local error** — that row is reported
and never sent. A wrong header is fatal.

A ready-made example lives at
[`testdata/sample_batch.csv`](./testdata/sample_batch.csv).

### Flags

| Flag        | Default                          | Purpose                                       |
| ----------- | -------------------------------- | --------------------------------------------- |
| `--file`    | _(required)_                     | Path to the CSV batch file                    |
| `--dry-run` | `false`                          | Build and print the request without sending   |
| `--method`  | `Wallet.ProcessTransactionBatch` | JSON-RPC batch method to call                 |

### Examples

```bash
# Send a batch (requires an admin token)
go run ./cmd/cli ingest \
  --file ./cmd/cli/testdata/sample_batch.csv \
  --url http://localhost:8080/api \
  --token "$ADMIN_TOKEN"

# Preview the exact request and apply-order without sending anything
go run ./cmd/cli ingest --file ./cmd/cli/testdata/sample_batch.csv --dry-run
```

### Output

A real run prints a tally, with any rejected rows (server-side rejections *and*
local parse errors) listed beneath, each carrying its originating line number:

```
file:         sample_batch.csv
processed:    7
accepted:     5
duplicates:   1
rejected:     1

rejections:
  row 6 (ref txn-0006): insufficient balance
```

A `--dry-run` additionally prints the marshalled request, the ordered
apply-table the server would process top-to-bottom, and a client-computable
breakdown (account count, earn/spend totals, in-file duplicate refs). It
deliberately reports **no** accepted/duplicate/rejected counts, since those
depend on server state (current balances, refs already recorded, ownership) and
are only knowable from a real run.

Every attempt — accepted, duplicate, or rejected — is also written to the
server's `audit_entries` table and is queryable afterwards via
`AuditService.FetchTransactionAuditTrail`.

The process exits non-zero if the file can't be opened, the header is invalid,
or the request can't be sent.

## Package layout

| Path                      | Responsibility                                                  |
| ------------------------- | --------------------------------------------------------------- |
| `cmd/cli/main.go`         | Entrypoint; maps a command error to a non-zero exit code.       |
| `cmd/cli/cmd/`            | Cobra command tree (`root`, `ingest`) and flag wiring.          |
| `cmd/cli/pkg/ingest/`     | The reusable, tested core: CSV parsing, sorting, request build, HTTP send, and summary/preview formatting. |

The `ingest` package is plain library code with no cobra dependency, so its
parsing, ordering, and summarising logic is unit-tested directly
([`ingest_test.go`](./pkg/ingest/ingest_test.go)).