# CLAUDE.md — Technical Context for AI Agents

This file documents the architecture, key files, known gotchas, and current state of the
**loyalty-points-app** so that any AI coding agent (and Claude Code specifically) can continue
development without prior context.

IMPORTANT: the conventions and gotchas below OVERRIDE default behavior — follow them exactly.

---

## What Is This?

A **loyalty-points ledger** exposed as a **JSON-RPC 2.0 API** over HTTP, written in Go using a
**ports-and-adapters** (hexagonal) design. Members hold accounts; points are earned and spent
through an idempotent, overdraft-safe ledger; an audit trail records every attempt. A thin CLI
(`loyalty-cli`) batch-loads transactions from CSV. Originally built as the *Sanlam Senior SWE
take-home* (`scratch/Sanlam … Assignment.pdf`).

- **Module:** `github.com/Thatooine/loyalty-points-app`
- **Go:** 1.25.0
- **Server binary:** `./cmd/app` — JSON-RPC server on `:8080/api`
- **CLI binary:** `./cmd/cli` (`loyalty-cli`) — thin client, holds no business logic
- **Bootstrap:** `./cmd/bootstrap` — seeds the root/admin user
- **DB:** PostgreSQL via `pgx` stdlib driver
- No Makefile, no linter config, no codegen, no CI workflows, no version tags — plain `go` tooling.

---

## Spec (authoritative)

The product spec lives at `SPEC.md` (project root). It restates the
brief as testable acceptance criteria and maps each to the test that proves it, with a recorded
conformance verdict (✅ test-proven / ⚠️ by-design / 📄 doc deliverable).

**Before implementing or changing loyalty-wallet behavior** — new RPC methods, ledger rules,
validations, error semantics — read the relevant spec and make the change conform. **If a change
alters behavior the spec describes, update the spec and its test mapping in the same change.**

To re-run the conformance pass: bring the environment fully up (below), run the suite, and confirm
tests actually **RAN** — a skipped DB/endpoint test proves nothing (see Gotcha #1).

---

## Build & Test

```bash
go build ./...                 # build everything
go vet ./...                   # vet
go test ./...                  # tests; DB-backed tests SKIP when TEST_POSTGRES_DSN is unset

# DB-backed tests need a real Postgres. Start the bundled one and point tests at it:
docker compose up -d
export TEST_POSTGRES_DSN='postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable'
go test ./...

# The tests/ integration suite ALSO needs a running server (it skips otherwise):
go run ./cmd/app                                       # in another shell
export LOYALTY_API_URL='http://localhost:8080/api'
go test ./tests/ -v -count=1                           # -count=1 to bypass cache; -race for concurrency tests

# Run a single test
go test ./internal/pkg/wallets/ -run TestProcessTransaction_DuplicateRef

# Run the server (DSN + dev JWT key baked into cmd/app/config.go, matching docker-compose):
docker compose up -d
go run ./cmd/app                # auto-applies migrations on startup

# Run the CLI client
go run ./cmd/cli --help
```

---

## Repository Layout

The split that matters: **`pkg/<domain>` = ports** (interfaces, DTOs, `Validate()`, wire adaptors);
**`internal/pkg/<domain>` = Postgres implementations** of those ports. Production code depends only
on `pkg` interfaces; `cmd/app/serviceProviders.go` is the single wiring point that injects impls.

```
cmd/
  app/              — server entry; config.go (baked defaults), serviceProviders.go (DI), setupRPCServer.go (gorilla mount)
  bootstrap/        — seeds the root/admin user
  cli/              — loyalty-cli (cobra); pkg/ingest = CSV parse/sort/preview/summary
internal/pkg/       — Postgres IMPLEMENTATIONS (depend only on pkg interfaces)
  accounts/  audit/  authentication/  users/  wallets/   (rewards/ = empty stub, not wired)
pkg/                — PORTS: interfaces, DTOs, Validate(), JSON-RPC adaptors
  accounts/ users/ wallets/ audits/ authentication/   — domains
  authorization/    — policy-only (no internal impl): middleware, policy.go, permissions.go, scope
  errs/             — sentinel errors + ValidationError + WithMessage wrapper
  jsonrpc/          — canonical error codes + MapError codec mapper
  logger/           — request-scoped logging middleware
  scope/            — permission-string parsing (resource:action:scope)
  sql/              — context-resolved executor + TxManager (unit of work)
  postgres/         — driver, migrations (embedded), SQLSTATE helpers
  time/             — RFC3339Nano UTC TEXT format
tests/              — black-box HTTP/JSON-RPC integration tests (need running server)
SPEC.md             — authoritative spec + conformance verdicts (project root)
```

When adding a repository/service method you touch **both** sides: interface + DTO + `Validate()` in
`pkg`, SQL in `internal/pkg`. (The `add-rpc-method`, `scaffold-service`, and
`scaffold-crud-repository` skills automate this — including the easy-to-forget Policy entry.)

---

## The Data Model

Schema lives in `pkg/postgres/migrations/*.sql` (currently one file, `0001_init.sql`), embedded and
applied in **lexical filename order** on startup and in tests. All timestamps are **RFC3339Nano UTC
TEXT** — human-readable *and* lexicographically sortable, which ordering and keyset pagination rely on.

| Table | Key columns & invariants |
|---|---|
| `users` | `id` (UUID), `email` UNIQUE, `password_hash` (bcrypt), `role` ∈ {member, admin}, `token_version` (bumped on logout to revoke all outstanding JWTs) |
| `accounts` | `id`, `owner_id`→users, `name`, `balance` BIGINT `CHECK (balance >= 0)` — DB-level overdraft backstop |
| `transactions` | `id`, **`ref` UNIQUE — the UNIQUE constraint IS the idempotency/dedupe mechanism**, `account_id`, `owner_id` (denormalised), `kind` ∈ {earn, spend}, `points` (signed delta: earn=+n, spend=−n), `occurred_at` (business time), `recorded_at` (server time), `created_by` (acting principal) |
| `audit_entries` | identity PK; `transaction_ref`/`account_id`/`owner_id`/`kind`/`points` all **nullable** (a rejected/malformed attempt may lack them); `outcome` ∈ {accepted, rejected, duplicate}, `reason`, `user_id` (submitter), `created_at` |

Keyset pagination orders by `(recorded_at DESC, ref)`; the `idx_transactions_owner_keyset` composite
also serves owner-scoped lookups via its leftmost column.

### OwnerID is always a user

`OwnerID` is a FK to `users.id`. The **DTO field is `UserID`; the SQL column is `owner_id`.** It is
denormalised onto `transactions` and `audit_entries` so entries attribute without a join. For a
member action it equals the actor; **for an admin action it is the account owner, not the acting admin**.

---

## Critical Design Decisions

### WalletService is the heart of the system

Every ledger write flows through `WalletServiceImpl.ProcessTransaction`
(`internal/pkg/wallets/walletServiceImpl.go`), which composes the account, transaction, and audit
repositories inside **one unit of work** so the invariants hold and are tested in exactly one place:

- **Idempotency** — the ledger insert is attempted first; the `UNIQUE(ref)` constraint *is* the
  dedupe mechanism (**never check-then-insert**). A duplicate returns the original outcome with `Duplicate=true`.
- **Overdraft floor** — a single guarded `UPDATE … WHERE balance + delta >= 0` makes read-check-write
  atomic; zero rows affected means insufficient balance *or* missing/unowned account. This atomicity
  is what makes concurrent requests safe (proven by `tests/TestConcurrentSpendsRespectOverdraftFloor`).
- **Audit trail** — accepted/duplicate rows commit inside the unit of work; **rejected rows are
  written on the plain context** so the trail survives the rolled-back transaction.

`EarnPoints`, `SpendPoints`, and `ProcessTransactionBatch` are thin wrappers that build a request
(fixing `Kind`) and delegate — there is no second write path to keep in sync.

### Unit of work via context-resolved executor

Repositories never hold a `*sql.Tx`. They call `pkgSQL.ExecutorFromContext(ctx, r.db)` (`pkg/sql`),
which returns the ambient transaction if `TxManager.RunInTx` put one on the context, else the pool.
Wrapping several repository calls in `RunInTx` makes them atomic; the same repository code works
standalone or composed.

### Two-layer authorization

1. **Method gate** — `authorizationMiddleware` (`pkg/authorization`) peeks the JSON-RPC method out of
   the request body and consults a `Policy` (method → permissions + public-method set). All-or-nothing,
   resolves *no* scope. **New protected RPC methods MUST be added to `DefaultPolicy()` in
   `pkg/authorization/policy.go` or they are rejected** (Gotcha #2).
2. **Ownership scope** — enforced in the data layer. Repositories call `authorization.IsGranted(ctx,
   Perm…All)`; without the `:all` permission the SQL is scoped to `owner_id = $UserID`, and a non-owner
   gets `ErrNotFound` (indistinguishable from missing — no existence leak). Permissions are
   `resource:action:scope` strings; roles map to fixed permission sets in `permissions.go`; the set is
   embedded in the JWT login claim placed on the context by the middleware.

### Transport: gorilla/rpc/v2

The RPC server uses `github.com/gorilla/rpc/v2` (not a hand-rolled dispatcher). Adaptor methods with
signature `func(r *http.Request, params *T, result *T2) error` are auto-registered as
`<ServiceName>.<Method>` (service name from the adaptor's `Name()`). All services — public and
protected — mount on the single `/api` endpoint (`cmd/app/setupRPCServer.go`).

### Error handling: sentinels in, codes out, mapped once

Handlers and services **never build JSON-RPC errors themselves** — they return domain sentinels from
`pkg/errs` (`ErrNotFound`, `ErrForbidden`, `ErrUnauthorized`, `ErrAlreadyExists`,
`ErrInsufficientBalance`, `ErrInvalidArgument`, `ErrInternal`). The codec registered with
`jsonrpc.MapError` (via gorilla's `NewCustomCodecWithErrorMapper`) is the single switch from sentinel
→ JSON-RPC code + machine-readable `data.reason`. Codes are defined once in `pkg/jsonrpc/error.go`.

- Attach a client-facing message while keeping the code: `errs.WithMessage(errs.ErrX, "friendly msg")`
  — `Error()` is the message, `Unwrap()` is the sentinel `MapError` matches on. A bare sentinel works too.
- The default/unmapped branch returns a fixed `"internal server error"` so nothing leaks; use
  `errs.WithMessage(errs.ErrInternal, …)` for a safe, specific internal message.
- `Validate()` returns `errs.NewValidationError(reasons)` (unwraps to `ErrInvalidArgument`) → surfaced
  as code `-32602` with per-field reasons under `data.fields`. Services wrap with `%w` so
  `errors.As`/`errors.Is` still reach it at the codec.

### Logging & observability

`logger.Middleware` (mounted first on `/api`) mints a `request_id` (honoring inbound `X-Request-ID`,
else a UUID), binds it into the zerolog context, echoes it back as `X-Request-ID`, and emits one
structured access-log line per request. **Always log through `log.Ctx(r.Context())`, never the global
logger**, so the request id rides along.

---

## Known Gotchas

1. **A skipped test proves nothing.** DB-backed tests SKIP when `TEST_POSTGRES_DSN` is unset; the
   `tests/` suite SKIPs when no server is reachable. A green `ok`/`PASS` summary can be *all skips*.
   Before claiming conformance, bring up `docker compose` + `go run ./cmd/app`, set both env vars, and
   verify the tests actually `RUN`.
2. **New protected RPC method → add it to `DefaultPolicy()`** in `pkg/authorization/policy.go`, or the
   method gate rejects it. The single most-forgotten step when adding an endpoint.
3. **Never check-then-insert for idempotency.** Attempt the insert and let `UNIQUE(ref)` dedupe. A
   check-then-insert reintroduces a race the design specifically eliminates.
4. **Rejected audit rows go on the plain context, not the tx** — otherwise they vanish when the unit
   of work rolls back, and the trail loses every rejection.
5. **Members cannot earn.** `PermWalletTransactOwn` unlocks `SpendPoints` only; `EarnPoints` /
   `ProcessTransaction` credit paths are operator-only so a member can't mint points into their own
   account (commit `8eb7c2b`). This is a deliberate divergence from a literal reading of the brief —
   see spec row A-2.
6. **Timestamps are RFC3339Nano UTC TEXT** (`pkg/time`), not native timestamps. Ordering and keyset
   pagination depend on lexical sortability — do not switch column types or store local-tz values.
7. **The `tests/` server is persistent across runs**, and `ref`/`email` carry UNIQUE constraints.
   Always mint fresh values via `uniqueRef(t)` / `uniqueEmail(t)`, or a rerun dedupes against prior rows.
8. **`t.Fatalf` must only be called from the test goroutine.** Concurrency tests use the goroutine-safe
   `apiClient.callRaw` (returns an error) and assert on the test goroutine after `wg.Wait()`.
9. **Migrations apply in lexical filename order.** Number new files (`0002_…`, …) to evolve the schema.
10. **`OwnerID` ≠ acting admin.** For admin actions the denormalised `owner_id` is the *account
    owner*; `created_by` is the admin. Don't conflate them.
11. **`rewards/` is an empty stub** (both `pkg` and `internal/pkg`) — not wired into anything yet.

---

## Tests

Two tiers, both runnable with plain `go test`:

- **Pure-logic & mock-first** (`pkg/**`, `internal/pkg/**`, `cmd/cli/pkg/ingest`) — validations,
  cursor codecs, scope parsing, service logic over hand-written mocks, adaptor behavior. No DB.
- **DB-backed** — call `testsupport.NewPostgresDB(t)`, which provisions a **fresh isolated schema per
  test** (so cross-package parallel runs don't collide), migrates into it, and drops it on cleanup.
- **Black-box integration** (`tests/`) — real HTTP/JSON-RPC against a running server, asserting both
  wire responses *and* DB persistence. Shared harness in `tests/harness_test.go` (`setup`, `call`,
  `callRaw`); register/login helpers; mandatory negative cases (unauthenticated, foreign-account,
  overdraft, member-forbidden).

Skills available for test work: `go-unit-tests`, `endpoint-integration-test`.

---

## Recent History

No version tags; this summarizes notable commits (newest first) so an agent grasps recent direction:

| Commit | Change |
|---|---|
| `76136c4` | Rename `ListByTransactionRef` → `FetchTransactionAuditTrail` across service/adaptors/tests/docs |
| `2d04b19` | Migrate `AuditEntryRepositoryJSONRPCAdaptor` → `AuditServiceJSONRPCAdaptor` |
| `dad8f96` | Introduce `AccountService` interface with validations, mock, impl |
| `2330d13` | Migrate audit logging to the `audit_entries` schema |
| `f151505` | Centralize JSON-RPC error handling; improve validation error reporting |
| `0abf305` / `51aa264` | Logout + token-versioning for session revocation |
| `8eb7c2b` | **Restrict point crediting to operator-only** (members can spend, not earn — see Gotcha #5) |
| `02baee1` | Replace `SystemUserID` with `RootUserID` |

---

## Code Formatting Rules

- Do NOT add explanatory comments for obvious or self-documenting code.
- Only add comments if highly complex algorithmic logic is used, and focus on "why", not "what".
- Do not generate block-level docstrings for every single function or class unless explicitly requested.
