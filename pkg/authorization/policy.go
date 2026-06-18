package authorization

// loginMethod and registerMethod are public: a caller has no token before
// authenticating or signing up.
const loginMethod = "EmailPasswordAuthenticator.Login"
const registerMethod = "UserRegistrationService.Register"

const (
	processTransactionMethod      = "Wallet.ProcessTransaction"
	processTransactionBatchMethod = "Wallet.ProcessTransactionBatch"
	earnPointsMethod              = "Wallet.EarnPoints"
	spendPointsMethod             = "Wallet.SpendPoints"
	fetchMyAccountsMethod         = "AccountService.FetchMyAccounts"
	getAccountMethod              = "AccountService.GetAccountByID"
	getAccountBalanceMethod       = "AccountService.GetAccountBalance"
	updateAccountNameMethod       = "AccountService.UpdateAccountName"
	updateAccountBalanceMethod    = "AccountService.UpdateAccountBalance"
	openAccountMethod             = "AccountOpener.OpenAccount"
	logoutMethod                  = "Session.Logout"
	listAuditByRefMethod          = "AuditService.FetchTransactionAuditTrail"
)

// Policy is an all-or-nothing method gate: it maps each method to the
// permissions that may invoke it (a caller is authorized when they hold any
// one) and records which methods are public. It resolves no scope; own-vs-all
// breadth is enforced separately by the data layer via IsGranted.
type Policy struct {
	byMethod map[string][]string
	public   map[string]bool
}

func NewPolicy(byMethod map[string][]string, public map[string]bool) *Policy {
	return &Policy{byMethod: byMethod, public: public}
}

// DefaultPolicy is the policy wired into the server. Members transact on their
// OWN account: EarnPoints and SpendPoints are both reachable with the own-scoped
// transact permission, and the data layer scopes the account to the caller, so a
// member can only earn/spend against accounts they own. The generic
// ProcessTransaction (arbitrary kind) and batch ingestion stay operator-only
// (all-scoped) — bulk and generic crediting remain an admin action.
func DefaultPolicy() *Policy {
	return NewPolicy(
		map[string][]string{
			processTransactionMethod:      {PermWalletTransactAll},
			earnPointsMethod:              {PermWalletTransactOwn, PermWalletTransactAll},
			spendPointsMethod:             {PermWalletTransactOwn, PermWalletTransactAll},
			processTransactionBatchMethod: {PermWalletBatchAll},
			fetchMyAccountsMethod:         {PermAccountReadOwn, PermAccountReadAll},
			getAccountMethod:              {PermAccountReadOwn, PermAccountReadAll},
			getAccountBalanceMethod:       {PermAccountReadOwn, PermAccountReadAll},
			updateAccountNameMethod:       {PermAccountWriteOwn, PermAccountWriteAll},
			updateAccountBalanceMethod:    {PermAccountWriteAll},
			openAccountMethod:             {PermAccountWriteOwn, PermAccountWriteAll},
			logoutMethod:                  {PermAuthLogout},
			listAuditByRefMethod:          {PermAuditReadOwn, PermAuditReadAll},
		},
		map[string]bool{
			loginMethod:    true,
			registerMethod: true,
		},
	)
}

func (p *Policy) IsPublic(method string) bool {
	return p.public[method]
}

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
