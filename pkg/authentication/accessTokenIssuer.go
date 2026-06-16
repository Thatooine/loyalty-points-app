package authentication

import "context"

// AccessTokenIssuer issues signed access tokens from login claims. It is the
// capability the login and registration flows depend on.
type AccessTokenIssuer interface {
	IssueAccessToken(ctx context.Context, request IssueAccessTokenRequest) (*IssueAccessTokenResponse, error)
}

type IssueAccessTokenRequest struct {
	LoginClaim LoginClaim
}

type IssueAccessTokenResponse struct {
	AccessToken string
}
