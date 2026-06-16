---
name: code-structure
description: Use when deciding what belongs in the adaptor vs the service vs the repository, when multiple RPC methods duplicate the same operational logic, or when refactoring repeated blocks across domain flows. Use when adding a feature that shares mechanics with an existing one (e.g. a new write path that should run through ProcessTransaction).
---

# Layered Architecture (ports-and-adapters)

## Overview

**Three layers, each owning one thing.** Requests flow `adaptor → service → repository`. Each layer has a single responsibility, and operational mechanics live in exactly one place so a fix in one path can't miss another.

| Layer | Lives in | Owns ("the…") |
|---|---|---|
| **Adaptor** (wire) | `pkg/<domain>/*JSONRPCAdaptor.go` | transport: claim extraction, wire DTO ↔ service DTO, client-facing error mapping |
| **Service** (orchestration) | `internal/pkg/<domain>/*ServiceImpl.go` | domain rules, invariants, unit-of-work composition, the "why/when" |
| **Repository** (mechanics) | `internal/pkg/<domain>/*RepositoryImpl.go` | reusable, policy-free persistence: one SQL operation, the "how" |

The `pkg/<domain>` side declares the *ports* (interfaces + DTOs + `Validate()`); `internal/pkg/<domain>` holds the implementations. Production code depends on the interfaces; `cmd/app/serviceProviders.go` is the single wiring point.

This prevents duplicated code, inconsistent behavior, and bugs fixed in one path but not the others.

## When to Use

- Two RPC methods need the same operation (process a ledger write, map an error, build an audit row)
- You're copy-pasting a block between adaptor methods or between service methods
- A fix in one write path doesn't propagate to the others doing the same thing
- Adding a feature that shares mechanics with an existing flow

**Don't use when:** the logic is genuinely specific to one caller and used nowhere else (over-abstraction is its own anti-pattern).

## Core Pattern

```
Adaptor (wire)                 Service (orchestration)          Repository (mechanics)
├── reads login claim          ├── owns invariants             ├── one SQL operation
│   (UserID, Role)             │   (idempotency, overdraft)    ├── ownership-scoped in SQL
├── wire DTO ↔ service DTO     ├── composes repos in a         │   (owner_id = $UserID
├── maps errs.* → opaque       │   unit of work (RunInTx)      │    unless :all)
│   client error               ├── classifies failures        ├── returns errs.* sentinels
└── calls ONE service method   └── calls repository methods    └── holds NO domain policy
```

**Rule of thumb:**
- "What this domain action means / when it's allowed" → service
- "How to do this persistence operation reliably" → repository
- "How the wire sees it" → adaptor

## The canonical example: one write path

`WalletServiceImpl.ProcessTransaction` (`internal/pkg/wallets/walletServiceImpl.go`) is the *single* path every ledger write flows through. It composes the account, transaction, and audit repositories inside one `RunInTx` so idempotency, the overdraft floor, and the audit trail are enforced — and tested — in one place.

`EarnPoints`, `SpendPoints`, and `ProcessTransactionBatch` add **nothing operational** — each fixes `Kind` and delegates:

```go
func (s *WalletServiceImpl) EarnPoints(ctx context.Context, request pkgWallet.EarnPointsRequest) (*pkgWallet.ProcessTransactionResponse, error) {
	return s.ProcessTransaction(ctx, pkgWallet.ProcessTransactionRequest{
		UserID: request.UserID, Ref: request.Ref, AccountID: request.AccountID,
		Kind: pkgWallet.KindEarn, // ← the only thing this wrapper adds
		Points: request.Points, OccurredAt: request.OccurredAt,
	})
}
```

There is no second write path to keep in sync. When you add a new way to write the ledger, **route it through `ProcessTransaction`** — do not reimplement the insert/balance/audit dance.

The adaptor mirrors this: `mapProcessError` and `fillProcessResult` are shared by `ProcessTransaction`, `EarnPoints`, and `SpendPoints` so the error mapping and response shaping live once.

## Designing service & repository methods

Design as **composable capability blocks**, not monoliths:

- Accept all required data as **explicit fields on a request struct** (`XxxRequest`); return a structured `*XxxResponse`.
- Repositories never reach for a `*sql.Tx` — they call `pkgSQL.ExecutorFromContext(ctx, r.db)`, so the same method works standalone or inside a service's `RunInTx`. That's what lets a service compose several repo calls atomically.
- Repositories hold **no domain policy**. Ownership is the one exception, and it's expressed as SQL scoping (`IsGranted(ctx, ...All)` → else `owner_id = $UserID`), returning `ErrNotFound` for a non-owner (no existence leak) — not an `if` in the service.
- Make failure explicit: repositories return `errs.*` sentinels (`ErrDuplicateRef`, `ErrInsufficientBalance`, `ErrNotFound`); the service classifies them; the adaptor maps them to opaque client errors.
- Idempotency/atomicity belong to the operation, not the caller: e.g. the `UNIQUE(ref)` insert *is* the dedupe, and a single guarded `UPDATE ... WHERE balance + delta >= 0` *is* the overdraft check. Never check-then-write.

## Migration Checklist

When extracting shared logic:

1. Write the flow inline first (clear behavior) — usually in the service.
2. Mark the operational chunks repeated across callers (e.g. building an audit row, mapping an error).
3. Extract **only** the repeated, non-domain chunks (an unexported helper like `buildAuditEntry`/`reasonFor`, or a repository method).
4. Replace one caller → `go build ./... && go test ./...` → replace the rest.
5. Keep domain policy in the service, ownership scoping in the repository SQL, transport concerns in the adaptor.
6. Touch both sides when adding an interface method: the port (`pkg`) gets the interface + DTO + `Validate()`; the impl (`internal/pkg`) gets the SQL. New RPC methods also need a `DefaultPolicy()` entry (`pkg/authorization/policy.go`) or they're rejected — see the `add-rpc-method` skill.

## Anti-Patterns

| Anti-Pattern | Problem | Here it looks like |
|---|---|---|
| **Second write path** | A bug fixed in one path lives on in the other | A new earn/spend flow that inserts + updates balance itself instead of calling `ProcessTransaction` |
| **Leaky repository** | Mechanics layer makes policy decisions | A repo deciding *whether* the caller may act, beyond the `owner_id` scoping |
| **Fat adaptor** | Domain rules in the wire layer | Computing balances or enforcing the overdraft floor inside a `*JSONRPCAdaptor` method |
| **Check-then-write** | Race between read and write | `if exists(ref) { ... }` instead of relying on `UNIQUE(ref)`; read balance then update instead of the guarded `UPDATE` |
| **Inconsistent API** | Each method invents its own arg/error style | Some methods take a request struct, others loose params; ad-hoc error strings instead of `errs.*` |
| **Over-abstraction** | Indirection with one caller | Extracting a "helper" used by exactly one method |

## Mental Model

```
New write/flow?
  → Does an operation already do this? → route through it (ProcessTransaction)
  → New domain rule/invariant?         → service, inside RunInTx if multi-repo
  → New persistence primitive?         → repository method (port + impl), policy-free
  → New wire surface?                  → adaptor method + DefaultPolicy() entry
  → Repeated block across callers?     → extract one helper, replace callers, verify
  → Used once?                         → leave it inline
```

In one sentence: **adaptors handle the wire, services orchestrate domain invariants in a unit of work, repositories provide reusable policy-free persistence — and every variant of an operation routes through its single canonical path.**
