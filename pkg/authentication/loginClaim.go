package authentication

import "github.com/Thatooine/loyalty-points-app/pkg/users"

// LoginClaim is the set of claims embedded in a signed access token,
// identifying the authenticated user and the token's expiration time.
type LoginClaim struct {
	UserID         string     `json:"userID"`
	ExpirationTime int64      `json:"expirationTime"`
	LastName       string     `json:"lastName"`
	Email          string     `json:"email"`
	Role           users.Role `json:"role"`

	// Permissions are the access-control permissions granted to the user,
	// resolved from their role at token-issue time and carried in the token so
	// the authorization middleware can gate methods without a fresh lookup.
	Permissions []string `json:"permissions"`
}
