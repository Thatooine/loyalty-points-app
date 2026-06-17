package wallets

import "time"

// Kind classifies a ledger entry: earn credits points, spend debits them.
type Kind string

const (
	KindEarn  Kind = "earn"
	KindSpend Kind = "spend"
)

type Transaction struct {
	ID string `json:"id"`

	// Ref is the caller-supplied idempotency key: the same ref never counts twice.
	Ref string `json:"ref"`

	AccountID string `json:"accountID"`

	OwnerID string `json:"ownerID"`

	Kind Kind `json:"kind"`

	// Points is the signed delta as applied (earn=+n, spend=-n), so SUM(points)
	// over an account's transactions equals its balance.
	Points int64 `json:"points"`

	OccurredAt time.Time `json:"occurredAt"`

	RecordedAt time.Time `json:"recordedAt"`

	CreatedBy string `json:"createdBy"`
}
