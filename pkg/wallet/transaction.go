package wallet

import "time"

// Kind classifies a ledger entry: earn credits points, spend debits them.
type Kind string

const (
	KindEarn  Kind = "earn"
	KindSpend Kind = "spend"
)

// Transaction is one entry in the append-only points ledger — the source of
// truth for every account balance.
type Transaction struct {
	// ID is the ledger entry's unique identifier, a UUID assigned at
	// persistence.
	ID string `json:"id"`

	// Ref is the caller-supplied idempotency key: the same ref never counts
	// twice. Unique across the ledger.
	Ref string `json:"ref"`

	AccountID string `json:"accountID"`

	// OwnerID is the owning user of the account (accounts.OwnerID), denormalised
	// onto the ledger so an account's entries can be attributed to an owner
	// without a join. For a member it equals CreatedBy; for an admin action it
	// is the account owner, not the acting admin.
	OwnerID string `json:"ownerID"`

	Kind Kind `json:"kind"`

	// Points is the signed delta as applied (earn=+n, spend=-n), so
	// SUM(points) over an account's transactions equals its balance.
	Points int64 `json:"points"`

	// OccurredAt is the business timestamp supplied by the caller.
	OccurredAt time.Time `json:"occurredAt"`

	// RecordedAt is the server timestamp, giving listings a trustworthy sort
	// order independent of caller clocks.
	RecordedAt time.Time `json:"recordedAt"`

	// CreatedBy is the acting principal (member or admin account ID).
	CreatedBy string `json:"createdBy"`
}
