---
name: endpoint-integration-test
description: Write a black-box HTTP/JSON-RPC integration test in the tests/ package that asserts both business rules and data persistence. Use when adding endpoint-level coverage for a wallet or account RPC method. Covers the shared harness, register/login helpers, dual wire-and-DB assertions, and the mandatory negative cases (unauthenticated, foreign-account, overdraft).
---

# Endpoint integration test (tests/ package)

These are black-box tests that POST real JSON-RPC to a running server (default `http://localhost:8080/api`) and, when a DB DSN is reachable, assert the persisted rows too. They **skip cleanly** when the server is unreachable so `go test ./...` stays green without a stack.

Add new tests to the existing files — `tests/wallet_flow_test.go` for wallet methods, `tests/account_flow_test.go` for account methods. Reuse the shared harness; don't re-declare it.

## What's already there (reuse, don't recreate)

From `tests/harness_test.go`:
- `setup(t) *apiClient` — skips if server unreachable; opens DB at the hardcoded DSN when available (`c.db` is nil otherwise).
- `c.call(t, method, params, token)` — POSTs JSON-RPC. **`params` is wrapped in a one-element array** by `call`; pass a single `map[string]any{...}` or struct. `token` is sent as a Bearer header (`""` for unauthenticated).
- `rpcResponse{Result, Error}`, `mustUnmarshal(t, raw, &dst)`, `requireNoError(t, label, resp)`, `uniqueEmail(t)`.

From `tests/auth_flow_test.go`:
- `registerMethod`, `loginMethod`, `testPassword`, `testName`, `testAccountName`.
- `registerResult{Token, UserID, AccountID, Email}`, `loginResult{Token, UserID, Email}`.

From `tests/wallet_flow_test.go` / `tests/account_flow_test.go`:
- `registerMember(t, c)` → fresh member `registerResult`.
- `registerAdmin(t, c)` → promotes a member via direct DB `UPDATE users SET role='admin'` then **re-logs in** (permissions are baked into the token at login). Skips if no DB.
- `uniqueRef(t)` — globally-unique transaction ref. **Always use this**; the server is persistent and `ref` has a `UNIQUE` constraint, so a hardcoded ref collides on the second run.
- `remoteBalance(t, c, token, accountID)`, `dbBalance(t, c, accountID)`, `dbAccountName(t, c, accountID)`.

## The dual-assertion pattern

Every happy-path test asserts twice:
1. **Wire response** — the result the client got back (fields, computed balance, `Duplicate` flag).
2. **Persistence** — read the row(s) directly via `c.db` (guard with the `(value, ok)` helpers; `ok` is false when no DB). For ledger writes also re-read via the public read endpoint (`GetAccountBalance`) to confirm the round-trip.

```go
func TestEarnPointsEndpoint(t *testing.T) {
    c := setup(t)
    member := registerMember(t, c)

    resp := c.call(t, earnPointsMethod, map[string]any{
        "ref": uniqueRef(t), "account_id": member.AccountID, "points": 150,
    }, member.Token)
    requireNoError(t, "EarnPoints", resp)

    var got walletResult
    mustUnmarshal(t, resp.Result, &got)
    if got.Balance != 150 { t.Errorf("balance = %d, want 150", got.Balance) }

    if got := remoteBalance(t, c, member.Token, member.AccountID); got != 150 { /* ... */ }
    if bal, ok := dbBalance(t, c, member.AccountID); ok && bal != 150 { /* ... */ }
}
```

## Mandatory negative cases

A method isn't covered until these are tested (those that apply):
- **Unauthenticated** — `token=""` → `resp.Error != nil`.
- **Foreign / ownership-scoped** — a second `registerMember` acting on the first member's `accountID` must error (reads as not-found; no existence leak). Assert the owner's data is **unchanged**.
- **Privilege gate** — for admin-only methods, a member token → error; assert no state change.
- **Domain invariants** — overdraft floor (debit below zero rejected, balance untouched), idempotency (same `ref` twice → `Duplicate=true`, exactly one DB row, balance unchanged).

## Finish
- Ensure the server running on `:8080` was built from current source (a stale IDE run won't have new methods). Rebuild: `go build -o /tmp/loyalty-app ./cmd/app` and run it; confirm `Registering: <Service>` in the log.
- `go test ./tests/ -v`
