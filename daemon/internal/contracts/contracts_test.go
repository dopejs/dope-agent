package contracts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

type contractHarness struct {
	validator  *contracts.Validator
	server     *api.Server
	store      *store.SQLiteStore
	authHeader string
}

func TestRequestSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/create-run.request.schema.json":                `{"entrypoint":"chat","goal":"ship daemon"}`,
		"schemas/api/create-step.request.schema.json":               `{"title":"plan","kind":"task","input":{"phase":"draft"}}`,
		"schemas/api/update-step-status.request.schema.json":        `{"status":"planning","output":{"phase":"ok"}}`,
		"schemas/api/create-tool-call.request.schema.json":          `{"capabilityId":"docs","toolName":"lookup","approvalId":"approval_1","input":{"query":"hello"}}`,
		"schemas/api/complete-tool-call.request.schema.json":        `{"output":{"ok":true}}`,
		"schemas/api/fail-tool-call.request.schema.json":            `{"error":"tool failed"}`,
		"schemas/api/create-connector.request.schema.json":          `{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`,
		"schemas/api/report-connector-health.request.schema.json":   `{"status":"healthy"}`,
		"schemas/api/report-connector-failure.request.schema.json":  `{"reason":"socket dropped"}`,
		"schemas/api/create-capability.request.schema.json":         `{"capabilityId":"docs","kind":"docs","displayName":"Docs"}`,
		"schemas/api/report-capability-health.request.schema.json":  `{"status":"degraded"}`,
		"schemas/api/report-capability-failure.request.schema.json": `{"reason":"worker exited"}`,
		"schemas/api/create-llm-dispatch.request.schema.json":       `{"provider":"echo","model":"echo-v1","messages":[{"role":"user","content":"hello"}],"timeoutMs":1000,"maxRetries":1}`,
		"schemas/api/chat-query.request.schema.json":                `{"provider":"echo","model":"echo-v1","query":"hello","timeoutMs":1000,"maxRetries":1}`,
		"schemas/api/request-approval.request.schema.json":          `{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"needs approval","requestedBy":"web-ui"}`,
		"schemas/api/resolve-approval.request.schema.json":          `{"resolution":"approved","comment":"allowed"}`,
		"schemas/api/start-pairing.request.schema.json":             `{"mode":"local","label":"web-ui","ttlSeconds":120}`,
		"schemas/api/complete-pairing.request.schema.json":          `{"code":"123456"}`,
	}

	for schemaPath, fixture := range fixtures {
		t.Run(filepath.Base(schemaPath), func(t *testing.T) {
			if err := validator.ValidateRelative(schemaPath, []byte(fixture)); err != nil {
				t.Fatalf("ValidateRelative returned error: %v", err)
			}
		})
	}
}

func TestValidatorRejectsInvalidRequestFixture(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	if err := validator.ValidateRelative("schemas/api/create-run.request.schema.json", []byte(`{"goal":"missing entrypoint"}`)); err == nil {
		t.Fatal("expected invalid create-run fixture to fail schema validation")
	}
}

func TestChatStreamSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/chat-query-stream-started.event.schema.json": `{"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","query":"hello"}`,
		"schemas/api/chat-query-stream-delta.event.schema.json":   `{"dispatchId":"dispatch_1","delta":"hello","reply":"hello"}`,
		"schemas/api/chat-query.response.schema.json":             `{"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","query":"hello","status":"completed","reply":"hello world","finishReason":"stop","usage":{"inputTokens":2,"outputTokens":3,"totalTokens":5}}`,
	}

	for schemaPath, fixture := range fixtures {
		t.Run(filepath.Base(schemaPath), func(t *testing.T) {
			if err := validator.ValidateRelative(schemaPath, []byte(fixture)); err != nil {
				t.Fatalf("ValidateRelative returned error: %v", err)
			}
		})
	}
}

func TestAPISchemasMatchCanonicalResponses(t *testing.T) {
	t.Parallel()

	h := newContractHarness(t)

	startPairingBody := h.request(t, http.MethodPost, "/v1/auth/pairings/start", `{"mode":"local","label":"contract-web"}`, "")
	h.mustValidate(t, "schemas/api/start-pairing.response.schema.json", startPairingBody)
	startPairing := decodeJSONMap(t, startPairingBody)
	pairing := startPairing["pairing"].(map[string]any)
	pairingID := pairing["pairingId"].(string)
	pairingCode := startPairing["pairingCode"].(string)

	completePairingBody := h.request(t, http.MethodPost, "/v1/auth/pairings/"+pairingID+"/complete", `{"code":"`+pairingCode+`"}`, "")
	h.mustValidate(t, "schemas/api/complete-pairing.response.schema.json", completePairingBody)
	completePairing := decodeJSONMap(t, completePairingBody)
	h.authHeader = "Bearer " + completePairing["accessToken"].(string)

	h.mustValidateResponse(t, http.MethodGet, "/v1/system/info", "", "", "schemas/api/system-info.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/config", "", h.authHeader, "schemas/api/config.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/auth/me", "", h.authHeader, "schemas/api/auth-access-token-resource.schema.json")

	h.mustValidateResponse(t, http.MethodPost, "/v1/connectors", `{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`, h.authHeader, "schemas/api/connector-resource.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/connectors", "", h.authHeader, "schemas/api/connector-list.response.schema.json")

	h.mustValidateResponse(t, http.MethodPost, "/v1/capabilities", `{"capabilityId":"docs","kind":"docs","displayName":"Docs"}`, h.authHeader, "schemas/api/capability-resource.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/capabilities", "", h.authHeader, "schemas/api/capability-list.response.schema.json")

	createRunBody := h.request(t, http.MethodPost, "/v1/runs", `{"entrypoint":"chat","goal":"validate contracts"}`, h.authHeader)
	h.mustValidate(t, "schemas/api/run-resource.schema.json", createRunBody)
	createRun := decodeJSONMap(t, createRunBody)
	runID := createRun["runId"].(string)
	sessionID := createRun["sessionId"].(string)

	h.mustValidateResponse(t, http.MethodGet, "/v1/runs", "", h.authHeader, "schemas/api/run-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/sessions", "", h.authHeader, "schemas/api/session-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/sessions/"+sessionID, "", h.authHeader, "schemas/api/session-resource.schema.json")

	createStepBody := h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps", `{"title":"plan contract suite","kind":"task"}`, h.authHeader)
	h.mustValidate(t, "schemas/api/step-resource.schema.json", createStepBody)
	stepID := decodeJSONMap(t, createStepBody)["stepId"].(string)

	h.mustValidateResponse(t, http.MethodGet, "/v1/runs/"+runID+"/steps", "", h.authHeader, "schemas/api/step-list.response.schema.json")

	createToolCallBody := h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/tool-calls", `{"capabilityId":"docs","toolName":"lookup","input":{"query":"contract"}}`, h.authHeader)
	h.mustValidate(t, "schemas/api/tool-call-resource.schema.json", createToolCallBody)
	toolCallID := decodeJSONMap(t, createToolCallBody)["toolCallId"].(string)

	h.mustValidateResponse(t, http.MethodGet, "/v1/runs/"+runID+"/steps/"+stepID+"/tool-calls", "", h.authHeader, "schemas/api/tool-call-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/tool-calls/"+toolCallID+"/complete", `{"output":{"ok":true}}`, h.authHeader, "schemas/api/tool-call-resource.schema.json")

	createApprovalBody := h.request(t, http.MethodPost, "/v1/policy/approvals", `{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"manual review","requestedBy":"contract-suite"}`, h.authHeader)
	h.mustValidate(t, "schemas/api/approval-decision.response.schema.json", createApprovalBody)
	approvalID := decodeJSONMap(t, createApprovalBody)["approval"].(map[string]any)["approvalId"].(string)
	h.mustValidateResponse(t, http.MethodGet, "/v1/policy/approvals", "", h.authHeader, "schemas/api/approval-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/policy/approvals/"+approvalID+"/resolve", `{"resolution":"approved","comment":"ok"}`, h.authHeader, "schemas/api/approval-decision.response.schema.json")

	createDispatchBody := h.request(t, http.MethodPost, "/v1/llm/dispatches", `{"provider":"echo","model":"echo-v1","messages":[{"role":"user","content":"hello"}]}`, h.authHeader)
	h.mustValidate(t, "schemas/api/llm-dispatch-resource.schema.json", createDispatchBody)
	h.mustValidateResponse(t, http.MethodGet, "/v1/llm/dispatches", "", h.authHeader, "schemas/api/llm-dispatch-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/chat/query", `{"provider":"echo","model":"echo-v1","query":"hello chat"}`, h.authHeader, "schemas/api/chat-query.response.schema.json")

	h.mustValidateResponse(t, http.MethodGet, "/v1/runs/"+runID+"/events", "", h.authHeader, "schemas/api/event-list.response.schema.json")
}

func TestEventSchemasMatchPersistedEvents(t *testing.T) {
	t.Parallel()

	h := newContractHarness(t)

	startPairingBody := h.request(t, http.MethodPost, "/v1/auth/pairings/start", `{"mode":"local","label":"events-web"}`, "")
	startPairing := decodeJSONMap(t, startPairingBody)
	pairing := startPairing["pairing"].(map[string]any)
	completePairingBody := h.request(t, http.MethodPost, "/v1/auth/pairings/"+pairing["pairingId"].(string)+"/complete", `{"code":"`+startPairing["pairingCode"].(string)+`"}`, "")
	h.authHeader = "Bearer " + decodeJSONMap(t, completePairingBody)["accessToken"].(string)

	h.request(t, http.MethodPost, "/v1/capabilities", `{"capabilityId":"docs","kind":"docs","displayName":"Docs"}`, h.authHeader)

	runBody := h.request(t, http.MethodPost, "/v1/runs", `{"entrypoint":"chat","goal":"validate events"}`, h.authHeader)
	runID := decodeJSONMap(t, runBody)["runId"].(string)
	stepBody := h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps", `{"title":"event step","kind":"task"}`, h.authHeader)
	stepID := decodeJSONMap(t, stepBody)["stepId"].(string)
	h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/status", `{"status":"planning"}`, h.authHeader)
	toolCallBody := h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/tool-calls", `{"capabilityId":"docs","toolName":"lookup","input":{"query":"events"}}`, h.authHeader)
	toolCallID := decodeJSONMap(t, toolCallBody)["toolCallId"].(string)
	h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/tool-calls/"+toolCallID+"/complete", `{"output":{"ok":true}}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/llm/dispatches", `{"provider":"echo","model":"echo-v1","messages":[{"role":"user","content":"stream"}]}`, h.authHeader)
	approvalBody := h.request(t, http.MethodPost, "/v1/policy/approvals", `{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"review","requestedBy":"events-suite"}`, h.authHeader)
	approvalID := decodeJSONMap(t, approvalBody)["approval"].(map[string]any)["approvalId"].(string)
	h.request(t, http.MethodPost, "/v1/policy/approvals/"+approvalID+"/resolve", `{"resolution":"approved","comment":"approved"}`, h.authHeader)

	items, err := h.store.ListEvents(context.Background(), events.Filter{})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	expectedSchemas := map[string]string{
		"run.created":               "schemas/events/run-created.event.schema.json",
		"step.created":              "schemas/events/step-created.event.schema.json",
		"step.status_changed":       "schemas/events/step-status-changed.event.schema.json",
		"run.status_changed":        "schemas/events/run-status-changed.event.schema.json",
		"tool_call.requested":       "schemas/events/tool-call-requested.event.schema.json",
		"tool_call.completed":       "schemas/events/tool-call-completed.event.schema.json",
		"llm.dispatch.requested":    "schemas/events/llm-dispatch-requested.event.schema.json",
		"llm.dispatch.completed":    "schemas/events/llm-dispatch-completed.event.schema.json",
		"policy.approval_requested": "schemas/events/policy-approval-requested.event.schema.json",
		"policy.approval_resolved":  "schemas/events/policy-approval-resolved.event.schema.json",
		"policy.decision_recorded":  "schemas/events/policy-decision-recorded.event.schema.json",
	}

	found := make(map[string]bool, len(expectedSchemas))
	for _, item := range items {
		schemaPath, ok := expectedSchemas[item.Name]
		if !ok {
			continue
		}
		payload, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("json.Marshal event %s returned error: %v", item.Name, err)
		}
		if err := h.validator.ValidateRelative(schemaPath, payload); err != nil {
			t.Fatalf("ValidateRelative(%s) returned error: %v", schemaPath, err)
		}
		found[item.Name] = true
	}

	for eventName := range expectedSchemas {
		if !found[eventName] {
			t.Fatalf("expected event %s to be emitted and validated", eventName)
		}
	}
}

func newContractHarness(t *testing.T) *contractHarness {
	t.Helper()

	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	eventBus := events.NewBus()
	t.Cleanup(eventBus.Close)

	sessionRouter := router.NewSessionRouter()
	runtimeManager := runtime.NewManager()
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	llmDispatcher := llm.NewDispatcher()
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	t.Cleanup(func() {
		if err := checkpointManager.Close(); err != nil {
			t.Fatalf("Close checkpoint manager returned error: %v", err)
		}
	})

	server := api.NewServer(api.Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:18789",
			DataDir:  "~/.dope",
			LogLevel: "info",
			Version:  "test",
		},
		Logger:       telemetry.New("error").Slog(),
		EventBus:     eventBus,
		Policy:       policyEngine,
		Auth:         authManager,
		Router:       sessionRouter,
		Runtime:      runtimeManager,
		LLM:          llmDispatcher,
		Connectors:   connectorSupervisor,
		Capabilities: capabilitySupervisor,
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
	})

	return &contractHarness{
		validator: contracts.NewValidator(schemaRootDir(t)),
		server:    server,
		store:     sqliteStore,
	}
}

func (h *contractHarness) mustValidateResponse(t *testing.T, method, path, body, authHeader, schemaPath string) {
	t.Helper()
	responseBody := h.request(t, method, path, body, authHeader)
	h.mustValidate(t, schemaPath, responseBody)
}

func (h *contractHarness) mustValidate(t *testing.T, schemaPath string, body []byte) {
	t.Helper()
	if err := h.validator.ValidateRelative(schemaPath, body); err != nil {
		t.Fatalf("ValidateRelative(%s) returned error: %v\nbody=%s", schemaPath, err, string(body))
	}
}

func (h *contractHarness) request(t *testing.T, method, path, body, authHeader string) []byte {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	rec := httptest.NewRecorder()
	h.server.Handler().ServeHTTP(rec, req)
	if rec.Code < http.StatusOK || rec.Code >= http.StatusMultipleChoices {
		t.Fatalf("%s %s returned status %d body=%s", method, path, rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func decodeJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v\nbody=%s", err, string(body))
	}
	return value
}

func schemaRootDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned no file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
}
