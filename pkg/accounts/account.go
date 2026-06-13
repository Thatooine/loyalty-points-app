package accounts

import "time"

// Account is a loyalty points wallet. Identity and credentials live on the
// owning User (users.User); a user can hold many accounts. The balance is a
// materialised cache of the transaction ledger: a SUM over the account's
// transactions must always equal it.
type Account struct {
	// AccountID is the caller-supplied natural key, e.g. "member-123".
	AccountID string `json:"accountID"`

	// UserID is the owning user (users.User.ID).
	UserID string `json:"userID"`

	Name string `json:"name"`

	Balance int64 `json:"balance"`

	CreatedAt time.Time `json:"createdAt"`
}
