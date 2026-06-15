package authorization

import (
	"testing"

	"github.com/Thatooine/loyalty-points-app/pkg/scope"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

func TestPolicy_Authorize(t *testing.T) {
	policy := DefaultPolicy()

	tests := []struct {
		name      string
		perms     []string
		method    string
		wantOK    bool
		wantScope scope.Scope
	}{
		{
			name:      "member own-scoped read is allowed with own scope",
			perms:     []string{PermAccountReadOwn},
			method:    getAccountMethod,
			wantOK:    true,
			wantScope: scope.Own,
		},
		{
			name:      "admin all-scoped read is allowed with all scope",
			perms:     []string{PermAccountReadAll},
			method:    getAccountMethod,
			wantOK:    true,
			wantScope: scope.All,
		},
		{
			name:      "holding both own and all yields the broadest scope",
			perms:     []string{PermAccountReadOwn, PermAccountReadAll},
			method:    getAccountMethod,
			wantOK:    true,
			wantScope: scope.All,
		},
		{
			name:   "member cannot run batch ingestion",
			perms:  []string{PermWalletTransactOwn, PermAccountReadOwn},
			method: processTransactionBatchMethod,
			wantOK: false,
		},
		{
			name:   "no permissions denies a known method",
			perms:  nil,
			method: getAccountMethod,
			wantOK: false,
		},
		{
			name:   "unknown method is denied",
			perms:  []string{PermAccountReadAll},
			method: "Nope.AtAll",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotOK := policy.Authorize(tt.perms, tt.method); gotOK != tt.wantOK {
				t.Fatalf("Authorize(%v, %q) = %v, want %v", tt.perms, tt.method, gotOK, tt.wantOK)
			}
			if tt.wantOK {
				if gotScope := policy.EffectiveScope(tt.perms, tt.method); gotScope != tt.wantScope {
					t.Fatalf("EffectiveScope(%v, %q) = %q, want %q", tt.perms, tt.method, gotScope, tt.wantScope)
				}
			}
		})
	}
}

func TestDefaultPolicy_PublicMethods(t *testing.T) {
	policy := DefaultPolicy()
	if !policy.IsPublic(loginMethod) {
		t.Fatalf("%q should be public", loginMethod)
	}
	if policy.IsPublic(getAccountMethod) {
		t.Fatalf("%q should not be public", getAccountMethod)
	}
}

func TestPermissionsForRole(t *testing.T) {
	if got := PermissionsForRole(users.RoleMember); len(got) == 0 {
		t.Fatal("member should be granted permissions")
	}
	if got := PermissionsForRole(users.Role("ghost")); got != nil {
		t.Fatalf("unknown role should have no permissions, got %v", got)
	}
}
