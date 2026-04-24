package evaluation

import (
	"path/filepath"
	"testing"
)

func TestLoadRegressionFixturesRequiresRequiredDomainClasses(t *testing.T) {
	fixtures, err := LoadRegressionFixtures(filepath.Join("testdata", "fixtures"), "test")
	if err != nil {
		t.Fatalf("LoadRegressionFixtures returned error: %v", err)
	}

	seen := map[FixtureDomainClass]bool{}
	for _, fixture := range fixtures {
		seen[fixture.DomainClass] = true
		if fixture.FixtureID == "" || fixture.ManifestPath == "" {
			t.Fatalf("expected fixture identity and manifest path, got %+v", fixture)
		}
		if len(fixture.SourceRefs) == 0 || len(fixture.CapturedEvidenceRefs) == 0 {
			t.Fatalf("expected provenance and evidence refs for %+v", fixture)
		}
	}

	for _, domain := range []FixtureDomainClass{FixtureDomainSchedule, FixtureDomainIntegration, FixtureDomainComputerUse} {
		if !seen[domain] {
			t.Fatalf("expected fixture for domain %s", domain)
		}
	}
}
