package providers

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
)

func TestR37ProviderAuthStateIsTenantLocal(t *testing.T) {
	manager := NewManager(config.Config{}, nil)
	now := time.Now().UTC()
	manager.RestoreManagedAuthStatesForTenant("ten_r37_a", []AuthState{{
		ProviderID: "codex", Family: FamilyCodexCLI, AuthMode: AuthModeLocalCLIBridge,
		Status: AuthStatusAuthenticated, AccountID: "same-human", CLIAvailable: true, LastCheckedAt: now,
	}})
	manager.RestoreManagedAuthStatesForTenant("ten_r37_b", []AuthState{{
		ProviderID: "codex", Family: FamilyCodexCLI, AuthMode: AuthModeLocalCLIBridge,
		Status: AuthStatusRevoked, AccountID: "same-human", CLIAvailable: true, LastCheckedAt: now,
	}})

	stateA, ok := manager.GetAuthStateForTenant("codex", "ten_r37_a")
	if !ok {
		t.Fatal("tenant A provider auth missing")
	}
	stateB, ok := manager.GetAuthStateForTenant("codex", "ten_r37_b")
	if !ok {
		t.Fatal("tenant B provider auth missing")
	}
	if stateA.Status != AuthStatusAuthenticated || stateB.Status != AuthStatusRevoked {
		t.Fatalf("tenant auth state mixed: A=%s B=%s", stateA.Status, stateB.Status)
	}
	if _, ok := manager.GetAuthState("codex"); ok {
		t.Fatal("tenant-owned provider auth fell back to global state")
	}
}

func TestR37ProviderAuthStateDoesNotFallbackToGlobal(t *testing.T) {
	manager := NewManager(config.Config{}, nil)
	now := time.Now().UTC()
	manager.RestoreManagedAuthStates([]AuthState{{
		ProviderID: "codex", Family: FamilyCodexCLI, AuthMode: AuthModeLocalCLIBridge,
		Status: AuthStatusAuthenticated, AccountID: "global", CLIAvailable: true, LastCheckedAt: now,
	}})

	if _, ok := manager.GetAuthStateForTenant("codex", "ten_r37_a"); ok {
		t.Fatal("tenant lookup fell back to global provider auth state")
	}
}
