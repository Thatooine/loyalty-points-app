package authentication

import "context"

type AccessTokenIssuer interface {
	IssueAccessToken(ctx context.Context, request IssueAccessTokenRequest) (*IssueAccessTokenResponse, error)
}

type IssueAccessTokenRequest struct {
	LoginClaim LoginClaim
}

type IssueAccessTokenResponse struct {
	AccessToken string
}
