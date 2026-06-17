package tests

import (
	"testing"
)

// Account JSON-RPC methods. GetAccountByID and GetAccountBalance are read methods;
// UpdateAccountName is an owner-scoped rename; UpdateAccountBalance is the
// admin-only raw balance adjustment (the ledger-bypassing write path).
const (
	getAccountMethod           = "AccountService.GetAccountByID"
	updateAccountNameMethod    = "AccountService.UpdateAccountName"
	updateAccountBalanceMethod = "AccountService.UpdateAccountBalance"
	openAccountMethod          = "AccountOpener.OpenAccount"
)

// accountResult mirrors accounts.AccountResult.
type accountResult struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	Name    string `json:"name"`
	Balance int64  `json:"balance"`
}

// openAccountResult mirrors accounts.OpenAccountResult.
type openAccountResult struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	OwnerID   string `json:"owner_id"`
	Balance   int64  `json:"balance"`
}

// dbAccountOwner reads accounts.owner_id directly. Returns false when no DB path.
func dbAccountOwner(t *testing.T, c *apiClient, accountID string) (string, bool) {
	t.Helper()
	if c.db == nil {
		return "", false
	}
	var ownerID string
	if err := c.db.QueryRow("SELECT owner_id FROM accounts WHERE id = $1", accountID).Scan(&ownerID); err != nil {
		t.Fatalf("query account owner: %v", err)
	}
	return ownerID, true
}

// registerAdmin onboards a fresh member, promotes them to admin directly in the
// DB, then re-logs in so the new token embeds the admin permission set (the
// claim's permissions are fixed at login time). Requires a DB path.
func registerAdmin(t *testing.T, c *apiClient) (registerResult, string) {
	t.Helper()
	admin := registerMember(t, c)
	if c.db == nil {
		t.Skip("LOYALTY_DB_DSN not set; admin promotion requires direct DB access")
	}
	if _, err := c.db.Exec("UPDATE users SET role = 'admin' WHERE id = $1", admin.UserID); err != nil {
		t.Fatalf("promote user to admin: %v", err)
	}
	resp := c.call(t, loginMethod, map[string]any{"email": admin.Email, "password": testPassword}, "")
	requireNoError(t, "Login (admin)", resp)
	var login loginResult
	mustUnmarshal(t, resp.Result, &login)
	if login.Token == "" {
		t.Fatal("Login (admin): empty token")
	}
	return admin, login.Token
}

// dbAccountName reads accounts.name directly. Returns false when no DB path.
func dbAccountName(t *testing.T, c *apiClient, accountID string) (string, bool) {
	t.Helper()
	if c.db == nil {
		return "", false
	}
	var name string
	if err := c.db.QueryRow("SELECT name FROM accounts WHERE id = $1", accountID).Scan(&name); err != nil {
		t.Fatalf("query account name: %v", err)
	}
	return name, true
}

// TestAccountGetAccountByIDEndpoint confirms a member reads their own account, and that
// the read is ownership-scoped: another user's account id reads as not found.
func TestAccountGetAccountByIDEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	resp := c.call(t, getAccountMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	requireNoError(t, "GetAccountByID", resp)

	var acc accountResult
	mustUnmarshal(t, resp.Result, &acc)
	if acc.ID != member.AccountID {
		t.Errorf("GetAccountByID: id = %q, want %q", acc.ID, member.AccountID)
	}
	if acc.UserID != member.UserID {
		t.Errorf("GetAccountByID: user_id = %q, want %q", acc.UserID, member.UserID)
	}
	if acc.Name != testAccountName {
		t.Errorf("GetAccountByID: name = %q, want %q", acc.Name, testAccountName)
	}
	if acc.Balance != 0 {
		t.Errorf("GetAccountByID: balance = %d, want 0", acc.Balance)
	}

	// Ownership scoping: an intruder cannot read the owner's account.
	intruder := registerMember(t, c)
	foreign := c.call(t, getAccountMethod, map[string]any{"account_id": member.AccountID}, intruder.Token)
	if foreign.Error == nil {
		t.Error("GetAccountByID on a foreign account: expected an error, got none")
	}
}

// TestAccountGetAccountBalanceEndpoint confirms the balance read reflects a prior
// wallet credit and is ownership-scoped.
func TestAccountGetAccountBalanceEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	// fresh account reads zero
	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 0 {
		t.Errorf("initial balance = %d, want 0", got)
	}

	// credit through the wallet (operator-only), then confirm the member's read
	// reflects it
	earn := c.call(t, earnPointsMethod, map[string]any{
		"ref":        uniqueRef(t),
		"account_id": member.AccountID,
		"points":     75,
	}, adminToken)
	requireNoError(t, "EarnPoints", earn)

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 75 {
		t.Errorf("balance after earn = %d, want 75", got)
	}

	// ownership scoping: an intruder cannot read the owner's balance
	intruder := registerMember(t, c)
	foreign := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, intruder.Token)
	if foreign.Error == nil {
		t.Error("GetAccountBalance on a foreign account: expected an error, got none")
	}
}

// TestOpenAccountEndpoint confirms a member opens a second wallet for themselves:
// the new account carries the requested name, is owned by the caller, starts at
// zero, and is immediately readable through the member's own (ownership-scoped)
// GetAccountByID — proving it persisted under their ownership.
func TestOpenAccountEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	const name = "Holiday Wallet"

	resp := c.call(t, openAccountMethod, map[string]any{"name": name}, member.Token)
	requireNoError(t, "OpenAccount", resp)

	var opened openAccountResult
	mustUnmarshal(t, resp.Result, &opened)
	if opened.AccountID == "" {
		t.Fatal("OpenAccount: empty account_id")
	}
	if opened.AccountID == member.AccountID {
		t.Errorf("OpenAccount: account_id %q collides with the registration wallet", opened.AccountID)
	}
	if opened.Name != name {
		t.Errorf("OpenAccount: name = %q, want %q", opened.Name, name)
	}
	if opened.OwnerID != member.UserID {
		t.Errorf("OpenAccount: owner_id = %q, want caller %q", opened.OwnerID, member.UserID)
	}
	if opened.Balance != 0 {
		t.Errorf("OpenAccount: balance = %d, want 0", opened.Balance)
	}

	// The owner can read the freshly opened account back through GetAccountByID,
	// confirming it persisted under their ownership scope.
	read := c.call(t, getAccountMethod, map[string]any{"account_id": opened.AccountID}, member.Token)
	requireNoError(t, "GetAccountByID after open", read)
	var acc accountResult
	mustUnmarshal(t, read.Result, &acc)
	if acc.Name != name || acc.UserID != member.UserID || acc.Balance != 0 {
		t.Errorf("GetAccountByID after open = %+v, want name=%q owner=%q balance=0", acc, name, member.UserID)
	}

	if owner, ok := dbAccountOwner(t, c, opened.AccountID); ok && owner != member.UserID {
		t.Errorf("persisted owner_id = %q, want %q", owner, member.UserID)
	}
}

// TestOpenAccountDefaultName confirms a blank name falls back to the service
// default rather than persisting an empty name.
func TestOpenAccountDefaultName(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	resp := c.call(t, openAccountMethod, map[string]any{}, member.Token)
	requireNoError(t, "OpenAccount (default name)", resp)

	var opened openAccountResult
	mustUnmarshal(t, resp.Result, &opened)
	if opened.Name != "Primary Wallet" {
		t.Errorf("OpenAccount default name = %q, want %q", opened.Name, "Primary Wallet")
	}

	if name, ok := dbAccountName(t, c, opened.AccountID); ok && name != "Primary Wallet" {
		t.Errorf("persisted default name = %q, want %q", name, "Primary Wallet")
	}
}

// TestOpenAccountUnauthenticated confirms the method is protected: without a
// token the request is rejected.
func TestOpenAccountUnauthenticated(t *testing.T) {
	c := setup(t)

	resp := c.call(t, openAccountMethod, map[string]any{"name": "No Token"}, "")
	if resp.Error == nil {
		t.Fatal("OpenAccount without a token: expected an error, got none")
	}
}

// TestUpdateAccountNameEndpoint confirms an owner renames their account and the
// change is observable through GetAccountByID and the persisted row.
func TestUpdateAccountNameEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	const newName = "Renamed Wallet"

	resp := c.call(t, updateAccountNameMethod, map[string]any{
		"account_id": member.AccountID,
		"name":       newName,
	}, member.Token)
	requireNoError(t, "UpdateAccountName", resp)

	var acc accountResult
	mustUnmarshal(t, resp.Result, &acc)
	if acc.Name != newName {
		t.Errorf("UpdateAccountName: name = %q, want %q", acc.Name, newName)
	}
	if acc.ID != member.AccountID {
		t.Errorf("UpdateAccountName: id = %q, want %q", acc.ID, member.AccountID)
	}

	// Re-read through GetAccountByID confirms the rename persisted.
	read := c.call(t, getAccountMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	requireNoError(t, "GetAccountByID after rename", read)
	var reread accountResult
	mustUnmarshal(t, read.Result, &reread)
	if reread.Name != newName {
		t.Errorf("GetAccountByID after rename: name = %q, want %q", reread.Name, newName)
	}

	if name, ok := dbAccountName(t, c, member.AccountID); ok && name != newName {
		t.Errorf("persisted account name = %q, want %q", name, newName)
	}
}

// TestUpdateAccountNameForeignRejected confirms ownership scoping on rename: a
// non-owner cannot rename another user's account, and the name is untouched.
func TestUpdateAccountNameForeignRejected(t *testing.T) {
	c := setup(t)
	owner := registerMember(t, c)
	intruder := registerMember(t, c)

	resp := c.call(t, updateAccountNameMethod, map[string]any{
		"account_id": owner.AccountID,
		"name":       "Hijacked",
	}, intruder.Token)
	if resp.Error == nil {
		t.Fatal("UpdateAccountName on a foreign account: expected an error, got none")
	}

	if name, ok := dbAccountName(t, c, owner.AccountID); ok && name != testAccountName {
		t.Errorf("account name after foreign rename attempt = %q, want %q (unchanged)", name, testAccountName)
	}
}

// TestUpdateAccountBalanceAdminEndpoint confirms an admin can apply a raw signed
// delta to any account, that credit and debit both apply, and that the change is
// observable through the owner's balance read and the persisted row.
func TestUpdateAccountBalanceAdminEndpoint(t *testing.T) {
	c := setup(t)
	owner := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	// credit +500
	credit := c.call(t, updateAccountBalanceMethod, map[string]any{
		"account_id": owner.AccountID,
		"delta":      500,
	}, adminToken)
	requireNoError(t, "UpdateAccountBalance (credit)", credit)
	var afterCredit balanceResult
	mustUnmarshal(t, credit.Result, &afterCredit)
	if afterCredit.Balance != 500 {
		t.Errorf("balance after +500 = %d, want 500", afterCredit.Balance)
	}

	// debit -200 -> 300
	debit := c.call(t, updateAccountBalanceMethod, map[string]any{
		"account_id": owner.AccountID,
		"delta":      -200,
	}, adminToken)
	requireNoError(t, "UpdateAccountBalance (debit)", debit)
	var afterDebit balanceResult
	mustUnmarshal(t, debit.Result, &afterDebit)
	if afterDebit.Balance != 300 {
		t.Errorf("balance after -200 = %d, want 300", afterDebit.Balance)
	}

	// the owner observes the admin-applied balance through their own read
	if got := remoteBalance(t, c, owner.Token, owner.AccountID); got != 300 {
		t.Errorf("owner GetAccountBalance = %d, want 300", got)
	}

	if bal, ok := dbBalance(t, c, owner.AccountID); ok && bal != 300 {
		t.Errorf("persisted balance = %d, want 300", bal)
	}
}

// TestUpdateAccountBalanceMemberForbidden confirms the endpoint is admin-only: a
// member token is rejected and the balance is untouched.
func TestUpdateAccountBalanceMemberForbidden(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	resp := c.call(t, updateAccountBalanceMethod, map[string]any{
		"account_id": member.AccountID,
		"delta":      1000,
	}, member.Token)
	if resp.Error == nil {
		t.Fatal("UpdateAccountBalance as member: expected a forbidden error, got none")
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 0 {
		t.Errorf("balance after forbidden adjustment = %d, want 0", got)
	}
}

// TestUpdateAccountBalanceOverdraftRejected confirms the overdraft floor still
// holds on the raw write path: an admin debit below zero is rejected and the
// balance is untouched.
func TestUpdateAccountBalanceOverdraftRejected(t *testing.T) {
	c := setup(t)
	owner := registerMember(t, c) // balance 0
	_, adminToken := registerAdmin(t, c)

	resp := c.call(t, updateAccountBalanceMethod, map[string]any{
		"account_id": owner.AccountID,
		"delta":      -100,
	}, adminToken)
	if resp.Error == nil {
		t.Fatal("UpdateAccountBalance below zero: expected an error, got none")
	}

	if bal, ok := dbBalance(t, c, owner.AccountID); ok && bal != 0 {
		t.Errorf("balance after rejected debit = %d, want 0", bal)
	}
}
