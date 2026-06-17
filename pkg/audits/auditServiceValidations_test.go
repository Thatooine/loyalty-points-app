package audits

import "testing"

func TestListAuditByRefRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request ListAuditByRefRequest
		wantErr bool
	}{
		{"valid", ListAuditByRefRequest{TransactionRef: "tx-1", UserID: "user-1"}, false},
		{"missing ref", ListAuditByRefRequest{UserID: "user-1"}, true},
		{"missing userID", ListAuditByRefRequest{TransactionRef: "tx-1"}, true},
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
