---
name: scaffold-service
description: Given a domain SERVICE or CAPABILITY interface (anything whose name ends in "Service", reads as a capability like AccountOpener / UserRegistration, or that the user explicitly calls a "service" or "capability"), scaffold its implementation, its JSON-RPC adaptor, its hand-written mock, its Validate() methods, and the pure-Go unit tests for both the impl and the adaptor. Use when adding a domain service that orchestrates one or more repositories. NOT for plain entity persistence — that is scaffold-crud-repository. OUT OF SCOPE — does NOT create the underlying repository SQL, the authorization Policy entry, or cmd/app wiring (use add-rpc-method for those).
---

# Scaffold a domain service / capability

A **service** (a.k.a. capability) is not a repository. A repository persists one entity (SQL, `ExecutorFromContext`, ownership scoping in the query). A service orchestrates the **domain policy the repository deliberately omits** — defaulting fields, pinning the owner to the caller, composing several repository calls inside one unit of work — over the `pkg` repository *interfaces*. It writes **no SQL**.

Trigger this skill when the target interface's name ends in `Service`, reads as a capability (`AccountOpener`, `UserRegistration`, `PointsExpirer`, `RewardRedeemer`…), or the user explicitly says "service" / "capability". For plain entity CRUD persistence use `scaffold-crud-repository` instead.

The two canonical references to mirror in every detail:
- **`AccountOpener`** — the minimal single-repository service. Files: `pkg/accounts/accountOpenerService.go`, `accountOpenerValidations.go`, `accountOpenerJSONRPCAdaptor.go`, `internal/pkg/accounts/accountOpenerServiceImpl.go`, `accountOpenerServiceImpl_test.go`.
- **`UserRegistration`** — the multi-repository service that composes a unit of work via `TxManager.RunInTx`. Files: `pkg/users/userRegistrationService.go` + `internal/pkg/users/userRegistrationServiceImpl*.go`.

## What this skill produces (and what it does NOT)

Produces (for service `<Svc>` in domain `<domain>`):

| File | Layer |
|------|-------|
| `pkg/<domain>/<svc>Service.go` | interface + request/response DTOs (only if the interface doesn't already exist) |
| `pkg/<domain>/<svc>Validations.go` | `Validate()` per request |
| `pkg/<domain>/<svc>Validations_test.go` | pure-Go table-driven validation tests |
| `pkg/<domain>/<svc>JSONRPCAdaptor.go` | the wire layer |
| `pkg/<domain>/<svc>JSONRPCAdaptor_test.go` | mock-first adaptor tests (external `_test` package) |
| `internal/pkg/<domain>/<svc>ServiceImpl.go` | the orchestration impl |
| `internal/pkg/<domain>/<svc>ServiceImpl_test.go` | mock-first impl tests |
| `internal/pkg/<domain>/<svc>Mock.go` | hand-written mock of the service interface (for the adaptor test) |

**Out of scope — do NOT create these:**
- the underlying repository / SQL (use `scaffold-crud-repository`); this skill **consumes** repository interfaces, it does not create them.
- the `authorization.DefaultPolicy()` entry and the `cmd/app/serviceProviders.go` + `setupRPCServer.go` wiring (use `add-rpc-method`) — a brand-new service adaptor MUST be registered there or it is unreachable.

State this boundary back to the user when you finish.

## Step 0 — read the interface and decide three things

If the service interface already exists, read it. Otherwise define it (Step 1). Either way determine:

1. **Names.** Domain package `<domain>`; service type `<Svc>` (e.g. `AccountOpener`); the method(s) it exposes. The impl is `<Svc>ServiceImpl` with `New<Svc>ServiceImpl(...)`; the adaptor is `<Svc>JSONRPCAdaptor` whose `Name()` returns `"<Svc>"`.
2. **Collaborators.** Which `pkg/<domain>.XxxRepository` interfaces (and which methods) does the policy need? The impl takes these as constructor params — always the `pkg` *interfaces*, never the `internal` impls (so it is mock-testable). These repositories must already exist; if one doesn't, stop and point at `scaffold-crud-repository`.
3. **Unit of work?** Does the operation make **more than one** write that must commit atomically? If yes, the impl also takes a `pkgSQL.TxManager` and wraps the writes in `s.txManager.RunInTx(ctx, func(ctx) error { ... })` (see `UserRegistration`). A single write needs **no** `TxManager` — the repository call composes on its own (see `AccountOpener`).

## Step 1 — interface + DTOs (`pkg/<domain>/<svc>Service.go`, only if absent)

Mirror `accountOpenerService.go`: a doc comment that names the policy the service owns and why the repository doesn't; the interface; and `<Method>Request` / `<Method>Response` structs.

**Ownership is the load-bearing rule:** if the operation acts on user-owned data, the request DTO carries `UserID string`, and its doc comment must say it is **always the calling principal, filled by the adaptor from the verified login claim, never from the wire** (copy the wording from `OpenAccountRequest.UserID`). Optional inputs (a name the service defaults) are documented as optional.

## Step 2 — validations + test (`pkg/<domain>/<svc>Validations.go` + `_test.go`)

`func (r *<Method>Request) Validate() error` in the `var reasons []string` / `strings.Join` style. Require only what the service genuinely needs — e.g. `OpenAccount` requires `UserID` but treats `Name` as optional because it defaults it. Add the table-driven, in-package, no-DB test per the `go-unit-tests` skill: first row valid, one failing row per required field.

## Step 3 — impl (`internal/pkg/<domain>/<svc>ServiceImpl.go`)

Mirror `accountOpenerServiceImpl.go` (single repo) or `userRegistrationServiceImpl.go` (TxManager + multiple repos).

- `package <domain>` under `internal/pkg`; import the port as `pkg<Domain> "…/pkg/<domain>"`.
- Struct holds the collaborator interfaces (and `txManager pkgSQL.TxManager` if Step 0 said so); `New<Svc>ServiceImpl(...)` injects them.
- **First line of every method:** `request.Validate()` → log + wrap `"invalid request for <Method>: %w"`. This is the fail-closed gate; nothing touches a collaborator before it passes.
- Apply the domain policy here: substitute defaults for blank optional fields; **pin the owner to `request.UserID`** (already the caller) when constructing the repository request — never trust a wire-supplied owner; stamp `time.Now().UTC()` for creation times.
- Multi-write: do the writes inside `s.txManager.RunInTx(ctx, func(ctx) error { ... })` so they share one transaction (the repos resolve the ambient tx via `ExecutorFromContext`). Single write: just call the repo.
- **Preserve sentinels:** wrap collaborator errors with `%w` (`fmt.Errorf("could not <do>: %w", err)`) so `errs.ErrAlreadyExists` etc. survive for callers/tests.

## Step 4 — adaptor + test (`pkg/<domain>/<svc>JSONRPCAdaptor.go` + `_test.go`)

Mirror `accountOpenerJSONRPCAdaptor.go`. This is the same wire contract as `add-rpc-method` step 3:

- Struct wraps the service **interface**; `New<Svc>JSONRPCAdaptor(svc <Svc>)`; `Name()` returns `"<Svc>"` (gorilla/rpc/v2 registers each method as `<Svc>.<Method>`).
- Wire `<Method>Params` / `<Method>Result` structs with `json:"snake_case"` tags. **The owner is NOT a wire field** — it comes from the claim.
- Method signature `func (a *Adaptor) <Method>(r *http.Request, params *<Method>Params, result *<Method>Result) error`. Pull the claim and **fail closed**:
  ```go
  claim, ok := authentication.LoginClaimFromContext(ctx)
  if !ok { return errors.New("unauthorized") }
  ```
- Build the service request with `UserID: claim.UserID` (pin to caller). Map domain errors to **opaque** wire errors — never leak existence (`errs.ErrNotFound` → "... not found"). Admin-only capability? Add the defence-in-depth `if claim.Role != users.RoleAdmin { return errors.New("forbidden: ...") }` even though the policy gates it.

Adaptor test — external `<domain>_test` package, mock-first (`go-unit-tests`): inject the claim with `authentication.ContextWithLoginClaim` on an `httptest.NewRequest` context (mirror `accountJSONRPCAdaptor_test.go`). Cover: happy path **spying that the adaptor pinned `UserID` to the claim, not the wire params**; no-claim → `unauthorized` with the service mock asserting it is **never called** (`t.Fatal` inside the mock func); a domain error mapped to the opaque wire string; and the role gate if admin-only.

## Step 5 — mocks

Two mock concerns:

1. **The service's own mock** — `internal/pkg/<domain>/<svc>Mock.go`. Hand-written function-field mock satisfying the service interface, identical shape to `accountRepositoryMockImpl.go`: `var _ pkg<Domain>.<Svc> = &Mock<Svc>{}`, a `T *testing.T`, one `<Method>Func` field per method, each method delegating (`nil` field → no-op zero return). This is what the **adaptor test** drives.
2. **Collaborators are already mocked.** The impl test reuses the existing repository mocks (`Mock<Entity>Repository` from the repository scaffolding). For the unit of work, define the tiny inline `mockTxManager` whose default `RunInTx` just calls `fn(ctx)` (copy it verbatim from `userRegistrationServiceImpl_test.go`) — it runs the body inline against the other mocks; override `RunInTxFunc` only to assert it is never reached on a fail-closed path.

## Step 6 — impl test (`internal/pkg/<domain>/<svc>ServiceImpl_test.go`)

Mock-first, `package <domain>` under `internal/pkg`, mirroring `accountOpenerServiceImpl_test.go`. Mandatory cases (happy-path-AND-error-case rule):
- **Happy path** — returns the right value AND, as a **spy** on the collaborator mock, asserts what the service passed it: owner pinned to the request `UserID`, defaults applied, a creation time stamped (assert `!IsZero()`, never a literal clock value).
- **Default/derived field** — a blank optional input is replaced before the collaborator is called.
- **Validation fails closed** — an invalid request returns an error and the collaborator's func has `t.Fatal(...)` proving it is never reached.
- **Collaborator error** — a repo returning `errs.ErrAlreadyExists` (etc.) surfaces from the service and `errors.Is` still matches the sentinel through the wrap; the response is `nil`.

## Finish

1. `go build ./... && go vet ./... && go test ./pkg/<domain>/... ./internal/pkg/<domain>/...` — all must pass.
2. Restate the boundary: **the service is implemented, wired to its repositories, and exposed via an adaptor type — but it is NOT yet reachable over the wire.** Still required via `add-rpc-method`: the `DefaultPolicy()` entry (or `public` set) for `<Svc>.<Method>`, and registering the new adaptor in `cmd/app/serviceProviders.go` + `setupRPCServer.go`. Without both, the method is rejected or unregistered.
