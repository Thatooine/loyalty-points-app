package audits

import "time"

// Outcome is the result of a transaction-processing attempt.
type Outcome string

const (
	OutcomeAccepted  Outcome = "accepted"
	OutcomeRejected  Outcome = "rejected"
	OutcomeDuplicate Outcome = "duplicate"
)

// AuditEntry records one transaction-processing attempt. Unlike the ledger, the
// same transaction ref can appear many times here.
type AuditEntry struct {
	ID int64 `json:"id"`

	UserID string `json:"userID"`

	// TransactionRef is nullable: a malformed CSV row may not have one.
	TransactionRef *string `json:"transactionRef"`

	AccountID *string `json:"accountID"`

	// OwnerID is nullable: an unknown account (or malformed row) has no owner to record.
	OwnerID *string `json:"ownerID"`

	Kind   *string `json:"kind"`
	Points *int64  `json:"points"`

	Outcome Outcome `json:"outcome"`

	Reason string `json:"reason"`

	CreatedAt time.Time `json:"createdAt"`
}
