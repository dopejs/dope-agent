package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestEvaluationProductFixtureStoreRoundTrip(t *testing.T) {
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_fixture", PrincipalID: "prn_fixture"})
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	if err := s.SaveDiscoveryRun(ctx, evaluation.DiscoveryRun{
		DiscoveryRunID:       "discovery_run_fixture_store",
		TenantID:             "ten_fixture",
		Status:               evaluation.ProductStatusCompleted,
		SourceKinds:          []evaluation.SourceKind{evaluation.SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
		StartedAt:            now,
		UpdatedAt:            now,
	}); err != nil {
		t.Fatalf("SaveDiscoveryRun: %v", err)
	}
	candidate := evaluation.DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_fixture_store",
		TenantID:              "ten_fixture",
		DiscoveryRunID:        "discovery_run_fixture_store",
		SourceKind:            evaluation.SourceKindRun,
		SourceID:              "run_fixture_store",
		Score:                 0.9,
		ScoreBand:             evaluation.ScoreBandHigh,
		RedactionStatus:       evaluation.RedactionStatusRedacted,
		ReadinessStatus:       evaluation.ReadinessFullyReplayable,
		SuppressionState:      evaluation.SuppressionStateNone,
		RetentionState:        evaluation.RetentionStateActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	evidence := evaluation.CandidateEvidence{
		EvidenceID:             "evidence_fixture_store",
		TenantID:               "ten_fixture",
		DiscoveredCandidateID:  candidate.DiscoveredCandidateID,
		RedactedPayload:        map[string]any{"goal": "safe"},
		MaterializationAllowed: true,
		RetentionState:         evaluation.RetentionStateActive,
		CreatedAt:              now,
	}
	if err := s.SaveDiscoveredCandidate(ctx, candidate, evidence); err != nil {
		t.Fatalf("SaveDiscoveredCandidate: %v", err)
	}
	gotEvidence, ok, err := s.GetLatestCandidateEvidence(ctx, "ten_fixture", candidate.DiscoveredCandidateID)
	if err != nil {
		t.Fatalf("GetLatestCandidateEvidence: %v", err)
	}
	if !ok || gotEvidence.EvidenceID != evidence.EvidenceID {
		t.Fatalf("candidate evidence=%+v ok=%v, want latest evidence", gotEvidence, ok)
	}

	fixture := evaluation.ProductManagedFixture{
		FixtureID:         "product_fixture_store",
		TenantID:          "ten_fixture",
		DisplayName:       "Product Fixture",
		DomainClass:       evaluation.FixtureDomainSchedule,
		SourceKind:        string(evaluation.ProductResourceDiscoveredCandidate),
		SourceCandidateID: candidate.DiscoveredCandidateID,
		CurrentRevisionID: "revision_product_fixture_store_1",
		ReviewState:       evaluation.ProductStatusDraft,
		SuppressionState:  evaluation.SuppressionStateNone,
		RetentionState:    evaluation.RetentionStateActive,
		CreatedBy:         "prn_fixture",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	revision := evaluation.FixtureRevision{
		RevisionID:         fixture.CurrentRevisionID,
		FixtureID:          fixture.FixtureID,
		TenantID:           "ten_fixture",
		RevisionNumber:     1,
		FixturePayload:     map[string]any{"goal": "safe"},
		SourceEvidenceRefs: []string{evidence.EvidenceID},
		RedactionStatus:    evaluation.RedactionStatusRedacted,
		CreatedBy:          "prn_fixture",
		CreatedAt:          now,
	}
	if err := s.UpsertProductFixture(ctx, fixture); err != nil {
		t.Fatalf("UpsertProductFixture: %v", err)
	}
	if err := s.SaveFixtureRevision(ctx, revision); err != nil {
		t.Fatalf("SaveFixtureRevision: %v", err)
	}
	gotFixture, ok, err := s.GetProductFixture(ctx, "ten_fixture", fixture.FixtureID)
	if err != nil {
		t.Fatalf("GetProductFixture: %v", err)
	}
	if !ok || gotFixture.CurrentRevisionID != revision.RevisionID {
		t.Fatalf("fixture=%+v ok=%v, want current revision", gotFixture, ok)
	}
	revisions, err := s.ListFixtureRevisions(ctx, "ten_fixture", fixture.FixtureID, 10)
	if err != nil {
		t.Fatalf("ListFixtureRevisions: %v", err)
	}
	if len(revisions) != 1 || revisions[0].RevisionID != revision.RevisionID {
		t.Fatalf("revisions=%+v, want saved revision", revisions)
	}
}
