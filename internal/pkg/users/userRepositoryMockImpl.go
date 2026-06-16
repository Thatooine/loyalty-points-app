package users

import (
	"context"
	"testing"

	pkgUsers "github.com/Thatooine/loyalty-points-app/pkg/users"
)

// Ensure that MockUserRepository implements the UserRepository interface.
var _ pkgUsers.UserRepository = &MockUserRepository{}

// MockUserRepository is a hand-written mock implementation of
// users.UserRepository. Each method delegates to a function field that a test
// sets to control the return value (the happy path) or the error (the failure
// path); an unset field is a no-op returning the zero value, so a test only
// wires the methods it exercises.
type MockUserRepository struct {
	T *testing.T

	CreateFunc                func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.CreateUserRequest) (*pkgUsers.CreateUserResponse, error)
	ListFunc                  func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.ListUsersRequest) (*pkgUsers.ListUsersResponse, error)
	GetByIDFunc               func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.GetUserByIDRequest) (*pkgUsers.GetUserByIDResponse, error)
	GetByEmailFunc            func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.GetUserByEmailRequest) (*pkgUsers.GetUserByEmailResponse, error)
	GetTokenVersionFunc       func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.GetTokenVersionRequest) (*pkgUsers.GetTokenVersionResponse, error)
	IncrementTokenVersionFunc func(t *testing.T, m *MockUserRepository, ctx context.Context, request pkgUsers.IncrementTokenVersionRequest) (*pkgUsers.IncrementTokenVersionResponse, error)
}

func (m *MockUserRepository) Create(ctx context.Context, request pkgUsers.CreateUserRequest) (*pkgUsers.CreateUserResponse, error) {
	if m.CreateFunc == nil {
		return nil, nil
	}
	return m.CreateFunc(m.T, m, ctx, request)
}

func (m *MockUserRepository) List(ctx context.Context, request pkgUsers.ListUsersRequest) (*pkgUsers.ListUsersResponse, error) {
	if m.ListFunc == nil {
		return nil, nil
	}
	return m.ListFunc(m.T, m, ctx, request)
}

func (m *MockUserRepository) GetByID(ctx context.Context, request pkgUsers.GetUserByIDRequest) (*pkgUsers.GetUserByIDResponse, error) {
	if m.GetByIDFunc == nil {
		return nil, nil
	}
	return m.GetByIDFunc(m.T, m, ctx, request)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, request pkgUsers.GetUserByEmailRequest) (*pkgUsers.GetUserByEmailResponse, error) {
	if m.GetByEmailFunc == nil {
		return nil, nil
	}
	return m.GetByEmailFunc(m.T, m, ctx, request)
}

func (m *MockUserRepository) GetTokenVersion(ctx context.Context, request pkgUsers.GetTokenVersionRequest) (*pkgUsers.GetTokenVersionResponse, error) {
	if m.GetTokenVersionFunc == nil {
		return nil, nil
	}
	return m.GetTokenVersionFunc(m.T, m, ctx, request)
}

func (m *MockUserRepository) IncrementTokenVersion(ctx context.Context, request pkgUsers.IncrementTokenVersionRequest) (*pkgUsers.IncrementTokenVersionResponse, error) {
	if m.IncrementTokenVersionFunc == nil {
		return nil, nil
	}
	return m.IncrementTokenVersionFunc(m.T, m, ctx, request)
}
