package contracts_test

import (
	"net/http"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func operatorContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/operator-readiness-item.schema.json":           `{"itemId":"integration-calendar-a","itemKind":"integration","resourceId":"calendar-a","displayName":"Calendar A","status":"degraded","healthState":"degraded","reason":"token refresh is required","requiredOperatorAction":"Refresh the calendar integration.","requiredForSelectedAction":false,"detailRoute":"/v1/integrations/calendar-a","environmentScope":"test","updatedAt":"2026-04-24T10:00:00Z"}`,
		"schemas/api/operator-first-use-action.schema.json":         `{"actionId":"test_run","actionKind":"test_run","displayName":"Launch test run","recommended":true,"available":true,"blockingItemIds":[],"summary":"Persist a bounded shell smoke action.","invokeRoute":"/v1/runs","resultRoute":"/v1/runs"}`,
		"schemas/api/operator-onboarding.response.schema.json":      `{"environmentScope":"test","status":"ready_for_action","currentStepId":"run-first-action","completedStepIds":["auth-ready"],"blockingItemIds":[],"optionalFollowUpItemIds":["integration-calendar-a"],"recommendedActionId":"test_run","readinessItems":[{"itemId":"auth-token","itemKind":"auth","resourceId":"token_1","displayName":"Operator access token","status":"ready","reason":"Authenticated shell session is active.","requiredForSelectedAction":true,"detailRoute":"/v1/auth/me","environmentScope":"test","updatedAt":"2026-04-24T10:00:00Z"},{"itemId":"integration-calendar-a","itemKind":"integration","resourceId":"calendar-a","displayName":"Calendar A","status":"degraded","healthState":"degraded","reason":"token refresh is required","requiredOperatorAction":"Refresh the calendar integration.","requiredForSelectedAction":false,"detailRoute":"/v1/integrations/calendar-a","environmentScope":"test","updatedAt":"2026-04-24T10:00:00Z"}],"firstUsefulActions":[{"actionId":"test_run","actionKind":"test_run","displayName":"Launch test run","recommended":true,"available":true,"blockingItemIds":[],"summary":"Persist a bounded shell smoke action.","invokeRoute":"/v1/runs","resultRoute":"/v1/runs"}],"lastEvaluatedAt":"2026-04-24T10:00:00Z"}`,
		"schemas/api/operator-activity-record.schema.json":          `{"activityId":"delivery-delivery_1","sourceKind":"delivery","sourceId":"delivery_1","title":"Delivery failure","status":"failed","summary":"Source workflow | target transport failed","attentionLevel":"critical","occurredAt":"2026-04-24T10:05:00Z","detailRoute":"/v1/deliveries/delivery_1","relatedResourceRefs":[{"kind":"run","id":"run_1","route":"/v1/runs/run_1"},{"kind":"workflow","id":"wf_1","route":"/v1/runs/run_1/workflows/wf_1"}],"environmentScope":"test"}`,
		"schemas/api/operator-activity-list.response.schema.json":   `{"environmentScope":"test","items":[{"activityId":"delivery-delivery_1","sourceKind":"delivery","sourceId":"delivery_1","title":"Delivery failure","status":"failed","summary":"Source workflow | target transport failed","attentionLevel":"critical","occurredAt":"2026-04-24T10:05:00Z","detailRoute":"/v1/deliveries/delivery_1","relatedResourceRefs":[{"kind":"run","id":"run_1","route":"/v1/runs/run_1"},{"kind":"workflow","id":"wf_1","route":"/v1/runs/run_1/workflows/wf_1"}],"environmentScope":"test"}],"generatedAt":"2026-04-24T10:05:00Z"}`,
		"schemas/api/operator-diagnostic-finding.schema.json":       `{"findingId":"delivery-delivery_1","sourceKind":"delivery","sourceId":"delivery_1","plane":"delivery","severity":"critical","status":"failed","reason":"Delivery transport exhausted retries.","recommendedAction":"Inspect delivery attempts and target state.","detailRoute":"/v1/deliveries/delivery_1","relatedResourceRefs":[{"kind":"run","id":"run_1","route":"/v1/runs/run_1"},{"kind":"workflow","id":"wf_1","route":"/v1/runs/run_1/workflows/wf_1"}],"environmentScope":"test","capturedAt":"2026-04-24T10:06:00Z"}`,
		"schemas/api/operator-diagnostic-list.response.schema.json": `{"environmentScope":"test","items":[{"findingId":"delivery-delivery_1","sourceKind":"delivery","sourceId":"delivery_1","plane":"delivery","severity":"critical","status":"failed","reason":"Delivery transport exhausted retries.","recommendedAction":"Inspect delivery attempts and target state.","detailRoute":"/v1/deliveries/delivery_1","relatedResourceRefs":[{"kind":"run","id":"run_1","route":"/v1/runs/run_1"},{"kind":"workflow","id":"wf_1","route":"/v1/runs/run_1/workflows/wf_1"}],"environmentScope":"test","capturedAt":"2026-04-24T10:06:00Z"}],"generatedAt":"2026-04-24T10:06:00Z"}`,
	}
}

func TestOperatorSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, operatorContractFixtures())
}

func TestOperatorRoutesMatchSchemas(t *testing.T) {
	t.Parallel()

	h := newContractHarness(t)

	startPairing := decodeJSONMap(t, h.request(t, http.MethodPost, "/v1/auth/pairings/start", `{"mode":"local","label":"operator-contract-web"}`, ""))
	pairing := startPairing["pairing"].(map[string]any)
	completePairing := decodeJSONMap(t, h.request(t, http.MethodPost, "/v1/auth/pairings/"+pairing["pairingId"].(string)+"/complete", `{"code":"`+startPairing["pairingCode"].(string)+`"}`, ""))
	h.authHeader = "Bearer " + completePairing["accessToken"].(string)

	h.request(t, http.MethodPost, "/v1/integrations", `{"integrationId":"calendar-operator","domainKind":"calendar","displayName":"Calendar Operator","backendKind":"fake_local","accountBinding":{"accountKey":"acct_calendar"},"canonicalDefault":true}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/integrations/calendar-operator/readiness", `{"readinessStatus":"degraded","authState":"authorized","healthState":"degraded","reason":"reauth required","requiredOperatorAction":"Refresh auth","secretResolution":"resolved"}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/policy/approvals", `{"action":"workflow.launch","resourceKind":"workflow","resourceId":"wf_operator","reason":"operator review","requestedBy":"operator-contract"}`, h.authHeader)
	h.request(t, http.MethodPost, "/v1/runs", `{"entrypoint":"operator.shell.test","goal":"contract operator smoke"}`, h.authHeader)

	h.mustValidateResponse(t, http.MethodGet, "/v1/operator/onboarding", "", h.authHeader, "schemas/api/operator-onboarding.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/operator/activity?attentionOnly=true&limit=10", "", h.authHeader, "schemas/api/operator-activity-list.response.schema.json")
	h.mustValidateResponse(t, http.MethodGet, "/v1/operator/diagnostics?severity=warning", "", h.authHeader, "schemas/api/operator-diagnostic-list.response.schema.json")
}
