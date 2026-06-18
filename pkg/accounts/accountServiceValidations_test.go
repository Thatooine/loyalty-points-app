package accounts

import "testing"

func TestFetchMyAccountsRequest_Validate(t *testing.T) {
	if err := (&FetchMyAccountsRequest{UserID: "user-1"}).Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	if err := (&FetchMyAccountsRequest{}).Validate(); err == nil {
		t.Fatalf("missing UserID: want error")
	}
}

func TestReadAccountRequest_Validate(t *testing.T) {
	if err := (&ReadAccountRequest{AccountID: "acc-1", UserID: "user-1"}).Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	if err := (&ReadAccountRequest{UserID: "user-1"}).Validate(); err == nil {
		t.Fatalf("missing AccountID: want error")
	}
	if err := (&ReadAccountRequest{AccountID: "acc-1"}).Validate(); err == nil {
		t.Fatalf("missing UserID: want error")
	}
}

func TestReadAccountBalanceRequest_Validate(t *testing.T) {
	if err := (&ReadAccountBalanceRequest{AccountID: "acc-1", UserID: "user-1"}).Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	if err := (&ReadAccountBalanceRequest{UserID: "user-1"}).Validate(); err == nil {
		t.Fatalf("missing AccountID: want error")
	}
	if err := (&ReadAccountBalanceRequest{AccountID: "acc-1"}).Validate(); err == nil {
		t.Fatalf("missing UserID: want error")
	}
}

func TestRenameAccountRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request RenameAccountRequest
		wantErr bool
	}{
		{"valid", RenameAccountRequest{AccountID: "acc-1", Name: "Wallet", UserID: "user-1"}, false},
		{"missing account", RenameAccountRequest{Name: "Wallet", UserID: "user-1"}, true},
		{"missing name", RenameAccountRequest{AccountID: "acc-1", UserID: "user-1"}, true},
		{"missing userID", RenameAccountRequest{AccountID: "acc-1", Name: "Wallet"}, true},
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

func TestAdjustAccountBalanceRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request AdjustAccountBalanceRequest
		wantErr bool
	}{
		{"valid credit", AdjustAccountBalanceRequest{AccountID: "acc-1", Delta: 100, UserID: "user-1"}, false},
		{"valid debit", AdjustAccountBalanceRequest{AccountID: "acc-1", Delta: -100, UserID: "user-1"}, false},
		{"missing account", AdjustAccountBalanceRequest{Delta: 100, UserID: "user-1"}, true},
		{"zero delta", AdjustAccountBalanceRequest{AccountID: "acc-1", Delta: 0, UserID: "user-1"}, true},
		{"missing userID", AdjustAccountBalanceRequest{AccountID: "acc-1", Delta: 100}, true},
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
