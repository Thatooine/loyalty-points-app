package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Thatooine/loyalty-points-app/internal/testsupport"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

func newTestRepository(t *testing.T) *UserRepositoryImpl {
	t.Helper()
	return NewUserRepositoryImpl(testsupport.NewPostgresDB(t))
}

func testUser(id, email string) pkgUsers.User {
	return pkgUsers.User{
		ID:           id,
		Email:        email,
		PasswordHash: "bcrypt-hash",
		Role:         pkgUsers.RoleMember,
		CreatedAt:    time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
	}
}

func TestUserRepositoryImpl_CreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	want := testUser("user-1", "user-1@example.com")
	if _, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: want}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, pkgUsers.GetUserByIDRequest{ID: "user-1"})
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.User != want {
		t.Fatalf("GetByID() = %+v, want %+v", got.User, want)
	}
}

func TestUserRepositoryImpl_CreateDuplicateID(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	if _, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: testUser("user-1", "a@example.com")}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: testUser("user-1", "b@example.com")})
	if !errors.Is(err, errs.ErrAlreadyExists) {
		t.Fatalf("Create() duplicate id error = %v, want errs.ErrAlreadyExists", err)
	}
}

func TestUserRepositoryImpl_CreateDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	if _, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: testUser("user-1", "shared@example.com")}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: testUser("user-2", "shared@example.com")})
	if !errors.Is(err, errs.ErrAlreadyExists) {
		t.Fatalf("Create() duplicate email error = %v, want errs.ErrAlreadyExists", err)
	}
}

func TestUserRepositoryImpl_GetByEmail(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	want := testUser("user-1", "user-1@example.com")
	if _, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: want}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByEmail(ctx, pkgUsers.GetUserByEmailRequest{Email: "user-1@example.com"})
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if got.User != want {
		t.Fatalf("GetByEmail() = %+v, want %+v", got.User, want)
	}
}

func TestUserRepositoryImpl_GetByEmailNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	_, err := repo.GetByEmail(ctx, pkgUsers.GetUserByEmailRequest{Email: "missing@example.com"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetByEmail() error = %v, want errs.ErrNotFound", err)
	}
}

func TestUserRepositoryImpl_GetByIDNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	_, err := repo.GetByID(ctx, pkgUsers.GetUserByIDRequest{ID: "missing"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetByID() error = %v, want errs.ErrNotFound", err)
	}
}

func TestUserRepositoryImpl_List(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	for _, id := range []string{"user-1", "user-2"} {
		if _, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: testUser(id, id+"@example.com")}); err != nil {
			t.Fatalf("Create(%s) error = %v", id, err)
		}
	}

	got, err := repo.List(ctx, pkgUsers.ListUsersRequest{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got.Users) != 2 {
		t.Fatalf("List() returned %d users, want 2", len(got.Users))
	}
}
