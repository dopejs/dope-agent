package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestRecoverPersistedStateRestoresRuntimeAndEventHistory(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	seedRuntime := runtime.NewManager()
	run, err := seedRuntime.CreateRun(runtime.CreateRunInput{
		SessionID:  "session_recovery",
		Entrypoint: "chat",
		Goal:       "recover after daemon restart",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := seedRuntime.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "plan recovery",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	step, runUpdate, err := seedRuntime.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
	})
	if err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun returned error: %v", err)
	}
	if runUpdate != nil {
		run = *runUpdate
	}

	session := router.Session{
		SessionID:    run.SessionID,
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "local",
		AccountID:    "local",
		PeerID:       "chat",
		RoutingKey:   "direct:local:local:chat",
		Generation:   1,
		CreatedAt:    time.Now().UTC().Add(-time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
		LastActiveAt: time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(ctx, run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertStep(ctx, step); err != nil {
		t.Fatalf("UpsertStep returned error: %v", err)
	}

	checkpointManager := checkpoints.NewManager(sqliteStore, seedRuntime)
	if err := checkpointManager.SaveRunCheckpoint(ctx, run.RunID); err != nil {
		t.Fatalf("SaveRunCheckpoint returned error: %v", err)
	}

	persistedEvent := events.Event{
		EventID:    "evt_recovery",
		Category:   "run",
		Name:       "run.status_changed",
		OccurredAt: time.Now().UTC(),
		Scope: events.Scope{
			SessionID: run.SessionID,
			RunID:     run.RunID,
		},
		Resource: events.Resource{
			Kind: "run",
			ID:   run.RunID,
		},
		Payload: map[string]any{
			"status": run.Status,
		},
	}
	if _, err := sqliteStore.AppendEvent(ctx, persistedEvent); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	restoredRuntime := runtime.NewManager()
	restoredRouter := router.NewSessionRouter()
	restoredEventBus := events.NewBus()
	restoreCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	gotRun, ok := restoredRuntime.GetRun(run.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.Status != runtime.RunStatusRunning {
		t.Fatalf("expected restored run status running, got %s", gotRun.Status)
	}

	gotStep, ok := restoredRuntime.GetStep(run.RunID, step.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	items := restoredEventBus.List(events.Filter{RunID: run.RunID})
	if len(items) != 1 {
		t.Fatalf("expected 1 restored event, got %d", len(items))
	}
	if items[0].EventID != persistedEvent.EventID {
		t.Fatalf("expected restored event ID %s, got %s", persistedEvent.EventID, items[0].EventID)
	}

	if _, ok := restoredRouter.GetSession(run.SessionID); !ok {
		t.Fatal("expected restored session")
	}
}

func TestRecoverPersistedStateRestoresSupervisionState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	connectorHeartbeat := time.Now().UTC().Add(-30 * time.Second)
	connector := connectors.Connector{
		ConnectorID:     "slack-main",
		Kind:            "slack",
		DisplayName:     "Slack Main",
		Status:          connectors.StatusHealthy,
		FailureCount:    0,
		RestartCount:    2,
		LastHeartbeatAt: &connectorHeartbeat,
		CreatedAt:       time.Now().UTC().Add(-time.Hour),
		UpdatedAt:       time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertConnector(ctx, connector); err != nil {
		t.Fatalf("UpsertConnector returned error: %v", err)
	}

	capabilityRestart := time.Now().UTC().Add(-20 * time.Second)
	capability := capabilities.Capability{
		CapabilityID:   "shell",
		Kind:           "exec",
		DisplayName:    "Shell",
		Status:         capabilities.StatusBackingOff,
		FailureCount:   3,
		RestartCount:   1,
		BackoffSeconds: 20,
		LastRestartAt:  &capabilityRestart,
		CreatedAt:      time.Now().UTC().Add(-time.Hour),
		UpdatedAt:      time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertCapability(ctx, capability); err != nil {
		t.Fatalf("UpsertCapability returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	gotConnector, ok := restoredConnectors.Get(connector.ConnectorID)
	if !ok {
		t.Fatal("expected restored connector")
	}
	if gotConnector.Status != connectors.StatusHealthy {
		t.Fatalf("expected restored connector status healthy, got %s", gotConnector.Status)
	}
	if gotConnector.RestartCount != 2 {
		t.Fatalf("expected restored connector restart count 2, got %d", gotConnector.RestartCount)
	}

	gotCapability, ok := restoredCapabilities.Get(capability.CapabilityID)
	if !ok {
		t.Fatal("expected restored capability")
	}
	if gotCapability.Status != capabilities.StatusBackingOff {
		t.Fatalf("expected restored capability status backing_off, got %s", gotCapability.Status)
	}
	if gotCapability.BackoffSeconds != 20 {
		t.Fatalf("expected restored capability backoff 20, got %d", gotCapability.BackoffSeconds)
	}
}

func TestRecoverPersistedStateRestoresAuthAndPolicyState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	ctx := context.Background()
	completedAt := time.Now().UTC().Add(-2 * time.Minute)
	pairing := auth.Pairing{
		PairingID:   "pair_restore",
		Mode:        auth.PairingModeLocal,
		Label:       "web-ui",
		Status:      auth.PairingStatusCompleted,
		CodeHash:    "hash",
		CreatedAt:   time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt:   time.Now().UTC().Add(-5 * time.Minute),
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
		CompletedAt: &completedAt,
	}
	if err := sqliteStore.UpsertPairing(ctx, pairing); err != nil {
		t.Fatalf("UpsertPairing returned error: %v", err)
	}

	token := auth.AccessToken{
		TokenID:      "tok_restore",
		Label:        "web-ui",
		Mode:         auth.PairingModeLocal,
		TokenHash:    "token-hash",
		TokenPreview: "dope_preview",
		CreatedAt:    time.Now().UTC().Add(-9 * time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-time.Minute),
	}
	if err := sqliteStore.UpsertAccessToken(ctx, token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}

	approval := policy.Approval{
		ApprovalID:   "approval_restore",
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Reason:       "needs approval",
		RequestedBy:  "web-ui",
		Status:       policy.ApprovalStatusApproved,
		CreatedAt:    time.Now().UTC().Add(-8 * time.Minute),
		UpdatedAt:    time.Now().UTC().Add(-7 * time.Minute),
		ResolvedAt:   &completedAt,
		Resolution:   string(policy.ApprovalStatusApproved),
	}
	if err := sqliteStore.UpsertApproval(ctx, approval); err != nil {
		t.Fatalf("UpsertApproval returned error: %v", err)
	}

	decision := policy.Decision{
		DecisionID:   "decision_restore",
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Outcome:      policy.DecisionOutcomeApproved,
		Reason:       "needs approval",
		ApprovalID:   approval.ApprovalID,
		CreatedAt:    time.Now().UTC().Add(-7 * time.Minute),
	}
	if err := sqliteStore.UpsertDecision(ctx, decision); err != nil {
		t.Fatalf("UpsertDecision returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	if got, ok := restoredAuth.GetPairing(pairing.PairingID); !ok || got.Status != auth.PairingStatusCompleted {
		t.Fatalf("expected restored completed pairing, got %+v ok=%v", got, ok)
	}
	if got, ok := restoredAuth.GetToken(token.TokenID); !ok || got.TokenPreview != token.TokenPreview {
		t.Fatalf("expected restored token, got %+v ok=%v", got, ok)
	}
	if got, ok := restoredPolicy.GetApproval(approval.ApprovalID); !ok || got.Status != policy.ApprovalStatusApproved {
		t.Fatalf("expected restored approved approval, got %+v ok=%v", got, ok)
	}
	if len(restoredPolicy.ListDecisions()) != 1 {
		t.Fatalf("expected 1 restored decision, got %d", len(restoredPolicy.ListDecisions()))
	}
}

func TestAppRunPublishesLifecycleEvents(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	reopenedStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := reopenedStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	items, err := reopenedStore.ListEvents(context.Background(), events.Filter{Category: "system"})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 system events, got %d", len(items))
	}
	if items[0].Name != "system.started" {
		t.Fatalf("expected system.started, got %s", items[0].Name)
	}
	if items[1].Name != "system.stopped" {
		t.Fatalf("expected system.stopped, got %s", items[1].Name)
	}
	if items[1].Sequence <= items[0].Sequence {
		t.Fatalf("expected monotonic system event sequence, got %d then %d", items[0].Sequence, items[1].Sequence)
	}
}

func TestAppRestartRestoresRuntimeBoundary(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	authHeader := testAuthHeader(t, first.Auth)

	createRunRec := httptest.NewRecorder()
	createRunReq := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"chat","goal":"restart-safe"}`))
	createRunReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(createRunRec, createRunReq)
	if createRunRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for run create, got %d", createRunRec.Code)
	}
	var createdRun runtime.Run
	if err := json.Unmarshal(createRunRec.Body.Bytes(), &createdRun); err != nil {
		t.Fatalf("failed to decode created run: %v", err)
	}

	createStepRec := httptest.NewRecorder()
	createStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps", strings.NewReader(`{"title":"recover me","kind":"task"}`))
	createStepReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(createStepRec, createStepReq)
	if createStepRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for step create, got %d", createStepRec.Code)
	}
	var createdStep runtime.Step
	if err := json.Unmarshal(createStepRec.Body.Bytes(), &createdStep); err != nil {
		t.Fatalf("failed to decode created step: %v", err)
	}

	updateStepRec := httptest.NewRecorder()
	updateStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps/"+createdStep.StepID+"/status", strings.NewReader(`{"status":"planning"}`))
	updateStepReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(updateStepRec, updateStepReq)
	if updateStepRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for step status update, got %d", updateStepRec.Code)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	gotRun, ok := second.Runtime.GetRun(createdRun.RunID)
	if !ok {
		t.Fatal("expected restored run")
	}
	if gotRun.Status != runtime.RunStatusRunning {
		t.Fatalf("expected restored run status running, got %s", gotRun.Status)
	}

	gotStep, ok := second.Runtime.GetStep(createdRun.RunID, createdStep.StepID)
	if !ok {
		t.Fatal("expected restored step")
	}
	if gotStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected restored step status planning, got %s", gotStep.Status)
	}

	eventsRec := httptest.NewRecorder()
	eventsReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+createdRun.RunID+"/events", nil)
	eventsReq.Header.Set("Authorization", testAuthHeader(t, second.Auth))
	second.Server.Handler().ServeHTTP(eventsRec, eventsReq)
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for run events after restart, got %d", eventsRec.Code)
	}
	var eventList struct {
		Items      []events.Event `json:"items"`
		NextCursor int64          `json:"nextCursor"`
	}
	if err := json.Unmarshal(eventsRec.Body.Bytes(), &eventList); err != nil {
		t.Fatalf("failed to decode run events response: %v", err)
	}
	if len(eventList.Items) < 3 {
		t.Fatalf("expected at least 3 restored events, got %d", len(eventList.Items))
	}
	if eventList.NextCursor == 0 {
		t.Fatal("expected next cursor after restart")
	}
}

func TestAppRestartRestoresAuthAndApprovalAPIState(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	first, err := New()
	if err != nil {
		t.Fatalf("first New returned error: %v", err)
	}

	startRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(startRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/start", strings.NewReader(`{"mode":"local","label":"web-ui"}`)))
	if startRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for pairing start, got %d", startRec.Code)
	}
	var pairingStart struct {
		Pairing     auth.Pairing `json:"pairing"`
		PairingCode string       `json:"pairingCode"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &pairingStart); err != nil {
		t.Fatalf("failed to decode pairing start response: %v", err)
	}

	completeRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(completeRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/"+pairingStart.Pairing.PairingID+"/complete", strings.NewReader(`{"code":"`+pairingStart.PairingCode+`"}`)))
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for pairing complete, got %d", completeRec.Code)
	}
	var pairingComplete struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(completeRec.Body.Bytes(), &pairingComplete); err != nil {
		t.Fatalf("failed to decode pairing complete response: %v", err)
	}
	authHeader := "Bearer " + pairingComplete.AccessToken

	createApprovalReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals", strings.NewReader(`{"action":"tool_call.execute","resourceKind":"capability","resourceId":"shell","reason":"restart persistence"}`))
	createApprovalReq.Header.Set("Authorization", authHeader)
	createApprovalRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createApprovalRec, createApprovalReq)
	if createApprovalRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approval create, got %d body=%s", createApprovalRec.Code, createApprovalRec.Body.String())
	}
	var created struct {
		Approval policy.Approval `json:"approval"`
	}
	if err := json.Unmarshal(createApprovalRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode approval create response: %v", err)
	}

	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}
	defer func() {
		if err := second.Close(context.Background()); err != nil {
			t.Fatalf("second Close returned error: %v", err)
		}
	}()

	meReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReq.Header.Set("Authorization", authHeader)
	meRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth me after restart, got %d body=%s", meRec.Code, meRec.Body.String())
	}

	approvalReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+created.Approval.ApprovalID, nil)
	approvalReq.Header.Set("Authorization", authHeader)
	approvalRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(approvalRec, approvalReq)
	if approvalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get after restart, got %d body=%s", approvalRec.Code, approvalRec.Body.String())
	}
	var restored policy.Approval
	if err := json.Unmarshal(approvalRec.Body.Bytes(), &restored); err != nil {
		t.Fatalf("failed to decode approval get response: %v", err)
	}
	if restored.ApprovalID != created.Approval.ApprovalID {
		t.Fatalf("expected approval ID %s, got %s", created.Approval.ApprovalID, restored.ApprovalID)
	}
}

func testAuthHeader(t *testing.T, manager *auth.Manager) string {
	t.Helper()

	pairing, code, err := manager.StartPairing(auth.StartPairingInput{
		Mode:  auth.PairingModeLocal,
		Label: "test-client",
	})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	_, _, tokenSecret, err := manager.CompletePairing(pairing.PairingID, auth.CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}
	return "Bearer " + tokenSecret
}
