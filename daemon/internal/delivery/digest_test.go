package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestDigestWindowsBatchRoutineSuccessAndUrgentBypasses(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	manager := NewManager("test", eventBus, sqliteStore, NewTestSinkAdapter())

	ctx := context.Background()
	target, _, err := seedDigestPreferenceState(ctx, manager)
	if err != nil {
		t.Fatalf("seedDigestPreferenceState returned error: %v", err)
	}

	first, err := manager.EmitOutcome(ctx, OutcomeInput{
		SourceKind:     "run",
		SourceID:       "run_success_1",
		RunID:          "run_success_1",
		ResultClass:    ResultClassRoutineSuccess,
		PayloadPreview: "routine success 1",
	})
	if err != nil {
		t.Fatalf("EmitOutcome(first) returned error: %v", err)
	}
	second, err := manager.EmitOutcome(ctx, OutcomeInput{
		SourceKind:     "run",
		SourceID:       "run_success_2",
		RunID:          "run_success_2",
		ResultClass:    ResultClassRoutineSuccess,
		PayloadPreview: "routine success 2",
	})
	if err != nil {
		t.Fatalf("EmitOutcome(second) returned error: %v", err)
	}
	urgent, err := manager.EmitOutcome(ctx, OutcomeInput{
		SourceKind:     "run",
		SourceID:       "run_urgent",
		RunID:          "run_urgent",
		ResultClass:    ResultClassUrgent,
		PayloadPreview: "urgent",
	})
	if err != nil {
		t.Fatalf("EmitOutcome(urgent) returned error: %v", err)
	}

	if first.Mode != DeliveryModeDigest || second.Mode != DeliveryModeDigest {
		t.Fatalf("expected routine successes to queue for digest, got first=%+v second=%+v", first, second)
	}
	if first.SummaryWindowID == "" || first.SummaryWindowID != second.SummaryWindowID {
		t.Fatalf("expected routine successes to share one summary window, got %q and %q", first.SummaryWindowID, second.SummaryWindowID)
	}
	if urgent.Status != OutcomeStatusDelivered || urgent.Mode != DeliveryModeImmediate {
		t.Fatalf("expected urgent result to bypass digest, got %+v", urgent)
	}

	window, ok, err := manager.GetSummaryWindow(ctx, first.SummaryWindowID)
	if err != nil || !ok {
		t.Fatalf("GetSummaryWindow returned ok=%v err=%v", ok, err)
	}
	window.WindowEndsAt = time.Now().UTC().Add(20 * time.Millisecond)
	window.UpdatedAt = time.Now().UTC()
	if err := manager.storeWindow(ctx, window); err != nil {
		t.Fatalf("storeWindow returned error: %v", err)
	}
	manager.clearWindowSchedule(window.SummaryWindowID)
	manager.scheduleWindow(window.SummaryWindowID, window.WindowEndsAt)

	waitForWindowStatus(t, manager, window.SummaryWindowID, SummaryWindowStatusDelivered)
	deliveredWindow, _, err := manager.GetSummaryWindow(ctx, window.SummaryWindowID)
	if err != nil {
		t.Fatalf("GetSummaryWindow(final) returned error: %v", err)
	}
	if deliveredWindow.EmittedDeliveryID == "" {
		t.Fatalf("expected emitted digest delivery id, got %+v", deliveredWindow)
	}
	digestOutcome, ok, err := manager.GetOutcome(ctx, deliveredWindow.EmittedDeliveryID)
	if err != nil || !ok {
		t.Fatalf("GetOutcome(digest) returned ok=%v err=%v", ok, err)
	}
	if digestOutcome.Status != OutcomeStatusDelivered || digestOutcome.ChosenTargetID != target.TargetID {
		t.Fatalf("expected delivered digest outcome on target %s, got %+v", target.TargetID, digestOutcome)
	}
}

func seedDigestPreferenceState(ctx context.Context, manager *Manager) (DeliveryTarget, DeliveryPreference, error) {
	target, err := manager.CreateTarget(ctx, DeliveryTarget{
		TargetID:         "digest-target",
		DisplayName:      "Digest Target",
		TargetKind:       TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		return DeliveryTarget{}, DeliveryPreference{}, err
	}
	pref, err := manager.UpsertPreference(ctx, DeliveryPreference{
		PreferenceID:     "pref-digest",
		EnvironmentScope: "test",
		ScopeKind:        PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[ResultClass]string{
			ResultClassRoutineSuccess: target.TargetID,
			ResultClassUrgent:         target.TargetID,
			ResultClassFailure:        target.TargetID,
		},
		SummaryPolicy: SummaryPolicy{
			RoutineSuccessMode: DeliveryModeDigest,
			WindowMinutes:      1,
		},
	})
	return target, pref, err
}

func waitForWindowStatus(t *testing.T, manager *Manager, summaryWindowID string, expected SummaryWindowStatus) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		window, ok, err := manager.GetSummaryWindow(context.Background(), summaryWindowID)
		if err != nil {
			t.Fatalf("GetSummaryWindow returned error: %v", err)
		}
		if ok && window.Status == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	window, _, _ := manager.GetSummaryWindow(context.Background(), summaryWindowID)
	t.Fatalf("summary window %s did not reach %s, last=%+v", summaryWindowID, expected, window)
}
