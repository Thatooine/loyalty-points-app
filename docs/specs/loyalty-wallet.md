# Spec: Loyalty Points Wallet

**Status:** conformance pass run 2026-06-17
**Brief (source of truth):** `scratch/Sanlam _ Senior Software Engineer Assignment.pdf`
**System under test:** this repository, full suite green against live server + Postgres

This spec is *derived from* the brief. Each row restates one requirement as a
testable acceptance criterion, cites where it comes from in the brief, names the
test that proves it, and records the verdict observed in the conformance run.

## How the conformance pass was run

```bash
docker compose up -d                       # Postgres
go run ./cmd/app                           # JSON-RPC server on :8080/api
export TEST_POSTGRES_DSN='postgres://loyalty:loyalty@localhost:5432/loyalty_points?sslmode=disable'
export LOYALTY_API_URL='http://localhost:8080/api'
go test ./... -count=1                     # every package
go test ./tests/ -v -count=1               # per-test verdicts (31 integration tests, 0 skips)
```

Result: **all packages `ok`; all 31 integration tests RAN and PASSED, zero skips.**
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
| T1-1 | Create and manage member accounts | Task 1 | `tests/TestOpenAccountEndpoint`, `TestUpdateAccountNameEndpoint`, `TestAccountGetAccountByIDEndpoint`; `internal/pkg/accounts` service tests | ✅ |
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
| C-4 | Data durable across process restarts | Constraints | **No restart test.** Design: Postgres-backed; schema in `pkg/postgres/migrations`, applied on startup. | ⚠️ |

## Task 2 — Access control

| ID | Criterion (from brief) | Source | Test (evidence) | Verdict |
|----|------------------------|--------|-----------------|---------|
| A-1 | At least two roles: member and admin | Task 2 | `pkg/authorization/TestPermissionsForRole`, `TestRolePermissions_AdminIsAllScopedOnly`; roles in `permissions.go` | ✅ |
| A-2 | Members read their own balance and submit their own earn/spend | Task 2 | `tests/TestSpendForeignAccountRejected`, `TestUpdateAccountNameForeignRejected`, `TestListAuditByTransactionRefForeignMemberScopedOut` | ✅ **with deliberate deviation** — see note ▼ |
| A-3 | Admins view any account and apply adjustments | Task 2 | `tests/TestUpdateAccountBalanceAdminEndpoint`, `tests/TestUpdateAccountBalanceMemberForbidden` | ✅ |
| A-4 | Token/credential shape, storage, validation, and role enforcement documented | Task 2 | `SOLUTION.md` + JWT login claim; `pkg/authorization` middleware tests (`TestAuthorizationMiddleware_*`) | 📄 + ✅ |

> **A-2 deviation (intentional).** The brief says members "submit their own
> earn/spend." This implementation lets a member **spend** their own points but
> **not earn** them: `PermWalletTransactOwn` unlocks `SpendPoints` only —
> `EarnPoints`/`ProcessTransaction` credit paths are operator-only, so a member
> cannot mint points into their own account (`pkg/authorization/permissions.go`).
> Enforced by `tests/TestEarnPointsMemberForbidden`, `TestProcessTransactionMemberForbidden`.
> This is a security-motivated divergence worth calling out in SOLUTION.md / the demo.

## Task 3 — Batch ingestion and concurrency safety

| ID | Criterion (from brief) | Source | Test (evidence) | Verdict |
|----|------------------------|--------|-----------------|---------|
| B-1 | Ingest a list of transactions from CSV (endpoint or CLI) | Task 3 | `cmd/cli/pkg/ingest/TestParseCSV`, `TestParseCSV_BadHeaderIsFatal`, `TestBuildRequest` | ✅ |
| B-2 | Apply safely even on close-together rows or a reprocessed file | Task 3 | Idempotency reused from C-1; ordering `tests/TestProcessTransactionBatchServerOrders`; concurrency safety shares C-3's `TestConcurrentSpendsRespectOverdraftFloor` | ✅ |
| B-3 | Produce a summary: processed, accepted, rejected, duplicates | Task 3 | `cmd/cli/pkg/ingest/TestSummarize`, `TestSummarize_WholeBatchError`, `TestSummary_FormatDryRun` | ✅ |
| B-4 | Audit trail per attempt with reason + timestamp | Task 3 | `tests/TestListAuditByTransactionRefEndpoint`; `internal/pkg/audit/TestServiceFetchTransactionAuditTrail_HappyPath` | ✅ |

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
2. **C-4 durability** — optional: a test that writes, restarts the process (or
   reconnects a fresh pool), and re-reads the balance. Low value since Postgres
   durability is a platform guarantee, but it closes the row honestly.
3. **D-4 video demo** — record before submission. Walk this spec table top to
   bottom as the demo script; each ✅ row is a thing to show live.
4. **SOLUTION.md** — confirm it includes the A-2 deviation rationale and the
   "prompts / AI workflow you used" section the brief explicitly asks for (this
   conformance pass *is* part of that workflow).
