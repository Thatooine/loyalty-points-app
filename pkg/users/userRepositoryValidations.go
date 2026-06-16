package users

import "github.com/Thatooine/loyalty-points-app/pkg/errs"

func (r *CreateUserRequest) Validate() error {
	var reasons []string

	if r.User.Email == "" {
		reasons = append(reasons, "Email is required")
	}

	if r.User.PasswordHash == "" {
		reasons = append(reasons, "PasswordHash is required")
	}

	switch r.User.Role {
	case RoleMember, RoleAdmin:
	default:
		reasons = append(reasons, "Role must be 'member' or 'admin'")
	}

	return errs.NewValidationError(reasons)
}

func (r *GetUserByIDRequest) Validate() error {
	var reasons []string

	if r.ID == "" {
		reasons = append(reasons, "ID is required")
	}

	return errs.NewValidationError(reasons)
}

func (r *GetUserByEmailRequest) Validate() error {
	var reasons []string

	if r.Email == "" {
		reasons = append(reasons, "Email is required")
	}

	return errs.NewValidationError(reasons)
}

// Validate has no fields to check; defined for a uniform call site.
func (r *ListUsersRequest) Validate() error {
	return nil
}

func (r *GetTokenVersionRequest) Validate() error {
	if r.UserID == "" {
		return errs.NewValidationError([]string{"UserID is required"})
	}
	return nil
}

func (r *IncrementTokenVersionRequest) Validate() error {
	if r.UserID == "" {
		return errs.NewValidationError([]string{"UserID is required"})
	}
	return nil
}
