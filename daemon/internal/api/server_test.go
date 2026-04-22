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
	"os/exec"
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
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
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

func waitForToolCallTerminalState(t *testing.T, manager *runtime.Manager, runID, stepID, toolCallID string) runtime.ToolCall {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, ok := manager.GetToolCall(runID, stepID, toolCallID)
		if !ok {
			t.Fatalf("expected tool call %s", toolCallID)
		}
		switch got.Status {
		case runtime.ToolCallStatusCompleted, runtime.ToolCallStatusFailed, runtime.ToolCallStatusDenied, runtime.ToolCallStatusCancelled:
			return got
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("tool call %s did not reach terminal state", toolCallID)
	return runtime.ToolCall{}
}

func TestRunRoutesProjectLatestDeliverySummaryWithoutForegroundRegression(t *testing.T) {
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
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	target, err := deliveryManager.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "target-run",
		DisplayName:      "Run Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget returned error: %v", err)
	}
	if _, err := deliveryManager.UpsertPreference(context.Background(), delivery.DeliveryPreference{
		PreferenceID:     "pref-run",
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

	backgroundRun, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "background delivery"})
	if err != nil {
		t.Fatalf("CreateRun(background) returned error: %v", err)
	}
	backgroundRun.Status = runtime.RunStatusCompleted
	backgroundRun.UpdatedAt = time.Now().UTC()
	if err := sqliteStore.UpsertRun(context.Background(), backgroundRun); err != nil {
		t.Fatalf("UpsertRun(background) returned error: %v", err)
	}
	if err := maybeEmitRunDelivery(context.Background(), deliveryManager, runtimeManager, backgroundRun); err != nil {
		t.Fatalf("maybeEmitRunDelivery(background) returned error: %v", err)
	}

	foregroundRun, err := runtimeManager.CreateRun(runtime.CreateRunInput{SessionID: "session_foreground", Entrypoint: "operator", Goal: "foreground reply"})
	if err != nil {
		t.Fatalf("CreateRun(foreground) returned error: %v", err)
	}
	if err := sqliteStore.UpsertSession(context.Background(), router.Session{
		SessionID:    "session_foreground",
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "discord",
		PeerID:       "user_foreground",
		RoutingKey:   "direct:discord::user_foreground:",
		Generation:   1,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		LastActiveAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	foregroundRun.Status = runtime.RunStatusCompleted
	foregroundRun.UpdatedAt = time.Now().UTC()
	if err := sqliteStore.UpsertRun(context.Background(), foregroundRun); err != nil {
		t.Fatalf("UpsertRun(foreground) returned error: %v", err)
	}
	if err := maybeEmitRunDelivery(context.Background(), deliveryManager, runtimeManager, foregroundRun); err != nil {
		t.Fatalf("maybeEmitRunDelivery(foreground) returned error: %v", err)
	}

	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		Auth:        authManager,
		EventBus:    eventBus,
		Runtime:     runtimeManager,
		Delivery:    deliveryManager,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "delivery-run-web")

	bgRec := httptest.NewRecorder()
	bgReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+backgroundRun.RunID, nil)
	bgReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(bgRec, bgReq)
	if bgRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for background run, got %d body=%s", bgRec.Code, bgRec.Body.String())
	}
	bg := decodeStrictResponse[runtime.Run](t, bgRec.Body.Bytes())
	if bg.LatestDeliveryID == "" || bg.LatestDeliveryStatus != string(delivery.OutcomeStatusDelivered) || bg.LatestDeliveryTargetID != target.TargetID {
		t.Fatalf("expected projected latest delivery on background run, got %+v", bg)
	}

	fgRec := httptest.NewRecorder()
	fgReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+foregroundRun.RunID, nil)
	fgReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(fgRec, fgReq)
	if fgRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for foreground run, got %d body=%s", fgRec.Code, fgRec.Body.String())
	}
	fg := decodeStrictResponse[runtime.Run](t, fgRec.Body.Bytes())
	if fg.LatestDeliveryID != "" || fg.LatestDeliveryStatus != "" || fg.LatestDeliveryTargetID != "" {
		t.Fatalf("expected foreground run to remain free of background delivery projection, got %+v", fg)
	}
}

func TestDeliveryRoutesExposeTargetsPreferencesSuppressionAndEvents(t *testing.T) {
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
	authManager := auth.NewManager()
	runtimeManager := runtime.NewManager()
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		Auth:        authManager,
		EventBus:    eventBus,
		Runtime:     runtimeManager,
		Delivery:    deliveryManager,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "delivery-admin")

	createTargetRec := httptest.NewRecorder()
	createTargetReq := httptest.NewRequest(http.MethodPost, "/v1/delivery/targets", strings.NewReader(`{"targetId":"ops-target","displayName":"Ops Target","targetKind":"test_sink","addressSummary":"ops sink"}`))
	createTargetReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(createTargetRec, createTargetReq)
	if createTargetRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createTargetRec.Code, createTargetRec.Body.String())
	}
	target := decodeStrictResponse[delivery.DeliveryTarget](t, createTargetRec.Body.Bytes())
	if target.TargetID != "ops-target" || target.Status != delivery.TargetStatusActive {
		t.Fatalf("unexpected target response %+v", target)
	}

	disableRec := httptest.NewRecorder()
	disableReq := httptest.NewRequest(http.MethodPost, "/v1/delivery/targets/ops-target/disable", nil)
	disableReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from disable, got %d body=%s", disableRec.Code, disableRec.Body.String())
	}
	activateRec := httptest.NewRecorder()
	activateReq := httptest.NewRequest(http.MethodPost, "/v1/delivery/targets/ops-target/activate", nil)
	activateReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(activateRec, activateReq)
	if activateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from activate, got %d body=%s", activateRec.Code, activateRec.Body.String())
	}

	prefRec := httptest.NewRecorder()
	prefReq := httptest.NewRequest(http.MethodPost, "/v1/delivery/preferences", strings.NewReader(`{"preferenceId":"ops-pref","scopeKind":"user_default","preferredTargetsByClass":{"routine_success":"ops-target","urgent":"ops-target","failure":"ops-target"},"suppressionPolicy":{"suppressFailure":true}}`))
	prefReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(prefRec, prefReq)
	if prefRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", prefRec.Code, prefRec.Body.String())
	}
	pref := decodeStrictResponse[delivery.DeliveryPreference](t, prefRec.Body.Bytes())
	if !pref.SuppressionPolicy.SuppressFailure {
		t.Fatalf("expected suppressFailure policy, got %+v", pref)
	}

	outcome, err := deliveryManager.EmitOutcome(context.Background(), delivery.OutcomeInput{
		SourceKind:     "run",
		SourceID:       "suppressed_run",
		RunID:          "suppressed_run",
		ResultClass:    delivery.ResultClassFailure,
		PayloadPreview: "suppressed failure",
	})
	if err != nil {
		t.Fatalf("EmitOutcome returned error: %v", err)
	}
	if outcome.Status != delivery.OutcomeStatusSuppressed {
		t.Fatalf("expected suppressed outcome, got %+v", outcome)
	}

	deliveriesRec := httptest.NewRecorder()
	deliveriesReq := httptest.NewRequest(http.MethodGet, "/v1/deliveries?sourceId=suppressed_run", nil)
	deliveriesReq.Header.Set("Authorization", authHeader)
	server.Handler().ServeHTTP(deliveriesRec, deliveriesReq)
	if deliveriesRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", deliveriesRec.Code, deliveriesRec.Body.String())
	}
	deliveries := decodeStrictResponse[DeliveryOutcomeListResponse](t, deliveriesRec.Body.Bytes())
	if len(deliveries.Items) != 1 {
		t.Fatalf("expected one delivery outcome, got %+v", deliveries.Items)
	}
	if deliveries.Items[0].Status != delivery.OutcomeStatusSuppressed || deliveries.Items[0].SuppressionReason == "" {
		t.Fatalf("expected visible suppression truth, got %+v", deliveries.Items[0])
	}

	persistedEvents, err := sqliteStore.ListEvents(context.Background(), events.Filter{Category: "delivery"})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	names := make([]string, 0, len(persistedEvents))
	for _, event := range persistedEvents {
		names = append(names, event.Name)
	}
	assertContainsEvent(t, names, "delivery.target_registered")
	assertContainsEvent(t, names, "delivery.target_status_changed")
	assertContainsEvent(t, names, "delivery.preference_updated")
}

func TestRunDeliveryUsesIntegrationOverrideTarget(t *testing.T) {
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
	deliveryManager := delivery.NewManager("test", eventBus, sqliteStore, delivery.NewTestSinkAdapter())
	defaultTarget, err := deliveryManager.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "run-default-target",
		DisplayName:      "Run Default Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget(default) returned error: %v", err)
	}
	overrideTarget, err := deliveryManager.CreateTarget(context.Background(), delivery.DeliveryTarget{
		TargetID:         "run-override-target",
		DisplayName:      "Run Override Target",
		TargetKind:       delivery.TargetKindTestSink,
		EnvironmentScope: "test",
	})
	if err != nil {
		t.Fatalf("CreateTarget(override) returned error: %v", err)
	}
	if _, err := deliveryManager.UpsertPreference(context.Background(), delivery.DeliveryPreference{
		PreferenceID:     "run-pref-default",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeUserDefault,
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassRoutineSuccess: defaultTarget.TargetID,
			delivery.ResultClassUrgent:         defaultTarget.TargetID,
			delivery.ResultClassFailure:        defaultTarget.TargetID,
		},
	}); err != nil {
		t.Fatalf("UpsertPreference(default) returned error: %v", err)
	}
	if _, err := deliveryManager.UpsertPreference(context.Background(), delivery.DeliveryPreference{
		PreferenceID:     "run-pref-calendar-b",
		EnvironmentScope: "test",
		ScopeKind:        delivery.PreferenceScopeIntegrationOverride,
		IntegrationID:    "calendar-b",
		PreferredTargetsByClass: map[delivery.ResultClass]string{
			delivery.ResultClassRoutineSuccess: overrideTarget.TargetID,
			delivery.ResultClassUrgent:         overrideTarget.TargetID,
			delivery.ResultClassFailure:        overrideTarget.TargetID,
		},
	}); err != nil {
		t.Fatalf("UpsertPreference(override) returned error: %v", err)
	}

	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: "run integration override"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := runtimeManager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "probe integration"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	if _, err := runtimeManager.CreateToolCall(run.RunID, step.StepID, runtime.CreateToolCallInput{
		ToolName:     "integration.probe",
		CapabilityID: "calendar-probe",
		IntegrationBindings: []integrations.BindingSummary{{
			IntegrationID: "calendar-b",
			DisplayName:   "Calendar B",
		}},
	}); err != nil {
		t.Fatalf("CreateToolCall returned error: %v", err)
	}
	if _, _, err := runtimeManager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusPlanning,
		Output: map[string]any{"phase": "planning"},
	}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(planning) returned error: %v", err)
	}
	if _, _, err := runtimeManager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusCallingModel,
		Output: map[string]any{"phase": "calling_model"},
	}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(calling_model) returned error: %v", err)
	}
	if _, _, err := runtimeManager.UpdateStepStatusAndReconcileRun(run.RunID, step.StepID, runtime.UpdateStepStatusInput{
		Status: runtime.StepStatusCompleted,
		Output: map[string]any{"ok": true},
	}); err != nil {
		t.Fatalf("UpdateStepStatusAndReconcileRun(completed) returned error: %v", err)
	}
	run, ok := runtimeManager.GetRun(run.RunID)
	if !ok {
		t.Fatal("expected completed run to remain addressable")
	}
	if run.Status != runtime.RunStatusCompleted {
		t.Fatalf("expected completed run, got %+v", run)
	}

	if err := maybeEmitRunDelivery(context.Background(), deliveryManager, runtimeManager, run); err != nil {
		t.Fatalf("maybeEmitRunDelivery returned error: %v", err)
	}

	outcomes, err := deliveryManager.ListOutcomes(context.Background(), delivery.OutcomeFilter{
		SourceKind: "run",
		SourceID:   run.RunID,
	})
	if err != nil {
		t.Fatalf("ListOutcomes returned error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected one run delivery outcome, got %+v", outcomes)
	}
	if outcomes[0].IntegrationID != "calendar-b" || outcomes[0].ChosenTargetID != overrideTarget.TargetID {
		t.Fatalf("expected run integration override routing, got %+v", outcomes[0])
	}
}

func assertContainsEvent(t *testing.T, names []string, expected string) {
	t.Helper()
	for _, name := range names {
		if name == expected {
			return
		}
	}
	t.Fatalf("expected event %s in %+v", expected, names)
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

func writeExecutableSkillForTest(t *testing.T, skillRoot, skillBody, scriptBody string) {
	t.Helper()
	writeSkillFileForTest(t, filepath.Join(skillRoot, "SKILL.md"), strings.TrimSpace(skillBody))
	scriptPath := filepath.Join(skillRoot, "scripts", "run.sh")
	writeSkillFileForTest(t, scriptPath, strings.TrimSpace(scriptBody))
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		t.Fatalf("Chmod returned error: %v", err)
	}
}

func writeExecutableSkillSecretsForTest(t *testing.T, dataRoot string, values map[string]string) {
	t.Helper()
	payload, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	writeSkillFileForTest(t, filepath.Join(dataRoot, "skill-secrets.json"), string(payload))
}

func requireRealDockerForTest(t *testing.T) {
	t.Helper()

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker CLI is not available on PATH")
	}
	if output, err := exec.Command(dockerPath, "version", "--format", "{{.Server.Version}}").CombinedOutput(); err != nil || strings.TrimSpace(string(output)) == "" {
		t.Skipf("docker runtime is unavailable: %s", strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command(dockerPath, "image", "inspect", "alpine:3.20", "--format", "{{.Id}}").CombinedOutput(); err != nil || strings.TrimSpace(string(output)) == "" {
		t.Skipf("docker image alpine:3.20 is not available locally: %s", strings.TrimSpace(string(output)))
	}
}

func TestRunsLifecycleRoutes(t *testing.T) {
	eventBus := events.NewBus()
	manager := runtime.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     "~/.dope",
			LogLevel:    "info",
			Version:     "test",
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
		CapabilityID: "search",
		Kind:         "knowledge",
		DisplayName:  "Search",
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

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"search","toolName":"search","input":{"q":"pwd"}}`))
	createRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for tool call create, got %d", createRec.Code)
	}

	created := decodeStrictResponse[runtime.ToolCall](t, createRec.Body.Bytes())
	if created.ToolCallID == "" {
		t.Fatal("expected tool call ID")
	}
	if created.CapabilityID != "search" {
		t.Fatalf("expected capability id search, got %s", created.CapabilityID)
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
	if persistedToolCalls[0].CapabilityID != "search" {
		t.Fatalf("expected persisted capability id search, got %s", persistedToolCalls[0].CapabilityID)
	}
}

func TestIntegrationRoutesProjectReadinessAndCanonicalDefault(t *testing.T) {
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
	integrationManager := integrations.NewManager("test")
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     "~/.dope",
			LogLevel:    "info",
			Version:     "test",
		},
		Logger:       telemetry.New("error").Slog(),
		EventBus:     eventBus,
		Integrations: integrationManager,
		Store:        sqliteStore,
	})

	createARec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createARec, httptest.NewRequest(http.MethodPost, "/v1/integrations", strings.NewReader(`{
		"integrationId":"calendar-a",
		"domainKind":"calendar",
		"displayName":"Calendar A",
		"backendKind":"fake_local",
		"accountBinding":{"accountKey":"acct_calendar","accountLabel":"Primary Calendar"},
		"canonicalDefault":true
	}`)))
	if createARec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first integration, got %d body=%s", createARec.Code, createARec.Body.String())
	}
	first := decodeStrictResponse[integrations.Resource](t, createARec.Body.Bytes())
	if !first.CanonicalDefault || first.ReadinessStatus != integrations.ReadinessStatusNotConfigured {
		t.Fatalf("expected canonical not_configured integration, got %+v", first)
	}

	createBRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createBRec, httptest.NewRequest(http.MethodPost, "/v1/integrations", strings.NewReader(`{
		"integrationId":"calendar-b",
		"domainKind":"calendar",
		"displayName":"Calendar B",
		"backendKind":"fake_local",
		"accountBinding":{"accountKey":"acct_calendar","accountLabel":"Primary Calendar"},
		"canonicalDefault":true
	}`)))
	if createBRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for second integration, got %d body=%s", createBRec.Code, createBRec.Body.String())
	}
	second := decodeStrictResponse[integrations.Resource](t, createBRec.Body.Bytes())
	if !second.CanonicalDefault {
		t.Fatalf("expected second integration to become canonical default, got %+v", second)
	}

	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/v1/integrations", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for integration list, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	list := decodeStrictResponse[IntegrationListResponse](t, listRec.Body.Bytes())
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 integrations, got %d", len(list.Items))
	}
	if list.Items[0].CanonicalDefault {
		t.Fatalf("expected first integration to be demoted in list projection, got %+v", list.Items)
	}

	readinessRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(readinessRec, httptest.NewRequest(http.MethodPost, "/v1/integrations/calendar-b/readiness", strings.NewReader(`{
		"readinessStatus":"healthy",
		"authState":"authorized",
		"healthState":"healthy",
		"reason":"probe passed",
		"requiredOperatorAction":"none",
		"secretResolution":"resolved"
	}`)))
	if readinessRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for readiness update, got %d body=%s", readinessRec.Code, readinessRec.Body.String())
	}
	healthy := decodeStrictResponse[integrations.Resource](t, readinessRec.Body.Bytes())
	if healthy.ReadinessStatus != integrations.ReadinessStatusHealthy || healthy.Provenance.SecretResolution != "resolved" {
		t.Fatalf("expected healthy integration with provenance, got %+v", healthy)
	}

	defaultRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(defaultRec, httptest.NewRequest(http.MethodPost, "/v1/integrations/calendar-a/default", nil))
	if defaultRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for canonical default swap, got %d body=%s", defaultRec.Code, defaultRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/v1/integrations/calendar-a", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for integration get, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	got := decodeStrictResponse[integrations.Resource](t, getRec.Body.Bytes())
	if !got.CanonicalDefault || got.EnvironmentScope != "test" {
		t.Fatalf("expected canonical integration in test environment, got %+v", got)
	}

	items, err := sqliteStore.ListEvents(context.Background(), events.Filter{})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	eventNames := make([]string, 0, len(items))
	for _, item := range items {
		eventNames = append(eventNames, item.Name)
	}
	expected := []string{"integration.registered", "integration.updated", "integration.readiness_changed", "integration.default_changed"}
	for _, name := range expected {
		found := false
		for _, item := range eventNames {
			if item == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected event %s in %+v", name, eventNames)
		}
	}
}

func TestIntegrationProbeRoutesLinkRuntimeApprovalAndProvenance(t *testing.T) {
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

	runtimeManager := runtime.NewManager()
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{
		Entrypoint: "operator",
		Goal:       "exercise fake integration probes",
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	eventBus := events.NewBus()
	policyEngine := policy.NewEngine()
	integrationManager := integrations.NewManager("test")
	integration, err := integrationManager.Create(integrations.CreateInput{
		IntegrationID:    "calendar-b",
		DomainKind:       "calendar",
		DisplayName:      "Calendar B",
		EnvironmentScope: "test",
		CanonicalDefault: true,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "acct_calendar",
			AccountLabel: "Primary Calendar",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	})
	if err != nil {
		t.Fatalf("CreateIntegration returned error: %v", err)
	}
	integration, err = integrationManager.UpdateReadiness(integration.IntegrationID, integrations.UpdateReadinessInput{
		ReadinessStatus:  integrations.ReadinessStatusDegraded,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateDegraded,
		ReadinessReason:  "upstream latency",
		SecretResolution: "resolved",
	})
	if err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}
	if err := sqliteStore.UpsertIntegration(context.Background(), integration); err != nil {
		t.Fatalf("UpsertIntegration returned error: %v", err)
	}

	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     "~/.dope",
			LogLevel:    "info",
			Version:     "test",
		},
		Logger:       telemetry.New("error").Slog(),
		EventBus:     eventBus,
		Policy:       policyEngine,
		Runtime:      runtimeManager,
		Integrations: integrationManager,
		Store:        sqliteStore,
		Checkpoints:  checkpoints.NewManager(sqliteStore, runtimeManager),
	})

	inspectRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(inspectRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/integrations/calendar-b/probes", strings.NewReader(`{
		"probeKind":"inspect",
		"input":{"mode":"readonly"}
	}`)))
	if inspectRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for inspect probe, got %d body=%s", inspectRec.Code, inspectRec.Body.String())
	}
	inspect := decodeStrictResponse[IntegrationProbeResponse](t, inspectRec.Body.Bytes())
	if inspect.StepID == "" || inspect.ToolCallID == "" || len(inspect.IntegrationBindings) != 1 {
		t.Fatalf("expected runtime linkage and binding snapshot, got %+v", inspect)
	}
	if inspect.IntegrationBindings[0].ReadinessAtInvocation != integrations.ReadinessStatusDegraded {
		t.Fatalf("expected degraded binding snapshot, got %+v", inspect.IntegrationBindings)
	}

	toolCallRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(toolCallRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/steps/"+inspect.StepID+"/tool-calls/"+inspect.ToolCallID, nil))
	if toolCallRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tool call get, got %d body=%s", toolCallRec.Code, toolCallRec.Body.String())
	}
	toolCall := decodeStrictResponse[runtime.ToolCall](t, toolCallRec.Body.Bytes())
	if len(toolCall.IntegrationBindings) != 1 || toolCall.IntegrationBindings[0].IntegrationID != "calendar-b" {
		t.Fatalf("expected tool call integration bindings, got %+v", toolCall)
	}

	mutatePendingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(mutatePendingRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/integrations/calendar-b/probes", strings.NewReader(`{
		"probeKind":"mutate",
		"input":{"mode":"write"}
	}`)))
	if mutatePendingRec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for mutate probe pending approval, got %d body=%s", mutatePendingRec.Code, mutatePendingRec.Body.String())
	}
	pending := decodeStrictResponse[IntegrationProbeResponse](t, mutatePendingRec.Body.Bytes())
	if pending.Approval == nil || len(pending.Approval.IntegrationBindings) != 1 {
		t.Fatalf("expected approval with integration bindings, got %+v", pending)
	}

	approvalRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(approvalRec, httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+pending.Approval.ApprovalID, nil))
	if approvalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get, got %d body=%s", approvalRec.Code, approvalRec.Body.String())
	}
	approval := decodeStrictResponse[policy.Approval](t, approvalRec.Body.Bytes())
	if len(approval.IntegrationBindings) != 1 || approval.IntegrationBindings[0].IntegrationID != "calendar-b" {
		t.Fatalf("expected approval integration bindings, got %+v", approval)
	}

	if _, _, err := policyEngine.ResolveApproval(pending.Approval.ApprovalID, policy.ResolveApprovalInput{
		Resolution: string(policy.ApprovalStatusApproved),
		Comment:    "approved for tests",
	}); err != nil {
		t.Fatalf("ResolveApproval returned error: %v", err)
	}

	mutateApprovedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(mutateApprovedRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/integrations/calendar-b/probes", strings.NewReader(`{
		"probeKind":"mutate",
		"approvalId":"`+pending.Approval.ApprovalID+`",
		"input":{"mode":"write"}
	}`)))
	if mutateApprovedRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approved mutate probe, got %d body=%s", mutateApprovedRec.Code, mutateApprovedRec.Body.String())
	}

	integration, err = integrationManager.UpdateReadiness(integration.IntegrationID, integrations.UpdateReadinessInput{
		ReadinessStatus: integrations.ReadinessStatusUnavailable,
		AuthState:       integrations.AuthStateExpired,
		HealthState:     integrations.HealthStateUnavailable,
		ReadinessReason: "token revoked",
	})
	if err != nil {
		t.Fatalf("UpdateReadiness(unavailable) returned error: %v", err)
	}
	if err := sqliteStore.UpsertIntegration(context.Background(), integration); err != nil {
		t.Fatalf("UpsertIntegration(unavailable) returned error: %v", err)
	}

	blockedRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(blockedRec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/integrations/calendar-b/probes", strings.NewReader(`{"probeKind":"inspect"}`)))
	if blockedRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for unavailable integration probe, got %d body=%s", blockedRec.Code, blockedRec.Body.String())
	}

	items, err := sqliteStore.ListEvents(context.Background(), events.Filter{RunID: run.RunID})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	foundRequested := false
	foundCompleted := false
	for _, item := range items {
		if item.Name == "tool_call.requested" {
			foundRequested = true
		}
		if item.Name == "tool_call.completed" {
			foundCompleted = true
		}
	}
	if !foundRequested || !foundCompleted {
		t.Fatalf("expected tool call requested/completed events, got %+v", items)
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
	var profileList struct {
		Items []sandbox.Profile `json:"items"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &profileList); err != nil {
		t.Fatalf("failed to decode sandbox profile list: %v", err)
	}
	if len(profileList.Items) < 2 {
		t.Fatalf("expected builtin subprocess and docker profiles, got %+v", profileList.Items)
	}
	foundDocker := false
	for _, item := range profileList.Items {
		if item.BackendKind == sandbox.BackendKindDocker {
			foundDocker = true
			if item.BackendCapability.BackendKind != sandbox.BackendKindDocker || item.BackendCapability.DisplayName == "" {
				t.Fatalf("expected docker backend capability projection, got %+v", item)
			}
		}
	}
	if !foundDocker {
		t.Fatalf("expected docker profile in list, got %+v", profileList.Items)
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
	if explain.Decision.SelectionOutcome != sandbox.BackendSelectionOutcomeSelected {
		t.Fatalf("expected selected explain outcome, got %+v", explain.Decision)
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

func TestSandboxExplainProjectsDockerUnsupportedOutcome(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

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

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		BindAddr:    "127.0.0.1:19191",
		DataDir:     t.TempDir(),
		LogLevel:    "info",
		Version:     "test",
	}
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	server := NewServer(Dependencies{
		Config:    cfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Policy:    policyEngine,
		Sandboxes: sandboxManager,
	})

	cwd := t.TempDir()
	explainReq := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/explain", strings.NewReader(`{
		"profileId":"docker_default",
		"command":"/workspace/run.sh",
		"cwd":"`+cwd+`",
		"access":{"readRoots":["`+cwd+`"],"writeRoots":["`+cwd+`"]}
	}`))
	explainRec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(explainRec, explainReq)
	if explainRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sandbox explain, got %d body=%s", explainRec.Code, explainRec.Body.String())
	}
	explain := decodeStrictResponse[SandboxExplainResponse](t, explainRec.Body.Bytes())
	if explain.Decision.SelectionOutcome != sandbox.BackendSelectionOutcomeUnsupported {
		t.Fatalf("expected unsupported explain outcome, got %+v", explain.Decision)
	}
	if explain.Decision.HostStatus != sandbox.BackendHostStatusMissingPrerequisite {
		t.Fatalf("expected missing prerequisite host status, got %+v", explain.Decision)
	}
	if explain.Decision.MismatchReason != "backend_unavailable" {
		t.Fatalf("expected backend_unavailable mismatch reason, got %+v", explain.Decision)
	}
}

func TestSandboxExplainRouteStaysUnder100ms(t *testing.T) {
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

	cfg := config.Config{
		Environment: config.EnvironmentTest,
		BindAddr:    "127.0.0.1:19191",
		DataDir:     t.TempDir(),
		LogLevel:    "info",
		Version:     "test",
	}
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	server := NewServer(Dependencies{
		Config:    cfg,
		Logger:    telemetry.New("error").Slog(),
		EventBus:  eventBus,
		Policy:    policyEngine,
		Sandboxes: sandboxManager,
	})

	cwd := t.TempDir()
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes/explain", strings.NewReader(`{
		"command":"echo",
		"args":["hello"],
		"cwd":"`+cwd+`",
		"access":{"readRoots":["`+cwd+`"],"writeRoots":["`+cwd+`"]}
	}`))
	rec := httptest.NewRecorder()

	started := time.Now()
	server.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sandbox explain, got %d body=%s", rec.Code, rec.Body.String())
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("expected explain route <=100ms, got %s", elapsed)
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
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	sandboxManager := sandbox.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		BindAddr:    "127.0.0.1:19191",
		DataDir:     t.TempDir(),
		LogLevel:    "info",
		Version:     "test",
	}, sqliteStore, events.NewBus(), policy.NewEngine())
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
		Logger:    logger.Slog(),
		EventBus:  events.NewBus(),
		Router:    router.NewSessionRouter(),
		Runtime:   runtime.NewManager(),
		Sandboxes: sandboxManager,
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
	if len(response.Sandbox.Backends) < 2 {
		t.Fatalf("expected projected sandbox backends, got %+v", response.Sandbox)
	}
	if response.Sandbox.Backends[0].BackendKind == "" {
		t.Fatalf("expected backend capability metadata, got %+v", response.Sandbox.Backends)
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
	sandboxDataDir := filepath.Join(t.TempDir(), "sandbox-runtime")
	sandboxManager := sandbox.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		BindAddr:    "127.0.0.1:19191",
		DataDir:     sandboxDataDir,
		LogLevel:    "info",
		Version:     "test",
	}, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxManager.Close(context.Background()) }()
	logger := telemetry.New("error")
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     sandboxDataDir,
			LogLevel:    "info",
			Version:     "test",
		},
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Auth:         authManager,
		Policy:       policyEngine,
		Router:       router.NewSessionRouter(),
		Runtime:      manager,
		Capabilities: capabilitySupervisor,
		Sandboxes:    sandboxManager,
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

	pendingReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd","cwd":"`+sandboxDataDir+`"}}`))
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

	approvedPendingReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd","cwd":"`+sandboxDataDir+`"}}`))
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

	approvedReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","approvalId":"`+approvedPending.Approval.ApprovalID+`","input":{"cmd":"pwd","cwd":"`+sandboxDataDir+`"}}`))
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
	if approvedToolCall.SandboxExecutionID == "" {
		t.Fatalf("expected approved tool call to link sandbox execution, got %+v", approvedToolCall)
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

func TestHighRiskToolPreflightStaysUnderHundredMilliseconds(t *testing.T) {
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
	authHeader := issueAuthHeaderForTest(t, authManager, "tool-call-latency")

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
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "measure guarded tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"shell","toolName":"shell","input":{"cmd":"pwd"}}`))
	req.Header.Set("Authorization", authHeader)

	started := time.Now()
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	elapsed := time.Since(started)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for approval-gated preflight, got %d body=%s", rec.Code, rec.Body.String())
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("expected local-tool preflight <=100ms, got %s", elapsed)
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

func TestSkillRoutesExposeExecutableManifestAndUnavailableState(t *testing.T) {
	cfg := config.Config{
		BindAddr:    "127.0.0.1:19191",
		DataDir:     filepath.Join(t.TempDir(), "dope-runtime"),
		LogLevel:    "info",
		Version:     "test",
		Environment: config.EnvironmentTest,
	}
	authManager := auth.NewManager()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := cfg.DataDir
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "exec-skill"), `
---
name: exec-skill
description: executable skill
execution.entrypoint: scripts/run.sh
execution.args: static-arg
execution.working_dir: .
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.timeout_ms: 5000
---
instructions
`, "#!/bin/sh\nprintf 'ok %s' \"$1\"")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "skills", "invalid-skill", "SKILL.md"), strings.TrimSpace(`
---
name: invalid-skill
description: invalid executable skill
execution.entrypoint: scripts/missing.sh
---
broken
`))

	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}

	server := NewServer(Dependencies{
		Config: cfg, Logger: telemetry.New("error").Slog(), EventBus: events.NewBus(),
		Auth: authManager, Router: router.NewSessionRouter(), Runtime: runtime.NewManager(),
		Skills: registry, Store: sqliteStore,
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
	if len(listResponse.Items) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(listResponse.Items))
	}

	var execSkill, invalidSkill SkillSummaryResponse
	for _, item := range listResponse.Items {
		switch item.SkillID {
		case "exec-skill":
			execSkill = item
		case "invalid-skill":
			invalidSkill = item
		}
	}
	if execSkill.ExecutionManifest == nil {
		t.Fatalf("expected executable manifest, got %+v", execSkill)
	}
	if execSkill.ExecutionManifest.ApprovalMode != sandbox.ApprovalModeAsk {
		t.Fatalf("expected default ask approval mode, got %+v", execSkill.ExecutionManifest)
	}
	if execSkill.AvailabilityStatus != "available" {
		t.Fatalf("expected available executable skill, got %+v", execSkill)
	}
	if invalidSkill.AvailabilityStatus != "unavailable" || invalidSkill.AvailabilityReason == "" {
		t.Fatalf("expected unavailable invalid skill with reason, got %+v", invalidSkill)
	}
}

func TestSkillToolCallLaunchUsesSandboxExecution(t *testing.T) {
	cfg := config.Config{
		BindAddr:    "127.0.0.1:19191",
		DataDir:     filepath.Join(t.TempDir(), "dope-runtime"),
		LogLevel:    "info",
		Version:     "test",
		Environment: config.EnvironmentTest,
	}
	authManager := auth.NewManager()
	eventBus := events.NewBus()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := cfg.DataDir
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "exec-skill"), `
---
name: exec-skill
description: executable skill
execution.entrypoint: scripts/run.sh
execution.args: static
execution.working_dir: .
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.approval_mode: allow
execution.timeout_ms: 5000
---
instructions
`, "#!/bin/sh\nprintf 'skill:%s' \"$1\"")
	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}

	manager := runtime.NewManager()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policy.NewEngine())
	defer func() { _ = sandboxManager.Close(context.Background()) }()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	server := NewServer(Dependencies{
		Config: cfg, Logger: telemetry.New("error").Slog(), EventBus: eventBus,
		Auth: authManager, Router: router.NewSessionRouter(), Runtime: manager,
		Skills: registry, Sandboxes: sandboxManager, Store: sqliteStore, Checkpoints: checkpointManager,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "skills-exec")

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "run skill"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"skillId":"exec-skill","toolName":"exec-skill"}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for skill tool call create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[runtime.ToolCall](t, createRec.Body.Bytes())
	if created.SkillID != "exec-skill" || created.InvocationKind != runtime.ToolCallInvocationKindSkill {
		t.Fatalf("expected skill-backed tool call, got %+v", created)
	}
	if created.SandboxExecutionID == "" {
		t.Fatalf("expected sandbox execution linkage, got %+v", created)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		got, ok := manager.GetToolCall(run.RunID, step.StepID, created.ToolCallID)
		if ok && got.Status == runtime.ToolCallStatusCompleted {
			output, ok := got.Output.(map[string]any)
			if !ok {
				t.Fatalf("expected structured tool call output, got %+v", got.Output)
			}
			stdout, _ := output["stdout"].(string)
			if !strings.Contains(stdout, "skill:static") {
				t.Fatalf("expected sandbox-backed stdout in tool call output, got %+v", got.Output)
			}
			return
		}
		if ok && (got.Status == runtime.ToolCallStatusFailed || got.Status == runtime.ToolCallStatusDenied || got.Status == runtime.ToolCallStatusCancelled) {
			t.Fatalf("expected completed skill-backed tool call, got %+v", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := manager.GetToolCall(run.RunID, step.StepID, created.ToolCallID)
	t.Fatalf("expected skill-backed tool call to complete, got toolCall=%+v runs=%+v", got, manager.ListRuns())
}

func TestSkillToolCallLaunchUsesRealDockerSandboxExecution(t *testing.T) {
	requireRealDockerForTest(t)

	cfg := config.Config{
		BindAddr:    "127.0.0.1:19191",
		DataDir:     filepath.Join(t.TempDir(), "dope-runtime"),
		LogLevel:    "info",
		Version:     "test",
		Environment: config.EnvironmentTest,
	}
	authManager := auth.NewManager()
	eventBus := events.NewBus()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := cfg.DataDir
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "docker-skill"), `
---
name: docker-skill
description: executable skill on real docker
execution.entrypoint: scripts/run.sh
execution.working_dir: .
execution.profile_id: docker_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.approval_mode: allow
execution.required_enforcement_strength: containerized
execution.timeout_ms: 5000
---
instructions
`, "#!/bin/sh\nprintf 'docker-real'")
	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}

	manager := runtime.NewManager()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policy.NewEngine())
	defer func() { _ = sandboxManager.Close(context.Background()) }()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	server := NewServer(Dependencies{
		Config: cfg, Logger: telemetry.New("error").Slog(), EventBus: eventBus,
		Auth: authManager, Router: router.NewSessionRouter(), Runtime: manager,
		Skills: registry, Sandboxes: sandboxManager, Store: sqliteStore, Checkpoints: checkpointManager,
	})
	authHeader := issueAuthHeaderForTest(t, authManager, "skills-real-docker")

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "run docker skill"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"skillId":"docker-skill","toolName":"docker-skill"}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for docker skill tool call create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[runtime.ToolCall](t, createRec.Body.Bytes())
	if created.SandboxExecutionID == "" {
		t.Fatalf("expected sandbox execution linkage, got %+v", created)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		got, ok := manager.GetToolCall(run.RunID, step.StepID, created.ToolCallID)
		if ok && got.Status == runtime.ToolCallStatusCompleted {
			output, ok := got.Output.(map[string]any)
			if !ok {
				t.Fatalf("expected structured tool call output, got %+v", got.Output)
			}
			stdout, _ := output["stdout"].(string)
			if !strings.Contains(stdout, "docker-real") {
				t.Fatalf("expected real docker stdout, got %+v", got.Output)
			}
			return
		}
		if ok && (got.Status == runtime.ToolCallStatusFailed || got.Status == runtime.ToolCallStatusDenied || got.Status == runtime.ToolCallStatusCancelled) {
			t.Fatalf("expected completed real docker skill-backed tool call, got %+v", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := manager.GetToolCall(run.RunID, step.StepID, created.ToolCallID)
	t.Fatalf("expected real docker skill-backed tool call to complete, got %+v", got)
}

func TestSkillToolCallRedactsSecretOutputAndSandboxProjection(t *testing.T) {
	t.Setenv("DOPE_ENV", "test")

	cfg := config.Config{
		BindAddr:    "127.0.0.1:19191",
		DataDir:     filepath.Join(t.TempDir(), "dope-runtime"),
		LogLevel:    "info",
		Version:     "test",
		Environment: config.EnvironmentTest,
	}
	eventBus := events.NewBus()
	policyEngine := policy.NewEngine()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := cfg.DataDir
	writeExecutableSkillSecretsForTest(t, dataRoot, map[string]string{"EXEC_SKILL_TOKEN": "top-secret-token"})
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "secret-skill"), `
---
name: secret-skill
description: executable skill with env secret
execution.entrypoint: scripts/run.sh
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.approval_mode: allow
execution.secret_refs: EXEC_SKILL_TOKEN
execution.timeout_ms: 5000
---
instructions
`, "#!/bin/sh\nprintf '%s' \"$EXEC_SKILL_TOKEN\"")
	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}

	manager := runtime.NewManager()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxManager.Close(context.Background()) }()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	server := NewServer(Dependencies{
		Config: cfg, Logger: telemetry.New("error").Slog(), EventBus: eventBus,
		Policy: policyEngine, Router: router.NewSessionRouter(), Runtime: manager,
		Skills: registry, Sandboxes: sandboxManager, Store: sqliteStore, Checkpoints: checkpointManager,
	})

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "run secret skill"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"skillId":"secret-skill","toolName":"secret-skill"}`))
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for skill tool call create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[runtime.ToolCall](t, createRec.Body.Bytes())
	if created.SandboxExecutionID == "" {
		t.Fatalf("expected sandbox execution linkage, got %+v", created)
	}

	deadline := time.Now().Add(5 * time.Second)
	var terminal runtime.ToolCall
	for time.Now().Before(deadline) {
		got, ok := manager.GetToolCall(run.RunID, step.StepID, created.ToolCallID)
		if !ok {
			t.Fatalf("expected tool call %s", created.ToolCallID)
		}
		if got.Status == runtime.ToolCallStatusCompleted {
			terminal = got
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if terminal.ToolCallID == "" {
		t.Fatalf("tool call %s did not complete", created.ToolCallID)
	}
	output, ok := terminal.Output.(map[string]any)
	if !ok {
		t.Fatalf("expected structured tool output, got %+v", terminal.Output)
	}
	if output["stdout"] != "[REDACTED]" {
		t.Fatalf("expected redacted stdout, got %+v", output)
	}
	if strings.Contains(createRec.Body.String(), "top-secret-token") {
		t.Fatalf("create response leaked secret: %s", createRec.Body.String())
	}

	toolCallReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls/"+created.ToolCallID, nil)
	toolCallRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(toolCallRec, toolCallReq)
	if toolCallRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tool call get, got %d body=%s", toolCallRec.Code, toolCallRec.Body.String())
	}
	if strings.Contains(toolCallRec.Body.String(), "top-secret-token") {
		t.Fatalf("tool call response leaked secret: %s", toolCallRec.Body.String())
	}
	gotToolCall := decodeStrictResponse[runtime.ToolCall](t, toolCallRec.Body.Bytes())
	toolOutput, ok := gotToolCall.Output.(map[string]any)
	if !ok || toolOutput["stdout"] != "[REDACTED]" {
		t.Fatalf("expected redacted persisted tool output, got %+v", gotToolCall.Output)
	}
	toolSecretScope, ok := gotToolCall.Sandbox["secretScope"].([]any)
	if !ok || len(toolSecretScope) != 1 {
		t.Fatalf("expected secret scope on tool call sandbox view, got %+v", gotToolCall.Sandbox)
	}

	executionReq := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/executions/"+created.SandboxExecutionID, nil)
	executionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(executionRec, executionReq)
	if executionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for sandbox execution get, got %d body=%s", executionRec.Code, executionRec.Body.String())
	}
	if strings.Contains(executionRec.Body.String(), "top-secret-token") {
		t.Fatalf("sandbox execution response leaked secret: %s", executionRec.Body.String())
	}
	execution := decodeStrictResponse[sandbox.Execution](t, executionRec.Body.Bytes())
	if execution.Result.Stdout != "[REDACTED]" {
		t.Fatalf("expected redacted sandbox stdout, got %+v", execution.Result)
	}
	if execution.Consumer == nil || len(execution.Consumer.SecretScope) != 1 || execution.Consumer.SecretScope[0].Resolution != sandbox.SecretResolutionResolved {
		t.Fatalf("expected resolved secret scope on sandbox execution, got %+v", execution.Consumer)
	}
}

func TestApprovalRoutesPreserveExecutableSkillDeclarationProvenance(t *testing.T) {
	t.Setenv("DOPE_ENV", "test")

	cfg := config.Config{
		BindAddr:    "127.0.0.1:19191",
		DataDir:     filepath.Join(t.TempDir(), "dope-runtime"),
		LogLevel:    "info",
		Version:     "test",
		Environment: config.EnvironmentTest,
	}
	eventBus := events.NewBus()
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := cfg.DataDir
	writeExecutableSkillSecretsForTest(t, dataRoot, map[string]string{"EXEC_SKILL_TOKEN": "top-secret-token"})
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "approval-skill"), `
---
name: approval-skill
description: executable skill with approval
execution.entrypoint: scripts/run.sh
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.approval_mode: ask
execution.secret_refs: EXEC_SKILL_TOKEN
execution.timeout_ms: 5000
---
instructions
`, "#!/bin/sh\nprintf ok")
	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}

	manager := runtime.NewManager()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxManager.Close(context.Background()) }()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	server := NewServer(Dependencies{
		Config: cfg, Logger: telemetry.New("error").Slog(), EventBus: eventBus,
		Policy: policyEngine, Auth: authManager, Router: router.NewSessionRouter(), Runtime: manager,
		Skills: registry, Sandboxes: sandboxManager, Store: sqliteStore, Checkpoints: checkpointManager,
	})

	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "run approval skill"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	authHeader := issueAuthHeaderForTest(t, authManager, "skills-approval")

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"skillId":"approval-skill","toolName":"approval-skill"}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for approval-gated skill call, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var pending struct {
		Approval policy.Approval `json:"approval"`
		Decision policy.Decision `json:"decision"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("failed to decode pending approval response: %v", err)
	}
	assertSandboxSecretScope(t, pending.Approval.Sandbox, "approval-skill", "EXEC_SKILL_TOKEN", "test", "resolved")
	declaration, ok := pending.Approval.Sandbox["declaration"].(map[string]any)
	if !ok {
		t.Fatalf("expected declaration payload, got %+v", pending.Approval.Sandbox)
	}
	if declaration["profileId"] != "subprocess_default" || declaration["approvalMode"] != "ask" {
		t.Fatalf("expected persisted executable skill declaration, got %+v", declaration)
	}
	if secretRefs, ok := declaration["secretRefs"].([]any); !ok || len(secretRefs) != 1 || secretRefs[0] != "EXEC_SKILL_TOKEN" {
		t.Fatalf("expected secret ref provenance, got %+v", declaration)
	}

	approvalGetReq := httptest.NewRequest(http.MethodGet, "/v1/policy/approvals/"+pending.Approval.ApprovalID, nil)
	approvalGetReq.Header.Set("Authorization", authHeader)
	approvalGetRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(approvalGetRec, approvalGetReq)
	if approvalGetRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval get, got %d body=%s", approvalGetRec.Code, approvalGetRec.Body.String())
	}
	gotApproval := decodeStrictResponse[policy.Approval](t, approvalGetRec.Body.Bytes())
	declaration, ok = gotApproval.Sandbox["declaration"].(map[string]any)
	if !ok || declaration["profileId"] != "subprocess_default" || declaration["approvalMode"] != "ask" {
		t.Fatalf("expected approval get to preserve executable declaration, got %+v", gotApproval.Sandbox)
	}
}

func TestExecCapabilityApprovalAlsoAuthorizesSandboxCommandEscalation(t *testing.T) {
	eventBus := events.NewBus()
	authManager := auth.NewManager()
	manager := runtime.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	logger := telemetry.New("error")
	policyEngine := policy.NewEngine()
	cfg := config.Config{
		BindAddr: "127.0.0.1:19191",
		DataDir:  filepath.Join(t.TempDir(), "dope"),
		LogLevel: "info",
		Version:  "test",
	}
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxManager.Close(context.Background()) }()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	server := NewServer(Dependencies{
		Config: cfg, Logger: logger.Slog(), EventBus: eventBus, Policy: policyEngine, Auth: authManager,
		Router: router.NewSessionRouter(), Runtime: manager, Capabilities: capabilitySupervisor,
		Sandboxes: sandboxManager, Store: sqliteStore, Checkpoints: checkpointManager,
	})

	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "exec",
		Kind:         "exec",
		DisplayName:  "Exec",
	}); err != nil {
		t.Fatalf("Register exec capability returned error: %v", err)
	}
	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "run exec tool"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	targetDir := t.TempDir()
	targetFile := filepath.Join(targetDir, "remove-me.txt")
	writeSkillFileForTest(t, targetFile, "payload")
	authHeader := issueAuthHeaderForTest(t, authManager, "exec-web")

	pendingReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"exec","toolName":"exec","input":{"command":"rm","args":["`+targetFile+`"],"cwd":"`+targetDir+`"}}`))
	pendingReq.Header.Set("Authorization", authHeader)
	pendingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(pendingRec, pendingReq)
	if pendingRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for exec approval, got %d body=%s", pendingRec.Code, pendingRec.Body.String())
	}
	var pending struct {
		Approval policy.Approval `json:"approval"`
	}
	if err := json.Unmarshal(pendingRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("failed to decode pending approval: %v", err)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+pending.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"approved","comment":"approved for exec"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for approval resolve, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"exec","toolName":"exec","approvalId":"`+pending.Approval.ApprovalID+`","input":{"command":"rm","args":["`+targetFile+`"],"cwd":"`+targetDir+`"}}`))
	createReq.Header.Set("Authorization", authHeader)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approved exec tool call, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[runtime.ToolCall](t, createRec.Body.Bytes())
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, ok := manager.GetToolCall(run.RunID, step.StepID, created.ToolCallID)
		if !ok {
			t.Fatalf("expected tool call %s", created.ToolCallID)
		}
		if got.Status == runtime.ToolCallStatusCompleted {
			break
		}
		if got.Status == runtime.ToolCallStatusFailed || got.Status == runtime.ToolCallStatusDenied || got.Status == runtime.ToolCallStatusCancelled {
			t.Fatalf("expected single approved execution path, got %+v", got)
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Fatalf("expected rm command to complete without second approval, stat err=%v", err)
	}
}

func TestSupportedSkillAndLocalToolCallsStaySandboxAligned(t *testing.T) {
	t.Setenv("DOPE_ENV", "test")

	cfg := config.Config{
		BindAddr:    "127.0.0.1:19191",
		DataDir:     filepath.Join(t.TempDir(), "dope-runtime"),
		LogLevel:    "info",
		Version:     "test",
		Environment: config.EnvironmentTest,
	}
	eventBus := events.NewBus()
	authManager := auth.NewManager()
	manager := runtime.NewManager()
	capabilitySupervisor := capabilities.NewSupervisor()
	logger := telemetry.New("error")
	policyEngine := policy.NewEngine()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := cfg.DataDir
	writeExecutableSkillSecretsForTest(t, dataRoot, map[string]string{"EXEC_SKILL_TOKEN": "test-secret"})
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "exec-skill"), `
---
name: exec-skill
description: executable skill
execution.entrypoint: scripts/run.sh
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.approval_mode: allow
execution.secret_refs: EXEC_SKILL_TOKEN
execution.timeout_ms: 5000
---
instructions
`, "#!/bin/sh\nprintf 'skill path'")
	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	defer func() { _ = sandboxManager.Close(context.Background()) }()
	checkpointManager := checkpoints.NewManager(sqliteStore, manager)
	server := NewServer(Dependencies{
		Config: cfg, Logger: logger.Slog(), EventBus: eventBus, Policy: policyEngine, Auth: authManager,
		Router: router.NewSessionRouter(), Runtime: manager, Skills: registry, Capabilities: capabilitySupervisor,
		Sandboxes: sandboxManager, Store: sqliteStore, Checkpoints: checkpointManager,
	})

	if _, _, err := capabilitySupervisor.Register(capabilities.RegisterInput{
		CapabilityID: "exec",
		Kind:         "exec",
		DisplayName:  "Exec",
	}); err != nil {
		t.Fatalf("Register exec capability returned error: %v", err)
	}
	run, err := manager.CreateRun(runtime.CreateRunInput{Entrypoint: "chat"})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := manager.CreateStep(run.RunID, runtime.CreateStepInput{Title: "sandbox alignment"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	authHeader := issueAuthHeaderForTest(t, authManager, "sandbox-alignment")

	skillReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"skillId":"exec-skill","toolName":"exec-skill"}`))
	skillReq.Header.Set("Authorization", authHeader)
	skillRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(skillRec, skillReq)
	if skillRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for skill tool call create, got %d body=%s", skillRec.Code, skillRec.Body.String())
	}
	skillCreated := decodeStrictResponse[runtime.ToolCall](t, skillRec.Body.Bytes())
	skillTerminal := waitForToolCallTerminalState(t, manager, run.RunID, step.StepID, skillCreated.ToolCallID)
	if skillTerminal.Status != runtime.ToolCallStatusCompleted {
		t.Fatalf("expected completed skill tool call, got %+v", skillTerminal)
	}

	skillGetReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls/"+skillCreated.ToolCallID, nil)
	skillGetReq.Header.Set("Authorization", authHeader)
	skillGetRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(skillGetRec, skillGetReq)
	if skillGetRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skill tool call get, got %d body=%s", skillGetRec.Code, skillGetRec.Body.String())
	}
	gotSkill := decodeStrictResponse[runtime.ToolCall](t, skillGetRec.Body.Bytes())
	if gotSkill.InvocationKind != runtime.ToolCallInvocationKindSkill || gotSkill.SkillID != "exec-skill" || gotSkill.SandboxExecutionID == "" {
		t.Fatalf("expected sandbox-linked skill tool call, got %+v", gotSkill)
	}
	assertSandboxDeclaration(t, gotSkill.Sandbox, "skill", "exec-skill", "tool_call.execute")
	assertSandboxPolicyRecord(t, gotSkill.Sandbox, "", "resolved")
	skillExecutionReq := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/executions/"+gotSkill.SandboxExecutionID, nil)
	skillExecutionReq.Header.Set("Authorization", authHeader)
	skillExecutionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(skillExecutionRec, skillExecutionReq)
	if skillExecutionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for skill sandbox execution get, got %d body=%s", skillExecutionRec.Code, skillExecutionRec.Body.String())
	}
	skillExecution := decodeStrictResponse[sandbox.Execution](t, skillExecutionRec.Body.Bytes())
	if skillExecution.ResourceKind != "skill" || skillExecution.ResourceID != "exec-skill" || skillExecution.Consumer == nil || skillExecution.Consumer.PolicyRecord == nil {
		t.Fatalf("expected skill sandbox execution provenance, got %+v", skillExecution)
	}
	if skillExecution.Consumer.PolicyRecord.ToolCallID != gotSkill.ToolCallID || skillExecution.Consumer.PolicyRecord.SandboxExecutionID != gotSkill.SandboxExecutionID {
		t.Fatalf("expected skill execution linkage, got %+v", skillExecution.Consumer.PolicyRecord)
	}

	targetDir := t.TempDir()
	targetFile := filepath.Join(targetDir, "remove-me.txt")
	writeSkillFileForTest(t, targetFile, "payload")

	pendingReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"exec","toolName":"exec","input":{"command":"rm","args":["`+targetFile+`"],"cwd":"`+targetDir+`"}}`))
	pendingReq.Header.Set("Authorization", authHeader)
	pendingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(pendingRec, pendingReq)
	if pendingRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for approval-gated exec call, got %d body=%s", pendingRec.Code, pendingRec.Body.String())
	}
	var pending struct {
		Approval policy.Approval `json:"approval"`
	}
	if err := json.Unmarshal(pendingRec.Body.Bytes(), &pending); err != nil {
		t.Fatalf("failed to decode pending approval response: %v", err)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/v1/policy/approvals/"+pending.Approval.ApprovalID+"/resolve", strings.NewReader(`{"resolution":"approved","comment":"ok"}`))
	resolveReq.Header.Set("Authorization", authHeader)
	resolveRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exec approval resolve, got %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}

	localReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls", strings.NewReader(`{"capabilityId":"exec","toolName":"exec","approvalId":"`+pending.Approval.ApprovalID+`","input":{"command":"rm","args":["`+targetFile+`"],"cwd":"`+targetDir+`"}}`))
	localReq.Header.Set("Authorization", authHeader)
	localRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(localRec, localReq)
	if localRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for approved exec tool call, got %d body=%s", localRec.Code, localRec.Body.String())
	}
	localCreated := decodeStrictResponse[runtime.ToolCall](t, localRec.Body.Bytes())
	localTerminal := waitForToolCallTerminalState(t, manager, run.RunID, step.StepID, localCreated.ToolCallID)
	if localTerminal.Status != runtime.ToolCallStatusCompleted {
		t.Fatalf("expected completed local tool call, got %+v", localTerminal)
	}

	localGetReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+run.RunID+"/steps/"+step.StepID+"/tool-calls/"+localCreated.ToolCallID, nil)
	localGetReq.Header.Set("Authorization", authHeader)
	localGetRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(localGetRec, localGetReq)
	if localGetRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for local tool call get, got %d body=%s", localGetRec.Code, localGetRec.Body.String())
	}
	gotLocal := decodeStrictResponse[runtime.ToolCall](t, localGetRec.Body.Bytes())
	if gotLocal.InvocationKind != runtime.ToolCallInvocationKindLocalTool || gotLocal.CapabilityID != "exec" || gotLocal.SandboxExecutionID == "" {
		t.Fatalf("expected sandbox-linked local tool call, got %+v", gotLocal)
	}
	assertSandboxDeclaration(t, gotLocal.Sandbox, "local_tool", "exec", "tool_call.execute")
	assertSandboxPolicyRecord(t, gotLocal.Sandbox, "", "not_applicable")
	localExecutionReq := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/executions/"+gotLocal.SandboxExecutionID, nil)
	localExecutionReq.Header.Set("Authorization", authHeader)
	localExecutionRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(localExecutionRec, localExecutionReq)
	if localExecutionRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for local-tool sandbox execution get, got %d body=%s", localExecutionRec.Code, localExecutionRec.Body.String())
	}
	localExecution := decodeStrictResponse[sandbox.Execution](t, localExecutionRec.Body.Bytes())
	if localExecution.ResourceKind != "capability" || localExecution.ResourceID != "exec" || localExecution.Consumer == nil || localExecution.Consumer.PolicyRecord == nil {
		t.Fatalf("expected local-tool sandbox execution provenance, got %+v", localExecution)
	}
	if localExecution.Consumer.PolicyRecord.ToolCallID != gotLocal.ToolCallID || localExecution.Consumer.PolicyRecord.SandboxExecutionID != gotLocal.SandboxExecutionID {
		t.Fatalf("expected local-tool execution linkage, got %+v", localExecution.Consumer.PolicyRecord)
	}
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Fatalf("expected approved exec command to complete through sandbox, stat err=%v", err)
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
		EventID:          "evt_stream_1",
		EnvironmentScope: "test",
		Category:         "run",
		Name:             "run.created",
		OccurredAt:       now,
		Resource:         events.Resource{Kind: "run", ID: "run_1"},
	})
	if err != nil {
		t.Fatalf("AppendEvent(first) returned error: %v", err)
	}
	eventBus.Publish(first)
	second, err := sqliteStore.AppendEvent(context.Background(), events.Event{
		EventID:          "evt_stream_2",
		EnvironmentScope: "test",
		Category:         "run",
		Name:             "run.status_changed",
		OccurredAt:       first.OccurredAt.Add(time.Second),
		Resource:         events.Resource{Kind: "run", ID: "run_1"},
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

func TestWorkflowInspectionRoutesExposePlanAndEnvironmentScopedDetail(t *testing.T) {
	t.Parallel()

	harness := newWorkflowServerHarness(t, workflowServerHarnessOptions{
		skillScript: "#!/bin/sh\nprintf 'workflow-ok %s' \"$1\"",
	})

	prodWorkflow := orchestration.Workflow{
		WorkflowID:       "wf_prod_hidden",
		RunID:            harness.run.RunID,
		EnvironmentScope: "prod",
		Goal:             "hidden",
		Status:           orchestration.WorkflowStatusPlanned,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := harness.store.UpsertWorkflow(context.Background(), prodWorkflow); err != nil {
		t.Fatalf("UpsertWorkflow returned error: %v", err)
	}

	created := createWorkflowForTest(t, harness.server, harness.run.RunID, `{}`)
	assertWorkflowInspectionResponse(t, created, 1)

	list := listWorkflowsForTest(t, harness.server, harness.run.RunID)
	if len(list.Items) != 1 {
		t.Fatalf("expected only test-environment workflow, got %+v", list)
	}

	startedAt := time.Now()
	got := getWorkflowForTest(t, harness.server, harness.run.RunID, created.WorkflowID)
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("expected workflow detail retrieval under 500ms, took %s", elapsed)
	}
	if got.EnvironmentScope != "test" {
		t.Fatalf("expected test environment scope, got %+v", got)
	}
	assertWorkflowInspectionResponse(t, got, 1)
}

func TestWorkflowExecutionRoutesCancelAndPreserveLegacyToolCallBehavior(t *testing.T) {
	t.Parallel()

	harness := newWorkflowServerHarness(t, workflowServerHarnessOptions{
		runGoal:     "workflow-cancel",
		skillScript: "#!/bin/sh\nif [ \"$1\" = \"workflow-cancel\" ]; then sleep 5; printf 'cancel-too-late'; else printf 'legacy:%s' \"$1\"; fi",
	})

	created := createWorkflowForTest(t, harness.server, harness.run.RunID, `{}`)
	started := startWorkflowForTest(t, harness.server, harness.run.RunID, created.WorkflowID)
	assertWorkflowRuntimeLinkage(t, started)

	cancelled := cancelWorkflowForTest(t, harness.server, harness.run.RunID, created.WorkflowID)
	if cancelled.Status != orchestration.WorkflowStatusCancelled {
		t.Fatalf("expected cancelled workflow, got %+v", cancelled)
	}
	if cancelled.CompletedAt == nil || cancelled.Steps[0].Status != orchestration.StepStatusCancelled {
		t.Fatalf("expected cancelled terminal truth, got %+v", cancelled)
	}

	step, ok := harness.runtime.GetStep(harness.run.RunID, cancelled.Steps[0].RuntimeStepID)
	if !ok {
		t.Fatal("expected runtime step for cancelled workflow")
	}
	if step.Status != runtime.StepStatusCancelled {
		t.Fatalf("expected cancelled runtime step, got %+v", step)
	}
	toolCall := waitForToolCallTerminalState(t, harness.runtime, harness.run.RunID, cancelled.Steps[0].RuntimeStepID, cancelled.Steps[0].ActiveToolCallID)
	if toolCall.Status != runtime.ToolCallStatusCancelled {
		t.Fatalf("expected cancelled tool call, got %+v", toolCall)
	}

	legacyStep, err := harness.runtime.CreateStep(harness.run.RunID, runtime.CreateStepInput{Title: "legacy skill call"})
	if err != nil {
		t.Fatalf("CreateStep returned error: %v", err)
	}
	legacyReq := httptest.NewRequest(http.MethodPost, "/v1/runs/"+harness.run.RunID+"/steps/"+legacyStep.StepID+"/tool-calls", strings.NewReader(`{"skillId":"exec-skill","toolName":"exec-skill","input":{"args":["legacy"]}}`))
	legacyRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(legacyRec, legacyReq)
	if legacyRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for legacy tool call create, got %d body=%s", legacyRec.Code, legacyRec.Body.String())
	}
	legacyCreated := decodeStrictResponse[runtime.ToolCall](t, legacyRec.Body.Bytes())
	legacyToolCall := waitForToolCallTerminalState(t, harness.runtime, harness.run.RunID, legacyStep.StepID, legacyCreated.ToolCallID)
	if legacyToolCall.Status != runtime.ToolCallStatusCompleted {
		t.Fatalf("expected legacy tool call success, got %+v", legacyToolCall)
	}
	if legacyToolCall.WorkflowID != "" || legacyToolCall.WorkflowStepID != "" {
		t.Fatalf("expected legacy tool call to remain non-workflow scoped, got %+v", legacyToolCall)
	}
}

func TestWorkflowExecutionRoutesRecordMixedPartialFailureAndAvoidCrossEnvironmentLeakage(t *testing.T) {
	t.Parallel()

	harness := newWorkflowServerHarness(t, workflowServerHarnessOptions{
		enableMCP:    true,
		skillScript:  "#!/bin/sh\nprintf 'fail:%s' \"$1\" >&2\nexit 1",
		hiddenProdID: "wf_prod_hidden",
	})

	created := createWorkflowForTest(t, harness.server, harness.run.RunID, `{}`)
	if len(created.Steps) != 2 {
		t.Fatalf("expected mixed-family workflow plan, got %+v", created)
	}

	startWorkflowForTest(t, harness.server, harness.run.RunID, created.WorkflowID)
	final := waitForWorkflowStatus(t, harness.server, harness.run.RunID, created.WorkflowID, orchestration.WorkflowStatusPartialFailed)
	if final.Status != orchestration.WorkflowStatusPartialFailed {
		t.Fatalf("expected partial_failed workflow, got %+v", final)
	}
	if len(final.Steps) != 2 || final.Steps[0].Status != orchestration.StepStatusCompleted || final.Steps[1].Status != orchestration.StepStatusFailed {
		t.Fatalf("expected mixed step outcomes, got %+v", final.Steps)
	}
	if final.Steps[1].AttemptCount != 2 {
		t.Fatalf("expected bounded retry count of 2, got %+v", final.Steps[1])
	}
	assertWorkflowRuntimeLinkage(t, final)
	if len(final.Handoffs) != 1 || final.Handoffs[0].Status != orchestration.HandoffStatusConsumed {
		t.Fatalf("expected visible consumed handoff, got %+v", final.Handoffs)
	}

	list := listWorkflowsForTest(t, harness.server, harness.run.RunID)
	if len(list.Items) != 1 || list.Items[0].WorkflowID != created.WorkflowID {
		t.Fatalf("expected no cross-environment leakage, got %+v", list)
	}
	missingRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(missingRec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+harness.run.RunID+"/workflows/"+harness.hiddenProdWorkflowID, nil))
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected hidden prod workflow to stay invisible, got %d body=%s", missingRec.Code, missingRec.Body.String())
	}
}

func TestWorkflowExecutionPublishesScopedWorkflowTransitionEvents(t *testing.T) {
	t.Parallel()

	harness := newWorkflowServerHarness(t, workflowServerHarnessOptions{
		skillScript: "#!/bin/sh\nprintf 'workflow-ok %s' \"$1\"",
	})

	created := createWorkflowForTest(t, harness.server, harness.run.RunID, `{}`)
	startWorkflowForTest(t, harness.server, harness.run.RunID, created.WorkflowID)
	final := waitForWorkflowStatus(t, harness.server, harness.run.RunID, created.WorkflowID, orchestration.WorkflowStatusCompleted)
	if final.Status != orchestration.WorkflowStatusCompleted {
		t.Fatalf("expected completed workflow, got %+v", final)
	}

	events := harness.eventBus.List(events.Filter{Category: "workflow", RunID: harness.run.RunID})
	if len(events) < 4 {
		t.Fatalf("expected workflow lifecycle events, got %+v", events)
	}

	planned := findNamedEvent(t, events, "workflow.planned")
	if planned.Scope.WorkflowID != created.WorkflowID {
		t.Fatalf("expected workflow.planned scope to include workflowId, got %+v", planned)
	}

	started := findNamedEvent(t, events, "workflow.started")
	if started.Scope.WorkflowID != created.WorkflowID || started.Payload["status"] != string(orchestration.WorkflowStatusRunning) {
		t.Fatalf("expected workflow.started to project running workflow scope, got %+v", started)
	}

	stepChanged := findNamedEvent(t, events, "workflow.step_status_changed")
	if stepChanged.Scope.WorkflowID != created.WorkflowID || stepChanged.Scope.WorkflowStepID != final.Steps[0].WorkflowStepID {
		t.Fatalf("expected step event scope to include workflow and step ids, got %+v", stepChanged)
	}
	if stepChanged.Payload["status"] != string(orchestration.StepStatusCompleted) {
		t.Fatalf("expected step event to project step status, got %+v", stepChanged.Payload)
	}

	statusChanged := findNamedEvent(t, events, "workflow.status_changed")
	if statusChanged.Scope.WorkflowID != created.WorkflowID || statusChanged.Payload["status"] != string(orchestration.WorkflowStatusCompleted) {
		t.Fatalf("expected workflow.status_changed to project completed workflow state, got %+v", statusChanged)
	}
}

type workflowServerHarnessOptions struct {
	runGoal      string
	skillScript  string
	enableMCP    bool
	hiddenProdID string
}

type workflowServerHarness struct {
	cfg                  config.Config
	server               *Server
	eventBus             *events.Bus
	store                *store.SQLiteStore
	runtime              *runtime.Manager
	run                  runtime.Run
	hiddenProdWorkflowID string
}

func newWorkflowServerHarness(t *testing.T, opts workflowServerHarnessOptions) workflowServerHarness {
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
	policyEngine := policy.NewEngine()
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	t.Cleanup(func() { _ = sandboxManager.Close(context.Background()) })

	registry := newAllowSkillRegistryForWorkflowHarness(t, cfg.DataDir, firstNonEmpty(opts.skillScript, "#!/bin/sh\nprintf 'workflow-ok %s' \"$1\""))
	runtimeManager := runtime.NewManager()
	runGoal := firstNonEmpty(opts.runGoal, "Use a skill to complete a deterministic workflow.")
	run, err := runtimeManager.CreateRun(runtime.CreateRunInput{Entrypoint: "operator", Goal: runGoal})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if err := sqliteStore.UpsertRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertRun returned error: %v", err)
	}

	var mcpManager *mcp.Manager
	if opts.enableMCP {
		writeAPIMCPSecretsFileForTest(t, cfg.DataDir, map[string]string{
			"GO_WANT_API_MCP_HELPER": "1",
			"API_MCP_HELPER_TOOLS":   `[{"name":"lookup","title":"Lookup","description":"Lookup tool","inputSchema":{"type":"object","properties":{"query":{"type":"string"}}}}]`,
		})
		mcpManager = mcp.NewManager(cfg, sqliteStore, eventBus, sandboxManager, policyEngine, mcp.NewTransportMux(mcp.NewStdioTransport(), nil))
		if _, _, err := mcpManager.CreateServer(context.Background(), mcp.CreateServerInput{
			ServerID:         "api-workflow-mcp",
			DisplayName:      "API Workflow MCP",
			Enabled:          true,
			SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
			DeclarationID:    "mcp_server:api-workflow-mcp:lifecycle.start",
			TransportKind:    mcp.TransportKindStdio,
			Command:          os.Args[0],
			Args:             []string{"-test.run=TestAPIMCPHelperProcess", "--"},
			WorkingDir:       t.TempDir(),
			SecretRefs:       []string{"GO_WANT_API_MCP_HELPER", "API_MCP_HELPER_TOOLS"},
			AutoRestart:      true,
		}); err != nil {
			t.Fatalf("CreateServer returned error: %v", err)
		}
		started, err := mcpManager.Start(context.Background(), "api-workflow-mcp", "workflow:test")
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
		if started.Server.State.Status != mcp.LifecycleStatusHealthy {
			t.Fatalf("expected healthy mcp server, got %+v", started.Server.State)
		}
		if _, err := mcpManager.UpdateToolExposure(context.Background(), "api-workflow-mcp", "lookup", mcp.UpdateExposureInput{
			RuntimeSurface: "chat",
			ExposureMode:   mcp.ExposureModeAllow,
			Active:         true,
			Reason:         "workflow tests require direct allow-mode MCP execution",
		}); err != nil {
			t.Fatalf("UpdateToolExposure returned error: %v", err)
		}
	}

	server := NewServer(Dependencies{
		Config:      cfg,
		Logger:      telemetry.New("error").Slog(),
		EventBus:    eventBus,
		Policy:      policyEngine,
		Runtime:     runtimeManager,
		Skills:      registry,
		Sandboxes:   sandboxManager,
		MCP:         mcpManager,
		Store:       sqliteStore,
		Checkpoints: checkpoints.NewManager(sqliteStore, runtimeManager),
	})

	if opts.hiddenProdID != "" {
		hidden := orchestration.Workflow{
			WorkflowID:       opts.hiddenProdID,
			RunID:            run.RunID,
			EnvironmentScope: "prod",
			Goal:             "hidden",
			Status:           orchestration.WorkflowStatusPlanned,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		}
		if err := sqliteStore.UpsertWorkflow(context.Background(), hidden); err != nil {
			t.Fatalf("UpsertWorkflow returned error: %v", err)
		}
	}

	return workflowServerHarness{
		cfg:                  cfg,
		server:               server,
		eventBus:             eventBus,
		store:                sqliteStore,
		runtime:              runtimeManager,
		run:                  run,
		hiddenProdWorkflowID: opts.hiddenProdID,
	}
}

func newAllowSkillRegistryForWorkflowHarness(t *testing.T, dataRoot, script string) *skills.Registry {
	t.Helper()

	homeRoot := filepath.Join(t.TempDir(), ".agents")
	writeSkillFileForTest(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeSkillFileForTest(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeExecutableSkillForTest(t, filepath.Join(dataRoot, "skills", "exec-skill"), `
---
name: exec-skill
description: executable skill
execution.entrypoint: scripts/run.sh
execution.working_dir: .
execution.profile_id: subprocess_default
execution.read_roots: .
execution.write_roots: .
execution.network_mode: deny
execution.timeout_ms: 5000
execution.approval_mode: allow
---
workflow test skill
`, script)

	registry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}
	return registry
}

func createWorkflowForTest(t *testing.T, server *Server, runID, body string) orchestration.Workflow {
	t.Helper()
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID+"/workflows", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeStrictResponse[orchestration.Workflow](t, rec.Body.Bytes())
}

func listWorkflowsForTest(t *testing.T, server *Server, runID string) WorkflowListResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID+"/workflows", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeStrictResponse[WorkflowListResponse](t, rec.Body.Bytes())
}

func getWorkflowForTest(t *testing.T, server *Server, runID, workflowID string) orchestration.Workflow {
	t.Helper()
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID+"/workflows/"+workflowID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeStrictResponse[orchestration.Workflow](t, rec.Body.Bytes())
}

func startWorkflowForTest(t *testing.T, server *Server, runID, workflowID string) orchestration.Workflow {
	t.Helper()
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID+"/workflows/"+workflowID+"/start", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeStrictResponse[orchestration.Workflow](t, rec.Body.Bytes())
}

func cancelWorkflowForTest(t *testing.T, server *Server, runID, workflowID string) orchestration.Workflow {
	t.Helper()
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/runs/"+runID+"/workflows/"+workflowID+"/cancel", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	return decodeStrictResponse[orchestration.Workflow](t, rec.Body.Bytes())
}

func waitForWorkflowStatus(t *testing.T, server *Server, runID, workflowID string, want orchestration.WorkflowStatus) orchestration.Workflow {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		got := getWorkflowForTest(t, server, runID, workflowID)
		if got.Status == want {
			return got
		}
		time.Sleep(25 * time.Millisecond)
	}
	got := getWorkflowForTest(t, server, runID, workflowID)
	t.Fatalf("workflow %s did not reach %s, got %+v", workflowID, want, got)
	return orchestration.Workflow{}
}

func assertWorkflowInspectionResponse(t *testing.T, workflow orchestration.Workflow, wantSteps int) {
	t.Helper()
	if workflow.Status != orchestration.WorkflowStatusPlanned {
		t.Fatalf("expected planned workflow, got %+v", workflow)
	}
	if len(workflow.Steps) != wantSteps {
		t.Fatalf("expected %d planned steps, got %+v", wantSteps, workflow.Steps)
	}
	for _, step := range workflow.Steps {
		if step.RuntimeStepID != "" || step.ActiveToolCallID != "" {
			t.Fatalf("expected inspect-only planning state, got %+v", step)
		}
		if step.SelectionRationale == "" || step.ApprovalModeExpected == "" {
			t.Fatalf("expected inspectable planning metadata, got %+v", step)
		}
	}
}

func assertWorkflowRuntimeLinkage(t *testing.T, workflow orchestration.Workflow) {
	t.Helper()
	for _, step := range workflow.Steps {
		if step.Status == orchestration.StepStatusReady || step.Status == orchestration.StepStatusWaitingDependency || step.Status == orchestration.StepStatusPlanned {
			continue
		}
		if step.RuntimeStepID == "" {
			t.Fatalf("expected runtime step linkage, got %+v", step)
		}
		if step.ActiveToolCallID == "" && step.Status != orchestration.StepStatusCancelled {
			t.Fatalf("expected tool call linkage, got %+v", step)
		}
	}
}

func assertInterruptedWorkflowStatus(t *testing.T, workflow orchestration.Workflow) {
	t.Helper()
	if workflow.Status != orchestration.WorkflowStatusInterrupted || workflow.InterruptedAt == nil {
		t.Fatalf("expected interrupted workflow truth, got %+v", workflow)
	}
	for _, step := range workflow.Steps {
		if step.Status == orchestration.StepStatusInterrupted {
			return
		}
	}
	t.Fatalf("expected at least one interrupted workflow step, got %+v", workflow.Steps)
}

func findNamedEvent(t *testing.T, items []events.Event, name string) events.Event {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("expected event %s in %+v", name, items)
	return events.Event{}
}
