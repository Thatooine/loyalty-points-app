# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Spec

The authoritative product spec lives in `docs/specs/` (currently `docs/specs/loyalty-wallet.md`). It restates the brief as testable acceptance criteria and maps each to the test that proves it. Before implementing or changing behavior in the loyalty wallet — new RPC methods, ledger rules, validations, error semantics — read the relevant spec and make the change conform to it. If a change alters behavior the spec describes, update the spec (and its test mapping) in the same change.

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
go test ./internal/pkg/wallets/ -run TestProcessTransaction_DuplicateRef

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

- **`pkg/<domain>`** is the *port* side: interfaces, request/response DTOs, `Validate()` methods, and the JSON-RPC adaptors (the wire layer). Domains: `accounts`, `users`, `wallets`, `audit`, `authentication`, `authorization`. (`authorization` is policy-only — there is no `internal` impl for it.)
- **`internal/pkg/<domain>`** holds the Postgres *implementations* of those interfaces. Production code depends only on the `pkg` interfaces; `cmd/app/serviceProviders.go` is the single wiring point that injects the `internal` impls.
- **Cross-cutting `pkg/` packages** (no domain of their own): `errs` (sentinel errors + the `ValidationError` type and `WithMessage` wrapper), `jsonrpc` (canonical error codes + the `MapError` codec mapper), `logger` (request-scoped logging middleware), `scope` (permission-string parsing), `sql` (the context-resolved executor + `TxManager`), `postgres` (driver, migrations, SQLSTATE helpers), and `time` (the RFC3339Nano text format).

When adding a repository/service method you touch both: the interface + DTOs + validation in `pkg`, the SQL in `internal/pkg`.

### WalletService is the heart of the system

Every ledger write flows through `WalletServiceImpl.ProcessTransaction` (`internal/pkg/wallets/walletServiceImpl.go`), which composes the account, transaction, and audit repositories inside **one unit of work** so the invariants hold and are tested in exactly one place:

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

### Error handling: sentinels in, codes out, mapped once

Errors are translated to the wire in exactly one place. Handlers and services **never build JSON-RPC errors themselves** — they return the domain sentinels in `pkg/errs` (`ErrNotFound`, `ErrForbidden`, `ErrUnauthorized`, `ErrAlreadyExists`, `ErrInsufficientBalance`, `ErrInvalidArgument`, `ErrInternal`). The codec is registered with `jsonrpc.MapError` (via gorilla's `NewCustomCodecWithErrorMapper` in `setupRPCServer.go`), which is the single switch from sentinel → JSON-RPC code + machine-readable `data.reason`. Codes are defined once in `pkg/jsonrpc/error.go` and shared by both the codec and the `authorizationMiddleware` (which writes errors before a handler runs).

Conventions when adding/extending an adaptor:
- To attach a client-facing message while keeping the code, wrap with `errs.WithMessage(errs.ErrX, "friendly message")` — `Error()` is the message, `Unwrap()` is the sentinel that `MapError` matches on. A bare sentinel works too (its own text becomes the message).
- The default/unmapped branch returns a fixed `"internal server error"` so an unexpected error never leaks internals; use `errs.WithMessage(errs.ErrInternal, …)` when you *want* a safe, specific internal message.
- Validation: `Validate()` returns `errs.NewValidationError(reasons)` (a `*errs.ValidationError` that unwraps to `ErrInvalidArgument`). `MapError` surfaces it as code `-32602` with the per-field reasons under `data.fields`. Because services wrap with `%w`, `errors.As`/`errors.Is` still reach it at the codec.
- Keep the existing `log.Ctx(ctx)…` line at the point of failure; logging is separate from the mapping.

### Logging & observability

`logger.Middleware` (`pkg/logger`, mounted first on `/api`) mints a `request_id` (honoring an inbound `X-Request-ID`, else a UUID), binds it into the zerolog context so every downstream `log.Ctx(ctx)…` line is correlated, echoes it back as the `X-Request-ID` response header, and emits one structured access-log line per request (status + duration). Always log through `log.Ctx(r.Context())`, never the global logger, so the request id rides along.

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
