package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/artifacts"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func TestScheduleRoutesCreateAndInspectOneTimeSchedule(t *testing.T) {
	harness := newScheduleServerHarness(t)

	fireAt := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(`{
		"trigger": {
			"kind": "once",
			"fireAt": "`+fireAt+`"
		},
		"target": {
			"kind": "run",
			"run": {
				"entrypoint": "operator",
				"goal": "dispatch one test run"
			}
		},
		"retryPolicy": {
			"maxRetries": 2,
			"backoffKind": "fixed",
			"baseDelaySeconds": 5,
			"maxDelaySeconds": 5
		}
	}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for schedule create, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	created := decodeStrictResponse[map[string]any](t, createRec.Body.Bytes())
	scheduleID, _ := created["scheduleId"].(string)
	if scheduleID == "" {
		t.Fatalf("expected scheduleId in response, got %+v", created)
	}
	if created["status"] != "scheduled" {
		t.Fatalf("expected scheduled status, got %+v", created)
	}
	if created["targetRefId"] == "" {
		t.Fatalf("expected stable targetRefId, got %+v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/schedules/"+scheduleID, nil)
	getReq.Header.Set("Authorization", harness.authHeader)
	getRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for schedule get, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	got := decodeStrictResponse[map[string]any](t, getRec.Body.Bytes())
	if got["scheduleId"] != scheduleID {
		t.Fatalf("expected scheduleId %s, got %+v", scheduleID, got)
	}
	if got["nextDueAt"] == "" {
		t.Fatalf("expected nextDueAt, got %+v", got)
	}
}

func TestScheduleRoutesHideSchedulesFromOtherEnvironments(t *testing.T) {
	harness := newScheduleServerHarness(t)
	now := time.Now().UTC()
	if err := harness.store.UpsertSchedule(context.Background(), store.ScheduleRecord{
		ScheduleID:       "sched_prod_hidden",
		EnvironmentScope: "prod",
		Kind:             "one_time",
		Status:           "scheduled",
		TargetRefID:      "target_prod_hidden",
		CreatedAt:        now,
		UpdatedAt:        now,
		Document:         []byte(`{"scheduleId":"sched_prod_hidden","environmentScope":"prod","kind":"one_time","status":"scheduled","targetRefId":"target_prod_hidden"}`),
	}); err != nil {
		t.Fatalf("UpsertSchedule returned error: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/schedules", nil)
	listReq.Header.Set("Authorization", harness.authHeader)
	listRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	list := decodeStrictResponse[ScheduleListResponse](t, listRec.Body.Bytes())
	if len(list.Items) != 0 {
		t.Fatalf("expected no cross-environment schedules, got %+v", list)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/schedules/sched_prod_hidden", nil)
	getReq.Header.Set("Authorization", harness.authHeader)
	getRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestScheduleRoutesCancelBeforeDuePreventsDispatch(t *testing.T) {
	harness := newScheduleServerHarness(t)

	fireAt := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(`{
		"trigger": {
			"kind": "once",
			"fireAt": "`+fireAt+`"
		},
		"target": {
			"kind": "run",
			"run": {
				"entrypoint": "operator",
				"goal": "cancel via api"
			}
		},
		"retryPolicy": {
			"maxRetries": 0,
			"backoffKind": "fixed",
			"baseDelaySeconds": 5,
			"maxDelaySeconds": 5
		}
	}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[map[string]any](t, createRec.Body.Bytes())
	scheduleID := created["scheduleId"].(string)

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/schedules/"+scheduleID+"/cancel", nil)
	cancelReq.Header.Set("Authorization", harness.authHeader)
	cancelRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cancel, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
	}

	cancelled := decodeStrictResponse[map[string]any](t, cancelRec.Body.Bytes())
	if cancelled["status"] != "cancelled" {
		t.Fatalf("expected cancelled status, got %+v", cancelled)
	}

	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler Tick returned error: %v", err)
	}
	if runs := harness.runtime.ListRuns(); len(runs) != 0 {
		t.Fatalf("expected no run dispatch after cancel, got %+v", runs)
	}
}

func TestScheduleRoutesCreateAndGetStayUnder500ms(t *testing.T) {
	harness := newScheduleServerHarness(t)

	fireAt := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(`{
		"trigger": {
			"kind": "once",
			"fireAt": "`+fireAt+`"
		},
		"target": {
			"kind": "run",
			"run": {
				"entrypoint": "operator",
				"goal": "latency test"
			}
		},
		"retryPolicy": {
			"maxRetries": 0,
			"backoffKind": "fixed",
			"baseDelaySeconds": 5,
			"maxDelaySeconds": 5
		}
	}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createStarted := time.Now()
	createRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createRec, createReq)
	createElapsed := time.Since(createStarted)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	if createElapsed > 500*time.Millisecond {
		t.Fatalf("expected create under 500ms, got %s", createElapsed)
	}

	created := decodeStrictResponse[map[string]any](t, createRec.Body.Bytes())
	getReq := httptest.NewRequest(http.MethodGet, "/v1/schedules/"+created["scheduleId"].(string), nil)
	getReq.Header.Set("Authorization", harness.authHeader)
	getStarted := time.Now()
	getRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(getRec, getReq)
	getElapsed := time.Since(getStarted)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	if getElapsed > 500*time.Millisecond {
		t.Fatalf("expected get under 500ms, got %s", getElapsed)
	}
}

func TestScheduleEventsRouteFiltersByEnvironmentAndScheduleScope(t *testing.T) {
	harness := newScheduleServerHarness(t)

	fireAt := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(`{
		"trigger": {
			"kind": "once",
			"fireAt": "`+fireAt+`"
		},
		"target": {
			"kind": "run",
			"run": {
				"entrypoint": "operator",
				"goal": "event filtering"
			}
		},
		"retryPolicy": {
			"maxRetries": 0,
			"backoffKind": "fixed",
			"baseDelaySeconds": 5,
			"maxDelaySeconds": 5
		}
	}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[map[string]any](t, createRec.Body.Bytes())
	scheduleID := created["scheduleId"].(string)

	if _, err := harness.store.AppendEvent(context.Background(), events.Event{
		EventID:          "evt_prod_schedule_hidden",
		EnvironmentScope: "prod",
		Category:         "schedule",
		Name:             "schedule.dispatch_recorded",
		OccurredAt:       time.Now().UTC(),
		Scope: events.Scope{
			ScheduleID:        "sched_prod_hidden",
			ScheduleAttemptID: "sched_attempt_prod_hidden",
		},
		Resource: events.Resource{Kind: "schedule", ID: "sched_prod_hidden"},
		Payload:  map[string]any{"dispatchStatus": "dispatched"},
	}); err != nil {
		t.Fatalf("AppendEvent(prod hidden) returned error: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/events?category=schedule&scheduleId="+scheduleID, nil)
	listReq.Header.Set("Authorization", harness.authHeader)
	listRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for filtered events, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	eventList := decodeStrictResponse[EventListResponse](t, listRec.Body.Bytes())
	if len(eventList.Items) != 1 {
		t.Fatalf("expected exactly one schedule event for current env and schedule, got %+v", eventList.Items)
	}
	if eventList.Items[0].Scope.ScheduleID != scheduleID {
		t.Fatalf("expected scheduleId %s, got %+v", scheduleID, eventList.Items[0])
	}
}

func TestScheduleRoutesDispatchWorkflowTargetAndLinkWorkflowTruth(t *testing.T) {
	harness := newWorkflowScheduleServerHarness(t)

	target, err := harness.delivery.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "scheduled-workflow-target",
		DisplayName:      "Scheduled Workflow Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	if _, err := harness.delivery.UpsertPreference(context.Background(), delivery.DeliveryPreference{
		PreferenceID:     "scheduled-workflow-pref",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassRoutineSuccess: target.TargetID,
			delivery.ResultClassUrgent:         target.TargetID,
			delivery.ResultClassFailure:        target.TargetID,
		},
	}); err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}

	fireAt := time.Now().UTC().Add(100 * time.Millisecond).Format(time.RFC3339)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(`{
		"trigger": {
			"kind": "once",
			"fireAt": "`+fireAt+`"
		},
		"target": {
			"kind": "workflow",
			"workflow": {
				"entrypoint": "operator",
				"runGoal": "dispatch scheduled workflow",
				"workflowGoal": "Use a skill to complete a deterministic workflow."
			}
		},
		"retryPolicy": {
			"maxRetries": 0,
			"backoffKind": "fixed",
			"baseDelaySeconds": 5,
			"maxDelaySeconds": 5
		}
	}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for workflow schedule create, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	created := decodeStrictResponse[map[string]any](t, createRec.Body.Bytes())
	scheduleID := created["scheduleId"].(string)
	time.Sleep(150 * time.Millisecond)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler Tick returned error: %v", err)
	}

	var got map[string]any
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := harness.scheduler.Tick(context.Background()); err != nil {
			t.Fatalf("scheduler Tick returned error: %v", err)
		}
		getReq := httptest.NewRequest(http.MethodGet, "/v1/schedules/"+scheduleID, nil)
		getReq.Header.Set("Authorization", harness.authHeader)
		getRec := httptest.NewRecorder()
		harness.server.Handler().ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for schedule get, got %d body=%s", getRec.Code, getRec.Body.String())
		}
		got = decodeStrictResponse[map[string]any](t, getRec.Body.Bytes())
		attempts := got["attempts"].([]any)
		if len(attempts) == 1 {
			attempt := attempts[0].(map[string]any)
			if attempt["workflowId"] != "" && attempt["downstreamStatus"] == "completed" {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	attempts := got["attempts"].([]any)
	if len(attempts) != 1 {
		t.Fatalf("expected one attempt, got %+v", got)
	}
	attempt := attempts[0].(map[string]any)
	workflowID, _ := attempt["workflowId"].(string)
	runID, _ := attempt["runId"].(string)
	if workflowID == "" || runID == "" {
		t.Fatalf("expected linked run and workflow IDs, got %+v", attempt)
	}
	if attempt["downstreamStatus"] != "completed" {
		t.Fatalf("expected completed downstream workflow, got %+v", attempt)
	}

	workflowReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID+"/workflows/"+workflowID, nil)
	workflowReq.Header.Set("Authorization", harness.authHeader)
	workflowRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(workflowRec, workflowReq)
	if workflowRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for workflow get, got %d body=%s", workflowRec.Code, workflowRec.Body.String())
	}
	workflow := decodeStrictResponse[orchestration.Workflow](t, workflowRec.Body.Bytes())
	if workflow.Status != orchestration.WorkflowStatusCompleted {
		t.Fatalf("expected completed workflow, got %+v", workflow)
	}
	if workflow.ScheduleID != scheduleID || workflow.ScheduleAttemptID == "" {
		t.Fatalf("expected workflow schedule linkage, got %+v", workflow)
	}
	if len(workflow.Steps) != 1 || workflow.Steps[0].RuntimeStepID == "" || workflow.Steps[0].ActiveToolCallID == "" {
		t.Fatalf("expected runtime-linked workflow step, got %+v", workflow.Steps)
	}
	if workflow.LatestDeliveryID == "" || workflow.LatestDeliveryStatus != string(delivery.OutcomeStatusDelivered) || workflow.LatestDeliveryTargetID != target.TargetID {
		t.Fatalf("expected workflow latest delivery projection, got %+v", workflow)
	}

	outcomes, err := harness.delivery.ListOutcomes(context.Background(), delivery.OutcomeFilter{
		SourceKind: "workflow",
		SourceID:   workflowID,
	})
	if err != nil {
		t.Fatalf("ListOutcomes returned error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected one workflow delivery outcome, got %+v", outcomes)
	}
	if outcomes[0].ScheduleID != scheduleID || outcomes[0].ScheduleAttemptID == "" || outcomes[0].ChosenTargetID != target.TargetID {
		t.Fatalf("expected schedule-linked workflow delivery outcome, got %+v", outcomes[0])
	}
}

func TestScheduleWorkflowComputerUseDoesNotReuseOperatorRunSession(t *testing.T) {
	harness := newWorkflowScheduleServerHarness(t)

	operatorRun := seedRunForScheduleTest(t, harness.store, harness.runtime, "operator", "prepare operator browser session")
	createSessionReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+operatorRun.RunID+"/computer-use/sessions", strings.NewReader(`{"driverKind":"browser"}`))
	createSessionReq.Header.Set("Authorization", harness.authHeader)
	createSessionReq.Header.Set("Content-Type", "application/json")
	createSessionRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createSessionRec, createSessionReq)
	if createSessionRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for operator session create, got %d body=%s", createSessionRec.Code, createSessionRec.Body.String())
	}
	operatorSession := decodeStrictResponse[computeruse.Session](t, createSessionRec.Body.Bytes())

	fireAt := time.Now().UTC().Add(100 * time.Millisecond).Format(time.RFC3339)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/schedules", strings.NewReader(`{
		"trigger": {
			"kind": "once",
			"fireAt": "`+fireAt+`"
		},
		"target": {
			"kind": "workflow",
			"workflow": {
				"entrypoint": "operator",
				"runGoal": "Schedule-owned browser workflow",
				"workflowGoal": "Use browser and a skill to complete a deterministic workflow."
			}
		},
		"retryPolicy": {
			"maxRetries": 0,
			"backoffKind": "fixed",
			"baseDelaySeconds": 5,
			"maxDelaySeconds": 5
		}
	}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for workflow schedule create, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	created := decodeStrictResponse[map[string]any](t, createRec.Body.Bytes())
	scheduleID := created["scheduleId"].(string)
	time.Sleep(150 * time.Millisecond)
	if err := harness.scheduler.Tick(context.Background()); err != nil {
		t.Fatalf("scheduler Tick returned error: %v", err)
	}

	var (
		runID      string
		workflowID string
	)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := harness.scheduler.Tick(context.Background()); err != nil {
			t.Fatalf("scheduler Tick returned error: %v", err)
		}
		getReq := httptest.NewRequest(http.MethodGet, "/v1/schedules/"+scheduleID, nil)
		getReq.Header.Set("Authorization", harness.authHeader)
		getRec := httptest.NewRecorder()
		harness.server.Handler().ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusOK {
			t.Fatalf("expected 200 for schedule get, got %d body=%s", getRec.Code, getRec.Body.String())
		}
		got := decodeStrictResponse[map[string]any](t, getRec.Body.Bytes())
		attempts := got["attempts"].([]any)
		if len(attempts) == 1 {
			attempt := attempts[0].(map[string]any)
			runID, _ = attempt["runId"].(string)
			workflowID, _ = attempt["workflowId"].(string)
			if runID != "" && workflowID != "" && attempt["downstreamStatus"] == "completed" {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if runID == "" || workflowID == "" {
		t.Fatalf("expected completed scheduled workflow attempt, got runId=%q workflowId=%q", runID, workflowID)
	}

	workflowReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID+"/workflows/"+workflowID, nil)
	workflowReq.Header.Set("Authorization", harness.authHeader)
	workflowRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(workflowRec, workflowReq)
	if workflowRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for workflow get, got %d body=%s", workflowRec.Code, workflowRec.Body.String())
	}
	workflow := decodeStrictResponse[orchestration.Workflow](t, workflowRec.Body.Bytes())
	if len(workflow.Steps) == 0 || workflow.Steps[0].ConsumerKind != "computer_use" {
		t.Fatalf("expected leading computer-use step, got %+v", workflow.Steps)
	}
	if workflow.Steps[0].ComputerUseSessionID == "" {
		t.Fatalf("expected scheduled workflow computer-use session linkage, got %+v", workflow.Steps[0])
	}
	if workflow.Steps[0].ComputerUseSessionID == operatorSession.ComputerUseSessionID {
		t.Fatalf("expected scheduled workflow to avoid reusing operator session, got %+v", workflow.Steps[0])
	}
}

type scheduleServerHarness struct {
	cfg        config.Config
	server     *Server
	store      *store.SQLiteStore
	runtime    *runtime.Manager
	scheduler  *scheduler.Scheduler
	delivery   *delivery.Manager
	authHeader string
}

func newScheduleServerHarness(t *testing.T) scheduleServerHarness {
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
	t.Cleanup(eventBus.Close)

	runtimeManager := runtime.NewManager()
	authManager := auth.NewManager()
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	scheduleManager := scheduler.New(scheduler.Dependencies{
		Config:      cfg,
		Runtime:     runtimeManager,
		EventBus:    eventBus,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		Auth:        authManager,
		EventBus:    eventBus,
		Runtime:     runtimeManager,
		Scheduler:   scheduleManager,
		Delivery:    deliveryManager,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})

	return scheduleServerHarness{
		cfg:        cfg,
		server:     server,
		store:      sqliteStore,
		runtime:    runtimeManager,
		scheduler:  scheduleManager,
		delivery:   deliveryManager,
		authHeader: issueAuthHeaderForTest(t, authManager, "schedule-web"),
	}
}

func TestScheduleRoutesProjectLatestDeliverySummaryOntoAttempts(t *testing.T) {
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
	t.Cleanup(eventBus.Close)

	runtimeManager := runtime.NewManager()
	authManager := auth.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	scheduleManager := scheduler.New(scheduler.Dependencies{
		Config:      cfg,
		Runtime:     runtimeManager,
		EventBus:    eventBus,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	target, err := deliveryManager.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "schedule-target",
		DisplayName:      "Schedule Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	if _, err := deliveryManager.UpsertPreference(context.Background(), delivery.DeliveryPreference{
		PreferenceID:     "pref-schedule",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassRoutineSuccess: target.TargetID,
			delivery.ResultClassUrgent:         target.TargetID,
			delivery.ResultClassFailure:        target.TargetID,
		},
	}); err != nil {
		t.Fatalf("UpsertPreference returned error: %v", err)
	}

	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		Auth:        authManager,
		EventBus:    eventBus,
		Runtime:     runtimeManager,
		Scheduler:   scheduleManager,
		Delivery:    deliveryManager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "delivery-schedule-web")

	fireAt := time.Now().UTC().Add(20 * time.Millisecond)
	schedule, err := scheduleManager.Create(context.Background(), scheduler.CreateInput{
		Trigger: scheduler.Trigger{
			Kind:   scheduler.TriggerKindOnce,
			FireAt: &fireAt,
		},
		Target: scheduler.Target{
			Kind: scheduler.TargetKindRun,
			Run: &scheduler.RunTarget{
				Entrypoint: "operator",
				Goal:       "scheduled background result",
			},
		},
		RetryPolicy: scheduler.RetryPolicy{MaxRetries: 0, BackoffKind: scheduler.RetryBackoffFixed, BaseDelaySeconds: 1, MaxDelaySeconds: 1},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	if err := scheduleManager.Tick(context.Background()); err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}

	schedule, ok, err := scheduleManager.Get(context.Background(), schedule.ScheduleID)
	if err != nil || !ok {
		t.Fatalf("Get returned ok=%v err=%v", ok, err)
	}
	if len(schedule.Attempts) != 1 {
		t.Fatalf("expected one attempt, got %+v", schedule.Attempts)
	}

	if _, err := deliveryManager.EmitOutcome(context.Background(), delivery.OutcomeInput{
		SourceKind:        "run",
		SourceID:          schedule.Attempts[0].RunID,
		RunID:             schedule.Attempts[0].RunID,
		ScheduleID:        schedule.ScheduleID,
		ScheduleAttemptID: schedule.Attempts[0].AttemptID,
		ResultClass:       delivery.ResultClassRoutineSuccess,
		PayloadPreview:    "scheduled background result",
	}); err != nil {
		t.Fatalf("EmitOutcome returned error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/schedules/"+schedule.ScheduleID, nil)
	req.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	got := decodeStrictResponse[scheduler.Schedule](t, rec.Body.Bytes())
	if len(got.Attempts) != 1 {
		t.Fatalf("expected one projected attempt, got %+v", got.Attempts)
	}
	if got.Attempts[0].LatestDeliveryID == "" || got.Attempts[0].LatestDeliveryStatus != string(delivery.OutcomeStatusDelivered) || got.Attempts[0].LatestDeliveryTargetID != target.TargetID {
		t.Fatalf("expected latest delivery projection on schedule attempt, got %+v", got.Attempts[0])
	}
}

func newWorkflowScheduleServerHarness(t *testing.T) scheduleServerHarness {
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
	t.Cleanup(eventBus.Close)

	runtimeManager := runtime.NewManager()
	authManager := auth.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	policyEngine := policy.NewEngine()
	skillRegistry := newAllowSkillRegistryForWorkflowTest(t, cfg.DataDir)
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	computerUseManager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: "test",
		Runtime:          runtimeManager,
		Policy:           policyEngine,
		Store:            sqliteStore,
		Artifacts:        artifacts.NewService(t.TempDir()),
	})
	workflowLauncher := NewScheduleWorkflowLauncher(ScheduleWorkflowLauncherDependencies{
		Config:      cfg,
		Runtime:     runtimeManager,
		Policy:      policyEngine,
		Skills:      skillRegistry,
		Sandboxes:   sandboxManager,
		ComputerUse: computerUseManager,
		Delivery:    deliveryManager,
		EventBus:    eventBus,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})
	scheduleManager := scheduler.New(scheduler.Dependencies{
		Config:           cfg,
		Runtime:          runtimeManager,
		EventBus:         eventBus,
		Store:            sqliteStore,
		Checkpoints:      checkpointManager,
		WorkflowLauncher: workflowLauncher,
	})
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		Auth:        authManager,
		EventBus:    eventBus,
		Policy:      policyEngine,
		Runtime:     runtimeManager,
		Skills:      skillRegistry,
		Sandboxes:   sandboxManager,
		ComputerUse: computerUseManager,
		Scheduler:   scheduleManager,
		Delivery:    deliveryManager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})

	return scheduleServerHarness{
		cfg:        cfg,
		server:     server,
		store:      sqliteStore,
		runtime:    runtimeManager,
		scheduler:  scheduleManager,
		delivery:   deliveryManager,
		authHeader: issueAuthHeaderForTest(t, authManager, "schedule-workflow-web"),
	}
}

func seedRunForScheduleTest(t *testing.T, sqliteStore *store.SQLiteStore, runtimeManager *runtime.Manager, entrypoint, goal string) runtime.Run {
	t.Helper()

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{
		Entrypoint: entrypoint,
		Goal:       goal,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	return run
}
