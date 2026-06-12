package accounts

import "time"

// Role determines what an account holder may do: members act on their own
// account, admins act on any account.
type Role string

const (
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

// Account is a loyalty points wallet holder. The balance is a materialised
// cache of the transaction ledger: a SUM over the account's transactions must
// always equal it.
type Account struct {
	// AccountID is the caller-supplied natural key, e.g. "member-123".
	AccountID string `json:"accountID"`

	Name string `json:"name"`

	Role Role `json:"role"`

	// PasswordHash is the bcrypt hash of the account's password. Never
	// serialised into any RPC response.
	PasswordHash string `json:"-"`

	Balance int64 `json:"balance"`

	CreatedAt time.Time `json:"createdAt"`
}
