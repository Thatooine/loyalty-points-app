package authentication

import "github.com/Thatooine/loyalty-points-app/pkg/users"

type LoginClaim struct {
	UserID         string     `json:"userID"`
	ExpirationTime int64      `json:"expirationTime"`
	LastName       string     `json:"lastName"`
	Email          string     `json:"email"`
	Role           users.Role `json:"role"`

	Permissions []string `json:"permissions"`

	// TokenVersion is the user's session epoch at issue time; a token is rejected
	// once it no longer matches the user's current token_version, which is how
	// logout revokes every outstanding token at once.
	TokenVersion int64 `json:"tokenVersion"`
}
