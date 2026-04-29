package livevalidation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestManagerStartDeniesMissingExecutePermissionBeforeQuota(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	manager := NewManager(Dependencies{Enabled: true, HostedBilling: true, Clock: func() time.Time { return now }})
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_viewer",
		Role:        identity.RoleViewer,
		Permissions: identity.PermissionsForRole(identity.RoleViewer, identity.StatusActive),
	})

	result, err := manager.Start(ctx, StartInput{
		ValidationID:         "lv_permission",
		CandidateID:          "candidate_1",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
		RequestedScope: SideEffectScope{
			ScopeID:             "scope_1",
			IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
			ApprovalMode:        ApprovalModeScopeLevel,
			DeclaredBy:          "prn_viewer",
			DeclaredAt:          now,
		},
	})
	if !errors.Is(err, ErrLiveValidationBlocked) {
		t.Fatalf("Start err=%v, want ErrLiveValidationBlocked", err)
	}
	if result.Attempt.Status != AttemptStatusBlocked || len(result.Denials) != 1 {
		t.Fatalf("expected blocked denial, got %+v", result)
	}
	if result.Denials[0].Gate != "permission" {
		t.Fatalf("denial gate=%q, want permission", result.Denials[0].Gate)
	}
	if !result.Attempt.QuotaDecision.Allowed {
		t.Fatalf("quota must not be evaluated after permission denial: %+v", result.Attempt.QuotaDecision)
	}
}

func TestManagerStartRequiresUnsupportedCandidateClassesToBeExplicitlyExcluded(t *testing.T) {
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

	blocked, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_mixed_blocked",
		CandidateID:          "candidate_mixed",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMCPToolCall},
		RequestedScope:       scope,
		FreshApprovals: []FreshApproval{{
			ApprovalID:    "approval_scope",
			ValidationID:  "lv_mixed_blocked",
			TenantID:      "ten_1",
			Target:        ApprovalTargetScope,
			ToolClass:     ToolClassDaemonInspectionRead,
			SafetyClass:   SafetyClassReadOnly,
			ApprovedScope: "scope_1",
			Status:        ApprovalStatusApproved,
		}},
	})
	if !errors.Is(err, ErrLiveValidationBlocked) || blocked.Denials[0].Gate != "support_matrix" {
		t.Fatalf("mixed candidate without exclusion err=%v result=%+v, want support_matrix block", err, blocked)
	}

	scope.ExcludedToolClasses = []ToolClass{ToolClassMCPToolCall}
	running, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_mixed_running",
		CandidateID:          "candidate_mixed",
		CandidateToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMCPToolCall},
		RequestedScope:       scope,
		FreshApprovals: []FreshApproval{{
			ApprovalID:    "approval_scope",
			ValidationID:  "lv_mixed_running",
			TenantID:      "ten_1",
			Target:        ApprovalTargetScope,
			ToolClass:     ToolClassDaemonInspectionRead,
			SafetyClass:   SafetyClassReadOnly,
			ApprovedScope: "scope_1",
			Status:        ApprovalStatusApproved,
		}},
	})
	if err != nil || running.Attempt.Status != AttemptStatusRunning {
		t.Fatalf("mixed candidate with explicit exclusion err=%v result=%+v, want running", err, running)
	}
}

func TestManagerStartResolvesCandidateToolClassesBeforeSupportGate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	manager := NewManager(Dependencies{
		Enabled: true,
		Store:   &memoryStore{},
		Clock:   func() time.Time { return now },
		CandidateToolClassResolver: func(_ context.Context, candidateID string) ([]ToolClass, error) {
			if candidateID != "candidate_mixed" {
				t.Fatalf("resolver candidateID=%q, want candidate_mixed", candidateID)
			}
			return []ToolClass{ToolClassDaemonInspectionRead, ToolClassMCPToolCall}, nil
		},
	})
	blocked, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID: "lv_resolved_mixed",
		CandidateID:  "candidate_mixed",
		RequestedScope: SideEffectScope{
			ScopeID:             "scope_1",
			IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
			ApprovalMode:        ApprovalModeScopeLevel,
			DeclaredBy:          "prn_operator",
			DeclaredAt:          now,
		},
		FreshApprovals: []FreshApproval{{
			ApprovalID:    "approval_scope",
			ValidationID:  "lv_resolved_mixed",
			TenantID:      "ten_1",
			Target:        ApprovalTargetScope,
			ToolClass:     ToolClassDaemonInspectionRead,
			SafetyClass:   SafetyClassReadOnly,
			ApprovedScope: "scope_1",
			Status:        ApprovalStatusApproved,
		}},
	})
	if !errors.Is(err, ErrLiveValidationBlocked) || blocked.Denials[0].Gate != "support_matrix" || blocked.Denials[0].Reference != string(ToolClassMCPToolCall) {
		t.Fatalf("resolved mixed candidate err=%v result=%+v, want unsupported support_matrix block", err, blocked)
	}
}
