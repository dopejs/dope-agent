package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
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

func assertManagedProviderMetadata(t *testing.T, metadata map[string]string, action string) {
	t.Helper()
	if metadata["managedProviderId"] == "" {
		t.Fatalf("expected managed provider id metadata, got %+v", metadata)
	}
	if metadata["managedProviderAction"] != action {
		t.Fatalf("expected managed provider action %s, got %+v", action, metadata)
	}
	if metadata["sandboxProfileId"] == "" {
		t.Fatalf("expected sandbox profile metadata, got %+v", metadata)
	}
	if metadata["sandboxDecision"] == "" {
		t.Fatalf("expected sandbox decision metadata, got %+v", metadata)
	}
	if metadata["enforcementStrength"] == "" {
		t.Fatalf("expected enforcement metadata, got %+v", metadata)
	}
}

func assertSandboxDeclaration(t *testing.T, view map[string]any, consumerKind, consumerID, operationKind string) {
	t.Helper()
	declaration, ok := view["declaration"].(map[string]any)
	if !ok {
		t.Fatalf("expected sandbox declaration, got %+v", view)
	}
	if declaration["consumerKind"] != consumerKind || declaration["consumerId"] != consumerID || declaration["operationKind"] != operationKind {
		t.Fatalf("expected declaration %s/%s/%s, got %+v", consumerKind, consumerID, operationKind, declaration)
	}
	if declaration["requiredEnforcementStrength"] == "" {
		t.Fatalf("expected required enforcement strength, got %+v", declaration)
	}
}

func assertSandboxPolicyRecord(t *testing.T, view map[string]any, status, secretResolution string) {
	t.Helper()
	record, ok := view["policyRecord"].(map[string]any)
	if !ok {
		t.Fatalf("expected sandbox policy record, got %+v", view)
	}
	if status != "" && record["status"] != status {
		t.Fatalf("expected policy status %s, got %+v", status, record)
	}
	if secretResolution != "" && record["secretResolution"] != secretResolution {
		t.Fatalf("expected secret resolution %s, got %+v", secretResolution, record)
	}
}

func assertSandboxSecretScope(t *testing.T, view map[string]any, consumerID, secretRef, environment, resolution string) {
	t.Helper()
	items, ok := view["secretScope"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected sandbox secret scope, got %+v", view)
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected secret scope item payload, got %+v", items[0])
	}
	if item["consumerId"] != consumerID || item["secretRef"] != secretRef || item["environmentScope"] != environment || item["resolution"] != resolution {
		t.Fatalf("unexpected secret scope item %+v", item)
	}
	if item["defaultRuleId"] == "" {
		t.Fatalf("expected default rule attribution, got %+v", item)
	}
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

type testManagedRegistry struct {
	bridges []providers.ManagedBridge
}

func (r testManagedRegistry) List() []providers.ManagedBridge {
	return append([]providers.ManagedBridge(nil), r.bridges...)
}

func (r testManagedRegistry) Get(providerID string) (providers.ManagedBridge, bool) {
	for _, bridge := range r.bridges {
		if bridge.ProviderID() == providerID {
			return bridge, true
		}
	}
	return nil, false
}

type testManagedBridge struct {
	providerID    string
	displayName   string
	family        providers.Family
	authMode      providers.AuthMode
	detectState   providers.AuthState
	startState    providers.AuthState
	completeState providers.AuthState
	refreshState  providers.AuthState
	revokeState   providers.AuthState
	models        []providers.Model
	provider      llm.Provider
}

func (b testManagedBridge) ProviderID() string           { return b.providerID }
func (b testManagedBridge) DisplayName() string          { return b.displayName }
func (b testManagedBridge) Family() providers.Family     { return b.family }
func (b testManagedBridge) AuthMode() providers.AuthMode { return b.authMode }
func (b testManagedBridge) Detect(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.detectState, cloneProviderModels(b.models), nil
}
func (b testManagedBridge) Start(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.startState, cloneProviderModels(b.models), nil
}
func (b testManagedBridge) Complete(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.completeState, cloneProviderModels(b.models), nil
}
func (b testManagedBridge) Refresh(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.refreshState, cloneProviderModels(b.models), nil
}
func (b testManagedBridge) Revoke(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.revokeState, cloneProviderModels(b.models), nil
}
func (b testManagedBridge) Provider() llm.Provider { return b.provider }

func cloneProviderModels(items []providers.Model) []providers.Model {
	cloned := make([]providers.Model, 0, len(items))
	for _, item := range items {
		model := item
		model.ReasoningLevels = append([]string(nil), item.ReasoningLevels...)
		cloned = append(cloned, model)
	}
	return cloned
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

func newProviderManagerAndChatServiceForTests(cfg config.Config, dispatcher *llm.Dispatcher, eventBus *events.Bus, sqliteStore *store.SQLiteStore, registry providers.ManagedRegistry) (*providers.Manager, *chat.Service) {
	manager := providers.NewManager(cfg, dispatcher, registry)
	return manager, chat.NewService(dispatcher, manager, nil, eventBus, sqliteStore)
}

func newSkillRegistryForTest(t *testing.T) *skills.Registry {
	t.Helper()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := filepath.Join(t.TempDir(), "dope-data")
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeSkillFileForTest(t, filepath.Join(homeRoot, "skills", "shared", "SKILL.md"), strings.TrimSpace(`
---
name: shared
description: home skill
---
home instructions
`))
	writeSkillFileForTest(t, filepath.Join(homeRoot, "skills", "shared", "notes.txt"), "home notes")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "skills", "shared", "SKILL.md"), strings.TrimSpace(`
---
name: shared
description: data skill
---
data instructions
`))
	writeSkillFileForTest(t, filepath.Join(dataRoot, "skills", "shared", "assets", "guide.md"), "data guide")

	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}
	return registry
}

func writeSkillFileForTest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func TestRunsLifecycleRoutes(t *testing.T) {
	eventBus := events.NewBus()
	manager := runtime.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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

func TestSandboxRoutes(t *testing.T) {
	eventBus := events.NewBus()
	policyEngine := policy.NewEngine()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	sandboxManager := sandbox.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     t.TempDir(),
		BindAddr:    "127.0.0.1:19191",
		LogLevel:    "info",
		Version:     "test",
	}, sqliteStore, eventBus, policyEngine)
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     t.TempDir(),
			LogLevel:    "info",
			Version:     "test",
		},
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Policy:    policyEngine,
		Sandboxes: sandboxManager,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/profiles", nil)
	listRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for profile list, got %d", listRec.Code)
	}

	explainReq := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/explain", strings.NewReader(`{
		"command":"echo",
		"args":["hello"],
		"cwd":"`+sqliteStore.DataDir+`",
		"access":{"readRoots":["`+sqliteStore.DataDir+`"],"writeRoots":["`+sqliteStore.DataDir+`"],"networkMode":"full"}
	}`))
	explainRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(explainRec, explainReq)
	if explainRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sandbox explain, got %d", explainRec.Code)
	}
	explain := decodeStrictResponse[SandboxExplainResponse](t, explainRec.Body.Bytes())
	if explain.Decision.Resolution != sandbox.DecisionResolutionAsk {
		t.Fatalf("expected ask explain resolution, got %s", explain.Decision.Resolution)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/executions", strings.NewReader(`{
		"command":"echo",
		"args":["hello"],
		"cwd":"`+sqliteStore.DataDir+`",
		"metadata":{"managedProviderId":"codex_managed","managedProviderAction":"prompt_execution","sandboxProfileId":"managed_provider_codex","sandboxDecision":"ask","enforcementStrength":"declared_only"},
		"access":{"readRoots":["`+sqliteStore.DataDir+`"],"writeRoots":["`+sqliteStore.DataDir+`"],"networkMode":"full"}
	}`))
	createRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for sandbox create, got %d", createRec.Code)
	}
	created := decodeStrictResponse[sandbox.Execution](t, createRec.Body.Bytes())
	if created.Status != sandbox.ExecutionStatusDenied {
		t.Fatalf("expected denied execution, got %s", created.Status)
	}
	if created.ApprovalID == "" {
		t.Fatal("expected approval id on denied execution")
	}
	if created.Metadata["managedProviderAction"] != "prompt_execution" {
		t.Fatalf("expected execution metadata, got %+v", created.Metadata)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/executions/"+created.ExecutionID, nil)
	getRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sandbox get, got %d", getRec.Code)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/executions/"+created.ExecutionID+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sandbox cancel, got %d", cancelRec.Code)
	}
}

func TestEventStreamReplaysMatchingHistory(t *testing.T) {
	eventBus := events.NewBus()
	manager := runtime.NewManager()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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

func TestConfigRouteProjectsManagedProviderSandboxByEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name        string
		environment config.Environment
		workDir     string
		expectedEnv string
	}{
		{name: "test", environment: config.EnvironmentTest, workDir: filepath.Join(t.TempDir(), "test-workdir"), expectedEnv: "test"},
		{name: "prod", environment: config.EnvironmentProd, workDir: filepath.Join(t.TempDir(), "prod-workdir"), expectedEnv: "prod"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(Dependencies{
				Config: config.Config{
					Environment: tc.environment,
					BindAddr:    "127.0.0.1:19191",
					DataDir:     "/tmp/dope",
					LogLevel:    "info",
					Version:     "test",
					LLM: config.LLMConfig{
						OpenAICompatible: config.OpenAICompatibleProviderConfig{
							APIKey: "secret",
						},
						Codex: config.ManagedCLIProviderConfig{
							CLIPath: "/usr/bin/codex",
							WorkDir: tc.workDir,
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
			if response.Environment != tc.expectedEnv {
				t.Fatalf("expected environment %s, got %+v", tc.expectedEnv, response)
			}
			if response.LLM.Codex.Sandbox == nil {
				t.Fatalf("expected codex sandbox projection, got %+v", response.LLM.Codex)
			}
			assertSandboxDeclaration(t, response.LLM.Codex.Sandbox, "managed_provider", "codex_managed", "config_inspect")
			if strings.Contains(rec.Body.String(), `"secret"`) {
				t.Fatalf("config response leaked secret: %s", rec.Body.String())
			}
		})
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
	if pending.Approval.Sandbox == nil || pending.Decision.Sandbox == nil {
		t.Fatalf("expected pending approval response to include sandbox provenance, got approval=%+v decision=%+v", pending.Approval.Sandbox, pending.Decision.Sandbox)
	}
	if policyRecord, ok := pending.Approval.Sandbox["policyRecord"].(map[string]any); !ok || policyRecord["policyRecordId"] == "" {
		t.Fatalf("expected pending sandbox policy record id, got %+v", pending.Approval.Sandbox)
	}

	resolveRejectedReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+pending.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"rejected","comment":"rejected for test"}`))
	resolveRejectedReq.Header.Set("Authorization", authHeader)
	resolveRejectedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveRejectedRec, resolveRejectedReq)
	if resolveRejectedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for rejected approval resolve, got %d body=%s", resolveRejectedRec.Code, resolveRejectedRec.Body.String())
	}
	var rejectedResolved struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(resolveRejectedRec.Body.Bytes(), &rejectedResolved); err != nil {
		t.Fatalf("failed to decode rejected approval resolve response: %v", err)
	}
	if rejectedResolved.Approval.Sandbox == nil || rejectedResolved.Decision.Sandbox == nil {
		t.Fatalf("expected rejected approval resolve response to include sandbox provenance, got approval=%+v decision=%+v", rejectedResolved.Approval.Sandbox, rejectedResolved.Decision.Sandbox)
	}

	rejectedReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","approvalId":"`+pending.Approval.ApprovalID+`"}`))
	rejectedReq.Header.Set("Authorization", authHeader)
	rejectedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rejectedRec, rejectedReq)
	if rejectedRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for rejected approval, got %d body=%s", rejectedRec.Code, rejectedRec.Body.String())
	}
	var rejectedBody struct {
		Approval policy.Approval `json:"approval"`
		Sandbox  map[string]any  `json:"sandbox"`
	}
	if err := json.Unmarshal(rejectedRec.Body.Bytes(), &rejectedBody); err != nil {
		t.Fatalf("failed to decode rejected approval body: %v", err)
	}
	if rejectedBody.Sandbox == nil {
		t.Fatalf("expected rejected tool call body to include sandbox provenance, got %s", rejectedRec.Body.String())
	}

	approvedPendingReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd"}}`))
	approvedPendingReq.Header.Set("Authorization", authHeader)
	approvedPendingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(approvedPendingRec, approvedPendingReq)
	if approvedPendingRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for second pending approval, got %d body=%s", approvedPendingRec.Code, approvedPendingRec.Body.String())
	}
	var approvedPending struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(approvedPendingRec.Body.Bytes(), &approvedPending); err != nil {
		t.Fatalf("failed to decode second pending approval response: %v", err)
	}
	if approvedPending.Approval.Sandbox == nil || approvedPending.Decision.Sandbox == nil {
		t.Fatalf("expected second pending approval response to include sandbox provenance, got approval=%+v decision=%+v", approvedPending.Approval.Sandbox, approvedPending.Decision.Sandbox)
	}
	resolveApprovedReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+approvedPending.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"approved","comment":"approved for test"}`))
	resolveApprovedReq.Header.Set("Authorization", authHeader)
	resolveApprovedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveApprovedRec, resolveApprovedReq)
	if resolveApprovedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approved approval resolve, got %d body=%s", resolveApprovedRec.Code, resolveApprovedRec.Body.String())
	}
	var approvedResolved struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(resolveApprovedRec.Body.Bytes(), &approvedResolved); err != nil {
		t.Fatalf("failed to decode approved approval resolve response: %v", err)
	}
	if approvedResolved.Approval.Sandbox == nil || approvedResolved.Decision.Sandbox == nil {
		t.Fatalf("expected approved approval resolve response to include sandbox provenance, got approval=%+v decision=%+v", approvedResolved.Approval.Sandbox, approvedResolved.Decision.Sandbox)
	}

	approvedReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","approvalId":"`+approvedPending.Approval.ApprovalID+`"}`))
	approvedReq.Header.Set("Authorization", authHeader)
	approvedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(approvedRec, approvedReq)
	if approvedRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approved tool call, got %d body=%s", approvedRec.Code, approvedRec.Body.String())
	}
	approvedToolCall := decodeStrictResponse[runtime.ToolCall](t, approvedRec.Body.Bytes())
	if approvedToolCall.Sandbox == nil {
		t.Fatalf("expected approved tool call resource to include sandbox provenance, got %+v", approvedToolCall)
	}

	approvalGetReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+pending.Approval.ApprovalID, nil)
	approvalGetReq.Header.Set("Authorization", authHeader)
	approvalGetRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(approvalGetRec, approvalGetReq)
	if approvalGetRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get, got %d body=%s", approvalGetRec.Code, approvalGetRec.Body.String())
	}
	gotApproval := decodeStrictResponse[policy.Approval](t, approvalGetRec.Body.Bytes())
	if gotApproval.Sandbox == nil {
		t.Fatalf("expected approval get to include sandbox provenance, got %+v", gotApproval)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals?status=rejected", nil)
	listReq.Header.Set("Authorization", authHeader)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval list, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listed struct {
		Items []policy.Approval `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("failed to decode approval list response: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Sandbox == nil {
		t.Fatalf("expected rejected approval list item with sandbox provenance, got %+v", listed.Items)
	}

	allowedReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"search","toolName":"lookup","input":{"q":"hi"}}`))
	allowedReq.Header.Set("Authorization", authHeader)
	allowedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for low-risk tool call, got %d body=%s", allowedRec.Code, allowedRec.Body.String())
	}
}

func TestToolCallApprovalUsesSharedDeclarationVocabulary(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	manager := runtime.NewManager()
	authManager := auth.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:       telemetry.New("error").Slog(),
		EventBus:     events.NewBus(),
		Auth:         authManager,
		Policy:       policy.NewEngine(),
		Router:       router.NewSessionRouter(),
		Runtime:      manager,
		Capabilities: capabilitySupervisor,
		Store:        sqliteStore,
		Checkpoints:  checkpoints.NewManager(sqliteStore, manager),
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "tool-call-vocabulary")

	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "shell",
		Kind:         "shell",
		DisplayName:  "Shell",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "guard shell"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd"}}`))
	req.Header.Set("Authorization", authHeader)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for approval-gated tool call, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
		Sandbox  map[string]any  `json:"sandbox"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	assertSandboxDeclaration(t, response.Sandbox, "local_tool", "shell", "tool_call.execute")
	assertSandboxPolicyRecord(t, response.Sandbox, "approval_pending", "not_applicable")
	assertSandboxDeclaration(t, response.Approval.Sandbox, "local_tool", "shell", "tool_call.execute")
	assertSandboxDeclaration(t, response.Decision.Sandbox, "local_tool", "shell", "tool_call.execute")
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
		name: "echo",
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
	testCfg := config.Config{
		BindAddr: "127.0.0.1:19191",
		DataDir:  "~/.dope",
		LogLevel: "info",
		Version:  "test",
		LLM: config.LLMConfig{
			DefaultTimeoutMs: 30000,
		},
	}
	eventBus := events.NewBus()
	providerManager, chatService := newProviderManagerAndChatServiceForTests(testCfg, dispatcher, eventBus, sqliteStore, nil)
	server := NewServer(Dependencies{
		Config:    testCfg,
		Logger:    logger.Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		Router:    router.NewSessionRouter(),
		Runtime:   runtime.NewManager(),
		LLM:       dispatcher,
		Chat:      chatService,
		Providers: providerManager,
		Store:     sqliteStore,
	})

	authHeader := issueAuthHeaderForTest(t, authManager, "chat-web")

	unauthorizedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorizedRec, httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"hello"}`)))
	if unauthorizedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthorized chat query, got %d", unauthorizedRec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"hello chat"}`))
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
		name: "echo",
		completeFn: func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, &llm.ProviderError{Code: "upstream_auth_failed", Message: "bad key"}
		},
		streamFn: func(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
	})

	testCfg := config.Config{
		BindAddr: "127.0.0.1:19191",
		DataDir:  "~/.dope",
		LogLevel: "info",
		Version:  "test",
		LLM: config.LLMConfig{
			DefaultTimeoutMs: 30000,
		},
	}
	eventBus := events.NewBus()
	providerManager, chatService := newProviderManagerAndChatServiceForTests(testCfg, dispatcher, eventBus, sqliteStore, nil)
	server := NewServer(Dependencies{
		Config:    testCfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Router:    router.NewSessionRouter(),
		Runtime:   runtime.NewManager(),
		LLM:       dispatcher,
		Chat:      chatService,
		Providers: providerManager,
		Store:     sqliteStore,
	})

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"hello chat"}`)))
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
		name: "echo",
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

	testCfg := config.Config{
		BindAddr: "127.0.0.1:19191",
		DataDir:  "~/.dope",
		LogLevel: "info",
		Version:  "test",
		LLM: config.LLMConfig{
			DefaultTimeoutMs: 30000,
		},
	}
	eventBus := events.NewBus()
	providerManager, chatService := newProviderManagerAndChatServiceForTests(testCfg, dispatcher, eventBus, sqliteStore, nil)
	server := NewServer(Dependencies{
		Config:    testCfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Router:    router.NewSessionRouter(),
		Runtime:   runtime.NewManager(),
		LLM:       dispatcher,
		Chat:      chatService,
		Providers: providerManager,
		Store:     sqliteStore,
	})

	testServer := httptest.NewServer(server.Handler())
	defer testServer.Close()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/v1/chat/query/stream", strings.NewReader(`{"provider":"echo","model":"echo-v1","query":"hello stream"}`))
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

func TestSkillRegistryRoutesAndChatQuerySkillSupport(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	registry := newSkillRegistryForTest(t)
	eventBus := events.NewBus()
	authManager := auth.NewManager()
	var captured llm.ProviderRequest
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testLLMProvider{
		name: "echo",
		completeFn: func(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			captured = request
			return llm.ProviderResponse{
				Output:       "ok",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 3, OutputTokens: 1, TotalTokens: 4},
			}, nil
		},
		streamFn: func(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			captured = request
			if emit != nil {
				if err := emit(llm.StreamChunk{Delta: "ok", Output: "ok"}); err != nil {
					return llm.ProviderResponse{}, err
				}
			}
			return llm.ProviderResponse{
				Output:       "ok",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 3, OutputTokens: 1, TotalTokens: 4},
			}, nil
		},
	})
	cfg := config.Config{
		BindAddr: "127.0.0.1:19191",
		DataDir:  filepath.Join(t.TempDir(), "dope-runtime"),
		LogLevel: "info",
		Version:  "test",
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
			DefaultModel:    "echo-v1",
		},
	}
	providerManager := providers.NewManager(cfg, dispatcher, nil)
	chatService := chat.NewService(dispatcher, providerManager, registry, eventBus, sqliteStore)
	server := NewServer(Dependencies{
		Config:    cfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		Router:    router.NewSessionRouter(),
		Runtime:   runtime.NewManager(),
		LLM:       dispatcher,
		Chat:      chatService,
		Providers: providerManager,
		Skills:    registry,
		Store:     sqliteStore,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "skills-web")

	listReq := httptest.NewRequest(http.MethodGet, "/v1/skills", nil)
	listReq.Header.Set("Authorization", authHeader)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skills list, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	listResponse := decodeStrictResponse[SkillRegistryResponse](t, listRec.Body.Bytes())
	if len(listResponse.Items) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(listResponse.Items))
	}
	if len(listResponse.Overlays) != 2 {
		t.Fatalf("expected 2 overlays, got %d", len(listResponse.Overlays))
	}
	if listResponse.Items[0].Description != "data skill" {
		t.Fatalf("expected data-dir skill to win precedence, got %q", listResponse.Items[0].Description)
	}
	if listResponse.Items[0].Sandbox == nil {
		t.Fatalf("expected skill summary sandbox contract, got %+v", listResponse.Items[0])
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/skills/shared", nil)
	getReq.Header.Set("Authorization", authHeader)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skill detail, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	detail := decodeStrictResponse[SkillDetailResponse](t, getRec.Body.Bytes())
	if !strings.Contains(detail.Body, "data instructions") {
		t.Fatalf("expected data-dir skill body, got %q", detail.Body)
	}
	if len(detail.Files) != 1 || detail.Files[0].Path != "assets/guide.md" {
		t.Fatalf("expected bundled data-dir file inventory, got %+v", detail.Files)
	}
	if detail.Sandbox == nil {
		t.Fatalf("expected skill detail sandbox contract, got %+v", detail)
	}

	reloadReq := httptest.NewRequest(http.MethodPost, "/v1/skills/reload", nil)
	reloadReq.Header.Set("Authorization", authHeader)
	reloadRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(reloadRec, reloadReq)
	if reloadRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skill reload, got %d body=%s", reloadRec.Code, reloadRec.Body.String())
	}

	chatReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","skills":["shared"],"query":"hello"}`))
	chatReq.Header.Set("Authorization", authHeader)
	chatReq.Header.Set("Content-Type", "application/json")
	chatRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(chatRec, chatReq)
	if chatRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skill chat query, got %d body=%s", chatRec.Code, chatRec.Body.String())
	}
	chatResponse := decodeStrictResponse[ChatQueryResponse](t, chatRec.Body.Bytes())
	if len(chatResponse.Skills) != 1 || chatResponse.Skills[0] != "shared" {
		t.Fatalf("expected selected skill in response, got %+v", chatResponse.Skills)
	}
	if len(chatResponse.SkillContracts) != 1 {
		t.Fatalf("expected selected skill contract in response, got %+v", chatResponse.SkillContracts)
	}
	if declaration, ok := chatResponse.SkillContracts[0]["declaration"].(map[string]any); !ok || declaration["consumerKind"] != "skill" {
		t.Fatalf("expected skill contract declaration in response, got %+v", chatResponse.SkillContracts)
	}
	if len(captured.Messages) != 4 {
		t.Fatalf("expected 4 prompt messages, got %d", len(captured.Messages))
	}
	if !strings.Contains(captured.Messages[0].Content, "home overlay") {
		t.Fatalf("expected home overlay first, got %q", captured.Messages[0].Content)
	}
	if !strings.Contains(captured.Messages[1].Content, "data overlay") {
		t.Fatalf("expected data overlay second, got %q", captured.Messages[1].Content)
	}
	if !strings.Contains(captured.Messages[2].Content, "data instructions") || strings.Contains(captured.Messages[2].Content, "home instructions") {
		t.Fatalf("expected data-dir skill instructions, got %q", captured.Messages[2].Content)
	}
	if captured.Messages[3].Role != llm.RoleUser || captured.Messages[3].Content != "hello" {
		t.Fatalf("expected user query as final message, got %+v", captured.Messages[3])
	}
	llmEvents := eventBus.List(events.Filter{Category: "llm"})
	if len(llmEvents) < 2 {
		t.Fatalf("expected llm events for chat query, got %d", len(llmEvents))
	}
	requestedPayload, ok := llmEvents[0].Payload["skillContracts"]
	if !ok {
		t.Fatalf("expected llm.dispatch.requested to include selected skill contracts, got %+v", llmEvents[0].Payload)
	}
	switch contracts := requestedPayload.(type) {
	case []any:
		if len(contracts) != 1 {
			t.Fatalf("expected exactly one selected skill contract, got %+v", llmEvents[0].Payload)
		}
	case []map[string]any:
		if len(contracts) != 1 {
			t.Fatalf("expected exactly one selected skill contract, got %+v", llmEvents[0].Payload)
		}
	default:
		t.Fatalf("expected selected skill contracts array, got %T payload=%+v", requestedPayload, llmEvents[0].Payload)
	}

	missingReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"echo-v1","skills":["missing"],"query":"hello"}`))
	missingReq.Header.Set("Authorization", authHeader)
	missingReq.Header.Set("Content-Type", "application/json")
	missingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown skill, got %d body=%s", missingRec.Code, missingRec.Body.String())
	}

	streamServer := httptest.NewServer(server.Handler())
	defer streamServer.Close()

	streamReq, err := http.NewRequest(http.MethodPost, streamServer.URL+"/v1/chat/query/stream", strings.NewReader(`{"provider":"echo","model":"echo-v1","skills":["shared"],"query":"stream hello"}`))
	if err != nil {
		t.Fatalf("http.NewRequest returned error: %v", err)
	}
	streamReq.Header.Set("Authorization", authHeader)
	streamReq.Header.Set("Content-Type", "application/json")
	streamResp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("http.DefaultClient.Do returned error: %v", err)
	}
	defer streamResp.Body.Close()

	body, err := io.ReadAll(streamResp.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	joined := string(body)
	if !strings.Contains(joined, `"skills":["shared"]`) {
		t.Fatalf("expected stream started payload to include selected skills, got %q", joined)
	}
}

func TestProviderRoutesAndChecks(t *testing.T) {
	eventBus := events.NewBus()
	logger := telemetry.New("error")
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	dispatcher := llm.NewDispatcher()
	providerManager := providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
		},
	}, dispatcher)
	authManager := auth.NewManager()

	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:    logger.Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		Providers: providerManager,
		Store:     sqliteStore,
	})

	authHeader := issueAuthHeaderForTest(t, authManager, "provider-web")

	listReq := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	listReq.Header.Set("Authorization", authHeader)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}
	listResponse := decodeStrictResponse[ProviderListResponse](t, listRec.Body.Bytes())
	if len(listResponse.Items) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(listResponse.Items))
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/providers/echo", nil)
	getReq.Header.Set("Authorization", authHeader)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}
	profile := decodeStrictResponse[providers.Profile](t, getRec.Body.Bytes())
	if profile.ProviderID != "echo" {
		t.Fatalf("expected echo profile, got %s", profile.ProviderID)
	}

	checkReq := httptest.NewRequest(http.MethodPost, "/v1/providers/echo/checks", strings.NewReader(`{"model":"echo-v1","prompt":"hello"}`))
	checkReq.Header.Set("Authorization", authHeader)
	checkReq.Header.Set("Content-Type", "application/json")
	checkRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(checkRec, checkReq)
	if checkRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", checkRec.Code)
	}
	check := decodeStrictResponse[providers.Check](t, checkRec.Body.Bytes())
	if check.Status != providers.CheckStatusPassed {
		t.Fatalf("expected passed check, got %s", check.Status)
	}

	getCheckReq := httptest.NewRequest(http.MethodGet, "/v1/providers/echo/checks/"+check.CheckID, nil)
	getCheckReq.Header.Set("Authorization", authHeader)
	getCheckRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getCheckRec, getCheckReq)
	if getCheckRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getCheckRec.Code)
	}

	checks, err := sqliteStore.ListProviderChecks(context.Background(), "echo")
	if err != nil {
		t.Fatalf("ListProviderChecks returned error: %v", err)
	}
	if len(checks) != 1 {
		t.Fatalf("expected 1 persisted provider check, got %d", len(checks))
	}

	failedCheckReq := httptest.NewRequest(http.MethodPost, "/v1/providers/openai_compatible/checks", strings.NewReader(`{}`))
	failedCheckReq.Header.Set("Authorization", authHeader)
	failedCheckReq.Header.Set("Content-Type", "application/json")
	failedCheckRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(failedCheckRec, failedCheckReq)
	if failedCheckRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for failed provider check, got %d", failedCheckRec.Code)
	}
	failedCheck := decodeStrictResponse[providers.Check](t, failedCheckRec.Body.Bytes())
	if failedCheck.Status != providers.CheckStatusFailed {
		t.Fatalf("expected failed check status, got %s", failedCheck.Status)
	}
	if failedCheck.ErrorClass != providers.CheckErrorClassConfig {
		t.Fatalf("expected config_error classification, got %s", failedCheck.ErrorClass)
	}

	foundFailed := false
	foundCompleted := false
	for _, item := range eventBus.List(events.Filter{Category: "provider"}) {
		switch item.Name {
		case "provider.check_completed":
			foundCompleted = true
		case "provider.check_failed":
			foundFailed = true
		}
	}
	if !foundCompleted {
		t.Fatal("expected provider.check_completed event")
	}
	if !foundFailed {
		t.Fatal("expected provider.check_failed event")
	}
}

func TestProviderResolutionAppliesProfilePolicyToChat(t *testing.T) {
	eventBus := events.NewBus()
	logger := telemetry.New("error")
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
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
		name: "openai_compatible",
		completeFn: func(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{
				Output:       request.Model,
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			}, nil
		},
		streamFn: func(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
	})
	authManager := auth.NewManager()
	providerManager := providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "openai_compatible",
			DefaultModel:    "gpt-5.4",
			OpenAICompatible: config.OpenAICompatibleProviderConfig{
				BaseURL: "https://example.com",
				APIKey:  "secret",
				Model:   "gpt-4.1-mini",
			},
		},
	}, dispatcher)
	chatService := chat.NewService(dispatcher, providerManager, nil, eventBus, sqliteStore)

	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:    logger.Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		LLM:       dispatcher,
		Chat:      chatService,
		Providers: providerManager,
		Store:     sqliteStore,
	})

	authHeader := issueAuthHeaderForTest(t, authManager, "chat-web")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"query":"hello"}`))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	response := decodeStrictResponse[ChatQueryResponse](t, rec.Body.Bytes())
	if response.Provider != "openai_compatible" {
		t.Fatalf("expected default provider openai_compatible, got %s", response.Provider)
	}
	if response.Model != "gpt-5.4" {
		t.Fatalf("expected configured default model gpt-5.4, got %s", response.Model)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","query":"hello"}`))
	invalidReq.Header.Set("Authorization", authHeader)
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for echo fallback model, got %d body=%s", invalidRec.Code, invalidRec.Body.String())
	}
	invalidResponse := decodeStrictResponse[ChatQueryResponse](t, invalidRec.Body.Bytes())
	if invalidResponse.Model != "echo-v1" {
		t.Fatalf("expected echo-v1 model, got %s", invalidResponse.Model)
	}

	rejectReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"echo","model":"gpt-5.4","query":"hello"}`))
	rejectReq.Header.Set("Authorization", authHeader)
	rejectReq.Header.Set("Content-Type", "application/json")
	rejectRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rejectRec, rejectReq)
	if rejectRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for incompatible provider/model, got %d body=%s", rejectRec.Code, rejectRec.Body.String())
	}
}

func TestManagedProviderAuthModelAndDefaultModelRoutes(t *testing.T) {
	eventBus := events.NewBus()
	logger := telemetry.New("error")
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
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
		name: "codex_managed",
		completeFn: func(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{
				Output:       request.Model,
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			}, nil
		},
		streamFn: func(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{}, errors.New("not used")
		},
	})
	authManager := auth.NewManager()
	now := time.Now().UTC()
	bridge := testManagedBridge{
		providerID:  "codex_managed",
		displayName: "Codex CLI",
		family:      providers.FamilyCodexCLI,
		authMode:    providers.AuthModeLocalCLIBridge,
		detectState: providers.AuthState{
			ProviderID:    "codex_managed",
			Family:        providers.FamilyCodexCLI,
			AuthMode:      providers.AuthModeLocalCLIBridge,
			Status:        providers.AuthStatusLoginRequired,
			CLIPath:       "/usr/bin/codex",
			CLIAvailable:  true,
			LoginCommand:  []string{"codex", "login"},
			LogoutCommand: []string{"codex", "logout"},
			LastCheckedAt: now,
			Metadata: map[string]string{
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "auth_status",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
		},
		startState: providers.AuthState{
			ProviderID:    "codex_managed",
			Family:        providers.FamilyCodexCLI,
			AuthMode:      providers.AuthModeLocalCLIBridge,
			Status:        providers.AuthStatusPendingLogin,
			CLIPath:       "/usr/bin/codex",
			CLIAvailable:  true,
			LoginCommand:  []string{"codex", "login"},
			LogoutCommand: []string{"codex", "logout"},
			LastCheckedAt: now,
			Metadata: map[string]string{
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "auth_status",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
		},
		completeState: providers.AuthState{
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
			LastCheckedAt: now,
			Metadata: map[string]string{
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "auth_status",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
		},
		refreshState: providers.AuthState{
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
			LastCheckedAt: now.Add(time.Minute),
			Metadata: map[string]string{
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "auth_status",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
		},
		revokeState: providers.AuthState{
			ProviderID:    "codex_managed",
			Family:        providers.FamilyCodexCLI,
			AuthMode:      providers.AuthModeLocalCLIBridge,
			Status:        providers.AuthStatusRevoked,
			CLIPath:       "/usr/bin/codex",
			CLIAvailable:  true,
			LoginCommand:  []string{"codex", "login"},
			LogoutCommand: []string{"codex", "logout"},
			LastCheckedAt: now.Add(2 * time.Minute),
			Metadata: map[string]string{
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "logout",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
		},
		models: []providers.Model{
			{ProviderID: "codex_managed", ModelID: "gpt-5.4", DisplayName: "GPT-5.4", Default: true, Available: true, Source: "cache", Chat: true, Stream: true, Coding: true, ReasoningLevels: []string{"medium", "high"}},
			{ProviderID: "codex_managed", ModelID: "gpt-5.4-mini", DisplayName: "GPT-5.4 mini", Available: true, Source: "cache", Chat: true, Stream: true, Coding: true},
		},
		provider: &testLLMProvider{
			name: "codex_managed",
			completeFn: func(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
				return llm.ProviderResponse{Output: request.Model, FinishReason: "stop", Usage: llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}, nil
			},
			streamFn: func(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
				return llm.ProviderResponse{}, errors.New("not used")
			},
		},
	}
	registry := testManagedRegistry{bridges: []providers.ManagedBridge{bridge}}
	providerManager := providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "codex_managed",
		},
	}, dispatcher, registry)
	providerManager.RestoreManagedAuthStates([]providers.AuthState{bridge.detectState})
	providerManager.RestoreProviderModels(bridge.models)
	chatService := chat.NewService(dispatcher, providerManager, nil, eventBus, sqliteStore)

	server := NewServer(Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:    logger.Slog(),
		EventBus:  eventBus,
		Auth:      authManager,
		LLM:       dispatcher,
		Chat:      chatService,
		Providers: providerManager,
		Store:     sqliteStore,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "managed-provider-web")

	getAuthReq := httptest.NewRequest(http.MethodGet, "/v1/providers/codex_managed/auth", nil)
	getAuthReq.Header.Set("Authorization", authHeader)
	getAuthRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getAuthRec, getAuthReq)
	if getAuthRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth get, got %d", getAuthRec.Code)
	}
	authResponse := decodeStrictResponse[ProviderAuthStateResponse](t, getAuthRec.Body.Bytes())
	if authResponse.Auth.Status != providers.AuthStatusLoginRequired {
		t.Fatalf("expected login_required auth state, got %s", authResponse.Auth.Status)
	}
	assertManagedProviderMetadata(t, authResponse.Auth.Metadata, "auth_status")

	startReq := httptest.NewRequest(http.MethodPost, "/v1/providers/codex_managed/auth/start", strings.NewReader(`{}`))
	startReq.Header.Set("Authorization", authHeader)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth start, got %d body=%s", startRec.Code, startRec.Body.String())
	}
	startResponse := decodeStrictResponse[ProviderAuthStateResponse](t, startRec.Body.Bytes())
	if startResponse.Auth.Status != providers.AuthStatusPendingLogin {
		t.Fatalf("expected pending auth state, got %s", startResponse.Auth.Status)
	}
	assertManagedProviderMetadata(t, startResponse.Auth.Metadata, "auth_status")

	completeReq := httptest.NewRequest(http.MethodPost, "/v1/providers/codex_managed/auth/complete", strings.NewReader(`{}`))
	completeReq.Header.Set("Authorization", authHeader)
	completeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth complete, got %d body=%s", completeRec.Code, completeRec.Body.String())
	}
	completeResponse := decodeStrictResponse[ProviderAuthStateResponse](t, completeRec.Body.Bytes())
	if completeResponse.Auth.Status != providers.AuthStatusAuthenticated {
		t.Fatalf("expected authenticated state, got %s", completeResponse.Auth.Status)
	}
	assertManagedProviderMetadata(t, completeResponse.Auth.Metadata, "auth_status")

	modelsReq := httptest.NewRequest(http.MethodGet, "/v1/providers/codex_managed/models", nil)
	modelsReq.Header.Set("Authorization", authHeader)
	modelsRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(modelsRec, modelsReq)
	if modelsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for models, got %d", modelsRec.Code)
	}
	modelList := decodeStrictResponse[ProviderModelListResponse](t, modelsRec.Body.Bytes())
	if len(modelList.Items) != 2 {
		t.Fatalf("expected 2 models, got %d", len(modelList.Items))
	}

	defaultModelReq := httptest.NewRequest(http.MethodPost, "/v1/providers/codex_managed/default-model", strings.NewReader(`{"model":"gpt-5.4-mini"}`))
	defaultModelReq.Header.Set("Authorization", authHeader)
	defaultModelReq.Header.Set("Content-Type", "application/json")
	defaultModelRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(defaultModelRec, defaultModelReq)
	if defaultModelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for default model update, got %d body=%s", defaultModelRec.Code, defaultModelRec.Body.String())
	}
	defaultModelResponse := decodeStrictResponse[ProviderDefaultModelResponse](t, defaultModelRec.Body.Bytes())
	if defaultModelResponse.DefaultModel != "gpt-5.4-mini" {
		t.Fatalf("expected updated model gpt-5.4-mini, got %s", defaultModelResponse.DefaultModel)
	}

	chatReq := httptest.NewRequest(http.MethodPost, "/v1/chat/query", strings.NewReader(`{"provider":"codex_managed","query":"hello"}`))
	chatReq.Header.Set("Authorization", authHeader)
	chatReq.Header.Set("Content-Type", "application/json")
	chatRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(chatRec, chatReq)
	if chatRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for managed provider chat, got %d body=%s", chatRec.Code, chatRec.Body.String())
	}
	chatResponse := decodeStrictResponse[ChatQueryResponse](t, chatRec.Body.Bytes())
	if chatResponse.Model != "gpt-5.4-mini" {
		t.Fatalf("expected managed provider default-model override to apply, got %s", chatResponse.Model)
	}

	refreshReq := httptest.NewRequest(http.MethodPost, "/v1/providers/codex_managed/auth/refresh", strings.NewReader(`{}`))
	refreshReq.Header.Set("Authorization", authHeader)
	refreshRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth refresh, got %d", refreshRec.Code)
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/v1/providers/codex_managed/auth/revoke", strings.NewReader(`{}`))
	revokeReq.Header.Set("Authorization", authHeader)
	revokeRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth revoke, got %d", revokeRec.Code)
	}
	revokeResponse := decodeStrictResponse[ProviderAuthStateResponse](t, revokeRec.Body.Bytes())
	if revokeResponse.Auth.Status != providers.AuthStatusRevoked {
		t.Fatalf("expected revoked state, got %s", revokeResponse.Auth.Status)
	}
	assertManagedProviderMetadata(t, revokeResponse.Auth.Metadata, "logout")

	authStates, err := sqliteStore.ListProviderAuthStates(context.Background())
	if err != nil {
		t.Fatalf("ListProviderAuthStates returned error: %v", err)
	}
	if len(authStates) != 1 {
		t.Fatalf("expected 1 persisted auth state, got %d", len(authStates))
	}
	preferences, err := sqliteStore.ListProviderPreferences(context.Background())
	if err != nil {
		t.Fatalf("ListProviderPreferences returned error: %v", err)
	}
	if len(preferences) != 1 || preferences[0].DefaultModel != "gpt-5.4-mini" {
		t.Fatalf("unexpected preferences: %+v", preferences)
	}

	foundStarted := false
	foundDefaultModelUpdated := false
	foundAuthMetadata := false
	for _, item := range eventBus.List(events.Filter{Category: "provider"}) {
		if item.Name == "provider.auth_started" {
			foundStarted = true
			switch metadata := item.Payload["metadata"].(type) {
			case map[string]any:
				if metadata["managedProviderAction"] == "auth_status" {
					foundAuthMetadata = true
				}
			case map[string]string:
				if metadata["managedProviderAction"] == "auth_status" {
					foundAuthMetadata = true
				}
			}
		}
		if item.Name == "provider.default_model_updated" {
			foundDefaultModelUpdated = true
		}
	}
	if !foundStarted {
		t.Fatal("expected provider.auth_started event")
	}
	if !foundDefaultModelUpdated {
		t.Fatal("expected provider.default_model_updated event")
	}
	if !foundAuthMetadata {
		t.Fatal("expected provider auth metadata in emitted event")
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
			BindAddr: "127.0.0.1:19191",
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
			BindAddr: "127.0.0.1:19191",
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
