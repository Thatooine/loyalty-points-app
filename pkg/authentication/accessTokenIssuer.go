package authentication

import "context"

// AccessTokenIssuer issues signed access tokens from login claims. It is the
// capability the login and registration flows depend on.
type AccessTokenIssuer interface {
	IssueAccessToken(ctx context.Context, request IssueAccessTokenRequest) (*IssueAccessTokenResponse, error)
}

// IssueAccessTokenRequest is the request for IssueAccessToken.
type IssueAccessTokenRequest struct {
	LoginClaim LoginClaim
}

// IssueAccessTokenResponse is the response for IssueAccessToken.
type IssueAccessTokenResponse struct {
	AccessToken string
}
