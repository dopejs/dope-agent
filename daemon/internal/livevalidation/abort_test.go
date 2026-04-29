package livevalidation

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestKillSwitchAbortsPendingFutureSideEffects(t *testing.T) {
	store := &memoryStore{}
	manager := NewManager(Dependencies{Enabled: true, Store: store, Clock: fixedClock})
	store.attempts = append(store.attempts, Attempt{ValidationID: "lv_1", TenantID: "ten_1", Status: AttemptStatusRunning})
	store.ledger = append(store.ledger,
		SideEffectLedgerEntry{LedgerEntryID: "ledger_pending", ValidationID: "lv_1", TenantID: "ten_1", Outcome: LedgerOutcomeAttempted},
		SideEffectLedgerEntry{LedgerEntryID: "ledger_done", ValidationID: "lv_1", TenantID: "ten_1", Outcome: LedgerOutcomeCompleted},
	)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_admin",
		Role:        identity.RoleAdmin,
		Permissions: identity.PermissionsForRole(identity.RoleAdmin, identity.StatusActive),
	})
	if _, err := manager.SetKillSwitch(ctx, KillSwitch{Scope: KillSwitchScopeTenant, Enabled: true, Reason: "containment"}); err != nil {
		t.Fatalf("SetKillSwitch returned error: %v", err)
	}
	updated, ok, err := store.GetLiveValidationAttempt(context.Background(), "ten_1", "lv_1")
	if err != nil || !ok {
		t.Fatalf("GetLiveValidationAttempt ok=%v err=%v", ok, err)
	}
	if updated.Status != AttemptStatusAborted {
		t.Fatalf("Status=%s, want aborted", updated.Status)
	}
	if store.ledger[0].Outcome != LedgerOutcomeAborted || store.ledger[1].Outcome != LedgerOutcomeCompleted {
		t.Fatalf("ledger=%+v, want pending aborted and completed preserved", store.ledger)
	}
}
