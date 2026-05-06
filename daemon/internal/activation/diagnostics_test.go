package activation

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestServiceDiagnosticsDoesNotReportCompletedTestChatAsFailure(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	stateStore := newMemoryStateStore()
	state := State{
		ActivationID:        "act_completed",
		PrincipalID:         "prn_completed",
		TenantID:            "ten_completed",
		EnvironmentScope:    "test",
		Status:              StatusFirstActionCompleted,
		CurrentStepID:       StepCompleted,
		CompletedStepIDs:    []string{StepTenantResolved, StepQuotaBaselineReady, StepTestChatCompleted},
		ReadinessItems:      []ReadinessItem{readyReadinessItem("tenant-access", ReadinessKindTenantAccess, "Tenant access", now)},
		BlockingReasonCodes: []ReasonCode{},
		FirstAction:         DefaultTestChatFirstAction(true, nil),
		TestChat: &TestChatMetadata{
			ActivationID: "act_completed",
			TenantID:     "ten_completed",
			DispatchID:   "dispatch_completed",
			Status:       TestChatStatusCompleted,
			Provider:     "test",
			Model:        "test-chat",
			Usage:        map[string]any{"totalTokens": 2},
			FinishReason: "stop",
			CompletedAt:  &now,
		},
		CreatedAt:       now,
		UpdatedAt:       now,
		LastEvaluatedAt: now,
	}
	if err := stateStore.UpsertActivationState(ctx, state); err != nil {
		t.Fatalf("UpsertActivationState returned error: %v", err)
	}
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         &fakeIdentityRepository{},
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	items, err := svc.Diagnostics(ctx, GetInput{
		Token:         identity.TokenAuthority{TokenID: "tok_completed", PrincipalID: "prn_completed", Status: identity.StatusActive},
		TenantContext: identity.TenantContext{PrincipalID: "prn_completed", TenantID: "ten_completed", TokenID: "tok_completed"},
	})
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("completed activation should not produce failure diagnostics, got %#v", items)
	}
}
