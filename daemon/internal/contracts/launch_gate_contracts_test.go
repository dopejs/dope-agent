package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestLaunchGateDecisionSchemaAcceptsCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/launch-gate-decision.schema.json": `{"result":"no_ship","reasons":["missing mail provider smoke entry"],"nonKnowledgeParityComplete":false,"gateStatement":"Context, knowledge, and memory work may begin only after non-knowledge parity release evidence passes or residual exceptions are explicitly accepted."}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
