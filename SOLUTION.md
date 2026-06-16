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

Every choice below is framed the same way: what it *buys* (the gain) and what it
*costs* (the trade-off I accepted). Nothing here is free; these are the calls I'd
defend in review.

**PostgreSQL, not SQLite.** The assignment recommends SQLite and allows Postgres.

- *Gain.* The correctness story for concurrent writes is the interesting part of
  this problem, and Postgres gives real row-level locking and a guarded-`UPDATE`
  concurrency model without SQLite's single-writer caveats. The overdraft floor
  (§3) leans directly on this — two overlapping spends serialise on the row, not
  on a process-wide write lock. It also has the richer toolbox I actually used:
  `CHECK` constraints as backstops, partial/composite indexes for keyset
  pagination, `GENERATED ALWAYS AS IDENTITY`.
- *Trade-off.* A real network dependency and a container to run, versus SQLite's
  zero-setup single file. I bought that back with `docker compose up` +
  baked-in defaults so local setup is still one command, but a reviewer who just
  wants `go run` with no Docker pays a small tax SQLite wouldn't have charged.

**JSON-RPC 2.0 over `gorilla/rpc/v2`.** A single `/api` endpoint hosts every
service; adaptor methods with the signature
`func(r *http.Request, params *T, result *T2) error` auto-register as
`<ServiceName>.<Method>`.

- *Why it's the right fit.* A loyalty ledger is a set of **verbs**, not a set of
  resources: `EarnPoints`, `SpendPoints`, `Logout`, `ProcessTransactionBatch`.
  These are commands with side effects, and they map cleanly onto named methods —
  whereas forcing them into REST means inventing resource nouns and arguing over
  whether "spend" is a `POST /accounts/{id}/debits` or a `PATCH` on the balance.
  JSON-RPC sidesteps that impedance mismatch entirely: the method *is* the
  operation, so the API reads the way the domain is actually spoken.
- *Gain.* Several things fall out of that for free:
  - **Declarative wire layer.** Adaptor methods auto-register by signature — no
    hand-rolled dispatcher, no per-route table, no verb/path bikeshedding. Adding
    a method is adding a function.
  - **One choke point for cross-cutting concerns.** Because every call lands on a
    single endpoint, authorization is *one* middleware in front of *one* mux
    route (§4), and logging/request-id correlation is likewise mounted once.
    Per-handler guards that are easy to forget simply don't exist — the gate is
    structural, not per-route discipline.
  - **The policy table is the attack surface.** Method-name dispatch makes
    `method → permissions` (`DefaultPolicy()`) a literal, auditable map of every
    callable operation. A method missing from the policy fails *closed*. You can
    read the whole authorization model in one file.
  - **Transport-agnostic and uniform.** One envelope, one content type, one error
    shape (sentinel → code, mapped in exactly one place — §6/error handling).
    Clients — including the CLI — share a single request/response codec instead
    of bespoke handling per route. The contract is symmetric and trivial to mock.
  - **Batches are first-class.** The domain genuinely needs an *ordered* batch
    (the overdraft floor makes writes order-dependent — §5); a single method
    taking an ordered slice expresses that far more naturally than orchestrating
    N REST calls and hoping they serialise.
- *Trade-off (and why it's cheap here).* JSON-RPC gives up HTTP-native semantics:
  everything is `POST`, errors come back as **HTTP 200** with the error in the
  body, and there's no resource-level caching or status-code tooling. But this
  ledger has almost no cacheable reads — calls are commands — so HTTP caching
  buys little, and the uniform in-body error shape is actually an *advantage* for
  a typed client over decoding meaning from status codes. The one real papercut
  is that the gorilla json2 codec won't decode JSON-RPC *batch arrays*, which is
  partly why batch ingestion travels as one ordered payload (§5) — and since I
  wanted a guaranteed order anyway, that constraint pushed me toward the design I
  already preferred.

**Ports and adapters.** `pkg/<domain>` holds the interfaces, DTOs, `Validate()`
methods, and JSON-RPC adaptors; `internal/pkg/<domain>` holds the PostgreSQL
implementations. Production code depends only on the `pkg` interfaces, and
`cmd/app/serviceProviders.go` is the single wiring point.

- *Gain.* The system is testable with mocks at the port boundary (the bulk of the
  suite needs no DB), the SQL is isolated to one layer, and swapping an
  implementation is a one-line change in the wiring file. The split also gives a
  natural home for the `internal/` visibility boundary so nothing imports a
  concrete repository by accident.
- *Trade-off.* More files and a layer of indirection (interface + DTO + impl +
  mock) per capability than a flat handler-hits-DB design. For a service this
  size that's real ceremony; I leaned on the `.claude/skills/` scaffolds (§8) to
  keep the per-domain boilerplate consistent rather than hand-written.

**`pgx/v5` via the `database/sql` stdlib interface.** The driver is registered
through `pgx`'s `stdlib` shim rather than its native API.

- *Gain.* I keep the standard `*sql.DB`/`*sql.Tx` abstractions — which is what
  the context-resolved executor and `TxManager` (§3) are built on — while still
  getting pgx's well-maintained Postgres driver and connection pool. Staying on
  the stdlib interface means the repositories aren't coupled to pgx types.
- *Trade-off.* I forgo pgx's native niceties (richer type mapping, `COPY`
  protocol, batch pipelining). None were needed here, and the portability of the
  stdlib seam was worth more than the raw-throughput features.

**JWS / RS256 via `go-jose`, not symmetric HS256.** Access tokens are
asymmetrically signed (§4).

- *Gain.* Only the holder of the private key can mint tokens; any service can
  verify with the *public* key. That's the right shape for a system that might
  later split issuer from verifier, and it means a leaked verification key can't
  forge tokens. `go-jose` is a mature, audited JOSE implementation, so I'm not
  hand-rolling crypto.
- *Trade-off.* RS256 is heavier than HMAC and brings asymmetric-key management
  (a PEM to provision; a dev key is baked in for local use only). For a
  single-process deployment HS256 would have been simpler — I paid for the
  verification model deliberately, anticipating split trust domains.

**`bcrypt` (`golang.org/x/crypto`) for password storage.**

- *Gain.* Adaptive, salted hashing with the salt embedded in the digest, so
  there's no separate salt column and the work factor can be raised over time.
  It's the conservative, well-understood default.
- *Trade-off.* bcrypt is deliberately slow (that's the point) and caps the input
  at 72 bytes; a memory-hard function (argon2/scrypt) resists GPU attacks better.
  bcrypt was the lower-risk, battle-tested pick for a take-home over a more
  modern but fussier KDF.

**`zerolog` for structured logging.** Mounted as the first middleware on `/api`.

- *Gain.* Zero-allocation structured (JSON) logs that are machine-parseable, plus
  a context-bound logger: the request middleware mints a `request_id` and binds
  it so every downstream `log.Ctx(ctx)…` line is correlated for free. Structured
  fields beat string-formatted logs the moment you need to grep production.
- *Trade-off.* A specific logging dependency threaded through the code via
  `log.Ctx(ctx)`, versus the stdlib `log`/`slog`. Given `slog` would cover much
  of this now, this is the choice I hold most loosely — but the context-bound
  correlation was the deciding feature.

**`cobra` + `viper` for the CLI and config.**

- *Gain.* `cobra` gives the CLI a real command tree, flag parsing, and help text
  for `loyalty-cli ingest` with little code; `viper` gives layered config
  (env-var binding over baked-in defaults) so the server runs out of the box yet
  every secret is overridable by environment variable.
- *Trade-off.* Both are heavy dependencies for what is, today, one subcommand and
  two config keys — arguably more machinery than the surface needs. I took the
  headroom because batch ingestion is the kind of feature that grows
  subcommands, and viper's env binding is exactly the 12-factor seam a real
  deployment wants.

**Hand-rolled embedded migrations, no migration library.** Numbered
`pkg/postgres/migrations/*.sql` files are `go:embed`-ed and applied in lexical
order on startup and in tests.

- *Gain.* Schema travels *inside* the binary (no external migration tool to
  install or version-match), the same code path runs in production and in the
  per-test fresh-schema harness, and "evolve the schema" is just "add the next
  numbered file." Zero operational moving parts.
- *Trade-off.* No down-migrations, no checksum/dirty-state tracking, no
  out-of-order detection — the safety net a `golang-migrate`/`goose` gives you.
  For a forward-only take-home that's fine; a long-lived service would outgrow it
  and want a real migration tool.

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

The approach I rejected here was the obvious one: `SELECT` the balance, compare
in Go, then `UPDATE`. That reads cleanly but opens a race window between the read
and the write — two concurrent spends both read the same balance, both pass the
check, and both write, driving the balance negative. Pushing the check into the
`WHERE` clause collapses read-check-write into a single atomic statement and lets
Postgres serialise the contending writers on the row, so the floor holds without
an explicit lock or a higher isolation level.

**Concurrency safety — a unit of work via context-resolved executor.**
Repositories never hold a `*sql.Tx`. Each resolves its executor from the context
(`pkgSQL.ExecutorFromContext`): if `TxManager.RunInTx` placed a transaction on
the context, the repository uses it; otherwise it uses the pool. So the *caller*
decides what is atomic, and the same repository code works standalone or
composed. `ProcessTransaction` wraps the ledger insert, the balance update, and
the audit write in one `RunInTx` so they commit or roll back together.

The implementation choice worth calling out is *how* the transaction reaches the
repositories. The alternative is to thread a `*sql.Tx` (or a `Querier`
interface) through every repository method signature. I rejected that because it
forces every method to exist in two flavours — one on the pool, one on a tx — or
to leak the transaction into its signature even when it runs standalone. Stashing
the executor on the context instead means each repository writes one query path
against a small `Executor` interface (the common subset of `*sql.DB` and
`*sql.Tx`), and `RunInTx` decides atomicity from the outside. The cost I accepted
is that the transaction travels *implicitly* on the context rather than visibly in
the signature — slightly more magic — but it keeps the repository layer free of
transaction bookkeeping, which was the trade I wanted.

**Audit trail — survives rollback when it must.** Accepted and duplicate
attempts are audited *inside* the unit of work (they commit with the ledger
row). Rejected attempts are audited on the *plain* context (the pool), so the
audit row survives the rolled-back transaction — a rejection is still recorded
even though nothing else was written. The naive approach — write every audit row
inside the same transaction — was wrong precisely for rejections: rolling back the
overdraft means rolling back its own audit record, so the trail would silently
lose every failed attempt. Splitting the rejected write onto the pool is what
keeps "someone tried to overspend" durable.

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

The fail-closed behaviour is a deliberate implementation choice, not an accident:
a method absent from the policy map falls through to "deny", never to "allow". I
preferred this over the easy alternative of guarding each handler individually
(`if !authorized { … }`) because a per-handler check is something you can *forget*
to add when you write a new method, and the failure mode of forgetting is an open
endpoint. Here the failure mode of forgetting is a *closed* endpoint — the new
method simply returns "method not allowed" until it is listed — so the unsafe
mistake is structurally impossible and the safe mistake is loud and immediate.

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
live on the server. It parses the CSV, **sorts rows by `occurred_at` then line**
(`ingest.SortRows`, a stable sort with the 1-based data line as the deterministic
tiebreaker; a blank `occurred_at` sorts first), and sends the whole batch as
**one ordered JSON-RPC request** to the admin-only
`Wallet.ProcessTransactionBatch`.

Why one ordered request rather than a JSON-RPC 2.0 batch array: the JSON-RPC
spec lets a server process a batch in any order and concurrently, and the
gorilla json2 codec does not decode batch arrays at all. Because the overdraft
floor makes each write order-dependent (an earn must be applied before the spend
it funds), ordering has to be guaranteed — so the batch travels as a single
ordered payload and the server applies it sequentially.

**Ordering is enforced on the server, not just the client.** Sorting in the CLI
alone would leave the invariant at the mercy of whichever client sent the
batch — a direct RPC caller could submit rows out of order and silently trip the
overdraft floor. So `ProcessTransactionBatch` re-sorts by `occurred_at` itself,
making correct chronology a property of the server. Two implementation details
there were deliberate. It sorts a *copy* of the input slice rather than in place,
so the caller's slice is never mutated under it and the per-element results can
still be reported against the order the client actually sent. And the sort is
*stable* with submission order as the implicit tiebreaker, so two transactions
with equal or absent timestamps keep their original relative order — without
stability, equal-timestamped rows could reorder run-to-run and an earn could land
after the spend it funds purely by sort nondeterminism. The CLI's own sort is
still useful — it lets the `--dry-run` preview show the true application order
before anything is sent — but it is now belt-and-braces, not the only line of
defence. Because the server may reorder, per-element results come back in
*applied* order and callers correlate them by `ref`, not by position (the CLI
keys its summary off `ref`).

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

This is the single inventory of what I *deliberately did not build* — the
out-of-scope notes scattered above all resolve here. Each was a conscious cut to
keep the take-home focused on the correctness-critical core, not an oversight, and
each has a clear next step.

- **Admin provisioning is manual** (SQL promotion — §4). Fine for a take-home; a
  real system needs an admin-bootstrap path or an invite flow. I chose manual
  promotion over an admin-creates-admin RPC specifically to avoid shipping a
  privilege-escalation surface I couldn't fully harden in scope.
- **One wallet per user.** Registration opens a single default account. The data
  model already supports multiple accounts per user (`accounts.owner_id`), so a
  `CreateAccount` RPC is a small addition — I left it out to stay within scope.
- **Permissions are snapshotted into the token.** A role change only takes effect
  on the next login (≤1h, the token TTL). Tokens are revocable via the
  `token_version` epoch (logout-everywhere — §4), but there is no refresh-token
  flow and no per-device revocation. I chose not to build per-device revocation
  because it needs per-token tracking (a server-side token table), which trades
  away the stateless-JWT property the whole auth model is built on; the epoch
  counter buys "log out everywhere" for one indexed read instead.
- **No rate limiting / request size limits** on ingestion. The batch is applied
  in a single unit of work, which is simple and correct but not ideal for very
  large files; chunking would be the next step.
- **Forward-only migrations** (§2). No down-migrations, checksums, or dirty-state
  tracking. I decided against pulling in `golang-migrate`/`goose` because the
  embedded-and-applied-on-startup approach has zero operational moving parts and
  is enough for a forward-only take-home; a long-lived service would outgrow it.

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

I accepted the model's mechanical scaffolding, SQL boilerplate, and table-driven
test stubs readily. I pushed back on / edited the correctness-critical calls: the
concurrency model (insisted on the single guarded `UPDATE` over check-then-insert),
the batch transport (one ordered request, not a JSON-RPC batch array, because
ordering must be guaranteed), and the audit-on-rollback behaviour (rejected
attempts must survive). I owned those decisions and used the model to implement
and test them rather than to decide them — so AI accelerated the breadth while the
load-bearing decisions stayed human-reviewed.