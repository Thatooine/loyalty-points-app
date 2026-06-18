package tests

import "testing"

// Logout bumps the per-user token_version, so every token issued before the bump
// is rejected on the next protected request.

const logoutMethod = "Session.Logout"

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
	var logout struct {
		OK bool `json:"ok"`
	}
	mustUnmarshal(t, out.Result, &logout)
	if !logout.OK {
		t.Errorf("Logout: ok = %v, want true", logout.OK)
	}

	after := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token)
	if after.Error == nil {
		t.Fatal("GetAccountBalance after logout: expected unauthorized error, got none")
	}

	if c.db == nil {
		t.Log("database unavailable; skipping token_version assertion")
		return
	}
	var version int
	row := c.db.QueryRow("SELECT token_version FROM users WHERE id = $1", member.UserID)
	if err := row.Scan(&version); err != nil {
		t.Fatalf("query token_version: %v", err)
	}
	if version < 1 {
		t.Errorf("token_version = %d after logout, want >= 1 (the bump that revokes outstanding tokens)", version)
	}
}

// Logout revokes outstanding tokens but does not lock the account: the user can
// log in again and the fresh token is accepted. (This is why the Postman/Requestly
// collection clears {{token}} on logout and tells you to re-run Login.)
func TestLoginWorksAfterLogout(t *testing.T) {
	c := setup(t)
	member := registerMember(t, c)

	out := c.call(t, logoutMethod, map[string]any{}, member.Token)
	requireNoError(t, "Logout", out)

	if revoked := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, member.Token); revoked.Error == nil {
		t.Fatal("old token after logout: expected unauthorized error, got none")
	}

	fresh := loginToken(t, c, member.Email)
	if fresh == member.Token {
		t.Fatal("fresh login returned the same (revoked) token")
	}

	resp := c.call(t, getBalanceMethod, map[string]any{"account_id": member.AccountID}, fresh)
	requireNoError(t, "GetAccountBalance with fresh token after re-login", resp)
}

// A single logout revokes all of a user's sessions, because token_version is
// per-user (logout-everywhere).
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
