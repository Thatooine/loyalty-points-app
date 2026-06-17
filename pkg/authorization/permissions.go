package authorization

import (
	"context"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

// Permissions are formatted "resource:action[:scope]" where scope is "own" or
// "all"; the scope segment is parsed by pkg/scope and drives ownership
// enforcement downstream.
const (
	PermAccountReadOwn = "account:read:own"
	PermAccountReadAll = "account:read:all"

	PermAccountWriteOwn = "account:write:own"
	// PermAccountWriteAll is a data-scope permission consumed by the account
	// repository; it is not mapped to an RPC method, since balance mutation is
	// reached only through the wallet transaction flow.
	PermAccountWriteAll = "account:write:all"

	PermAuditReadOwn = "audit:read:own"
	PermAuditReadAll = "audit:read:all"

	PermTransactionReadOwn = "transaction:read:own"
	PermTransactionReadAll = "transaction:read:all"

	// For users the owner identity is the row's own id, so "own" means id == caller.
	PermUserReadOwn = "user:read:own"
	PermUserReadAll = "user:read:all"

	// PermWalletTransactOwn does NOT authorize crediting: ProcessTransaction and
	// EarnPoints are operator-only, so the only transact method it unlocks is
	// SpendPoints. A member must not be able to mint points into their own account.
	PermWalletTransactOwn = "wallet:transact:own"
	PermWalletTransactAll = "wallet:transact:all"

	PermWalletBatchAll = "wallet:batch:all"

	// PermAuthLogout is deliberately scope-less: logout carries no own-vs-all
	// data scope, so it is exempt from the all-scoped invariant that applies to
	// admins' data-access permissions.
	PermAuthLogout = "auth:logout"
)

// RolePermissions enumerates every capability explicitly — no wildcard — so a
// role's full reach is auditable in one place.
var RolePermissions = map[users.Role][]string{
	users.RoleMember: {
		PermAccountReadOwn,
		PermAccountWriteOwn,
		PermAuditReadOwn,
		PermTransactionReadOwn,
		PermUserReadOwn,
		PermWalletTransactOwn,
		PermAuthLogout,
	},
	users.RoleAdmin: {
		PermAccountReadAll,
		PermAccountWriteAll,
		PermAuditReadAll,
		PermTransactionReadAll,
		PermUserReadAll,
		PermWalletTransactAll,
		PermWalletBatchAll,
		PermAuthLogout,
	},
}

func PermissionsForRole(role users.Role) []string {
	return RolePermissions[role]
}

// IsGranted reports whether the caller's login claim (read from ctx) holds the
// given permission. It drives ownership enforcement: a read scopes to the
// caller's own rows unless they hold the matching ":all" permission.
func IsGranted(ctx context.Context, permission string) bool {
	claim, ok := authentication.LoginClaimFromContext(ctx)
	if !ok {
		return false
	}
	for _, p := range claim.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}
