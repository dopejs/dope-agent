package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func TestEvaluationRecordsPersistAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dataDir := filepath.Join(t.TempDir(), "dope-data")
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)

	sqliteStore, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	candidate := evaluation.ReplayCandidate{
		CandidateID:       "candidate_1",
		CandidateKind:     evaluation.CandidateKindFixture,
		DisplayName:       "Candidate 1",
		SourceKind:        evaluation.SourceKindFixture,
		SourceID:          "fixture_1",
		EnvironmentScope:  "test",
		ReadinessStatus:   evaluation.ReadinessFullyReplayable,
		DefaultReplayMode: evaluation.ReplayModeNonLive,
		SourceRefs:        []evaluation.SourceRef{{Kind: evaluation.SourceKindRun, ID: "run_1", Route: "/v1/runs/run_1"}},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := sqliteStore.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}
	attempt := evaluation.ReplayAttempt{
		AttemptID:          "attempt_1",
		CandidateID:        candidate.CandidateID,
		EnvironmentScope:   "test",
		Mode:               evaluation.ReplayModeNonLive,
		Status:             evaluation.ReplayAttemptStatusCompleted,
		ApprovalHandling:   evaluation.ApprovalEvidenceOnly,
		SideEffectHandling: evaluation.SideEffectEvidenceOnly,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := sqliteStore.UpsertReplayAttempt(ctx, attempt); err != nil {
		t.Fatalf("UpsertReplayAttempt returned error: %v", err)
	}
	comparison := evaluation.ComparisonResult{
		ComparisonID:       "comparison_1",
		CandidateID:        candidate.CandidateID,
		AttemptID:          attempt.AttemptID,
		EnvironmentScope:   "test",
		TerminalStatus:     evaluation.ComparisonMatched,
		RuntimeSummary:     "runtime matched",
		PolicySummary:      "policy matched",
		IntegrationSummary: "integration matched",
		DeliverySummary:    "delivery matched",
		EvidenceSummary:    "evidence matched",
		Confidence:         "high",
		GeneratedAt:        now,
	}
	if err := sqliteStore.UpsertComparisonResult(ctx, comparison); err != nil {
		t.Fatalf("UpsertComparisonResult returned error: %v", err)
	}
	fixture := evaluation.RegressionFixture{
		FixtureID:            "fixture_1",
		DisplayName:          "Fixture 1",
		DomainClass:          evaluation.FixtureDomainSchedule,
		ManifestPath:         "fixtures/fixture_1/manifest.json",
		SourceRefs:           candidate.SourceRefs,
		CapturedEvidenceRefs: candidate.SourceRefs,
		ExpectedReplayMode:   evaluation.ReplayModeNonLive,
		CandidateID:          candidate.CandidateID,
		EnvironmentScope:     "test",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := sqliteStore.UpsertRegressionFixture(ctx, fixture); err != nil {
		t.Fatalf("UpsertRegressionFixture returned error: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	reopened, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore(reopen) returned error: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	candidates, err := reopened.ListReplayCandidates(ctx, evaluation.CandidateFilter{EnvironmentScope: "test"})
	if err != nil {
		t.Fatalf("ListReplayCandidates returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].CandidateID != candidate.CandidateID {
		t.Fatalf("expected persisted candidate, got %+v", candidates)
	}
	attempts, err := reopened.ListReplayAttempts(ctx, evaluation.AttemptFilter{EnvironmentScope: "test", CandidateID: candidate.CandidateID})
	if err != nil {
		t.Fatalf("ListReplayAttempts returned error: %v", err)
	}
	if len(attempts) != 1 || attempts[0].AttemptID != attempt.AttemptID {
		t.Fatalf("expected persisted attempt, got %+v", attempts)
	}
	comparisons, err := reopened.ListComparisonResults(ctx, evaluation.ComparisonFilter{EnvironmentScope: "test"})
	if err != nil {
		t.Fatalf("ListComparisonResults returned error: %v", err)
	}
	if len(comparisons) != 1 || comparisons[0].ComparisonID != comparison.ComparisonID {
		t.Fatalf("expected persisted comparison, got %+v", comparisons)
	}
	fixtures, err := reopened.ListRegressionFixtures(ctx, evaluation.FixtureFilter{EnvironmentScope: "test"})
	if err != nil {
		t.Fatalf("ListRegressionFixtures returned error: %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].FixtureID != fixture.FixtureID {
		t.Fatalf("expected persisted fixture, got %+v", fixtures)
	}
}
