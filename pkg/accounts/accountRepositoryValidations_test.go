package accounts

import "testing"

func TestCreateAccountRequest_Validate(t *testing.T) {
	valid := func() CreateAccountRequest {
		return CreateAccountRequest{Account: Account{UserID: "user-1", Name: "Wallet", Balance: 0}}
	}

	tests := []struct {
		name    string
		mutate  func(*CreateAccountRequest)
		wantErr bool
	}{
		{"valid", func(r *CreateAccountRequest) {}, false},
		{"missing userID", func(r *CreateAccountRequest) { r.Account.UserID = "" }, true},
		{"missing name", func(r *CreateAccountRequest) { r.Account.Name = "" }, true},
		{"negative balance", func(r *CreateAccountRequest) { r.Account.Balance = -1 }, true},
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

func TestUpdateAccountBalanceRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request UpdateAccountBalanceRequest
		wantErr bool
	}{
		{"valid credit", UpdateAccountBalanceRequest{AccountID: "acc-1", Delta: 100}, false},
		{"valid debit", UpdateAccountBalanceRequest{AccountID: "acc-1", Delta: -100}, false},
		{"missing account", UpdateAccountBalanceRequest{Delta: 100}, true},
		{"zero delta", UpdateAccountBalanceRequest{AccountID: "acc-1", Delta: 0}, true},
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

func TestGetAccountByIDRequest_Validate(t *testing.T) {
	if err := (&GetAccountByIDRequest{AccountID: "acc-1"}).Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	if err := (&GetAccountByIDRequest{}).Validate(); err == nil {
		t.Fatalf("empty AccountID: want error")
	}
}
