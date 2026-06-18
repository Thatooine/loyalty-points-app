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

// DefaultPolicy is the policy wired into the server. Crediting is operator-only:
// ProcessTransaction and EarnPoints require the all-scoped transact permission
// (admins only) so a member cannot mint points into their own account; only
// SpendPoints is reachable with the own-scoped transact permission.
func DefaultPolicy() *Policy {
	return NewPolicy(
		map[string][]string{
			processTransactionMethod:      {PermWalletTransactAll},
			earnPointsMethod:              {PermWalletTransactAll},
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
