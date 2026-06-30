package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestEvidenceBundleSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/generate-evidence-bundle.request.schema.json": `{"actor":"support@dope","scope":{"kind":"routine","ref":"routine_1"}}`,
		"schemas/api/evidence-bundle-resource.schema.json":         `{"bundleId":"evidence_bundle_1","tenantId":"ten_a","actor":"support@dope","scope":{"kind":"routine","ref":"routine_1"},"sections":[{"kind":"routine","resourceRefs":["routine_1"],"summary":{"state":"failed","accessToken":"[redacted]"},"links":["/v1/routines/routine_1"]}],"redactionStatus":"redacted","createdAt":"2026-06-30T10:00:00Z","retentionExpiresAt":"2026-07-14T10:00:00Z"}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
