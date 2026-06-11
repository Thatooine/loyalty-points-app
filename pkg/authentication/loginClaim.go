package authentication

// LoginClaim is the set of claims embedded in a signed access token,
// identifying the authenticated user and the token's expiration time.
type LoginClaim struct {
	UserID         string `json:"userID"`
	ExpirationTime int64  `json:"expirationTime"`
	LastName       string `json:"lastName"`
	Email          string `json:"email"`
}
