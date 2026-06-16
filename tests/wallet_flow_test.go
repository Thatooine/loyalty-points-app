package tests

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
)

// Wallet JSON-RPC methods, by the exact "<ServiceName>.<Method>" string the
// client sends. ListTransactions/GetByID are not exposed over the wire; balance
// is observed through the account read endpoint instead.
const (
	processTransactionMethod      = "Wallet.ProcessTransaction"
	processTransactionBatchMethod = "Wallet.ProcessTransactionBatch"
	earnPointsMethod              = "Wallet.EarnPoints"
	spendPointsMethod             = "Wallet.SpendPoints"
	getBalanceMethod              = "Account.GetAccountBalance"
)

// walletResult mirrors wallet.ProcessTransactionJSONRPCResponse — the shape
// returned by ProcessTransaction, EarnPoints and SpendPoints alike.
type walletResult struct {
	Ref       string `json:"ref"`
	AccountID string `json:"account_id"`
	Kind      string `json:"kind"`
	Points    int64  `json:"points"`
	Balance   int64  `json:"balance"`
	Duplicate bool   `json:"duplicate"`
}

// balanceResult mirrors accounts.BalanceResult.
type balanceResult struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
}

// batchResult mirrors wallet.ProcessTransactionBatchResult.
type batchResult struct {
	Results []struct {
		Ref     string `json:"ref"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
		Balance int64  `json:"balance"`
	} `json:"results"`
	Summary struct {
		Accepted  int `json:"accepted"`
		Duplicate int `json:"duplicate"`
		Rejected  int `json:"rejected"`
	} `json:"summary"`
}

// uniqueRef returns a ref unlikely to collide with a previous run. The server is
// persistent and ref carries a global UNIQUE constraint, so every test must mint
// its own or a rerun would dedupe against the prior run's rows.
func uniqueRef(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate unique ref: %v", err)
	}
	return "tx-" + hex.EncodeToString(buf)
}

// registerMember onboards a fresh member and returns their token, user id and
// the wallet account opened for them at registration.
func registerMember(t *testing.T, c *apiClient) registerResult {
	t.Helper()
	var reg registerResult
	resp := c.call(t, registerMethod, registerParams(uniqueEmail(t)), "")
	requireNoError(t, "Register", resp)
	mustUnmarshal(t, resp.Result, &reg)
	if reg.Token == "" || reg.AccountID == "" || reg.UserID == "" {
		t.Fatalf("Register returned incomplete identity: %+v", reg)
	}
	return reg
}

// remoteBalance reads the account balance through the Account.GetAccountBalance
// endpoint — proving the wallet write is observable through a separate read path.
func remoteBalance(t *testing.T, c *apiClient, token, accountID string) int64 {
	t.Helper()
	resp := c.call(t, getBalanceMethod, map[string]any{"account_id": accountID}, token)
	requireNoError(t, "GetAccountBalance", resp)
	var bal balanceResult
	mustUnmarshal(t, resp.Result, &bal)
	return bal.Balance
}

// dbBalance reads accounts.balance directly. Skips the assertion when no DB path
// was provided.
func dbBalance(t *testing.T, c *apiClient, accountID string) (int64, bool) {
	t.Helper()
	if c.db == nil {
		return 0, false
	}
	var balance int64
	if err := c.db.QueryRow("SELECT balance FROM accounts WHERE id = $1", accountID).Scan(&balance); err != nil {
		t.Fatalf("query account balance: %v", err)
	}
	return balance, true
}

// TestEarnPointsEndpoint credits a member's account through the EarnPoints
// endpoint and verifies the outcome three ways: the RPC response, a second read
// through GetAccountBalance, and the persisted ledger + account rows. Crediting
// is operator-only, so the earn is performed by an admin against the member's
// account; the member observes the result through their own read path.
func TestEarnPointsEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	admin, adminToken := registerAdmin(t, c)
	ref := uniqueRef(t)

	resp := c.call(t, earnPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": member.AccountID,
		"points":     150,
	}, adminToken)
	requireNoError(t, "EarnPoints", resp)

	var earn walletResult
	mustUnmarshal(t, resp.Result, &earn)
	if earn.Duplicate {
		t.Error("EarnPoints: Duplicate = true, want false")
	}
	if earn.Kind != "earn" {
		t.Errorf("EarnPoints: kind = %q, want \"earn\"", earn.Kind)
	}
	if earn.Points != 150 {
		t.Errorf("EarnPoints: points = %d, want 150", earn.Points)
	}
	if earn.Balance != 150 {
		t.Errorf("EarnPoints: balance = %d, want 150", earn.Balance)
	}

	// Cross-endpoint: the balance is observable through the account read path.
	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 150 {
		t.Errorf("GetAccountBalance = %d, want 150", got)
	}

	// Direct persistence: the ledger row landed with the signed delta, the right
	// kind, ownership stamped to the account owner (the member), and authorship
	// stamped to the acting admin.
	if c.db == nil {
		t.Log("LOYALTY_DB_DSN not set; skipping direct DB assertions")
		return
	}
	var (
		points              int64
		kind, owner, author string
	)
	row := c.db.QueryRow("SELECT points, kind, owner_id, created_by FROM transactions WHERE ref = $1", ref)
	if err := row.Scan(&points, &kind, &owner, &author); err != nil {
		t.Fatalf("query transaction row: %v", err)
	}
	if points != 150 {
		t.Errorf("persisted points = %d, want 150", points)
	}
	if kind != "earn" {
		t.Errorf("persisted kind = %q, want \"earn\"", kind)
	}
	if owner != member.UserID {
		t.Errorf("persisted owner_id = %q, want %q", owner, member.UserID)
	}
	if author != admin.UserID {
		t.Errorf("persisted created_by = %q, want %q", author, admin.UserID)
	}
	if bal, ok := dbBalance(t, c, member.AccountID); ok && bal != 150 {
		t.Errorf("persisted account balance = %d, want 150", bal)
	}
}

// TestSpendPointsEndpoint earns then spends and confirms the debit applies and
// the ledger sum equals the materialised balance.
func TestSpendPointsEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	// Crediting is operator-only, so an admin seeds the starting balance the
	// member then spends from.
	earn := c.call(t, earnPointsMethod, map[string]any{
		"ref":        uniqueRef(t),
		"account_id": member.AccountID,
		"points":     200,
	}, adminToken)
	requireNoError(t, "EarnPoints", earn)

	spendRef := uniqueRef(t)
	resp := c.call(t, spendPointsMethod, map[string]any{
		"ref":        spendRef,
		"account_id": member.AccountID,
		"points":     50,
	}, member.Token)
	requireNoError(t, "SpendPoints", resp)

	var spend walletResult
	mustUnmarshal(t, resp.Result, &spend)
	if spend.Kind != "spend" {
		t.Errorf("SpendPoints: kind = %q, want \"spend\"", spend.Kind)
	}
	// Spend is recorded as a signed debit.
	if spend.Points != -50 {
		t.Errorf("SpendPoints: points = %d, want -50", spend.Points)
	}
	if spend.Balance != 150 {
		t.Errorf("SpendPoints: balance = %d, want 150", spend.Balance)
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 150 {
		t.Errorf("GetAccountBalance after spend = %d, want 150", got)
	}

	if c.db == nil {
		t.Log("LOYALTY_DB_DSN not set; skipping direct DB assertions")
		return
	}
	// Invariant: the materialised balance equals the sum of the ledger.
	var ledgerSum int64
	if err := c.db.QueryRow("SELECT COALESCE(SUM(points), 0) FROM transactions WHERE account_id = $1", member.AccountID).Scan(&ledgerSum); err != nil {
		t.Fatalf("query ledger sum: %v", err)
	}
	if ledgerSum != 150 {
		t.Errorf("ledger sum = %d, want 150", ledgerSum)
	}
	if bal, ok := dbBalance(t, c, member.AccountID); ok && bal != ledgerSum {
		t.Errorf("account balance %d != ledger sum %d (must be equal)", bal, ledgerSum)
	}
}

// TestSpendOverdraftRejected confirms the balance floor is enforced over the
// wire: a spend that would overdraw fails, the balance is untouched, and the
// rejected ledger insert rolled back so its ref is free again.
func TestSpendOverdraftRejected(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c) // opens with balance 0
	ref := uniqueRef(t)

	resp := c.call(t, spendPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": member.AccountID,
		"points":     50,
	}, member.Token)
	if resp.Error == nil {
		t.Fatal("SpendPoints over balance: expected an error, got none")
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 0 {
		t.Errorf("balance after rejected spend = %d, want 0", got)
	}

	if c.db == nil {
		t.Log("LOYALTY_DB_DSN not set; skipping direct DB assertions")
		return
	}
	// The rejected attempt left no ledger row — the insert rolled back.
	var n int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE ref = $1", ref).Scan(&n); err != nil {
		t.Fatalf("count transaction rows: %v", err)
	}
	if n != 0 {
		t.Errorf("rejected ledger rows for %q = %d, want 0", ref, n)
	}
	if bal, ok := dbBalance(t, c, member.AccountID); ok && bal != 0 {
		t.Errorf("persisted balance = %d, want 0", bal)
	}
}

// TestProcessTransactionDuplicateRef confirms idempotency through the generic
// ProcessTransaction endpoint: resubmitting a ref returns the original outcome
// flagged Duplicate, never double-counts, and persists a single ledger row.
func TestProcessTransactionDuplicateRef(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)
	ref := uniqueRef(t)

	// ProcessTransaction is operator-only; the admin credits the member's
	// account and re-submits the same ref.
	params := map[string]any{
		"ref":        ref,
		"account_id": member.AccountID,
		"kind":       "earn",
		"points":     100,
	}

	first := c.call(t, processTransactionMethod, params, adminToken)
	requireNoError(t, "ProcessTransaction", first)
	var firstRes walletResult
	mustUnmarshal(t, first.Result, &firstRes)
	if firstRes.Duplicate {
		t.Fatal("first ProcessTransaction: Duplicate = true, want false")
	}

	second := c.call(t, processTransactionMethod, params, adminToken)
	requireNoError(t, "ProcessTransaction (resubmit)", second)
	var secondRes walletResult
	mustUnmarshal(t, second.Result, &secondRes)
	if !secondRes.Duplicate {
		t.Error("resubmit: Duplicate = false, want true")
	}
	if secondRes.Balance != firstRes.Balance {
		t.Errorf("balance changed on duplicate: %d -> %d", firstRes.Balance, secondRes.Balance)
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != firstRes.Balance {
		t.Errorf("GetAccountBalance after duplicate = %d, want %d", got, firstRes.Balance)
	}

	if c.db == nil {
		t.Log("LOYALTY_DB_DSN not set; skipping direct DB assertions")
		return
	}
	var n int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE ref = $1", ref).Scan(&n); err != nil {
		t.Fatalf("count transaction rows: %v", err)
	}
	if n != 1 {
		t.Errorf("ledger rows for duplicated ref %q = %d, want 1", ref, n)
	}
}

// TestProcessTransactionUnauthenticated confirms the method gate rejects a
// wallet write with no token before it reaches the service.
func TestProcessTransactionUnauthenticated(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	resp := c.call(t, processTransactionMethod, map[string]any{
		"ref":        uniqueRef(t),
		"account_id": member.AccountID,
		"kind":       "earn",
		"points":     100,
	}, "") // no Bearer token
	if resp.Error == nil {
		t.Fatal("ProcessTransaction without token: expected an error, got none")
	}
}

// TestSpendForeignAccountRejected confirms ownership scoping over the wire: a
// member cannot transact against an account they do not own — the scoped lookup
// makes it indistinguishable from a missing account. SpendPoints is the
// member-callable transact method, so ownership is exercised through it (earn
// and the generic ProcessTransaction are operator-only).
func TestSpendForeignAccountRejected(t *testing.T) {
	c := setup(t)
	owner := registerMember(t, c)
	intruder := registerMember(t, c)
	ref := uniqueRef(t)

	resp := c.call(t, spendPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": owner.AccountID, // not the intruder's account
		"points":     100,
	}, intruder.Token)
	if resp.Error == nil {
		t.Fatal("SpendPoints against a foreign account: expected an error, got none")
	}

	// The owner's balance is untouched.
	if got := remoteBalance(t, c, owner.Token, owner.AccountID); got != 0 {
		t.Errorf("owner balance after foreign attempt = %d, want 0", got)
	}

	if c.db == nil {
		return
	}
	var n int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE ref = $1", ref).Scan(&n); err != nil {
		t.Fatalf("count transaction rows: %v", err)
	}
	if n != 0 {
		t.Errorf("ledger rows for rejected foreign attempt = %d, want 0", n)
	}
}

// TestProcessTransactionBatchMemberForbidden confirms batch ingestion is
// admin-only: a member token is rejected at the endpoint.
func TestProcessTransactionBatchMemberForbidden(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	resp := c.call(t, processTransactionBatchMethod, map[string]any{
		"transactions": []map[string]any{
			{"ref": uniqueRef(t), "account_id": member.AccountID, "kind": "earn", "points": 10},
		},
	}, member.Token)
	if resp.Error == nil {
		t.Fatal("ProcessTransactionBatch as member: expected a forbidden error, got none")
	}
}

// TestEarnPointsMemberForbidden confirms crediting is operator-only: a member
// cannot mint points into their own account through EarnPoints, and the
// rejection leaves no ledger row and an untouched balance.
func TestEarnPointsMemberForbidden(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	ref := uniqueRef(t)

	resp := c.call(t, earnPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": member.AccountID,
		"points":     1_000_000,
	}, member.Token)
	if resp.Error == nil {
		t.Fatal("EarnPoints as member: expected a forbidden error, got none")
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 0 {
		t.Errorf("balance after forbidden earn = %d, want 0", got)
	}

	if c.db == nil {
		return
	}
	var n int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE ref = $1", ref).Scan(&n); err != nil {
		t.Fatalf("count transaction rows: %v", err)
	}
	if n != 0 {
		t.Errorf("ledger rows for forbidden earn = %d, want 0", n)
	}
}

// TestProcessTransactionMemberForbidden confirms the generic transaction method
// is operator-only too: a member cannot reach it to credit their own account by
// passing kind=earn.
func TestProcessTransactionMemberForbidden(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	ref := uniqueRef(t)

	resp := c.call(t, processTransactionMethod, map[string]any{
		"ref":        ref,
		"account_id": member.AccountID,
		"kind":       "earn",
		"points":     1_000_000,
	}, member.Token)
	if resp.Error == nil {
		t.Fatal("ProcessTransaction as member: expected a forbidden error, got none")
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 0 {
		t.Errorf("balance after forbidden transaction = %d, want 0", got)
	}
}
