package authorization

import "github.com/Thatooine/loyalty-points-app/pkg/users"

// wildcard, when present in a role's method set, grants that role every method.
const wildcard = "*"

// Permissions answers "may this role call this JSON-RPC method?". Methods are
// identified by the exact "<ServiceName>.<Method>" string the JSON-RPC client
// sends, where ServiceName is the value returned by the adaptor's Name().
type Permissions struct {
	byRole map[users.Role]map[string]bool
}

// NewPermissions builds a Permissions from an explicit role→methods map. A role
// whose set contains the wildcard "*" may call any method.
func NewPermissions(byRole map[users.Role]map[string]bool) *Permissions {
	return &Permissions{byRole: byRole}
}

// DefaultPermissions is the permission set wired into the server. Protected
// business methods are added here as their JSON-RPC adaptors are built; the
// member entries below are scaffolding showing the intended shape. Public
// methods (e.g. login) are not listed because they are served off a router
// that the authorization middleware does not guard.
func DefaultPermissions() *Permissions {
	return NewPermissions(map[users.Role]map[string]bool{
		// Admins may call every method.
		users.RoleAdmin: {
			wildcard: true,
		},
		// Members may act on their own wallet. Extend as adaptors land, e.g.
		// "Wallet.ProcessTransaction", "Account.GetByID".
		users.RoleMember: {},
	})
}

// Can reports whether the role may call the method.
func (p *Permissions) Can(role users.Role, method string) bool {
	methods, ok := p.byRole[role]
	if !ok {
		return false
	}
	return methods[wildcard] || methods[method]
}
