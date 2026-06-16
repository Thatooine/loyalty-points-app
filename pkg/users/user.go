package users

import "time"

// Role determines what a user may do: members act on their own accounts,
// admins act on any account.
type Role string

const (
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

// User is an authenticated principal. A user owns zero or more wallet
// accounts (Account.OwnerID points back here); credentials and identity live
// on the user, never on the account.
type User struct {
	ID string `json:"id"`

	Email string `json:"email"`

	// PasswordHash is the bcrypt hash of the user's password. Never
	// serialised into any RPC response.
	PasswordHash string `json:"-"`

	Role Role `json:"role"`

	CreatedAt time.Time `json:"createdAt"`

	// TokenVersion is the user's session epoch. It is stamped into every access
	// token issued for the user and re-checked on each protected request;
	// incrementing it (on logout) invalidates all of that user's outstanding
	// tokens at once. Never serialised into an RPC response.
	TokenVersion int64 `json:"-"`
}
