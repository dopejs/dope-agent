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
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
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

type contractManagedRegistry struct {
	bridges []providers.ManagedBridge
}

func (r contractManagedRegistry) List() []providers.ManagedBridge {
	return append([]providers.ManagedBridge(nil), r.bridges...)
}

func (r contractManagedRegistry) Get(providerID string) (providers.ManagedBridge, bool) {
	for _, bridge := range r.bridges {
		if bridge.ProviderID() == providerID {
			return bridge, true
		}
	}
	return nil, false
}

type contractManagedBridge struct {
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

func (b contractManagedBridge) ProviderID() string           { return b.providerID }
func (b contractManagedBridge) DisplayName() string          { return b.displayName }
func (b contractManagedBridge) Family() providers.Family     { return b.family }
func (b contractManagedBridge) AuthMode() providers.AuthMode { return b.authMode }
func (b contractManagedBridge) Provider() llm.Provider       { return b.provider }
func (b contractManagedBridge) Detect(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.detectState, cloneContractProviderModels(b.models), nil
}
func (b contractManagedBridge) Start(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.startState, cloneContractProviderModels(b.models), nil
}
func (b contractManagedBridge) Complete(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.completeState, cloneContractProviderModels(b.models), nil
}
func (b contractManagedBridge) Refresh(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.refreshState, cloneContractProviderModels(b.models), nil
}
func (b contractManagedBridge) Revoke(context.Context) (providers.AuthState, []providers.Model, error) {
	return b.revokeState, cloneContractProviderModels(b.models), nil
}

func cloneContractProviderModels(items []providers.Model) []providers.Model {
	cloned := make([]providers.Model, 0, len(items))
	for _, item := range items {
		model := item
		model.ReasoningLevels = append([]string(nil), item.ReasoningLevels...)
		cloned = append(cloned, model)
	}
	return cloned
}

type contractManagedLLMProvider struct {
	name string
}

func (p *contractManagedLLMProvider) Name() string { return p.name }

func (p *contractManagedLLMProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       request.Model,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *contractManagedLLMProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	if emit != nil {
		if err := emit(llm.StreamChunk{Delta: request.Model, Output: request.Model}); err != nil {
			return llm.ProviderResponse{}, err
		}
	}
	return llm.ProviderResponse{
		Output:       request.Model,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func ptrContractTime(value time.Time) *time.Time {
	return &value
}

func TestRequestSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/create-run.request.schema.json":                `{"entrypoint":"chat","goal":"ship daemon"}`,
		"schemas/api/connector-ingress-message.request.schema.json": `{"route":{"kind":"group","accountId":"bot-main","peerId":"channel-1","threadId":"thread-1"},"message":{"messageId":"msg_1","text":"hello"},"run":{"entrypoint":"connector.message","goal":"handle inbound"}}`,
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
		"schemas/api/run-provider-check.request.schema.json":        `{"model":"echo-v1","prompt":"hello"}`,
		"schemas/api/provider-default-model.request.schema.json":    `{"model":"gpt-5.4"}`,
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

	if err := validator.ValidateRelative("schemas/api/create-run.request.schema.json", []byte(`{"entrypoint":"chat","route":{"kind":"direct","channel":"telegram","accountId":"bot-main","peerId":"dm-1"}}`)); err != nil {
		t.Fatalf("ValidateRelative(create-run route fixture) returned error: %v", err)
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
		"schemas/api/chat-query.response.schema.json":             `{"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","query":"hello","status":"completed","partial":false,"reply":"hello world","finishReason":"stop","usage":{"inputTokens":2,"outputTokens":3,"totalTokens":5}}`,
	}

	for schemaPath, fixture := range fixtures {
		t.Run(filepath.Base(schemaPath), func(t *testing.T) {
			if err := validator.ValidateRelative(schemaPath, []byte(fixture)); err != nil {
				t.Fatalf("ValidateRelative returned error: %v", err)
			}
		})
	}
}

func TestProviderSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/provider-resource.schema.json":                       `{"providerId":"echo","title":"Echo","family":"builtin_echo","authMode":"none","source":"builtin","modelSelectionMode":"fixed","knownModels":["echo-v1"],"registered":true,"configured":true,"ready":true,"default":true,"defaultModel":"echo-v1","effectiveModel":"echo-v1","effectiveTimeoutMs":30000,"effectiveMaxRetries":0,"secretConfigured":false,"capabilities":{"chat":true,"stream":true}}`,
		"schemas/api/provider-check-resource.schema.json":                 `{"checkId":"provider_check_1","providerId":"echo","family":"builtin_echo","authMode":"none","status":"passed","model":"echo-v1","usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2},"createdAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:01Z"}`,
		"schemas/api/provider-auth-state.response.schema.json":            `{"auth":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"authenticated","cliAvailable":true,"accountLabel":"user@example.com","accountId":"acct_1","plan":"pro","authMethod":"chatgpt","loginCommand":["codex","login"],"logoutCommand":["codex","logout"],"lastCheckedAt":"2026-04-18T12:00:00Z","lastAuthenticatedAt":"2026-04-18T11:59:00Z","metadata":{"source":"contract"}}}`,
		"schemas/api/provider-model.schema.json":                          `{"providerId":"codex_managed","modelId":"gpt-5.4","displayName":"GPT-5.4","description":"Primary coding model","default":true,"available":true,"source":"cache","chat":true,"stream":true,"coding":true,"toolUse":false,"reasoningLevels":["medium","high"]}`,
		"schemas/api/provider-model-list.response.schema.json":            `{"items":[{"providerId":"codex_managed","modelId":"gpt-5.4","displayName":"GPT-5.4","default":true,"available":true,"source":"cache","chat":true,"stream":true,"coding":true,"toolUse":false}]}`,
		"schemas/api/provider-default-model.response.schema.json":         `{"providerId":"codex_managed","defaultModel":"gpt-5.4","updatedAt":"2026-04-18T12:00:00Z"}`,
		"schemas/events/provider-check-completed.event.schema.json":       `{"eventId":"evt_1","sequence":1,"category":"provider","name":"provider.check_completed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_check","id":"provider_check_1"},"payload":{"providerId":"echo","family":"builtin_echo","authMode":"none","status":"passed","model":"echo-v1","endpoint":"","usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2}}}`,
		"schemas/events/provider-check-failed.event.schema.json":          `{"eventId":"evt_2","sequence":2,"category":"provider","name":"provider.check_failed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_check","id":"provider_check_2"},"payload":{"providerId":"openai_compatible","family":"openai_compatible","authMode":"api_key","status":"failed","model":"gpt-5.4","errorClass":"auth_error","errorCode":"upstream_auth_failed","errorMessage":"unauthorized","usage":{"inputTokens":0,"outputTokens":0,"totalTokens":0}}}`,
		"schemas/events/provider-auth-started.event.schema.json":          `{"eventId":"evt_3","sequence":3,"category":"provider","name":"provider.auth_started","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"pending_login","cliAvailable":true,"accountLabel":"","accountId":"","plan":"","authMethod":"","lastError":""}}`,
		"schemas/events/provider-auth-completed.event.schema.json":        `{"eventId":"evt_4","sequence":4,"category":"provider","name":"provider.auth_completed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"authenticated","cliAvailable":true,"accountLabel":"user@example.com","accountId":"acct_1","plan":"pro","authMethod":"chatgpt","lastError":""}}`,
		"schemas/events/provider-auth-refreshed.event.schema.json":        `{"eventId":"evt_5","sequence":5,"category":"provider","name":"provider.auth_refreshed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"authenticated","cliAvailable":true,"accountLabel":"user@example.com","accountId":"acct_1","plan":"pro","authMethod":"chatgpt","lastError":""}}`,
		"schemas/events/provider-auth-revoked.event.schema.json":          `{"eventId":"evt_6","sequence":6,"category":"provider","name":"provider.auth_revoked","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"revoked","cliAvailable":true,"accountLabel":"","accountId":"","plan":"","authMethod":"","lastError":""}}`,
		"schemas/events/provider-default-model-updated.event.schema.json": `{"eventId":"evt_7","sequence":7,"category":"provider","name":"provider.default_model_updated","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider","id":"codex_managed"},"payload":{"providerId":"codex_managed","defaultModel":"gpt-5.4","updatedAt":"2026-04-18T12:00:01Z"}}`,
	}

	for schemaPath, fixture := range fixtures {
		t.Run(filepath.Base(schemaPath), func(t *testing.T) {
			if err := validator.ValidateRelative(schemaPath, []byte(fixture)); err != nil {
				t.Fatalf("ValidateRelative returned error: %v", err)
			}
		})
	}
}

func TestStreamingTimeoutSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/events/llm-dispatch-partial-failed.event.schema.json": `{"eventId":"evt_partial_1","sequence":8,"category":"llm","name":"llm.dispatch.partial_failed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"llm_dispatch","id":"dispatch_1"},"payload":{"provider":"openai_compatible","model":"gpt-5.4","status":"partial_failed","partial":true,"attemptCount":1,"finishReason":"","usage":{"inputTokens":1,"outputTokens":2,"totalTokens":3},"errorCode":"idle_timeout","error":"stream stalled"}}`,
		"schemas/events/connector-reply-partial.event.schema.json":     `{"eventId":"evt_partial_2","sequence":9,"category":"connector","name":"connector.reply_partial","occurredAt":"2026-04-18T12:00:01Z","scope":{"runId":"run_1","stepId":"step_1","connectorId":"discord-main"},"resource":{"kind":"connector","id":"discord-main"},"payload":{"messageId":"msg_1","replyMessageId":"reply_1","replyMessageIds":["reply_1"],"partCount":1,"contentLength":128,"error":"stream stalled","errorClass":""}}`,
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
	h.mustValidateResponse(t, http.MethodGet, "/v1/providers", "", h.authHeader, "schemas/api/provider-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/providers/echo", "", h.authHeader, "schemas/api/provider-resource.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/providers/echo/checks", `{"model":"echo-v1","prompt":"hello"}`, h.authHeader, "schemas/api/provider-check-resource.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/providers/echo/checks", "", h.authHeader, "schemas/api/provider-check-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/providers/codex_managed", "", h.authHeader, "schemas/api/provider-resource.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/providers/codex_managed/auth", "", h.authHeader, "schemas/api/provider-auth-state.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/providers/codex_managed/auth/start", `{}`, h.authHeader, "schemas/api/provider-auth-state.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/providers/codex_managed/auth/complete", `{}`, h.authHeader, "schemas/api/provider-auth-state.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/providers/codex_managed/auth/refresh", `{}`, h.authHeader, "schemas/api/provider-auth-state.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/providers/codex_managed/models", "", h.authHeader, "schemas/api/provider-model-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/providers/codex_managed/default-model", `{"model":"gpt-5.4-mini"}`, h.authHeader, "schemas/api/provider-default-model.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/providers/codex_managed/auth/revoke", `{}`, h.authHeader, "schemas/api/provider-auth-state.response.schema.json")

	h.mustValidateResponse(t, http.MethodPost, "/v1/connectors", `{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`, h.authHeader, "schemas/api/connector-resource.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/connectors", "", h.authHeader, "schemas/api/connector-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/connectors/telegram-main/ingress/messages", `{"route":{"kind":"direct","accountId":"bot-main","peerId":"dm-1"},"message":{"messageId":"msg_1","text":"hello"},"run":{"entrypoint":"connector.message","goal":"contract ingress"}}`, h.authHeader, "schemas/api/connector-ingress-message.response.schema.json")

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
	h.request(t, http.MethodPost, "/v1/connectors", `{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/connectors/telegram-main/ingress/messages", `{"route":{"kind":"direct","accountId":"bot-main","peerId":"dm-1"},"message":{"messageId":"msg_1","text":"hello"},"run":{"entrypoint":"connector.message","goal":"validate ingress events"}}`, h.authHeader)

	runBody := h.request(t, http.MethodPost, "/v1/runs", `{"entrypoint":"chat","goal":"validate events"}`, h.authHeader)
	runID := decodeJSONMap(t, runBody)["runId"].(string)
	stepBody := h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps", `{"title":"event step","kind":"task"}`, h.authHeader)
	stepID := decodeJSONMap(t, stepBody)["stepId"].(string)
	h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/status", `{"status":"planning"}`, h.authHeader)
	toolCallBody := h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/tool-calls", `{"capabilityId":"docs","toolName":"lookup","input":{"query":"events"}}`, h.authHeader)
	toolCallID := decodeJSONMap(t, toolCallBody)["toolCallId"].(string)
	h.request(t, http.MethodPost, "/v1/runs/"+runID+"/steps/"+stepID+"/tool-calls/"+toolCallID+"/complete", `{"output":{"ok":true}}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/llm/dispatches", `{"provider":"echo","model":"echo-v1","messages":[{"role":"user","content":"stream"}]}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/providers/echo/checks", `{"model":"echo-v1","prompt":"check"}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/providers/codex_managed/auth/start", `{}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/providers/codex_managed/auth/complete", `{}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/providers/codex_managed/auth/refresh", `{}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/providers/codex_managed/default-model", `{"model":"gpt-5.4-mini"}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/providers/codex_managed/auth/revoke", `{}`, h.authHeader)
	approvalBody := h.request(t, http.MethodPost, "/v1/policy/approvals", `{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"review","requestedBy":"events-suite"}`, h.authHeader)
	approvalID := decodeJSONMap(t, approvalBody)["approval"].(map[string]any)["approvalId"].(string)
	h.request(t, http.MethodPost, "/v1/policy/approvals/"+approvalID+"/resolve", `{"resolution":"approved","comment":"approved"}`, h.authHeader)

	items, err := h.store.ListEvents(context.Background(), events.Filter{})
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}

	expectedSchemas := map[string]string{
		"run.created":                    "schemas/events/run-created.event.schema.json",
		"step.created":                   "schemas/events/step-created.event.schema.json",
		"step.status_changed":            "schemas/events/step-status-changed.event.schema.json",
		"run.status_changed":             "schemas/events/run-status-changed.event.schema.json",
		"tool_call.requested":            "schemas/events/tool-call-requested.event.schema.json",
		"tool_call.completed":            "schemas/events/tool-call-completed.event.schema.json",
		"llm.dispatch.requested":         "schemas/events/llm-dispatch-requested.event.schema.json",
		"llm.dispatch.completed":         "schemas/events/llm-dispatch-completed.event.schema.json",
		"provider.check_completed":       "schemas/events/provider-check-completed.event.schema.json",
		"provider.auth_started":          "schemas/events/provider-auth-started.event.schema.json",
		"provider.auth_completed":        "schemas/events/provider-auth-completed.event.schema.json",
		"provider.auth_refreshed":        "schemas/events/provider-auth-refreshed.event.schema.json",
		"provider.auth_revoked":          "schemas/events/provider-auth-revoked.event.schema.json",
		"provider.default_model_updated": "schemas/events/provider-default-model-updated.event.schema.json",
		"connector.ingress_accepted":     "schemas/events/connector-ingress-accepted.event.schema.json",
		"policy.approval_requested":      "schemas/events/policy-approval-requested.event.schema.json",
		"policy.approval_resolved":       "schemas/events/policy-approval-resolved.event.schema.json",
		"policy.decision_recorded":       "schemas/events/policy-decision-recorded.event.schema.json",
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
	llmDispatcher.RegisterProvider(&contractManagedLLMProvider{name: "codex_managed"})
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	managedBridge := contractManagedBridge{
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
			Metadata:      map[string]string{"source": "contract"},
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
			Metadata:      map[string]string{"source": "contract"},
		},
		completeState: providers.AuthState{
			ProviderID:          "codex_managed",
			Family:              providers.FamilyCodexCLI,
			AuthMode:            providers.AuthModeLocalCLIBridge,
			Status:              providers.AuthStatusAuthenticated,
			CLIPath:             "/usr/bin/codex",
			CLIAvailable:        true,
			AccountLabel:        "user@example.com",
			AccountID:           "acct_1",
			Plan:                "pro",
			AuthMethod:          "chatgpt",
			LoginCommand:        []string{"codex", "login"},
			LogoutCommand:       []string{"codex", "logout"},
			LastCheckedAt:       now,
			LastAuthenticatedAt: ptrContractTime(now.Add(-time.Minute)),
			Metadata:            map[string]string{"source": "contract"},
		},
		refreshState: providers.AuthState{
			ProviderID:          "codex_managed",
			Family:              providers.FamilyCodexCLI,
			AuthMode:            providers.AuthModeLocalCLIBridge,
			Status:              providers.AuthStatusAuthenticated,
			CLIPath:             "/usr/bin/codex",
			CLIAvailable:        true,
			AccountLabel:        "user@example.com",
			AccountID:           "acct_1",
			Plan:                "pro",
			AuthMethod:          "chatgpt",
			LoginCommand:        []string{"codex", "login"},
			LogoutCommand:       []string{"codex", "logout"},
			LastCheckedAt:       now.Add(time.Minute),
			LastAuthenticatedAt: ptrContractTime(now.Add(-time.Minute)),
			Metadata:            map[string]string{"source": "contract"},
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
			Metadata:      map[string]string{"source": "contract"},
		},
		models: []providers.Model{
			{ProviderID: "codex_managed", ModelID: "gpt-5.4", DisplayName: "GPT-5.4", Description: "Primary coding model", Default: true, Available: true, Source: "cache", Chat: true, Stream: true, Coding: true, ToolUse: false, ReasoningLevels: []string{"medium", "high"}},
			{ProviderID: "codex_managed", ModelID: "gpt-5.4-mini", DisplayName: "GPT-5.4 mini", Available: true, Source: "cache", Chat: true, Stream: true, Coding: true, ToolUse: false},
		},
		provider: &contractManagedLLMProvider{name: "codex_managed"},
	}
	providerManager := providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
		},
	}, llmDispatcher, contractManagedRegistry{bridges: []providers.ManagedBridge{managedBridge}})
	providerManager.RestoreManagedAuthStates([]providers.AuthState{managedBridge.detectState})
	providerManager.RestoreProviderModels(managedBridge.models)
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	chatService := chat.NewService(llmDispatcher, providerManager, eventBus, sqliteStore)
	t.Cleanup(func() {
		if err := checkpointManager.Close(); err != nil {
			t.Fatalf("Close checkpoint manager returned error: %v", err)
		}
	})

	server := api.NewServer(api.Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
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
		Chat:         chatService,
		Providers:    providerManager,
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
