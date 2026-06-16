package accounts

import "testing"

func TestOpenAccountRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request OpenAccountRequest
		wantErr bool
	}{
		{"valid with name", OpenAccountRequest{UserID: "user-1", Name: "Savings"}, false},
		{"valid blank name (defaulted by service)", OpenAccountRequest{UserID: "user-1"}, false},
		{"missing userID", OpenAccountRequest{Name: "Savings"}, true},
		{"missing userID and name", OpenAccountRequest{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}
