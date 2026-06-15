package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Thatooine/loyalty-points-app/internal/testsupport"
	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/authorization"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

func newTestRepository(t *testing.T) *UserRepositoryImpl {
	t.Helper()
	return NewUserRepositoryImpl(testsupport.NewPostgresDB(t))
}

// ctxWithPerms returns a context carrying a login claim with the given
// permissions, mirroring what the authorization middleware places on a request.
func ctxWithPerms(perms ...string) context.Context {
	return authentication.ContextWithLoginClaim(
		context.Background(),
		authentication.LoginClaim{Permissions: perms},
	)
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

	// A caller reading their own record (id == caller) is scoped in.
	got, err := repo.GetByID(ctx, pkgUsers.GetUserByIDRequest{ID: "user-1", UserID: "user-1"})
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

	// The login flow looks users up as the system principal.
	got, err := repo.GetByEmail(ctx, pkgUsers.GetUserByEmailRequest{Email: "user-1@example.com", UserID: pkgUsers.SystemUserID})
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

	_, err := repo.GetByEmail(ctx, pkgUsers.GetUserByEmailRequest{Email: "missing@example.com", UserID: pkgUsers.SystemUserID})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetByEmail() error = %v, want errs.ErrNotFound", err)
	}
}

func TestUserRepositoryImpl_GetByEmailOwnershipScoped(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	want := testUser("owner", "owner@example.com")
	if _, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: want}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// The owner (id == caller) resolves their own record.
	if _, err := repo.GetByEmail(ctx, pkgUsers.GetUserByEmailRequest{Email: "owner@example.com", UserID: "owner"}); err != nil {
		t.Fatalf("own GetByEmail() error = %v", err)
	}

	// A different, non-system caller without user:read:all is scoped out.
	_, err := repo.GetByEmail(ctx, pkgUsers.GetUserByEmailRequest{Email: "owner@example.com", UserID: "intruder"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("scoped GetByEmail() error = %v, want errs.ErrNotFound", err)
	}

	// A user:read:all caller reads any user by email.
	all := ctxWithPerms(authorization.PermUserReadAll)
	if _, err := repo.GetByEmail(all, pkgUsers.GetUserByEmailRequest{Email: "owner@example.com", UserID: "intruder"}); err != nil {
		t.Fatalf("all-scope GetByEmail() error = %v", err)
	}
}

func TestUserRepositoryImpl_GetByIDOwnershipScoped(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepository(t)

	for _, id := range []string{"owner", "other"} {
		if _, err := repo.Create(ctx, pkgUsers.CreateUserRequest{User: testUser(id, id+"@example.com")}); err != nil {
			t.Fatalf("Create(%s) error = %v", id, err)
		}
	}

	// A scoped caller can read their own record...
	if _, err := repo.GetByID(ctx, pkgUsers.GetUserByIDRequest{ID: "owner", UserID: "owner"}); err != nil {
		t.Fatalf("own GetByID() error = %v", err)
	}

	// ...but another user's id reads as ErrNotFound (no existence leak).
	_, err := repo.GetByID(ctx, pkgUsers.GetUserByIDRequest{ID: "other", UserID: "owner"})
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("cross-user GetByID() error = %v, want errs.ErrNotFound", err)
	}

	// With user:read:all the same caller reads any record.
	all := ctxWithPerms(authorization.PermUserReadAll)
	if _, err := repo.GetByID(all, pkgUsers.GetUserByIDRequest{ID: "other", UserID: "owner"}); err != nil {
		t.Fatalf("all-scope GetByID() error = %v", err)
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

	// A user:read:all caller lists every user.
	all := ctxWithPerms(authorization.PermUserReadAll)
	got, err := repo.List(all, pkgUsers.ListUsersRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got.Users) != 2 {
		t.Fatalf("List() returned %d users, want 2", len(got.Users))
	}

	// A scoped caller lists only their own record.
	own, err := repo.List(context.Background(), pkgUsers.ListUsersRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("scoped List() error = %v", err)
	}
	if len(own.Users) != 1 || own.Users[0].ID != "user-1" {
		t.Fatalf("scoped List() = %+v, want only user-1", own.Users)
	}
}
