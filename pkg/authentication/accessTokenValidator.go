package authentication

import "context"

// AccessTokenValidator validates tokens presented by clients. It is the
// capability the authentication/authorization middleware depends on.
type AccessTokenValidator interface {
	ValidateAccessToken(ctx context.Context, request ValidateAccessTokenRequest) (*ValidateAccessTokenResponse, error)
}

// ValidateAccessTokenRequest is the request for ValidateAccessToken.
type ValidateAccessTokenRequest struct {
	AccessToken string
}

// ValidateAccessTokenResponse is the response for ValidateAccessToken.
type ValidateAccessTokenResponse struct {
	LoginClaim LoginClaim
}
