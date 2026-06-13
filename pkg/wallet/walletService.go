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

	// OccurredAt is the business timestamp supplied by the caller.
	OccurredAt time.Time

	// Actor is the acting principal (the user ID submitting the transaction).
	Actor string

	// Source records where the attempt came from: "api", "admin", or
	// "batch:<filename>".
	Source string
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
