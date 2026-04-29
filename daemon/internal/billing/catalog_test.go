package billing

import (
	"testing"
	"time"
)

func TestInitialCatalogCoversRequiredCategories(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	entries := InitialCatalog(now)
	if len(entries) != len(RequiredCategories()) {
		t.Fatalf("catalog size=%d, want %d", len(entries), len(RequiredCategories()))
	}
	seen := map[Category]CatalogEntry{}
	for _, entry := range entries {
		if entry.Definition.Category == "" {
			t.Fatal("catalog entry missing category")
		}
		if entry.Definition.PeriodAnchor != PeriodAnchorUTC {
			t.Fatalf("%s period anchor=%q", entry.Definition.Category, entry.Definition.PeriodAnchor)
		}
		if entry.Definition.DenialReasonCode == "" {
			t.Fatalf("%s missing denial reason", entry.Definition.Category)
		}
		if entry.OperationKeyShape == "" || entry.ConcurrencyGuard == "" || len(entry.RequiredTests) == 0 {
			t.Fatalf("%s missing contract metadata: %+v", entry.Definition.Category, entry)
		}
		seen[entry.Definition.Category] = entry
	}
	for _, category := range RequiredCategories() {
		if _, ok := seen[category]; !ok {
			t.Fatalf("missing category %s", category)
		}
	}
	artifact := seen[CategoryArtifactStorageBytes]
	if !artifact.OverLimitCommit || !artifact.FutureDenialOnOver {
		t.Fatalf("artifact storage bytes must encode over-limit commit and future denial: %+v", artifact)
	}
}
