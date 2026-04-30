package evaluation

import (
	"errors"
	"testing"
	"time"
)

func TestBuildDiscoveredCandidateFromSignalsScoresAndExplainsCandidate(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	candidate, err := BuildDiscoveredCandidateFromSignals(CandidateScoringInput{
		TenantID:              "ten_eval",
		DiscoveryRunID:        "discovery_run_1",
		SourceKind:            SourceKindRun,
		SourceID:              "run_1",
		SourceRefs:            []SourceRef{{Kind: SourceKindRun, ID: "run_1", Route: "/v1/runs/run_1"}},
		FailureRecurrence:     3,
		DriftSignal:           true,
		ToolCallClass:         "mail.send",
		LiveValidationOutcome: "operator_action_needed",
		WorkflowCoverage:      2,
		OperatorRelevance:     2,
		ObservedAt:            now.Add(-2 * time.Hour),
		RedactionStatus:       RedactionStatusRedacted,
	}, now)
	if err != nil {
		t.Fatalf("BuildDiscoveredCandidateFromSignals: %v", err)
	}
	if candidate.ScoreBand != ScoreBandHigh || candidate.Score < 0.75 {
		t.Fatalf("score=%v band=%s, want high", candidate.Score, candidate.ScoreBand)
	}
	if candidate.ReadinessStatus != ReadinessFullyReplayable || candidate.RedactionStatus != RedactionStatusRedacted {
		t.Fatalf("unexpected readiness/redaction: %+v", candidate)
	}
	if candidate.ExplanationFields["toolCallClass"] != "mail.send" || candidate.ExplanationFields["liveValidationOutcome"] != "operator_action_needed" {
		t.Fatalf("missing explanation fields: %+v", candidate.ExplanationFields)
	}
}

func TestBuildDiscoveredCandidateFromSignalsFailsClosedOnRedactionFailure(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	candidate, err := BuildDiscoveredCandidateFromSignals(CandidateScoringInput{
		TenantID:        "ten_eval",
		DiscoveryRunID:  "discovery_run_1",
		SourceKind:      SourceKindWorkflow,
		SourceID:        "workflow_1",
		RedactionStatus: RedactionStatusFailed,
	}, now)
	if err != nil {
		t.Fatalf("BuildDiscoveredCandidateFromSignals: %v", err)
	}
	if candidate.ReadinessStatus != ReadinessBlocked {
		t.Fatalf("readiness=%s, want blocked after redaction failure", candidate.ReadinessStatus)
	}
}

func TestBuildDiscoveredCandidateFromSignalsRequiresSource(t *testing.T) {
	_, err := BuildDiscoveredCandidateFromSignals(CandidateScoringInput{
		TenantID:       "ten_eval",
		DiscoveryRunID: "discovery_run_1",
		SourceKind:     SourceKindRun,
	}, time.Now())
	if !errors.Is(err, ErrEvaluationProductSourceRequired) {
		t.Fatalf("err=%v, want source required", err)
	}
}
