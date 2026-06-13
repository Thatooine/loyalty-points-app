package authentication

import "testing"

func TestEmailPasswordAuthenticatorRequest_Validate(t *testing.T) {
	valid := func() EmailPasswordAuthenticatorRequest {
		return EmailPasswordAuthenticatorRequest{Email: "a@example.com", Password: "s3cretpw!"}
	}

	tests := []struct {
		name    string
		mutate  func(*EmailPasswordAuthenticatorRequest)
		wantErr bool
	}{
		{"valid", func(r *EmailPasswordAuthenticatorRequest) {}, false},
		{"missing email", func(r *EmailPasswordAuthenticatorRequest) { r.Email = "" }, true},
		{"missing password", func(r *EmailPasswordAuthenticatorRequest) { r.Password = "" }, true},
		{"missing both", func(r *EmailPasswordAuthenticatorRequest) { r.Email = ""; r.Password = "" }, true},
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
