package contracts_test

import (
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func setupWizardContractFixtures() map[string]string {
	readySession := `{"setupSessionId":"setup_ten_personal_provider_openai_compatible","tenantId":"ten_personal","actorPrincipalId":"prn_1","targetId":"provider.openai_compatible","targetKind":"provider","setupStyle":"submitted_secret","state":"ready","reasonCode":"healthy","retryable":false,"remediationOwner":"none_required","safeUseMode":"normal","allowedCapabilities":[],"currentAttemptId":"attempt_1","diagnosticResultId":"diag_openai_setup","diagnosticRunId":"diag_run_openai_setup","diagnosticStage":"credential_probe","diagnosticSourceKind":"provider_check","diagnosticSourceId":"provider.openai_compatible","redactionStatus":"redacted","resourceRefs":[{"kind":"tenant_secret","id":"provider/openai-compatible","route":"/v1/tenant-secrets/provider%2Fopenai-compatible"}],"redactedEvidence":{"secretRef":"provider/openai-compatible","secretVersionId":"secver_1"},"createdAt":"2026-05-06T00:00:00Z","updatedAt":"2026-05-06T00:01:00Z","lastTransitionAt":"2026-05-06T00:01:00Z","lastTransitionAuditEventId":"audit_setup_1"}`
	return map[string]string{
		"schemas/api/setup-target-list.response.schema.json":     `{"items":[{"targetId":"integration.feishu_lark","tenantId":"ten_personal","targetKind":"integration","setupStyle":"oauth","displayName":"Feishu/Lark OAuth","proofTarget":true,"supportStatus":"supported","requiredPermissions":["secrets.manage","integrations.manage"],"limitedSafeCapabilities":["metadata_read"],"currentSessionId":"setup_ten_personal_integration_feishu_lark","currentState":"action_required","diagnosticResultId":"diag_lark_setup"},{"targetId":"provider.openai_compatible","tenantId":"ten_personal","targetKind":"provider","setupStyle":"submitted_secret","displayName":"OpenAI-compatible provider","proofTarget":true,"supportStatus":"supported","requiredPermissions":["secrets.manage","integrations.manage"],"limitedSafeCapabilities":["metadata_read"],"currentSessionId":"setup_ten_personal_provider_openai_compatible","currentState":"ready","diagnosticResultId":"diag_openai_setup"}]}`,
		"schemas/api/setup-session-resource.schema.json":         readySession,
		"schemas/api/setup-session.response.schema.json":         `{"session":` + readySession + `}`,
		"schemas/api/setup-session-list.response.schema.json":    `{"items":[` + readySession + `]}`,
		"schemas/api/setup-secret-submit.request.schema.json":    `{"secretRef":"provider/openai-compatible","value":"R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK","displayName":"OpenAI-compatible API key"}`,
		"schemas/api/setup-oauth-start.request.schema.json":      `{"redirectRoute":"/setup/oauth/feishu-lark/callback"}`,
		"schemas/api/setup-oauth-callback.request.schema.json":   `{"state":"oauth_state_ref_1","result":"denied","accountLabel":"tenant workspace"}`,
		"schemas/api/setup-diagnostic-list.response.schema.json": `{"items":[{"setupSessionId":"setup_ten_personal_integration_feishu_lark","targetId":"integration.feishu_lark","diagnosticResultId":"diag_lark_scope","diagnosticRunId":"diag_run_1","diagnosticStage":"oauth_probe","diagnosticSourceKind":"integration_diagnostic","diagnosticSourceId":"integration.feishu_lark","status":"action_required","reasonCode":"scope_missing","retrySafety":"retryable","remediationOwner":"tenant_admin","allowedCapabilities":[],"checkedAt":"2026-05-06T00:02:00Z","staleAfter":"2026-05-06T00:12:00Z","redactionStatus":"redacted"}]}`,
		"schemas/api/setup-error.response.schema.json":           `{"error":"setup permission denied","code":"setup_denied:missing_permission","reasonCode":"setup_denied:missing_permission","stage":"permission","retryable":false,"remediationOwner":"tenant_admin"}`,
	}
}

func setupWizardRedactedOutputFixtures() map[string]string {
	fixtures := setupWizardContractFixtures()
	delete(fixtures, "schemas/api/setup-secret-submit.request.schema.json")
	return fixtures
}

func TestSetupWizardSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, setupWizardContractFixtures())
}

func TestSetupWizardRedactedOutputFixturesExcludeForbiddenCredentialMaterial(t *testing.T) {
	t.Parallel()

	for name, fixture := range setupWizardRedactedOutputFixtures() {
		lower := strings.ToLower(fixture)
		for _, forbidden := range []string{
			"r46_fake_openai_compatible_key_do_not_leak",
			"authorizationcode",
			"authorization_code",
			"access_token",
			"refreshtoken",
			"refresh_token",
			"bearer ",
			"callbackpayload",
			"client_secret",
		} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s contains forbidden credential or OAuth material marker %q: %s", name, forbidden, fixture)
			}
		}
	}
}
