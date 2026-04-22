package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type scriptedAdapter struct {
	targetKind TargetKind
	results    []error
	sends      int
}

func (a *scriptedAdapter) Supports(kind TargetKind) bool {
	return kind == a.targetKind
}

func (a *scriptedAdapter) Send(_ context.Context, _ DeliveryTarget, _ DeliveryOutcome) (SendResult, error) {
	idx := a.sends
	a.sends++
	if idx < len(a.results) && a.results[idx] != nil {
		return SendResult{TransportKind: string(a.targetKind)}, a.results[idx]
	}
	return SendResult{TransportKind: string(a.targetKind), ReceiptSummary: "ok"}, nil
}

func TestDeliveryRetriesWithoutFailoverAndRetainsAttemptHistory(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	adapter := &scriptedAdapter{
		targetKind: TargetKindTestSink,
		results:    []error{errors.New("transient send failure"), nil},
	}
	manager := NewManager("test", events.NewBus(), sqliteStore, adapter)
	manager.baseRetryDelay = 10 * time.Millisecond
	manager.maxRetryDelay = 20 * time.Millisecond
	manager.maxAttempts = 3

	ctx := context.Background()
	primary, pref := seedDeliveryPreferenceState(t, ctx, manager, "primary-target")
	secondary, err := manager.CreateTarget(ctx, DeliveryTarget{
		TargetID:         "secondary-target",
		DisplayName:      "Secondary",
		TargetKind:       TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget(secondary) returned error: %v", err)
	}
	pref.PreferredTargetsByClass[ResultClassFailure] = primary.TargetID
	if _, err := manager.UpsertPreference(ctx, pref); err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}

	outcome, err := manager.EmitOutcome(ctx, OutcomeInput{
		SourceKind:     "run",
		SourceID:       "run_retry",
		RunID:          "run_retry",
		ResultClass:    ResultClassFailure,
		PayloadPreview: "retry me",
	})
	if err != nil {
		t.Fatalf("EmitOutcome returned error: %v", err)
	}
	if outcome.Status != OutcomeStatusQueued {
		t.Fatalf("expected queued status after first retryable failure, got %+v", outcome)
	}

	final := waitForOutcomeStatus(t, manager, outcome.DeliveryID, OutcomeStatusDelivered)
	if len(final.Attempts) != 2 {
		t.Fatalf("expected two attempts, got %+v", final.Attempts)
	}
	if final.Attempts[0].Status != AttemptStatusRetryableFailure {
		t.Fatalf("expected retryable first attempt, got %+v", final.Attempts[0])
	}
	if final.Attempts[1].Status != AttemptStatusDelivered {
		t.Fatalf("expected delivered second attempt, got %+v", final.Attempts[1])
	}
	if final.Attempts[0].TargetID != primary.TargetID || final.Attempts[1].TargetID != primary.TargetID {
		t.Fatalf("expected attempts to stay on chosen target %s, got %+v", primary.TargetID, final.Attempts)
	}
	if final.Attempts[0].TargetID == secondary.TargetID || final.Attempts[1].TargetID == secondary.TargetID {
		t.Fatalf("expected no failover to secondary target, got %+v", final.Attempts)
	}
}

func TestDeliveryRestoreResumesQueuedAttempt(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	firstManager := NewManager("test", events.NewBus(), sqliteStore, &scriptedAdapter{
		targetKind: TargetKindTestSink,
		results:    []error{errors.New("transient send failure")},
	})
	firstManager.baseRetryDelay = time.Hour
	firstManager.maxAttempts = 3
	target, _ := seedDeliveryPreferenceState(t, ctx, firstManager, "restore-target")

	outcome, err := firstManager.EmitOutcome(ctx, OutcomeInput{
		SourceKind:     "run",
		SourceID:       "run_restore",
		RunID:          "run_restore",
		ResultClass:    ResultClassFailure,
		PayloadPreview: "restore me",
	})
	if err != nil {
		t.Fatalf("EmitOutcome returned error: %v", err)
	}
	if outcome.Status != OutcomeStatusQueued {
		t.Fatalf("expected queued retryable outcome, got %+v", outcome)
	}
	outcome, ok, err := firstManager.GetOutcome(ctx, outcome.DeliveryID)
	if err != nil || !ok {
		t.Fatalf("GetOutcome returned ok=%v err=%v", ok, err)
	}
	attempt := outcome.Attempts[0]
	retrySoon := time.Now().UTC().Add(20 * time.Millisecond)
	attempt.NextRetryAt = &retrySoon
	if err := firstManager.storeAttempt(ctx, attempt); err != nil {
		t.Fatalf("storeAttempt returned error: %v", err)
	}

	secondAdapter := &scriptedAdapter{targetKind: TargetKindTestSink}
	secondManager := NewManager("test", events.NewBus(), sqliteStore, secondAdapter)
	secondManager.baseRetryDelay = 10 * time.Millisecond
	secondManager.maxAttempts = 3
	if err := secondManager.Restore(ctx); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	final := waitForOutcomeStatus(t, secondManager, outcome.DeliveryID, OutcomeStatusDelivered)
	if secondAdapter.sends != 1 {
		t.Fatalf("expected one resumed send after restore, got %d", secondAdapter.sends)
	}
	if len(final.Attempts) != 2 {
		t.Fatalf("expected retained attempt history across restore, got %+v", final.Attempts)
	}
	if final.Attempts[1].TargetID != target.TargetID {
		t.Fatalf("expected resumed attempt to stay on target %s, got %+v", target.TargetID, final.Attempts[1])
	}
}

func seedDeliveryPreferenceState(t *testing.T, ctx context.Context, manager *Manager, targetID string) (DeliveryTarget, DeliveryPreference) {
	t.Helper()

	target, err := manager.CreateTarget(ctx, DeliveryTarget{
		TargetID:         targetID,
		DisplayName:      "Primary",
		TargetKind:       TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	pref, err := manager.UpsertPreference(ctx, DeliveryPreference{
		PreferenceID:     "pref-default",
		EnvironmentScope: "test",
		ScopeKind:        PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[ResultClass]string{
			ResultClassRoutineSuccess: target.TargetID,
			ResultClassUrgent:         target.TargetID,
			ResultClassFailure:        target.TargetID,
		},
	})
	if err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}
	return target, pref
}

func waitForOutcomeStatus(t *testing.T, manager *Manager, deliveryID string, expected OutcomeStatus) DeliveryOutcome {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		outcome, ok, err := manager.GetOutcome(context.Background(), deliveryID)
		if err != nil {
			t.Fatalf("GetOutcome returned error: %v", err)
		}
		if ok && outcome.Status == expected {
			return outcome
		}
		time.Sleep(10 * time.Millisecond)
	}
	outcome, _, _ := manager.GetOutcome(context.Background(), deliveryID)
	t.Fatalf("delivery %s did not reach %s, last=%+v", deliveryID, expected, outcome)
	return DeliveryOutcome{}
}
