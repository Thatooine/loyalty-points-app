package authorization

import (
	"testing"

	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

func TestPermissions_Can(t *testing.T) {
	perms := NewPermissions(map[users.Role]map[string]bool{
		users.RoleAdmin:  {wildcard: true},
		users.RoleMember: {"Wallet.GetByID": true},
	})

	tests := []struct {
		name   string
		role   users.Role
		method string
		want   bool
	}{
		{"admin wildcard allows anything", users.RoleAdmin, "Wallet.ProcessTransaction", true},
		{"member allowed listed method", users.RoleMember, "Wallet.GetByID", true},
		{"member denied unlisted method", users.RoleMember, "Wallet.ProcessTransaction", false},
		{"unknown role denied", users.Role("ghost"), "Wallet.GetByID", false},
		{"empty method denied", users.RoleMember, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perms.Can(tt.role, tt.method); got != tt.want {
				t.Fatalf("Can(%q, %q) = %v, want %v", tt.role, tt.method, got, tt.want)
			}
		})
	}
}

func TestDefaultPermissions_AdminIsPermissive(t *testing.T) {
	perms := DefaultPermissions()
	if !perms.Can(users.RoleAdmin, "Anything.AtAll") {
		t.Fatalf("admin should be able to call any method")
	}
	if perms.Can(users.RoleMember, "Anything.AtAll") {
		t.Fatalf("member should not be able to call an unlisted method")
	}
}
