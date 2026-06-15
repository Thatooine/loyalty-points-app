package wallet

import "testing"

func validProcessRequest() ProcessTransactionRequest {
	return ProcessTransactionRequest{
		Ref:       "tx-001",
		AccountID: "acc-1",
		Kind:      KindEarn,
		Points:    150,
		UserID:    "user-1",
	}
}

func TestProcessTransactionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ProcessTransactionRequest)
		wantErr bool
	}{
		{"valid earn", func(r *ProcessTransactionRequest) {}, false},
		{"valid spend", func(r *ProcessTransactionRequest) { r.Kind = KindSpend }, false},
		{"missing ref", func(r *ProcessTransactionRequest) { r.Ref = "" }, true},
		{"missing account", func(r *ProcessTransactionRequest) { r.AccountID = "" }, true},
		{"missing user", func(r *ProcessTransactionRequest) { r.UserID = "" }, true},
		{"earn with zero points", func(r *ProcessTransactionRequest) { r.Points = 0 }, true},
		{"earn with negative points", func(r *ProcessTransactionRequest) { r.Points = -5 }, true},
		{"spend with negative points", func(r *ProcessTransactionRequest) { r.Kind = KindSpend; r.Points = -5 }, true},
		{"unknown kind", func(r *ProcessTransactionRequest) { r.Kind = "transfer" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validProcessRequest()
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

func TestCreateTransactionRequest_Validate(t *testing.T) {
	valid := func() CreateTransactionRequest {
		return CreateTransactionRequest{Transaction: Transaction{
			Ref: "tx-001", AccountID: "acc-1", Kind: KindEarn, Points: 150, CreatedBy: "user-1",
		}}
	}

	tests := []struct {
		name    string
		mutate  func(*CreateTransactionRequest)
		wantErr bool
	}{
		{"valid", func(r *CreateTransactionRequest) {}, false},
		{"negative points ok (spend)", func(r *CreateTransactionRequest) { r.Transaction.Kind = KindSpend; r.Transaction.Points = -50 }, false},
		{"missing ref", func(r *CreateTransactionRequest) { r.Transaction.Ref = "" }, true},
		{"missing account", func(r *CreateTransactionRequest) { r.Transaction.AccountID = "" }, true},
		{"missing createdBy", func(r *CreateTransactionRequest) { r.Transaction.CreatedBy = "" }, true},
		{"zero points", func(r *CreateTransactionRequest) { r.Transaction.Points = 0 }, true},
		{"unknown kind", func(r *CreateTransactionRequest) { r.Transaction.Kind = "x" }, true},
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
