package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func decodeStrictResponse[T any](t *testing.T, body []byte) T {
	t.Helper()

	var value T
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("failed to strict-decode response: %v\nbody=%s", err, string(body))
	}

	return value
}

type testLLMProvider struct {
	name       string
	completeFn func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error)
	streamFn   func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error)
}

func (p *testLLMProvider) Name() string { return p.name }

func (p *testLLMProvider) Complete(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return p.completeFn(ctx, request)
}

func (p *testLLMProvider) Stream(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	return p.streamFn(ctx, request, emit)
}

func issueAuthHeaderForTest(t *testing.T, manager *auth.Manager, label string) string {
	t.Helper()

	pairing, code, err := manager.StartPairing(auth.StartPairingInput{
		Mode:  auth.PairingModeLocal,
		Label: label,
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

func TestRunsLifecycleRoutes(t *testing.T) {
	eventBus := events.NewBus()
	manager := runtime.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Router:       router.NewSessionRouter(),
		Runtime:      manager,
		Capabilities: capabilitySupervisor,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"chat","goal":"ship a task"}`))
	createRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}

	created := decodeStrictResponse[runtime.Run](t, createRec.Body.Bytes())
	if created.RunID == "" {
		t.Fatal("expected created run ID")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/runs", nil)
	listRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+created.RunID, nil)
	getRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+created.RunID+"/events", nil)
	eventsRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(eventsRec, eventsReq)

	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for run events, got %d", eventsRec.Code)
	}

	eventList := decodeStrictResponse[EventListResponse](t, eventsRec.Body.Bytes())
	if len(eventList.Items) != 1 {
		t.Fatalf("expected 1 run-scoped event after run create, got %d", len(eventList.Items))
	}
	if eventList.Items[0].Name != "run.created" {
		t.Fatalf("expected run.created event, got %s", eventList.Items[0].Name)
	}

	createStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+created.RunID+"/steps", strings.NewReader(`{"title":"plan the task","kind":"task"}`))
	createStepRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createStepRec, createStepReq)

	if createStepRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for step create, got %d", createStepRec.Code)
	}

	createdStep := decodeStrictResponse[runtime.Step](t, createStepRec.Body.Bytes())
	if createdStep.StepID == "" {
		t.Fatal("expected created step ID")
	}

	listStepsReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+created.RunID+"/steps", nil)
	listStepsRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(listStepsRec, listStepsReq)

	if listStepsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for step list, got %d", listStepsRec.Code)
	}

	getStepReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+created.RunID+"/steps/"+createdStep.StepID, nil)
	getStepRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(getStepRec, getStepReq)

	if getStepRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for step get, got %d", getStepRec.Code)
	}

	eventsReq = httptest.NewRequest(http.MethodGet, "/v1/runs/"+created.RunID+"/events", nil)
	eventsRec = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(eventsRec, eventsReq)

	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for run events after step create, got %d", eventsRec.Code)
	}
	eventList = decodeStrictResponse[EventListResponse](t, eventsRec.Body.Bytes())
	if len(eventList.Items) != 2 {
		t.Fatalf("expected 2 run-scoped events after step create, got %d", len(eventList.Items))
	}
	if eventList.Items[1].Name != "step.created" {
		t.Fatalf("expected step.created event, got %s", eventList.Items[1].Name)
	}

	updateStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+created.RunID+"/steps/"+createdStep.StepID+"/status", strings.NewReader(`{"status":"planning"}`))
	updateStepRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(updateStepRec, updateStepReq)

	if updateStepRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for step status update, got %d", updateStepRec.Code)
	}

	createdStep = decodeStrictResponse[runtime.Step](t, updateStepRec.Body.Bytes())
	if createdStep.Status != runtime.StepStatusPlanning {
		t.Fatalf("expected planning step status, got %s", createdStep.Status)
	}

	eventsReq = httptest.NewRequest(http.MethodGet, "/v1/runs/"+created.RunID+"/events", nil)
	eventsRec = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(eventsRec, eventsReq)

	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for run events after step status update, got %d", eventsRec.Code)
	}
	eventList = decodeStrictResponse[EventListResponse](t, eventsRec.Body.Bytes())
	if len(eventList.Items) != 4 {
		t.Fatalf("expected 4 run-scoped events after step status update, got %d", len(eventList.Items))
	}
	if eventList.Items[2].Name != "step.status_changed" {
		t.Fatalf("expected step.status_changed event, got %s", eventList.Items[2].Name)
	}
	if eventList.Items[3].Name != "run.status_changed" {
		t.Fatalf("expected run.status_changed event, got %s", eventList.Items[3].Name)
	}

	getReq = httptest.NewRequest(http.MethodGet, "/v1/runs/"+created.RunID, nil)
	getRec = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 after run status update, got %d", getRec.Code)
	}
	created = decodeStrictResponse[runtime.Run](t, getRec.Body.Bytes())
	if created.Status != runtime.RunStatusRunning {
		t.Fatalf("expected run status running, got %s", created.Status)
	}
}

func TestCreateRunRequiresBodyAndEntrypoint(t *testing.T) {
	manager := runtime.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  manager,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateStepRequiresRunAndTitle(t *testing.T) {
	manager := runtime.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  manager,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/run_missing/steps", strings.NewReader(`{"title":"plan the task"}`))
	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing run, got %d", rec.Code)
	}

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid step create, got %d", rec.Code)
	}
}

func TestUpdateStepStatusRejectsInvalidTransition(t *testing.T) {
	manager := runtime.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  manager,
	})

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "plan the task",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/status", strings.NewReader(`{"status":"completed"}`))
	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid transition, got %d", rec.Code)
	}
}

func TestToolCallLifecycleRoutes(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	eventBus := events.NewBus()
	manager := runtime.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Router:       router.NewSessionRouter(),
		Runtime:      manager,
		Capabilities: capabilitySupervisor,
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
	})

	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "shell",
		Kind:         "exec",
		DisplayName:  "Shell",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "execute shell command",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd"}}`))
	createRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for tool call create, got %d", createRec.Code)
	}

	created := decodeStrictResponse[runtime.ToolCall](t, createRec.Body.Bytes())
	if created.ToolCallID == "" {
		t.Fatal("expected tool call ID")
	}
	if created.CapabilityID != "shell" {
		t.Fatalf("expected capability id shell, got %s", created.CapabilityID)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", nil)
	listRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tool call list, got %d", listRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls/"+created.ToolCallID, nil)
	getRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tool call get, got %d", getRec.Code)
	}

	completeReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls/"+created.ToolCallID+"/complete", strings.NewReader(`{"output":{"exitCode":0}}`))
	completeRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tool call complete, got %d", completeRec.Code)
	}

	created = decodeStrictResponse[runtime.ToolCall](t, completeRec.Body.Bytes())
	if created.Status != runtime.ToolCallStatusCompleted {
		t.Fatalf("expected completed tool call status, got %s", created.Status)
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/events", nil)
	eventsRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(eventsRec, eventsReq)
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for run events, got %d", eventsRec.Code)
	}

	eventList := decodeStrictResponse[EventListResponse](t, eventsRec.Body.Bytes())
	if len(eventList.Items) != 2 {
		t.Fatalf("expected 2 tool call events, got %d", len(eventList.Items))
	}
	if eventList.Items[0].Name != "tool_call.requested" {
		t.Fatalf("expected tool_call.requested, got %s", eventList.Items[0].Name)
	}
	if eventList.Items[1].Name != "tool_call.completed" {
		t.Fatalf("expected tool_call.completed, got %s", eventList.Items[1].Name)
	}

	persistedToolCalls, err := sqliteStore.ListToolCalls(context.Background(), run.RunID, step.StepID)
	if err != nil {
		t.Fatalf("ListToolCalls returned error: %v", err)
	}
	if len(persistedToolCalls) != 1 {
		t.Fatalf("expected 1 persisted tool call, got %d", len(persistedToolCalls))
	}
	if persistedToolCalls[0].CapabilityID != "shell" {
		t.Fatalf("expected persisted capability id shell, got %s", persistedToolCalls[0].CapabilityID)
	}
}

func TestToolCallFailRoute(t *testing.T) {
	eventBus := events.NewBus()
	manager := runtime.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Router:       router.NewSessionRouter(),
		Runtime:      manager,
		Capabilities: capabilitySupervisor,
	})

	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "shell",
		Kind:         "exec",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{
		Title: "execute shell command",
	})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	toolCall, err := manager.CreateToolCall(run.RunID, step.StepID, runtime.CreateToolCallInput{
		CapabilityID: "shell",
		ToolName:     "shell",
	})
	if err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}

	failReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls/"+toolCall.ToolCallID+"/fail", strings.NewReader(`{"error":"command failed"}`))
	failRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(failRec, failReq)

	if failRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tool call fail, got %d", failRec.Code)
	}
	toolCall = decodeStrictResponse[runtime.ToolCall](t, failRec.Body.Bytes())
	if toolCall.Status != runtime.ToolCallStatusFailed {
		t.Fatalf("expected failed tool call status, got %s", toolCall.Status)
	}
}

func TestRunLifecyclePersistsToSQLiteStore(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	eventBus := events.NewBus()
	manager := runtime.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: eventBus,
		Router:   router.NewSessionRouter(),
		Runtime:  manager,
		Store:    sqliteStore,
	})

	createRunReq := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"chat","goal":"persist this run"}`))
	createRunRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createRunRec, createRunReq)

	if createRunRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for run create, got %d", createRunRec.Code)
	}

	createdRun := decodeStrictResponse[runtime.Run](t, createRunRec.Body.Bytes())

	createStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps", strings.NewReader(`{"title":"persist this step","kind":"task"}`))
	createStepRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createStepRec, createStepReq)

	if createStepRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for step create, got %d", createStepRec.Code)
	}

	createdStep := decodeStrictResponse[runtime.Step](t, createStepRec.Body.Bytes())

	updateStepReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+createdRun.RunID+"/steps/"+createdStep.StepID+"/status", strings.NewReader(`{"status":"planning"}`))
	updateStepRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(updateStepRec, updateStepReq)

	if updateStepRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for step status update, got %d", updateStepRec.Code)
	}

	ctx := context.Background()

	runs, err := sqliteStore.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 persisted run, got %d", len(runs))
	}
	if runs[0].Status != runtime.RunStatusRunning {
		t.Fatalf("expected persisted run status running, got %s", runs[0].Status)
	}

	sessions, err := sqliteStore.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 persisted session, got %d", len(sessions))
	}

	steps, err := sqliteStore.ListSteps(ctx, createdRun.RunID)
	if err != nil {
		t.Fatalf("ListSteps returned error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 persisted step, got %d", len(steps))
	}
	if steps[0].Status != runtime.StepStatusPlanning {
		t.Fatalf("expected persisted step status planning, got %s", steps[0].Status)
	}

	items, err := sqliteStore.ListEvents(ctx, events.Filter{RunID: createdRun.RunID})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 persisted run-scoped events, got %d", len(items))
	}
}

func TestConnectorAndCapabilitySupervisionRoutes(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	eventBus := events.NewBus()
	logger := telemetry.New("error")
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Router:       router.NewSessionRouter(),
		Runtime:      runtime.NewManager(),
		Connectors:   connectorSupervisor,
		Capabilities: capabilitySupervisor,
		Store:        sqliteStore,
	})

	connectorCreateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(connectorCreateRec, httptest.NewRequest(http.MethodPost, "/v1/connectors", strings.NewReader(`{"connectorId":"slack-main","kind":"slack","displayName":"Slack Main"}`)))
	if connectorCreateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for connector create, got %d", connectorCreateRec.Code)
	}
	connector := decodeStrictResponse[connectors.Connector](t, connectorCreateRec.Body.Bytes())
	if connector.Status != connectors.StatusRegistered {
		t.Fatalf("expected registered connector, got %s", connector.Status)
	}

	connectorFailRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(connectorFailRec, httptest.NewRequest(http.MethodPost, "/v1/connectors/"+connector.ConnectorID+"/fail", strings.NewReader(`{"reason":"socket disconnected"}`)))
	if connectorFailRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for connector fail, got %d", connectorFailRec.Code)
	}
	connector = decodeStrictResponse[connectors.Connector](t, connectorFailRec.Body.Bytes())
	if connector.Status != connectors.StatusBackingOff {
		t.Fatalf("expected backing_off connector, got %s", connector.Status)
	}

	connectorRestartRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(connectorRestartRec, httptest.NewRequest(http.MethodPost, "/v1/connectors/"+connector.ConnectorID+"/restart", nil))
	if connectorRestartRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for connector restart, got %d", connectorRestartRec.Code)
	}
	connector = decodeStrictResponse[connectors.Connector](t, connectorRestartRec.Body.Bytes())
	if connector.RestartCount != 1 {
		t.Fatalf("expected connector restart count 1, got %d", connector.RestartCount)
	}

	capabilityCreateRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(capabilityCreateRec, httptest.NewRequest(http.MethodPost, "/v1/capabilities", strings.NewReader(`{"capabilityId":"shell","kind":"exec","displayName":"Shell"}`)))
	if capabilityCreateRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for capability create, got %d", capabilityCreateRec.Code)
	}
	capability := decodeStrictResponse[capabilities.Capability](t, capabilityCreateRec.Body.Bytes())

	capabilityHealthRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(capabilityHealthRec, httptest.NewRequest(http.MethodPost, "/v1/capabilities/"+capability.CapabilityID+"/health", strings.NewReader(`{"status":"healthy"}`)))
	if capabilityHealthRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for capability health, got %d", capabilityHealthRec.Code)
	}
	capability = decodeStrictResponse[capabilities.Capability](t, capabilityHealthRec.Body.Bytes())
	if capability.Status != capabilities.StatusHealthy {
		t.Fatalf("expected healthy capability, got %s", capability.Status)
	}

	capabilityListRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(capabilityListRec, httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil))
	if capabilityListRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for capability list, got %d", capabilityListRec.Code)
	}
	capabilityList := decodeStrictResponse[ListResponse[capabilities.Capability]](t, capabilityListRec.Body.Bytes())
	if len(capabilityList.Items) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(capabilityList.Items))
	}

	connectorEvents := eventBus.List(events.Filter{Category: "connector"})
	if len(connectorEvents) < 3 {
		t.Fatalf("expected connector events, got %d", len(connectorEvents))
	}
	capabilityEvents := eventBus.List(events.Filter{Category: "capability"})
	if len(capabilityEvents) < 2 {
		t.Fatalf("expected capability events, got %d", len(capabilityEvents))
	}

	persistedConnectors, err := sqliteStore.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("ListConnectors returned error: %v", err)
	}
	if len(persistedConnectors) != 1 {
		t.Fatalf("expected 1 persisted connector, got %d", len(persistedConnectors))
	}
	if persistedConnectors[0].RestartCount != 1 {
		t.Fatalf("expected persisted connector restart count 1, got %d", persistedConnectors[0].RestartCount)
	}

	persistedCapabilities, err := sqliteStore.ListCapabilities(context.Background())
	if err != nil {
		t.Fatalf("ListCapabilities returned error: %v", err)
	}
	if len(persistedCapabilities) != 1 {
		t.Fatalf("expected 1 persisted capability, got %d", len(persistedCapabilities))
	}
	if persistedCapabilities[0].Status != capabilities.StatusHealthy {
		t.Fatalf("expected persisted capability healthy, got %s", persistedCapabilities[0].Status)
	}
}

func TestSessionRoutesAndReset(t *testing.T) {
	eventBus := events.NewBus()
	sessionRouter := router.NewSessionRouter()
	manager := runtime.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: eventBus,
		Router:   sessionRouter,
		Runtime:  manager,
	})

	createRunReq := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"chat","goal":"create a session"}`))
	createRunRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createRunRec, createRunReq)
	if createRunRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for run create, got %d", createRunRec.Code)
	}

	createdRun := decodeStrictResponse[runtime.Run](t, createRunRec.Body.Bytes())
	if createdRun.SessionID == "" {
		t.Fatal("expected run to be bound to a session")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	listRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for session list, got %d", listRec.Code)
	}

	sessionList := decodeStrictResponse[ListResponse[router.Session]](t, listRec.Body.Bytes())
	if len(sessionList.Items) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessionList.Items))
	}
	if sessionList.Items[0].Kind != router.SessionKindDirect {
		t.Fatalf("expected direct session kind, got %s", sessionList.Items[0].Kind)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+createdRun.SessionID, nil)
	getRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for session get, got %d", getRec.Code)
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+createdRun.SessionID+"/reset", nil)
	resetRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for session reset, got %d", resetRec.Code)
	}

	resetSession := decodeStrictResponse[router.Session](t, resetRec.Body.Bytes())
	if resetSession.Generation != 2 {
		t.Fatalf("expected generation 2 after reset, got %d", resetSession.Generation)
	}

	eventsReq := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+createdRun.SessionID+"/events", nil)
	eventsRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(eventsRec, eventsReq)
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for session events, got %d", eventsRec.Code)
	}

	eventList := decodeStrictResponse[EventListResponse](t, eventsRec.Body.Bytes())
	if len(eventList.Items) != 4 {
		t.Fatalf("expected 4 session-scoped events, got %d", len(eventList.Items))
	}
	if eventList.Items[0].Name != "session.created" {
		t.Fatalf("expected session.created event, got %s", eventList.Items[0].Name)
	}
	if eventList.Items[3].Name != "session.reset" {
		t.Fatalf("expected session.reset event, got %s", eventList.Items[3].Name)
	}
}

func TestCreateRunWithExplicitRoute(t *testing.T) {
	eventBus := events.NewBus()
	sessionRouter := router.NewSessionRouter()
	manager := runtime.NewManager()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	defer func() {
		if err := checkpointManager.Close(); err != nil {
			t.Fatalf("Close checkpoint manager returned error: %v", err)
		}
	}()

	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Router:      sessionRouter,
		Runtime:     manager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{
		"entrypoint":"connector.message",
		"goal":"route-aware run",
		"route":{
			"kind":"group",
			"channel":"telegram",
			"accountId":"bot-main",
			"peerId":"chat-1",
			"threadId":"thread-1"
		}
	}`))
	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for route-based run create, got %d body=%s", rec.Code, rec.Body.String())
	}

	createdRun := decodeStrictResponse[runtime.Run](t, rec.Body.Bytes())
	if createdRun.SessionID == "" {
		t.Fatal("expected created run to be bound to a routed session")
	}

	session, ok := sessionRouter.GetSession(createdRun.SessionID)
	if !ok {
		t.Fatal("expected routed session to exist")
	}
	if session.Kind != router.SessionKindGroup {
		t.Fatalf("expected group session, got %s", session.Kind)
	}
	if session.Channel != "telegram" {
		t.Fatalf("expected telegram channel, got %s", session.Channel)
	}

	persistedRuns, err := sqliteStore.ListRuns(context.Background())
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(persistedRuns) != 1 || persistedRuns[0].SessionID != createdRun.SessionID {
		t.Fatalf("expected persisted run bound to session %s, got %+v", createdRun.SessionID, persistedRuns)
	}
}

func TestConnectorIngressRoutesSessionAndCreatesRun(t *testing.T) {
	eventBus := events.NewBus()
	sessionRouter := router.NewSessionRouter()
	manager := runtime.NewManager()
	connectorSupervisor := connectors.NewSupervisor()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	defer func() {
		if err := checkpointManager.Close(); err != nil {
			t.Fatalf("Close checkpoint manager returned error: %v", err)
		}
	}()

	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Router:      sessionRouter,
		Runtime:     manager,
		Connectors:  connectorSupervisor,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})

	registerReq := httptest.NewRequest(http.MethodPost, "/v1/connectors", strings.NewReader(`{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`))
	registerRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for connector register, got %d", registerRec.Code)
	}

	ingressReq := httptest.NewRequest(http.MethodPost, "/v1/connectors/telegram-main/ingress/messages", strings.NewReader(`{
		"route":{
			"kind":"direct",
			"accountId":"bot-main",
			"peerId":"dm-1"
		},
		"message":{
			"messageId":"msg_1",
			"text":"hello"
		},
		"run":{
			"entrypoint":"connector.message",
			"goal":"handle inbound message"
		}
	}`))
	ingressRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(ingressRec, ingressReq)
	if ingressRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for connector ingress, got %d body=%s", ingressRec.Code, ingressRec.Body.String())
	}

	response := decodeStrictResponse[ConnectorIngressMessageResponse](t, ingressRec.Body.Bytes())
	if response.ConnectorID != "telegram-main" {
		t.Fatalf("expected connector telegram-main, got %s", response.ConnectorID)
	}
	if !response.SessionCreated {
		t.Fatal("expected ingress to create a new session")
	}
	if !response.RunCreated || response.Run == nil {
		t.Fatal("expected ingress to create a run")
	}
	if response.Session.Channel != "telegram" {
		t.Fatalf("expected ingress session channel telegram, got %s", response.Session.Channel)
	}
	if response.Run.SessionID != response.Session.SessionID {
		t.Fatalf("expected ingress run to bind to session %s, got %s", response.Session.SessionID, response.Run.SessionID)
	}

	persistedSessions, err := sqliteStore.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(persistedSessions) != 1 {
		t.Fatalf("expected 1 persisted session, got %d", len(persistedSessions))
	}
	persistedRuns, err := sqliteStore.ListRuns(context.Background())
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(persistedRuns) != 1 {
		t.Fatalf("expected 1 persisted run, got %d", len(persistedRuns))
	}
	if persistedRuns[0].SessionID != response.Session.SessionID {
		t.Fatalf("expected persisted run session %s, got %s", response.Session.SessionID, persistedRuns[0].SessionID)
	}

	connectorEvents := eventBus.List(events.Filter{Category: "connector"})
	if len(connectorEvents) != 2 {
		t.Fatalf("expected 2 connector events, got %d", len(connectorEvents))
	}
	if connectorEvents[1].Name != "connector.ingress_accepted" {
		t.Fatalf("expected connector.ingress_accepted, got %s", connectorEvents[1].Name)
	}

	sessionEvents := eventBus.List(events.Filter{SessionID: response.Session.SessionID})
	if len(sessionEvents) < 3 {
		t.Fatalf("expected session-scoped routing and run events, got %d", len(sessionEvents))
	}
}

func TestPolicyApprovalLifecycleRoutes(t *testing.T) {
	eventBus := events.NewBus()
	sessionRouter := router.NewSessionRouter()
	manager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: eventBus,
		Policy:   policyEngine,
		Router:   sessionRouter,
		Runtime:  manager,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals", strings.NewReader(`{
		"action":"capability.exec",
		"resourceKind":"capability",
		"resourceId":"shell",
		"reason":"shell execution requires approval",
		"requestedBy":"operator"
	}`))
	createRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approval create, got %d", createRec.Code)
	}

	var created struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode approval create response: %v", err)
	}
	if created.Approval.ApprovalID == "" {
		t.Fatal("expected approval ID")
	}
	if created.Decision.Outcome != policy.DecisionOutcomeRequiresApproval {
		t.Fatalf("expected requires_approval outcome, got %s", created.Decision.Outcome)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals?status=pending", nil)
	listRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval list, got %d", listRec.Code)
	}

	var list struct {
		Items []policy.Approval `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode approval list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(list.Items))
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+created.Approval.ApprovalID+"/resolve", strings.NewReader(`{
		"resolution":"approved",
		"comment":"approved for local shell execution"
	}`))
	resolveRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval resolve, got %d", resolveRec.Code)
	}

	var resolved struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("failed to decode approval resolve response: %v", err)
	}
	if resolved.Approval.Status != policy.ApprovalStatusApproved {
		t.Fatalf("expected approved status, got %s", resolved.Approval.Status)
	}
	if resolved.Decision.Outcome != policy.DecisionOutcomeApproved {
		t.Fatalf("expected approved decision outcome, got %s", resolved.Decision.Outcome)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+created.Approval.ApprovalID, nil)
	getRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get, got %d", getRec.Code)
	}

	policyEvents := eventBus.List(events.Filter{Category: "policy"})
	if len(policyEvents) != 4 {
		t.Fatalf("expected 4 policy events, got %d", len(policyEvents))
	}
	if policyEvents[0].Name != "policy.approval_requested" {
		t.Fatalf("expected policy.approval_requested, got %s", policyEvents[0].Name)
	}
	if policyEvents[1].Name != "policy.decision_recorded" {
		t.Fatalf("expected first policy.decision_recorded, got %s", policyEvents[1].Name)
	}
	if policyEvents[2].Name != "policy.approval_resolved" {
		t.Fatalf("expected policy.approval_resolved, got %s", policyEvents[2].Name)
	}
	if policyEvents[3].Name != "policy.decision_recorded" {
		t.Fatalf("expected second policy.decision_recorded, got %s", policyEvents[3].Name)
	}
}

func TestEventStreamReplaysMatchingHistory(t *testing.T) {
	eventBus := events.NewBus()
	manager := runtime.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: eventBus,
		Router:   router.NewSessionRouter(),
		Runtime:  manager,
	})

	run, err := manager.CreateRun(runtime.CreateRunInput{
		Entrypoint: "chat",
		Goal:       "stream events",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	eventBus.Publish(events.Event{
		Category: "run",
		Name:     "run.created",
		Scope: events.Scope{
			RunID: run.RunID,
		},
		Resource: events.Resource{
			Kind: "run",
			ID:   run.RunID,
		},
		Payload: map[string]any{
			"entrypoint": run.Entrypoint,
			"goal":       run.Goal,
		},
	})

	testServer := httptest.NewServer(server.server.Handler)
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/v1/events/stream?runId="+run.RunID, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	body := resp.Body
	defer body.Close()

	reader := bufio.NewReader(body)
	var chunks []string
	for range 6 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed reading SSE response: %v", err)
		}
		chunks = append(chunks, line)
		if strings.Contains(strings.Join(chunks, ""), "run.created") {
			break
		}
	}

	if !strings.Contains(strings.Join(chunks, ""), "run.created") {
		t.Fatalf("expected SSE stream to contain run.created, got %q", strings.Join(chunks, ""))
	}
	if !strings.Contains(strings.Join(chunks, ""), "id: 1") {
		t.Fatalf("expected SSE stream to contain cursor id, got %q", strings.Join(chunks, ""))
	}
}

func TestConfigRouteUsesStrictResponseShape(t *testing.T) {
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "/tmp/dope",
			LogLevel: "info",
			Version:  "test",
			LLM: config.LLMConfig{
				DefaultTimeoutMs: 30000,
			},
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/config", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	response := decodeStrictResponse[ConfigResponse](t, rec.Body.Bytes())
	if response.ConfigFilePath == "" {
		t.Fatal("expected config file path")
	}
	if len(response.RedactedFields) != 0 {
		t.Fatalf("expected no redacted fields, got %+v", response.RedactedFields)
	}
}

func TestConfigRouteRedactsProviderSecrets(t *testing.T) {
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "/tmp/dope",
			LogLevel: "info",
			Version:  "test",
			LLM: config.LLMConfig{
				DefaultProvider:  "openai_compatible",
				DefaultModel:     "gpt-test",
				DefaultTimeoutMs: 30000,
				OpenAICompatible: config.OpenAICompatibleProviderConfig{
					BaseURL:   "https://api.example.com/v1",
					APIKey:    "secret",
					APIKeyEnv: "OPENAI_API_KEY",
					Model:     "gpt-test",
					TimeoutMs: 30000,
				},
			},
		},
		Logger:   telemetry.New("error").Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
	})

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	response := decodeStrictResponse[ConfigResponse](t, rec.Body.Bytes())
	if len(response.RedactedFields) != 1 || response.RedactedFields[0] != "llm.openaiCompatible.apiKey" {
		t.Fatalf("expected redacted api key field, got %+v", response.RedactedFields)
	}
	if !response.LLM.OpenAICompatible.APIKeyConfigured {
		t.Fatal("expected apiKeyConfigured true")
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("response leaked secret: %s", rec.Body.String())
	}
}

func TestAuthPairingAndProtectedRoutes(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	authManager := auth.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Auth:     authManager,
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		Store:    sqliteStore,
	})

	unauthorizedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorizedRec, httptest.NewRequest(http.MethodGet, "/v1/config", nil))
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %d", unauthorizedRec.Code)
	}

	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/start", strings.NewReader(`{"mode":"local","label":"web-ui"}`)))
	if startRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for pairing start, got %d body=%s", startRec.Code, startRec.Body.String())
	}
	var pairingStart struct {
		Pairing     auth.Pairing `json:"pairing"`
		PairingCode string       `json:"pairingCode"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &pairingStart); err != nil {
		t.Fatalf("failed to decode pairing start response: %v", err)
	}
	if pairingStart.Pairing.PairingID == "" || pairingStart.PairingCode == "" {
		t.Fatal("expected pairing id and pairing code")
	}

	completeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(completeRec, httptest.NewRequest(http.MethodPost, "/v1/auth/pairings/"+pairingStart.Pairing.PairingID+"/complete", strings.NewReader(`{"code":"`+pairingStart.PairingCode+`"}`)))
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for pairing complete, got %d body=%s", completeRec.Code, completeRec.Body.String())
	}
	var pairingComplete struct {
		Pairing     auth.Pairing     `json:"pairing"`
		Token       auth.AccessToken `json:"token"`
		AccessToken string           `json:"accessToken"`
	}
	if err := json.Unmarshal(completeRec.Body.Bytes(), &pairingComplete); err != nil {
		t.Fatalf("failed to decode pairing complete response: %v", err)
	}
	if pairingComplete.AccessToken == "" {
		t.Fatal("expected access token")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+pairingComplete.AccessToken)
	meRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth me, got %d body=%s", meRec.Code, meRec.Body.String())
	}
	me := decodeStrictResponse[auth.AccessToken](t, meRec.Body.Bytes())
	if me.TokenID != pairingComplete.Token.TokenID {
		t.Fatalf("expected token ID %s, got %s", pairingComplete.Token.TokenID, me.TokenID)
	}

	pairings, err := sqliteStore.ListPairings(context.Background())
	if err != nil {
		t.Fatalf("ListPairings returned error: %v", err)
	}
	if len(pairings) != 1 || pairings[0].Status != auth.PairingStatusCompleted {
		t.Fatalf("expected one completed persisted pairing, got %+v", pairings)
	}
	tokens, err := sqliteStore.ListAccessTokens(context.Background())
	if err != nil {
		t.Fatalf("ListAccessTokens returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].TokenID != pairingComplete.Token.TokenID {
		t.Fatalf("expected one persisted token, got %+v", tokens)
	}
}

func TestToolCallApprovalEnforcement(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	eventBus := events.NewBus()
	manager := runtime.NewManager()
	authManager := auth.NewManager()
	policyEngine := policy.NewEngine()
	capabilitySupervisor := capabilities.NewSupervisor()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Auth:         authManager,
		Policy:       policyEngine,
		Router:       router.NewSessionRouter(),
		Runtime:      manager,
		Capabilities: capabilitySupervisor,
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "web-ui")

	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "shell",
		Kind:         "exec",
		DisplayName:  "Shell",
	}); err != nil {
		t.Fatalf("Register shell capability returned error: %v", err)
	}
	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "search",
		Kind:         "knowledge",
		DisplayName:  "Search",
	}); err != nil {
		t.Fatalf("Register search capability returned error: %v", err)
	}

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "run guarded tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	pendingReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd"}}`))
	pendingReq.Header.Set("Authorization", authHeader)
	pendingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(pendingRec, pendingReq)
	if pendingRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for pending approval, got %d body=%s", pendingRec.Code, pendingRec.Body.String())
	}
	var pending struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(pendingRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("failed to decode pending approval response: %v", err)
	}
	if pending.Approval.Status != policy.ApprovalStatusPending {
		t.Fatalf("expected pending approval, got %s", pending.Approval.Status)
	}

	_, _, err = policyEngine.ResolveApproval(pending.Approval.ApprovalID, policy.ResolveApprovalInput{
		Resolution: string(policy.ApprovalStatusRejected),
		Comment:    "rejected for test",
	})
	if err != nil {
		t.Fatalf("ResolveApproval rejected returned error: %v", err)
	}
	rejectedApproval, ok := policyEngine.GetApproval(pending.Approval.ApprovalID)
	if !ok {
		t.Fatal("expected rejected approval")
	}
	if err := sqliteStore.UpsertApproval(context.Background(), rejectedApproval); err != nil {
		t.Fatalf("UpsertApproval returned error: %v", err)
	}

	rejectedReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","approvalId":"`+pending.Approval.ApprovalID+`"}`))
	rejectedReq.Header.Set("Authorization", authHeader)
	rejectedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rejectedRec, rejectedReq)
	if rejectedRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for rejected approval, got %d body=%s", rejectedRec.Code, rejectedRec.Body.String())
	}

	approvedApproval, _, err := policyEngine.RequestApproval(policy.RequestApprovalInput{
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Reason:       "approve shell",
		RequestedBy:  "web-ui",
	})
	if err != nil {
		t.Fatalf("RequestApproval returned error: %v", err)
	}
	approvedApproval, _, err = policyEngine.ResolveApproval(approvedApproval.ApprovalID, policy.ResolveApprovalInput{
		Resolution: string(policy.ApprovalStatusApproved),
		Comment:    "approved for test",
	})
	if err != nil {
		t.Fatalf("ResolveApproval approved returned error: %v", err)
	}
	if err := sqliteStore.UpsertApproval(context.Background(), approvedApproval); err != nil {
		t.Fatalf("UpsertApproval returned error: %v", err)
	}

	approvedReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","approvalId":"`+approvedApproval.ApprovalID+`"}`))
	approvedReq.Header.Set("Authorization", authHeader)
	approvedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(approvedRec, approvedReq)
	if approvedRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approved tool call, got %d body=%s", approvedRec.Code, approvedRec.Body.String())
	}

	allowedReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"search","toolName":"lookup","input":{"q":"hi"}}`))
	allowedReq.Header.Set("Authorization", authHeader)
	allowedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for low-risk tool call, got %d body=%s", allowedRec.Code, allowedRec.Body.String())
	}
}

func TestLLMDispatchRoutes(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: eventBus,
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		LLM:      dispatcher,
		Store:    sqliteStore,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/llm/dispatches", strings.NewReader(`{"provider":"echo","model":"test-model","messages":[{"role":"user","content":"hello world"}]}`))
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for llm dispatch create, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	created := decodeStrictResponse[llm.Dispatch](t, createRec.Body.Bytes())
	if created.Status != llm.DispatchStatusCompleted {
		t.Fatalf("expected completed dispatch, got %s", created.Status)
	}
	if created.Provider != "echo" {
		t.Fatalf("expected provider echo, got %s", created.Provider)
	}
	if created.Usage.TotalTokens == 0 {
		t.Fatal("expected usage accounting to be recorded")
	}

	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/v1/llm/dispatches", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for llm dispatch list, got %d", listRec.Code)
	}
	list := decodeStrictResponse[ListResponse[llm.Dispatch]](t, listRec.Body.Bytes())
	if len(list.Items) != 1 {
		t.Fatalf("expected 1 llm dispatch, got %d", len(list.Items))
	}

	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/llm/dispatches/"+created.DispatchID, nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for llm dispatch get, got %d", getRec.Code)
	}
	got := decodeStrictResponse[llm.Dispatch](t, getRec.Body.Bytes())
	if got.DispatchID != created.DispatchID {
		t.Fatalf("expected dispatch ID %s, got %s", created.DispatchID, got.DispatchID)
	}

	persisted, ok, err := sqliteStore.GetLLMDispatch(context.Background(), created.DispatchID)
	if err != nil {
		t.Fatalf("GetLLMDispatch returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected persisted llm dispatch")
	}
	if persisted.Status != llm.DispatchStatusCompleted {
		t.Fatalf("expected persisted completed dispatch, got %s", persisted.Status)
	}

	llmEvents := eventBus.List(events.Filter{Category: "llm"})
	if len(llmEvents) != 2 {
		t.Fatalf("expected 2 llm events, got %d", len(llmEvents))
	}
	if llmEvents[0].Name != "llm.dispatch.requested" {
		t.Fatalf("expected llm.dispatch.requested, got %s", llmEvents[0].Name)
	}
	if llmEvents[1].Name != "llm.dispatch.completed" {
		t.Fatalf("expected llm.dispatch.completed, got %s", llmEvents[1].Name)
	}
}

func TestLLMDispatchRetryAndTimeoutRoutes(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "retryable",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			if request.Attempt == 1 {
				return llm.ProviderResponse{}, &llm.ProviderError{Code: "upstream_unavailable", Message: "upstream unavailable", Retryable: true}
			}
			return llm.ProviderResponse{
				Output:       "recovered",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1},
			}, nil
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
	})
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "slow",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			select {
			case <-ctx.Done():
				return llm.ProviderResponse{}, ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return llm.ProviderResponse{Output: "slow"}, nil
			}
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
	})

	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		LLM:      dispatcher,
		Store:    sqliteStore,
	})

	retryRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(retryRec, httptest.NewRequest(http.MethodPost, "/v1/llm/dispatches", strings.NewReader(`{"provider":"retryable","model":"test-model","messages":[{"role":"user","content":"retry"}],"maxRetries":2}`)))
	if retryRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for retry dispatch, got %d body=%s", retryRec.Code, retryRec.Body.String())
	}
	retryDispatch := decodeStrictResponse[llm.Dispatch](t, retryRec.Body.Bytes())
	if retryDispatch.AttemptCount != 2 {
		t.Fatalf("expected retry dispatch attempt count 2, got %d", retryDispatch.AttemptCount)
	}

	timeoutRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(timeoutRec, httptest.NewRequest(http.MethodPost, "/v1/llm/dispatches", strings.NewReader(`{"provider":"slow","model":"test-model","messages":[{"role":"user","content":"timeout"}],"timeoutMs":20}`)))
	if timeoutRec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 for timeout dispatch, got %d body=%s", timeoutRec.Code, timeoutRec.Body.String())
	}
	timeoutDispatch := decodeStrictResponse[llm.Dispatch](t, timeoutRec.Body.Bytes())
	if timeoutDispatch.ErrorCode != "timeout" {
		t.Fatalf("expected timeout error code, got %s", timeoutDispatch.ErrorCode)
	}
}

func TestLLMDispatchStreamRoute(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "stream-provider",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			if err := emit(llm.StreamChunk{Delta: "hello"}); err != nil {
				return llm.ProviderResponse{}, err
			}
			if err := emit(llm.StreamChunk{Delta: " world"}); err != nil {
				return llm.ProviderResponse{}, err
			}
			return llm.ProviderResponse{
				Output:       "hello world",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 1, OutputTokens: 2},
			}, nil
		},
	})

	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		LLM:      dispatcher,
		Store:    sqliteStore,
	})

	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/v1/llm/dispatches/stream", strings.NewReader(`{"provider":"stream-provider","model":"test-model","messages":[{"role":"user","content":"stream"}]}`))
	if err != nil {
		t.Fatalf("failed to create stream request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute stream request: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var chunks []string
	for range 12 {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		chunks = append(chunks, line)
		if strings.Contains(strings.Join(chunks, ""), "llm.dispatch.completed") {
			break
		}
	}

	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "event: llm.dispatch.started") {
		t.Fatalf("expected stream start event, got %q", joined)
	}
	if !strings.Contains(joined, "event: llm.dispatch.delta") {
		t.Fatalf("expected stream delta event, got %q", joined)
	}
	if !strings.Contains(joined, "event: llm.dispatch.completed") {
		t.Fatalf("expected stream completed event, got %q", joined)
	}

	dispatches, err := sqliteStore.ListLLMDispatches(context.Background())
	if err != nil {
		t.Fatalf("ListLLMDispatches returned error: %v", err)
	}
	if len(dispatches) != 1 {
		t.Fatalf("expected 1 persisted streamed dispatch, got %d", len(dispatches))
	}
	if dispatches[0].Status != llm.DispatchStatusCompleted {
		t.Fatalf("expected persisted streamed dispatch completed, got %s", dispatches[0].Status)
	}
}

func TestChatQueryRoute(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "chat-provider",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{
				Output:       "chat reply",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 2, OutputTokens: 2, TotalTokens: 4},
			}, nil
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
	})

	authManager := auth.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
			LLM: config.LLMConfig{
				DefaultTimeoutMs: 30000,
			},
		},
		Logger:   logger.Slog(),
		EventBus: events.NewBus(),
		Auth:     authManager,
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		LLM:      dispatcher,
		Store:    sqliteStore,
	})

	authHeader := issueAuthHeaderForTest(t, authManager, "chat-web")

	unauthorizedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorizedRec, httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"chat-provider","model":"test-model","query":"hello"}`)))
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized chat query, got %d", unauthorizedRec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"chat-provider","model":"test-model","query":"hello chat"}`))
	req.Header.Set("Authorization", authHeader)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for chat query, got %d body=%s", rec.Code, rec.Body.String())
	}

	response := decodeStrictResponse[ChatQueryResponse](t, rec.Body.Bytes())
	if response.Reply != "chat reply" {
		t.Fatalf("expected chat reply, got %q", response.Reply)
	}
	if response.Status != string(llm.DispatchStatusCompleted) {
		t.Fatalf("expected completed status, got %s", response.Status)
	}

	persisted, ok, err := sqliteStore.GetLLMDispatch(context.Background(), response.DispatchID)
	if err != nil {
		t.Fatalf("GetLLMDispatch returned error: %v", err)
	}
	if !ok || persisted.Output != "chat reply" {
		t.Fatalf("expected persisted chat dispatch, got %+v ok=%v", persisted, ok)
	}
}

func TestChatQueryRouteReturnsProviderFailure(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "chat-provider",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, &llm.ProviderError{Code: "upstream_auth_failed", Message: "bad key"}
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
	})

	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
			LLM: config.LLMConfig{
				DefaultTimeoutMs: 30000,
			},
		},
		Logger:   telemetry.New("error").Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		LLM:      dispatcher,
		Store:    sqliteStore,
	})

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"chat-provider","model":"test-model","query":"hello chat"}`)))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for provider failure, got %d body=%s", rec.Code, rec.Body.String())
	}
	response := decodeStrictResponse[ChatQueryResponse](t, rec.Body.Bytes())
	if response.ErrorCode != "upstream_auth_failed" {
		t.Fatalf("expected upstream_auth_failed, got %s", response.ErrorCode)
	}
}

func TestChatQueryStreamRoute(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "chat-stream",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			if err := emit(llm.StreamChunk{Delta: "hello"}); err != nil {
				return llm.ProviderResponse{}, err
			}
			if err := emit(llm.StreamChunk{Delta: " world"}); err != nil {
				return llm.ProviderResponse{}, err
			}
			return llm.ProviderResponse{
				Output:       "hello world",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
			}, nil
		},
	})

	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
			LLM: config.LLMConfig{
				DefaultTimeoutMs: 30000,
			},
		},
		Logger:   telemetry.New("error").Slog(),
		EventBus: events.NewBus(),
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		LLM:      dispatcher,
		Store:    sqliteStore,
	})

	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/v1/chat/query/stream", strings.NewReader(`{"provider":"chat-stream","model":"test-model","query":"hello stream"}`))
	if err != nil {
		t.Fatalf("failed to create stream request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute stream request: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
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
	if !strings.Contains(joined, "event: chat.query.started") {
		t.Fatalf("expected chat.query.started, got %q", joined)
	}
	if !strings.Contains(joined, "event: chat.query.delta") {
		t.Fatalf("expected chat.query.delta, got %q", joined)
	}
	if !strings.Contains(joined, "event: chat.query.completed") {
		t.Fatalf("expected chat.query.completed, got %q", joined)
	}
}

func TestRunCommandRoutesAndEventCursor(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	eventBus := events.NewBus()
	manager := runtime.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:      logger.Slog(),
		EventBus:    eventBus,
		Router:      router.NewSessionRouter(),
		Runtime:     manager,
		Store:       sqliteStore,
		Checkpoints: checkpointManager,
	})

	createRunRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRunRec, httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"chat","goal":"roadmap one"}`)))
	if createRunRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRunRec.Code)
	}
	run := decodeStrictResponse[runtime.Run](t, createRunRec.Body.Bytes())

	createStepRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createStepRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps", strings.NewReader(`{"title":"cancel me","kind":"task"}`)))
	if createStepRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createStepRec.Code)
	}
	step := decodeStrictResponse[runtime.Step](t, createStepRec.Body.Bytes())

	cancelStepRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(cancelStepRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/cancel", nil))
	if cancelStepRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", cancelStepRec.Code)
	}
	cancelledStep := decodeStrictResponse[runtime.Step](t, cancelStepRec.Body.Bytes())
	if cancelledStep.Status != runtime.StepStatusCancelled {
		t.Fatalf("expected cancelled step, got %s", cancelledStep.Status)
	}

	cancelRunRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(cancelRunRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/cancel", nil))
	if cancelRunRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", cancelRunRec.Code)
	}
	cancelledRun := decodeStrictResponse[runtime.Run](t, cancelRunRec.Body.Bytes())
	if cancelledRun.Status != runtime.RunStatusCancelled {
		t.Fatalf("expected cancelled run, got %s", cancelledRun.Status)
	}

	resumeRunRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resumeRunRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/resume", nil))
	if resumeRunRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resumeRunRec.Code)
	}
	resumedRun := decodeStrictResponse[runtime.Run](t, resumeRunRec.Body.Bytes())
	if resumedRun.Status != runtime.RunStatusQueued {
		t.Fatalf("expected queued run after resume, got %s", resumedRun.Status)
	}

	eventsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(eventsRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/events", nil))
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", eventsRec.Code)
	}
	eventList := decodeStrictResponse[EventListResponse](t, eventsRec.Body.Bytes())
	if len(eventList.Items) < 5 {
		t.Fatalf("expected at least 5 events, got %d", len(eventList.Items))
	}
	if eventList.NextCursor == 0 {
		t.Fatal("expected next cursor")
	}

	cursorRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(cursorRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/events?cursor="+strconv.FormatInt(eventList.Items[1].Sequence, 10), nil))
	if cursorRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", cursorRec.Code)
	}
	cursorList := decodeStrictResponse[EventListResponse](t, cursorRec.Body.Bytes())
	if len(cursorList.Items) == 0 {
		t.Fatal("expected events after cursor")
	}
	if cursorList.Items[0].Sequence <= eventList.Items[1].Sequence {
		t.Fatalf("expected cursor filtering, got first sequence %d after cursor %d", cursorList.Items[0].Sequence, eventList.Items[1].Sequence)
	}
}

func TestEventStreamSupportsLastEventIDResume(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	eventBus := events.NewBus()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:   logger.Slog(),
		EventBus: eventBus,
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		Store:    sqliteStore,
	})

	now := time.Now().UTC()
	first, err := sqliteStore.AppendEvent(context.Background(), events.Event{
		EventID:    "evt_stream_1",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: now,
		Resource:   events.Resource{Kind: "run", ID: "run_1"},
	})
	if err != nil {
		t.Fatalf("AppendEvent(first) returned error: %v", err)
	}
	eventBus.Publish(first)
	second, err := sqliteStore.AppendEvent(context.Background(), events.Event{
		EventID:    "evt_stream_2",
		Category:   "run",
		Name:       "run.status_changed",
		OccurredAt: first.OccurredAt.Add(time.Second),
		Resource:   events.Resource{Kind: "run", ID: "run_1"},
	})
	if err != nil {
		t.Fatalf("AppendEvent(second) returned error: %v", err)
	}
	eventBus.Publish(second)

	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testServer.URL+"/v1/events/stream?category=run", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Last-Event-ID", strconv.FormatInt(first.Sequence, 10))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	var lines []string
	for range 6 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("failed to read SSE response: %v", err)
		}
		lines = append(lines, line)
		if strings.Contains(strings.Join(lines, ""), second.Name) {
			cancel()
			break
		}
	}

	body := []byte(strings.Join(lines, ""))
	if !strings.Contains(string(body), "id: "+strconv.FormatInt(second.Sequence, 10)) {
		t.Fatalf("expected stream body to contain resumed sequence %d, got %q", second.Sequence, string(body))
	}
	if strings.Contains(string(body), first.Name) {
		t.Fatalf("expected resumed stream to exclude first event, got %q", string(body))
	}

	if _, err := io.ReadAll(resp.Body); err != nil && !strings.Contains(err.Error(), "context canceled") {
		if err != nil {
			t.Fatalf("unexpected read error after cancel: %v", err)
		}
	}
}
