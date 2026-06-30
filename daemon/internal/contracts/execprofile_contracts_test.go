package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestExecutionProfileSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/execution-profile-resource.schema.json":   `{"profile":{"profileId":"subprocess","name":"Subprocess Sandbox","backendKind":"subprocess","riskTier":"low","provides":["local_fs"],"description":"repo-owned subprocess sandbox","createdAt":"2026-06-30T10:00:00Z"},"status":{"profileId":"subprocess","health":"ready","available":true}}`,
		"schemas/api/execution-denial-explanation.schema.json": `{"requiredCapabilities":["docker"],"eligibleProfiles":[],"missingCapabilities":{"subprocess":["docker"]},"unavailable":{"docker":"backend unavailable"}}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
