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
	// UserID is the acting principal; ownership is enforced by scoping every
	// account read/update to it, so a caller can only transact on accounts they own.
	UserID string

	// Ref is the idempotency key: the same ref never counts twice.
	Ref string

	AccountID string

	Kind Kind

	Points int64

	OccurredAt time.Time
}

type EarnPointsRequest struct {
	UserID string

	Ref string

	AccountID string

	Points int64

	// OccurredAt is optional: when zero the service stamps it with the processing time.
	OccurredAt time.Time
}

type SpendPointsRequest struct {
	UserID string

	Ref string

	AccountID string

	Points int64

	// OccurredAt is optional: when zero the service stamps it with the processing time.
	OccurredAt time.Time
}

type ProcessTransactionResponse struct {
	// Transaction is the newly recorded entry, or the original when Duplicate is true.
	Transaction Transaction

	Balance int64

	// Duplicate is true when this ref was already recorded; the original outcome
	// is returned unchanged and no new effect is applied.
	Duplicate bool
}

// ProcessTransactionBatchRequest is a batch of transactions. The server applies
// them in chronological order (by OccurredAt), so the caller need not pre-sort;
// submission order is the stable tiebreaker for equal or absent timestamps.
type ProcessTransactionBatchRequest struct {
	Transactions []ProcessTransactionRequest
}

type BatchOutcome string

const (
	BatchOutcomeAccepted  BatchOutcome = "accepted"
	BatchOutcomeDuplicate BatchOutcome = "duplicate"
	BatchOutcomeRejected  BatchOutcome = "rejected"
)

type BatchElementResult struct {
	Ref     string
	Outcome BatchOutcome
	// Reason is set only when Outcome is rejected.
	Reason string
	// Balance is set for accepted and duplicate outcomes.
	Balance int64
}

// ProcessTransactionBatchResponse holds the per-element outcomes in the order
// the server applied them. Correlate elements back to inputs by Ref, not position.
type ProcessTransactionBatchResponse struct {
	Results   []BatchElementResult
	Accepted  int
	Duplicate int
	Rejected  int
}
