package tests

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Thatooine/loyalty-points-app/pkg/postgres"
)

const (
	processTransactionMethod      = "Wallet.ProcessTransaction"
	processTransactionBatchMethod = "Wallet.ProcessTransactionBatch"
	earnPointsMethod              = "Wallet.EarnPoints"
	spendPointsMethod             = "Wallet.SpendPoints"
	getBalanceMethod              = "AccountService.GetAccountBalance"
)

// invalidParamsCode is the JSON-RPC code the server returns for a malformed
// request (validation failure, body too large): pkg/jsonrpc.CodeInvalidParams.
const invalidParamsCode = -32602

type walletResult struct {
	Ref       string `json:"ref"`
	AccountID string `json:"account_id"`
	Kind      string `json:"kind"`
	Points    int64  `json:"points"`
	Balance   int64  `json:"balance"`
	Duplicate bool   `json:"duplicate"`
}

type balanceResult struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
}

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

// The server is persistent and ref carries a global UNIQUE constraint, so each
// test must mint a fresh ref or a rerun would dedupe against prior rows.
func uniqueRef(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate unique ref: %v", err)
	}
	return "tx-" + hex.EncodeToString(buf)
}

// TestBalanceDurableAcrossReconnect covers C-4: a committed balance lives in
// Postgres, not process memory. It writes a balance, then re-reads it through a
// brand-new connection pool opened after the write — the database-level analogue
// of a process restart (the spec's allowed "reconnect a fresh pool" form).
func TestBalanceDurableAcrossReconnect(t *testing.T) {
	c := setup(t)
	if c.db == nil {
		t.Skip("database unavailable; durability check needs direct DB access")
	}
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	earn := c.call(t, earnPointsMethod, map[string]any{
		"ref":        uniqueRef(t),
		"account_id": member.AccountID,
		"points":     250,
	}, adminToken)
	requireNoError(t, "EarnPoints", earn)

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 250 {
		t.Fatalf("balance after earn = %d, want 250", got)
	}

	// Reconnect a fresh pool to the same database: the writing connection is gone,
	// so a balance the new pool can still read proves it was durably committed.
	fresh, err := postgres.NewClient(context.Background(), testDBDSN)
	if err != nil {
		t.Fatalf("reconnect fresh pool: %v", err)
	}
	defer fresh.Close()

	var balance int64
	if err := fresh.QueryRow("SELECT balance FROM accounts WHERE id = $1", member.AccountID).Scan(&balance); err != nil {
		t.Fatalf("re-read balance from fresh pool: %v", err)
	}
	if balance != 250 {
		t.Errorf("balance read from fresh pool = %d, want 250 (not durable)", balance)
	}
}

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

func remoteBalance(t *testing.T, c *apiClient, token, accountID string) int64 {
	t.Helper()
	resp := c.call(t, getBalanceMethod, map[string]any{"account_id": accountID}, token)
	requireNoError(t, "GetAccountBalance", resp)
	var bal balanceResult
	mustUnmarshal(t, resp.Result, &bal)
	return bal.Balance
}

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

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 150 {
		t.Errorf("GetAccountBalance = %d, want 150", got)
	}

	// owner_id is the account owner (member); created_by is the acting admin.
	if c.db == nil {
		t.Log("database unavailable; skipping direct DB assertions")
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

func TestSpendPointsEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

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
		t.Log("database unavailable; skipping direct DB assertions")
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

// A rejected spend must roll back its ledger insert so the ref is free again.
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
		t.Log("database unavailable; skipping direct DB assertions")
		return
	}
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

// A duplicate ref must not move the balance and must persist a single row.
func TestProcessTransactionDuplicateRef(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)
	ref := uniqueRef(t)

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
		t.Log("database unavailable; skipping direct DB assertions")
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

// A transact against an unowned account is indistinguishable from a missing one.
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

// The server sorts the batch by occurred_at, so a spend listed before the earn
// that funds it is still accepted; slice order alone would trip the floor.
func TestProcessTransactionBatchServerOrders(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	earnRef := uniqueRef(t)
	spendRef := uniqueRef(t)

	resp := c.call(t, processTransactionBatchMethod, map[string]any{
		"transactions": []map[string]any{
			{"ref": spendRef, "account_id": member.AccountID, "kind": "spend", "points": 100, "occurred_at": "2024-01-01T11:00:00Z"},
			{"ref": earnRef, "account_id": member.AccountID, "kind": "earn", "points": 100, "occurred_at": "2024-01-01T10:00:00Z"},
		},
	}, adminToken)
	requireNoError(t, "ProcessTransactionBatch", resp)

	var batch batchResult
	mustUnmarshal(t, resp.Result, &batch)
	if batch.Summary.Accepted != 2 || batch.Summary.Rejected != 0 {
		t.Fatalf("accepted=%d rejected=%d, want accepted=2 rejected=0 (server should apply earn before spend)", batch.Summary.Accepted, batch.Summary.Rejected)
	}

	if got := remoteBalance(t, c, adminToken, member.AccountID); got != 0 {
		t.Errorf("balance after earn(100) then spend(100) = %d, want 0", got)
	}
}

// A batch larger than the server's cap is rejected wholesale as invalid params,
// before any insert — so the targeted account is left untouched.
func TestProcessTransactionBatchExceedsMaxRejected(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	// One past the cap (maxBatchSize=1000). Validation fires before processing,
	// so these refs are never inserted and cannot collide on a rerun.
	const tooMany = 1001
	txs := make([]map[string]any, 0, tooMany)
	for i := 0; i < tooMany; i++ {
		txs = append(txs, map[string]any{
			"ref": uniqueRef(t), "account_id": member.AccountID, "kind": "earn", "points": 1,
		})
	}

	resp := c.call(t, processTransactionBatchMethod, map[string]any{"transactions": txs}, adminToken)
	if resp.Error == nil {
		t.Fatal("oversize batch: expected an invalid-params error, got none")
	}
	if resp.Error.Code != invalidParamsCode {
		t.Errorf("oversize batch: error code = %d, want %d", resp.Error.Code, invalidParamsCode)
	}

	if got := remoteBalance(t, c, adminToken, member.AccountID); got != 0 {
		t.Errorf("balance after rejected oversize batch = %d, want 0", got)
	}
}

// A request body over the transport cap is rejected during the read, before
// auth or dispatch — an unauthenticated call to a public method is enough to
// exercise the guard.
func TestRequestBodyTooLargeRejected(t *testing.T) {
	c := setup(t)

	huge := strings.Repeat("a", 5<<20) // 5 MiB, over the 4 MiB body cap
	resp := c.call(t, loginMethod, map[string]any{"email": huge, "password": "x"}, "")

	if resp.Error == nil {
		t.Fatal("oversize body: expected an error, got none")
	}
	if resp.Error.Code != invalidParamsCode {
		t.Errorf("oversize body: error code = %d, want %d", resp.Error.Code, invalidParamsCode)
	}
	if !strings.Contains(resp.Error.Message, "too large") {
		t.Errorf("oversize body: message = %q, want it to mention 'too large'", resp.Error.Message)
	}
}

// A member may earn on their OWN account (A-2). They are both the owner and the
// author of the resulting ledger row.
func TestEarnPointsMemberOwnAccount(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	ref := uniqueRef(t)

	resp := c.call(t, earnPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": member.AccountID,
		"points":     150,
	}, member.Token)
	requireNoError(t, "EarnPoints", resp)

	var earn walletResult
	mustUnmarshal(t, resp.Result, &earn)
	if earn.Kind != "earn" {
		t.Errorf("EarnPoints: kind = %q, want \"earn\"", earn.Kind)
	}
	if earn.Balance != 150 {
		t.Errorf("EarnPoints: balance = %d, want 150", earn.Balance)
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != 150 {
		t.Errorf("GetAccountBalance after self-earn = %d, want 150", got)
	}

	if c.db == nil {
		return
	}
	var owner, author string
	if err := c.db.QueryRow("SELECT owner_id, created_by FROM transactions WHERE ref = $1", ref).Scan(&owner, &author); err != nil {
		t.Fatalf("query transaction row: %v", err)
	}
	if owner != member.UserID {
		t.Errorf("owner_id = %q, want %q", owner, member.UserID)
	}
	if author != member.UserID {
		t.Errorf("created_by = %q, want %q (the acting member)", author, member.UserID)
	}
}

// Ownership still binds: a member cannot earn into someone else's account. The
// scoped account lookup reads as not-found, so the attempt is rejected and the
// owner's balance is untouched (no existence leak, no ledger row).
func TestEarnForeignAccountRejected(t *testing.T) {
	c := setup(t)
	owner := registerMember(t, c)
	intruder := registerMember(t, c)
	ref := uniqueRef(t)

	resp := c.call(t, earnPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": owner.AccountID,
		"points":     1_000_000,
	}, intruder.Token)
	if resp.Error == nil {
		t.Fatal("EarnPoints into a foreign account: expected an error, got none")
	}

	if got := remoteBalance(t, c, owner.Token, owner.AccountID); got != 0 {
		t.Errorf("owner balance after foreign earn = %d, want 0", got)
	}

	if c.db == nil {
		return
	}
	var n int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE ref = $1", ref).Scan(&n); err != nil {
		t.Fatalf("count transaction rows: %v", err)
	}
	if n != 0 {
		t.Errorf("ledger rows for foreign earn = %d, want 0", n)
	}
}

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
