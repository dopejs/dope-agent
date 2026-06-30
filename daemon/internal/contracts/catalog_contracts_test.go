package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestCatalogSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/catalog-item-resource.schema.json":       `{"itemId":"catalog_item_1","kind":"skill","name":"pdf-extract","trustTier":"verified","permissions":["skills.manage"],"versions":[{"version":"1.0.0","source":"registry://pdf-extract","publishedAt":"2026-06-30T10:00:00Z"},{"version":"1.1.0","source":"registry://pdf-extract","requirements":[{"key":"sandbox_backend","description":"needs a sandbox"}],"publishedAt":"2026-06-30T10:00:00Z"}],"createdAt":"2026-06-30T10:00:00Z","updatedAt":"2026-06-30T10:00:00Z"}`,
		"schemas/api/catalog-enablement-resource.schema.json": `{"tenantId":"ten_a","itemId":"catalog_item_1","state":"enabled","activeVersion":"1.0.0","versionStack":["1.0.0"],"history":[{"action":"enabled","version":"1.0.0","actor":"op","occurredAt":"2026-06-30T10:00:01Z"}],"updatedAt":"2026-06-30T10:00:01Z"}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
