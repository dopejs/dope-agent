package livevalidation

import (
	"errors"
	"testing"
	"time"
)

func TestManagerStartCalculatesScopeAndPerActionFreshApprovals(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	store := &memoryStore{}
	manager := NewManager(Dependencies{Enabled: true, Store: store, Clock: func() time.Time { return now }})
	scope := SideEffectScope{
		ScopeID:             "scope_1",
		IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMailSend},
		IncludedActions:     []string{"mail_send_1"},
		ApprovalMode:        ApprovalModeMixed,
		DeclaredBy:          "prn_operator",
		DeclaredAt:          now,
	}

	awaiting, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_awaiting",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMailSend},
		RequestedScope:       scope,
		FreshApprovals:       []FreshApproval{{ApprovalID: "approval_unrelated", Target: ApprovalTargetScope, ToolClass: ToolClassCalendarEventCreate, Status: ApprovalStatusApproved}},
	})
	if err != nil {
		t.Fatalf("Start awaiting approval err=%v", err)
	}
	if awaiting.Attempt.Status != AttemptStatusAwaitingApproval {
		t.Fatalf("status=%s, want awaiting_approval", awaiting.Attempt.Status)
	}
	if awaiting.Attempt.ApprovalSummary.Required != 2 || awaiting.Attempt.ApprovalSummary.Pending != 2 {
		t.Fatalf("approval summary=%+v, want required=2 pending=2", awaiting.Attempt.ApprovalSummary)
	}

	running, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_running",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMailSend},
		RequestedScope:       scope,
		FreshApprovals: []FreshApproval{
			{ApprovalID: "approval_scope", ValidationID: "lv_running", TenantID: "ten_1", Target: ApprovalTargetScope, ToolClass: ToolClassDaemonInspectionRead, SafetyClass: SafetyClassReadOnly, ApprovedScope: "scope_1", Status: ApprovalStatusApproved},
			{ApprovalID: "approval_action", ValidationID: "lv_running", TenantID: "ten_1", Target: ApprovalTargetAction, ToolClass: ToolClassMailSend, SafetyClass: SafetyClassNonIdempotentMutation, ActionRef: "mail_send_1", Status: ApprovalStatusApproved},
		},
	})
	if err != nil {
		t.Fatalf("Start running err=%v", err)
	}
	if running.Attempt.Status != AttemptStatusRunning {
		t.Fatalf("status=%s, want running", running.Attempt.Status)
	}
	if running.Attempt.ApprovalSummary.Required != 2 || running.Attempt.ApprovalSummary.Approved != 2 {
		t.Fatalf("approval summary=%+v, want required=2 approved=2", running.Attempt.ApprovalSummary)
	}
}

func TestManagerStartRejectsStaleDeniedAndExpiredApprovals(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	manager := NewManager(Dependencies{Enabled: true, Store: &memoryStore{}, Clock: func() time.Time { return now }})
	scope := SideEffectScope{
		ScopeID:             "scope_1",
		IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
		ApprovalMode:        ApprovalModeScopeLevel,
		DeclaredBy:          "prn_operator",
		DeclaredAt:          now,
	}

	stale, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_current",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
		RequestedScope:       scope,
		FreshApprovals: []FreshApproval{{
			ApprovalID:    "approval_prior_validation",
			ValidationID:  "lv_prior",
			TenantID:      "ten_1",
			Target:        ApprovalTargetScope,
			ToolClass:     ToolClassDaemonInspectionRead,
			SafetyClass:   SafetyClassReadOnly,
			ApprovedScope: "scope_1",
			Status:        ApprovalStatusApproved,
		}},
	})
	if err != nil {
		t.Fatalf("Start with stale approval returned error: %v", err)
	}
	if stale.Attempt.Status != AttemptStatusAwaitingApproval || stale.Attempt.ApprovalSummary.Pending != 1 {
		t.Fatalf("stale approval should be ignored and remain pending, got %+v", stale.Attempt.ApprovalSummary)
	}

	crossTenant, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_cross_tenant",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
		RequestedScope:       scope,
		FreshApprovals: []FreshApproval{{
			ApprovalID:    "approval_other_tenant",
			ValidationID:  "lv_cross_tenant",
			TenantID:      "ten_other",
			Target:        ApprovalTargetScope,
			ToolClass:     ToolClassDaemonInspectionRead,
			SafetyClass:   SafetyClassReadOnly,
			ApprovedScope: "scope_1",
			Status:        ApprovalStatusApproved,
		}},
	})
	if err != nil {
		t.Fatalf("Start with cross-tenant approval returned error: %v", err)
	}
	if crossTenant.Attempt.Status != AttemptStatusAwaitingApproval || crossTenant.Attempt.ApprovalSummary.Pending != 1 {
		t.Fatalf("cross-tenant approval should be ignored and remain pending, got %+v", crossTenant.Attempt.ApprovalSummary)
	}

	denied, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_denied",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
		RequestedScope:       scope,
		FreshApprovals: []FreshApproval{{
			ApprovalID:    "approval_denied",
			ValidationID:  "lv_denied",
			TenantID:      "ten_1",
			Target:        ApprovalTargetScope,
			ToolClass:     ToolClassDaemonInspectionRead,
			SafetyClass:   SafetyClassReadOnly,
			ApprovedScope: "scope_1",
			Status:        ApprovalStatusDenied,
		}},
	})
	if !errors.Is(err, ErrLiveValidationBlocked) || denied.Denials[0].Gate != "approval" {
		t.Fatalf("denied approval err=%v result=%+v, want approval block", err, denied)
	}

	expired, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_expired",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
		RequestedScope:       scope,
		FreshApprovals: []FreshApproval{{
			ApprovalID:    "approval_expired",
			ValidationID:  "lv_expired",
			TenantID:      "ten_1",
			Target:        ApprovalTargetScope,
			ToolClass:     ToolClassDaemonInspectionRead,
			SafetyClass:   SafetyClassReadOnly,
			ApprovedScope: "scope_1",
			Status:        ApprovalStatusExpired,
		}},
	})
	if !errors.Is(err, ErrLiveValidationBlocked) || expired.Denials[0].ReasonCode != "live_validation.approval_expired" {
		t.Fatalf("expired approval err=%v result=%+v, want approval_expired block", err, expired)
	}
}
