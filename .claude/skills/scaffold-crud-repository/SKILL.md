---
name: scaffold-crud-repository
description: Given a domain entity struct (e.g. pkg/users/user.go), scaffold its full repository layer — the port interface + CRUD DTOs, the Validate() methods, the Postgres implementation, the hand-written mock, and the pure-Go unit tests (table-driven validations). Use when adding persistence for a new entity. OUT OF SCOPE — does NOT create the JSON-RPC adaptor, the authorization Policy entry, or cmd/app wiring (use add-rpc-method for those).
---

# Scaffold a CRUD repository for an entity

Given one entity file `pkg/<domain>/<entity>.go` (a struct like `Account` or `User`), generate the whole repository layer following this repo's ports-and-adapters conventions. The canonical reference to mirror in every detail is the **accounts** domain (`pkg/accounts/accountRepository*.go` + `internal/pkg/accounts/accountRepository*.go`).

## What this skill produces (and what it does NOT)

Produces five files:

| File | Layer |
|------|-------|
| `pkg/<domain>/<entity>Repository.go` | interface + CRUD request/response DTOs |
| `pkg/<domain>/<entity>RepositoryValidations.go` | `Validate()` per request |
| `pkg/<domain>/<entity>RepositoryValidations_test.go` | pure-Go table-driven validation tests |
| `internal/pkg/<domain>/<entity>RepositoryImpl.go` | Postgres implementation |
| `internal/pkg/<domain>/<entity>RepositoryMockImpl.go` | hand-written mock |

**Out of scope — do NOT create these** (they belong to `add-rpc-method`):
- the JSON-RPC adaptor (`<entity>JSONRPCAdaptor.go`)
- the `authorization.DefaultPolicy()` entry
- `cmd/app/serviceProviders.go` / `setupRPCServer.go` wiring

State this boundary back to the user when you finish, so they know the entity is not yet reachable over the wire.

## Step 0 — read the entity and decide two things

Read `pkg/<domain>/<entity>.go`. From it derive:

1. **Names.** Package = `<domain>` (e.g. `accounts`). Entity = the struct name (e.g. `Account`). Use the entity name to build method/DTO names; use a lowercase plural for the SQL table (`accounts`, `users`).
2. **Ownership.** Does the struct have an owner field — `OwnerID string` (FK to `users.id`, see `[[ownerid-is-a-user]]`)?
   - **Owned entity** (like `Account`): reads/writes are ownership-scoped. Every scoped DTO carries a `UserID string`; the impl scopes on `owner_id` unless the caller holds the `:all` permission. This is the default and the richer template below.
   - **Top-level entity** (like `User`: no `OwnerID`): no per-row ownership scoping. Drop the `UserID` fields, the `authorization.IsGranted` branches, and the `owner_id` column from every query. Keep everything else.

Also confirm a **migration** exists for the table (`pkg/postgres/migrations/NNNN_*.sql`) with columns matching the struct fields. If none exists, tell the user the impl needs one to run, and offer to add the next numbered file — the column list must match the `INSERT`/`SELECT` you generate. Timestamps are stored as RFC3339Nano UTC TEXT (`pkg/time`), IDs are `TEXT PRIMARY KEY` UUIDs.

## Step 1 — interface + DTOs (`pkg/<domain>/<entity>Repository.go`)

Mirror `pkg/accounts/accountRepository.go`. Generate the five CRUD methods:

```go
package <domain>

import "context"

type <Entity>Repository interface {
	Create(ctx context.Context, request Create<Entity>Request) (*Create<Entity>Response, error)
	GetByID(ctx context.Context, request Get<Entity>ByIDRequest) (*Get<Entity>ByIDResponse, error)
	List(ctx context.Context, request List<Entity>sRequest) (*List<Entity>sResponse, error)
	Update(ctx context.Context, request Update<Entity>Request) (*Update<Entity>Response, error)
	Delete(ctx context.Context, request Delete<Entity>Request) (*Delete<Entity>Response, error)
}
```

DTO rules:
- `Create<Entity>Request{ <Entity> <Entity> }` / `...Response{ <Entity> <Entity> }`.
- `Get<Entity>ByIDRequest{ <Entity>ID string; UserID string }` / `...Response{ <Entity> <Entity> }`.
- `List<Entity>sRequest{ UserID string }` / `...Response{ <Entity>s []<Entity> }`.
- `Update<Entity>Request` carries `<Entity>ID string`, the mutable field(s) from the struct, and `UserID string`. `...Response{ <Entity> <Entity> }`.
- `Delete<Entity>Request{ <Entity>ID string; UserID string }` / `...Response{}` (empty struct is fine).
- **Owned:** keep the `UserID string` field on every read/write DTO with the exact doc comment style from `accountRepository.go` ("UserID, when non-empty, scopes ... Leave empty for internal/admin ..."). **Top-level:** omit every `UserID` field.

## Step 2 — validations (`pkg/<domain>/<entity>RepositoryValidations.go`)

One `func (r *XxxRequest) Validate() error` per request, in the exact `var reasons []string` / `strings.Join` style of `accountRepositoryValidations.go`:
- Non-empty check for every required ID and string field.
- For `Create`, validate the embedded struct's required fields (`r.<Entity>.OwnerID == ""`, `r.<Entity>.Name == ""`, numeric floors like `Balance < 0`).
- **Owned:** every scoped request also requires `UserID`. **Top-level:** no `UserID` checks.

## Step 3 — validation tests (`pkg/<domain>/<entity>RepositoryValidations_test.go`)

Pure-Go, table-driven, **in-package** (`package <domain>`), no DB — follow `accountRepositoryValidations_test.go` and the `go-unit-tests` skill. Per request: a `valid()` constructor returning a fully-populated request, then rows where the first is `{"valid", func(r *Req){}, false}` and every other row breaks exactly one field with `wantErr: true`. Cover the happy path AND one failing row per required field — this is the mandatory happy-path-AND-error-case rule.

## Step 4 — Postgres impl (`internal/pkg/<domain>/<entity>RepositoryImpl.go`)

Mirror `internal/pkg/accounts/accountRepositoryImpl.go` exactly. Non-negotiable patterns:

- `package <domain>` under `internal/pkg`, importing the port as `pkg<Domain> "github.com/Thatooine/loyalty-points-app/pkg/<domain>"`.
- Struct `<Entity>RepositoryImpl{ db *sql.DB }` + `New<Entity>RepositoryImpl(db *sql.DB)`.
- **Every method starts with** `request.Validate()` (log + wrap `"invalid request for <Method>: %w"`), then `exec := pkgSQL.ExecutorFromContext(ctx, r.db)` so it composes into an ambient unit of work.
- **Create:** assign `uuid.NewString()` if ID empty; stamp time via `time.FormatTime`; on `postgres.IsUniqueConstraintViolation(err)` return `errs.ErrAlreadyExists`.
- **Reads / Update / Delete (owned):** build the query with `args`, and unless `authorization.IsGranted(ctx, authorization.Perm<Resource>ReadAll)` (or `...WriteAll` for mutations) append `AND owner_id = $N` scoped to `request.UserID`. A missing-or-unowned row returns `errs.ErrNotFound` — **never leak existence**. (Top-level entity: skip these `IsGranted` branches and the `owner_id` clause entirely.)
- **GetByID / Update / Delete** map `sql.ErrNoRows` → `errs.ErrNotFound`. `Update` uses a single `UPDATE ... WHERE id=$ [AND owner_id=$] RETURNING <cols>` and scans the returned row. `Delete` checks `RowsAffected()==0` → `errs.ErrNotFound`.
- Add a `scan<Entity>(scan func(dest ...any) error) (*pkg<Domain>.<Entity>, error)` helper that scans columns in a fixed order and parses TEXT timestamps with `time.ParseTime`, mirroring `scanAccount`.
- The permission constants (`authorization.Perm<Resource>ReadAll` / `WriteAll`) for a brand-new resource may not exist yet. If they don't, note it — they're defined in `pkg/authorization/permissions.go` and are part of authorization wiring (adjacent to the out-of-scope policy step). For a top-level entity this doesn't arise.

## Step 5 — mock (`internal/pkg/<domain>/<entity>RepositoryMockImpl.go`)

Hand-written function-field mock, identical shape to `accountRepositoryMockImpl.go`:
- `var _ pkg<Domain>.<Entity>Repository = &Mock<Entity>Repository{}` (drift breaks the build).
- `Mock<Entity>Repository{ T *testing.T; <Method>Func func(t *testing.T, m *Mock<Entity>Repository, ctx context.Context, request ...) (..., error) }` — one field per method.
- Each method: `if m.<Method>Func == nil { return nil, nil }` else delegate. No mutex/counters unless a test needs call-count assertions.

This mock is the testing seam for any future service/adaptor; per the `go-unit-tests` skill the SQL itself (idempotency, scoping, overdraft) is **DB-backed and out of scope** for pure-Go tests — do not write a DB test here.

## Finish

1. `go build ./... && go vet ./... && go test ./<domain-paths>/...` — must all pass.
2. Tell the user exactly what was created and restate the boundary: **the entity now has a repository but is NOT reachable over the wire** — the adaptor, the `DefaultPolicy()` entry, and `cmd/app` wiring still need `add-rpc-method`.
