# Spec: Loyalty Points Wallet

**Status:** conformance pass run 2026-06-18
**Brief (source of truth):** `Sanlam _ Senior Software Engineer Assignment.pdf`
**System under test:** this repository, full suite green against live server + Postgres + Redis

This spec is *derived from* the brief. Each row restates one requirement as a
testable acceptance criterion, cites where it comes from in the brief, names the
test that proves it, and records the verdict observed in the conformance run.

## How the conformance pass was run

```bash
docker compose up -d                       # Postgres + Redis
go run ./cmd/app                           # JSON-RPC server on :8080/api
export TEST_POSTGRES_DSN='postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable'
export LOYALTY_API_URL='http://localhost:8080/api'
export LOYALTY_DB_DSN='postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable'
go test ./... -count=1                     # every package
go test ./tests/ -v -count=1               # per-test verdicts (40 integration tests, 0 skips)
```

Result: **all packages `ok`; all 40 integration tests RAN and PASSED, zero skips.**
The zero-skip part matters — a skipped DB/endpoint test proves nothing, so the
environment was brought fully up first.

## Verdict legend

- ✅ **Satisfied** — a test names this behavior and passed in the run above.
- ⚠️ **By design, not test-proven** — the implementation structurally satisfies it,
  but no automated test exercises it. Honest gap, not a failure.
- ❌ **Violated** — a test failed. (none)
- 📄 **Manual deliverable** — satisfied by a document/artifact, not a test.

---

## Task 1 — Points wallet service

| ID | Criterion (from brief) | Source | Test (evidence) | Verdict |
|----|------------------------|--------|-----------------|---------|
| T1-1 | Create and manage member accounts | Task 1 | `tests/TestOpenAccountEndpoint`, `TestUpdateAccountNameEndpoint`, `TestAccountGetAccountByIDEndpoint`, `TestFetchMyAccountsEndpoint` (lists every account the caller owns), `TestFetchMyAccountsScopedToOwner`; `internal/pkg/accounts` service tests | ✅ |
| T1-2 | Record earn and spend operations against an account | Task 1 | `tests/TestEarnPointsEndpoint`, `tests/TestSpendPointsEndpoint` | ✅ |
| T1-3 | Track current balance per account | Task 1 | `tests/TestAccountGetAccountBalanceEndpoint` | ✅ |
| T1-4 | Prevent double-counting when the same `ref` is submitted again | Task 1 + Constraints | `tests/TestProcessTransactionDuplicateRef` (asserts balance unchanged, `Duplicate=true`, single ledger row) | ✅ |
| T1-5 | Reject spends that would make the balance negative | Task 1 + Constraints | `tests/TestSpendOverdraftRejected`; admin path `tests/TestUpdateAccountBalanceOverdraftRejected` | ✅ |
| T1-6 | Example requests/responses documented for the API | Task 1 | `README.md` | 📄 |

## Constraints (cross-cutting)

| ID | Criterion (from brief) | Source | Test (evidence) | Verdict |
|----|------------------------|--------|-----------------|---------|
| C-1 | Same reference counted at most once | Constraints | `tests/TestProcessTransactionDuplicateRef` | ✅ |
| C-2 | No spend may drive balance below zero | Constraints | `tests/TestSpendOverdraftRejected` | ✅ |
| C-3 | Overlapping requests on one account stay correct | Constraints | `tests/TestConcurrentEarnsNoLostUpdates` (20 concurrent earns → no lost updates), `tests/TestConcurrentSpendsRespectOverdraftFloor` (over-demand spends never breach the floor); both pass under `-race`. Design: single guarded `UPDATE ... WHERE balance + delta >= 0`. | ✅ |
| C-4 | Data durable across process restarts | Constraints | `tests/TestBalanceDurableAcrossReconnect` (writes a balance, then re-reads it through a fresh connection pool opened after the write — the DB-level analogue of a restart). Postgres-backed; schema in `pkg/postgres/migrations`, applied on startup. | ✅ |
| C-5 | Request body size is bounded (DoS guard) | Hardening (not in brief) | `tests/TestRequestBodyTooLargeRejected` — body over the 4 MiB cap rejected as `-32602` during the read, before auth/dispatch. Design: `http.MaxBytesReader` in `pkg/authorization/authorizationMiddleware.go`. | ✅ |
| C-6 | Request rate is bounded per IP / per user (brute-force + DoS guard) | Hardening (not in brief) | `pkg/rateLimiting` middleware tests (`TestIPRateLimiter_*`, `TestUserRateLimiter_*`) — over-limit → HTTP 429 / code `-32006`; under-limit passes; non-targeted methods and claim-less requests bypass; body preserved for downstream. Design: Redis-backed token bucket (`github.com/mennanov/limiters`), IP limiter on the public auth methods + per-user limiter on authenticated traffic, wired in `cmd/app/setupRPCServer.go`. Live 429 verified manually; not in the `tests/` suite because the limit is server-global state and would make the shared-IP suite flaky. | ✅ unit, ⚠️ no integration test |

## Task 2 — Access control

| ID | Criterion (from brief) | Source | Test (evidence) | Verdict |
|----|------------------------|--------|-----------------|---------|
| A-1 | At least two roles: member and admin | Task 2 | `pkg/authorization/TestPermissionsForRole`, `TestRolePermissions_AdminIsAllScopedOnly`; roles in `permissions.go` | ✅ |
| A-2 | Members read their own balance and submit their own earn/spend | Task 2 | `tests/TestEarnPointsMemberOwnAccount` (member earns own), `tests/TestSpendPointsEndpoint` (member spends own), `tests/TestEarnForeignAccountRejected`, `tests/TestSpendForeignAccountRejected`, `tests/TestUpdateAccountNameForeignRejected` | ✅ — see note ▼ |
| A-3 | Admins view any account and apply adjustments | Task 2 | `tests/TestUpdateAccountBalanceAdminEndpoint`, `tests/TestUpdateAccountBalanceMemberForbidden` | ✅ |
| A-4 | Token/credential shape, storage, validation, and role enforcement documented | Task 2 | `SOLUTION.md` + JWT login claim; `pkg/authorization` middleware tests (`TestAuthorizationMiddleware_*`) | 📄 + ✅ |

> **A-2 note.** Members earn **and** spend on their **own** account:
> `PermWalletTransactOwn` unlocks both `EarnPoints` and `SpendPoints`, and the data
> layer scopes the account to the caller, so the action can only land on an account
> they own (`pkg/authorization/policy.go`, `permissions.go`). The generic
> `ProcessTransaction` (arbitrary `kind`) and `ProcessTransactionBatch` remain
> operator-only — bulk and arbitrary crediting stay an admin action, enforced by
> `tests/TestProcessTransactionMemberForbidden`.

## Task 3 — Batch ingestion and concurrency safety

| ID | Criterion (from brief) | Source | Test (evidence) | Verdict |
|----|------------------------|--------|-----------------|---------|
| B-1 | Ingest a list of transactions from CSV (endpoint or CLI) | Task 3 | `cmd/cli/pkg/ingest/TestParseCSV`, `TestParseCSV_BadHeaderIsFatal`, `TestBuildRequest` | ✅ |
| B-2 | Apply safely even on close-together rows or a reprocessed file | Task 3 | Idempotency reused from C-1; ordering `tests/TestProcessTransactionBatchServerOrders`; concurrency safety shares C-3's `TestConcurrentSpendsRespectOverdraftFloor` | ✅ |
| B-3 | Produce a summary: processed, accepted, rejected, duplicates | Task 3 | `cmd/cli/pkg/ingest/TestSummarize`, `TestSummarize_WholeBatchError`, `TestSummary_FormatDryRun` | ✅ |
| B-4 | Audit trail per attempt with reason + timestamp | Task 3 | `tests/TestListAuditByTransactionRefEndpoint`; `internal/pkg/audit/TestServiceFetchTransactionAuditTrail_HappyPath` | ✅ |
| B-5 | Batch size is bounded so one request can't enqueue unbounded work | Hardening (not in brief) | `tests/TestProcessTransactionBatchExceedsMaxRejected` — a batch past `maxBatchSize` (1000) rejected wholesale as `-32602`, balance untouched. Design: `ProcessTransactionBatchRequest.Validate()` in `pkg/wallets`. | ✅ |

## Deliverables

| ID | Criterion (from brief) | Source | Evidence | Verdict |
|----|------------------------|--------|----------|---------|
| D-1 | Single GitHub repo with all source | Deliverables | this repo | 📄 |
| D-2 | `README.md` with local run instructions | Deliverables | `README.md` present | 📄 |
| D-3 | `SOLUTION.md` describing design + trade-offs + AI workflow used | Deliverables | `SOLUTION.md` present | 📄 |
| D-4 | 5–10 min video demo (voiceover, architecture, decisions) | Deliverables | manual | 📄 (TODO before submission) |

---

## Worklist (everything not ✅)

These are the only open items — the rest of the system conforms with test evidence.

1. ~~**C-3 / B-2 concurrency**~~ — ✅ **DONE.** `tests/wallet_concurrency_test.go`
   adds `TestConcurrentEarnsNoLostUpdates` and `TestConcurrentSpendsRespectOverdraftFloor`
   (goroutines + `sync.WaitGroup`, pass under `-race`). C-3 and B-2 are now test-proven.
2. ~~**C-4 durability**~~ — ✅ **DONE.** `tests/TestBalanceDurableAcrossReconnect`
   writes a balance and re-reads it through a fresh pool opened after the write
   (reconnect form), closing the row honestly.
3. **D-4 video demo** — record before submission. Walk this spec table top to
   bottom as the demo script; each ✅ row is a thing to show live.
4. **SOLUTION.md** — confirm it includes the A-2 deviation rationale and the
   "prompts / AI workflow you used" section the brief explicitly asks for (this
   conformance pass *is* part of that workflow).
