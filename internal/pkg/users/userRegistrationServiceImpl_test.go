package users

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	internalAccounts "github.com/Thatooine/loyalty-points-app/internal/pkg/accounts"
	internalAuth "github.com/Thatooine/loyalty-points-app/internal/pkg/authentication"
	pkgAccounts "github.com/Thatooine/loyalty-points-app/pkg/accounts"
	pkgAuth "github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/errs"
	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// newTestRegistrationService wires the registration service against a real
// migrated Postgres database (shared with the user repo so assertions can read
// it back) and a token-service mock that hands out a fixed token.
func newTestRegistrationService(t *testing.T) (*UserRegistrationServiceImpl, pkgAccounts.AccountRepository) {
	t.Helper()

	userRepo := newTestRepository(t)
	db := userRepo.db
	accountRepo := internalAccounts.NewAccountRepositoryImpl(db)

	tokenMock := &internalAuth.AccessTokenServiceMock{
		IssueAccessTokenFn: func(ctx context.Context, request pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error) {
			return &pkgAuth.IssueAccessTokenResponse{AccessToken: "test-token-" + request.LoginClaim.UserID}, nil
		},
	}

	service := NewUserRegistrationServiceImpl(
		postgres.NewPostgresTxManager(db),
		userRepo,
		accountRepo,
		tokenMock,
	)
	return service, accountRepo
}

func validRegisterRequest() pkgUsers.RegisterRequest {
	return pkgUsers.RegisterRequest{
		Email:    "new@example.com",
		Password: "s3cretpw!",
		Name:     "New User",
	}
}

func TestRegister_HappyPath(t *testing.T) {
	ctx := context.Background()
	service, accountRepo := newTestRegistrationService(t)

	resp, err := service.Register(ctx, validRegisterRequest())
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if resp.UserID == "" || resp.AccountID == "" {
		t.Fatalf("Register() returned empty ids: %+v", resp)
	}
	if resp.Token != "test-token-"+resp.UserID {
		t.Fatalf("Token = %q, want test-token-%s", resp.Token, resp.UserID)
	}
	if resp.Email != "new@example.com" {
		t.Fatalf("Email = %q, want new@example.com", resp.Email)
	}

	// the user is persisted, a member, with a hashed (not plaintext) password
	got, err := service.users.GetByID(ctx, pkgUsers.GetUserByIDRequest{ID: resp.UserID})
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.User.Role != pkgUsers.RoleMember {
		t.Fatalf("Role = %q, want member", got.User.Role)
	}
	if got.User.PasswordHash == "s3cretpw!" {
		t.Fatalf("password stored in plaintext")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.User.PasswordHash), []byte("s3cretpw!")); err != nil {
		t.Fatalf("stored hash does not match password: %v", err)
	}

	// exactly one account was opened, owned by the new user, named by default
	accounts, err := accountRepo.List(ctx, pkgAccounts.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("account List() error = %v", err)
	}
	if len(accounts.Accounts) != 1 {
		t.Fatalf("opened %d accounts, want 1", len(accounts.Accounts))
	}
	account := accounts.Accounts[0]
	if account.ID != resp.AccountID || account.UserID != resp.UserID {
		t.Fatalf("account %+v does not link to user %s", account, resp.UserID)
	}
	if account.Name != defaultAccountName || account.Balance != 0 {
		t.Fatalf("account name=%q balance=%d, want %q / 0", account.Name, account.Balance, defaultAccountName)
	}
}

func TestRegister_CustomAccountName(t *testing.T) {
	ctx := context.Background()
	service, accountRepo := newTestRegistrationService(t)

	req := validRegisterRequest()
	req.AccountName = "Holiday Points"
	if _, err := service.Register(ctx, req); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	accounts, err := accountRepo.List(ctx, pkgAccounts.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("account List() error = %v", err)
	}
	if accounts.Accounts[0].Name != "Holiday Points" {
		t.Fatalf("account name = %q, want Holiday Points", accounts.Accounts[0].Name)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	service, accountRepo := newTestRegistrationService(t)

	if _, err := service.Register(ctx, validRegisterRequest()); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	_, err := service.Register(ctx, validRegisterRequest())
	if !errors.Is(err, errs.ErrAlreadyExists) {
		t.Fatalf("duplicate Register() error = %v, want errs.ErrAlreadyExists", err)
	}

	// the rolled-back second attempt left no orphan account behind
	accounts, err := accountRepo.List(ctx, pkgAccounts.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("account List() error = %v", err)
	}
	if len(accounts.Accounts) != 1 {
		t.Fatalf("after duplicate registration there are %d accounts, want 1", len(accounts.Accounts))
	}
}

func TestRegister_ValidationFailure(t *testing.T) {
	ctx := context.Background()
	service, accountRepo := newTestRegistrationService(t)

	req := validRegisterRequest()
	req.Password = "short" // below minPasswordLength

	if _, err := service.Register(ctx, req); err == nil {
		t.Fatalf("Register() with short password: error = nil, want error")
	}

	// nothing was written
	users, err := service.users.List(ctx, pkgUsers.ListUsersRequest{})
	if err != nil {
		t.Fatalf("user List() error = %v", err)
	}
	if len(users.Users) != 0 {
		t.Fatalf("validation failure wrote %d users, want 0", len(users.Users))
	}
	accounts, err := accountRepo.List(ctx, pkgAccounts.ListAccountsRequest{})
	if err != nil {
		t.Fatalf("account List() error = %v", err)
	}
	if len(accounts.Accounts) != 0 {
		t.Fatalf("validation failure wrote %d accounts, want 0", len(accounts.Accounts))
	}
}
