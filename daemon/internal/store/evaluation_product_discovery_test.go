package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func TestEvaluationProductDiscoveryStoreFiltersCandidatesByRunSourceAndSuppression(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Now().UTC()
	policy := evaluation.DiscoveryPolicy{
		PolicyID:             "policy_discovery",
		TenantID:             "ten_eval",
		Enabled:              true,
		SourceKinds:          []evaluation.SourceKind{evaluation.SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.UpsertDiscoveryPolicy(ctx, policy); err != nil {
		t.Fatalf("UpsertDiscoveryPolicy: %v", err)
	}
	run, err := evaluation.BuildDiscoveryRunFromPolicy(policy, evaluation.StartDiscoveryRunInput{IdempotencyKey: "idem_discovery"}, now)
	if err != nil {
		t.Fatalf("BuildDiscoveryRunFromPolicy: %v", err)
	}
	run.Status = evaluation.ProductStatusCompleted
	if err := s.SaveDiscoveryRun(ctx, run); err != nil {
		t.Fatalf("SaveDiscoveryRun: %v", err)
	}
	candidate := evaluation.DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_discovery",
		TenantID:              "ten_eval",
		DiscoveryRunID:        run.DiscoveryRunID,
		SourceKind:            evaluation.SourceKindRun,
		SourceID:              "run_source",
		Score:                 0.8,
		ScoreBand:             evaluation.ScoreBandHigh,
		RedactionStatus:       evaluation.RedactionStatusRedacted,
		ReadinessStatus:       evaluation.ReadinessFullyReplayable,
		SuppressionState:      evaluation.SuppressionStateSuppressed,
		RetentionState:        evaluation.RetentionStateActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.SaveDiscoveredCandidate(ctx, candidate, evaluation.CandidateEvidence{
		EvidenceID:            "evidence_discovery",
		TenantID:              "ten_eval",
		DiscoveredCandidateID: candidate.DiscoveredCandidateID,
		RedactedPayload:       map[string]any{"safe": "value"},
		RetentionState:        evaluation.RetentionStateActive,
		CreatedAt:             now,
	}); err != nil {
		t.Fatalf("SaveDiscoveredCandidate: %v", err)
	}
	if err := s.CreateSuppression(ctx, evaluation.SuppressionRecord{
		SuppressionID: "suppression_discovery",
		TenantID:      "ten_eval",
		TargetKind:    evaluation.ProductResourceDiscoveredCandidate,
		TargetID:      candidate.DiscoveredCandidateID,
		ReasonCode:    "operator_hidden",
		Active:        true,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("CreateSuppression: %v", err)
	}

	candidates, err := s.ListDiscoveredCandidates(ctx, evaluation.DiscoveredCandidateFilter{
		ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"},
		DiscoveryRunID:    run.DiscoveryRunID,
		SourceKind:        evaluation.SourceKindRun,
		SuppressionState:  evaluation.SuppressionStateSuppressed,
	})
	if err != nil {
		t.Fatalf("ListDiscoveredCandidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].DiscoveredCandidateID != candidate.DiscoveredCandidateID {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	otherTenant, err := s.ListDiscoveredCandidates(ctx, evaluation.DiscoveredCandidateFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_other"}})
	if err != nil {
		t.Fatalf("ListDiscoveredCandidates(other): %v", err)
	}
	if len(otherTenant) != 0 {
		t.Fatalf("cross-tenant candidates leaked: %+v", otherTenant)
	}
}
