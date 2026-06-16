package wallets

import (
	"context"
	"time"
)

type WalletService interface {
	ProcessTransaction(ctx context.Context, request ProcessTransactionRequest) (*ProcessTransactionResponse, error)

	EarnPoints(ctx context.Context, request EarnPointsRequest) (*ProcessTransactionResponse, error)

	SpendPoints(ctx context.Context, request SpendPointsRequest) (*ProcessTransactionResponse, error)

	ProcessTransactionBatch(ctx context.Context, request ProcessTransactionBatchRequest) (*ProcessTransactionBatchResponse, error)
}

type ProcessTransactionRequest struct {
	// UserID is the acting principal (the user ID submitting the transaction).
	// Ownership is enforced by scoping every account read/update to this user,
	// so a caller can only transact on an account they own.
	UserID string

	// Ref is the idempotency key: the same ref never counts twice.
	Ref string

	AccountID string

	Kind Kind

	// Points is the positive amount for earn and spend; the sign is derived
	// from Kind.
	Points int64

	// OccurredAt is the business timestamp of the transaction.
	OccurredAt time.Time
}

// Mirrors ProcessTransactionRequest minus Kind, which the method fixes to KindEarn.
type EarnPointsRequest struct {
	// UserID is the acting principal (the user ID submitting the transaction).
	UserID string

	// Ref is the idempotency key: the same ref never counts twice.
	Ref string

	AccountID string

	// Points is the positive amount to credit.
	Points int64

	// OccurredAt is the business timestamp of the transaction. Optional: when it
	// is the zero value the service stamps it with the processing time.
	OccurredAt time.Time
}

// Mirrors ProcessTransactionRequest minus Kind, which the method fixes to KindSpend.
type SpendPointsRequest struct {
	// UserID is the acting principal (the user ID submitting the transaction).
	UserID string

	// Ref is the idempotency key: the same ref never counts twice.
	Ref string

	AccountID string

	// Points is the positive amount to debit; the debit is subject to the
	// balance floor.
	Points int64

	// OccurredAt is the business timestamp of the transaction. Optional: when it
	// is the zero value the service stamps it with the processing time.
	OccurredAt time.Time
}

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
