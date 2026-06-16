package authentication

import (
	"context"
	"testing"

	internalUsers "github.com/Thatooine/loyalty-points-app/internal/pkg/users"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// TestLogout_Success confirms Logout bumps the user's token_version through the
// repository and returns the new value, keyed to the requested user.
func TestLogout_Success(t *testing.T) {
	ctx := context.Background()

	userRepo := &internalUsers.MockUserRepository{
		T: t,
		IncrementTokenVersionFunc: func(t *testing.T, m *internalUsers.MockUserRepository, ctx context.Context, request pkgUsers.IncrementTokenVersionRequest) (*pkgUsers.IncrementTokenVersionResponse, error) {
			if request.UserID != "user-1" {
				t.Errorf("IncrementTokenVersion UserID = %q, want user-1", request.UserID)
			}
			return &pkgUsers.IncrementTokenVersionResponse{TokenVersion: 7}, nil
		},
	}
	service := NewLogoutServiceImpl(userRepo)

	resp, err := service.Logout(ctx, pkgAuth.LogoutRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if resp.TokenVersion != 7 {
		t.Errorf("TokenVersion = %d, want 7", resp.TokenVersion)
	}
}

// TestLogout_ValidationError confirms an empty UserID is rejected before any
// repository call.
func TestLogout_ValidationError(t *testing.T) {
	ctx := context.Background()

	userRepo := &internalUsers.MockUserRepository{
		T: t,
		IncrementTokenVersionFunc: func(t *testing.T, m *internalUsers.MockUserRepository, ctx context.Context, request pkgUsers.IncrementTokenVersionRequest) (*pkgUsers.IncrementTokenVersionResponse, error) {
			t.Fatal("IncrementTokenVersion must not be called on a validation failure")
			return nil, nil
		},
	}
	service := NewLogoutServiceImpl(userRepo)

	if _, err := service.Logout(ctx, pkgAuth.LogoutRequest{UserID: ""}); err == nil {
		t.Fatal("Logout() with empty UserID: expected error, got nil")
	}
}

// TestLogout_RepositoryError confirms a repository failure is surfaced.
func TestLogout_RepositoryError(t *testing.T) {
	ctx := context.Background()

	userRepo := &internalUsers.MockUserRepository{
		T: t,
		IncrementTokenVersionFunc: func(t *testing.T, m *internalUsers.MockUserRepository, ctx context.Context, request pkgUsers.IncrementTokenVersionRequest) (*pkgUsers.IncrementTokenVersionResponse, error) {
			return nil, errs.ErrNotFound
		},
	}
	service := NewLogoutServiceImpl(userRepo)

	if _, err := service.Logout(ctx, pkgAuth.LogoutRequest{UserID: "ghost"}); err == nil {
		t.Fatal("Logout() with repository error: expected error, got nil")
	}
}
