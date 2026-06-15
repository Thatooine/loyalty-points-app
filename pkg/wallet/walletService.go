package wallet

import (
	"context"
	"time"
)

// WalletService owns the wallet's business rules. ProcessTransaction is the
// single write path for every earn, spend, and adjustment — from the API and
// from CSV batch ingestion alike — so idempotency, the overdraft floor, and
// the audit trail are enforced once and tested once.
type WalletService interface {
	ProcessTransaction(ctx context.Context, request ProcessTransactionRequest) (*ProcessTransactionResponse, error)

	// ProcessTransactionBatch applies an ordered batch as a single, sequential
	// unit. Because the balance floor makes each write order-dependent, the
	// transactions are applied strictly in slice order — never concurrently and
	// never reordered — so the caller's ordering (the CLI sorts by OccurredAt,
	// then line) is the order they post. Each element is still its own unit of
	// work: a rejected element does not roll back earlier accepted ones, and a
	// previously seen ref is reported as a duplicate rather than re-applied.
	ProcessTransactionBatch(ctx context.Context, request ProcessTransactionBatchRequest) (*ProcessTransactionBatchResponse, error)
}

// ProcessTransactionRequest is the request for ProcessTransaction.
type ProcessTransactionRequest struct {
	// Ref is the idempotency key: the same ref never counts twice.
	Ref string

	AccountID string

	Kind Kind

	// Points is positive for earn and spend (the sign is derived from Kind);
	// for adjust it is the signed delta as supplied by the admin.
	Points int64

	// OccurredAt is the business timestamp of the transaction. Optional: when it
	// is the zero value the service stamps it with the processing time, so it is
	// never persisted empty.
	OccurredAt time.Time

	// Actor is the acting principal (the user ID submitting the transaction).
	Actor string

	// ActorIsAdmin reports whether the actor holds the admin role. Admins may
	// transact on any account; a non-admin actor may only transact on an
	// account they own. The adaptor sets this from the verified login claim —
	// it is never taken from client input.
	ActorIsAdmin bool
}

// ProcessTransactionResponse is the response for ProcessTransaction.
type ProcessTransactionResponse struct {
	// Transaction is the ledger entry — the newly recorded one, or the
	// original when Duplicate is true.
	Transaction Transaction

	// Balance is the account balance after processing.
	Balance int64

	// Duplicate is true when this ref was already recorded; the original
	// outcome is returned unchanged and no new effect is applied.
	Duplicate bool
}

// ProcessTransactionBatchRequest is an ordered batch of transactions. They are
// applied in slice order; the caller is responsible for sorting them into the
// intended chronology before submitting.
type ProcessTransactionBatchRequest struct {
	Transactions []ProcessTransactionRequest
}

// BatchOutcome is the per-element result classification.
type BatchOutcome string

const (
	// BatchOutcomeAccepted means the transaction was applied.
	BatchOutcomeAccepted BatchOutcome = "accepted"
	// BatchOutcomeDuplicate means the ref was already recorded; no new effect.
	BatchOutcomeDuplicate BatchOutcome = "duplicate"
	// BatchOutcomeRejected means the transaction was not applied (validation,
	// overdraft floor, unknown account, or ownership).
	BatchOutcomeRejected BatchOutcome = "rejected"
)

// BatchElementResult is the outcome of a single transaction within a batch.
type BatchElementResult struct {
	// Ref echoes the element's idempotency key so the caller can correlate
	// results back to input rows.
	Ref string
	// Outcome classifies what happened.
	Outcome BatchOutcome
	// Reason is a human-readable explanation, set only when Outcome is rejected.
	Reason string
	// Balance is the account balance after processing, set for accepted and
	// duplicate outcomes.
	Balance int64
}

// ProcessTransactionBatchResponse is the per-element outcome of a batch, in the
// same order as the request, with summary tallies.
type ProcessTransactionBatchResponse struct {
	Results   []BatchElementResult
	Accepted  int
	Duplicate int
	Rejected  int
}
