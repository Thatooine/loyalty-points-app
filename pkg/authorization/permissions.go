package authorization

import (
	"context"

	"github.com/Thatooine/loyalty-points-app/pkg/authentication"
	"github.com/Thatooine/loyalty-points-app/pkg/users"
)

// Permissions are the unit of access control. Each is a string formatted
// "resource:action[:scope]" where scope is "own" (the caller's own data) or
// "all" (every owner's data). A method maps to one or more permissions (see
// Policy); a role is granted a fixed set of them (see RolePermissions). The
// scope segment is parsed by pkg/scope and drives ownership enforcement
// downstream.
const (
	// PermAccountReadOwn lets a caller read their own accounts and balances.
	PermAccountReadOwn = "account:read:own"
	// PermAccountReadAll lets a caller read any account or balance.
	PermAccountReadAll = "account:read:all"

	// PermAccountWriteOwn lets a caller mutate the balance of an account they
	// own.
	PermAccountWriteOwn = "account:write:own"
	// PermAccountWriteAll lets a caller mutate the balance of any account. It is
	// a data-scope permission consumed by the account repository to widen
	// ownership enforcement; it is not mapped to an RPC method in Policy, since
	// balance mutation is reached only through the wallet transaction flow.
	PermAccountWriteAll = "account:write:all"

	// PermAuditReadOwn lets a caller read audit entries they own.
	PermAuditReadOwn = "audit:read:own"
	// PermAuditReadAll lets a caller read any owner's audit entries.
	PermAuditReadAll = "audit:read:all"

	// PermTransactionReadOwn lets a caller read transactions they own.
	PermTransactionReadOwn = "transaction:read:own"
	// PermTransactionReadAll lets a caller read any owner's transactions.
	PermTransactionReadAll = "transaction:read:all"

	// PermUserReadOwn lets a caller read their own user record. For users the
	// owner identity is the row's own id, so "own" means id == caller.
	PermUserReadOwn = "user:read:own"
	// PermUserReadAll lets a caller read any user record.
	PermUserReadAll = "user:read:all"

	// PermWalletTransactOwn lets a caller spend points from an account they own.
	// It does NOT authorize crediting: ProcessTransaction and EarnPoints are
	// operator-only (see DefaultPolicy), so the only transact method this
	// permission unlocks is SpendPoints. A member must not be able to mint
	// points into their own account.
	PermWalletTransactOwn = "wallet:transact:own"
	// PermWalletTransactAll lets a caller process any transaction — earn, spend,
	// or the generic ProcessTransaction — against any account. It is an operator
	// capability: crediting points is an admin action, never self-service.
	PermWalletTransactAll = "wallet:transact:all"

	// PermWalletBatchAll lets a caller run batch ingestion across any account;
	// it is an operator capability with no "own" form.
	PermWalletBatchAll = "wallet:batch:all"

	// PermAuthLogout lets a caller revoke their own sessions (log out). Every
	// authenticated user holds it, so the logout method is reachable by anyone
	// with a valid token; the acting user is taken from the token, so it only
	// ever revokes the caller's own tokens. It is deliberately scope-less: logout
	// carries no own-vs-all data scope (it never reads another user's data), so
	// it is exempt from the all-scoped invariant that applies to admin's
	// data-access permissions.
	PermAuthLogout = "auth:logout"
)

// RolePermissions is the fixed set of permissions granted by holding a role.
// Every capability is enumerated explicitly — there is no wildcard — so a
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

// PermissionsForRole returns the permissions granted to a role, or nil for an
// unknown role. The token issuers embed the result in the login claim so the
// caller's permissions travel with their access token.
func PermissionsForRole(role users.Role) []string {
	return RolePermissions[role]
}

// IsGranted reports whether the authenticated caller holds the given
// permission. The caller's login claim — carrying the permissions they were
// granted — is read from ctx, where the authorization middleware placed it. It
// returns false when no claim is present or the permission is not among those
// granted.
//
// The result drives ownership enforcement downstream: a read scopes to the
// caller's own rows unless they hold the matching ":all" permission, e.g.
// PermAccountReadAll.
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
