package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/reminders"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

type reminderTestClock struct {
	now time.Time
}

func (c *reminderTestClock) Now() time.Time {
	return c.now
}

func (c *reminderTestClock) Set(now time.Time) {
	c.now = now.UTC()
}

type reminderWorkflowLauncherStub struct {
	result reminders.WorkflowLaunchResult
	err    error
}

func (l reminderWorkflowLauncherStub) LaunchReminderWorkflow(_ context.Context, _ reminders.WorkflowLaunchConfig, _, _ string) (reminders.WorkflowLaunchResult, error) {
	if l.err != nil {
		return reminders.WorkflowLaunchResult{}, l.err
	}
	return l.result, nil
}

type reminderServerHarness struct {
	server      *Server
	manager     *reminders.Manager
	delivery    *delivery.Manager
	clock       *reminderTestClock
	sqliteStore *store.SQLiteStore
}

type reminderServerHarnessOptions struct {
	summaryPolicy    *delivery.SummaryPolicy
	workflowLauncher reminders.WorkflowLauncher
}

type reminderTransitionResponse struct {
	Reminder   reminders.Reminder     `json:"reminder"`
	Occurrence reminders.Occurrence   `json:"occurrence"`
	Action     reminders.ActionRecord `json:"action"`
}

func TestReminderRoutesCreateInspectOccurrencesAndActions(t *testing.T) {
	t.Parallel()

	harness := newReminderServerHarness(t, reminderServerHarnessOptions{})
	dueAt := time.Date(2026, 4, 23, 10, 5, 0, 0, time.UTC)
	harness.clock.Set(dueAt.Add(-time.Minute))

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Check nightly backup","details":"Inspect last run","trigger":{"kind":"once","fireAt":"2026-04-23T10:05:00Z"}}`))
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[reminders.Reminder](t, createRec.Body.Bytes())
	if created.BehaviorMode != reminders.BehaviorModeNotifyOnly || created.CurrentState != reminders.StatePending {
		t.Fatalf("expected pending notify_only reminder, got %+v", created)
	}

	listRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/v1/reminders", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	list := decodeStrictResponse[ReminderListResponse](t, listRec.Body.Bytes())
	if len(list.Items) != 1 || list.Items[0].ReminderID != created.ReminderID {
		t.Fatalf("expected created reminder in list, got %+v", list)
	}

	harness.clock.Set(dueAt)
	if err := harness.manager.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}

	detailRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(detailRec, httptest.NewRequest(http.MethodGet, "/v1/reminders/"+created.ReminderID, nil))
	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", detailRec.Code, detailRec.Body.String())
	}
	detail := decodeStrictResponse[reminders.Reminder](t, detailRec.Body.Bytes())
	if detail.CurrentState != reminders.StateDue || detail.ActiveOccurrenceID == "" {
		t.Fatalf("expected due reminder with occurrence, got %+v", detail)
	}

	occListRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(occListRec, httptest.NewRequest(http.MethodGet, "/v1/reminders/occurrences?reminderId="+created.ReminderID, nil))
	if occListRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", occListRec.Code, occListRec.Body.String())
	}
	occList := decodeStrictResponse[ReminderOccurrenceListResponse](t, occListRec.Body.Bytes())
	if len(occList.Items) != 1 {
		t.Fatalf("expected one occurrence, got %+v", occList)
	}
	if occList.Items[0].LatestDeliveryID == "" || occList.Items[0].LatestDeliveryStatus != string(delivery.OutcomeStatusDelivered) {
		t.Fatalf("expected linked delivery summary, got %+v", occList.Items[0])
	}

	actionRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(actionRec, httptest.NewRequest(http.MethodGet, "/v1/reminders/"+created.ReminderID+"/actions", nil))
	if actionRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", actionRec.Code, actionRec.Body.String())
	}
	actions := decodeStrictResponse[ReminderActionListResponse](t, actionRec.Body.Bytes())
	if len(actions.Items) != 3 {
		t.Fatalf("expected created, due, and delivery-linked actions, got %+v", actions.Items)
	}
}

func TestReminderRoutesReuseDigestDeliveryPreference(t *testing.T) {
	t.Parallel()

	harness := newReminderServerHarness(t, reminderServerHarnessOptions{
		summaryPolicy: &delivery.SummaryPolicy{
			RoutineSuccessMode: delivery.DeliveryModeDigest,
			WindowMinutes:      5,
		},
	})
	dueAt := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)
	harness.clock.Set(dueAt.Add(-time.Minute))

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Digest me","trigger":{"kind":"once","fireAt":"2026-04-23T11:00:00Z"}}`))
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[reminders.Reminder](t, createRec.Body.Bytes())

	harness.clock.Set(dueAt)
	if err := harness.manager.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	current, ok, err := harness.manager.Get(context.Background(), created.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get returned ok=%v err=%v", ok, err)
	}
	occurrence, ok, err := harness.manager.GetOccurrence(context.Background(), current.ActiveOccurrenceID)
	if err != nil || !ok {
		t.Fatalf("GetOccurrence returned ok=%v err=%v", ok, err)
	}

	outcomes, err := harness.delivery.ListOutcomes(context.Background(), delivery.OutcomeFilter{
		SourceKind: "reminder_occurrence",
		SourceID:   occurrence.OccurrenceID,
	})
	if err != nil {
		t.Fatalf("ListOutcomes returned error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected one digest outcome, got %+v", outcomes)
	}
	if outcomes[0].Mode != delivery.DeliveryModeDigest || outcomes[0].SummaryWindowID == "" {
		t.Fatalf("expected reminder delivery to reuse digest preference, got %+v", outcomes[0])
	}
}

func TestReminderLifecycleRoutesAndWorkflowLinkage(t *testing.T) {
	t.Parallel()

	harness := newReminderServerHarness(t, reminderServerHarnessOptions{
		workflowLauncher: reminderWorkflowLauncherStub{
			result: reminders.WorkflowLaunchResult{RunID: "run_reminder_api", WorkflowID: "wf_reminder_api"},
		},
	})
	ctx := context.Background()
	base := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)

	harness.clock.Set(base.Add(-time.Minute))
	createLifecycleRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createLifecycleRec, httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Lifecycle reminder","trigger":{"kind":"once","fireAt":"2026-04-23T12:00:00Z"}}`)))
	if createLifecycleRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createLifecycleRec.Code, createLifecycleRec.Body.String())
	}
	lifecycleReminder := decodeStrictResponse[reminders.Reminder](t, createLifecycleRec.Body.Bytes())

	harness.clock.Set(base)
	if err := harness.manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(lifecycle due) returned error: %v", err)
	}
	lifecycleCurrent, ok, err := harness.manager.Get(ctx, lifecycleReminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(lifecycle) returned ok=%v err=%v", ok, err)
	}

	ackRec := httptest.NewRecorder()
	ackReq := httptest.NewRequest(http.MethodPost, "/v1/reminders/"+lifecycleReminder.ReminderID+"/acknowledge", strings.NewReader(`{"occurrenceId":"`+lifecycleCurrent.ActiveOccurrenceID+`","reason":"saw it"}`))
	harness.server.Handler().ServeHTTP(ackRec, ackReq)
	if ackRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", ackRec.Code, ackRec.Body.String())
	}
	acknowledged := decodeStrictResponse[reminderTransitionResponse](t, ackRec.Body.Bytes())
	if acknowledged.Occurrence.State != reminders.StateAcknowledged {
		t.Fatalf("expected acknowledged occurrence, got %+v", acknowledged)
	}

	rescheduleRec := httptest.NewRecorder()
	rescheduleReq := httptest.NewRequest(http.MethodPost, "/v1/reminders/"+lifecycleReminder.ReminderID+"/reschedule", strings.NewReader(`{"trigger":{"kind":"once","fireAt":"2026-04-23T12:30:00Z"},"reason":"later"}`))
	harness.server.Handler().ServeHTTP(rescheduleRec, rescheduleReq)
	if rescheduleRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rescheduleRec.Code, rescheduleRec.Body.String())
	}
	rescheduled := decodeStrictResponse[reminderTransitionResponse](t, rescheduleRec.Body.Bytes())
	if rescheduled.Reminder.CurrentState != reminders.StatePending {
		t.Fatalf("expected pending reminder after reschedule, got %+v", rescheduled)
	}

	snoozeCreateRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(snoozeCreateRec, httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Snooze reminder","trigger":{"kind":"once","fireAt":"2026-04-23T12:01:00Z"}}`)))
	if snoozeCreateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", snoozeCreateRec.Code, snoozeCreateRec.Body.String())
	}
	snoozeReminder := decodeStrictResponse[reminders.Reminder](t, snoozeCreateRec.Body.Bytes())
	harness.clock.Set(base.Add(time.Minute))
	if err := harness.manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(snooze due) returned error: %v", err)
	}
	snoozeCurrent, ok, err := harness.manager.Get(ctx, snoozeReminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(snooze) returned ok=%v err=%v", ok, err)
	}

	snoozeRec := httptest.NewRecorder()
	snoozeReq := httptest.NewRequest(http.MethodPost, "/v1/reminders/"+snoozeReminder.ReminderID+"/snooze", strings.NewReader(`{"occurrenceId":"`+snoozeCurrent.ActiveOccurrenceID+`","snoozedUntil":"2026-04-23T12:05:00Z"}`))
	harness.server.Handler().ServeHTTP(snoozeRec, snoozeReq)
	if snoozeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", snoozeRec.Code, snoozeRec.Body.String())
	}
	snoozed := decodeStrictResponse[reminderTransitionResponse](t, snoozeRec.Body.Bytes())
	if snoozed.Occurrence.State != reminders.StateSnoozed {
		t.Fatalf("expected snoozed occurrence, got %+v", snoozed)
	}

	completeRec := httptest.NewRecorder()
	completeReq := httptest.NewRequest(http.MethodPost, "/v1/reminders/"+snoozeReminder.ReminderID+"/complete", strings.NewReader(`{"occurrenceId":"`+snoozeCurrent.ActiveOccurrenceID+`","reason":"done"}`))
	harness.server.Handler().ServeHTTP(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", completeRec.Code, completeRec.Body.String())
	}
	completed := decodeStrictResponse[reminderTransitionResponse](t, completeRec.Body.Bytes())
	if completed.Occurrence.State != reminders.StateCompleted {
		t.Fatalf("expected completed occurrence, got %+v", completed)
	}

	dismissCreateRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(dismissCreateRec, httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Dismiss reminder","trigger":{"kind":"once","fireAt":"2026-04-23T12:02:00Z"}}`)))
	if dismissCreateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", dismissCreateRec.Code, dismissCreateRec.Body.String())
	}
	dismissReminder := decodeStrictResponse[reminders.Reminder](t, dismissCreateRec.Body.Bytes())
	harness.clock.Set(base.Add(2 * time.Minute))
	if err := harness.manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(dismiss due) returned error: %v", err)
	}
	dismissCurrent, ok, err := harness.manager.Get(ctx, dismissReminder.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get(dismiss) returned ok=%v err=%v", ok, err)
	}
	dismissRec := httptest.NewRecorder()
	dismissReq := httptest.NewRequest(http.MethodPost, "/v1/reminders/"+dismissReminder.ReminderID+"/dismiss", strings.NewReader(`{"occurrenceId":"`+dismissCurrent.ActiveOccurrenceID+`"}`))
	harness.server.Handler().ServeHTTP(dismissRec, dismissReq)
	if dismissRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", dismissRec.Code, dismissRec.Body.String())
	}
	dismissed := decodeStrictResponse[reminderTransitionResponse](t, dismissRec.Body.Bytes())
	if dismissed.Occurrence.State != reminders.StateDismissed {
		t.Fatalf("expected dismissed occurrence, got %+v", dismissed)
	}

	cancelCreateRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(cancelCreateRec, httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Cancel reminder","trigger":{"kind":"once","fireAt":"2026-04-23T12:10:00Z"}}`)))
	if cancelCreateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", cancelCreateRec.Code, cancelCreateRec.Body.String())
	}
	cancelReminder := decodeStrictResponse[reminders.Reminder](t, cancelCreateRec.Body.Bytes())
	cancelRec := httptest.NewRecorder()
	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/reminders/"+cancelReminder.ReminderID+"/cancel", strings.NewReader(`{"reason":"not needed"}`))
	harness.server.Handler().ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	cancelled := decodeStrictResponse[reminderTransitionResponse](t, cancelRec.Body.Bytes())
	if cancelled.Reminder.CurrentState != reminders.StateCancelled {
		t.Fatalf("expected cancelled reminder, got %+v", cancelled)
	}

	workflowCreateRec := httptest.NewRecorder()
	workflowBody := `{"title":"Workflow reminder","behaviorMode":"launch_workflow","trigger":{"kind":"once","fireAt":"2026-04-23T12:03:00Z"},"workflowLaunchConfig":{"entrypoint":"operator","workflowGoal":"follow up"}}`
	harness.server.Handler().ServeHTTP(workflowCreateRec, httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(workflowBody)))
	if workflowCreateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", workflowCreateRec.Code, workflowCreateRec.Body.String())
	}
	workflowReminder := decodeStrictResponse[reminders.Reminder](t, workflowCreateRec.Body.Bytes())
	harness.clock.Set(base.Add(3 * time.Minute))
	if err := harness.manager.Tick(ctx); err != nil {
		t.Fatalf("Tick(workflow due) returned error: %v", err)
	}

	workflowDetailRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(workflowDetailRec, httptest.NewRequest(http.MethodGet, "/v1/reminders/"+workflowReminder.ReminderID, nil))
	if workflowDetailRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", workflowDetailRec.Code, workflowDetailRec.Body.String())
	}
	workflowDetail := decodeStrictResponse[reminders.Reminder](t, workflowDetailRec.Body.Bytes())
	if workflowDetail.CurrentState != reminders.StateAcknowledged {
		t.Fatalf("expected workflow reminder acknowledged after launch, got %+v", workflowDetail)
	}

	workflowOccRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(workflowOccRec, httptest.NewRequest(http.MethodGet, "/v1/reminders/occurrences/"+workflowDetail.ActiveOccurrenceID, nil))
	if workflowOccRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", workflowOccRec.Code, workflowOccRec.Body.String())
	}
	workflowOccurrence := decodeStrictResponse[reminders.Occurrence](t, workflowOccRec.Body.Bytes())
	if workflowOccurrence.RunID != "run_reminder_api" || workflowOccurrence.WorkflowID != "wf_reminder_api" || workflowOccurrence.State != reminders.StateAcknowledged {
		t.Fatalf("expected workflow-linked occurrence, got %+v", workflowOccurrence)
	}
}

func TestScheduleWorkflowLauncherPersistsReminderLinkageOnRunsAndWorkflows(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(t.TempDir(), "dope-data"),
	}
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	runtimeManager := runtime.NewManager()
	launcher := NewScheduleWorkflowLauncher(ScheduleWorkflowLauncherDependencies{
		Config:      cfg,
		Runtime:     runtimeManager,
		EventBus:    eventBus,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})

	_, err = launcher.LaunchReminderWorkflow(context.Background(), reminders.WorkflowLaunchConfig{
		Entrypoint: "operator",
		RunGoal:    "launch reminder workflow",
	}, "rem_persisted", "rem_occ_persisted")
	if err == nil {
		t.Fatal("expected planning failure to surface as launch error")
	}

	runs, err := sqliteStore.ListRunsAllTenantsForTest(context.Background())
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one persisted run, got %+v", runs)
	}
	if runs[0].ReminderID != "rem_persisted" || runs[0].ReminderOccurrenceID != "rem_occ_persisted" || runs[0].RunID == "" {
		t.Fatalf("expected run reminder linkage, got %+v", runs[0])
	}

	workflows, err := sqliteStore.ListWorkflows(context.Background(), "test", runs[0].RunID)
	if err != nil {
		t.Fatalf("ListWorkflows returned error: %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("expected one persisted workflow, got %+v", workflows)
	}
	workflow, ok, err := sqliteStore.GetWorkflowByID(context.Background(), "test", workflows[0].WorkflowID)
	if err != nil || !ok {
		t.Fatalf("GetWorkflowByID returned ok=%v err=%v", ok, err)
	}
	if workflow.ReminderID != "rem_persisted" || workflow.ReminderOccurrenceID != "rem_occ_persisted" {
		t.Fatalf("expected workflow reminder linkage, got %+v", workflow)
	}
}

func TestReminderRoutePerformanceSmoke(t *testing.T) {
	t.Parallel()

	harness := newReminderServerHarness(t, reminderServerHarnessOptions{})
	base := time.Date(2026, 4, 23, 14, 0, 0, 0, time.UTC)
	dueAt := base.Add(time.Minute)
	harness.clock.Set(base)

	createStarted := time.Now()
	createRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/v1/reminders", strings.NewReader(`{"title":"Route perf reminder","trigger":{"kind":"once","fireAt":"2026-04-23T14:01:00Z"}}`)))
	createElapsed := time.Since(createStarted)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	if createElapsed > 500*time.Millisecond {
		t.Fatalf("expected reminder create route smoke under 500ms, got %s", createElapsed)
	}
	created := decodeStrictResponse[reminders.Reminder](t, createRec.Body.Bytes())

	listStarted := time.Now()
	listRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/v1/reminders", nil))
	listElapsed := time.Since(listStarted)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	if listElapsed > 500*time.Millisecond {
		t.Fatalf("expected reminder list route smoke under 500ms, got %s", listElapsed)
	}

	harness.clock.Set(dueAt)
	if err := harness.manager.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}
	current, ok, err := harness.manager.Get(context.Background(), created.ReminderID)
	if err != nil || !ok {
		t.Fatalf("Get returned ok=%v err=%v", ok, err)
	}

	occurrenceStarted := time.Now()
	occurrenceRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(occurrenceRec, httptest.NewRequest(http.MethodGet, "/v1/reminders/occurrences/"+current.ActiveOccurrenceID, nil))
	occurrenceElapsed := time.Since(occurrenceStarted)
	if occurrenceRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", occurrenceRec.Code, occurrenceRec.Body.String())
	}
	occurrence := decodeStrictResponse[reminders.Occurrence](t, occurrenceRec.Body.Bytes())
	if occurrence.LatestDeliveryID == "" {
		t.Fatalf("expected linked delivery projection, got %+v", occurrence)
	}
	if occurrenceElapsed > 500*time.Millisecond {
		t.Fatalf("expected occurrence route smoke under 500ms, got %s", occurrenceElapsed)
	}

	t.Logf("reminder route performance smoke: create=%s list=%s occurrence=%s", createElapsed, listElapsed, occurrenceElapsed)
}

func newReminderServerHarness(t *testing.T, opts reminderServerHarnessOptions) reminderServerHarness {
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

	eventBus := events.NewBus()
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	target, err := deliveryManager.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "reminder-api-target",
		DisplayName:      "Reminder API Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	preference := delivery.DeliveryPreference{
		PreferenceID:     "reminder-api-pref",
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

	clock := &reminderTestClock{now: time.Date(2026, 4, 23, 9, 0, 0, 0, time.UTC)}
	reminderManager := reminders.NewManager(reminders.Dependencies{
		EnvironmentScope: "test",
		Store:            sqliteStore,
		EventBus:         eventBus,
		Delivery:         deliveryManager,
		WorkflowLauncher: opts.workflowLauncher,
		Clock:            clock,
		TickInterval:     10 * time.Millisecond,
	})
	server := NewServer(Dependencies{
		Config:    cfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Reminders: reminderManager,
		Delivery:  deliveryManager,
		Store:     sqliteStore,
	})
	return reminderServerHarness{
		server:      server,
		manager:     reminderManager,
		delivery:    deliveryManager,
		clock:       clock,
		sqliteStore: sqliteStore,
	}
}

func TestReminderSchemasRoundTripJSONPayloads(t *testing.T) {
	t.Parallel()

	payload := reminderTransitionResponse{
		Reminder: reminders.Reminder{ReminderID: "rem_1", Title: "x", BehaviorMode: reminders.BehaviorModeNotifyOnly, Trigger: reminders.Reminder{}.Trigger, CurrentState: reminders.StatePending, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
	}
	if _, err := json.Marshal(payload); err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
}
