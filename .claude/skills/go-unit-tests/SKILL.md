---
name: go-unit-tests
description: Write, run, and maintain meaningful pure-Go unit tests for this project. Use when adding tests for a service, adaptor, or validation, or when checking coverage. Covers the two pure-Go test tiers (pure logic and mock-first logic/adaptor), the mock-first convention, the mandatory happy-path-AND-error-case rule, the table-driven validation style, and the run/coverage commands. No database or running app required.
---

# Go unit tests

This skill is for **pure-Go unit tests only** — tests that run with nothing but `go test`. There is **no expectation that the app, a database, or any external service is running.** Every test here passes against mocked collaborators and in-memory logic; if a test would need a live Postgres, it does not belong to this skill (mock the collaborator instead, or leave that invariant to the separate DB-backed/integration suites).

Two conventions are non-negotiable in this repo:

1. **Every method is tested for the happy-day path AND each error branch.** A test that only proves the success case is incomplete — the error mapping, the fail-closed guard, the not-found path are where bugs hide.
2. **Mock as much as possible.** Test a unit against mocked collaborators (the `pkg` interfaces), never a live database. Scope pinning, error mapping, role gates, branching — all of it is provable with hand-written mocks and zero infrastructure.

## The two pure-Go test tiers

| Tier | Where | Backing | Use for |
|------|-------|---------|---------|
| **Pure logic** | `pkg/<domain>/*_test.go` | none | `Validate()`, scope parsing, cursor codecs — table-driven |
| **Logic / adaptor (mock-first)** | `pkg/<domain>/*_test.go` (external `_test` package) | **mocks** | adaptors, services — branch coverage against mocked repos/TxManager |

These two tiers are the entire scope of this skill. Both run offline with plain `go test ./...` — no `docker compose`, no `TEST_POSTGRES_DSN`, no server process. Default to the mock-first tier for anything with branches; use the pure-logic tier for anything that is just data-in/data-out.

## Mock-first unit tests (the primary technique)

Mocks are **hand-written function-field stubs**, one per interface, living next to the implementation in `internal/pkg/<domain>/<x>RepositoryMockImpl.go`. We do not use gomock/mockery — CLAUDE.md mandates plain `go` tooling, no codegen. The canonical example is `internal/pkg/accounts/accountRepositoryMockImpl.go`.

### Mock structure

```go
// var _ asserts the mock satisfies the interface — drift fails the build.
var _ pkgAccounts.AccountRepository = &MockAccountRepository{}

type MockAccountRepository struct {
	T *testing.T
	// one function field per interface method
	GetByIDFunc func(t *testing.T, m *MockAccountRepository, ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error)
	// ...one per method...
}

func (m *MockAccountRepository) GetByID(ctx context.Context, request pkgAccounts.GetAccountByIDRequest) (*pkgAccounts.GetAccountByIDResponse, error) {
	if m.GetByIDFunc == nil {
		return nil, nil // unset = no-op; a test wires only the methods it exercises
	}
	return m.GetByIDFunc(m.T, m, ctx, request)
}
```

Each method delegates to its `XxxFunc`; an unset field is a no-op returning the zero value. Passing `m.T` and `m` into the func lets the func assert and (if needed) reach sibling fields. Add a `sync.Mutex` + `XxxFuncInvocations int` counter back **only** when a test needs call-count assertions or drives the unit concurrently — keep it trimmed otherwise.

### Mocking the unit of work

`TxManager.RunInTx(ctx, fn)` is mocked by simply invoking the function so the wrapped body runs against the other mocks:

```go
txm.RunInTxFunc = func(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx) // execute the unit-of-work body inline; no real transaction
}
```

### Writing the test — mock as stub AND spy

Put the test in an **external `<domain>_test` package** when it imports a mock from `internal/...` (the mock package imports `pkg/<domain>`, so an in-package test would cycle). Inject the claim with the real `authentication.ContextWithLoginClaim` on an `httptest.NewRequest` context — identical to what the middleware does. The worked reference is `pkg/accounts/accountJSONRPCAdaptor_test.go`.

- **Happy path** — func returns a value; assert the unit mapped/returned it correctly, and assert *what the unit passed the collaborator* (e.g. that the adaptor pinned `UserID` to the claim, not the wire). The mock is a spy here.
- **Error path** — func returns `errs.ErrNotFound` etc.; assert the unit maps it to the right opaque wire error (`"account not found"` — never leak existence).
- **Fail-closed path** — put `t.Fatal(...)` *inside* the func to prove the collaborator is **never called** (no claim → unauthorized; non-admin → forbidden, before the repo).

## Pure-logic tests — table-driven `Validate()`

The established style (`pkg/wallet/walletServiceValidations_test.go`): a `valid()` constructor, a `mutate func(*Request)`, and a `wantErr bool`. The happy-path-and-errors rule is satisfied structurally — the first row is valid, every other row breaks exactly one field.

```go
func validReq() ProcessTransactionRequest { return ProcessTransactionRequest{Ref: "tx-1", AccountID: "acc-1", Kind: KindEarn, Points: 150, UserID: "user-1"} }

tests := []struct {
	name    string
	mutate  func(*ProcessTransactionRequest)
	wantErr bool
}{
	{"valid earn", func(r *ProcessTransactionRequest) {}, false},
	{"missing ref", func(r *ProcessTransactionRequest) { r.Ref = "" }, true},
	{"missing userID", func(r *ProcessTransactionRequest) { r.UserID = "" }, true},
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		req := validReq()
		tt.mutate(&req)
		err := req.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
		}
	})
}
```

## Out of scope: invariants that ARE the database

Some behaviour can only be proven against a real engine — mocking it would test the mock, not the system. **These are explicitly out of scope for this skill** and belong to the separate DB-backed/integration suites, which require infrastructure this skill assumes is absent:

- **Idempotency** — the `UNIQUE(ref)` constraint as the dedupe mechanism.
- **Overdraft floor** — the atomic `UPDATE ... WHERE balance + delta >= 0`.
- **Ownership scoping SQL** — that `owner_id = $UserID` actually filters and an unowned row reads as `ErrNotFound`.
- **The ledger invariant** — `accounts.balance == SUM(transactions.points)` after a unit of work commits.

When you reach one of these, **stop and mock the boundary instead**: assert that the unit *calls the repository with the right scoped request*, not that the SQL filters. `WalletServiceImpl` is the boundary case — its pure branching (kind selection, error mapping, fail-closed gates) is fully mock-testable here; only its true DB composition lives elsewhere. If a behaviour genuinely cannot be tested without Postgres, leave a `// DB-backed: see integration suite` note rather than pulling a database into this tier.

## Running tests & coverage

No setup, no services — these run on a clean checkout:

```bash
go test ./...                                   # all pure-Go tests; no DB needed
go test ./pkg/accounts/ -run TestGetByID -v     # one package / one test (regex)
go test -cover ./...                            # per-package coverage %

# Coverage report, then inspect uncovered branches in the browser
go test -coverprofile=cover.out ./...
go tool cover -html=cover.out
```

Chase coverage of **branches that carry behaviour** (each error return, each gate), not a percentage. Generated/trivial getters and the `nil`-func no-op arms of a mock are not worth a line.

## Maintenance

- Mark helpers with `t.Helper()` so failures point at the call site.
- Keep tests deterministic: never call `time.Now()`/`rand` in a unit test — inject the value or assert structurally. With no database or clock in play, a pure-Go test that flakes is a test bug, not infrastructure.
- When you change an interface, the mock's `var _ Interface = ...` assertion breaks the build — update the mock in the same change.
- Adding an RPC method? Pair this with the `add-rpc-method` skill (table-driven `Validate()` test there). For black-box, infrastructure-backed coverage, that is the separate `endpoint-integration-test` skill — out of scope here.