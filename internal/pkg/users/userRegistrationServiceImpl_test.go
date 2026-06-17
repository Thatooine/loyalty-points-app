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
	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// mockTxManager satisfies sql.TxManager structurally. By default it runs the
// unit-of-work body inline against the other mocks; set RunInTxFunc to override
// (e.g. to assert it is never reached on a fail-closed path).
type mockTxManager struct {
	RunInTxFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (m *mockTxManager) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	if m.RunInTxFunc == nil {
		return fn(ctx)
	}
	return m.RunInTxFunc(ctx, fn)
}

// echoRegistrationToken issues a token that echoes the claim's user id.
func echoRegistrationToken() *internalAuth.AccessTokenServiceMock {
	return &internalAuth.AccessTokenServiceMock{
		IssueAccessTokenFn: func(ctx context.Context, request pkgAuth.IssueAccessTokenRequest) (*pkgAuth.IssueAccessTokenResponse, error) {
			return &pkgAuth.IssueAccessTokenResponse{AccessToken: "test-token-" + request.LoginClaim.UserID}, nil
		},
	}
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

	var createdUser pkgUsers.User
	var createdAccount pkgAccounts.Account
	userRepo := &MockUserRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.CreateUserRequest) (*pkgUsers.CreateUserResponse, error) {
			createdUser = request.User
			return &pkgUsers.CreateUserResponse{User: request.User}, nil
		},
	}
	accountRepo := &internalAccounts.MockAccountRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *internalAccounts.MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
			createdAccount = request.Account
			created := request.Account
			created.ID = "acc-1"
			return &pkgAccounts.CreateAccountResponse{Account: created}, nil
		},
	}
	service := NewUserRegistrationServiceImpl(&mockTxManager{}, userRepo, accountRepo, echoRegistrationToken())

	resp, err := service.Register(ctx, validRegisterRequest())
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if resp.UserID == "" {
		t.Fatal("Register() returned empty UserID")
	}
	if resp.AccountID != "acc-1" {
		t.Errorf("AccountID = %q, want acc-1", resp.AccountID)
	}
	if resp.Token != "test-token-"+resp.UserID {
		t.Errorf("Token = %q, want test-token-%s", resp.Token, resp.UserID)
	}
	if resp.Email != "new@example.com" {
		t.Errorf("Email = %q, want new@example.com", resp.Email)
	}

	if createdUser.Role != pkgUsers.RoleMember {
		t.Errorf("created user Role = %q, want member", createdUser.Role)
	}
	if createdUser.PasswordHash == "s3cretpw!" {
		t.Error("password stored in plaintext")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(createdUser.PasswordHash), []byte("s3cretpw!")); err != nil {
		t.Errorf("stored hash does not match password: %v", err)
	}

	if createdAccount.OwnerID != createdUser.ID {
		t.Errorf("account OwnerID = %q, want new user %q", createdAccount.OwnerID, createdUser.ID)
	}
	if createdAccount.Name != defaultAccountName {
		t.Errorf("account Name = %q, want default %q", createdAccount.Name, defaultAccountName)
	}
	if createdAccount.Balance != 0 {
		t.Errorf("account Balance = %d, want 0", createdAccount.Balance)
	}
}

func TestRegister_CustomAccountName(t *testing.T) {
	ctx := context.Background()

	var createdAccount pkgAccounts.Account
	userRepo := &MockUserRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.CreateUserRequest) (*pkgUsers.CreateUserResponse, error) {
			return &pkgUsers.CreateUserResponse{User: request.User}, nil
		},
	}
	accountRepo := &internalAccounts.MockAccountRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *internalAccounts.MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
			createdAccount = request.Account
			return &pkgAccounts.CreateAccountResponse{Account: request.Account}, nil
		},
	}
	service := NewUserRegistrationServiceImpl(&mockTxManager{}, userRepo, accountRepo, echoRegistrationToken())

	req := validRegisterRequest()
	req.AccountName = "Holiday Points"
	if _, err := service.Register(ctx, req); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if createdAccount.Name != "Holiday Points" {
		t.Errorf("account Name = %q, want Holiday Points", createdAccount.Name)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	ctx := context.Background()

	userRepo := &MockUserRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.CreateUserRequest) (*pkgUsers.CreateUserResponse, error) {
			return nil, errs.ErrAlreadyExists
		},
	}
	// The account must never be opened once the user insert fails.
	accountRepo := &internalAccounts.MockAccountRepository{
		T: t,
		CreateFunc: func(t *testing.T, m *internalAccounts.MockAccountRepository, ctx context.Context, request pkgAccounts.CreateAccountRequest) (*pkgAccounts.CreateAccountResponse, error) {
			t.Fatal("account Create must not be called when the user insert fails")
			return nil, nil
		},
	}
	service := NewUserRegistrationServiceImpl(&mockTxManager{}, userRepo, accountRepo, echoRegistrationToken())

	_, err := service.Register(ctx, validRegisterRequest())
	if !errors.Is(err, errs.ErrAlreadyExists) {
		t.Fatalf("duplicate Register() error = %v, want errs.ErrAlreadyExists", err)
	}
}

func TestRegister_ValidationFailure(t *testing.T) {
	ctx := context.Background()

	// Fail closed: an invalid request must never open a unit of work.
	txm := &mockTxManager{
		RunInTxFunc: func(ctx context.Context, fn func(context.Context) error) error {
			t.Fatal("RunInTx must not be called when the request is invalid")
			return nil
		},
	}
	service := NewUserRegistrationServiceImpl(txm, &MockUserRepository{T: t}, &internalAccounts.MockAccountRepository{T: t}, echoRegistrationToken())

	req := validRegisterRequest()
	req.Password = "short" // below minPasswordLength

	if _, err := service.Register(ctx, req); err == nil {
		t.Fatal("Register() with short password: error = nil, want error")
	}
}
