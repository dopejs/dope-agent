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

func TestSkillSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/skill-file.schema.json":                      `{"path":"assets/guide.md","sizeBytes":42}`,
		"schemas/api/skill-summary.schema.json":                   `{"skillId":"shared","name":"shared","description":"data skill","source":"data_dir","rootPath":"/tmp/dope/skills","skillPath":"/tmp/dope/skills/shared","instructionPath":"/tmp/dope/skills/shared/SKILL.md","files":[{"path":"assets/guide.md","sizeBytes":42}],"frontmatter":{"name":"shared","description":"data skill"}}`,
		"schemas/api/skill-detail.response.schema.json":           `{"skillId":"shared","name":"shared","description":"data skill","source":"data_dir","rootPath":"/tmp/dope/skills","skillPath":"/tmp/dope/skills/shared","instructionPath":"/tmp/dope/skills/shared/SKILL.md","files":[{"path":"assets/guide.md","sizeBytes":42}],"frontmatter":{"name":"shared","description":"data skill"},"frontmatterRaw":"name: shared","body":"data instructions"}`,
		"schemas/api/skill-overlay.schema.json":                   `{"overlayId":"data_dir_agents","source":"data_dir","path":"/tmp/dope/AGENTS.md","sizeBytes":12,"modifiedAt":"2026-04-18T12:00:00Z"}`,
		"schemas/api/skill-registry.response.schema.json":         `{"loadedAt":"2026-04-18T12:00:00Z","items":[{"skillId":"shared","name":"shared","description":"data skill","source":"data_dir","rootPath":"/tmp/dope/skills","skillPath":"/tmp/dope/skills/shared","instructionPath":"/tmp/dope/skills/shared/SKILL.md","files":[{"path":"assets/guide.md","sizeBytes":42}],"frontmatter":{"name":"shared","description":"data skill"}}],"overlays":[{"overlayId":"home_agents","source":"home","path":"/tmp/home/.agents/AGENTS.md","sizeBytes":11,"modifiedAt":"2026-04-18T12:00:00Z"}]}`,
		"schemas/api/sandbox-profile.schema.json":                 `{"profileId":"subprocess_default","title":"Default Subprocess Sandbox","description":"Conservative local subprocess execution for the harness control plane.","backendKind":"subprocess","defaultWorkDir":"/tmp/dope-data","filesystemPolicy":{"mode":"scoped","readRoots":["/tmp/dope-data"],"writeRoots":["/tmp/dope-data"],"tempRoots":["/tmp"],"allowDataDir":true,"allowUserAgentsDir":true,"allowHomeRead":false,"allowHomeWrite":false},"networkPolicy":{"mode":"deny","allowedHosts":[],"allowedPorts":[],"allowLoopback":false,"enforcementMode":"declared_only"},"envPolicy":{"mode":"inherit_safe","allowedVars":["PATH"],"injectedVars":{"DOPE_DATA_DIR":"/tmp/dope-data"},"redactedVars":[]},"approvalPolicy":{"mode":"ask","requiredForCommands":["curl"],"requiredForWritesOutsideRoots":true,"requiredForNetwork":true,"requiredForUnknownBackends":true},"processPolicy":{"timeoutMs":30000,"maxTimeoutMs":300000,"killGraceMs":1000,"captureStdout":true,"captureStderr":true,"maxOutputBytes":65536,"allowStreaming":false,"restartOnFailure":false},"defaultTimeoutMs":30000,"maxTimeoutMs":300000,"restartable":false,"source":"builtin","active":true}`,
		"schemas/api/sandbox-profile-list.response.schema.json":   `{"items":[{"profileId":"subprocess_default","title":"Default Subprocess Sandbox","description":"Conservative local subprocess execution for the harness control plane.","backendKind":"subprocess","defaultWorkDir":"/tmp/dope-data","filesystemPolicy":{"mode":"scoped","readRoots":["/tmp/dope-data"],"writeRoots":["/tmp/dope-data"],"tempRoots":["/tmp"],"allowDataDir":true,"allowUserAgentsDir":true,"allowHomeRead":false,"allowHomeWrite":false},"networkPolicy":{"mode":"deny","allowedHosts":[],"allowedPorts":[],"allowLoopback":false,"enforcementMode":"declared_only"},"envPolicy":{"mode":"inherit_safe","allowedVars":["PATH"],"injectedVars":{"DOPE_DATA_DIR":"/tmp/dope-data"},"redactedVars":[]},"approvalPolicy":{"mode":"ask","requiredForCommands":["curl"],"requiredForWritesOutsideRoots":true,"requiredForNetwork":true,"requiredForUnknownBackends":true},"processPolicy":{"timeoutMs":30000,"maxTimeoutMs":300000,"killGraceMs":1000,"captureStdout":true,"captureStderr":true,"maxOutputBytes":65536,"allowStreaming":false,"restartOnFailure":false},"defaultTimeoutMs":30000,"maxTimeoutMs":300000,"restartable":false,"source":"builtin","active":true}]}`,
		"schemas/api/sandbox-decision.schema.json":                `{"decisionId":"sandbox_decision_1","executionId":"sandbox_exec_1","resolution":"ask","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","explanation":"sandbox execution requires approval"}`,
		"schemas/api/sandbox-result.schema.json":                  `{"executionId":"sandbox_exec_1","status":"denied","outputTruncated":false,"partial":false,"errorClass":"approval_required","errorCode":"sandbox_approval_required","error":"sandbox execution requires approval","backendMetadata":{"managedProviderId":"codex_managed","managedProviderAction":"prompt_execution","managedProviderOperationId":"managed_provider_op_1","enforcementStrength":"declared_only","sensitiveStateClasses":["config_file","temp_output"]}}`,
		"schemas/api/sandbox-execution.resource.schema.json":      `{"executionId":"sandbox_exec_1","profileId":"subprocess_default","backendKind":"subprocess","command":"echo","args":["hello"],"cwd":"/tmp/dope","envKeys":["HOME"],"stdinProvided":false,"timeoutMs":1000,"requestedBy":"web-ui","resourceKind":"skill","resourceId":"shared","scope":"chat","approvalId":"approval_1","reason":"inspect profile","metadata":{"ticket":"sandbox-16","managedProviderId":"codex_managed","managedProviderAction":"prompt_execution","managedProviderOperationId":"managed_provider_op_1","sandboxProfileId":"managed_provider_codex","sandboxDecision":"ask","enforcementStrength":"declared_only","sensitiveStateClasses":"config_file,temp_output"},"access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"networkMode":"allow_list","allowedHosts":["localhost"],"allowedPorts":[80],"allowLoopback":true},"status":"denied","decision":{"decisionId":"sandbox_decision_1","executionId":"sandbox_exec_1","resolution":"ask","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","explanation":"sandbox execution requires approval"},"result":{"executionId":"sandbox_exec_1","status":"denied","outputTruncated":false,"partial":false,"errorClass":"approval_required","errorCode":"sandbox_approval_required","error":"sandbox execution requires approval","backendMetadata":{"managedProviderId":"codex_managed","managedProviderAction":"prompt_execution","managedProviderOperationId":"managed_provider_op_1","enforcementStrength":"declared_only","sensitiveStateClasses":["config_file","temp_output"]}},"requestedAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z"}`,
		"schemas/api/sandbox-execution-list.response.schema.json": `{"items":[{"executionId":"sandbox_exec_1","profileId":"subprocess_default","backendKind":"subprocess","command":"echo","args":["hello"],"cwd":"/tmp/dope","envKeys":["HOME"],"stdinProvided":false,"timeoutMs":1000,"access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"networkMode":"allow_list","allowedHosts":["localhost"],"allowedPorts":[80],"allowLoopback":true},"status":"denied","decision":{"decisionId":"sandbox_decision_1","executionId":"sandbox_exec_1","resolution":"ask","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","explanation":"sandbox execution requires approval"},"result":{"executionId":"sandbox_exec_1","status":"denied","outputTruncated":false,"partial":false,"errorClass":"approval_required","errorCode":"sandbox_approval_required","error":"sandbox execution requires approval"},"requestedAt":"2026-04-18T12:00:00Z","updatedAt":"2026-04-18T12:00:00Z"}]}`,
		"schemas/api/sandbox-explain.response.schema.json":        `{"decision":{"decisionId":"sandbox_decision_1","resolution":"ask","matchedRules":["profile:subprocess_default","network:approval_required"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","explanation":"sandbox execution requires approval"}}`,
	}

	for schemaPath, fixture := range fixtures {
		t.Run(filepath.Base(schemaPath), func(t *testing.T) {
			if err := validator.ValidateRelative(schemaPath, []byte(fixture)); err != nil {
				t.Fatalf("ValidateRelative returned error: %v", err)
			}
		})
	}
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
		"schemas/events/sandbox-decision-recorded.event.schema.json":   `{"eventId":"evt_sandbox_2","sequence":11,"category":"sandbox","name":"sandbox.decision_recorded","occurredAt":"2026-04-18T12:00:01Z","scope":{},"resource":{"kind":"sandbox_execution","id":"sandbox_exec_1"},"payload":{"decisionId":"sandbox_decision_1","resolution":"ask","matchedRules":["profile:subprocess_default"],"approvalRequired":true,"approvalStatus":"pending","effectiveProfileId":"subprocess_default","effectiveBackendKind":"subprocess","explanation":"sandbox execution requires approval"}}`,
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

	body := decodeJSONMap(t, []byte(fixture))
	var sandboxView map[string]any
	switch {
	case body["sandbox"] != nil:
		sandboxView = body["sandbox"].(map[string]any)
	case body["payload"] != nil:
		payload := body["payload"].(map[string]any)
		if payload["sandbox"] != nil {
			sandboxView = payload["sandbox"].(map[string]any)
		} else if payload["skillContracts"] != nil {
			items := payload["skillContracts"].([]any)
			if len(items) > 0 {
				sandboxView = items[0].(map[string]any)
			}
		}
	case body["skillContracts"] != nil:
		items := body["skillContracts"].([]any)
		if len(items) > 0 {
			sandboxView = items[0].(map[string]any)
		}
	}
	if sandboxView == nil {
		t.Fatalf("expected sandbox view in fixture %s", fixture)
	}
	declaration := sandboxView["declaration"].(map[string]any)
	if declaration["consumerKind"] != consumerKind || declaration["consumerId"] != consumerID || declaration["operationKind"] != operationKind {
		t.Fatalf("unexpected sandbox declaration %+v", declaration)
	}
}

func assertFixtureSandboxPolicyRecord(t *testing.T, fixture string, status, secretResolution string) {
	t.Helper()

	body := decodeJSONMap(t, []byte(fixture))
	sandboxView := body["sandbox"].(map[string]any)
	record := sandboxView["policyRecord"].(map[string]any)
	if record["status"] != status || record["secretResolution"] != secretResolution {
		t.Fatalf("unexpected sandbox policy record %+v", record)
	}
}

func schemaRootDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller returned no file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
}
