package users

import "testing"

func TestCreateUserRequest_Validate(t *testing.T) {
	valid := func() CreateUserRequest {
		return CreateUserRequest{User: User{Email: "a@example.com", PasswordHash: "hash", Role: RoleMember}}
	}

	tests := []struct {
		name    string
		mutate  func(*CreateUserRequest)
		wantErr bool
	}{
		{"valid member", func(r *CreateUserRequest) {}, false},
		{"valid admin", func(r *CreateUserRequest) { r.User.Role = RoleAdmin }, false},
		{"missing email", func(r *CreateUserRequest) { r.User.Email = "" }, true},
		{"missing hash", func(r *CreateUserRequest) { r.User.PasswordHash = "" }, true},
		{"empty role", func(r *CreateUserRequest) { r.User.Role = "" }, true},
		{"unknown role", func(r *CreateUserRequest) { r.User.Role = "superuser" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid()
			tt.mutate(&req)
			err := req.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestGetUserRequests_Validate(t *testing.T) {
	if err := (&GetUserByIDRequest{ID: "u1"}).Validate(); err != nil {
		t.Fatalf("valid GetByID: %v", err)
	}
	if err := (&GetUserByIDRequest{}).Validate(); err == nil {
		t.Fatalf("empty ID: want error")
	}
	if err := (&GetUserByEmailRequest{Email: "a@example.com"}).Validate(); err != nil {
		t.Fatalf("valid GetByEmail: %v", err)
	}
	if err := (&GetUserByEmailRequest{}).Validate(); err == nil {
		t.Fatalf("empty Email: want error")
	}
}
