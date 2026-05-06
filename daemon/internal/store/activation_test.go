package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
)

func TestSQLiteStorePersistsActivationStateAcrossRestart(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataDir := t.TempDir()
	now := time.Now().UTC().Truncate(0)
	completedAt := now.Add(3 * time.Minute)

	s, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}

	state := activation.State{
		ActivationID:        "act_1",
		PrincipalID:         "prn_1",
		TenantID:            "ten_personal",
		EnvironmentScope:    "test",
		Status:              activation.StatusFirstActionCompleted,
		CurrentStepID:       activation.StepCompleted,
		CompletedStepIDs:    []string{activation.StepTenantResolved, activation.StepQuotaBaselineReady, activation.StepTestChatCompleted},
		BlockingReasonCodes: []activation.ReasonCode{},
		ReadinessItems: []activation.ReadinessItem{
			{
				ItemID:                "quota-baseline",
				ItemKind:              activation.ReadinessKindQuotaBaseline,
				Status:                activation.ReadinessStatusReady,
				DisplayName:           "Quota baseline",
				RequiredForActivation: true,
				Retryable:             false,
				RemediationOwner:      activation.RemediationOwnerNoneRequired,
				UpdatedAt:             now,
			},
		},
		QuotaBaseline: &activation.QuotaBaseline{
			TenantID:         "ten_personal",
			PlanKey:          "free",
			EnforcementMode:  "enforced",
			Status:           activation.QuotaBaselineStatusAvailable,
			Quotas:           []activation.QuotaProjection{},
			ProjectedAt:      now,
			ProjectionSource: "billing",
		},
		FirstAction: activation.FirstAction{
			ActionID:        activation.FirstActionTestChat,
			ActionKind:      activation.FirstActionTestChat,
			DisplayName:     "Test chat",
			Recommended:     true,
			Available:       true,
			BlockingItemIDs: []string{},
			InvokeRoute:     "/v1/activation/test-chat",
			ResultRoute:     "/v1/activation",
		},
		TestChat: &activation.TestChatMetadata{
			ActivationID: "act_1",
			TenantID:     "ten_personal",
			DispatchID:   "dispatch_1",
			Status:       activation.TestChatStatusCompleted,
			Provider:     "test",
			Model:        "test-chat",
			Usage:        map[string]any{"inputTokens": float64(8), "outputTokens": float64(4)},
			FinishReason: "stop",
			CompletedAt:  &completedAt,
		},
		CreatedAt:              now,
		UpdatedAt:              now.Add(time.Minute),
		FirstActionCompletedAt: &completedAt,
		LastEvaluatedAt:        now.Add(2 * time.Minute),
	}

	if err := s.UpsertActivationState(ctx, state); err != nil {
		t.Fatalf("UpsertActivationState returned error: %v", err)
	}
	got, ok, err := s.GetActivationState(ctx, state.ActivationID)
	if err != nil {
		t.Fatalf("GetActivationState returned error: %v", err)
	}
	if !ok {
		t.Fatal("GetActivationState did not find activation state")
	}
	assertActivationStateRoundTrip(t, got, state)

	gotByPrincipalTenant, ok, err := s.GetActivationStateForPrincipalTenant(ctx, state.PrincipalID, state.TenantID)
	if err != nil {
		t.Fatalf("GetActivationStateForPrincipalTenant returned error: %v", err)
	}
	if !ok {
		t.Fatal("GetActivationStateForPrincipalTenant did not find activation state")
	}
	assertActivationStateRoundTrip(t, gotByPrincipalTenant, state)

	if err := s.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	reopened, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("reopen NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = reopened.Close()
	})
	restarted, ok, err := reopened.GetActivationState(ctx, state.ActivationID)
	if err != nil {
		t.Fatalf("restarted GetActivationState returned error: %v", err)
	}
	if !ok {
		t.Fatal("restarted GetActivationState did not find activation state")
	}
	assertActivationStateRoundTrip(t, restarted, state)
}

func TestSQLiteStoreActivationStateUniquePerPrincipalTenant(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(0)
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})

	original := activation.State{
		ActivationID:        "act_original",
		PrincipalID:         "prn_1",
		TenantID:            "ten_personal",
		EnvironmentScope:    "test",
		Status:              activation.StatusInProgress,
		CurrentStepID:       activation.StepQuotaBaseline,
		CompletedStepIDs:    []string{activation.StepTenantResolved},
		BlockingReasonCodes: []activation.ReasonCode{},
		ReadinessItems:      []activation.ReadinessItem{},
		FirstAction:         activation.DefaultTestChatFirstAction(true, nil),
		CreatedAt:           now,
		UpdatedAt:           now,
		LastEvaluatedAt:     now,
	}
	replacement := original
	replacement.ActivationID = "act_replacement"
	replacement.Status = activation.StatusActive
	replacement.CurrentStepID = activation.StepTestChat
	replacement.CompletedStepIDs = append(replacement.CompletedStepIDs, activation.StepQuotaBaselineReady)
	replacement.UpdatedAt = now.Add(time.Minute)
	replacement.LastEvaluatedAt = now.Add(time.Minute)

	if err := s.UpsertActivationState(ctx, original); err != nil {
		t.Fatalf("UpsertActivationState original returned error: %v", err)
	}
	if err := s.UpsertActivationState(ctx, replacement); err != nil {
		t.Fatalf("UpsertActivationState replacement returned error: %v", err)
	}

	got, ok, err := s.GetActivationStateForPrincipalTenant(ctx, "prn_1", "ten_personal")
	if err != nil {
		t.Fatalf("GetActivationStateForPrincipalTenant returned error: %v", err)
	}
	if !ok {
		t.Fatal("activation state not found by principal tenant")
	}
	if got.ActivationID != replacement.ActivationID || got.Status != activation.StatusActive {
		t.Fatalf("expected replacement activation %q active, got id=%q status=%q", replacement.ActivationID, got.ActivationID, got.Status)
	}
	if _, ok, err := s.GetActivationState(ctx, original.ActivationID); err != nil {
		t.Fatalf("GetActivationState original returned error: %v", err)
	} else if ok {
		t.Fatalf("expected original activation id %q to be replaced by unique principal tenant upsert", original.ActivationID)
	}
}

func assertActivationStateRoundTrip(t *testing.T, got, want activation.State) {
	t.Helper()
	if got.ActivationID != want.ActivationID || got.PrincipalID != want.PrincipalID || got.TenantID != want.TenantID {
		t.Fatalf("identity mismatch: got %#v want %#v", got, want)
	}
	if got.Status != want.Status || got.CurrentStepID != want.CurrentStepID || got.EnvironmentScope != want.EnvironmentScope {
		t.Fatalf("state mismatch: got status=%q step=%q env=%q, want status=%q step=%q env=%q", got.Status, got.CurrentStepID, got.EnvironmentScope, want.Status, want.CurrentStepID, want.EnvironmentScope)
	}
	if len(got.ReadinessItems) != 1 || got.ReadinessItems[0].ItemKind != activation.ReadinessKindQuotaBaseline {
		t.Fatalf("readiness did not round-trip: %#v", got.ReadinessItems)
	}
	if got.QuotaBaseline == nil || got.QuotaBaseline.Status != activation.QuotaBaselineStatusAvailable {
		t.Fatalf("quota baseline did not round-trip: %#v", got.QuotaBaseline)
	}
	if got.TestChat == nil || got.TestChat.DispatchID != "dispatch_1" || got.TestChat.Status != activation.TestChatStatusCompleted {
		t.Fatalf("test chat metadata did not round-trip: %#v", got.TestChat)
	}
	if got.FirstActionCompletedAt == nil || !got.FirstActionCompletedAt.Equal(*want.FirstActionCompletedAt) {
		t.Fatalf("first action completed at did not round-trip: got %#v want %#v", got.FirstActionCompletedAt, want.FirstActionCompletedAt)
	}
}
