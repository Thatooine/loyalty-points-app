package audits

import "testing"

func TestCreateAuditEntryRequest_Validate(t *testing.T) {
	valid := func() CreateAuditEntryRequest {
		return CreateAuditEntryRequest{AuditEntry: AuditEntry{
			Outcome: OutcomeAccepted, Reason: "ok", UserID: "user-1",
		}}
	}

	tests := []struct {
		name    string
		mutate  func(*CreateAuditEntryRequest)
		wantErr bool
	}{
		{"valid", func(r *CreateAuditEntryRequest) {}, false},
		{"missing reason", func(r *CreateAuditEntryRequest) { r.AuditEntry.Reason = "" }, true},
		{"missing user", func(r *CreateAuditEntryRequest) { r.AuditEntry.UserID = "" }, true},
		{"unknown outcome", func(r *CreateAuditEntryRequest) { r.AuditEntry.Outcome = "pending" }, true},
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

func TestGetAuditEntryByIDRequest_Validate(t *testing.T) {
	if err := (&GetAuditEntryByIDRequest{ID: 1, UserID: "user-1"}).Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	if err := (&GetAuditEntryByIDRequest{ID: 0, UserID: "user-1"}).Validate(); err == nil {
		t.Fatalf("zero ID: want error")
	}
	if err := (&GetAuditEntryByIDRequest{ID: 1}).Validate(); err == nil {
		t.Fatalf("missing UserID: want error")
	}
}
