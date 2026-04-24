package evaluation

import (
	"context"
	"testing"
)

func TestCreateComparisonReportsDriftAndLimitations(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	manager := NewManager(Dependencies{EnvironmentScope: "test", Store: store, Clock: fixedClock})
	candidate := ReplayCandidate{
		CandidateID:       "candidate_drift",
		CandidateKind:     CandidateKindCuratedWork,
		DisplayName:       "Runtime drift",
		SourceKind:        SourceKindRun,
		SourceID:          "run_base",
		EnvironmentScope:  "test",
		ReadinessStatus:   ReadinessFullyReplayable,
		DefaultReplayMode: ReplayModeNonLive,
		SourceRefs:        []SourceRef{{Kind: SourceKindRun, ID: "run_base", Route: "/v1/runs/run_base"}},
		ExpectedComparison: PlaneSummaries{
			Runtime:     "baseline runtime completed",
			Policy:      "baseline policy allowed",
			Integration: "baseline integration stable",
			Delivery:    "baseline delivery completed",
			Evidence:    "baseline evidence complete",
		},
	}
	if err := manager.UpsertReplayCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertReplayCandidate returned error: %v", err)
	}
	attempt, err := manager.CreateReplayAttempt(ctx, candidate.CandidateID, CreateReplayAttemptInput{})
	if err != nil {
		t.Fatalf("CreateReplayAttempt returned error: %v", err)
	}
	attempt.RuntimeSummary = "replay runtime changed"
	if err := store.UpsertReplayAttempt(ctx, attempt); err != nil {
		t.Fatalf("UpsertReplayAttempt returned error: %v", err)
	}

	comparison, err := manager.CreateComparison(ctx, attempt.AttemptID, CreateComparisonInput{})
	if err != nil {
		t.Fatalf("CreateComparison returned error: %v", err)
	}
	if comparison.TerminalStatus != ComparisonDrifted {
		t.Fatalf("expected drifted comparison, got %s", comparison.TerminalStatus)
	}
	if len(comparison.DriftFindings) != 1 {
		t.Fatalf("expected one drift finding, got %+v", comparison.DriftFindings)
	}
	if comparison.DriftFindings[0].Plane != DriftPlaneRuntime {
		t.Fatalf("expected runtime drift, got %s", comparison.DriftFindings[0].Plane)
	}
}
