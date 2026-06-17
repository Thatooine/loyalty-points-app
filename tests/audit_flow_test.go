package tests

import "testing"

// Audit JSON-RPC method, by the exact "<ServiceName>.<Method>" string the client
// sends. The audit trail is readable by both members (scoped to their own
// entries) and admins (every owner's).
const listAuditByRefMethod = "AuditService.FetchTransactionAuditTrail"

// auditEntryResult mirrors audit.AuditEntryResult — one recorded attempt.
type auditEntryResult struct {
	ID             int64   `json:"id"`
	UserID         string  `json:"user_id"`
	TransactionRef *string `json:"transaction_ref"`
	AccountID      *string `json:"account_id"`
	OwnerID        *string `json:"owner_id"`
	Kind           *string `json:"kind"`
	Points         *int64  `json:"points"`
	Outcome        string  `json:"outcome"`
	Reason         string  `json:"reason"`
}

// trailResult mirrors audit.FetchTransactionAuditTrailResult.
type trailResult struct {
	TransactionRef string             `json:"transaction_ref"`
	Entries        []auditEntryResult `json:"entries"`
}

// trail calls AuditService.FetchTransactionAuditTrail for ref with the given token and
// returns the decoded result, asserting no wire error.
func trail(t *testing.T, c *apiClient, token, ref string) trailResult {
	t.Helper()
	resp := c.call(t, listAuditByRefMethod, map[string]any{"transaction_ref": ref}, token)
	requireNoError(t, "FetchTransactionAuditTrail", resp)
	var tr trailResult
	mustUnmarshal(t, resp.Result, &tr)
	return tr
}

// TestListAuditByTransactionRefEndpoint earns points against a member's account, then
// confirms both the owning member and an admin can read the resulting audit
// trail for that ref. The member is scoped to their own entries (owner_id), the
// admin sees the same entry through the all-scoped read. The wire result and the
// persisted audit_entries row are asserted.
func TestListAuditByTransactionRefEndpoint(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)
	admin, adminToken := registerAdmin(t, c)
	ref := uniqueRef(t)

	// Crediting is operator-only: the admin earns against the member's account,
	// which records one accepted audit entry owned by the member.
	earn := c.call(t, earnPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": member.AccountID,
		"points":     150,
	}, adminToken)
	requireNoError(t, "EarnPoints", earn)

	// The owning member reads their own trail.
	memberTrail := trail(t, c, member.Token, ref)
	if memberTrail.TransactionRef != ref {
		t.Errorf("trail ref = %q, want %q", memberTrail.TransactionRef, ref)
	}
	if len(memberTrail.Entries) != 1 {
		t.Fatalf("member trail entries = %d, want 1: %+v", len(memberTrail.Entries), memberTrail.Entries)
	}
	entry := memberTrail.Entries[0]
	if entry.Outcome != "accepted" {
		t.Errorf("outcome = %q, want \"accepted\"", entry.Outcome)
	}
	if entry.Kind == nil || *entry.Kind != "earn" {
		t.Errorf("kind = %v, want \"earn\"", entry.Kind)
	}
	if entry.Points == nil || *entry.Points != 150 {
		t.Errorf("points = %v, want 150", entry.Points)
	}
	// owner_id is the account owner (the member); user_id is the acting admin.
	if entry.OwnerID == nil || *entry.OwnerID != member.UserID {
		t.Errorf("owner_id = %v, want %q", entry.OwnerID, member.UserID)
	}
	if entry.UserID != admin.UserID {
		t.Errorf("user_id = %q, want %q (the acting admin)", entry.UserID, admin.UserID)
	}

	// The admin reads the same trail through the all-scoped read path.
	adminTrail := trail(t, c, adminToken, ref)
	if len(adminTrail.Entries) != 1 {
		t.Errorf("admin trail entries = %d, want 1", len(adminTrail.Entries))
	}

	// Direct persistence: exactly one audit_entries row exists for the ref, owned by
	// the member.
	if c.db == nil {
		t.Log("LOYALTY_DB_DSN not set; skipping direct DB assertions")
		return
	}
	var (
		n     int
		owner string
	)
	if err := c.db.QueryRow("SELECT COUNT(*), MAX(owner_id) FROM audit_entries WHERE transaction_ref = $1", ref).Scan(&n, &owner); err != nil {
		t.Fatalf("query audit_entries rows: %v", err)
	}
	if n != 1 {
		t.Errorf("audit_entries rows for ref %q = %d, want 1", ref, n)
	}
	if owner != member.UserID {
		t.Errorf("persisted owner_id = %q, want %q", owner, member.UserID)
	}
}

// TestListAuditByTransactionRefUnauthenticated confirms the method gate rejects a trail
// read with no token before it reaches the service.
func TestListAuditByTransactionRefUnauthenticated(t *testing.T) {
	c := setup(t)
	resp := c.call(t, listAuditByRefMethod, map[string]any{
		"transaction_ref": uniqueRef(t),
	}, "") // no Bearer token
	if resp.Error == nil {
		t.Fatal("FetchTransactionAuditTrail without token: expected an error, got none")
	}
}

// TestListAuditByTransactionRefForeignMemberScopedOut confirms ownership scoping over
// the wire: a member cannot read the audit trail of a ref recorded against
// another member's account. The scoped read returns an empty trail (no error,
// no existence leak), while the owner still sees their entry.
func TestListAuditByTransactionRefForeignMemberScopedOut(t *testing.T) {
	c := setup(t)
	owner := registerMember(t, c)
	intruder := registerMember(t, c)
	_, adminToken := registerAdmin(t, c)
	ref := uniqueRef(t)

	earn := c.call(t, earnPointsMethod, map[string]any{
		"ref":        ref,
		"account_id": owner.AccountID,
		"points":     75,
	}, adminToken)
	requireNoError(t, "EarnPoints", earn)

	// The intruder sees nothing — the entry is scoped to the owner.
	intruderTrail := trail(t, c, intruder.Token, ref)
	if len(intruderTrail.Entries) != 0 {
		t.Errorf("intruder trail entries = %d, want 0: %+v", len(intruderTrail.Entries), intruderTrail.Entries)
	}

	// The owner still sees their own entry.
	ownerTrail := trail(t, c, owner.Token, ref)
	if len(ownerTrail.Entries) != 1 {
		t.Errorf("owner trail entries = %d, want 1", len(ownerTrail.Entries))
	}
}

// TestListAuditByTransactionRefMissingRef confirms an empty (required) ref is rejected
// by validation rather than returning an empty trail.
func TestListAuditByTransactionRefMissingRef(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	resp := c.call(t, listAuditByRefMethod, map[string]any{
		"transaction_ref": "",
	}, member.Token)
	if resp.Error == nil {
		t.Fatal("FetchTransactionAuditTrail with empty ref: expected a validation error, got none")
	}
}
