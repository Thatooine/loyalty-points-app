package scope

import (
	"context"
	"testing"
)

func TestOf(t *testing.T) {
	tests := []struct {
		name       string
		permission string
		wantScope  Scope
		wantOK     bool
	}{
		{"own scope", "account:read:own", Own, true},
		{"all scope", "account:read:all", All, true},
		{"all scope on action without own form", "wallet:batch:all", All, true},
		{"no scope segment", "wallet:earn", "", false},
		{"empty scope segment", "wallet:transact:", "", false},
		{"empty string", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Of(tt.permission)
			if got != tt.wantScope || ok != tt.wantOK {
				t.Fatalf("Of(%q) = (%q, %v), want (%q, %v)", tt.permission, got, ok, tt.wantScope, tt.wantOK)
			}
		})
	}
}

func TestIsAllIsOwn(t *testing.T) {
	if !IsAll("account:read:all") {
		t.Fatal("IsAll(account:read:all) = false, want true")
	}
	if IsAll("account:read:own") {
		t.Fatal("IsAll(account:read:own) = true, want false")
	}
	if !IsOwn("account:read:own") {
		t.Fatal("IsOwn(account:read:own) = false, want true")
	}
	if IsOwn("wallet:earn") {
		t.Fatal("IsOwn(wallet:earn) = true, want false")
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := ContextWithScope(context.Background(), All)
	got, ok := FromContext(ctx)
	if !ok || got != All {
		t.Fatalf("FromContext = (%q, %v), want (%q, true)", got, ok, All)
	}

	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("FromContext on bare context = ok, want not ok")
	}
}
