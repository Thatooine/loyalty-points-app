package audit

import "time"

// Outcome is the result of a transaction-processing attempt. Every attempt
// resolves to exactly one outcome.
type Outcome string

const (
	OutcomeAccepted  Outcome = "accepted"
	OutcomeRejected  Outcome = "rejected"
	OutcomeDuplicate Outcome = "duplicate"
)

// AuditEntry records one transaction-processing attempt — accepted, rejected,
// or duplicate — with the reason and a timestamp. Unlike the ledger, the same
// transaction ref can appear many times here (each reprocessing attempt is logged).
type AuditEntry struct {
	// ID is the autoincrement key; an attempt log has no natural key.
	ID int64 `json:"id"`

	// TransactionRef is nullable: a malformed CSV row may not have one.
	TransactionRef *string `json:"transactionRef"`

	AccountID *string `json:"accountID"`

	// Kind and Points echo the attempted payload so the audit trail is
	// readable without cross-referencing the ledger.
	Kind   *string `json:"kind"`
	Points *int64  `json:"points"`

	Outcome Outcome `json:"outcome"`

	// Reason explains the outcome: 'ok', 'insufficient balance', 'unknown
	// account', ...
	Reason string `json:"reason"`

	// Actor is the principal that submitted the attempt.
	Actor string `json:"actor"`

	CreatedAt time.Time `json:"createdAt"`
}
