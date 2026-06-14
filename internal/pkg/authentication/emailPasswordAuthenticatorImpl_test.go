package authentication

import (
	"context"
	"errors"
	"testing"
	"time"

	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	"github.com/Thatooine/loyalty-points-app/internal/testsupport"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
	"golang.org/x/crypto/bcrypt"
)

// newAuthService wires the real Postgres user repository to a token-service mock
// that echoes the claim's user id as the token.
func newAuthService(t *testing.T) *EmailPasswordAuthenticatorImpl {
	t.Helper()
	ctx := context.Background()

	db := testsupport.NewPostgresDB(t)
	userRepo := internalUsers.NewUserRepositoryImpl(db)

	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt hash error = %v", err)
	}
	if _, err := userRepo.Create(ctx, pkgUsers.CreateUserRequest{
		User: pkgUsers.User{
			ID:           "user-1",
			Email:        "member@example.com",
			PasswordHash: string(hash),
			Role:         pkgUsers.RoleMember,
			CreatedAt:    time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		},
	}); err != nil {
		t.Fatalf("seed user error = %v", err)
	}

	tokenMock := &AccessTokenServiceMock{
		IssueAccessTokenFn: func(_ context.Context, request pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error) {
			return &pkgAuth.IssueAccessTokenResponse{AccessToken: "token-for-" + request.LoginClaim.UserID}, nil
		},
	}

	return NewEmailPasswordAuthenticatorImpl(userRepo, tokenMock)
}

func TestAuthenticate_Success(t *testing.T) {
	ctx := context.Background()
	service := newAuthService(t)

	resp, err := service.Authenticate(ctx, pkgAuth.EmailPasswordAuthenticatorRequest{
		Email:    "member@example.com",
		Password: "correct-horse",
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
	service := newAuthService(t)

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
	service := newAuthService(t)

	_, err := service.Authenticate(ctx, pkgAuth.EmailPasswordAuthenticatorRequest{
		Email:    "ghost@example.com",
		Password: "correct-horse",
	})
	if !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("Authenticate() error = %v, want errs.ErrUnauthorized", err)
	}
}
