# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...                 # build everything
go vet ./...                   # vet
go test ./...                  # run tests (DB-backed tests SKIP when TEST_POSTGRES_DSN is unset)

# DB-backed tests need a real Postgres. Start the bundled one and point tests at it:
docker compose up -d
export TEST_POSTGRES_DSN='postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable'
go test ./...

# Run a single test
go test ./internal/pkg/wallet/ -run TestProcessTransaction_DuplicateRef

# Run the server (defaults — DSN and a dev JWT key — are baked into cmd/app/config.go,
# matching the docker-compose container, so this works out of the box once Postgres is up)
docker compose up -d
go run ./cmd/app                # JSON-RPC server on :8080/api; auto-applies migrations on startup

# Run the CLI client (thin JSON-RPC client; all business rules live on the server)
go run ./cmd/cli --help
```

There is no Makefile, linter config, or codegen step — plain `go` tooling only.

## Architecture

A loyalty-points ledger exposed as a JSON-RPC 2.0 API. The design is ports-and-adapters, and a handful of cross-cutting patterns matter far more than any single file.

### Package layout: ports vs. implementations

- **`pkg/<domain>`** is the *port* side: interfaces, request/response DTOs, `Validate()` methods, and the JSON-RPC adaptors (the wire layer). Domains: `accounts`, `users`, `wallet`, `audit`, `authentication`, `authorization`.
- **`internal/pkg/<domain>`** holds the Postgres *implementations* of those interfaces. Production code depends only on the `pkg` interfaces; `cmd/app/serviceProviders.go` is the single wiring point that injects the `internal` impls.

When adding a repository/service method you touch both: the interface + DTOs + validation in `pkg`, the SQL in `internal/pkg`.

### WalletService is the heart of the system

Every ledger write flows through `WalletServiceImpl.ProcessTransaction` (`internal/pkg/wallet/walletServiceImpl.go`), which composes the account, transaction, and audit repositories inside **one unit of work** so the invariants hold and are tested in exactly one place:

- **Idempotency** — the ledger insert is attempted first; the `UNIQUE(ref)` constraint *is* the dedupe mechanism (never check-then-insert). A duplicate returns the original outcome with `Duplicate=true`.
- **Overdraft floor** — a single guarded `UPDATE ... WHERE balance + delta >= 0` makes the read-check-write atomic; zero rows affected means insufficient balance or missing/unowned account.
- **Audit trail** — accepted/duplicate rows commit inside the unit of work; rejected rows are written on the plain context so the trail survives the rolled-back transaction.

`EarnPoints`, `SpendPoints`, and `ProcessTransactionBatch` are thin wrappers that construct a request (fixing `Kind`) and delegate, so there is no second write path to keep in sync.

### Unit of work via context-resolved executor

Repositories never hold a `*sql.Tx`. They call `pkgSQL.ExecutorFromContext(ctx, r.db)` (`pkg/sql`), which returns the ambient transaction if `TxManager.RunInTx` put one on the context, else the pool. This is how a service runs several repository calls atomically: wrap them in `RunInTx` and they all share the transaction. The same repository code works standalone or composed.

### Two-layer authorization

1. **Method gate** — `authorizationMiddleware` (`pkg/authorization`) peeks the JSON-RPC method out of the request body, then consults a `Policy` (method → permissions, plus a public-method set). This is all-or-nothing and resolves *no* scope. New protected RPC methods must be added to `DefaultPolicy()` in `pkg/authorization/policy.go` or they are rejected.
2. **Ownership scope** — enforced in the data layer, on demand. Repositories call `authorization.IsGranted(ctx, Perm...All)`; without the `:all` permission the SQL is scoped to `owner_id = $UserID`, and a non-owner gets `ErrNotFound` (indistinguishable from missing — no existence leak). Permissions are `resource:action:scope` strings (`pkg/scope` parses them); roles map to fixed permission sets in `permissions.go`; the set is embedded in the JWT login claim, which the middleware places on the context.

### OwnerID is always a user

`OwnerID` is a foreign key to `users.id`. The request/DTO field is named `UserID`; the SQL column is `owner_id`. It is denormalized onto the `transactions` and `audit_log` tables so entries can be attributed without a join. For a member action it equals the actor; for an admin action it is the account owner, not the acting admin.

### Transport: gorilla/rpc/v2

The RPC server uses `github.com/gorilla/rpc/v2` (not a hand-rolled dispatcher). Adaptor methods with the signature `func(r *http.Request, params *T, result *T2) error` are auto-registered as `<ServiceName>.<Method>` (the service name comes from the adaptor's `Name()`). All services — public and protected — mount on the single `/api` endpoint (`cmd/app/setupRPCServer.go`).

### Persistence

- Postgres via `pgx` stdlib driver. Schema lives in `pkg/postgres/migrations/*.sql`, embedded and applied in **lexical filename order** on startup and in tests (`postgres.Migrate`). Add a new numbered file to evolve the schema.
- Timestamps are stored as **RFC3339Nano UTC TEXT** (`pkg/time`), chosen because it is human-readable and lexicographically sortable — ordering and keyset pagination rely on this.

### Tests

DB-backed tests call `testsupport.NewPostgresDB(t)`, which provisions a **fresh isolated schema** per test (so cross-package parallel runs don't collide), migrates into it, and drops it on cleanup. They skip cleanly when `TEST_POSTGRES_DSN` is unset, so `go test ./...` stays green without a database. Pure-logic tests (validations, cursor codecs, scope parsing) need no DB.

### CLI

`cmd/cli` (`loyalty-cli`) is a thin cobra client over the same JSON-RPC endpoint — notably `ingest` for batch transaction loading. It holds no business logic; the server is authoritative. It sorts batches by `OccurredAt` before sending because the server applies them in slice order.

### Code Formatting Rules
- Do NOT add explanatory comments for obvious or self-documenting code.
- Only add comments if highly complex algorithmic logic is used, and focus on "why", not "what".
- Do not generate block-level docstrings for every single function or class unless explicitly requested.
