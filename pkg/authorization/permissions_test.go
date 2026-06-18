package authorization

import (
	"context"
	"testing"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
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
			name:   "member can earn on own account",
			perms:  []string{PermWalletTransactOwn},
			method: earnPointsMethod,
			wantOK: true,
		},
		{
			name:   "member can spend on own account",
			perms:  []string{PermWalletTransactOwn},
			method: spendPointsMethod,
			wantOK: true,
		},
		{
			name:   "member cannot use the generic ProcessTransaction",
			perms:  []string{PermWalletTransactOwn},
			method: processTransactionMethod,
			wantOK: false,
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

func TestRolePermissions_AdminIsAllScopedOnly(t *testing.T) {
	for _, perm := range PermissionsForRole(users.RoleAdmin) {
		if s, ok := scope.Of(perm); ok && s != scope.All {
			t.Fatalf("admin holds non-all-scoped permission %q", perm)
		}
	}
}

func TestRolePermissions_AccountWriteGrants(t *testing.T) {
	if !contains(PermissionsForRole(users.RoleMember), PermAccountWriteOwn) {
		t.Fatalf("member should hold %q", PermAccountWriteOwn)
	}
	if !contains(PermissionsForRole(users.RoleAdmin), PermAccountWriteAll) {
		t.Fatalf("admin should hold %q", PermAccountWriteAll)
	}
}

func contains(perms []string, want string) bool {
	for _, p := range perms {
		if p == want {
			return true
		}
	}
	return false
}

func TestIsGranted(t *testing.T) {
	withPerms := func(perms ...string) context.Context {
		return authentication.ContextWithLoginClaim(context.Background(), authentication.LoginClaim{Permissions: perms})
	}

	tests := []struct {
		name       string
		ctx        context.Context
		permission string
		want       bool
	}{
		{"holds the permission", withPerms(PermAccountReadAll), PermAccountReadAll, true},
		{"holds it among others", withPerms(PermAccountReadOwn, PermWalletTransactOwn), PermAccountReadOwn, true},
		{"own does not satisfy all", withPerms(PermAccountReadOwn), PermAccountReadAll, false},
		{"permission not granted", withPerms(PermWalletTransactOwn), PermAccountReadAll, false},
		{"no claim in context", context.Background(), PermAccountReadAll, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGranted(tt.ctx, tt.permission); got != tt.want {
				t.Fatalf("IsGranted(ctx, %q) = %v, want %v", tt.permission, got, tt.want)
			}
		})
	}
}
