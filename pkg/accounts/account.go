package accounts

import "time"

// Account is a loyalty points wallet. Balance is a materialised cache of the
// transaction ledger: SUM(points) over the account's transactions must equal it.
type Account struct {
	ID string `json:"id"`

	OwnerID string `json:"ownerID"`

	Name string `json:"name"`

	Balance int64 `json:"balance"`

	CreatedAt time.Time `json:"createdAt"`
}
