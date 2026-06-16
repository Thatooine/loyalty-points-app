# SOLUTION

Design notes and trade-offs for the loyalty-points wallet. For how to run it and
call it, see [README.md](./README.md).

---

## 1. What I built

A loyalty-points ledger exposed as a JSON-RPC 2.0 API, backed by PostgreSQL:

- **Accounts** — opened for a user at registration; readable, renameable, and
  (for admins) adjustable.
- **Earn / spend** — recorded against an account, with current balance tracked.
- **Idempotency** — the same transaction `ref` never counts twice.
- **Overdraft floor** — a spend can never drive a balance below zero.
- **Access control** — `member` and `admin` roles, enforced in two layers;
  crediting points is operator-only, members may only spend from their own
  account.
- **Sessions** — short-lived (1h) JWTs with server-side revocation: logout
  invalidates every token the user holds.
- **Batch ingestion** — a CLI ingests a CSV of transactions safely and reports a
  summary; every attempt is recorded in an audit trail.

All three assignment tasks are implemented and covered by unit, adaptor,
validation, and black-box HTTP integration tests.

---

## 2. Key technology choices

**PostgreSQL, not SQLite.** The assignment recommends SQLite and allows
Postgres. I chose Postgres because the correctness story for concurrent writes
is the interesting part of this problem, and Postgres gives real row-level
locking and a guarded-`UPDATE` concurrency model without SQLite's single-writer
caveats. Durability across restarts is satisfied either way; the migration
machinery and the `pgx` stdlib driver keep local setup to one `docker compose
up`.

**JSON-RPC 2.0 over `gorilla/rpc/v2`.** A single `/api` endpoint hosts every
service. Adaptor methods with the signature
`func(r *http.Request, params *T, result *T2) error` auto-register as
`<ServiceName>.<Method>`. This keeps the wire layer declarative — no hand-rolled
dispatcher — and means authorization can be a single middleware in front of one
endpoint.

**Ports and adapters.** `pkg/<domain>` holds the interfaces, DTOs, `Validate()`
methods, and JSON-RPC adaptors; `internal/pkg/<domain>` holds the PostgreSQL
implementations. Production code depends only on the `pkg` interfaces, and
`cmd/app/serviceProviders.go` is the single wiring point. This made the system
testable with mocks at the port boundary and kept the SQL isolated.

---

## 3. The invariants, and where they live

Every ledger write flows through **one** method —
`WalletServiceImpl.ProcessTransaction` — so the three invariants are enforced
and tested in exactly one place. `EarnPoints`, `SpendPoints`, and
`ProcessTransactionBatch` are thin wrappers that build a request (fixing `kind`)
and delegate, so there is no second write path to keep in sync.

**Idempotency — the constraint *is* the dedupe.** The ledger insert is attempted
first; `transactions.ref` carries a `UNIQUE` constraint. A duplicate insert
fails on the constraint and the original outcome is returned with
`duplicate = true`. This is deliberately *not* check-then-insert — that has a
race window; the unique index does not.

**Overdraft floor — read-check-write in one statement.** Balance is mutated with
a single guarded statement:

```sql
UPDATE accounts SET balance = balance + $delta
WHERE id = $id AND balance + $delta >= 0
```

Zero rows affected means either insufficient balance or a missing/unowned
account — the read, the check, and the write are atomic, so two overlapping
spends cannot both pass the check against the same starting balance. There is
also a table-level `CHECK (balance >= 0)` as a backstop.

**Concurrency safety — a unit of work via context-resolved executor.**
Repositories never hold a `*sql.Tx`. Each resolves its executor from the context
(`pkgSQL.ExecutorFromContext`): if `TxManager.RunInTx` placed a transaction on
the context, the repository uses it; otherwise it uses the pool. So the *caller*
decides what is atomic, and the same repository code works standalone or
composed. `ProcessTransaction` wraps the ledger insert, the balance update, and
the audit write in one `RunInTx` so they commit or roll back together.

**Audit trail — survives rollback when it must.** Accepted and duplicate
attempts are audited *inside* the unit of work (they commit with the ledger
row). Rejected attempts are audited on the *plain* context (the pool), so the
audit row survives the rolled-back transaction — a rejection is still recorded
even though nothing else was written.

---

## 4. Access control

Two roles, enforced in two independent layers.

### Token shape, storage, and validation

Access is carried by a **signed JWT** (JWS, RS256, compact serialization via
`go-jose`). The token's payload is the `LoginClaim`:

```json
{
  "userID": "0c5f...",
  "email": "rina@example.com",
  "role": "member",
  "permissions": ["account:read:own", "wallet:transact:own", "..."],
  "expirationTime": 1718534400,
  "tokenVersion": 0,
  "lastName": ""
}
```

- **Issued** at register and at login, with a **1h expiry**. The permission list
  is resolved from the user's role *at issue time* and embedded in the claim, so
  authorization needs no per-request DB lookup for permissions. The user's
  current `tokenVersion` (session epoch) is stamped in too — see *Session
  revocation* below.
- **Stored** client-side; there is no server-side session table. It is presented
  either as `Authorization: Bearer <token>` or an `access_token` cookie.
- **Validated** by parsing the JWS, verifying the RS256 signature against the
  RSA public key, unmarshalling the claim, checking `expirationTime`, and
  comparing the claim's `tokenVersion` against the user's current value
  (rejecting a token whose epoch is stale). The signing key is configured via
  `JWT_PRIVATE_KEY_PEM` (a dev key is baked in for local use). Signature
  verification means the embedded permissions cannot be tampered with
  client-side.

### Session revocation (logout)

A signed JWT is otherwise self-contained, so a leaked or stale token would stay
valid until expiry with no way to cut it off. To close that gap each user row
carries a `token_version` (a session epoch): it is stamped into every issued
token and re-checked on every protected request. `Session.Logout` increments
it, which invalidates **every** token that user currently holds in one step —
"log out everywhere". Login deliberately does *not* bump the version, so
concurrent sessions on multiple devices coexist until an explicit logout. The
cost is one indexed read per protected request (a short-TTL cache would remove
it if needed); per-device revocation would require per-token tracking, which I
left out of scope.

### Layer 1 — method gate (all-or-nothing)

`authorizationMiddleware` reads the JSON-RPC method out of the request body and
consults a `Policy` (`pkg/authorization/policy.go`) mapping method → accepted
permissions, plus a set of public methods. Public methods (register, login) pass
through; everything else requires a valid token whose permissions satisfy the
method. **New protected methods must be added to `DefaultPolicy()` or they are
rejected** — fail-closed by default.

### Layer 2 — ownership scope (own vs all)

The method gate resolves no scope. Breadth is enforced in the data layer, on
demand: a repository calls `authorization.IsGranted(ctx, Perm...All)`, and
without the `:all` permission the SQL is scoped to `owner_id = $UserID`. A
non-owner therefore gets `ErrNotFound` — indistinguishable from genuinely
missing, so ownership is enforced without leaking existence.

Permissions are `resource:action:scope` strings (e.g. `account:read:own`,
`wallet:transact:all`); roles map to a fixed, explicit (no-wildcard) permission
set in `permissions.go`. Crediting points is an operator action, so
`Wallet.EarnPoints` and the generic `Wallet.ProcessTransaction` require the
all-scoped `wallet:transact:all` that only admins hold; a member's
`wallet:transact:own` unlocks `Wallet.SpendPoints` against their own account
only — a member cannot mint points into their own balance. Both roles hold the
scope-less `auth:logout`. The acting principal (`UserID`) is always taken from
the verified claim, never from the client payload.

### How a user becomes an admin

Registration always creates a **member** — there is intentionally no
self-service path to admin. An admin is provisioned out-of-band by promoting an
existing user in the database:

```sql
UPDATE users SET role = 'admin' WHERE email = 'ops@example.com';
```

The next login then issues a token carrying the admin permission set. This is a
deliberate trade-off for a take-home (see §7): no admin-bootstrap RPC — promotion
of a normal user is an operator action.

For local development there is a bootstrap command (`go run ./cmd/bootstrap`)
that resets the database and recreates a single well-known **admin** system user
(`system@mail.com` / `systemUser123`), so a login yields an admin token to work
with immediately. It is a dev convenience and a clean-slate reset, not a
production provisioning path.

---

## 5. Batch ingestion and ordering

The CLI (`cmd/cli`, `loyalty-cli ingest`) is a thin client; all business rules
live on the server. It parses the CSV, **sorts rows by `occurred_at` then line**,
and sends the whole batch as **one ordered JSON-RPC request** to the admin-only
`Wallet.ProcessTransactionBatch`.

Why one ordered request rather than a JSON-RPC 2.0 batch array: the JSON-RPC
spec lets a server process a batch in any order and concurrently, and the
gorilla json2 codec does not decode batch arrays at all. Because the overdraft
floor makes each write order-dependent (an earn must be applied before the spend
it funds), ordering has to be guaranteed — so the batch travels as a single
ordered payload and the server is a faithful sequential executor of that order.
Ordering *policy* lives in the CLI; ordering *guarantee* lives in the server.

Reprocessing the same file is safe by construction: idempotency dedupes on `ref`,
so re-ingestion yields duplicates, not double counts. The server returns
per-element outcomes (`accepted` / `duplicate` / `rejected` with a reason) plus
summary tallies; the CLI prints `processed / accepted / duplicates / rejected`
and lists rejections with their line and reason.

---

## 6. Data model

Timestamps are stored as **RFC3339Nano UTC TEXT** — human-readable and
lexicographically sortable, which lets ordering and keyset pagination rely on
plain string comparison. `OwnerID` is a foreign key to `users.id`
(denormalized onto `transactions` and `audit_log` so entries are attributable
without a join); the DTO field is `UserID`, the SQL column is `owner_id`. Schema
lives in `pkg/postgres/migrations/*.sql`, embedded and applied in lexical
filename order on startup and in tests.

---

## 7. Trade-offs and what I'd do next

- **Admin provisioning is manual** (SQL promotion). Fine for a take-home; a real
  system needs an admin-bootstrap path or an invite flow.
- **One wallet per user.** Registration opens a single default account. The data
  model already supports multiple accounts per user (`accounts.owner_id`), so a
  `CreateAccount` RPC is a small addition — I left it out to stay within scope.
- **Permissions are snapshotted into the token.** A role change only takes effect
  on the next login (≤1h, the token TTL). Tokens are revocable via the
  `token_version` epoch (logout-everywhere), but there is no refresh-token flow
  and no per-device revocation — both would be the next step.
- **No rate limiting / request size limits** on ingestion. The batch is applied
  in a single unit of work, which is simple and correct but not ideal for very
  large files; chunking would be the next step.

---

## 8. AI workflow

> This section documents how AI tooling was used, per the assignment. Edit it to
> reflect your own prompts and judgements before submitting.

I used **Claude Code** as a pair-programmer throughout, with a deliberately
plan-first, test-backed workflow rather than free-form generation.

### Conventions encoded as reusable skills

Rather than re-explain the architecture in every prompt, I captured the
recurring, error-prone tasks as Claude Code skills in `.claude/skills/`. Each
skill is a checklist plus a canonical reference the model follows, so generated
code mirrors the existing patterns instead of drifting, and the steps that fail
*silently* are never skipped.

| Skill | What it does | What it deliberately leaves out |
| --- | --- | --- |
| `code-structure` | The arbiter: decides what belongs in the adaptor vs service vs repository, and routes any new write path through the single owner of that mechanic (e.g. `ProcessTransaction`) instead of duplicating it. | — |
| `scaffold-crud-repository` | Given one entity struct, generates the whole persistence layer — port interface + CRUD DTOs, `Validate()`, the Postgres impl (with `ExecutorFromContext` + ownership scoping), the hand-written mock, and table-driven validation tests. | adaptor, policy, wiring |
| `scaffold-service` | Given a capability interface (`AccountOpener`, `UserRegistration`…), generates the impl that orchestrates repositories in one unit of work, its adaptor, mock, `Validate()`, and impl+adaptor unit tests. Writes **no SQL**. | repository SQL, policy, wiring |
| `add-rpc-method` | Makes a method callable over the wire end-to-end: interface, DTO, `Validate()`, adaptor, SQL impl, *and* — critically — the `DefaultPolicy()` entry, the one omission that makes a method silently return "method not allowed" regardless of token. | — |
| `go-unit-tests` | Pure-Go tests (no DB): the mock-first convention, table-driven validations, and the non-negotiable happy-path-**and**-each-error-branch rule. | — |
| `endpoint-integration-test` | Black-box HTTP/JSON-RPC tests that assert both the wire response **and** the persisted rows, plus the mandatory negative cases (unauthenticated, foreign-account, overdraft). Skips cleanly when no server/DB is reachable. | — |

**How they compose.** `code-structure` decides the shape; the two `scaffold-*`
skills generate a layer each (they intentionally stop short of the wire so the
division stays honest); `add-rpc-method` then exposes it and closes the
easy-to-forget policy gap; the two test skills cover it at both the unit and
endpoint level. The scope boundaries are deliberate — each skill's "out of
scope" list points at the next skill, so the model hands work off rather than
half-doing a neighbouring layer. This was the single biggest lever on
consistency: the skills, not me, kept the near-identical files of each domain in
lockstep.

### How I steered the rest

1. **Invariants pinned with prose, then tests.** I asked it to write design
   notes (`scratch/Notes.md`) explaining the unit-of-work and validation
   patterns in plain language, which doubled as a spec I could check the code
   against. Every business rule (idempotency, overdraft floor, ownership
   scoping) got a happy-path *and* an error-case test; I treated a rule as "not
   done" until both existed.

2. **What I accepted vs. rejected.** I accepted the model's mechanical
   scaffolding, SQL boilerplate, and table-driven test stubs readily. I pushed
   back on / edited: the concurrency model (insisted on the single guarded
   `UPDATE` over check-then-insert), the batch transport (one ordered request,
   not a JSON-RPC batch array, because ordering must be guaranteed), and the
   audit-on-rollback behaviour (rejected attempts must survive). These are the
   correctness-critical decisions, so I owned them and used the model to
   implement and test them rather than to decide them.

The net effect: AI accelerated the breadth (lots of small, consistent files and
tests) while the load-bearing decisions stayed human-reviewed and are documented
above.