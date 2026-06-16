package tests

import "testing"

// Endpoint tests for token revocation via the per-user token_version (session
// epoch). Logout bumps the version, so every access token issued before the
// bump is rejected by the validator on the next protected request.

const logoutMethod = "Session.Logout"

// Logs the member in again, yielding a second concurrent session for the user.
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

func TestLogoutRevokesToken(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	before := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	requireNoError(t, "GetAccountBalance (before logout)", before)

	out := c.call(t, logoutMethod, map[string]any{}, member.Token)
	requireNoError(t, "Logout", out)

	after := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	if after.Error == nil {
		t.Fatal("GetAccountBalance after logout: expected unauthorized error, got none")
	}
}

// Two concurrent sessions for one user are both revoked by a single logout,
// because token_version is per-user (logout-everywhere).
func TestLogoutRevokesAllSessions(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c) // session A (the registration token)
	tokenB := loginToken(t, c, member.Email)

	a := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	requireNoError(t, "GetAccountBalance (session A before)", a)
	b := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, tokenB)
	requireNoError(t, "GetAccountBalance (session B before)", b)

	out := c.call(t, logoutMethod, map[string]any{}, member.Token)
	requireNoError(t, "Logout", out)

	if afterA := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token); afterA.Error == nil {
		t.Error("session A after logout: expected unauthorized error, got none")
	}
	if afterB := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, tokenB); afterB.Error == nil {
		t.Error("session B after logout: expected unauthorized error, got none")
	}
}

func TestLogoutUnauthenticated(t *testing.T) {
	c := setup(t)

	resp := c.call(t, logoutMethod, map[string]any{}, "")
	if resp.Error == nil {
		t.Fatal("Logout without token: expected an error, got none")
	}
}
