package authentication

import "context"

// AccessTokenService issues signed access tokens from login claims and
// validates tokens presented by clients.
type AccessTokenService interface {
	IssueAccessToken(ctx context.Context, request IssueAccessTokenRequest) (*IssueAccessTokenResponse, error)
	ValidateAccessToken(ctx context.Context, request ValidateAccessTokenRequest) (*ValidateAccessTokenResponse, error)
}

// IssueAccessTokenRequest is the request for IssueAccessToken.
type IssueAccessTokenRequest struct {
	LoginClaim LoginClaim
}

// IssueAccessTokenResponse is the response for IssueAccessToken.
type IssueAccessTokenResponse struct {
	AccessToken string
}

// ValidateAccessTokenRequest is the request for ValidateAccessToken.
type ValidateAccessTokenRequest struct {
	AccessToken string
}

// ValidateAccessTokenResponse is the response for ValidateAccessToken.
type ValidateAccessTokenResponse struct {
	LoginClaim LoginClaim
}
