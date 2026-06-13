package accounts

import "time"

// Account is a loyalty points wallet. Identity and credentials live on the
// owning User (users.User); a user can hold many accounts. The balance is a
// materialised cache of the transaction ledger: a SUM over the account's
// transactions must always equal it.
type Account struct {
	// ID is the account's unique identifier, a UUID assigned at persistence.
	ID string `json:"id"`

	// UserID is the owning user (users.User.ID).
	UserID string `json:"userID"`

	Name string `json:"name"`

	Balance int64 `json:"balance"`

	CreatedAt time.Time `json:"createdAt"`
}
