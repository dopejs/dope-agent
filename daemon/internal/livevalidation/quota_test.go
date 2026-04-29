package livevalidation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestManagerStartHostedQuotaUnavailableFailsClosed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	manager := NewManager(Dependencies{Enabled: true, HostedBilling: true, Clock: func() time.Time { return now }})
	result, err := manager.Start(liveValidationOperatorContext(), StartInput{
		ValidationID:         "lv_quota",
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
	if result.Denials[0].Gate != "quota" || result.Attempt.QuotaDecision.Allowed {
		t.Fatalf("expected quota denial, got %+v", result)
	}
	if result.Attempt.QuotaDecision.ReasonCode != "quota_denied:quota_state_unavailable" {
		t.Fatalf("quota reason=%q", result.Attempt.QuotaDecision.ReasonCode)
	}
}

func liveValidationOperatorContext() context.Context {
	return tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_operator",
		Role:        identity.RoleOperator,
		Permissions: identity.PermissionsForRole(identity.RoleOperator, identity.StatusActive),
	})
}
