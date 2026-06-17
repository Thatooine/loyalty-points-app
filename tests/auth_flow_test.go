package tests

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	registerMethod = "UserRegistrationService.Register"
	loginMethod    = "EmailPasswordAuthenticator.Login"

	testPassword    = "hunter2pw"
	testName        = "Ada Lovelace"
	testAccountName = "Primary Wallet"
)

type registerResult struct {
	Token     string `json:"token"`
	UserID    string `json:"userID"`
	AccountID string `json:"accountID"`
	Email     string `json:"email"`
}

type loginResult struct {
	Token  string `json:"token"`
	UserID string `json:"userID"`
	Email  string `json:"email"`
}

func registerParams(email string) map[string]any {
	return map[string]any{
		"email":       email,
		"password":    testPassword,
		"name":        testName,
		"accountName": testAccountName,
	}
}

// Logging in with the same credentials in a fresh request proves the user and
// password hash were durably stored.
func TestRegisterThenLogin(t *testing.T) {
	c := setup(t)
	email := uniqueEmail(t)

	var reg registerResult
	resp := c.call(t, registerMethod, registerParams(email), "")
	requireNoError(t, "Register", resp)
	mustUnmarshal(t, resp.Result, &reg)

	if reg.Token == "" {
		t.Error("Register: empty token")
	}
	if reg.UserID == "" {
		t.Error("Register: empty userID")
	}
	if reg.AccountID == "" {
		t.Error("Register: empty accountID")
	}
	if reg.Email != email {
		t.Errorf("Register: email = %q, want %q", reg.Email, email)
	}

	var login loginResult
	resp = c.call(t, loginMethod, map[string]any{"email": email, "password": testPassword}, "")
	requireNoError(t, "Login", resp)
	mustUnmarshal(t, resp.Result, &login)

	if login.Token == "" {
		t.Error("Login: empty token")
	}
	if login.UserID != reg.UserID {
		t.Errorf("Login: userID = %q, want %q (from register)", login.UserID, reg.UserID)
	}

	if c.db == nil {
		t.Log("LOYALTY_DB_DSN not set; skipping direct DB assertions")
		return
	}

	var dbEmail, dbRole, dbHash string
	row := c.db.QueryRow("SELECT email, role, password_hash FROM users WHERE id = $1", reg.UserID)
	if err := row.Scan(&dbEmail, &dbRole, &dbHash); err != nil {
		t.Fatalf("query user row: %v", err)
	}
	if dbEmail != email {
		t.Errorf("persisted email = %q, want %q", dbEmail, email)
	}
	if dbRole != "member" {
		t.Errorf("persisted role = %q, want %q", dbRole, "member")
	}
	if !strings.HasPrefix(dbHash, "$2") {
		t.Errorf("password_hash %q is not a bcrypt hash", dbHash)
	}
	if dbHash == testPassword {
		t.Error("password stored in plaintext")
	}

	var (
		accName, accUser string
		accBalance       int64
	)
	row = c.db.QueryRow("SELECT name, balance, owner_id FROM accounts WHERE id = $1", reg.AccountID)
	if err := row.Scan(&accName, &accBalance, &accUser); err != nil {
		t.Fatalf("query account row: %v", err)
	}
	if accName != testAccountName {
		t.Errorf("account name = %q, want %q", accName, testAccountName)
	}
	if accBalance != 0 {
		t.Errorf("account balance = %d, want 0", accBalance)
	}
	if accUser != reg.UserID {
		t.Errorf("account user_id = %q, want %q", accUser, reg.UserID)
	}
}

func TestRegisterDuplicateEmailRejected(t *testing.T) {
	c := setup(t)
	email := uniqueEmail(t)

	resp := c.call(t, registerMethod, registerParams(email), "")
	requireNoError(t, "Register", resp)

	dup := c.call(t, registerMethod, registerParams(email), "")
	if dup.Error == nil {
		t.Fatal("duplicate registration: expected an error, got none")
	}
}

func TestLoginWrongPasswordRejected(t *testing.T) {
	c := setup(t)
	email := uniqueEmail(t)

	resp := c.call(t, registerMethod, registerParams(email), "")
	requireNoError(t, "Register", resp)

	bad := c.call(t, loginMethod, map[string]any{"email": email, "password": "wrong-password"}, "")
	if bad.Error == nil {
		t.Fatal("login with wrong password: expected an error, got none")
	}
}

func TestLoginUnknownUserRejected(t *testing.T) {
	c := setup(t)

	resp := c.call(t, loginMethod, map[string]any{"email": uniqueEmail(t), "password": testPassword}, "")
	if resp.Error == nil {
		t.Fatal("login for unknown user: expected an error, got none")
	}
}

func requireNoError(t *testing.T, label string, resp rpcResponse) {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("%s: unexpected JSON-RPC error: code=%d message=%q", label, resp.Error.Code, resp.Error.Message)
	}
}

func mustUnmarshal(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode result %q: %v", raw, err)
	}
}
