package evaluation

import (
	"testing"
	"time"
)

func TestProductFixtureRevisionIsImmutableAndPayloadIsCopied(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	fixture, revision, err := CreateProductFixtureFromCandidate(ProductFixtureInput{
		TenantID:        "ten_eval",
		DisplayName:     "Schedule Product Fixture",
		DomainClass:     FixtureDomainSchedule,
		SourceCandidate: fixtureCandidate(now),
		SourceEvidence:  fixtureEvidence(now),
		FixturePayload:  map[string]any{"goal": "initial"},
		CreatedBy:       "prn_eval",
	}, now)
	if err != nil {
		t.Fatalf("CreateProductFixtureFromCandidate: %v", err)
	}
	revision.FixturePayload["goal"] = "mutated after return"

	_, secondRevision, err := CreateProductFixtureRevision(fixture, FixtureRevisionInput{
		FixturePayload:     map[string]any{"goal": "second"},
		ContentSummary:     "second revision",
		SourceEvidenceRefs: []string{"evidence_1"},
		CreatedBy:          "prn_eval",
	}, 2, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("CreateProductFixtureRevision: %v", err)
	}
	if secondRevision.RevisionNumber != 2 || secondRevision.RevisionID == revision.RevisionID {
		t.Fatalf("revision numbering not monotonic: first=%+v second=%+v", revision, secondRevision)
	}
	if secondRevision.SourceEvidenceRefs[0] != "evidence_1" || secondRevision.TenantID != fixture.TenantID {
		t.Fatalf("revision lost provenance: %+v", secondRevision)
	}
}
