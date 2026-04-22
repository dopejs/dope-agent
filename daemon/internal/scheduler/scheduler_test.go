package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestSchedulerDispatchesOneTimeScheduleExactlyOnce(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	harness := newSchedulerHarness(t, now)

	fireAt := now.Add(30 * time.Second)
	schedule, err := harness.scheduler.Create(context.Background(), CreateInput{
		Trigger: Trigger{
			Kind:   TriggerKindOnce,
			FireAt: &fireAt,
		},
		Target: Target{
			Kind: TargetKindRun,
			Run: &RunTarget{
				Entrypoint: "operator",
				Goal:       "dispatch exactly once",
			},
		},
		RetryPolicy: RetryPolicy{MaxRetries: 1, BackoffKind: RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(before due) returned error: %v", err)
	}
	if runs := harness.runtime.ListRuns(); len(runs) != 0 {
		t.Fatalf("expected no runs before due time, got %+v", runs)
	}

	harness.clock.now = fireAt.Add(time.Second)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(after due) returned error: %v", err)
	}
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(after dispatch) returned error: %v", err)
	}

	runs := harness.runtime.ListRuns()
	if len(runs) != 1 {
		t.Fatalf("expected one dispatched run, got %+v", runs)
	}
	if runs[0].ScheduleID != schedule.ScheduleID || runs[0].ScheduleAttemptID == "" {
		t.Fatalf("expected schedule linkage on run, got %+v", runs[0])
	}

	got, ok, err := harness.scheduler.Get(context.Background(), schedule.ScheduleID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected persisted schedule")
	}
	if got.Status != ScheduleStatusCompleted {
		t.Fatalf("expected completed one-time schedule, got %+v", got)
	}
	if len(got.Attempts) != 1 || got.Attempts[0].DispatchStatus != DispatchStatusDispatched {
		t.Fatalf("expected one dispatched attempt, got %+v", got.Attempts)
	}
}

func TestSchedulerCancelPreDispatchPreventsRunAndRecordsVisibleHistory(t *testing.T) {
	now := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	harness := newSchedulerHarness(t, now)

	fireAt := now.Add(30 * time.Second)
	schedule, err := harness.scheduler.Create(context.Background(), CreateInput{
		Trigger: Trigger{
			Kind:   TriggerKindOnce,
			FireAt: &fireAt,
		},
		Target: Target{
			Kind: TargetKindRun,
			Run: &RunTarget{
				Entrypoint: "operator",
				Goal:       "cancel before dispatch",
			},
		},
		RetryPolicy: RetryPolicy{MaxRetries: 0, BackoffKind: RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, ok, err := harness.scheduler.Cancel(context.Background(), schedule.ScheduleID); err != nil || !ok {
		t.Fatalf("Cancel returned ok=%v err=%v", ok, err)
	}

	harness.clock.now = fireAt.Add(time.Second)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}

	if runs := harness.runtime.ListRuns(); len(runs) != 0 {
		t.Fatalf("expected no dispatched runs after cancel, got %+v", runs)
	}
	got, ok, err := harness.scheduler.Get(context.Background(), schedule.ScheduleID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected persisted cancelled schedule")
	}
	if len(got.Attempts) == 0 {
		t.Fatalf("expected visible cancel/skip history, got %+v", got)
	}
	if got.Attempts[0].DispatchStatus != DispatchStatusSkippedCancelled {
		t.Fatalf("expected skipped_cancelled history, got %+v", got.Attempts[0])
	}
}

func TestSchedulerRecurringPauseResumeAndOverlapTruth(t *testing.T) {
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	harness := newSchedulerHarness(t, now)

	schedule, err := harness.scheduler.Create(context.Background(), CreateInput{
		Trigger: Trigger{
			Kind:     TriggerKindCron,
			CronExpr: "*/1 * * * *",
			Timezone: "UTC",
		},
		Target: Target{
			Kind: TargetKindRun,
			Run: &RunTarget{
				Entrypoint: "operator",
				Goal:       "recurring dispatch",
			},
		},
		RetryPolicy: RetryPolicy{MaxRetries: 0, BackoffKind: RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	harness.clock.now = time.Date(2026, 4, 22, 12, 1, 1, 0, time.UTC)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(first due) returned error: %v", err)
	}
	runs := harness.runtime.ListRuns()
	if len(runs) != 1 {
		t.Fatalf("expected first recurring run, got %+v", runs)
	}

	harness.clock.now = time.Date(2026, 4, 22, 12, 2, 1, 0, time.UTC)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(overlap due) returned error: %v", err)
	}
	if runs = harness.runtime.ListRuns(); len(runs) != 1 {
		t.Fatalf("expected overlap to skip without new run, got %+v", runs)
	}

	got, ok, err := harness.scheduler.Get(context.Background(), schedule.ScheduleID)
	if err != nil || !ok {
		t.Fatalf("Get returned ok=%v err=%v", ok, err)
	}
	if len(got.Attempts) < 2 || got.Attempts[0].DispatchStatus != DispatchStatusSkippedOverlap {
		t.Fatalf("expected visible skipped_overlap history, got %+v", got.Attempts)
	}

	completeRunForSchedulerTest(t, harness.runtime, runs[0].RunID)

	if _, ok, err := harness.scheduler.Pause(context.Background(), schedule.ScheduleID); err != nil || !ok {
		t.Fatalf("Pause returned ok=%v err=%v", ok, err)
	}
	harness.clock.now = time.Date(2026, 4, 22, 12, 3, 1, 0, time.UTC)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(paused due) returned error: %v", err)
	}

	got, ok, err = harness.scheduler.Get(context.Background(), schedule.ScheduleID)
	if err != nil || !ok {
		t.Fatalf("Get(after pause) returned ok=%v err=%v", ok, err)
	}
	if got.Attempts[0].DispatchStatus != DispatchStatusSkippedPaused {
		t.Fatalf("expected skipped_paused history, got %+v", got.Attempts[0])
	}

	harness.clock.now = time.Date(2026, 4, 22, 12, 3, 10, 0, time.UTC)
	resumed, ok, err := harness.scheduler.Resume(context.Background(), schedule.ScheduleID)
	if err != nil || !ok {
		t.Fatalf("Resume returned ok=%v err=%v", ok, err)
	}
	if resumed.Status != ScheduleStatusActive || resumed.NextDueAt == nil || !resumed.NextDueAt.After(harness.clock.now) {
		t.Fatalf("expected resumed recurring schedule with future next due, got %+v", resumed)
	}

	harness.clock.now = time.Date(2026, 4, 22, 12, 4, 1, 0, time.UTC)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(resumed due) returned error: %v", err)
	}
	if runs = harness.runtime.ListRuns(); len(runs) != 2 {
		t.Fatalf("expected second recurring run after resume, got %+v", runs)
	}
}

func TestSchedulerRetryAndExhaustedTruthForDispatchFailure(t *testing.T) {
	now := time.Date(2026, 4, 22, 13, 0, 0, 0, time.UTC)
	harness := newSchedulerHarness(t, now)

	fireAt := now.Add(30 * time.Second)
	schedule, err := harness.scheduler.Create(context.Background(), CreateInput{
		Trigger: Trigger{
			Kind:   TriggerKindOnce,
			FireAt: &fireAt,
		},
		Target: Target{
			Kind: TargetKindRun,
			Run: &RunTarget{
				Entrypoint: "operator",
				Goal:       "retry dispatch failure",
			},
		},
		RetryPolicy: RetryPolicy{MaxRetries: 1, BackoffKind: RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	targetRecord, ok, err := harness.store.GetScheduleTarget(context.Background(), schedule.ScheduleID, schedule.TargetRefID)
	if err != nil || !ok {
		t.Fatalf("GetScheduleTarget returned ok=%v err=%v", ok, err)
	}
	targetRecord.Active = false
	if err := harness.store.UpsertScheduleTarget(context.Background(), targetRecord); err != nil {
		t.Fatalf("UpsertScheduleTarget returned error: %v", err)
	}

	harness.clock.now = fireAt.Add(time.Second)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(first failure) returned error: %v", err)
	}

	got, ok, err := harness.scheduler.Get(context.Background(), schedule.ScheduleID)
	if err != nil || !ok {
		t.Fatalf("Get(after first failure) returned ok=%v err=%v", ok, err)
	}
	if len(got.Attempts) != 1 || got.Attempts[0].DispatchStatus != DispatchStatusFailed || got.Attempts[0].NextRetryAt == nil {
		t.Fatalf("expected retryable dispatch failure, got %+v", got.Attempts)
	}
	if len(harness.runtime.ListRuns()) != 0 {
		t.Fatalf("expected no downstream run on dispatch failure, got %+v", harness.runtime.ListRuns())
	}

	harness.clock.now = got.Attempts[0].NextRetryAt.Add(time.Second)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick(exhaust retry) returned error: %v", err)
	}
	got, ok, err = harness.scheduler.Get(context.Background(), schedule.ScheduleID)
	if err != nil || !ok {
		t.Fatalf("Get(after retry) returned ok=%v err=%v", ok, err)
	}
	if got.Attempts[0].DispatchStatus != DispatchStatusExhausted {
		t.Fatalf("expected exhausted attempt, got %+v", got.Attempts[0])
	}
	if got.Status != ScheduleStatusDispatchFailed {
		t.Fatalf("expected one-time schedule dispatch_failed, got %+v", got)
	}
}

func TestSchedulerDueDetectionStaysUnder1SecondFor100Schedules(t *testing.T) {
	now := time.Date(2026, 4, 22, 15, 0, 0, 0, time.UTC)
	harness := newSchedulerHarness(t, now)

	fireAt := now.Add(30 * time.Second)
	for idx := 0; idx < 100; idx++ {
		_, err := harness.scheduler.Create(context.Background(), CreateInput{
			Trigger: Trigger{
				Kind:   TriggerKindOnce,
				FireAt: &fireAt,
			},
			Target: Target{
				Kind: TargetKindRun,
				Run: &RunTarget{
					Entrypoint: "operator",
					Goal:       "bulk due detection",
				},
			},
			RetryPolicy: RetryPolicy{MaxRetries: 0, BackoffKind: RetryBackoffFixed, BaseDelaySeconds: 5, MaxDelaySeconds: 5},
		})
		if err != nil {
			t.Fatalf("Create(%d) returned error: %v", idx, err)
		}
	}

	harness.clock.now = fireAt.Add(time.Second)
	started := time.Now()
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	elapsed := time.Since(started)
	if elapsed > time.Second {
		t.Fatalf("expected due detection under 1s, got %s", elapsed)
	}
	if runs := harness.runtime.ListRuns(); len(runs) != 100 {
		t.Fatalf("expected 100 dispatched runs, got %d", len(runs))
	}
}

type schedulerHarness struct {
	clock     *fakeClock
	store     *store.SQLiteStore
	runtime   *runtime.Manager
	scheduler *Scheduler
}

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time {
	return f.now
}

func newSchedulerHarness(t *testing.T, now time.Time) schedulerHarness {
	t.Helper()

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(t.TempDir(), "dope-data"),
	}
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	runtimeManager := runtime.NewManager()
	clock := &fakeClock{now: now}
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	scheduleManager := New(Dependencies{
		Config:       cfg,
		Runtime:      runtimeManager,
		EventBus:     events.NewBus(),
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
		Clock:        clock,
		TickInterval: 10 * time.Millisecond,
	})
	return schedulerHarness{
		clock:     clock,
		store:     sqliteStore,
		runtime:   runtimeManager,
		scheduler: scheduleManager,
	}
}

func completeRunForSchedulerTest(t *testing.T, runtimeManager *runtime.Manager, runID string) {
	t.Helper()

	step, err := runtimeManager.CreateStep(runID, runtime.CreateStepInput{
		Title: "complete scheduled run",
		Kind:  "task",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	if _, _, err := runtimeManager.UpdateStepStatusAndReconcileRun(runID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
	}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(planning) returned error: %v", err)
	}
	if _, _, err := runtimeManager.UpdateStepStatusAndReconcileRun(runID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusExecutingTool,
	}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(executing_tool) returned error: %v", err)
	}
	if _, _, err := runtimeManager.UpdateStepStatusAndReconcileRun(runID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusCompleted,
		Output: map[string]any{"ok": true},
	}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
}
