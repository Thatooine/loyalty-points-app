package authorization

import "github.com/Thatooine/loyalty-points-app/pkg/users"

// wildcard, when present in a role's method set, grants that role every method.
const wildcard = "*"

// loginMethod is the JSON-RPC method that authenticates a caller and hands back
// a token; it must be callable without one, so it is public.
const loginMethod = "EmailPasswordAuthenticator.Login"

// registerMethod onboards a new user and issues their first token; a caller has
// no token before signing up, so it is public.
const registerMethod = "UserRegistrationService.Register"

// Protected business methods a member may call. The wallet method enforces
// account ownership in the service layer (admins bypass it); the account reads
// are ownership-scoped in the repository.
const (
	processTransactionMethod = "Wallet.ProcessTransaction"
	getAccountMethod         = "Account.GetByID"
	getAccountBalanceMethod  = "Account.GetAccountBalance"
)

// Permissions answers "may this caller invoke this JSON-RPC method?". Methods
// are identified by the exact "<ServiceName>.<Method>" string the JSON-RPC
// client sends, where ServiceName is the value returned by the adaptor's
// Name(). Public methods need no token; the rest require a role permitted to
// call them.
type Permissions struct {
	byRole map[users.Role]map[string]bool
	public map[string]bool
}

// NewPermissions builds a Permissions from an explicit role→methods map and a
// set of public methods. A role whose set contains the wildcard "*" may call
// any method.
func NewPermissions(byRole map[users.Role]map[string]bool, public map[string]bool) *Permissions {
	return &Permissions{byRole: byRole, public: public}
}

// DefaultPermissions is the permission set wired into the server. Protected
// business methods are added here as their JSON-RPC adaptors are built; the
// member entry below is scaffolding showing the intended shape.
func DefaultPermissions() *Permissions {
	return NewPermissions(
		map[users.Role]map[string]bool{
			// Admins may call every method.
			users.RoleAdmin: {
				wildcard: true,
			},
			// Members may act on their own wallet and read their own accounts.
			// Ownership is enforced beneath these methods, not by the
			// permission map.
			users.RoleMember: {
				processTransactionMethod: true,
				getAccountMethod:         true,
				getAccountBalanceMethod:  true,
			},
		},
		map[string]bool{
			loginMethod:    true,
			registerMethod: true,
		},
	)
}

// IsPublic reports whether the method may be called without authentication.
func (p *Permissions) IsPublic(method string) bool {
	return p.public[method]
}

// Can reports whether the role may call the method.
func (p *Permissions) Can(role users.Role, method string) bool {
	methods, ok := p.byRole[role]
	if !ok {
		return false
	}
	return methods[wildcard] || methods[method]
}
