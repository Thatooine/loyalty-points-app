package authentication

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/rs/zerolog/log"

	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
)

// AccessTokenServiceImpl issues and validates signed JWT access tokens using
// go-jose.
type AccessTokenServiceImpl struct {
	tokenSigner jose.Signer
	publicKey   *rsa.PublicKey
}

// NewAccessTokenServiceImpl returns a new AccessTokenServiceImpl with the
// provided jose.Signer for signing tokens and RSA public key for verifying
// token signatures.
func NewAccessTokenServiceImpl(tokenSigner jose.Signer, publicKey *rsa.PublicKey) *AccessTokenServiceImpl {
	return &AccessTokenServiceImpl{
		tokenSigner: tokenSigner,
		publicKey:   publicKey,
	}
}

// IssueAccessToken marshals the login claims, signs the payload, and returns
// the compact-serialized JWT as the access token.
func (a *AccessTokenServiceImpl) IssueAccessToken(ctx context.Context, request pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error) {
	// marshal claims to JSON
	claimsPayload, err := json.Marshal(request.LoginClaim)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("could not marshal claims for token")
		return nil, fmt.Errorf("IssueAccessToken failed: could not marshal claims: %w", err)
	}

	// sign the marshalled payload
	signedObj, err := a.tokenSigner.Sign(claimsPayload)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("could not sign payload")
		return nil, fmt.Errorf("IssueAccessToken failed: could not sign payload: %w", err)
	}

	// serialize the signed object into a compact JWT string
	signedJWT, err := signedObj.CompactSerialize()
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("could not serialize signed token")
		return nil, fmt.Errorf("IssueAccessToken failed: could not serialize token: %w", err)
	}

	return &pkgAuth.IssueAccessTokenResponse{
		AccessToken: signedJWT,
	}, nil
}

// ValidateAccessToken parses the compact-serialized JWT, verifies its signature,
// unmarshals the login claims, and checks that the token has not expired.
func (a *AccessTokenServiceImpl) ValidateAccessToken(ctx context.Context, request pkgAuth.ValidateAccessTokenRequest) (*pkgAuth.ValidateAccessTokenResponse, error) {
	// parse the compact-serialized JWS
	signed, err := jose.ParseSigned(request.AccessToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("could not parse access token")
		return nil, fmt.Errorf("ValidateAccessToken failed: could not parse token: %w", err)
	}

	// verify the signature and extract the payload
	payload, err := signed.Verify(a.publicKey)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("could not verify access token signature")
		return nil, fmt.Errorf("ValidateAccessToken failed: could not verify token signature: %w", err)
	}

	// unmarshal the payload into LoginClaim
	var claim pkgAuth.LoginClaim
	if err := json.Unmarshal(payload, &claim); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("could not unmarshal token claims")
		return nil, fmt.Errorf("ValidateAccessToken failed: could not unmarshal claims: %w", err)
	}

	// check token expiration
	if time.Now().Unix() > claim.ExpirationTime {
		log.Ctx(ctx).Warn().Str("userID", claim.UserID).Msg("access token has expired")
		return nil, fmt.Errorf("ValidateAccessToken failed: token has expired")
	}

	return &pkgAuth.ValidateAccessTokenResponse{
		LoginClaim: claim,
	}, nil
}
