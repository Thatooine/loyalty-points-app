---
name: add-rpc-method
description: Scaffold a new JSON-RPC method end-to-end across the ports-and-adapters layers. Use when adding a repository/service method that must be callable over the wire, exposing an existing internal method, or wiring a new RPC service. Covers the interface, DTO, Validate(), adaptor, SQL impl, and — critically — the authorization Policy entry that is easy to forget.
---

# Add a JSON-RPC method end-to-end

One logical method touches several files. Skipping any one fails silently or at runtime — most often a forgotten `DefaultPolicy()` entry, which makes the method return "method not allowed" no matter the token.

## The checklist

Work through these in order. The first four live in `pkg/<domain>` (the port side); the SQL lives in `internal/pkg/<domain>`; policy and wiring are global.

### 1. Interface + DTOs — `pkg/<domain>/<x>Repository.go` (or `<x>Service.go`)
- Add the method to the interface.
- Define `<Method>Request` and `<Method>Response` structs.
- **Ownership:** if the method reads or writes user-owned data, the request DTO MUST carry a `UserID string` field. This is the field the adaptor fills from the login claim and the impl scopes on. (Wire/DTO field is `UserID`; the SQL column is `owner_id` — see `[[ownerid-is-a-user]]`.)

### 2. Validation — `pkg/<domain>/<x>Validations.go`
- Add `func (r <Method>Request) Validate() error`.
- Validate every required field, including `UserID` when present. Mirror the existing validations (e.g. `accountRepositoryValidations.go`): non-empty IDs, non-zero deltas, etc.
- Add a table-driven test in the matching `*Validations_test.go` (pure logic, no DB).

### 3. Adaptor — `pkg/<domain>/<x>JSONRPCAdaptor.go`
- Signature MUST be `func (a *Adaptor) Method(r *http.Request, params *Params, result *Result) error` — gorilla/rpc/v2 auto-registers it as `<ServiceName>.<Method>` where the service name comes from `Name()`.
- Define wire `Params`/`Result` structs with `json:"snake_case"` tags.
- Pull the caller from the claim and fail closed:
  ```go
  claim, ok := authentication.LoginClaimFromContext(ctx)
  if !ok { return errors.New("unauthorized") }
  ```
- Pass `UserID: claim.UserID` into the request so scoping pins it to the caller. Do NOT let `UserID` come from the wire params — that would let a caller act as anyone.
- Map domain errors to opaque wire errors: `errs.ErrNotFound` → "... not found", `errs.ErrInsufficientBalance` → "insufficient balance". Never leak existence (a non-owner must get "not found", indistinguishable from missing).
- **Admin-only / privileged raw method?** Add a defence-in-depth role check even though the policy gates it (mirrors `Account.UpdateAccountBalance`):
  ```go
  if claim.Role != users.RoleAdmin { return errors.New("forbidden: ... is admin-only") }
  ```

### 4. SQL impl — `internal/pkg/<domain>/<x>Impl.go`
- Resolve the executor so the method composes into a unit of work when one is active:
  ```go
  ex := pkgSQL.ExecutorFromContext(ctx, r.db)
  ```
- Enforce ownership scope on demand:
  ```go
  if !authorization.IsGranted(ctx, authorization.Perm<Resource><Action>All) {
      // add: AND owner_id = $UserID  to the query
  }
  ```
  Without the `:all` permission, scope to `owner_id` and return `errs.ErrNotFound` for missing-or-unowned (no existence leak).
- For balance writes use a single guarded statement (`UPDATE ... WHERE ... AND balance + $delta >= 0`); never read-check-write. Zero rows affected → re-read to distinguish `ErrNotFound` from `ErrInsufficientBalance`.
- **Ledger writes go through `WalletServiceImpl.ProcessTransaction`, not a new write path.** Only add a raw mutator if the method is a deliberate ledger-bypass (operator correction) — and then make it admin-only per step 3.

### 5. Authorization policy — `pkg/authorization/policy.go` (DON'T SKIP)
- Add a `const <method>Method = "<ServiceName>.<Method>"`.
- Add an entry to `DefaultPolicy()`'s `byMethod` map:
  - Owner-or-admin read: `{PermAccountReadOwn, PermAccountReadAll}`
  - Owner-or-admin write: `{Perm...WriteOwn, Perm...WriteAll}`
  - Admin/operator only: `{Perm...All}` (the `:all` permission alone)
- A method NOT in `byMethod` and NOT in the `public` set is rejected. If it should need no token (login/register), add it to `public` instead.

### 6. Wiring — `cmd/app/serviceProviders.go` + `cmd/app/setupRPCServer.go`
- Only needed for a brand-new **service** (new adaptor type). Construct the impl, inject it into the adaptor, and register the adaptor in `setupRPCServer.go`. Adding a method to an existing adaptor needs no wiring change.

## Finish
- `go build ./... && go vet ./...`
- Add an endpoint integration test — see the `endpoint-integration-test` skill.
- Restart the server from a fresh build before testing; a stale IDE-managed `:8080` process won't have the new method registered (check the log for `Registering: <ServiceName>`).