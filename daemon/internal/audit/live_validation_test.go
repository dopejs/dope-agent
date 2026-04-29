package audit

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestLiveValidationLedgerReconciliationAndKillSwitchAuditBuilders(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	ledger := BuildLiveValidationLedgerAuditEvent(LiveValidationLedgerAuditInput{Entry: livevalidation.SideEffectLedgerEntry{
		LedgerEntryID: "ledger_1",
		ValidationID:  "lv_1",
		TenantID:      "ten_1",
		ToolClass:     livevalidation.ToolClassMailSend,
		Outcome:       livevalidation.LedgerOutcomeOperatorActionNeeded,
		UpdatedAt:     now,
	}})
	if ledger.TenantID != "ten_1" || ledger.Document["ledgerEntryId"] != "ledger_1" {
		t.Fatalf("ledger audit=%+v", ledger)
	}
	reconciliation := BuildLiveValidationReconciliationAuditEvent(LiveValidationReconciliationAuditInput{Resolution: livevalidation.ReconciliationResolution{
		ReconciliationID:  "rec_1",
		AmbiguousCommitID: "amb_1",
		TenantID:          "ten_1",
		ResolvedBy:        "prn_admin",
		Resolution:        livevalidation.ResolutionConfirmedCommitted,
		ResolvedAt:        now,
	}})
	if reconciliation.PrincipalID != "prn_admin" || reconciliation.Document["resolution"] != livevalidation.ResolutionConfirmedCommitted {
		t.Fatalf("reconciliation audit=%+v", reconciliation)
	}
	killSwitch := BuildLiveValidationKillSwitchAuditEvent(LiveValidationKillSwitchAuditInput{KillSwitch: livevalidation.KillSwitch{
		KillSwitchID: "kill_1",
		Scope:        livevalidation.KillSwitchScopeTenant,
		TenantID:     "ten_1",
		Enabled:      true,
		ChangedBy:    "prn_admin",
		ChangedAt:    now,
	}})
	if killSwitch.Document["enabled"] != true || killSwitch.PrincipalID != "prn_admin" {
		t.Fatalf("kill switch audit=%+v", killSwitch)
	}
}
