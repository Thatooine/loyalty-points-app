package authentication

import (
	"context"
	"errors"
	"testing"
	"time"

	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
	"golang.org/x/crypto/bcrypt"
)

// seededPassword is the plaintext whose bcrypt hash the mock user carries.
const seededPassword = "correct-horse"

// hashFor returns a bcrypt hash of pw at MinCost (fast, for tests only).
func hashFor(t *testing.T, pw string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash error = %v", err)
	}
	return string(hash)
}

// seededUser is the member the GetByEmail mock resolves for member@example.com.
func seededUser(t *testing.T) pkgUsers.User {
	t.Helper()
	return pkgUsers.User{
		ID:           "user-1",
		Email:        "member@example.com",
		PasswordHash: hashFor(t, seededPassword),
		Role:         pkgUsers.RoleMember,
		CreatedAt:    time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
	}
}

// echoTokenMock issues a token that echoes the claim's user id, so the test can
// assert the authenticator built the claim from the resolved user.
func echoTokenMock() *AccessTokenServiceMock {
	return &AccessTokenServiceMock{
		IssueAccessTokenFn: func(_ context.Context, request pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error) {
			return &pkgAuth.IssueAccessTokenResponse{AccessToken: "token-for-" + request.LoginClaim.UserID}, nil
		},
	}
}

// failIfIssued is a token mock that fails the test if a token is ever issued —
// used to prove the authenticator fails closed before token issuance.
func failIfIssued(t *testing.T) *AccessTokenServiceMock {
	t.Helper()
	return &AccessTokenServiceMock{
		IssueAccessTokenFn: func(_ context.Context, _ pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error) {
			t.Fatal("IssueAccessToken must not be called when authentication fails")
			return nil, nil
		},
	}
}

func TestAuthenticate_Success(t *testing.T) {
	ctx := context.Background()
	user := seededUser(t)

	userRepo := &internalUsers.MockUserRepository{
		T: t,
		GetByEmailFunc: func(t *testing.T, m *internalUsers.MockUserRepository, ctx context.Context, request pkgUsers.GetUserByEmailRequest) (*pkgUsers.GetUserByEmailResponse, error) {
			// Login acts as the system principal so the lookup is unscoped.
			if request.UserID != pkgUsers.SystemUserID {
				t.Errorf("GetByEmail UserID = %q, want SystemUserID %q", request.UserID, pkgUsers.SystemUserID)
			}
			if request.Email != "member@example.com" {
				t.Errorf("GetByEmail Email = %q, want member@example.com", request.Email)
			}
			return &pkgUsers.GetUserByEmailResponse{User: user}, nil
		},
	}
	service := NewEmailPasswordAuthenticatorImpl(userRepo, echoTokenMock())

	resp, err := service.Authenticate(ctx, pkgAuth.EmailPasswordAuthenticatorRequest{
		Email:    "member@example.com",
		Password: seededPassword,
	})
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if resp.Token != "token-for-user-1" {
		t.Fatalf("Token = %q, want token-for-user-1", resp.Token)
	}
	if resp.UserID != "user-1" || resp.Email != "member@example.com" {
		t.Fatalf("UserID/Email = %q/%q, want user-1/member@example.com", resp.UserID, resp.Email)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	ctx := context.Background()
	user := seededUser(t)

	userRepo := &internalUsers.MockUserRepository{
		T: t,
		GetByEmailFunc: func(t *testing.T, m *internalUsers.MockUserRepository, ctx context.Context, request pkgUsers.GetUserByEmailRequest) (*pkgUsers.GetUserByEmailResponse, error) {
			return &pkgUsers.GetUserByEmailResponse{User: user}, nil
		},
	}
	// Fail closed: a wrong password must never reach token issuance.
	service := NewEmailPasswordAuthenticatorImpl(userRepo, failIfIssued(t))

	_, err := service.Authenticate(ctx, pkgAuth.EmailPasswordAuthenticatorRequest{
		Email:    "member@example.com",
		Password: "wrong",
	})
	if !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("Authenticate() error = %v, want errs.ErrUnauthorized", err)
	}
}

func TestAuthenticate_UnknownUser(t *testing.T) {
	ctx := context.Background()

	userRepo := &internalUsers.MockUserRepository{
		T: t,
		GetByEmailFunc: func(t *testing.T, m *internalUsers.MockUserRepository, ctx context.Context, request pkgUsers.GetUserByEmailRequest) (*pkgUsers.GetUserByEmailResponse, error) {
			return nil, errs.ErrNotFound
		},
	}
	// Fail closed: an unknown user must never reach token issuance.
	service := NewEmailPasswordAuthenticatorImpl(userRepo, failIfIssued(t))

	_, err := service.Authenticate(ctx, pkgAuth.EmailPasswordAuthenticatorRequest{
		Email:    "ghost@example.com",
		Password: seededPassword,
	})
	if !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("Authenticate() error = %v, want errs.ErrUnauthorized", err)
	}
}
