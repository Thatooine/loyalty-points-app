# SOLUTION

Design notes and trade-offs for the loyalty-points wallet. For how to run and call
it, see [README.md](./README.md).

---

## 1. What I built

A loyalty-points ledger exposed as a JSON-RPC 2.0 API over PostgreSQL:

- **Accounts** — opened at registration; readable, renameable, admin-adjustable.
- **Earn / spend** — recorded against an account with a running balance.
- **Idempotency** — the same transaction `ref` never counts twice.
- **Overdraft floor** — a spend can never drive a balance below zero.
- **Access control** — `member` / `admin` roles in two layers; only operators
  credit points, members may only spend from their own account.
- **Sessions** — 1h JWTs with server-side revocation (logout invalidates every
  token a user holds).
- **Batch ingestion** — a CLI loads a CSV safely, reports a summary, and audits
  every attempt.

All three assignment tasks are implemented and covered by unit, adaptor,
validation, and black-box HTTP integration tests.

---

## 2. Key technology choices

Each choice below lists what it buys and what it costs.

| Choice | Why | Cost accepted |
|---|---|---|
| **PostgreSQL** (over SQLite) | Real row-level locking — the guarded `UPDATE` overdraft floor (§3) serialises overlapping spends on the row, not a process-wide lock. Plus `CHECK` constraints, composite indexes for keyset pagination. | A network dependency and a container. Bought back with `docker compose up` + baked-in defaults (one-command setup). |
| **JSON-RPC 2.0** on `gorilla/rpc/v2` | A ledger is a set of *verbs* (`EarnPoints`, `SpendPoints`), not REST resources. Methods auto-register by signature: no dispatcher, no route table. One endpoint → one auth middleware, one log mount, one auditable `method → permissions` policy that fails *closed*. Ordered batches are first-class. | Everything is `POST`, errors return HTTP 200 with the error in the body, no resource caching. Cheap here: calls are commands, not cacheable reads. The gorilla codec can't decode JSON-RPC batch arrays — which nudged batch ingestion toward the single ordered payload I wanted anyway (§5). |
| **Ports & adapters** | `pkg/<domain>` = interfaces, DTOs, `Validate()`, adaptors; `internal/pkg/<domain>` = Postgres impls. Production code depends only on `pkg`; most tests need no DB; swapping an impl is one line in the wiring file. | More files per capability. Tamed with the `.claude/skills/` scaffolds (§8). |
| **`pgx/v5` via `database/sql`** | Keeps the stdlib `*sql.DB`/`*sql.Tx` seam (what the executor and `TxManager` build on) while using pgx's driver and pool. Repositories aren't coupled to pgx types. | Forgoes pgx native features (`COPY`, batch pipelining) — none needed here. |
| **JWS / RS256** (`go-jose`) | Only the private-key holder mints tokens; any service verifies with the public key. Right shape if issuer and verifier later split. Audited JOSE library, no hand-rolled crypto. | Heavier than HMAC and brings key management (dev key baked in for local use). Deliberate, anticipating split trust domains. |
| **`bcrypt`** | Adaptive, salted, salt embedded in the digest; work factor raisable over time. Conservative default. | Slow by design, 72-byte input cap; argon2/scrypt resist GPU attacks better. Lower-risk pick for a take-home. |
| **`zerolog`** | Zero-alloc structured logs plus a context-bound logger: the middleware mints a `request_id` so every `log.Ctx(ctx)` line is correlated. | A specific dependency threaded via `log.Ctx(ctx)`; `slog` would cover much of this now. The choice I hold most loosely. |
| **`cobra` + `viper`** | Real command tree + help for the CLI; layered config (env over baked-in defaults) so the server runs out of the box yet every secret is overridable. | Heavy for today's one subcommand and few keys. Took the headroom for the 12-factor env seam. |
| **Hand-rolled embedded migrations** | Schema travels in the binary; same path runs in prod and the per-test fresh-schema harness; "evolve" = add the next numbered file. Zero operational parts. | No down-migrations, checksums, or dirty-state tracking. Fine forward-only; a long-lived service would want `goose`/`golang-migrate`. |

---

## 3. The invariants, and where they live

Every ledger write flows through **one** method —
`WalletServiceImpl.ProcessTransaction`. `EarnPoints`, `SpendPoints`, and
`ProcessTransactionBatch` are thin wrappers that fix `kind` and delegate, so there
is no second write path to keep in sync.

**Idempotency — the constraint *is* the dedupe.** The insert is attempted first;
`transactions.ref` has a `UNIQUE` constraint. A duplicate fails on the constraint
and returns the original outcome with `duplicate = true`. Deliberately *not*
check-then-insert, which has a race window the unique index doesn't.

**Overdraft floor — read-check-write in one statement:**

```sql
UPDATE accounts SET balance = balance + $delta
WHERE id = $id AND balance + $delta >= 0
```

Zero rows affected = insufficient balance or missing/unowned account. The read,
check, and write are atomic, so two overlapping spends can't both pass against the
same starting balance; Postgres serialises them on the row, no explicit lock
needed. A table-level `CHECK (balance >= 0)` is the backstop. The rejected
alternative — `SELECT`, compare in Go, `UPDATE` — opens the race this design
closes.

**Concurrency safety — a unit of work via context-resolved executor.**
Repositories never hold a `*sql.Tx`; each resolves its executor from the context
(`pkgSQL.ExecutorFromContext`). If `TxManager.RunInTx` put a tx on the context the
repo uses it, else the pool — so the *caller* decides what's atomic and the same
code works standalone or composed. `ProcessTransaction` wraps the ledger insert,
balance update, and audit write in one `RunInTx`. The alternative — threading a
`*sql.Tx` through every signature — forces two flavours of each method or leaks the
tx into standalone calls. The cost: the tx travels implicitly on the context
(slightly more magic) in exchange for a repository layer free of tx bookkeeping.

**Audit trail — survives rollback when it must.** Accepted/duplicate attempts are
audited *inside* the unit of work (they commit with the ledger row). Rejected
attempts are audited on the *plain* context (the pool), so the audit row survives
the rolled-back transaction. Auditing rejections inside the tx would roll them back
with the failed write — silently losing every failed attempt.

---

## 4. Access control

Two roles, two independent layers.

### Token

A signed JWT (JWS, RS256, `go-jose`) whose payload is the `LoginClaim`
(`userID`, `email`, `role`, `permissions`, `expirationTime`, `tokenVersion`).

- **Issued** at register and login with a **1h expiry**. Permissions are resolved
  from the role *at issue time* and embedded, so authorization needs no per-request
  DB lookup. The current `tokenVersion` is stamped in too.
- **Stored** client-side (Bearer header or `access_token` cookie); no server-side
  session table.
- **Validated** by verifying the RS256 signature against the public key, checking
  `expirationTime`, and comparing `tokenVersion` against the user's current value.
  Signing key via `JWT_PRIVATE_KEY_PEM` (dev key baked in). Signature verification
  means embedded permissions can't be tampered with client-side.

### Session revocation (logout)

A self-contained JWT would otherwise stay valid until expiry. Each user row carries
a `token_version` (session epoch), stamped into every token and re-checked per
request. `Session.Logout` increments it, invalidating **every** token that user
holds at once ("log out everywhere"). Login does *not* bump it, so multi-device
sessions coexist until explicit logout. Cost: one indexed read per protected
request. Per-device revocation would need per-token tracking — left out of scope.

### Layer 1 — method gate (all-or-nothing)

`authorizationMiddleware` reads the JSON-RPC method from the body and consults a
`Policy` (`pkg/authorization/policy.go`) mapping method → accepted permissions,
plus a public-method set. **A method absent from the policy fails closed** — it
returns "method not allowed" until listed. Preferred over per-handler `if
!authorized` checks, where the failure mode of forgetting is an *open* endpoint;
here it's a *closed* one.

### Layer 2 — ownership scope (own vs all)

Breadth is enforced in the data layer: a repository calls
`authorization.IsGranted(ctx, Perm…All)`; without `:all` the SQL is scoped to
`owner_id = $UserID`, so a non-owner gets `ErrNotFound` — no existence leak.
Permissions are `resource:action:scope` strings; roles map to a fixed, no-wildcard
set in `permissions.go`. Crediting is operator-only: `EarnPoints` and
`ProcessTransaction` need `wallet:transact:all` (admins only); a member's
`wallet:transact:own` unlocks `SpendPoints` against their own account — they cannot
mint points. The acting `UserID` always comes from the verified claim, never the
client payload.

### Becoming an admin

Registration always creates a **member**; there's no self-service path to admin. An
admin is provisioned out-of-band:

```sql
UPDATE users SET role = 'admin' WHERE email = 'ops@example.com';
```

The next login issues an admin token. For local dev, `go run ./cmd/bootstrap`
resets the DB and recreates a known admin user (`system@mail.com` /
`admin-user-123`) — a dev convenience, not a production provisioning path.

---

## 5. Batch ingestion and ordering

The CLI (`loyalty-cli ingest`) is a thin client: it parses the CSV, **sorts rows by
`occurred_at` then line** (stable sort, 1-based line as tiebreaker, blank
`occurred_at` first), and sends the whole batch as **one ordered JSON-RPC request**
to the admin-only `Wallet.ProcessTransactionBatch`.

One ordered request rather than a JSON-RPC batch array because the spec lets a
server process batches in any order (and the gorilla codec can't decode them
anyway), while the overdraft floor makes writes order-dependent — an earn must land
before the spend it funds.

**Ordering is enforced on the server, not just the client.** Client-only sorting
would leave the invariant at the mercy of the caller. So `ProcessTransactionBatch`
re-sorts by `occurred_at` itself. Two details: it sorts a *copy* (the caller's
slice isn't mutated, so per-element results map to sent order), and the sort is
*stable* (equal/absent timestamps keep submission order, so an earn can't land
after its spend by sort nondeterminism). The CLI's own sort now just drives the
`--dry-run` preview. Because the server may reorder, results come back in *applied*
order and callers correlate by `ref`, not position.

Reprocessing a file is safe: idempotency dedupes on `ref`, so re-ingestion yields
duplicates, not double counts. The server returns per-element outcomes (`accepted`
/ `duplicate` / `rejected` + reason) and summary tallies; the CLI prints `processed
/ accepted / duplicates / rejected` and lists rejections with line and reason.

---

## 6. Data model

Timestamps are **RFC3339Nano UTC TEXT** — human-readable and lexicographically
sortable, so ordering and keyset pagination use plain string comparison. `OwnerID`
is a FK to `users.id`, denormalized onto `transactions` and `audit_entries` so
entries attribute without a join (DTO field `UserID`, SQL column `owner_id`). Schema
lives in `pkg/postgres/migrations/*.sql`, embedded and applied in lexical order on
startup and in tests.

---

## 7. Trade-offs and what I'd do next

Deliberate cuts to keep the take-home on the correctness-critical core — each with a
next step:

- **Admin provisioning is manual** (SQL promotion — §4). Chose this over an
  admin-creates-admin RPC to avoid a privilege-escalation surface I couldn't fully
  harden in scope. Next: a bootstrap/invite flow.
- **Permissions are snapshotted into the token.** A role change takes effect on next
  login (≤1h). Revocable via the `token_version` epoch, but no refresh-token flow
  and no per-device revocation — that needs a server-side token table, trading away
  the stateless-JWT property.
- **No rate / request-size limits** on ingestion. The batch is one unit of work —
  simple and correct, but not ideal for huge files. Next: chunking.
- **Forward-only migrations** (§2). No down-migrations or dirty-state tracking; a
  long-lived service would adopt a real migration tool.

---

## 8. AI workflow

> How AI tooling was used, per the assignment.

I used **Claude Code** as a pair-programmer with a plan-first, test-backed workflow.
Three artifacts carry the workflow: **`CLAUDE.md`** (the always-loaded context),
**`.claude/skills/`** (per-layer mechanics), and **`SPEC.md`** (the acceptance
contract).

### `CLAUDE.md` — the always-loaded context

`CLAUDE.md` is the technical brief every agent reads first: architecture, key files,
the data model, the critical design decisions, and — most importantly — a **Known
Gotchas** list whose conventions *override* default behavior. Its job is to let an
agent (or a future me) continue work safely without prior context.

It **evolved by accretion**. It began as a plain description of the layout and the
ports/adapters split, then grew an entry every time the project hit something that
fails *silently* — each gotcha is a lesson paid for once and then written down so it
isn't repeated:

- "a skipped test proves nothing" (a green run can be all skips — §3 conformance);
- "new protected method → add it to `DefaultPolicy()`" (the most-forgotten step,
  Layer 1 — §4);
- "never check-then-insert for idempotency" and "rejected audit rows go on the plain
  context" (the two invariants that look right but quietly break — §3);
- "members cannot earn" (the deliberate deviation, A-2 — §4).

A **Recent History** table also tracks notable commits so an agent grasps direction
without spelunking the log.

Its role in the workflow is fail-*safe* guidance: where the skills say *how* to build
a layer and the spec says *what* must be true, `CLAUDE.md` says *what to watch out
for* — the load-bearing invariants and the easy-to-forget steps — so generated code
conforms to the existing patterns instead of drifting. It is the single highest-leverage
file for keeping an agent productive and correct across sessions.

### Conventions encoded as reusable skills

Rather than re-explain the architecture each prompt, I captured the recurring,
error-prone tasks as Claude Code skills in `.claude/skills/` — each a checklist plus
a canonical reference, so generated code mirrors existing patterns and the
silently-failing steps are never skipped.

| Skill | What it does | Leaves out |
|---|---|---|
| `code-structure` | The arbiter: decides adaptor vs service vs repository, and routes any new write path through the single owner of that mechanic instead of duplicating it. | — |
| `scaffold-crud-repository` | From one entity struct → the whole persistence layer: interface + DTOs, `Validate()`, Postgres impl (executor + ownership scoping), mock, validation tests. | adaptor, policy, wiring |
| `scaffold-service` | From a capability interface → the orchestration impl, adaptor, mock, `Validate()`, unit tests. Writes **no SQL**. | repository SQL, policy, wiring |
| `add-rpc-method` | Makes a method callable end-to-end, including the easy-to-forget `DefaultPolicy()` entry. | — |
| `go-unit-tests` | Pure-Go tests: mock-first, table-driven, happy-path-**and**-each-error-branch. | — |
| `endpoint-integration-test` | Black-box HTTP/JSON-RPC tests asserting wire response **and** persisted rows, plus mandatory negative cases. Skips cleanly with no server/DB. | — |

**How they compose.** `code-structure` decides the shape; the `scaffold-*` skills
generate a layer each (stopping short of the wire); `add-rpc-method` exposes it and
closes the policy gap; the test skills cover it at both levels. Each skill's
"out of scope" list points at the next, so the model hands work off rather than
half-doing a neighbouring layer. This was the biggest lever on consistency.

### Spec as the conformance contract

The brief is restated as a testable spec in `SPEC.md`: each row turns a requirement
into an acceptance criterion, cites its origin, names the proving test, and records
a verdict (✅ test-proven / ⚠️ by-design / 📄 doc deliverable) — including honest gaps
(C-4 durability is ⚠️) and the deliberate deviation (A-2: members spend but can't
earn). `CLAUDE.md` makes it **authoritative**: before changing wallet behavior an
agent reads the relevant row and conforms, updating the spec and its test mapping in
the same change. So `CLAUDE.md` carries architecture and gotchas, `.claude/skills/`
encode per-layer mechanics, and `SPEC.md` is the acceptance contract — together
keeping changes structurally consistent and provably conformant.

### Test-driven, conformance-checked

The workflow treats a behavior as unfinished until a test exercises it *and is
observed to run* — not merely written. Two conventions, both encoded in the test
skills, make that real:

- **Happy-path AND every error branch.** `go-unit-tests` requires the success case
  plus each failure mode (table-driven validations, mock-first service logic). The
  `endpoint-integration-test` skill adds a **dual assertion** — wire response *and*
  persisted rows — and a set of **mandatory negative cases** (unauthenticated,
  foreign/ownership-scoped, overdraft, privilege gate). A method isn't "covered"
  until those exist.
- **A skipped test proves nothing.** DB and endpoint tests skip cleanly when no
  server/DB is reachable, so a green `go test ./...` can be *all skips*. The
  conformance pass therefore brings the stack fully up and confirms tests actually
  **RAN** with zero skips (recorded in `SPEC.md`'s header). This gotcha is in
  `CLAUDE.md` precisely because the failure is silent.

This loop also **catches defects the model would otherwise hide.** The recent
resource-bound hardening is the clearest example: I added a 4 MiB body cap and a
1000-row batch cap, then drove each with a live integration test
(`TestRequestBodyTooLargeRejected`, `TestProcessTransactionBatchExceedsMaxRejected`,
now mapped as spec rows C-5/B-5). Running them against a real server surfaced that
the batch adaptor was collapsing *every* service error — including the new
validation error — into a generic `-32603 internal`, masking the very guard under
test. The fix (surface `ErrInvalidArgument` directly, like the single-transaction
path) only became visible *because* the test asserted the precise wire code rather
than "some error". Asserting the exact error contract, not just failure, is what
turned the test into a bug detector.

### How I steered the rest

I accepted the model's scaffolding, SQL boilerplate, and test stubs readily. I owned
the correctness-critical calls: the concurrency model (single guarded `UPDATE` over
check-then-insert), the batch transport (one ordered request), and audit-on-rollback
(rejections must survive). AI accelerated the breadth; the load-bearing decisions
stayed human-reviewed.
