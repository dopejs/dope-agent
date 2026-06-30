package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestTriageSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/create-triage-policy.request.schema.json": `{"name":"inbox","rules":[{"description":"newsletters","conditions":[{"field":"sender","operator":"contains","value":"newsletter"}],"classification":"newsletter","outcome":"delivery_digest"}],"defaultClassification":"fyi"}`,
		"schemas/api/triage-policy-resource.schema.json":       `{"policyId":"triage_policy_1","environmentScope":"test","name":"inbox","rules":[{"ruleId":"triage_rule_1","description":"urgent from boss","conditions":[{"field":"sender","operator":"contains","value":"boss@"},{"field":"subject","operator":"contains","value":"urgent"}],"classification":"urgent","outcome":"reminder"}],"defaultClassification":"fyi","createdAt":"2026-06-30T10:00:00Z","updatedAt":"2026-06-30T10:00:00Z"}`,
		"schemas/api/triage-run-resource.schema.json":          `{"runId":"triage_run_1","policyId":"triage_policy_1","environmentScope":"test","messageCount":2,"decisions":[{"messageId":"m1","classification":"urgent","matchedRuleId":"triage_rule_1","matchedEvidence":[{"field":"sender","operator":"contains","value":"boss@"}],"outcome":"reminder","replayCandidate":true,"decidedAt":"2026-06-30T10:00:01Z"},{"messageId":"m2","classification":"fyi","outcome":"no_action","defaultApplied":true,"replayCandidate":true,"decidedAt":"2026-06-30T10:00:01Z"}],"createdAt":"2026-06-30T10:00:01Z"}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
