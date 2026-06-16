package authentication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pkgAuth "github.com/bash/the-dancing-pony-v2-rnyfbr/pkg/authentication"
	"github.com/bash/the-dancing-pony-v2-rnyfbr/pkg/users"
	"github.com/rs/zerolog/log"
)

type EmailAndPasswordAuthenticatorService struct {
	accessTokenCreator pkgAuth.AccessTokenCreatorService
	userReader         users.UserReaderService
	firebaseAPIKey     string
}

func NewEmailAndPasswordAuthenticatorService(
	accessTokenCreator pkgAuth.AccessTokenCreatorService,
	userReader users.UserReaderService,
	firebaseAPIKey string,
) *EmailAndPasswordAuthenticatorService {
	return &EmailAndPasswordAuthenticatorService{
		accessTokenCreator: accessTokenCreator,
		userReader:         userReader,
		firebaseAPIKey:     firebaseAPIKey,
	}
}

// Firebase sign-in request/response bodies.

type firebaseSignInRequestBody struct {
	Email             string `json:"email"`
	Password          string `json:"password"`
	ReturnSecureToken bool   `json:"returnSecureToken"`
}

type firebaseSignInResponseBody struct {
	IDToken      string `json:"idToken"`
	Email        string `json:"email"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    string `json:"expiresIn"`
	LocalID      string `json:"localId"`
	Registered   bool   `json:"registered"`
}

func (s *EmailAndPasswordAuthenticatorService) AuthenticateWithEmailAndPassword(ctx context.Context, request pkgAuth.EmailAndPasswordAuthRequest) (*pkgAuth.EmailAndPasswordAuthResponse, error) {
	// Verify credentials against Firebase
	firebaseResp, err := s.signInFirebaseViaEmailAndPassword(ctx, request.Email, request.Password)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("firebase sign in failed")
		return nil, fmt.Errorf("AuthenticateWithEmailAndPassword failed: %w", err)
	}

	// Retrieve the user from the database by email
	userResp, err := s.userReader.GetUser(ctx, users.GetUserRequest{Email: firebaseResp.Email})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to retrieve user by email")
		return nil, fmt.Errorf("AuthenticateWithEmailAndPassword failed: %w", err)
	}

	// Build login claim using the database user ID
	expiresIn, err := time.ParseDuration(fmt.Sprintf("%ss", firebaseResp.ExpiresIn))
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("error parsing expiration duration")
		return nil, fmt.Errorf("AuthenticateWithEmailAndPassword failed: error parsing expiry: %w", err)
	}

	loginClaim := pkgAuth.LoginClaim{
		UserID:         userResp.User.ID,
		Email:          userResp.User.Email,
		ExpirationTime: time.Now().Add(expiresIn).Unix(),
	}

	// Issue a signed access token
	tokenResp, err := s.accessTokenCreator.CreateAccessToken(
		ctx,
		pkgAuth.CreateAccessTokenRequest{
			LoginClaim: loginClaim,
		})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to create access token")
		return nil, fmt.Errorf("AuthenticateWithEmailAndPassword failed: %w", err)
	}

	return &pkgAuth.EmailAndPasswordAuthResponse{
		Token:  tokenResp.AccessToken,
		UserID: userResp.User.ID,
		Email:  userResp.User.Email,
	}, nil
}

// signInFirebaseViaEmailAndPassword calls the Firebase Identity Toolkit API to verify
// the user's email and password credentials.
func (s *EmailAndPasswordAuthenticatorService) signInFirebaseViaEmailAndPassword(ctx context.Context, email string, password string) (*firebaseSignInResponseBody, error) {
	// prepare request body
	bodyData, err := json.Marshal(firebaseSignInRequestBody{
		Email:             email,
		Password:          password,
		ReturnSecureToken: true,
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("error marshalling sign in request body")
		return nil, fmt.Errorf("error marshalling sign in request body: %w", err)
	}

	// prepare request
	url := fmt.Sprintf(
		"https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=%s",
		s.firebaseAPIKey,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyData))
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("error constructing sign in request")
		return nil, fmt.Errorf("error constructing sign in request: %w", err)
	}

	// set headers
	req.Header.Set("Content-Type", "application/json")

	// perform request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("error performing sign in request")
		return nil, fmt.Errorf("error performing sign in request: %w", err)
	}
	defer resp.Body.Close()

	// check response code
	if resp.StatusCode != http.StatusOK {
		log.Ctx(ctx).Error().Int("statusCode", resp.StatusCode).Msg("firebase sign in failed")
		return nil, fmt.Errorf("firebase sign in failed with status %d", resp.StatusCode)
	}

	// parse response
	var signInResp firebaseSignInResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&signInResp); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("error unmarshalling sign in response")
		return nil, fmt.Errorf("error unmarshalling sign in response: %w", err)
	}

	return &signInResp, nil
}
