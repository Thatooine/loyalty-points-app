package users

import "time"

// Role determines what a user may do: members act on their own accounts,
// admins act on any account.
type Role string

const (
	RoleMember Role = "member"
	RoleAdmin  Role = "admin"
)

type User struct {
	ID string `json:"id"`

	Email string `json:"email"`

	PasswordHash string `json:"-"`

	Role Role `json:"role"`

	CreatedAt time.Time `json:"createdAt"`

	// TokenVersion is the user's session epoch; incrementing it (on logout)
	// invalidates all of that user's outstanding tokens at once.
	TokenVersion int64 `json:"-"`
}
