package reminders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Set(now time.Time) {
	c.now = now.UTC()
}

type fakeWorkflowLauncher struct {
	result WorkflowLaunchResult
	err    error
}

func (l fakeWorkflowLauncher) LaunchReminderWorkflow(_ context.Context, _ WorkflowLaunchConfig, _, _ string) (WorkflowLaunchResult, error) {
	if l.err != nil {
		return WorkflowLaunchResult{}, l.err
	}
	return l.result, nil
}

func TestManagerTickCreatesDueOccurrenceAndLinksDeliveryOutcome(t *testing.T) {
	t.Parallel()

	manager, deliveryManager, clock := newReminderManagerHarness(t, reminderManagerHarnessOptions{})
	ctx := context.Background()

	dueAt := time.Date(2026, 4, 23, 10, 5, 0, 0, time.UTC)
	clock.Set(dueAt.Add(-time.Minute))
	reminder, err := manager.Create(ctx, CreateInput{
		Title:        "Send digest review",
		BehaviorMode: BehaviorModeNotifyOnly,
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: ptrTime(dueAt),
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	clock.Set(dueAt)
	if err := manager.Tick(ctx); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}

	updated, ok, err := manager.Get(ctx, reminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get returned ok=%v err=%v", ok, err)
	}
	if updated.CurrentState != StateDue {
		t.Fatalf("expected reminder state due, got %+v", updated)
	}
	if updated.ActiveOccurrenceID == "" {
		t.Fatalf("expected active occurrence id, got %+v", updated)
	}

	occurrence, ok, err := manager.GetOccurrence(ctx, updated.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence returned ok=%v err=%v", ok, err)
	}
	if occurrence.State != StateDue {
		t.Fatalf("expected occurrence state due, got %+v", occurrence)
	}
	if occurrence.LatestDeliveryID == "" || occurrence.LatestDeliveryStatus != string(delivery.OutcomeStatusDelivered) {
		t.Fatalf("expected delivery linkage on occurrence, got %+v", occurrence)
	}

	outcomes, err := deliveryManager.ListOutcomes(ctx, delivery.OutcomeFilter{
		SourceKind: "reminder_occurrence",
		SourceID:   occurrence.OccurrenceID,
	})
	if err != nil {
		t.Fatalf("ListOutcomes returned error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 delivery outcome, got %d", len(outcomes))
	}
	if outcomes[0].Mode != delivery.DeliveryModeImmediate || outcomes[0].ChosenTargetID == "" {
		t.Fatalf("expected immediate reminder delivery, got %+v", outcomes[0])
	}

	actions, err := manager.ListActions(ctx, reminder.ReminderID)
	if err != nil {
		t.Fatalf("ListActions returned error: %v", err)
	}
	if len(actions) != 3 {
		t.Fatalf("expected created, due, and delivery_linked actions, got %+v", actions)
	}
	actionKinds := map[ActionKind]int{}
	for _, action := range actions {
		actionKinds[action.ActionKind]++
	}
	if actionKinds[ActionKindCreated] != 1 || actionKinds[ActionKindDue] != 1 || actionKinds[ActionKindDeliveryLinked] != 1 {
		t.Fatalf("unexpected action history contents %+v", actions)
	}
}

func TestManagerRecurringRemindersMarkMissedAndPreserveAcknowledgedHistory(t *testing.T) {
	t.Parallel()

	manager, _, clock := newReminderManagerHarness(t, reminderManagerHarnessOptions{})
	ctx := context.Background()

	start := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	clock.Set(start)
	reminder, err := manager.Create(ctx, CreateInput{
		Title:        "Recurring follow-up",
		BehaviorMode: BehaviorModeNotifyOnly,
		Trigger: scheduler.Trigger{
			Kind:     scheduler.TriggerKindCron,
			CronExpr: "*/1 * * * *",
			Timezone: "UTC",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	clock.Set(start.Add(time.Minute))
	if err := manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(first due) returned error: %v", err)
	}
	firstReminder, _, err := manager.Get(ctx, reminder.ReminderID)
	if err != nil {
		t.Fatalf("Get(first) returned error: %v", err)
	}
	firstOccurrenceID := firstReminder.ActiveOccurrenceID
	if firstOccurrenceID == "" {
		t.Fatalf("expected first occurrence, got %+v", firstReminder)
	}

	clock.Set(start.Add(2 * time.Minute))
	if err := manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(overdue) returned error: %v", err)
	}
	if err := manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(rollover) returned error: %v", err)
	}
	firstOccurrence, ok, err := manager.GetOccurrence(ctx, firstOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence(first) returned ok=%v err=%v", ok, err)
	}
	if firstOccurrence.State != StateMissed {
		t.Fatalf("expected first occurrence missed, got %+v", firstOccurrence)
	}

	secondReminder, ok, err := manager.Get(ctx, reminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(second reminder) returned ok=%v err=%v", ok, err)
	}
	secondOccurrenceID := secondReminder.ActiveOccurrenceID
	if secondOccurrenceID == "" || secondOccurrenceID == firstOccurrenceID {
		t.Fatalf("expected rollover to new occurrence, got %+v", secondReminder)
	}

	_, secondOccurrence, _, err := manager.Acknowledge(ctx, reminder.ReminderID, TransitionInput{
		OccurrenceID: secondOccurrenceID,
		ActorKind:    ActorKindUser,
		Reason:       "seen",
	})
	if err != nil {
		t.Fatalf("Acknowledge returned error: %v", err)
	}
	if secondOccurrence.State != StateAcknowledged {
		t.Fatalf("expected second occurrence acknowledged, got %+v", secondOccurrence)
	}

	clock.Set(start.Add(3 * time.Minute))
	if err := manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(third due) returned error: %v", err)
	}
	finalReminder, ok, err := manager.Get(ctx, reminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(final reminder) returned ok=%v err=%v", ok, err)
	}
	if finalReminder.ActiveOccurrenceID == "" || finalReminder.ActiveOccurrenceID == secondOccurrenceID {
		t.Fatalf("expected new active occurrence after acknowledged history, got %+v", finalReminder)
	}
	thirdOccurrence, ok, err := manager.GetOccurrence(ctx, finalReminder.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence(third) returned ok=%v err=%v", ok, err)
	}
	if thirdOccurrence.State != StateDue {
		t.Fatalf("expected third occurrence due, got %+v", thirdOccurrence)
	}

	preservedSecond, ok, err := manager.GetOccurrence(ctx, secondOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence(acknowledged history) returned ok=%v err=%v", ok, err)
	}
	if preservedSecond.State != StateAcknowledged {
		t.Fatalf("expected acknowledged history preserved, got %+v", preservedSecond)
	}
}

func TestManagerWorkflowLinkedReminderAcknowledgesOnSuccessAndStaysDueOnFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dueAt := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)

	successManager, _, successClock := newReminderManagerHarness(t, reminderManagerHarnessOptions{
		workflowLauncher: fakeWorkflowLauncher{
			result: WorkflowLaunchResult{RunID: "run_reminder_1", WorkflowID: "wf_reminder_1"},
		},
	})
	successClock.Set(dueAt.Add(-time.Minute))
	successReminder, err := successManager.Create(ctx, CreateInput{
		Title:        "Launch follow-up workflow",
		BehaviorMode: BehaviorModeLaunchWorkflow,
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: ptrTime(dueAt),
		},
		WorkflowLaunchConfig: &WorkflowLaunchConfig{
			Entrypoint: "operator",
		},
	})
	if err != nil {
		t.Fatalf("Create(success) returned error: %v", err)
	}
	successClock.Set(dueAt)
	if err := successManager.Tick(ctx); err != nil {
		t.Fatalf("Tick(success) returned error: %v", err)
	}
	successCurrent, ok, err := successManager.Get(ctx, successReminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(success) returned ok=%v err=%v", ok, err)
	}
	if successCurrent.CurrentState != StateAcknowledged {
		t.Fatalf("expected acknowledged reminder after workflow launch, got %+v", successCurrent)
	}
	successOccurrence, ok, err := successManager.GetOccurrence(ctx, successCurrent.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence(success) returned ok=%v err=%v", ok, err)
	}
	if successOccurrence.State != StateAcknowledged || successOccurrence.RunID != "run_reminder_1" || successOccurrence.WorkflowID != "wf_reminder_1" {
		t.Fatalf("expected linked acknowledged occurrence, got %+v", successOccurrence)
	}

	failureManager, _, failureClock := newReminderManagerHarness(t, reminderManagerHarnessOptions{
		workflowLauncher: fakeWorkflowLauncher{err: errors.New("launch failed")},
	})
	failureClock.Set(dueAt.Add(-time.Minute))
	failureReminder, err := failureManager.Create(ctx, CreateInput{
		Title:        "Fail workflow launch",
		BehaviorMode: BehaviorModeLaunchWorkflow,
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: ptrTime(dueAt),
		},
		WorkflowLaunchConfig: &WorkflowLaunchConfig{
			Entrypoint: "operator",
		},
	})
	if err != nil {
		t.Fatalf("Create(failure) returned error: %v", err)
	}
	failureClock.Set(dueAt)
	if err := failureManager.Tick(ctx); err != nil {
		t.Fatalf("Tick(failure due) returned error: %v", err)
	}
	failureCurrent, ok, err := failureManager.Get(ctx, failureReminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(failure due) returned ok=%v err=%v", ok, err)
	}
	failureOccurrence, ok, err := failureManager.GetOccurrence(ctx, failureCurrent.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence(failure due) returned ok=%v err=%v", ok, err)
	}
	if failureOccurrence.State != StateDue {
		t.Fatalf("expected occurrence to remain due after launch failure, got %+v", failureOccurrence)
	}
	failureClock.Set(dueAt.Add(20 * time.Millisecond))
	if err := failureManager.Tick(ctx); err != nil {
		t.Fatalf("Tick(failure overdue) returned error: %v", err)
	}
	failureOccurrence, ok, err = failureManager.GetOccurrence(ctx, failureCurrent.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence(failure overdue) returned ok=%v err=%v", ok, err)
	}
	if failureOccurrence.State != StateOverdue {
		t.Fatalf("expected occurrence overdue after unhandled launch failure, got %+v", failureOccurrence)
	}
}

func TestManagerRefreshesFollowUpLinkStaleness(t *testing.T) {
	t.Parallel()

	manager, _, clock := newReminderManagerHarness(t, reminderManagerHarnessOptions{})
	ctx := context.Background()
	clock.Set(time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC))

	session := router.Session{
		SessionID:    "session_existing",
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "chat",
		RoutingKey:   "direct:local:local:chat",
		Generation:   1,
		CreatedAt:    clock.Now(),
		UpdatedAt:    clock.Now(),
		LastActiveAt: clock.Now(),
	}
	if err := manager.store.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}

	runSeed := runtime.Run{
		RunID:      "run_existing",
		SessionID:  session.SessionID,
		Entrypoint: "operator",
		Status:     runtime.RunStatusCompleted,
		Goal:       "existing work",
		CreatedAt:  clock.Now(),
		UpdatedAt:  clock.Now(),
	}
	if err := manager.store.UpsertRun(ctx, runSeed); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	existingRunReminder, err := manager.Create(ctx, CreateInput{
		Title:        "Follow up existing run",
		BehaviorMode: BehaviorModeNotifyOnly,
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: ptrTime(clock.Now().Add(time.Hour)),
		},
		FollowUpLink: &FollowUpLink{
			LinkKind: FollowUpLinkKindRun,
			SourceID: runSeed.RunID,
		},
	})
	if err != nil {
		t.Fatalf("Create(existing run) returned error: %v", err)
	}
	refreshedRunReminder, ok, err := manager.Get(ctx, existingRunReminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(existing run) returned ok=%v err=%v", ok, err)
	}
	if refreshedRunReminder.FollowUpLink == nil || refreshedRunReminder.FollowUpLink.Stale {
		t.Fatalf("expected existing run follow-up link to stay fresh, got %+v", refreshedRunReminder.FollowUpLink)
	}

	missingWorkflowReminder, err := manager.Create(ctx, CreateInput{
		Title:        "Follow up missing workflow",
		BehaviorMode: BehaviorModeNotifyOnly,
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: ptrTime(clock.Now().Add(2 * time.Hour)),
		},
		FollowUpLink: &FollowUpLink{
			LinkKind: FollowUpLinkKindWorkflow,
			SourceID: "wf_missing",
		},
	})
	if err != nil {
		t.Fatalf("Create(missing workflow) returned error: %v", err)
	}
	refreshedWorkflowReminder, ok, err := manager.Get(ctx, missingWorkflowReminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(missing workflow) returned ok=%v err=%v", ok, err)
	}
	if refreshedWorkflowReminder.FollowUpLink == nil || !refreshedWorkflowReminder.FollowUpLink.Stale {
		t.Fatalf("expected missing workflow follow-up link to be stale, got %+v", refreshedWorkflowReminder.FollowUpLink)
	}
	if refreshedWorkflowReminder.FollowUpLink.SourceDisplayState != "stale" || refreshedWorkflowReminder.FollowUpLink.LastCheckedAt == nil {
		t.Fatalf("expected stale projection metadata, got %+v", refreshedWorkflowReminder.FollowUpLink)
	}
}

func TestManagerPerformanceSmoke(t *testing.T) {
	t.Parallel()

	manager, _, clock := newReminderManagerHarness(t, reminderManagerHarnessOptions{})
	ctx := context.Background()
	base := time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC)
	dueAt := base.Add(time.Minute)
	clock.Set(base)

	for i := 0; i < 100; i++ {
		if _, err := manager.Create(ctx, CreateInput{
			Title:        "Perf reminder",
			BehaviorMode: BehaviorModeNotifyOnly,
			Trigger: scheduler.Trigger{
				Kind:   scheduler.TriggerKindOnce,
				FireAt: ptrTime(dueAt),
			},
		}); err != nil {
			t.Fatalf("Create(%d) returned error: %v", i, err)
		}
	}

	listStarted := time.Now()
	items, err := manager.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	listElapsed := time.Since(listStarted)
	if len(items) != 100 {
		t.Fatalf("expected 100 reminders, got %d", len(items))
	}
	if listElapsed > 500*time.Millisecond {
		t.Fatalf("expected reminder inspect smoke under 500ms, got %s", listElapsed)
	}

	clock.Set(dueAt)
	tickStarted := time.Now()
	if err := manager.Tick(ctx); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	tickElapsed := time.Since(tickStarted)
	if tickElapsed > time.Second {
		t.Fatalf("expected due detection smoke under 1s, got %s", tickElapsed)
	}

	first, ok, err := manager.Get(ctx, items[0].ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get returned ok=%v err=%v", ok, err)
	}
	occurrenceStarted := time.Now()
	occurrence, ok, err := manager.GetOccurrence(ctx, first.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence returned ok=%v err=%v", ok, err)
	}
	occurrenceElapsed := time.Since(occurrenceStarted)
	if occurrence.LatestDeliveryID == "" {
		t.Fatalf("expected delivery linkage on occurrence, got %+v", occurrence)
	}
	if occurrenceElapsed > 500*time.Millisecond {
		t.Fatalf("expected occurrence projection smoke under 500ms, got %s", occurrenceElapsed)
	}

	ackStarted := time.Now()
	_, acknowledged, _, err := manager.Acknowledge(ctx, first.ReminderID, TransitionInput{OccurrenceID: first.ActiveOccurrenceID})
	if err != nil {
		t.Fatalf("Acknowledge returned error: %v", err)
	}
	ackElapsed := time.Since(ackStarted)
	if acknowledged.State != StateAcknowledged {
		t.Fatalf("expected acknowledged occurrence, got %+v", acknowledged)
	}
	if ackElapsed > time.Second {
		t.Fatalf("expected occurrence transition persistence smoke under 1s, got %s", ackElapsed)
	}

	t.Logf("reminder performance smoke: inspect=%s due_tick=%s occurrence_projection=%s acknowledge=%s", listElapsed, tickElapsed, occurrenceElapsed, ackElapsed)
}

type reminderManagerHarnessOptions struct {
	summaryPolicy    *delivery.SummaryPolicy
	workflowLauncher WorkflowLauncher
	tickInterval     time.Duration
}

func newReminderManagerHarness(t *testing.T, opts reminderManagerHarnessOptions) (*Manager, *delivery.Manager, *fakeClock) {
	t.Helper()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	// Roadmap 35 (Pass B): bootstrap a personal tenant + seed the
	// default-tenant cache so the legacy Upsert* helpers used by this
	// test harness bind tenant_id correctly. Without this the reminder
	// follow-up projection would treat run/workflow links as stale (the
	// FR-006 isolation guarantee — there is no visible tenant context).
	bootstrapTestPersonalTenant(t, sqliteStore)

	eventBus := events.NewBus()
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	target, err := deliveryManager.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "reminder-target",
		DisplayName:      "Reminder Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}

	preference := delivery.DeliveryPreference{
		PreferenceID:     "reminder-pref",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassRoutineSuccess: target.TargetID,
			delivery.ResultClassUrgent:         target.TargetID,
			delivery.ResultClassFailure:        target.TargetID,
		},
	}
	if opts.summaryPolicy != nil {
		preference.SummaryPolicy = *opts.summaryPolicy
	}
	if _, err := deliveryManager.UpsertPreference(context.Background(), preference); err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}

	clock := &fakeClock{now: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC)}
	tickInterval := opts.tickInterval
	if tickInterval <= 0 {
		tickInterval = 10 * time.Millisecond
	}
	manager := NewManager(Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		EventBus:         eventBus,
		Delivery:         deliveryManager,
		WorkflowLauncher: opts.workflowLauncher,
		Clock:            clock,
		TickInterval:     tickInterval,
	})
	return manager, deliveryManager, clock
}

// bootstrapTestPersonalTenant inserts a personal tenant row directly
// (bypassing identity.Manager.BootstrapLocal which has wider deps the
// reminder tests do not need) and primes the default-tenant cache so
// that the legacy Upsert* helpers in this test harness bind tenant_id
// correctly. Required after Roadmap 35 Pass B made tenant_id the
// authoritative ownership marker for runtime-spine tables.
func bootstrapTestPersonalTenant(t *testing.T, s *store.SQLiteStore) {
	t.Helper()
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	tenant := identity.Tenant{
		TenantID:    "ten_test_personal",
		TenantKind:  identity.TenantKindPersonal,
		DisplayName: "Test Personal",
		Status:      identity.StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.UpsertTenant(context.Background(), tenant); err != nil {
		t.Fatalf("UpsertTenant: %v", err)
	}
	if err := s.SeedDefaultTenantCache(context.Background()); err != nil {
		t.Fatalf("SeedDefaultTenantCache: %v", err)
	}
}
