# Assignment Task List — Loyalty Points Wallet (Go)

Tracking against **Sanlam / OfferZen — Senior Software Engineer Assignment**.
Each item is checked against the current codebase. Evidence = file(s) that satisfy it.

**Legend:** `[x]` done · `[~]` partial · `[ ]` not started

**Overall status:** The *functional* requirements (Tasks 1–3) are complete and
tested, and the **written deliverables** (`README.md`, `SOLUTION.md`) are now in
place. The only remaining gap is the **video demo**. One optional design decision
remains open (standalone account creation — see Task 1).

---

## Tech stack & global constraints

- [x] **Go backend service** — JSON-RPC 2.0 over `gorilla/rpc/v2`, mounted at `/api` (`cmd/app/setupRPCServer.go`)
- [x] **Relational DB, durable across restarts** — PostgreSQL via `pgx` (assignment allows Postgres); embedded migrations applied on startup (`pkg/postgres/migrate.go`, `pkg/postgres/migrations/*.sql`)
- [x] **Runnable locally** — `docker compose up -d` + `go run ./cmd/app`; defaults baked into `cmd/app/config.go` (`docker-compose.yml`)
- [x] **Prevent double-counting on repeated `ref`** — ledger insert first; `UNIQUE(ref)` is the dedupe; duplicate returns original outcome with `Duplicate=true` (`internal/pkg/wallets/walletServiceImpl.go`)
- [x] **No spend below zero** — single guarded `UPDATE ... WHERE balance + delta >= 0`; 0 rows ⇒ rejected (`internal/pkg/wallets/`)
- [x] **Safe overlapping requests on same account** — unit-of-work via context-resolved executor + atomic guarded update (`pkg/sql/txManager.go`, `pkg/sql/executor.go`)
- [x] **Durable across process restarts** — Postgres persistence + RFC3339Nano UTC TEXT timestamps (`pkg/time/time.go`)

---

## Task 1 — Points wallet service

- [x] **Create & manage member accounts** — account opened atomically at registration; rename via `Account.UpdateAccountName`; admin balance adjust via `Account.UpdateAccountBalance` (`pkg/users/userRegistrationService.go`, `pkg/accounts/accountJSONRPCAdaptor.go`)
  - [ ] _Consider:_ assignment shows a standalone Account `{account_id, name}` shape. Currently an account is only created via user registration (one wallet per user, default "Primary Wallet"). Decide if a standalone create-account RPC is needed or document the registration-creates-account choice in SOLUTION.md.
- [x] **Record earn & spend operations** — `Wallet.EarnPoints`, `Wallet.SpendPoints`, `Wallet.ProcessTransaction` (`pkg/wallets/walletService.go`, `walletServiceJSONRPCAdaptor.go`)
- [x] **Track current balance per account** — `Account.GetAccountBalance`, balance on `Account.GetByID` (`pkg/accounts/accountJSONRPCAdaptor.go`)
- [x] **Prevent double-counting on repeated `ref`** — see global constraint above
- [x] **Reject spends that make balance negative** — overdraft floor; maps to `ErrInsufficientBalance`
- [x] **Example requests/responses in README for the endpoints** — `README.md` now documents every endpoint with `curl` request + JSON response examples (register, login, earn, spend, process, account read/balance/rename, admin balance adjust, batch). Postman collection also present (`api/loyalty-points.postman_collection.json`).

---

## Task 2 — Access control

- [x] **At least two roles: member & admin** — `users.RoleMember`, `users.RoleAdmin` (`pkg/authorization/permissions.go`)
- [x] **Members read own balance + submit own earn/spend** — member permission set is all `:own`-scoped; data layer scopes to `owner_id = UserID` (`pkg/authorization/permissions.go`, repository impls)
- [x] **Admins view any account + apply adjustments** — admin holds `:all` perms; `Account.UpdateAccountBalance` is admin-only adjustment path (`pkg/accounts/accountJSONRPCAdaptor.go`)
- [x] **Token issuance & verification chosen** — signed JWT access tokens; permissions resolved from role at issue time and embedded in claim (`pkg/authentication/`, `internal/pkg/authentication/accessTokenServiceImpl.go`)
- [x] **Two-layer enforcement implemented** — method gate (`authorizationMiddleware` + `DefaultPolicy`) and ownership scope in data layer via `IsGranted` (`pkg/authorization/`)
- [x] **Document token/credential shape, storage/validation, role enforcement** — documented in `SOLUTION.md §4` (RS256 JWS `LoginClaim` shape, issue/store/validate flow, two-layer enforcement, and admin provisioning) with a summary in `README.md`.

---

## Task 3 — Batch ingestion & concurrency safety

- [x] **Ingest a list of transactions from CSV** — `loyalty-cli ingest --file ...`; header `ref,account_id,kind,points,occurred_at` (`cmd/cli/cmd/ingest.go`, `cmd/cli/pkg/ingest/csv.go`)
- [x] **Apply safely with close-together / reprocessed rows** — rows sorted by `occurred_at` then applied in order via admin-only `Wallet.ProcessTransactionBatch`; idempotency + floor + unit-of-work reused from single-tx path (`internal/pkg/wallets/walletServiceImpl.go`)
- [x] **Summary on completion (processed / accepted / rejected / duplicates)** — `ingest.Summarize` / `Summary.Format` (`cmd/cli/pkg/ingest/summary.go`); batch response carries `Accepted/Duplicate/Rejected` tallies (`pkg/wallets/walletService.go`)
- [x] **Audit trail per attempt (accepted/rejected) with reason + timestamp** — `audit_log` table; accepted/duplicate commit in the UoW, rejected written on plain ctx so it survives rollback (`pkg/audit/auditEntry.go`, `internal/pkg/audit/`, `pkg/postgres/migrations/0002_audit.sql`)

---

## Deliverables

### 1. GitHub repository
- [x] **All source code present**
- [x] **`README.md` with local run instructions** — covers prerequisites, `docker compose up`, run server, run CLI, env/DSN overrides, build/vet/test, the JSON-RPC wire format, per-endpoint request/response examples, and CSV ingestion.
- [x] **`SOLUTION.md` — design & trade-offs** — covers architecture (ports & adapters), unit-of-work + idempotency + overdraft-floor reasoning, Postgres-over-SQLite choice, two-layer authz model, token shape/validation/role enforcement, batch-ordering trade-off, and a "what I'd do next" section.
- [~] **`SOLUTION.md` — AI prompts/workflow section** — present (`§8`), grounded in repo artifacts (implementation plan, notes, `.claude/skills/`). **Review and personalise with your actual prompts before submitting.**

### 2. Video demo (5–10 min, Loom or similar) — _manual, not code_
- [ ] Voice-over walkthrough of working project
- [ ] Walk through code architecture
- [ ] Explain key design decisions & trade-offs
- [ ] Highlight interesting technical implementations

---

## Health check (current)
- [x] `go build ./...` — passes
- [x] `go vet ./...` — clean
- [x] `go test ./...` — green (DB-backed tests skip without `TEST_POSTGRES_DSN`; run with Postgres for full coverage)
- [x] Unit + adaptor + validation tests across all domains
- [x] Black-box HTTP integration tests (`tests/`: auth, account, wallet flows)

---

## Remaining work
1. **Personalise `SOLUTION.md §8`** (AI workflow) with your actual prompts — what you asked, accepted/edited, and why.
2. **Decide** the standalone account-creation question (Task 1) — implement a `CreateAccount` RPC or accept the documented registration-based choice (already noted in `SOLUTION.md §7`).
3. **Record the video demo** (5–10 min): working project walkthrough, architecture, key decisions/trade-offs, interesting implementations.
4. _(optional)_ Run the full DB-backed test suite against a live Postgres before submission.

## Done in latest pass
- `README.md` written — run instructions + per-endpoint request/response examples.
- `SOLUTION.md` written — design/trade-offs + token documentation + AI-workflow section.
- Verified the JSON-RPC business-error code is `-32000` (gorilla json2 `E_SERVER`) and documented it accurately.