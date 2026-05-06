package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func activationContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/activation-state-resource.schema.json":           `{"activationId":"act_1","principalId":"prn_1","tenantId":"ten_personal","environmentScope":"test","status":"active","currentStepId":"test_chat","completedStepIds":["tenant_resolved","quota_baseline_ready"],"blockingReasonCodes":[],"readinessItems":[],"quotaBaseline":{"tenantId":"ten_personal","planKey":"free","enforcementMode":"enforced","status":"available","quotas":[]},"firstAction":{"actionId":"test_chat","actionKind":"test_chat","recommended":true,"available":true,"blockingItemIds":[],"invokeRoute":"/v1/activation/test-chat","resultRoute":"/v1/activation"},"lastEvaluatedAt":"2026-05-06T00:00:00Z"}`,
		"schemas/api/activation.response.schema.json":                 `{"activation":{"activationId":"act_1","principalId":"prn_1","tenantId":"ten_personal","environmentScope":"test","status":"active","currentStepId":"test_chat","completedStepIds":["tenant_resolved","quota_baseline_ready"],"blockingReasonCodes":[],"readinessItems":[],"quotaBaseline":{"tenantId":"ten_personal","planKey":"free","enforcementMode":"enforced","status":"available","quotas":[]},"firstAction":{"actionId":"test_chat","actionKind":"test_chat","recommended":true,"available":true,"blockingItemIds":[],"invokeRoute":"/v1/activation/test-chat","resultRoute":"/v1/activation"},"lastEvaluatedAt":"2026-05-06T00:00:00Z"}}`,
		"schemas/api/activation-test-chat.request.schema.json":        `{"message":"Run a safe hosted activation test."}`,
		"schemas/api/activation-test-chat.response.schema.json":       `{"activation":{"activationId":"act_1","principalId":"prn_1","tenantId":"ten_personal","environmentScope":"test","status":"first_action_completed","currentStepId":"completed","completedStepIds":["tenant_resolved","quota_baseline_ready","test_chat_completed"],"blockingReasonCodes":[],"readinessItems":[],"quotaBaseline":{"tenantId":"ten_personal","planKey":"free","enforcementMode":"enforced","status":"available","quotas":[]},"firstAction":{"actionId":"test_chat","actionKind":"test_chat","recommended":true,"available":true,"blockingItemIds":[],"invokeRoute":"/v1/activation/test-chat","resultRoute":"/v1/activation"},"firstActionCompletedAt":"2026-05-06T00:00:00Z","lastEvaluatedAt":"2026-05-06T00:00:00Z"},"testChat":{"dispatchId":"dispatch_1","status":"completed","provider":"test","model":"test-chat","finishReason":"stop","usage":{},"completedAt":"2026-05-06T00:00:00Z"}}`,
		"schemas/api/activation-diagnostic-list.response.schema.json": `{"items":[{"activationId":"act_1","tenantId":"ten_personal","principalId":"prn_1","status":"blocked","stage":"quota_baseline","reasonCode":"activation_blocked:quota_baseline_unavailable","retryable":true,"remediationOwner":"operator","lastTransitionAt":"2026-05-06T00:00:00Z","readinessItemIds":["quota-baseline"],"quotaBaselineStatus":"unavailable"}]}`,
	}
}

func TestActivationSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, activationContractFixtures())
}

func TestActivationContractAcceptsBlockedQuotaReadinessResponse(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixture := `{"activation":{"activationId":"act_1","principalId":"prn_1","tenantId":"ten_personal","environmentScope":"prod","status":"blocked","currentStepId":"quota_baseline","completedStepIds":["tenant_resolved"],"blockingReasonCodes":["activation_blocked:quota_baseline_unavailable"],"readinessItems":[{"itemId":"tenant-access","itemKind":"tenant_access","status":"ready","displayName":"Tenant access","requiredForActivation":true,"retryable":false,"remediationOwner":"none_required","updatedAt":"2026-05-06T00:00:00Z"},{"itemId":"environment","itemKind":"environment","status":"ready","displayName":"Hosted environment","requiredForActivation":true,"retryable":false,"remediationOwner":"none_required","updatedAt":"2026-05-06T00:00:00Z"},{"itemId":"quota-baseline","itemKind":"quota_baseline","status":"blocked","reasonCode":"activation_blocked:quota_baseline_unavailable","displayName":"Quota baseline","requiredForActivation":true,"retryable":true,"remediationOwner":"operator","updatedAt":"2026-05-06T00:00:00Z"}],"quotaBaseline":{"tenantId":"ten_personal","planKey":"unknown","enforcementMode":"not_measurable","status":"unavailable","reasonCode":"activation_blocked:quota_baseline_unavailable","quotas":[]},"firstAction":{"actionId":"test_chat","actionKind":"test_chat","recommended":true,"available":false,"blockingItemIds":["quota-baseline"],"invokeRoute":"/v1/activation/test-chat","resultRoute":"/v1/activation"},"failureReason":{"reasonCode":"activation_blocked:quota_baseline_unavailable","stage":"quota_baseline","retryable":true,"remediationOwner":"operator"},"lastEvaluatedAt":"2026-05-06T00:00:00Z"}}`
	if err := validator.ValidateRelative("schemas/api/activation.response.schema.json", []byte(fixture)); err != nil {
		t.Fatalf("ValidateRelative returned error: %v", err)
	}
}

func TestActivationContractFixturesExcludeForbiddenTestChatFields(t *testing.T) {
	t.Parallel()

	for schemaPath, fixture := range activationContractFixtures() {
		var value any
		if err := json.Unmarshal([]byte(fixture), &value); err != nil {
			t.Fatalf("decode %s: %v", schemaPath, err)
		}
		for _, forbidden := range []string{"query", "reply", "transcript", "delta", "prompt", "rawProviderPayload", "authorization", "accessToken", "refreshToken", "secret"} {
			if containsActivationForbiddenField(value, forbidden) {
				t.Fatalf("%s contains forbidden field %q", schemaPath, forbidden)
			}
		}
		if strings.Contains(strings.ToLower(fixture), "authorization") || strings.Contains(strings.ToLower(fixture), "transcript") {
			t.Fatalf("%s contains forbidden activation evidence text", schemaPath)
		}
	}
}

func containsActivationForbiddenField(value any, field string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if strings.EqualFold(key, field) {
				return true
			}
			if containsActivationForbiddenField(item, field) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsActivationForbiddenField(item, field) {
				return true
			}
		}
	}
	return false
}
