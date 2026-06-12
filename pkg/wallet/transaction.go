package wallet

import "time"

// Kind classifies a ledger entry. Earn and spend come from members; adjust is
// admin-only and the only kind whose API points are signed.
type Kind string

const (
	KindEarn   Kind = "earn"
	KindSpend  Kind = "spend"
	KindAdjust Kind = "adjust"
)

// Transaction is one entry in the append-only points ledger — the source of
// truth for every account balance.
type Transaction struct {
	// Ref is the caller-supplied idempotency key and the ledger's natural
	// key: the same ref never counts twice.
	Ref string `json:"ref"`

	AccountID string `json:"accountID"`

	Kind Kind `json:"kind"`

	// Points is the signed delta as applied (earn=+n, spend=-n, adjust=±n),
	// so SUM(points) over an account's transactions equals its balance.
	Points int64 `json:"points"`

	// OccurredAt is the business timestamp supplied by the caller.
	OccurredAt time.Time `json:"occurredAt"`

	// RecordedAt is the server timestamp, giving listings a trustworthy sort
	// order independent of caller clocks.
	RecordedAt time.Time `json:"recordedAt"`

	// CreatedBy is the acting principal (member or admin account ID).
	CreatedBy string `json:"createdBy"`
}
