package contracts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
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
		"schemas/api/chat-query.request.schema.json":                `{"provider":"echo","model":"echo-v1","skills":["shared"],"query":"hello","timeoutMs":1000,"maxRetries":1}`,
		"schemas/api/run-provider-check.request.schema.json":        `{"model":"echo-v1","prompt":"hello"}`,
		"schemas/api/provider-default-model.request.schema.json":    `{"model":"gpt-5.4"}`,
		"schemas/api/request-approval.request.schema.json":          `{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"needs approval","requestedBy":"web-ui"}`,
		"schemas/api/resolve-approval.request.schema.json":          `{"resolution":"approved","comment":"allowed"}`,
		"schemas/api/sandbox-execution.request.schema.json":         `{"profileId":"subprocess_default","command":"echo","args":["hello"],"cwd":"/tmp/dope","timeoutMs":1000,"requestedBy":"web-ui","resourceKind":"skill","resourceId":"shared","scope":"chat","reason":"inspect profile","metadata":{"ticket":"sandbox-16"},"access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"networkMode":"allow_list","allowedHosts":["localhost"],"allowedPorts":[80],"allowLoopback":true}}`,
		"schemas/api/sandbox-explain.request.schema.json":           `{"profileId":"subprocess_default","command":"echo","args":["hello"],"cwd":"/tmp/dope","access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"allowedHosts":[],"allowedPorts":[]}}`,
		"schemas/api/mcp-server-create.request.schema.json":         `{"serverId":"mcp-test","displayName":"MCP Test","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:mcp-test:lifecycle.start","transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","secretRefs":["MCP_TEST_TOKEN"],"autoRestart":true}`,
		"schemas/api/mcp-server-update.request.schema.json":         `{"displayName":"Updated MCP","enabled":false,"autoRestart":false}`,
		"schemas/api/mcp-catalog-install-request.schema.json":       `{"serverId":"filesystem-test","displayName":"Filesystem Test","workingDir":"/tmp/dope"}`,
		"schemas/api/mcp-tool-exposure-update.request.schema.json":  `{"runtimeSurface":"chat","exposureMode":"approval_required","active":true,"reason":"needs approval"}`,
		"schemas/api/mcp-tool-authorization.request.schema.json":    `{"runtimeSurface":"chat","approvalId":"approval_1","requestedBy":"web-ui"}`,
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
		"schemas/api/chat-query-stream-started.event.schema.json": `{"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","skills":["shared"],"skillContracts":[{"declaration":{"declarationId":"skill:shared:selection","consumerKind":"skill","consumerId":"shared","operationKind":"skill_selection","profileId":"subprocess_default","executionMode":"declaration_only","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/shared"],"writeRoots":[],"networkMode":"deny","secretRefs":[],"approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"}}],"query":"hello"}`,
		"schemas/api/chat-query-stream-delta.event.schema.json":   `{"dispatchId":"dispatch_1","delta":"hello","reply":"hello"}`,
		"schemas/api/chat-query.response.schema.json":             `{"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","skills":["shared"],"skillContracts":[{"declaration":{"declarationId":"skill:shared:selection","consumerKind":"skill","consumerId":"shared","operationKind":"skill_selection","profileId":"subprocess_default","executionMode":"declaration_only","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/shared"],"writeRoots":[],"networkMode":"deny","secretRefs":[],"approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"}}],"query":"hello","status":"completed","partial":false,"reply":"hello world","finishReason":"stop","usage":{"inputTokens":2,"outputTokens":3,"totalTokens":5}}`,
	}

	assertFixtureSandboxDeclaration(t, fixtures["schemas/api/chat-query.response.schema.json"], "skill", "shared", "skill_selection")
	mustValidateFixtures(t, validator, fixtures)
}

func TestPolicySchemasAcceptSandboxFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/approval-resource.schema.json":                  `{"approvalId":"approval_1","action":"tool_call.execute","resourceKind":"capability","resourceId":"shell","reason":"need approval","status":"pending","createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","sandbox":{"declaration":{"declarationId":"local_tool:shell:tool_call.execute","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"access_only","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":[],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_local_tool_shell_1","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","declarationId":"local_tool:shell:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_1","decision":"ask","approvalStatus":"pending","secretResolution":"not_applicable","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}}`,
		"schemas/api/decision-resource.schema.json":                  `{"decisionId":"decision_1","action":"tool_call.execute","resourceKind":"capability","resourceId":"shell","outcome":"requires_approval","reason":"need approval","approvalId":"approval_1","createdAt":"2026-04-18T12:00:00Z","sandbox":{"declaration":{"declarationId":"local_tool:shell:tool_call.execute","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"access_only","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":[],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_local_tool_shell_1","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","declarationId":"local_tool:shell:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_1","decision":"ask","approvalStatus":"pending","secretResolution":"not_applicable","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}}`,
		"schemas/events/policy-approval-requested.event.schema.json": `{"eventId":"evt_policy_1","sequence":12,"category":"policy","name":"policy.approval_requested","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"approval","id":"approval_1"},"payload":{"action":"tool_call.execute","resourceKind":"capability","resourceId":"shell","status":"pending","sandbox":{"declaration":{"declarationId":"local_tool:shell:tool_call.execute","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"access_only","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":[],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_local_tool_shell_1","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","declarationId":"local_tool:shell:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_1","decision":"ask","approvalStatus":"pending","secretResolution":"not_applicable","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}}}`,
		"schemas/events/policy-approval-resolved.event.schema.json":  `{"eventId":"evt_policy_2","sequence":13,"category":"policy","name":"policy.approval_resolved","occurredAt":"2026-04-18T12:00:02Z","scope":{},"resource":{"kind":"approval","id":"approval_1"},"payload":{"action":"tool_call.execute","resourceKind":"capability","resourceId":"shell","status":"approved","resolution":"approved","sandbox":{"declaration":{"declarationId":"local_tool:shell:tool_call.execute","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"access_only","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":[],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_local_tool_shell_1","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","declarationId":"local_tool:shell:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_2","decision":"allow","approvalStatus":"approved","secretResolution":"not_applicable","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:02Z","status":"preflight_allowed"}}}}`,
		"schemas/events/policy-decision-recorded.event.schema.json":  `{"eventId":"evt_policy_3","sequence":14,"category":"policy","name":"policy.decision_recorded","occurredAt":"2026-04-18T12:00:02Z","scope":{},"resource":{"kind":"decision","id":"decision_2"},"payload":{"action":"tool_call.execute","resourceKind":"capability","resourceId":"shell","outcome":"approved","approvalId":"approval_1","sandbox":{"declaration":{"declarationId":"local_tool:shell:tool_call.execute","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"access_only","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":[],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_local_tool_shell_1","consumerKind":"local_tool","consumerId":"shell","operationKind":"tool_call.execute","declarationId":"local_tool:shell:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_2","decision":"allow","approvalStatus":"approved","secretResolution":"not_applicable","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:02Z","status":"preflight_allowed"}}}}`,
	}

	assertFixtureSandboxDeclaration(t, fixtures["schemas/api/approval-resource.schema.json"], "local_tool", "shell", "tool_call.execute")
	assertFixtureSandboxPolicyRecord(t, fixtures["schemas/api/approval-resource.schema.json"], "approval_pending", "not_applicable")
	mustValidateFixtures(t, validator, fixtures)
}

func TestProviderSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/provider-resource.schema.json":                       `{"providerId":"echo","title":"Echo","family":"builtin_echo","authMode":"none","source":"builtin","modelSelectionMode":"fixed","knownModels":["echo-v1"],"registered":true,"configured":true,"ready":true,"default":true,"defaultModel":"echo-v1","effectiveModel":"echo-v1","effectiveTimeoutMs":30000,"effectiveMaxRetries":0,"secretConfigured":false,"capabilities":{"chat":true,"stream":true}}`,
		"schemas/api/provider-check-resource.schema.json":                 `{"checkId":"provider_check_1","providerId":"echo","family":"builtin_echo","authMode":"none","status":"passed","model":"echo-v1","usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2},"createdAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:01Z"}`,
		"schemas/api/provider-auth-state.response.schema.json":            `{"auth":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"authenticated","cliAvailable":true,"accountLabel":"user@example.com","accountId":"acct_1","plan":"pro","authMethod":"chatgpt","loginCommand":["codex","login"],"logoutCommand":["codex","logout"],"lastCheckedAt":"2026-04-18T12:00:00Z","lastAuthenticatedAt":"2026-04-18T11:59:00Z","metadata":{"source":"contract","managedProviderId":"codex_managed","managedProviderAction":"auth_status","sandboxProfileId":"managed_provider_codex","sandboxDecision":"allow","enforcementStrength":"declared_only"}}}`,
		"schemas/api/provider-model.schema.json":                          `{"providerId":"codex_managed","modelId":"gpt-5.4","displayName":"GPT-5.4","description":"Primary coding model","default":true,"available":true,"source":"cache","chat":true,"stream":true,"coding":true,"toolUse":false,"reasoningLevels":["medium","high"]}`,
		"schemas/api/provider-model-list.response.schema.json":            `{"items":[{"providerId":"codex_managed","modelId":"gpt-5.4","displayName":"GPT-5.4","default":true,"available":true,"source":"cache","chat":true,"stream":true,"coding":true,"toolUse":false}]}`,
		"schemas/api/provider-default-model.response.schema.json":         `{"providerId":"codex_managed","defaultModel":"gpt-5.4","updatedAt":"2026-04-18T12:00:00Z"}`,
		"schemas/events/provider-check-completed.event.schema.json":       `{"eventId":"evt_1","sequence":1,"category":"provider","name":"provider.check_completed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_check","id":"provider_check_1"},"payload":{"providerId":"echo","family":"builtin_echo","authMode":"none","status":"passed","model":"echo-v1","endpoint":"","usage":{"inputTokens":1,"outputTokens":1,"totalTokens":2}}}`,
		"schemas/events/provider-check-failed.event.schema.json":          `{"eventId":"evt_2","sequence":2,"category":"provider","name":"provider.check_failed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_check","id":"provider_check_2"},"payload":{"providerId":"openai_compatible","family":"openai_compatible","authMode":"api_key","status":"failed","model":"gpt-5.4","errorClass":"auth_error","errorCode":"upstream_auth_failed","errorMessage":"unauthorized","usage":{"inputTokens":0,"outputTokens":0,"totalTokens":0}}}`,
		"schemas/events/provider-auth-started.event.schema.json":          `{"eventId":"evt_3","sequence":3,"category":"provider","name":"provider.auth_started","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"pending_login","cliAvailable":true,"accountLabel":"","accountId":"","plan":"","authMethod":"","lastError":"","metadata":{"source":"contract","managedProviderId":"codex_managed","managedProviderAction":"auth_status","sandboxProfileId":"managed_provider_codex","sandboxDecision":"allow","enforcementStrength":"declared_only"}}}`,
		"schemas/events/provider-auth-completed.event.schema.json":        `{"eventId":"evt_4","sequence":4,"category":"provider","name":"provider.auth_completed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"authenticated","cliAvailable":true,"accountLabel":"user@example.com","accountId":"acct_1","plan":"pro","authMethod":"chatgpt","lastError":"","metadata":{"source":"contract","managedProviderId":"codex_managed","managedProviderAction":"auth_status","sandboxProfileId":"managed_provider_codex","sandboxDecision":"allow","enforcementStrength":"declared_only"}}}`,
		"schemas/events/provider-auth-refreshed.event.schema.json":        `{"eventId":"evt_5","sequence":5,"category":"provider","name":"provider.auth_refreshed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"authenticated","cliAvailable":true,"accountLabel":"user@example.com","accountId":"acct_1","plan":"pro","authMethod":"chatgpt","lastError":"","metadata":{"source":"contract","managedProviderId":"codex_managed","managedProviderAction":"auth_status","sandboxProfileId":"managed_provider_codex","sandboxDecision":"allow","enforcementStrength":"declared_only"}}}`,
		"schemas/events/provider-auth-revoked.event.schema.json":          `{"eventId":"evt_6","sequence":6,"category":"provider","name":"provider.auth_revoked","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"provider_auth","id":"codex_managed"},"payload":{"providerId":"codex_managed","family":"codex_cli","authMode":"local_cli_bridge","status":"revoked","cliAvailable":true,"accountLabel":"","accountId":"","plan":"","authMethod":"","lastError":"","metadata":{"source":"contract","managedProviderId":"codex_managed","managedProviderAction":"logout","sandboxProfileId":"managed_provider_codex","sandboxDecision":"allow","enforcementStrength":"declared_only"}}}`,
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

func TestMCPSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/mcp-server-resource.schema.json":                         `{"serverId":"mcp-test","displayName":"MCP Test","source":"api","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:current","driftStatus":"catalog_updated","driftReason":"installed server no longer matches the current catalog revision","installedAt":"2026-04-18T12:00:00Z","lastMaintainedAt":"2026-04-18T12:05:00Z","lastActionAt":"2026-04-18T12:06:00Z","lastAction":"revalidate","lastActionStatus":"completed","lastActionReason":"installed server no longer matches the current catalog revision","installInputSnapshot":{"serverId":"mcp-test","displayName":"MCP Test","enabled":true,"sandboxProfileId":"subprocess_default","secretRefs":["MCP_TEST_TOKEN"],"installMethod":"api"},"lastRevalidation":{"checkedAt":"2026-04-18T12:06:00Z","status":"ready","classification":"catalog_drift","reason":"installed server no longer matches the current catalog revision","issues":[{"kind":"catalog","name":"filesystem","status":"warning","reason":"installed server no longer matches the current catalog revision","environmentScope":"test"}]}},"enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:mcp-test:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","secretRefs":["MCP_TEST_TOKEN"],"autoRestart":true,"createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","state":{"serverId":"mcp-test","status":"healthy","failureCount":0,"restartCount":0,"lastStartedAt":"2026-04-18T12:00:01Z","lastHeartbeatAt":"2026-04-18T12:00:02Z","lastExecutionId":"sandbox_exec_1","lastPolicyRecordId":"policy_mcp_1","updatedAt":"2026-04-18T12:00:02Z"},"secretSummary":[{"consumerId":"mcp-test","secretRef":"MCP_TEST_TOKEN","environmentScope":"test","defaultRuleId":"mcp_server:mcp-test","resolution":"resolved","deliveryKind":"environment_variable","redactionRule":"value_redacted"}],"toolCount":1,"tools":[{"serverId":"mcp-test","toolName":"lookup","title":"Lookup","description":"Lookup tool","schemaFingerprint":"abc123","discoveryStatus":"discovered","lastDiscoveredAt":"2026-04-18T12:00:02Z","updatedAt":"2026-04-18T12:00:02Z","effectiveAvailability":"blocked","approvalRequired":false}]}`,
		"schemas/api/mcp-server-list.response.schema.json":                    `{"items":[{"serverId":"mcp-test","displayName":"MCP Test","source":"api","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:current","driftStatus":"catalog_updated","driftReason":"installed server no longer matches the current catalog revision","installedAt":"2026-04-18T12:00:00Z","lastMaintainedAt":"2026-04-18T12:05:00Z","lastActionAt":"2026-04-18T12:06:00Z","lastAction":"revalidate","lastActionStatus":"completed","lastActionReason":"installed server no longer matches the current catalog revision","installInputSnapshot":{"serverId":"mcp-test","displayName":"MCP Test","enabled":true,"sandboxProfileId":"subprocess_default","secretRefs":["MCP_TEST_TOKEN"],"installMethod":"api"},"lastRevalidation":{"checkedAt":"2026-04-18T12:06:00Z","status":"ready","classification":"catalog_drift","reason":"installed server no longer matches the current catalog revision","issues":[{"kind":"catalog","name":"filesystem","status":"warning","reason":"installed server no longer matches the current catalog revision","environmentScope":"test"}]}},"enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:mcp-test:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","secretRefs":["MCP_TEST_TOKEN"],"autoRestart":true,"createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","state":{"serverId":"mcp-test","status":"healthy","failureCount":0,"restartCount":0,"updatedAt":"2026-04-18T12:00:02Z"},"toolCount":1,"tools":[{"serverId":"mcp-test","toolName":"lookup","discoveryStatus":"discovered","updatedAt":"2026-04-18T12:00:02Z","effectiveAvailability":"blocked","approvalRequired":false}]}]}`,
		"schemas/api/mcp-server-lifecycle.response.schema.json":               `{"action":"start","server":{"serverId":"mcp-test","displayName":"MCP Test","source":"api","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:mcp-test:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","secretRefs":["MCP_TEST_TOKEN"],"autoRestart":true,"createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","state":{"serverId":"mcp-test","status":"healthy","failureCount":0,"restartCount":0,"updatedAt":"2026-04-18T12:00:02Z"},"toolCount":1,"tools":[{"serverId":"mcp-test","toolName":"lookup","discoveryStatus":"discovered","updatedAt":"2026-04-18T12:00:02Z","effectiveAvailability":"blocked","approvalRequired":false}]},"idempotent":false,"executionId":"sandbox_exec_1","blocked":false,"preflightMs":42}`,
		"schemas/api/mcp-transport-capability.schema.json":                    `{"transportKind":"websocket","availabilityStatus":"ready","healthStatus":"degraded","reason":"one or more servers are recovering","prerequisites":["websocket endpoint must be configured per server","authenticated endpoints require secret-ref-backed header auth"],"environmentScope":"test","supportedAuthKinds":["bearer_header","header"],"daemonManagedReconnect":true,"recoverySummary":"daemon manages bounded websocket reconnect and restore history"}`,
		"schemas/api/mcp-transport-capability-list.response.schema.json":      `{"items":[{"transportKind":"stdio","availabilityStatus":"ready","healthStatus":"healthy","environmentScope":"test","daemonManagedReconnect":false},{"transportKind":"websocket","availabilityStatus":"ready","healthStatus":"degraded","reason":"one or more servers are recovering","prerequisites":["websocket endpoint must be configured per server"],"environmentScope":"test","supportedAuthKinds":["bearer_header"],"daemonManagedReconnect":true,"recoverySummary":"daemon manages bounded websocket reconnect and restore history"}]}`,
		"schemas/api/mcp-tool-resource.schema.json":                           `{"serverId":"mcp-test","toolName":"lookup","title":"Lookup","description":"Lookup tool","schemaFingerprint":"abc123","discoveryStatus":"discovered","lastDiscoveredAt":"2026-04-18T12:00:02Z","updatedAt":"2026-04-18T12:00:02Z","exposure":[{"serverId":"mcp-test","toolName":"lookup","runtimeSurface":"chat","exposureMode":"approval_required","active":true,"reason":"needs approval","updatedAt":"2026-04-18T12:00:03Z"}],"effectiveAvailability":"available","approvalRequired":true}`,
		"schemas/api/mcp-tool-list.response.schema.json":                      `{"items":[{"serverId":"mcp-test","toolName":"lookup","discoveryStatus":"discovered","updatedAt":"2026-04-18T12:00:02Z","effectiveAvailability":"blocked","approvalRequired":false}]}`,
		"schemas/api/mcp-tool-authorization.response.schema.json":             `{"status":"pending","tool":{"serverId":"mcp-test","toolName":"lookup","discoveryStatus":"discovered","updatedAt":"2026-04-18T12:00:02Z","effectiveAvailability":"available","approvalRequired":true},"message":"tool use requires approval","approval":{"approvalId":"approval_1","action":"tool_call.execute","resourceKind":"mcp_tool","resourceId":"mcp-test:lookup:chat","reason":"MCP tool execution requires approval","requestedBy":"web-ui","status":"pending","createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","sandbox":{"declaration":{"declarationId":"mcp_server:mcp-test:lifecycle.start:tool:chat:lookup","consumerKind":"mcp_server","consumerId":"mcp-test","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":["MCP_TEST_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_mcp_mcp-test_tool_call_execute_1","consumerKind":"mcp_server","consumerId":"mcp-test","operationKind":"tool_call.execute","declarationId":"mcp_server:mcp-test:lifecycle.start:tool:chat:lookup","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_1","decision":"ask","approvalStatus":"pending","secretResolution":"resolved","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}},"decision":{"decisionId":"decision_1","action":"tool_call.execute","resourceKind":"mcp_tool","resourceId":"mcp-test:lookup:chat","outcome":"requires_approval","reason":"MCP tool execution requires approval","approvalId":"approval_1","createdAt":"2026-04-18T12:00:00Z","sandbox":{"declaration":{"declarationId":"mcp_server:mcp-test:lifecycle.start:tool:chat:lookup","consumerKind":"mcp_server","consumerId":"mcp-test","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":["MCP_TEST_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_mcp_mcp-test_tool_call_execute_1","consumerKind":"mcp_server","consumerId":"mcp-test","operationKind":"tool_call.execute","declarationId":"mcp_server:mcp-test:lifecycle.start:tool:chat:lookup","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_1","decision":"ask","approvalStatus":"pending","secretResolution":"resolved","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}},"sandbox":{"declaration":{"declarationId":"mcp_server:mcp-test:lifecycle.start:tool:chat:lookup","consumerKind":"mcp_server","consumerId":"mcp-test","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":["MCP_TEST_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_mcp_mcp-test_tool_call_execute_1","consumerKind":"mcp_server","consumerId":"mcp-test","operationKind":"tool_call.execute","declarationId":"mcp_server:mcp-test:lifecycle.start:tool:chat:lookup","requestedBy":"web-ui","approvalId":"approval_1","decisionId":"decision_1","decision":"ask","approvalStatus":"pending","secretResolution":"resolved","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}}`,
		"schemas/api/mcp-catalog-entry.schema.json":                           `{"id":"filesystem","displayName":"Filesystem","description":"Local project filesystem access.","transportKind":"stdio","sourceKind":"bundled","tags":["local","filesystem"],"immediateUse":false,"prerequisites":[{"kind":"binary","name":"npx","required":true,"description":"Node.js with npx available on PATH"}],"environmentEligibility":["test","prod"],"availabilityStatus":"unavailable","availabilityReason":"default bundled stdio command requires a local command override because sandbox network is denied","installSupport":{"scriptSupported":true,"scriptArgs":["filesystem"]},"defaultInstallSpec":{"displayName":"Filesystem","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:filesystem:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/tmp/dope"],"workingDir":"/tmp/dope","autoRestart":true}}`,
		"schemas/api/mcp-catalog-list.response.schema.json":                   `{"items":[{"id":"filesystem","displayName":"Filesystem","description":"Local project filesystem access.","transportKind":"stdio","sourceKind":"bundled","tags":["local","filesystem"],"immediateUse":false,"availabilityStatus":"unavailable","availabilityReason":"default bundled stdio command requires a local command override because sandbox network is denied","installSupport":{"scriptSupported":true,"scriptArgs":["filesystem"]},"defaultInstallSpec":{"displayName":"Filesystem","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:filesystem:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/tmp/dope"],"workingDir":"/tmp/dope","autoRestart":true}}]}`,
		"schemas/api/mcp-catalog-detail.response.schema.json":                 `{"id":"filesystem","displayName":"Filesystem","description":"Local project filesystem access.","transportKind":"stdio","sourceKind":"bundled","tags":["local","filesystem"],"immediateUse":false,"availabilityStatus":"unavailable","availabilityReason":"default bundled stdio command requires a local command override because sandbox network is denied","installSupport":{"scriptSupported":true,"scriptArgs":["filesystem"]},"defaultInstallSpec":{"displayName":"Filesystem","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:filesystem:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/tmp/dope"],"workingDir":"/tmp/dope","autoRestart":true}}`,
		"schemas/api/mcp-catalog-install-result.schema.json":                  `{"installId":"mcp_install_1","status":"installed","catalogEntryId":"filesystem","serverId":"filesystem-test","availabilityStatus":"ready","auditEventIds":["evt_install_1","evt_install_2"],"server":{"serverId":"filesystem-test","displayName":"Filesystem","source":"api","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:installed","driftStatus":"in_sync","installedAt":"2026-04-18T12:00:00Z","lastActionAt":"2026-04-18T12:00:00Z","lastAction":"install","lastActionStatus":"completed","installInputSnapshot":{"serverId":"filesystem-test","displayName":"Filesystem","enabled":true,"sandboxProfileId":"subprocess_default","installMethod":"api"}},"enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:filesystem:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","autoRestart":true,"createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","state":{"serverId":"filesystem-test","status":"stopped","failureCount":0,"restartCount":0,"updatedAt":"2026-04-18T12:00:00Z"},"transportConfigSummary":"/tmp/mcp-helper --stdio","availabilityStatus":"ready","toolCount":0}}`,
		"schemas/api/mcp-catalog-lifecycle-result.schema.json":                `{"actionId":"mcp_catalog_refresh_1","action":"refresh","status":"completed","serverId":"filesystem-test","catalogEntryId":"filesystem","auditEventIds":["evt_maintenance_1","evt_maintenance_2"],"server":{"serverId":"filesystem-test","displayName":"Filesystem","source":"api","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:installed","driftStatus":"in_sync","installedAt":"2026-04-18T12:00:00Z","lastMaintainedAt":"2026-04-18T12:05:00Z","lastActionAt":"2026-04-18T12:05:00Z","lastAction":"refresh","lastActionStatus":"completed","installInputSnapshot":{"serverId":"filesystem-test","displayName":"Filesystem","enabled":true,"sandboxProfileId":"subprocess_default","installMethod":"api"}},"enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:filesystem:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","autoRestart":true,"createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:05:00Z","state":{"serverId":"filesystem-test","status":"stopped","failureCount":0,"restartCount":0,"updatedAt":"2026-04-18T12:05:00Z"},"availabilityStatus":"ready","toolCount":0},"preflightMs":42}`,
		"schemas/api/mcp-catalog-revalidation-result.schema.json":             `{"actionId":"mcp_revalidate_1","action":"revalidate","serverId":"filesystem-test","catalogEntryId":"filesystem","status":"blocked","classification":"prerequisite_lost","reason":"MCP_TEST_TOKEN is required","issues":[{"kind":"secret","name":"MCP_TEST_TOKEN","status":"blocked","reason":"MCP_TEST_TOKEN is required","environmentScope":"test"}],"auditEventIds":["evt_revalidate_1","evt_revalidate_2"],"server":{"serverId":"filesystem-test","displayName":"Filesystem","source":"api","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","environmentScope":"test","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:installed","driftStatus":"in_sync","installedAt":"2026-04-18T12:00:00Z","lastActionAt":"2026-04-18T12:06:00Z","lastAction":"revalidate","lastActionStatus":"completed","lastActionReason":"MCP_TEST_TOKEN is required","installInputSnapshot":{"serverId":"filesystem-test","displayName":"Filesystem","enabled":true,"sandboxProfileId":"subprocess_default","secretRefs":["MCP_TEST_TOKEN"],"installMethod":"api"},"lastRevalidation":{"checkedAt":"2026-04-18T12:06:00Z","status":"blocked","classification":"prerequisite_lost","reason":"MCP_TEST_TOKEN is required","issues":[{"kind":"secret","name":"MCP_TEST_TOKEN","status":"blocked","reason":"MCP_TEST_TOKEN is required","environmentScope":"test"}]}},"enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:filesystem:lifecycle.start","declaration":{"executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true},"transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","secretRefs":["MCP_TEST_TOKEN"],"autoRestart":true,"createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","state":{"serverId":"filesystem-test","status":"stopped","failureCount":0,"restartCount":0,"updatedAt":"2026-04-18T12:00:00Z"},"availabilityStatus":"blocked","availabilityReason":"MCP_TEST_TOKEN is required","toolCount":0},"preflightMs":25}`,
		"schemas/api/tool-call-resource.schema.json":                          `{"toolCallId":"tool_call_mcp_1","runId":"run_1","stepId":"step_1","invocationKind":"mcp_tool","mcpServerId":"filesystem-test","mcpServerName":"Filesystem","mcpToolName":"lookup","mcpTransportKind":"stdio","mcpSessionId":"session_1","authorizationResult":"allowed","toolName":"lookup","status":"completed","createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:01Z","output":{"result":{"content":[{"type":"text","text":"[REDACTED]"}]}},"sandbox":{"declaration":{"declarationId":"mcp_server:filesystem-test:lifecycle.start:tool:chat:lookup","consumerKind":"mcp_server","consumerId":"filesystem-test","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"networkMode":"deny","secretRefs":["MCP_TEST_TOKEN"],"approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_mcp_1","consumerKind":"mcp_server","consumerId":"filesystem-test","operationKind":"tool_call.execute","declarationId":"mcp_server:filesystem-test:lifecycle.start:tool:chat:lookup","requestedBy":"web-ui","decision":"allow","approvalStatus":"approved","secretResolution":"resolved","enforcementStrength":"declared_only","toolCallId":"tool_call_mcp_1","startedAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:01Z","status":"completed"}}}`,
		"schemas/events/mcp-server-registered.event.schema.json":              `{"eventId":"evt_mcp_1","sequence":20,"category":"mcp","name":"mcp.server_registered","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","displayName":"MCP Test","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:mcp-test:lifecycle.start","transportKind":"stdio","availabilityStatus":"ready","availabilityReason":"","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:installed","driftStatus":"in_sync"},"created":true}}`,
		"schemas/events/mcp-server-updated.event.schema.json":                 `{"eventId":"evt_mcp_2","sequence":21,"category":"mcp","name":"mcp.server_updated","occurredAt":"2026-04-18T12:00:02Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","displayName":"MCP Test","originKind":"catalog","catalogEntryId":"filesystem","installMethod":"api","enabled":false,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:mcp-test:lifecycle.start","transportKind":"stdio","availabilityStatus":"unavailable","availabilityReason":"server is not healthy","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:current","driftStatus":"catalog_updated","driftReason":"installed server no longer matches the current catalog revision"},"created":false}}`,
		"schemas/events/mcp-server-started.event.schema.json":                 `{"eventId":"evt_mcp_3","sequence":22,"category":"mcp","name":"mcp.server_started","occurredAt":"2026-04-18T12:00:03Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","status":"healthy","executionId":"sandbox_exec_1","toolCount":1,"transportKind":"stdio"}}`,
		"schemas/events/mcp-server-stopped.event.schema.json":                 `{"eventId":"evt_mcp_4","sequence":23,"category":"mcp","name":"mcp.server_stopped","occurredAt":"2026-04-18T12:00:04Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","status":"stopping","executionId":"sandbox_exec_1","cancelled":false}}`,
		"schemas/events/mcp-server-failed.event.schema.json":                  `{"eventId":"evt_mcp_5","sequence":24,"category":"mcp","name":"mcp.server_failed","occurredAt":"2026-04-18T12:00:05Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","status":"failed","reason":"transport failed","failureClass":"transport_runtime_failure"}}`,
		"schemas/events/mcp-server-health-changed.event.schema.json":          `{"eventId":"evt_mcp_6","sequence":25,"category":"mcp","name":"mcp.server_health_changed","occurredAt":"2026-04-18T12:00:06Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","status":"degraded","reason":"heartbeat stale","availabilityStatus":"unavailable","availabilityReason":"heartbeat stale","catalogManagement":{"sourceKind":"bundled","installedRevision":"sha256:installed","currentRevision":"sha256:current","driftStatus":"catalog_updated","driftReason":"installed server no longer matches the current catalog revision"}}}`,
		"schemas/events/mcp-server-reconnect-scheduled.event.schema.json":     `{"eventId":"evt_mcp_6a","sequence":25,"category":"mcp","name":"mcp.server_reconnect_scheduled","occurredAt":"2026-04-18T12:00:06Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","transportKind":"websocket","attempt":1,"reason":"websocket disconnected","nextRetryAt":"2026-04-18T12:00:11Z"}}`,
		"schemas/events/mcp-server-reconnect-completed.event.schema.json":     `{"eventId":"evt_mcp_6b","sequence":26,"category":"mcp","name":"mcp.server_reconnect_completed","occurredAt":"2026-04-18T12:00:07Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","transportKind":"websocket","attempt":1,"sessionId":"session_ws_1"}}`,
		"schemas/events/mcp-server-reconnect-failed.event.schema.json":        `{"eventId":"evt_mcp_6c","sequence":27,"category":"mcp","name":"mcp.server_reconnect_failed","occurredAt":"2026-04-18T12:00:08Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","transportKind":"websocket","attempt":3,"reason":"reconnect exhausted","failureClass":"reconnect_exhausted"}}`,
		"schemas/events/mcp-server-restore-completed.event.schema.json":       `{"eventId":"evt_mcp_6d","sequence":28,"category":"mcp","name":"mcp.server_restore_completed","occurredAt":"2026-04-18T12:00:09Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","transportKind":"websocket","sessionId":"session_ws_2","toolCount":1}}`,
		"schemas/events/mcp-server-restore-failed.event.schema.json":          `{"eventId":"evt_mcp_6e","sequence":29,"category":"mcp","name":"mcp.server_restore_failed","occurredAt":"2026-04-18T12:00:10Z","scope":{},"resource":{"kind":"mcp_server","id":"mcp-test"},"payload":{"serverId":"mcp-test","transportKind":"websocket","reason":"dial failed","failureClass":"transport_runtime_failure"}}`,
		"schemas/events/mcp-catalog-lifecycle-requested.event.schema.json":    `{"eventId":"evt_mcp_11","sequence":30,"category":"mcp","name":"mcp.catalog_lifecycle_requested","occurredAt":"2026-04-18T12:00:11Z","scope":{},"resource":{"kind":"mcp_server","id":"filesystem-test"},"payload":{"actionId":"mcp_catalog_refresh_1","action":"refresh","serverId":"filesystem-test","catalogEntryId":"filesystem","environment":"test"}}`,
		"schemas/events/mcp-catalog-lifecycle-completed.event.schema.json":    `{"eventId":"evt_mcp_12","sequence":31,"category":"mcp","name":"mcp.catalog_lifecycle_completed","occurredAt":"2026-04-18T12:00:12Z","scope":{},"resource":{"kind":"mcp_server","id":"filesystem-test"},"payload":{"actionId":"mcp_catalog_refresh_1","action":"refresh","serverId":"filesystem-test","catalogEntryId":"filesystem","status":"completed","removed":false,"environment":"test"}}`,
		"schemas/events/mcp-catalog-lifecycle-failed.event.schema.json":       `{"eventId":"evt_mcp_13","sequence":32,"category":"mcp","name":"mcp.catalog_lifecycle_failed","occurredAt":"2026-04-18T12:00:13Z","scope":{},"resource":{"kind":"mcp_server","id":"filesystem-test"},"payload":{"actionId":"mcp_catalog_refresh_2","action":"refresh","serverId":"filesystem-test","catalogEntryId":"filesystem","status":"blocked","failureClass":"conflict","reason":"server has local operator modifications","environment":"test"}}`,
		"schemas/events/mcp-catalog-revalidation-completed.event.schema.json": `{"eventId":"evt_mcp_14","sequence":33,"category":"mcp","name":"mcp.catalog_revalidation_completed","occurredAt":"2026-04-18T12:00:14Z","scope":{},"resource":{"kind":"mcp_server","id":"filesystem-test"},"payload":{"actionId":"mcp_revalidate_1","action":"revalidate","serverId":"filesystem-test","catalogEntryId":"filesystem","status":"blocked","classification":"prerequisite_lost","reason":"MCP_TEST_TOKEN is required","issues":[{"kind":"secret","name":"MCP_TEST_TOKEN","status":"blocked","reason":"MCP_TEST_TOKEN is required","environmentScope":"test"}],"environment":"test"}}`,
		"schemas/events/mcp-tool-exposure-updated.event.schema.json":          `{"eventId":"evt_mcp_7","sequence":26,"category":"mcp","name":"mcp.tool_exposure_updated","occurredAt":"2026-04-18T12:00:07Z","scope":{},"resource":{"kind":"mcp_tool","id":"mcp-test:lookup"},"payload":{"serverId":"mcp-test","toolName":"lookup","runtimeSurface":"chat","exposureMode":"approval_required","active":true,"reason":"needs approval"}}`,
		"schemas/events/mcp-catalog-install-requested.event.schema.json":      `{"eventId":"evt_mcp_8","sequence":27,"category":"mcp","name":"mcp.catalog_install_requested","occurredAt":"2026-04-18T12:00:08Z","scope":{},"resource":{"kind":"mcp_catalog_install","id":"mcp_install_1"},"payload":{"installId":"mcp_install_1","catalogEntryId":"filesystem","method":"api","environment":"test"}}`,
		"schemas/events/mcp-catalog-install-completed.event.schema.json":      `{"eventId":"evt_mcp_9","sequence":28,"category":"mcp","name":"mcp.catalog_install_completed","occurredAt":"2026-04-18T12:00:09Z","scope":{},"resource":{"kind":"mcp_catalog_install","id":"mcp_install_1"},"payload":{"installId":"mcp_install_1","catalogEntryId":"filesystem","serverId":"filesystem-test","method":"api","status":"installed","availabilityStatus":"ready"}}`,
		"schemas/events/mcp-catalog-install-failed.event.schema.json":         `{"eventId":"evt_mcp_10","sequence":29,"category":"mcp","name":"mcp.catalog_install_failed","occurredAt":"2026-04-18T12:00:10Z","scope":{},"resource":{"kind":"mcp_catalog_install","id":"mcp_install_2"},"payload":{"installId":"mcp_install_2","catalogEntryId":"github","method":"api","status":"blocked","availabilityStatus":"blocked","availabilityReason":"GitHub personal access token"}}`,
	}

	mustValidateFixtures(t, validator, fixtures)
}

func TestSkillSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/skill-file.schema.json":                      `{"path":"assets/guide.md","sizeBytes":42}`,
		"schemas/api/skill-summary.schema.json":                   `{"skillId":"exec-skill","name":"exec-skill","description":"executable skill","source":"data_dir","rootPath":"/tmp/dope/skills","skillPath":"/tmp/dope/skills/exec-skill","instructionPath":"/tmp/dope/skills/exec-skill/SKILL.md","files":[{"path":"assets/guide.md","sizeBytes":42}],"frontmatter":{"name":"exec-skill","description":"executable skill"},"executionManifest":{"entrypoint":"/tmp/dope/skills/exec-skill/scripts/run.sh","args":["alpha","beta"],"workingDir":"/tmp/dope/skills/exec-skill","profileId":"subprocess_default","backendKind":"subprocess","readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","secretRefs":["EXEC_SKILL_TOKEN"],"approvalMode":"ask","timeoutMs":1000,"requiredEnforcementStrength":"declared_only"},"availabilityStatus":"available","sandbox":{"declaration":{"declarationId":"skill:exec-skill:tool_call.execute","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","secretRefs":["EXEC_SKILL_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"secretScope":[{"consumerKind":"skill","consumerId":"exec-skill","secretRef":"EXEC_SKILL_TOKEN","environmentScope":"test","defaultSource":"kind_default","defaultRuleId":"skill:exec-skill","deliveryKind":"environment_variable","redactionRule":"value_redacted","resolution":"resolved"}]}}`,
		"schemas/api/skill-detail.response.schema.json":           `{"skillId":"broken-skill","name":"broken-skill","description":"invalid executable skill","source":"data_dir","rootPath":"/tmp/dope/skills","skillPath":"/tmp/dope/skills/broken-skill","instructionPath":"/tmp/dope/skills/broken-skill/SKILL.md","files":[{"path":"assets/guide.md","sizeBytes":42}],"frontmatter":{"name":"broken-skill","description":"invalid executable skill"},"frontmatterRaw":"name: broken-skill","body":"data instructions","executionManifest":{"entrypoint":"/tmp/dope/skills/broken-skill/scripts/run.sh","profileId":"subprocess_default","backendKind":"subprocess","approvalMode":"ask"},"availabilityStatus":"unavailable","availabilityReason":"executable skill secret ref EXEC_SKILL_TOKEN is unavailable for test environment"}`,
		"schemas/api/skill-overlay.schema.json":                   `{"overlayId":"data_dir_agents","source":"data_dir","path":"/tmp/dope/AGENTS.md","sizeBytes":12,"modifiedAt":"2026-04-18T12:00:00Z"}`,
		"schemas/api/skill-registry.response.schema.json":         `{"loadedAt":"2026-04-18T12:00:00Z","items":[{"skillId":"exec-skill","name":"exec-skill","description":"executable skill","source":"data_dir","rootPath":"/tmp/dope/skills","skillPath":"/tmp/dope/skills/exec-skill","instructionPath":"/tmp/dope/skills/exec-skill/SKILL.md","files":[{"path":"assets/guide.md","sizeBytes":42}],"frontmatter":{"name":"exec-skill","description":"executable skill"},"executionManifest":{"entrypoint":"/tmp/dope/skills/exec-skill/scripts/run.sh","profileId":"subprocess_default","backendKind":"subprocess","approvalMode":"ask"},"availabilityStatus":"available"},{"skillId":"broken-skill","name":"broken-skill","description":"invalid executable skill","source":"data_dir","rootPath":"/tmp/dope/skills","skillPath":"/tmp/dope/skills/broken-skill","instructionPath":"/tmp/dope/skills/broken-skill/SKILL.md","files":[{"path":"assets/guide.md","sizeBytes":42}],"frontmatter":{"name":"broken-skill","description":"invalid executable skill"},"executionManifest":{"entrypoint":"/tmp/dope/skills/broken-skill/scripts/run.sh","profileId":"subprocess_default","backendKind":"subprocess","approvalMode":"ask"},"availabilityStatus":"unavailable","availabilityReason":"executable skill secret ref EXEC_SKILL_TOKEN is unavailable for test environment"}],"overlays":[{"overlayId":"home_agents","source":"home","path":"/tmp/home/.agents/AGENTS.md","sizeBytes":11,"modifiedAt":"2026-04-18T12:00:00Z"}]}`,
		"schemas/api/sandbox-backend-capability.schema.json":      `{"backendKind":"docker","displayName":"Docker","filesystemEnforcement":"container_mount_scoped","networkEnforcement":"container_network_mode","envInjectionMode":"container_env_injection","approvalBehavior":"profile_and_command_policy","restartBehavior":"interrupted_execution_recovers_as_cancelled","hostPrerequisites":["docker CLI available on PATH"],"availabilityStatus":"unavailable","availabilityReason":"docker CLI is not available on PATH"}`,
		"schemas/api/sandbox-profile.schema.json":                 `{"profileId":"subprocess_default","title":"Default Subprocess Sandbox","description":"Conservative local subprocess execution for the harness control plane.","backendKind":"subprocess","backendCapability":{"backendKind":"subprocess","displayName":"Subprocess","filesystemEnforcement":"declared_scoped","networkEnforcement":"declared_only","envInjectionMode":"filtered_host_env","approvalBehavior":"profile_and_command_policy","restartBehavior":"interrupted_execution_recovers_as_cancelled","availabilityStatus":"available"},"defaultWorkDir":"/tmp/dope-data","filesystemPolicy":{"mode":"scoped","readRoots":["/tmp/dope-data"],"writeRoots":["/tmp/dope-data"],"tempRoots":["/tmp"],"allowDataDir":true,"allowUserAgentsDir":true,"allowHomeRead":false,"allowHomeWrite":false},"networkPolicy":{"mode":"deny","allowedHosts":[],"allowedPorts":[],"allowLoopback":false,"enforcementMode":"declared_only"},"envPolicy":{"mode":"inherit_safe","allowedVars":["PATH"],"injectedVars":{"DOPE_DATA_DIR":"/tmp/dope-data"},"redactedVars":[]},"approvalPolicy":{"mode":"ask","requiredForCommands":["curl"],"requiredForWritesOutsideRoots":true,"requiredForNetwork":true,"requiredForUnknownBackends":true},"processPolicy":{"timeoutMs":30000,"maxTimeoutMs":300000,"killGraceMs":1000,"captureStdout":true,"captureStderr":true,"maxOutputBytes":65536,"allowStreaming":false,"restartOnFailure":false},"defaultTimeoutMs":30000,"maxTimeoutMs":300000,"restartable":false,"source":"builtin","active":true}`,
		"schemas/api/sandbox-profile-list.response.schema.json":   `{"items":[{"profileId":"subprocess_default","title":"Default Subprocess Sandbox","description":"Conservative local subprocess execution for the harness control plane.","backendKind":"subprocess","backendCapability":{"backendKind":"subprocess","displayName":"Subprocess","filesystemEnforcement":"declared_scoped","networkEnforcement":"declared_only","envInjectionMode":"filtered_host_env","approvalBehavior":"profile_and_command_policy","restartBehavior":"interrupted_execution_recovers_as_cancelled","availabilityStatus":"available"},"defaultWorkDir":"/tmp/dope-data","filesystemPolicy":{"mode":"scoped","readRoots":["/tmp/dope-data"],"writeRoots":["/tmp/dope-data"],"tempRoots":["/tmp"],"allowDataDir":true,"allowUserAgentsDir":true,"allowHomeRead":false,"allowHomeWrite":false},"networkPolicy":{"mode":"deny","allowedHosts":[],"allowedPorts":[],"allowLoopback":false,"enforcementMode":"declared_only"},"envPolicy":{"mode":"inherit_safe","allowedVars":["PATH"],"injectedVars":{"DOPE_DATA_DIR":"/tmp/dope-data"},"redactedVars":[]},"approvalPolicy":{"mode":"ask","requiredForCommands":["curl"],"requiredForWritesOutsideRoots":true,"requiredForNetwork":true,"requiredForUnknownBackends":true},"processPolicy":{"timeoutMs":30000,"maxTimeoutMs":300000,"killGraceMs":1000,"captureStdout":true,"captureStderr":true,"maxOutputBytes":65536,"allowStreaming":false,"restartOnFailure":false},"defaultTimeoutMs":30000,"maxTimeoutMs":300000,"restartable":false,"source":"builtin","active":true}]}`,
		"schemas/api/sandbox-decision.schema.json":                `{"decisionId":"sandbox_decision_1","executionId":"sandbox_exec_1","resolution":"ask","selectionOutcome":"selected","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","requiredBackendKind":"subprocess","hostStatus":"ready","explanation":"sandbox execution requires approval"}`,
		"schemas/api/sandbox-result.schema.json":                  `{"executionId":"sandbox_exec_1","status":"denied","outputTruncated":false,"partial":false,"errorClass":"approval_required","errorCode":"sandbox_approval_required","error":"sandbox execution requires approval","backendMetadata":{"managedProviderId":"codex_managed","managedProviderAction":"prompt_execution","managedProviderOperationId":"managed_provider_op_1","enforcementStrength":"declared_only","sensitiveStateClasses":["config_file","temp_output"]}}`,
		"schemas/api/sandbox-execution.resource.schema.json":      `{"executionId":"sandbox_exec_1","profileId":"subprocess_default","backendKind":"subprocess","command":"echo","args":["hello"],"cwd":"/tmp/dope","envKeys":["HOME"],"stdinProvided":false,"timeoutMs":1000,"requestedBy":"web-ui","resourceKind":"skill","resourceId":"shared","scope":"chat","approvalId":"approval_1","reason":"inspect profile","metadata":{"ticket":"sandbox-16","managedProviderId":"codex_managed","managedProviderAction":"prompt_execution","managedProviderOperationId":"managed_provider_op_1","sandboxProfileId":"managed_provider_codex","sandboxDecision":"ask","enforcementStrength":"declared_only","sensitiveStateClasses":"config_file,temp_output"},"access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"networkMode":"allow_list","allowedHosts":["localhost"],"allowedPorts":[80],"allowLoopback":true},"status":"denied","decision":{"decisionId":"sandbox_decision_1","executionId":"sandbox_exec_1","resolution":"ask","selectionOutcome":"selected","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","requiredBackendKind":"subprocess","hostStatus":"ready","explanation":"sandbox execution requires approval"},"result":{"executionId":"sandbox_exec_1","status":"denied","outputTruncated":false,"partial":false,"errorClass":"approval_required","errorCode":"sandbox_approval_required","error":"sandbox execution requires approval","backendMetadata":{"managedProviderId":"codex_managed","managedProviderAction":"prompt_execution","managedProviderOperationId":"managed_provider_op_1","enforcementStrength":"declared_only","sensitiveStateClasses":["config_file","temp_output"]}},"requestedAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z"}`,
		"schemas/api/sandbox-execution-list.response.schema.json": `{"items":[{"executionId":"sandbox_exec_1","profileId":"subprocess_default","backendKind":"subprocess","command":"echo","args":["hello"],"cwd":"/tmp/dope","envKeys":["HOME"],"stdinProvided":false,"timeoutMs":1000,"access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"networkMode":"allow_list","allowedHosts":["localhost"],"allowedPorts":[80],"allowLoopback":true},"status":"denied","decision":{"decisionId":"sandbox_decision_1","executionId":"sandbox_exec_1","resolution":"ask","selectionOutcome":"selected","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","requiredBackendKind":"subprocess","hostStatus":"ready","explanation":"sandbox execution requires approval"},"result":{"executionId":"sandbox_exec_1","status":"denied","outputTruncated":false,"partial":false,"errorClass":"approval_required","errorCode":"sandbox_approval_required","error":"sandbox execution requires approval"},"requestedAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z"}]}`,
		"schemas/api/sandbox-explain.response.schema.json":        `{"decision":{"decisionId":"sandbox_decision_1","resolution":"ask","selectionOutcome":"selected","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","requiredBackendKind":"subprocess","hostStatus":"ready","explanation":"sandbox execution requires approval"}}`,
	}

	assertFixtureExecutableSkillInspection(t, fixtures["schemas/api/skill-summary.schema.json"], "exec-skill", "available", "ask")
	assertFixtureExecutableSkillInspection(t, fixtures["schemas/api/skill-detail.response.schema.json"], "broken-skill", "unavailable", "ask")
	for schemaPath, fixture := range fixtures {
		t.Run(filepath.Base(schemaPath), func(t *testing.T) {
			if err := validator.ValidateRelative(schemaPath, []byte(fixture)); err != nil {
				t.Fatalf("ValidateRelative returned error: %v", err)
			}
		})
	}
}

func TestSkillBackedExecutionSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/tool-call-resource.schema.json":         `{"toolCallId":"tool_call_skill_1","runId":"run_1","stepId":"step_1","invocationKind":"skill","skillId":"exec-skill","toolName":"exec-skill","status":"completed","sandboxExecutionId":"sandbox_exec_skill_1","createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:02Z","output":{"stdout":"[REDACTED]"},"sandbox":{"declaration":{"declarationId":"skill:exec-skill:tool_call.execute","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","secretRefs":["EXEC_SKILL_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"secretScope":[{"consumerKind":"skill","consumerId":"exec-skill","secretRef":"EXEC_SKILL_TOKEN","environmentScope":"test","defaultSource":"kind_default","defaultRuleId":"skill:exec-skill","deliveryKind":"environment_variable","redactionRule":"value_redacted","resolution":"resolved"}],"policyRecord":{"policyRecordId":"policy_skill_1","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","declarationId":"skill:exec-skill:tool_call.execute","requestedBy":"web-ui","decision":"allow","approvalStatus":"approved","secretResolution":"resolved","enforcementStrength":"declared_only","sandboxExecutionId":"sandbox_exec_skill_1","toolCallId":"tool_call_skill_1","startedAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:02Z","status":"completed"}}}`,
		"schemas/api/tool-call-list.response.schema.json":    `{"items":[{"toolCallId":"tool_call_skill_1","runId":"run_1","stepId":"step_1","invocationKind":"skill","skillId":"exec-skill","toolName":"exec-skill","status":"completed","sandboxExecutionId":"sandbox_exec_skill_1","createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:02Z","sandbox":{"declaration":{"declarationId":"skill:exec-skill:tool_call.execute","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","secretRefs":["EXEC_SKILL_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"policyRecord":{"policyRecordId":"policy_skill_1","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","declarationId":"skill:exec-skill:tool_call.execute","requestedBy":"web-ui","decision":"allow","approvalStatus":"approved","secretResolution":"resolved","enforcementStrength":"declared_only","sandboxExecutionId":"sandbox_exec_skill_1","toolCallId":"tool_call_skill_1","startedAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:02Z","status":"completed"}}}]}`,
		"schemas/api/approval-resource.schema.json":          `{"approvalId":"approval_skill_1","action":"tool_call.execute","resourceKind":"skill","resourceId":"exec-skill","reason":"needs approval","status":"pending","createdAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z","sandbox":{"declaration":{"declarationId":"skill:exec-skill:tool_call.execute","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","secretRefs":["EXEC_SKILL_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"secretScope":[{"consumerKind":"skill","consumerId":"exec-skill","secretRef":"EXEC_SKILL_TOKEN","environmentScope":"test","defaultSource":"kind_default","defaultRuleId":"skill:exec-skill","deliveryKind":"environment_variable","redactionRule":"value_redacted","resolution":"resolved"}],"policyRecord":{"policyRecordId":"policy_skill_approval_1","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","declarationId":"skill:exec-skill:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_skill_1","decisionId":"decision_skill_1","decision":"ask","approvalStatus":"pending","secretResolution":"resolved","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}}`,
		"schemas/api/decision-resource.schema.json":          `{"decisionId":"decision_skill_1","action":"tool_call.execute","resourceKind":"skill","resourceId":"exec-skill","outcome":"requires_approval","reason":"needs approval","approvalId":"approval_skill_1","createdAt":"2026-04-18T12:00:00Z","sandbox":{"declaration":{"declarationId":"skill:exec-skill:tool_call.execute","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","secretRefs":["EXEC_SKILL_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"secretScope":[{"consumerKind":"skill","consumerId":"exec-skill","secretRef":"EXEC_SKILL_TOKEN","environmentScope":"test","defaultSource":"kind_default","defaultRuleId":"skill:exec-skill","deliveryKind":"environment_variable","redactionRule":"value_redacted","resolution":"resolved"}],"policyRecord":{"policyRecordId":"policy_skill_approval_1","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","declarationId":"skill:exec-skill:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_skill_1","decisionId":"decision_skill_1","decision":"ask","approvalStatus":"pending","secretResolution":"resolved","enforcementStrength":"declared_only","startedAt":"2026-04-18T12:00:00Z","status":"approval_pending"}}}`,
		"schemas/api/sandbox-execution.resource.schema.json": `{"executionId":"sandbox_exec_skill_1","profileId":"subprocess_default","backendKind":"subprocess","command":"/tmp/dope/skills/exec-skill/scripts/run.sh","args":["alpha"],"cwd":"/tmp/dope/skills/exec-skill","envKeys":["EXEC_SKILL_TOKEN"],"stdinProvided":false,"timeoutMs":1000,"requestedBy":"web-ui","resourceKind":"skill","resourceId":"exec-skill","scope":"tool_call","approvalId":"approval_skill_1","access":{"readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","allowedHosts":[],"allowedPorts":[]},"status":"completed","decision":{"decisionId":"sandbox_decision_skill_1","executionId":"sandbox_exec_skill_1","resolution":"allow","selectionOutcome":"selected","matchedRules":["profile:subprocess_default"],"approvalRequired":false,"approvalStatus":"approved","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","requiredBackendKind":"subprocess","hostStatus":"ready","explanation":"sandbox execution allowed"},"result":{"executionId":"sandbox_exec_skill_1","status":"completed","stdout":"[REDACTED]","stderr":"","exitCode":0,"outputTruncated":false,"partial":false},"requestedAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:02Z","startedAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:02Z","consumer":{"declaration":{"declarationId":"skill:exec-skill:tool_call.execute","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","profileId":"subprocess_default","executionMode":"subprocess","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/exec-skill"],"writeRoots":["/tmp/dope/skills/exec-skill"],"networkMode":"deny","secretRefs":["EXEC_SKILL_TOKEN"],"approvalMode":"ask","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"},"secretScope":[{"consumerKind":"skill","consumerId":"exec-skill","secretRef":"EXEC_SKILL_TOKEN","environmentScope":"test","defaultSource":"kind_default","defaultRuleId":"skill:exec-skill","deliveryKind":"environment_variable","redactionRule":"value_redacted","resolution":"resolved"}],"policyRecord":{"policyRecordId":"policy_skill_1","consumerKind":"skill","consumerId":"exec-skill","operationKind":"tool_call.execute","declarationId":"skill:exec-skill:tool_call.execute","requestedBy":"web-ui","approvalId":"approval_skill_1","decisionId":"decision_skill_2","decision":"allow","approvalStatus":"approved","secretResolution":"resolved","enforcementStrength":"declared_only","sandboxExecutionId":"sandbox_exec_skill_1","toolCallId":"tool_call_skill_1","startedAt":"2026-04-18T12:00:00Z","completedAt":"2026-04-18T12:00:02Z","status":"completed"}}}`,
	}

	assertFixtureToolCallSandboxLinkage(t, fixtures["schemas/api/tool-call-resource.schema.json"], "skill", "exec-skill", "tool_call_skill_1", "sandbox_exec_skill_1")
	assertFixtureSandboxDeclaration(t, fixtures["schemas/api/approval-resource.schema.json"], "skill", "exec-skill", "tool_call.execute")
	assertFixtureSandboxPolicyRecord(t, fixtures["schemas/api/approval-resource.schema.json"], "approval_pending", "resolved")
	assertFixtureSandboxExecutionConsumerLinkage(t, fixtures["schemas/api/sandbox-execution.resource.schema.json"], "skill", "exec-skill", "tool_call_skill_1", "sandbox_exec_skill_1")
	mustValidateFixtures(t, validator, fixtures)
}

func TestSandboxResultSchemaAcceptsManagedProviderFailureClass(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixture := `{"executionId":"sandbox_exec_provider_1","status":"failed","outputTruncated":false,"partial":false,"errorClass":"provider_auth_failed","errorCode":"upstream_auth_failed","error":"not logged in","backendMetadata":{"managedProviderId":"claude_managed","managedProviderAction":"prompt_execution","managedProviderOperationId":"managed_provider_op_1","enforcementStrength":"declared_only","sensitiveStateClasses":["settings_file"]}}`
	if err := validator.ValidateRelative("schemas/api/sandbox-result.schema.json", []byte(fixture)); err != nil {
		t.Fatalf("ValidateRelative returned error: %v", err)
	}
}

func TestStreamingTimeoutSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/events/llm-dispatch-partial-failed.event.schema.json": `{"eventId":"evt_partial_1","sequence":8,"category":"llm","name":"llm.dispatch.partial_failed","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"llm_dispatch","id":"dispatch_1"},"payload":{"provider":"openai_compatible","model":"gpt-5.4","status":"partial_failed","partial":true,"attemptCount":1,"finishReason":"","usage":{"inputTokens":1,"outputTokens":2,"totalTokens":3},"errorCode":"idle_timeout","error":"stream stalled","skills":["shared"],"skillContracts":[{"declaration":{"declarationId":"skill:shared:selection","consumerKind":"skill","consumerId":"shared","operationKind":"skill_selection","profileId":"subprocess_default","executionMode":"declaration_only","allowedBackendKinds":["subprocess"],"readRoots":["/tmp/dope/skills/shared"],"writeRoots":[],"networkMode":"deny","secretRefs":[],"approvalMode":"allow","requiredEnforcementStrength":"declared_only","active":true,"source":"builtin"}}]}}`,
		"schemas/events/connector-reply-partial.event.schema.json":     `{"eventId":"evt_partial_2","sequence":9,"category":"connector","name":"connector.reply_partial","occurredAt":"2026-04-18T12:00:01Z","scope":{"runId":"run_1","stepId":"step_1","connectorId":"discord-main"},"resource":{"kind":"connector","id":"discord-main"},"payload":{"messageId":"msg_1","replyMessageId":"reply_1","replyMessageIds":["reply_1"],"partCount":1,"contentLength":128,"error":"stream stalled","errorClass":""}}`,
		"schemas/events/sandbox-execution-requested.event.schema.json": `{"eventId":"evt_sandbox_1","sequence":10,"category":"sandbox","name":"sandbox.execution_requested","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_1"},"payload":{"profileId":"subprocess_default","backendKind":"subprocess","command":"echo","args":["hello"],"cwd":"/tmp/dope","requestedBy":"web-ui","resourceKind":"skill","resourceId":"shared","scope":"chat","status":"denied"}}`,
		"schemas/events/sandbox-decision-recorded.event.schema.json":   `{"eventId":"evt_sandbox_2","sequence":11,"category":"sandbox","name":"sandbox.decision_recorded","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_1"},"payload":{"decisionId":"sandbox_decision_1","resolution":"ask","selectionOutcome":"selected","matchedRules":["profile:subprocess_default"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","requiredBackendKind":"subprocess","hostStatus":"ready","explanation":"sandbox execution requires approval"}}`,
		"schemas/events/sandbox-execution-started.event.schema.json":   `{"eventId":"evt_sandbox_3","sequence":12,"category":"sandbox","name":"sandbox.execution_started","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_2"},"payload":{"profileId":"subprocess_default","backendKind":"subprocess","status":"running","startedAt":"2026-04-18T12:00:01Z"}}`,
		"schemas/events/sandbox-execution-completed.event.schema.json": `{"eventId":"evt_sandbox_4","sequence":13,"category":"sandbox","name":"sandbox.execution_completed","occurredAt":"2026-04-18T12:00:02Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_2"},"payload":{"profileId":"subprocess_default","backendKind":"subprocess","status":"completed","exitCode":0,"completedAt":"2026-04-18T12:00:02Z","outputTruncated":false,"partial":false}}`,
		"schemas/events/sandbox-execution-failed.event.schema.json":    `{"eventId":"evt_sandbox_5","sequence":14,"category":"sandbox","name":"sandbox.execution_failed","occurredAt":"2026-04-18T12:00:02Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_3"},"payload":{"profileId":"subprocess_default","backendKind":"subprocess","status":"failed","completedAt":"2026-04-18T12:00:02Z","outputTruncated":false,"partial":false,"errorClass":"process_failed","errorCode":"sandbox_process_failed","error":"exit status 1"}}`,
		"schemas/events/sandbox-execution-cancelled.event.schema.json": `{"eventId":"evt_sandbox_6","sequence":15,"category":"sandbox","name":"sandbox.execution_cancelled","occurredAt":"2026-04-18T12:00:02Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_4"},"payload":{"profileId":"subprocess_default","backendKind":"subprocess","status":"cancelled","completedAt":"2026-04-18T12:00:02Z","outputTruncated":false,"partial":false,"errorClass":"cancelled","errorCode":"sandbox_cancelled","error":"execution was cancelled"}}`,
		"schemas/events/sandbox-execution-denied.event.schema.json":    `{"eventId":"evt_sandbox_7","sequence":16,"category":"sandbox","name":"sandbox.execution_denied","occurredAt":"2026-04-18T12:00:02Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_1"},"payload":{"profileId":"subprocess_default","backendKind":"subprocess","status":"denied","completedAt":"2026-04-18T12:00:02Z","outputTruncated":false,"partial":false,"errorClass":"approval_required","errorCode":"sandbox_approval_required","error":"sandbox execution requires approval"}}`,
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
	h.mustValidateResponse(t, http.MethodPost, "/v1/chat/query", `{"provider":"echo","model":"echo-v1","skills":["shared"],"query":"hello chat"}`, h.authHeader, "schemas/api/chat-query.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/skills", "", h.authHeader, "schemas/api/skill-registry.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/skills/shared", "", h.authHeader, "schemas/api/skill-detail.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/skills/reload", "", h.authHeader, "schemas/api/skill-registry.response.schema.json")
	sandboxCwd := filepath.Join(os.TempDir(), "dope-contract")
	h.mustValidateResponse(t, http.MethodGet, "/v1/sandboxes/profiles", "", h.authHeader, "schemas/api/sandbox-profile-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/sandboxes/profiles/subprocess_default", "", h.authHeader, "schemas/api/sandbox-profile.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/sandboxes/explain", `{"command":"echo","args":["hello"],"cwd":"`+sandboxCwd+`","access":{"readRoots":["`+sandboxCwd+`"],"writeRoots":["`+sandboxCwd+`"],"networkMode":"full"}}`, h.authHeader, "schemas/api/sandbox-explain.response.schema.json")
	h.mustValidateResponse(t, http.MethodPost, "/v1/sandboxes/executions", `{"command":"echo","args":["hello"],"cwd":"`+sandboxCwd+`","access":{"readRoots":["`+sandboxCwd+`"],"writeRoots":["`+sandboxCwd+`"],"networkMode":"full"}}`, h.authHeader, "schemas/api/sandbox-execution.resource.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/sandboxes/executions", "", h.authHeader, "schemas/api/sandbox-execution-list.response.schema.json")

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
	h.request(t, http.MethodPost, "/v1/sandboxes/executions", `{"command":"`+contractSandboxCommand()+`","args":`+contractSandboxArgsJSON(`echo sandbox-ok`)+`}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/sandboxes/executions", `{"command":"echo","args":["sandbox-denied"],"access":{"networkMode":"full","allowedHosts":[],"allowedPorts":[]}}`, h.authHeader)
	waitForContractEvent(t, h.store, "sandbox.execution_denied")

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
		"sandbox.execution_requested":    "schemas/events/sandbox-execution-requested.event.schema.json",
		"sandbox.decision_recorded":      "schemas/events/sandbox-decision-recorded.event.schema.json",
		"sandbox.execution_denied":       "schemas/events/sandbox-execution-denied.event.schema.json",
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
			Metadata: map[string]string{
				"source":                "contract",
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
				"source":                "contract",
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "auth_status",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
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
			Metadata: map[string]string{
				"source":                "contract",
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "auth_status",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
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
			Metadata: map[string]string{
				"source":                "contract",
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
				"source":                "contract",
				"managedProviderId":     "codex_managed",
				"managedProviderAction": "logout",
				"sandboxProfileId":      "managed_provider_codex",
				"sandboxDecision":       "allow",
				"enforcementStrength":   "declared_only",
			},
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
	homeRoot := filepath.Join(t.TempDir(), ".agents")
	dataRoot := filepath.Join(t.TempDir(), "dope-data")
	writeContractSkillFile(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")
	writeContractSkillFile(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")
	writeContractSkillFile(t, filepath.Join(homeRoot, "skills", "shared", "SKILL.md"), strings.TrimSpace(`
---
name: shared
description: home skill
---
home instructions
`))
	writeContractSkillFile(t, filepath.Join(dataRoot, "skills", "shared", "SKILL.md"), strings.TrimSpace(`
---
name: shared
description: data skill
---
data instructions
`))
	writeContractSkillFile(t, filepath.Join(dataRoot, "skills", "shared", "assets", "guide.md"), "guide")
	skillRegistry, err := skills.NewRegistryWithRoots(homeRoot, dataRoot)
	if err != nil {
		t.Fatalf("NewRegistryWithRoots returned error: %v", err)
	}
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	chatService := chat.NewService(llmDispatcher, providerManager, skillRegistry, eventBus, sqliteStore)
	sandboxManager := sandbox.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     dataRoot,
	}, sqliteStore, eventBus, policyEngine)
	t.Cleanup(func() {
		if err := checkpointManager.Close(); err != nil {
			t.Fatalf("Close checkpoint manager returned error: %v", err)
		}
	})

	server := api.NewServer(api.Dependencies{
		Config: config.Config{
			BindAddr: "127.0.0.1:19191",
			DataDir:  dataRoot,
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
		Skills:       skillRegistry,
		Sandboxes:    sandboxManager,
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

func writeContractSkillFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func contractSandboxCommand() string {
	if goruntime.GOOS == "windows" {
		return "cmd"
	}
	return "/bin/sh"
}

func contractSandboxArgsJSON(script string) string {
	args := []string{"-c", script}
	if goruntime.GOOS == "windows" {
		args = []string{"/c", script}
	}
	payload, _ := json.Marshal(args)
	return string(payload)
}

func (h *contractHarness) mustValidateResponse(t *testing.T, method, path, body, authHeader, schemaPath string) {
	t.Helper()
	responseBody := h.request(t, method, path, body, authHeader)
	h.mustValidate(t, schemaPath, responseBody)
}

func waitForContractEvent(t *testing.T, sqliteStore *store.SQLiteStore, eventName string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		items, err := sqliteStore.ListEvents(context.Background(), events.Filter{})
		if err != nil {
			t.Fatalf("ListEvents returned error: %v", err)
		}
		for _, item := range items {
			if item.Name == eventName {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("expected event %s to be emitted", eventName)
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

func mustValidateFixtures(t *testing.T, validator *contracts.Validator, fixtures map[string]string) {
	t.Helper()

	for schemaPath, fixture := range fixtures {
		t.Run(filepath.Base(schemaPath), func(t *testing.T) {
			if err := validator.ValidateRelative(schemaPath, []byte(fixture)); err != nil {
				t.Fatalf("ValidateRelative returned error: %v", err)
			}
		})
	}
}

func assertFixtureSandboxDeclaration(t *testing.T, fixture string, consumerKind, consumerID, operationKind string) {
	t.Helper()

	sandboxView := fixtureSandboxView(t, fixture)
	declaration := sandboxView["declaration"].(map[string]any)
	if declaration["consumerKind"] != consumerKind || declaration["consumerId"] != consumerID || declaration["operationKind"] != operationKind {
		t.Fatalf("unexpected sandbox declaration %+v", declaration)
	}
}

func assertFixtureSandboxPolicyRecord(t *testing.T, fixture string, status, secretResolution string) {
	t.Helper()

	sandboxView := fixtureSandboxView(t, fixture)
	record := sandboxView["policyRecord"].(map[string]any)
	if record["status"] != status || record["secretResolution"] != secretResolution {
		t.Fatalf("unexpected sandbox policy record %+v", record)
	}
}

func assertFixtureExecutableSkillInspection(t *testing.T, fixture string, skillID, availabilityStatus, approvalMode string) {
	t.Helper()

	body := decodeJSONMap(t, []byte(fixture))
	if body["skillId"] != skillID {
		t.Fatalf("expected skillId %s, got %+v", skillID, body)
	}
	if body["availabilityStatus"] != availabilityStatus {
		t.Fatalf("expected availability %s, got %+v", availabilityStatus, body)
	}
	manifest, ok := body["executionManifest"].(map[string]any)
	if !ok {
		t.Fatalf("expected execution manifest, got %+v", body)
	}
	if manifest["approvalMode"] != approvalMode || manifest["profileId"] != "subprocess_default" {
		t.Fatalf("unexpected execution manifest %+v", manifest)
	}
}

func assertFixtureToolCallSandboxLinkage(t *testing.T, fixture string, invocationKind, consumerID, toolCallID, sandboxExecutionID string) {
	t.Helper()

	body := decodeJSONMap(t, []byte(fixture))
	if body["invocationKind"] != invocationKind || body["toolCallId"] != toolCallID || body["sandboxExecutionId"] != sandboxExecutionID {
		t.Fatalf("unexpected tool-call linkage %+v", body)
	}
	if body["skillId"] != consumerID && body["capabilityId"] != consumerID {
		t.Fatalf("expected consumer id %s, got %+v", consumerID, body)
	}
	assertFixtureSandboxDeclaration(t, fixture, invocationKind, consumerID, "tool_call.execute")
	assertFixtureSandboxPolicyRecord(t, fixture, "completed", "resolved")
}

func assertFixtureSandboxExecutionConsumerLinkage(t *testing.T, fixture string, consumerKind, consumerID, toolCallID, sandboxExecutionID string) {
	t.Helper()

	body := decodeJSONMap(t, []byte(fixture))
	if body["executionId"] != sandboxExecutionID || body["resourceId"] != consumerID || body["resourceKind"] != consumerKind {
		t.Fatalf("unexpected sandbox execution linkage %+v", body)
	}
	consumer, ok := body["consumer"].(map[string]any)
	if !ok {
		t.Fatalf("expected consumer view, got %+v", body)
	}
	declaration := consumer["declaration"].(map[string]any)
	if declaration["consumerKind"] != consumerKind || declaration["consumerId"] != consumerID {
		t.Fatalf("unexpected consumer declaration %+v", declaration)
	}
	record := consumer["policyRecord"].(map[string]any)
	if record["toolCallId"] != toolCallID || record["sandboxExecutionId"] != sandboxExecutionID {
		t.Fatalf("unexpected consumer policy record %+v", record)
	}
}

func fixtureSandboxView(t *testing.T, fixture string) map[string]any {
	t.Helper()

	body := decodeJSONMap(t, []byte(fixture))
	switch {
	case body["sandbox"] != nil:
		return body["sandbox"].(map[string]any)
	case body["consumer"] != nil:
		return body["consumer"].(map[string]any)
	case body["payload"] != nil:
		payload := body["payload"].(map[string]any)
		if payload["sandbox"] != nil {
			return payload["sandbox"].(map[string]any)
		}
		if payload["skillContracts"] != nil {
			items := payload["skillContracts"].([]any)
			if len(items) > 0 {
				return items[0].(map[string]any)
			}
		}
	case body["skillContracts"] != nil:
		items := body["skillContracts"].([]any)
		if len(items) > 0 {
			return items[0].(map[string]any)
		}
	}
	t.Fatalf("expected sandbox view in fixture %s", fixture)
	return nil
}

func schemaRootDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned no file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
}
