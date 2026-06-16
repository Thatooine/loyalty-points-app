package authentication

import "context"

// AccessTokenValidator validates tokens presented by clients. It is the
// capability the authentication/authorization middleware depends on.
type AccessTokenValidator interface {
	ValidateAccessToken(ctx context.Context, request ValidateAccessTokenRequest) (*ValidateAccessTokenResponse, error)
}

type ValidateAccessTokenRequest struct {
	AccessToken string
}

type ValidateAccessTokenResponse struct {
	LoginClaim LoginClaim
}
