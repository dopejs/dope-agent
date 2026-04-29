package livevalidation

import (
	"errors"
	"testing"
	"time"
)

func TestManagerStartDeniesEnabledTenantKillSwitchBeforeApproval(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	store := &memoryStore{
		killSwitches: []KillSwitch{{
			KillSwitchID: "kill_1",
			Scope:        KillSwitchScopeTenant,
			TenantID:     "ten_1",
			Enabled:      true,
			Reason:       "contain live validation",
			ChangedBy:    "prn_owner",
			ChangedAt:    now,
		}},
	}
	manager := NewManager(Dependencies{Enabled: true, Store: store, Clock: func() time.Time { return now }})
	result, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_kill",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
		RequestedScope: SideEffectScope{
			ScopeID:             "scope_1",
			IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
			ApprovalMode:        ApprovalModeScopeLevel,
			DeclaredBy:          "prn_operator",
			DeclaredAt:          now,
		},
	})
	if !errors.Is(err, ErrLiveValidationBlocked) {
		t.Fatalf("Start err=%v, want ErrLiveValidationBlocked", err)
	}
	if result.Denials[0].Gate != "kill_switch" {
		t.Fatalf("denial gate=%q, want kill_switch", result.Denials[0].Gate)
	}
	if result.Attempt.ApprovalSummary.Required != 0 {
		t.Fatalf("approval must not be evaluated after kill switch denial: %+v", result.Attempt.ApprovalSummary)
	}
}
