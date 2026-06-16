package tests

import "testing"

// Endpoint tests for token revocation via the per-user token_version (session
// epoch). Logout bumps the version, so every access token issued before the
// bump is rejected by the validator on the next protected request.

const logoutMethod = "Session.Logout"

// loginToken logs the given member in again and returns a fresh access token —
// a second concurrent session for the same user.
func loginToken(t *testing.T, c *apiClient, email string) string {
	t.Helper()
	resp := c.call(t, loginMethod, map[string]any{"email": email, "password": testPassword}, "")
	requireNoError(t, "Login", resp)
	var login struct {
		Token string `json:"token"`
	}
	mustUnmarshal(t, resp.Result, &login)
	if login.Token == "" {
		t.Fatal("Login: empty token")
	}
	return login.Token
}

// TestLogoutRevokesToken confirms a token works before logout and is rejected
// after: logout bumps token_version, so the now-stale token fails validation.
func TestLogoutRevokesToken(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	// The token works on a protected method before logout.
	before := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	requireNoError(t, "GetAccountBalance (before logout)", before)

	// Log out: this bumps the user's token_version.
	out := c.call(t, logoutMethod, map[string]any{}, member.Token)
	requireNoError(t, "Logout", out)

	// The same token is now rejected — its stamped version is stale.
	after := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	if after.Error == nil {
		t.Fatal("GetAccountBalance after logout: expected unauthorized error, got none")
	}
}

// TestLogoutRevokesAllSessions confirms the multi-session semantics: two
// concurrent sessions for one user are both valid, and a single logout revokes
// both (logout-everywhere), because token_version is per-user.
func TestLogoutRevokesAllSessions(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c) // session A (the registration token)
	tokenB := loginToken(t, c, member.Email)

	// Both sessions work before logout.
	a := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	requireNoError(t, "GetAccountBalance (session A before)", a)
	b := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, tokenB)
	requireNoError(t, "GetAccountBalance (session B before)", b)

	// Log out through session A.
	out := c.call(t, logoutMethod, map[string]any{}, member.Token)
	requireNoError(t, "Logout", out)

	// Both sessions are now revoked — one bump invalidates every token.
	if afterA := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token); afterA.Error == nil {
		t.Error("session A after logout: expected unauthorized error, got none")
	}
	if afterB := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, tokenB); afterB.Error == nil {
		t.Error("session B after logout: expected unauthorized error, got none")
	}
}

// TestLogoutUnauthenticated confirms logout itself is a protected method: with
// no token the method gate rejects it.
func TestLogoutUnauthenticated(t *testing.T) {
	c := setup(t)

	resp := c.call(t, logoutMethod, map[string]any{}, "")
	if resp.Error == nil {
		t.Fatal("Logout without token: expected an error, got none")
	}
}
