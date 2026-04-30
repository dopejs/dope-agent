package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func evaluationProductFoundationFixtures() map[string]string {
	return map[string]string{
		"schemas/api/evaluation-product-pagination.schema.json":                      `{"applicationId":"retention_1","tenantId":"ten_eval","resourceKind":"discovered_candidate","resourceId":"candidate_1","dryRun":false,"outcome":"expired","affectedCount":1,"appliedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/evaluation-discovery-policy-resource.schema.json":               `{"policyId":"policy_1","tenantId":"ten_eval","enabled":true,"sourceKinds":["run"],"windowStart":"2026-04-29T09:00:00Z","windowEnd":"2026-04-29T10:00:00Z","maxInspectedRecords":10,"maxEmittedCandidates":2,"costBudget":5,"createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/evaluation-discovery-run-resource.schema.json":                  `{"discoveryRunId":"discovery_run_1","tenantId":"ten_eval","policyId":"policy_1","status":"partial","cursor":"cur_1","sourceKinds":["run"],"windowStart":"2026-04-29T09:00:00Z","windowEnd":"2026-04-29T10:00:00Z","maxInspectedRecords":10,"maxEmittedCandidates":2,"costBudget":5,"inspectedRecords":10,"emittedCandidates":1,"partialReason":"max_inspected_records","startedAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:01:00Z"}`,
		"schemas/api/evaluation-discovered-candidate-resource.schema.json":           `{"discoveredCandidateId":"candidate_1","tenantId":"ten_eval","discoveryRunId":"discovery_run_1","sourceKind":"run","sourceId":"run_1","sourceRefs":[{"kind":"run","id":"run_1","route":"/v1/runs/run_1"}],"score":0.9,"scoreBand":"high","redactionStatus":"redacted","evidenceRef":"evidence_1","readinessStatus":"fully_replayable","suppressionState":"none","retentionState":"active","createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/evaluation-product-fixture-resource.schema.json":                `{"fixture":{"fixtureId":"product_fixture_1","tenantId":"ten_eval","displayName":"Schedule Product Fixture","domainClass":"schedule","sourceKind":"discovered_candidate","sourceCandidateId":"candidate_1","currentRevisionId":"revision_1","reviewState":"draft","suppressionState":"none","retentionState":"active","createdBy":"prn_eval","createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"},"revision":{"revisionId":"revision_1","fixtureId":"product_fixture_1","tenantId":"ten_eval","revisionNumber":1,"fixturePayload":{"goal":"safe"},"sourceEvidenceRefs":["evidence_1"],"redactionStatus":"redacted","createdBy":"prn_eval","createdAt":"2026-04-29T10:00:00Z"}}`,
		"schemas/api/evaluation-campaign-resource.schema.json":                       `{"attemptGroupId":"attempt_group_1","campaignId":"campaign_1","campaignItemId":"campaign_item_1","tenantId":"ten_eval","replayAttemptIds":["attempt_1"],"comparisonIds":["comparison_1"],"liveValidationIds":["ledger_1"],"status":"completed","driftCount":1,"failureCount":0,"unsupportedCount":0,"operatorActionNeededCount":1,"summary":"1 drift","createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/evaluation-dashboard-resource.schema.json":                      `{"projectionId":"projection_1","tenantId":"ten_eval","windowStart":"2026-04-29T09:00:00Z","windowEnd":"2026-04-29T10:00:00Z","campaignStatusCounts":{"completed":1},"driftSummary":{"total":1},"failureSummary":{"total":0},"unsupportedSummary":{"total":0},"operatorActionNeededSummary":{"total":1},"liveValidationSummary":{"linked":1},"generatedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/api/evaluation-tool-call-inspection-resource.schema.json":           `{"inspectionId":"inspection_1","tenantId":"ten_eval","campaignId":"campaign_1","campaignItemId":"campaign_item_1","toolCallRef":"tool_call_1","originalEvidenceRef":"original_1","nonLiveReplayEvidenceRef":"replay_1","liveValidationLedgerRefs":["ledger_1"],"classification":"live_validation_completed","diffSummary":"redacted matched","redactionStatus":"redacted","createdAt":"2026-04-29T10:00:00Z","updatedAt":"2026-04-29T10:00:00Z"}`,
		"schemas/events/evaluation-product-audit-recorded.event.schema.json":         `{"eventId":"evt_eval_product_1","sequence":1,"category":"evaluation","name":"evaluation.product_retention_applied","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"retention_application","id":"retention_1"},"payload":{"tenantId":"ten_eval","actorId":"prn_eval","action":"retention.apply","targetKind":"discovered_candidate","targetId":"candidate_1","outcome":"retention_applied","retentionApplicationId":"retention_1","createdAt":"2026-04-29T10:00:00Z"}}`,
		"schemas/events/evaluation-discovery-started.event.schema.json":              `{"eventId":"evt_eval_discovery_1","sequence":1,"category":"evaluation","name":"evaluation.discovery_started","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"discovery_run","id":"discovery_run_1"},"payload":{"tenantId":"ten_eval","policyId":"policy_1","discoveryRunId":"discovery_run_1","status":"queued"}}`,
		"schemas/events/evaluation-fixture-created.event.schema.json":                `{"eventId":"evt_eval_fixture_1","sequence":1,"category":"evaluation","name":"evaluation.fixture.created","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"product_fixture","id":"product_fixture_1"},"payload":{"tenantId":"ten_eval","actorId":"prn_eval","fixtureId":"product_fixture_1","revisionId":"revision_1","sourceCandidateId":"candidate_1","sourceEvidenceRefs":["evidence_1"],"outcome":"created"}}`,
		"schemas/events/evaluation-campaign-created.event.schema.json":               `{"eventId":"evt_eval_campaign_1","sequence":1,"category":"evaluation","name":"evaluation.campaign.created","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"campaign","id":"campaign_1"},"payload":{"tenantId":"ten_eval","actorId":"prn_eval","campaignId":"campaign_1","status":"queued","outcome":"created"}}`,
		"schemas/events/evaluation-dashboard-projection-generated.event.schema.json": `{"eventId":"evt_eval_dashboard_1","sequence":1,"category":"evaluation","name":"evaluation.dashboard.projection_generated","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"dashboard_projection","id":"projection_1"},"payload":{"tenantId":"ten_eval","projectionId":"projection_1","windowStart":"2026-04-29T09:00:00Z","windowEnd":"2026-04-29T10:00:00Z","generatedAt":"2026-04-29T10:00:00Z","outcome":"generated"}}`,
		"schemas/events/evaluation-tool-call-inspection-generated.event.schema.json": `{"eventId":"evt_eval_inspection_1","sequence":1,"category":"evaluation","name":"evaluation.tool_call_inspection.generated","occurredAt":"2026-04-29T10:00:00Z","scope":{},"resource":{"kind":"tool_call_inspection","id":"inspection_1"},"payload":{"tenantId":"ten_eval","inspectionId":"inspection_1","campaignId":"campaign_1","campaignItemId":"campaign_item_1","classification":"matched","redactionStatus":"clean","outcome":"generated"}}`,
	}
}

func TestEvaluationProductFoundationSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, evaluationProductFoundationFixtures())
}

func TestEvaluationProductPlanningContractsReferenceFoundationRoutes(t *testing.T) {
	t.Parallel()

	root := contractRepoRoot(t)
	checks := map[string][]string{
		"specs/026-evaluation-product-expansion/contracts/candidate-discovery.md": {
			"GET /v1/evaluation/discovery-policies",
			"POST /v1/evaluation/discovery-runs",
			"POST /v1/evaluation/retention/apply",
		},
		"specs/026-evaluation-product-expansion/contracts/campaign-dashboard.md": {
			"GET /v1/evaluation/dashboard",
		},
		"specs/026-evaluation-product-expansion/contracts/tool-call-inspection.md": {
			"GET /v1/evaluation/campaigns/{campaignId}/tool-call-inspections",
		},
		"specs/026-evaluation-product-expansion/spec.md": {
			"retention",
			"deletion",
			"redaction",
		},
	}
	for rel, needles := range checks {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(body)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s missing contract phrase %q", rel, needle)
			}
		}
	}
}
