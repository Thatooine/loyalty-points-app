package authorization

// loginMethod is the JSON-RPC method that authenticates a caller and hands back
// a token; it must be callable without one, so it is public.
const loginMethod = "EmailPasswordAuthenticator.Login"

// registerMethod onboards a new user and issues their first token; a caller has
// no token before signing up, so it is public.
const registerMethod = "UserRegistrationService.Register"

// Protected business methods, identified by the exact "<ServiceName>.<Method>"
// string the JSON-RPC client sends.
const (
	processTransactionMethod      = "Wallet.ProcessTransaction"
	processTransactionBatchMethod = "Wallet.ProcessTransactionBatch"
	earnPointsMethod              = "Wallet.EarnPoints"
	spendPointsMethod             = "Wallet.SpendPoints"
	getAccountMethod              = "Account.GetByID"
	getAccountBalanceMethod       = "Account.GetAccountBalance"
)

// Policy is the access-control entity: it maps each JSON-RPC method to the
// permissions that may invoke it, and records which methods are public. A
// method maps to one or more permissions; a caller is authorized when they hold
// any one of them. This is purely an all-or-nothing method gate — it resolves
// no scope; how broadly the caller may act (own vs all) is enforced separately
// by the data layer via IsGranted.
type Policy struct {
	byMethod map[string][]string
	public   map[string]bool
}

// NewPolicy builds a Policy from a method→permissions map and a set of public
// methods.
func NewPolicy(byMethod map[string][]string, public map[string]bool) *Policy {
	return &Policy{byMethod: byMethod, public: public}
}

// DefaultPolicy is the policy wired into the server. Each protected method
// lists the permissions that satisfy it: read methods accept the own- or
// all-scoped read permission, the single transaction method accepts the own- or
// all-scoped transact permission, and batch ingestion accepts only the operator
// permission.
func DefaultPolicy() *Policy {
	return NewPolicy(
		map[string][]string{
			processTransactionMethod:      {PermWalletTransactOwn, PermWalletTransactAll},
			earnPointsMethod:              {PermWalletTransactOwn, PermWalletTransactAll},
			spendPointsMethod:             {PermWalletTransactOwn, PermWalletTransactAll},
			processTransactionBatchMethod: {PermWalletBatchAll},
			getAccountMethod:              {PermAccountReadOwn, PermAccountReadAll},
			getAccountBalanceMethod:       {PermAccountReadOwn, PermAccountReadAll},
		},
		map[string]bool{
			loginMethod:    true,
			registerMethod: true,
		},
	)
}

// IsPublic reports whether the method may be called without authentication.
func (p *Policy) IsPublic(method string) bool {
	return p.public[method]
}

// Authorize reports whether a caller holding callerPerms may invoke method: it
// is true when the caller holds at least one of the method's permissions. The
// breadth of that access (own vs all) is enforced separately, on demand, by
// checking the caller's claim for a specific permission via IsGranted.
func (p *Policy) Authorize(callerPerms []string, method string) bool {
	required, ok := p.byMethod[method]
	if !ok {
		return false
	}
	held := make(map[string]bool, len(callerPerms))
	for _, perm := range callerPerms {
		held[perm] = true
	}
	for _, perm := range required {
		if held[perm] {
			return true
		}
	}
	return false
}
