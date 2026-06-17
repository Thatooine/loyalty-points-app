package tests

import (
	"sync"
	"testing"
)

// preMintRefs generates refs on the test goroutine. uniqueRef calls t.Fatalf,
// which is illegal off the test goroutine, so concurrency tests must mint every
// ref up front rather than inside a worker.
func preMintRefs(t *testing.T, n int) []string {
	t.Helper()
	refs := make([]string, n)
	for i := range refs {
		refs[i] = uniqueRef(t)
	}
	return refs
}

// TestConcurrentEarnsNoLostUpdates fires many earns at one account at once.
// The single guarded `UPDATE ... balance = balance + delta` must serialise them
// so none is lost: an app-level read-modify-write would drop increments under
// contention and leave the balance below workers*pointsEach. Proves the
// "overlapping requests on the same account stay correct" constraint for credits.
func TestConcurrentEarnsNoLostUpdates(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	const (
		workers    = 20
		pointsEach = 50
	)
	wantBalance := int64(workers * pointsEach)
	refs := preMintRefs(t, workers)

	var wg sync.WaitGroup
	resps := make([]rpcResponse, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resps[i], errs[i] = c.callRaw(earnPointsMethod, map[string]any{
				"ref":        refs[i],
				"account_id": member.AccountID,
				"points":     pointsEach,
			}, adminToken)
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d transport error: %v", i, errs[i])
		}
		if resps[i].Error != nil {
			t.Fatalf("worker %d earn rejected: %+v", i, resps[i].Error)
		}
	}

	if got := remoteBalance(t, c, member.Token, member.AccountID); got != wantBalance {
		t.Errorf("balance after %d concurrent earns = %d, want %d (lost update)", workers, got, wantBalance)
	}

	if c.db == nil {
		return
	}
	var ledgerSum int64
	if err := c.db.QueryRow("SELECT COALESCE(SUM(points), 0) FROM transactions WHERE account_id = $1", member.AccountID).Scan(&ledgerSum); err != nil {
		t.Fatalf("query ledger sum: %v", err)
	}
	if ledgerSum != wantBalance {
		t.Errorf("ledger sum = %d, want %d", ledgerSum, wantBalance)
	}
	if bal, ok := dbBalance(t, c, member.AccountID); ok && bal != ledgerSum {
		t.Errorf("account balance %d != ledger sum %d", bal, ledgerSum)
	}
}

// TestConcurrentSpendsRespectOverdraftFloor seeds a fixed balance, then fires
// more concurrent spends than the balance can fund. The atomic guarded
// `UPDATE ... WHERE balance + delta >= 0` must let through only as many spends
// as the balance allows and never drive it negative; a check-then-update race
// would let several spends pass the same read and overspend. Proves the floor
// holds under contention — the concurrency-safety half of the constraint.
func TestConcurrentSpendsRespectOverdraftFloor(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)

	const (
		seed      = 500
		workers   = 20
		spendEach = 50 // demand 20*50=1000 against 500 funded → at most 10 succeed
	)
	maxAffordable := seed / spendEach

	seedResp := c.call(t, earnPointsMethod, map[string]any{
		"ref":        uniqueRef(t),
		"account_id": member.AccountID,
		"points":     seed,
	}, adminToken)
	requireNoError(t, "EarnPoints (seed)", seedResp)

	refs := preMintRefs(t, workers)

	var wg sync.WaitGroup
	resps := make([]rpcResponse, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resps[i], errs[i] = c.callRaw(spendPointsMethod, map[string]any{
				"ref":        refs[i],
				"account_id": member.AccountID,
				"points":     spendEach,
			}, member.Token) // a member may spend their own points
		}(i)
	}
	wg.Wait()

	accepted := 0
	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d transport error: %v", i, errs[i])
		}
		if resps[i].Error == nil {
			accepted++
		}
	}

	if accepted > maxAffordable {
		t.Errorf("accepted %d spends, want <= %d (overdraft floor breached under contention)", accepted, maxAffordable)
	}

	wantBalance := int64(seed - accepted*spendEach)
	if wantBalance < 0 {
		t.Fatalf("balance went negative: accepted=%d drove seed=%d below zero", accepted, seed)
	}
	if got := remoteBalance(t, c, member.Token, member.AccountID); got != wantBalance {
		t.Errorf("balance = %d, want %d (seed - accepted*spendEach)", got, wantBalance)
	}

	if c.db == nil {
		return
	}
	// Rejected spends roll back, so only the seed earn + accepted spends persist.
	var rows int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE account_id = $1", member.AccountID).Scan(&rows); err != nil {
		t.Fatalf("count transaction rows: %v", err)
	}
	if rows != accepted+1 {
		t.Errorf("ledger rows = %d, want %d (seed earn + %d accepted spends)", rows, accepted+1, accepted)
	}
	if bal, ok := dbBalance(t, c, member.AccountID); ok && bal != wantBalance {
		t.Errorf("persisted balance = %d, want %d", bal, wantBalance)
	}
}
