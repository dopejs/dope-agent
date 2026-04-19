package app

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
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

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoreCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil); err != nil {
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

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil); err != nil {
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

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, providers.NewManager(config.Config{}, llm.NewDispatcher()), nil); err != nil {
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

func TestAppRestartRestoresConnectorIngressBoundRuns(t *testing.T) {
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

	registerConnectorRec := httptest.NewRecorder()
	registerConnectorReq := httptest.NewRequest(http.MethodPost, "/v1/connectors", strings.NewReader(`{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`))
	registerConnectorReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(registerConnectorRec, registerConnectorReq)
	if registerConnectorRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for connector create, got %d body=%s", registerConnectorRec.Code, registerConnectorRec.Body.String())
	}

	ingressRec := httptest.NewRecorder()
	ingressReq := httptest.NewRequest(http.MethodPost, "/v1/connectors/telegram-main/ingress/messages", strings.NewReader(`{
		"route":{"kind":"group","accountId":"bot-main","peerId":"chat-1","threadId":"thread-1"},
		"message":{"messageId":"msg_1","text":"hello"},
		"run":{"entrypoint":"connector.message","goal":"restart ingress"}
	}`))
	ingressReq.Header.Set("Authorization", authHeader)
	first.Server.Handler().ServeHTTP(ingressRec, ingressReq)
	if ingressRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for ingress, got %d body=%s", ingressRec.Code, ingressRec.Body.String())
	}

	var ingressResponse api.ConnectorIngressMessageResponse
	if err := json.Unmarshal(ingressRec.Body.Bytes(), &ingressResponse); err != nil {
		t.Fatalf("failed to decode ingress response: %v", err)
	}
	if ingressResponse.Run == nil {
		t.Fatal("expected ingress-created run")
	}

	second, err := New()
	if err != nil {
		t.Fatalf("second New returned error: %v", err)
	}

	restoredRun, ok := second.Runtime.GetRun(ingressResponse.Run.RunID)
	if !ok {
		t.Fatal("expected ingress-created run to be restored")
	}
	if restoredRun.SessionID != ingressResponse.Session.SessionID {
		t.Fatalf("expected restored run session %s, got %s", ingressResponse.Session.SessionID, restoredRun.SessionID)
	}
	restoredSession, ok := second.Router.GetSession(ingressResponse.Session.SessionID)
	if !ok {
		t.Fatal("expected ingress-created session to be restored")
	}
	if restoredSession.Channel != "telegram" {
		t.Fatalf("expected restored session channel telegram, got %s", restoredSession.Channel)
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

func TestAppRestartRestoresHighRiskApprovalSandboxProvenance(t *testing.T) {
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

	if _, _, err := first.CapabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "shell",
		Kind:         "exec",
		DisplayName:  "Shell",
	}); err != nil {
		t.Fatalf("Register shell capability returned error: %v", err)
	}
	run, err := first.Runtime.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := first.Runtime.CreateStep(run.RunID, runtime.CreateStepInput{Title: "approval restart"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd"}}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for pending approval tool call, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var pending struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("failed to decode pending approval response: %v", err)
	}
	if pending.Approval.Sandbox == nil || pending.Decision.Sandbox == nil {
		t.Fatalf("expected pending approval response sandbox provenance, got approval=%+v decision=%+v", pending.Approval.Sandbox, pending.Decision.Sandbox)
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

	approvalReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+pending.Approval.ApprovalID, nil)
	approvalReq.Header.Set("Authorization", authHeader)
	approvalRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(approvalRec, approvalReq)
	if approvalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get after restart, got %d body=%s", approvalRec.Code, approvalRec.Body.String())
	}
	var restoredApproval policy.Approval
	if err := json.Unmarshal(approvalRec.Body.Bytes(), &restoredApproval); err != nil {
		t.Fatalf("failed to decode restored approval: %v", err)
	}
	if restoredApproval.Sandbox == nil {
		t.Fatalf("expected restored approval sandbox provenance, got %+v", restoredApproval)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+pending.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"rejected","comment":"still denied"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval resolve after restart, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}
	var resolved struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("failed to decode resolved approval response: %v", err)
	}
	if resolved.Approval.Sandbox == nil || resolved.Decision.Sandbox == nil {
		t.Fatalf("expected resolved approval response sandbox provenance after restart, got approval=%+v decision=%+v", resolved.Approval.Sandbox, resolved.Decision.Sandbox)
	}
}

func TestAppRestartRestoresOperatorStateAcrossSubsystems(t *testing.T) {
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

	createConnectorReq := httptest.NewRequest(http.MethodPost, "/v1/connectors", strings.NewReader(`{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`))
	createConnectorReq.Header.Set("Authorization", authHeader)
	createConnectorRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createConnectorRec, createConnectorReq)
	if createConnectorRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for connector create, got %d body=%s", createConnectorRec.Code, createConnectorRec.Body.String())
	}

	createCapabilityReq := httptest.NewRequest(http.MethodPost, "/v1/capabilities", strings.NewReader(`{"capabilityId":"docs","kind":"docs","displayName":"Docs"}`))
	createCapabilityReq.Header.Set("Authorization", authHeader)
	createCapabilityRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createCapabilityRec, createCapabilityReq)
	if createCapabilityRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for capability create, got %d body=%s", createCapabilityRec.Code, createCapabilityRec.Body.String())
	}

	createDispatchReq := httptest.NewRequest(http.MethodPost, "/v1/llm/dispatches", strings.NewReader(`{"provider":"echo","model":"echo-v1","messages":[{"role":"user","content":"hello restart"}]}`))
	createDispatchReq.Header.Set("Authorization", authHeader)
	createDispatchRec := httptest.NewRecorder()
	first.Server.Handler().ServeHTTP(createDispatchRec, createDispatchReq)
	if createDispatchRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for llm dispatch create, got %d body=%s", createDispatchRec.Code, createDispatchRec.Body.String())
	}
	var createdDispatch llm.Dispatch
	if err := json.Unmarshal(createDispatchRec.Body.Bytes(), &createdDispatch); err != nil {
		t.Fatalf("failed to decode llm dispatch create response: %v", err)
	}

	createApprovalReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals", strings.NewReader(`{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"restart coverage"}`))
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

	connectorsReq := httptest.NewRequest(http.MethodGet, "/v1/connectors", nil)
	connectorsReq.Header.Set("Authorization", authHeader)
	connectorsRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(connectorsRec, connectorsReq)
	if connectorsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for connector list after restart, got %d body=%s", connectorsRec.Code, connectorsRec.Body.String())
	}
	var connectorList struct {
		Items []connectors.Connector `json:"items"`
	}
	if err := json.Unmarshal(connectorsRec.Body.Bytes(), &connectorList); err != nil {
		t.Fatalf("failed to decode connector list response: %v", err)
	}
	if len(connectorList.Items) != 1 || connectorList.Items[0].ConnectorID != "telegram-main" {
		t.Fatalf("expected restored connector telegram-main, got %+v", connectorList.Items)
	}

	capabilitiesReq := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	capabilitiesReq.Header.Set("Authorization", authHeader)
	capabilitiesRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(capabilitiesRec, capabilitiesReq)
	if capabilitiesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for capability list after restart, got %d body=%s", capabilitiesRec.Code, capabilitiesRec.Body.String())
	}
	var capabilityList struct {
		Items []capabilities.Capability `json:"items"`
	}
	if err := json.Unmarshal(capabilitiesRec.Body.Bytes(), &capabilityList); err != nil {
		t.Fatalf("failed to decode capability list response: %v", err)
	}
	if len(capabilityList.Items) != 1 || capabilityList.Items[0].CapabilityID != "docs" {
		t.Fatalf("expected restored capability docs, got %+v", capabilityList.Items)
	}

	dispatchesReq := httptest.NewRequest(http.MethodGet, "/v1/llm/dispatches", nil)
	dispatchesReq.Header.Set("Authorization", authHeader)
	dispatchesRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(dispatchesRec, dispatchesReq)
	if dispatchesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for llm dispatch list after restart, got %d body=%s", dispatchesRec.Code, dispatchesRec.Body.String())
	}
	var dispatchList struct {
		Items []llm.Dispatch `json:"items"`
	}
	if err := json.Unmarshal(dispatchesRec.Body.Bytes(), &dispatchList); err != nil {
		t.Fatalf("failed to decode llm dispatch list response: %v", err)
	}
	if len(dispatchList.Items) != 1 || dispatchList.Items[0].DispatchID != createdDispatch.DispatchID {
		t.Fatalf("expected restored llm dispatch %s, got %+v", createdDispatch.DispatchID, dispatchList.Items)
	}

	approvalReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+created.Approval.ApprovalID, nil)
	approvalReq.Header.Set("Authorization", authHeader)
	approvalRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(approvalRec, approvalReq)
	if approvalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get after restart, got %d body=%s", approvalRec.Code, approvalRec.Body.String())
	}
	var restoredApproval policy.Approval
	if err := json.Unmarshal(approvalRec.Body.Bytes(), &restoredApproval); err != nil {
		t.Fatalf("failed to decode approval get response: %v", err)
	}
	if restoredApproval.ApprovalID != created.Approval.ApprovalID {
		t.Fatalf("expected restored approval ID %s, got %s", created.Approval.ApprovalID, restoredApproval.ApprovalID)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReq.Header.Set("Authorization", authHeader)
	meRec := httptest.NewRecorder()
	second.Server.Handler().ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth me after restart, got %d body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestNewConfiguresOpenAICompatibleProviderAndServesChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				t.Fatalf("expected bearer auth header, got %q", r.Header.Get("Authorization"))
			}
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), `"stream":true`) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":2,\"total_tokens\":4}}\n\n"))
				_, _ = w.Write([]byte("data: [DONE]\n\n"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":2,"total_tokens":4}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	dataDir := filepath.Join(t.TempDir(), "dope")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"llm": {
			"defaultProvider": "openai_compatible",
			"defaultModel": "gpt-test",
			"defaultTimeoutMs": 30000,
			"openaiCompatible": {
				"baseURL": "`+upstream.URL+`/v1",
				"apiKeyEnv": "OPENAI_TEST_KEY",
				"model": "gpt-test"
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")
	t.Setenv("OPENAI_TEST_KEY", "secret")

	application, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer func() {
		if err := application.Close(context.Background()); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	authHeader := testAuthHeader(t, application.Auth)

	queryReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"query":"hello"}`))
	queryReq.Header.Set("Authorization", authHeader)
	queryRec := httptest.NewRecorder()
	application.Server.Handler().ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for chat query, got %d body=%s", queryRec.Code, queryRec.Body.String())
	}
	var response struct {
		Reply    string `json:"reply"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := json.Unmarshal(queryRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	if response.Reply != "hello world" {
		t.Fatalf("expected hello world reply, got %q", response.Reply)
	}
	if response.Provider != llm.OpenAICompatibleProviderName {
		t.Fatalf("expected provider %s, got %s", llm.OpenAICompatibleProviderName, response.Provider)
	}
	if response.Model != "gpt-test" {
		t.Fatalf("expected model gpt-test, got %s", response.Model)
	}

	streamServer := httptest.NewServer(application.Server.Handler())
	defer streamServer.Close()

	streamReq, err := http.NewRequest(http.MethodPost, streamServer.URL+"/v1/chat/query/stream", strings.NewReader(`{"query":"hello"}`))
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	streamReq.Header.Set("Authorization", authHeader)
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer streamResp.Body.Close()

	reader := bufio.NewReader(streamResp.Body)
	var chunks []string
	for range 16 {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		chunks = append(chunks, line)
		if strings.Contains(strings.Join(chunks, ""), "event: chat.query.completed") {
			break
		}
	}
	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "event: chat.query.started") || !strings.Contains(joined, "event: chat.query.delta") || !strings.Contains(joined, "event: chat.query.completed") {
		t.Fatalf("unexpected stream payload %q", joined)
	}
}

func TestRecoverPersistedStateRestoresManagedProviderState(t *testing.T) {
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
	authState := providers.AuthState{
		ProviderID:    "codex_managed",
		Family:        providers.FamilyCodexCLI,
		AuthMode:      providers.AuthModeLocalCLIBridge,
		Status:        providers.AuthStatusAuthenticated,
		CLIPath:       "/usr/bin/codex",
		CLIAvailable:  true,
		AccountLabel:  "user@example.com",
		Plan:          "pro",
		AuthMethod:    "chatgpt",
		LoginCommand:  []string{"codex", "login"},
		LogoutCommand: []string{"codex", "logout"},
		LastCheckedAt: time.Now().UTC(),
		Metadata: map[string]string{
			"managedProviderId":     "codex_managed",
			"managedProviderAction": "auth_status",
			"sandboxProfileId":      "managed_provider_codex",
			"sandboxDecision":       "allow",
			"enforcementStrength":   "declared_only",
		},
	}
	if err := sqliteStore.UpsertProviderAuthState(ctx, authState); err != nil {
		t.Fatalf("UpsertProviderAuthState returned error: %v", err)
	}
	models := []providers.Model{
		{ProviderID: "codex_managed", ModelID: "gpt-5.4", DisplayName: "GPT-5.4", Default: true, Available: true, Source: "cache", Chat: true, Stream: true, Coding: true},
	}
	if err := sqliteStore.ReplaceProviderModels(ctx, "codex_managed", models); err != nil {
		t.Fatalf("ReplaceProviderModels returned error: %v", err)
	}
	preference := providers.Preference{
		ProviderID:   "codex_managed",
		DefaultModel: "gpt-5.4",
		UpdatedAt:    time.Now().UTC(),
	}
	if err := sqliteStore.UpsertProviderPreference(ctx, preference); err != nil {
		t.Fatalf("UpsertProviderPreference returned error: %v", err)
	}

	restoredRouter := router.NewSessionRouter()
	restoredRuntime := runtime.NewManager()
	restoredEventBus := events.NewBus()
	restoredCheckpoints := checkpoints.NewManager(sqliteStore, restoredRuntime)
	restoredConnectors := connectors.NewSupervisor()
	restoredCapabilities := capabilities.NewSupervisor()
	restoredPolicy := policy.NewEngine()
	restoredAuth := auth.NewManager()
	providerManager := providers.NewManager(config.Config{}, llm.NewDispatcher())

	if err := recoverPersistedState(ctx, sqliteStore, restoredRouter, restoredCheckpoints, restoredEventBus, restoredConnectors, restoredCapabilities, restoredPolicy, restoredAuth, providerManager, nil); err != nil {
		t.Fatalf("recoverPersistedState returned error: %v", err)
	}

	state, ok := providerManager.GetAuthState("codex_managed")
	if !ok {
		t.Fatal("expected restored provider auth state")
	}
	if state.Status != providers.AuthStatusAuthenticated {
		t.Fatalf("expected restored authenticated state, got %s", state.Status)
	}
	if state.Metadata["managedProviderAction"] != "auth_status" {
		t.Fatalf("expected restored managed-provider metadata, got %+v", state.Metadata)
	}
	persistedModels, ok := providerManager.ListModels("codex_managed")
	if !ok || len(persistedModels) != 1 {
		t.Fatalf("expected restored provider models, got %+v", persistedModels)
	}
	if persistedModels[0].ModelID != "gpt-5.4" {
		t.Fatalf("expected restored model gpt-5.4, got %s", persistedModels[0].ModelID)
	}
	if pref, ok := providerManager.GetPreference("codex_managed"); !ok || pref.DefaultModel != "gpt-5.4" {
		t.Fatalf("expected restored provider preference, got %+v ok=%v", pref, ok)
	}
}

func TestNewRejectsInvalidProviderConfig(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dope")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"llm": {
			"defaultProvider": "openai_compatible",
			"openaiCompatible": {
				"baseURL": "not-a-url",
				"apiKey": "secret",
				"model": "gpt-test"
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")

	if _, err := New(); err == nil {
		t.Fatal("expected invalid provider config to fail startup")
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
