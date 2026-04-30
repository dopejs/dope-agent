package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func TestEvaluationProductStoreRoundTripAndRetention(t *testing.T) {
	t.Parallel()

	var _ evaluation.ProductStore = (*SQLiteStore)(nil)

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	policy := evaluation.DiscoveryPolicy{
		PolicyID:             "policy_eval_1",
		TenantID:             "ten_eval",
		Enabled:              true,
		SourceKinds:          []evaluation.SourceKind{evaluation.SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  25,
		MaxEmittedCandidates: 5,
		CostBudget:           10,
		CreatedBy:            "prn_eval",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.UpsertDiscoveryPolicy(ctx, policy); err != nil {
		t.Fatalf("UpsertDiscoveryPolicy: %v", err)
	}

	run := evaluation.DiscoveryRun{
		DiscoveryRunID:       "discovery_run_eval_1",
		TenantID:             "ten_eval",
		PolicyID:             policy.PolicyID,
		Status:               evaluation.ProductStatusCompleted,
		SourceKinds:          policy.SourceKinds,
		WindowStart:          policy.WindowStart,
		WindowEnd:            policy.WindowEnd,
		MaxInspectedRecords:  policy.MaxInspectedRecords,
		MaxEmittedCandidates: policy.MaxEmittedCandidates,
		CostBudget:           policy.CostBudget,
		InspectedRecords:     4,
		EmittedCandidates:    1,
		StartedBy:            "prn_eval",
		StartedAt:            now,
		CompletedAt:          &now,
		UpdatedAt:            now,
		IdempotencyKey:       "idem_eval_1",
	}
	if err := s.SaveDiscoveryRun(ctx, run); err != nil {
		t.Fatalf("SaveDiscoveryRun: %v", err)
	}

	candidate := evaluation.DiscoveredCandidate{
		DiscoveredCandidateID: "candidate_eval_1",
		TenantID:              "ten_eval",
		DiscoveryRunID:        run.DiscoveryRunID,
		SourceKind:            evaluation.SourceKindRun,
		SourceID:              "run_eval_1",
		Score:                 0.91,
		ScoreBand:             evaluation.ScoreBandHigh,
		RedactionStatus:       evaluation.RedactionStatusRedacted,
		EvidenceRef:           "evidence_eval_1",
		ReadinessStatus:       evaluation.ReadinessFullyReplayable,
		SuppressionState:      evaluation.SuppressionStateNone,
		RetentionState:        evaluation.RetentionStateActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	evidence := evaluation.CandidateEvidence{
		EvidenceID:              "evidence_eval_1",
		TenantID:                "ten_eval",
		DiscoveredCandidateID:   candidate.DiscoveredCandidateID,
		Summary:                 "redacted evidence",
		RedactedPayload:         map[string]any{"token": "[REDACTED]"},
		RedactionRulesApplied:   []string{"token"},
		SensitiveFieldsExcluded: []string{"token"},
		MaterializationAllowed:  true,
		RetentionState:          evaluation.RetentionStateActive,
		CreatedAt:               now,
	}
	if err := s.SaveDiscoveredCandidate(ctx, candidate, evidence); err != nil {
		t.Fatalf("SaveDiscoveredCandidate: %v", err)
	}
	if err := s.CreateSuppression(ctx, evaluation.SuppressionRecord{
		SuppressionID: "suppression_eval_1",
		TenantID:      "ten_eval",
		TargetKind:    evaluation.ProductResourceDiscoveredCandidate,
		TargetID:      candidate.DiscoveredCandidateID,
		ReasonCode:    "operator_hidden",
		CreatedBy:     "prn_eval",
		CreatedAt:     now,
		Active:        true,
	}); err != nil {
		t.Fatalf("CreateSuppression: %v", err)
	}

	other := policy
	other.PolicyID = "policy_other"
	other.TenantID = "ten_other"
	if err := s.UpsertDiscoveryPolicy(ctx, other); err != nil {
		t.Fatalf("UpsertDiscoveryPolicy(other): %v", err)
	}

	policies, err := s.ListDiscoveryPolicies(ctx, evaluation.DiscoveryPolicyFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"}})
	if err != nil {
		t.Fatalf("ListDiscoveryPolicies: %v", err)
	}
	if len(policies) != 1 || policies[0].PolicyID != policy.PolicyID {
		t.Fatalf("unexpected policies: %+v", policies)
	}
	runs, err := s.ListDiscoveryRuns(ctx, evaluation.DiscoveryRunFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"}, Status: evaluation.ProductStatusCompleted})
	if err != nil {
		t.Fatalf("ListDiscoveryRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].DiscoveryRunID != run.DiscoveryRunID {
		t.Fatalf("unexpected runs: %+v", runs)
	}
	candidates, err := s.ListDiscoveredCandidates(ctx, evaluation.DiscoveredCandidateFilter{
		ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"},
		ReadinessStatus:   evaluation.ReadinessFullyReplayable,
	})
	if err != nil {
		t.Fatalf("ListDiscoveredCandidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].RetentionState != evaluation.RetentionStateActive {
		t.Fatalf("unexpected candidates before retention: %+v", candidates)
	}
	if err := s.ApplyRetention(ctx, evaluation.RetentionApplicationFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"}, ResourceKinds: []evaluation.ProductResourceKind{evaluation.ProductResourceDiscoveredCandidate}, DryRun: true}); err != nil {
		t.Fatalf("ApplyRetention(dry-run): %v", err)
	}
	candidates, err = s.ListDiscoveredCandidates(ctx, evaluation.DiscoveredCandidateFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"}})
	if err != nil {
		t.Fatalf("ListDiscoveredCandidates(after dry-run): %v", err)
	}
	if candidates[0].RetentionState != evaluation.RetentionStateActive {
		t.Fatalf("dry-run mutated candidate retention state: %+v", candidates[0])
	}
	if err := s.ApplyRetention(ctx, evaluation.RetentionApplicationFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"}, ResourceKinds: []evaluation.ProductResourceKind{evaluation.ProductResourceDiscoveredCandidate}}); err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}
	candidates, err = s.ListDiscoveredCandidates(ctx, evaluation.DiscoveredCandidateFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval"}})
	if err != nil {
		t.Fatalf("ListDiscoveredCandidates(after retention): %v", err)
	}
	if candidates[0].RetentionState != evaluation.RetentionStateExpired {
		t.Fatalf("retention did not expire candidate: %+v", candidates[0])
	}
}

func TestEvaluationProductStoreRequiresTenantForLists(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	_, err = s.ListDiscoveryPolicies(context.Background(), evaluation.DiscoveryPolicyFilter{})
	if !errors.Is(err, evaluation.ErrEvaluationProductTenantRequired) {
		t.Fatalf("ListDiscoveryPolicies err=%v, want ErrEvaluationProductTenantRequired", err)
	}
}

func TestEvaluationProductStoreRoundTripsProductResources(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	fixture := evaluation.ProductManagedFixture{
		FixtureID:        "fixture_eval_1",
		TenantID:         "ten_eval",
		DisplayName:      "Fixture",
		DomainClass:      evaluation.FixtureDomainComputerUse,
		SourceKind:       "run",
		ReviewState:      evaluation.ProductStatusApproved,
		SuppressionState: evaluation.SuppressionStateNone,
		RetentionState:   evaluation.RetentionStateActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.UpsertProductFixture(ctx, fixture); err != nil {
		t.Fatalf("UpsertProductFixture: %v", err)
	}
	if err := s.SaveFixtureRevision(ctx, evaluation.FixtureRevision{
		RevisionID:      "revision_eval_1",
		FixtureID:       fixture.FixtureID,
		TenantID:        "ten_eval",
		RevisionNumber:  1,
		FixturePayload:  map[string]any{"input": "redacted"},
		RedactionStatus: evaluation.RedactionStatusRedacted,
		CreatedAt:       now,
	}); err != nil {
		t.Fatalf("SaveFixtureRevision: %v", err)
	}
	campaign := evaluation.ReplayCampaign{
		CampaignID:     "campaign_eval_1",
		TenantID:       "ten_eval",
		DisplayName:    "Campaign",
		Status:         evaluation.ProductStatusCompleted,
		CreatedAt:      now,
		StartedAt:      &now,
		CompletedAt:    &now,
		RetentionState: evaluation.RetentionStateActive,
	}
	if err := s.SaveReplayCampaign(ctx, campaign); err != nil {
		t.Fatalf("SaveReplayCampaign: %v", err)
	}
	item := evaluation.CampaignItem{
		CampaignItemID:       "campaign_item_eval_1",
		CampaignID:           campaign.CampaignID,
		TenantID:             "ten_eval",
		SourceType:           evaluation.ProductResourceProductFixture,
		SourceID:             fixture.FixtureID,
		SuppressionCheckedAt: now,
		CreatedAt:            now,
	}
	if err := s.SaveCampaignItem(ctx, item); err != nil {
		t.Fatalf("SaveCampaignItem: %v", err)
	}
	if err := s.SaveCampaignAttemptGroup(ctx, evaluation.CampaignAttemptGroup{
		AttemptGroupID: "attempt_group_eval_1",
		CampaignID:     campaign.CampaignID,
		CampaignItemID: item.CampaignItemID,
		TenantID:       "ten_eval",
		Status:         evaluation.ProductStatusCompleted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("SaveCampaignAttemptGroup: %v", err)
	}
	if err := s.SaveDashboardProjection(ctx, evaluation.DashboardProjection{
		ProjectionID:         "projection_eval_1",
		TenantID:             "ten_eval",
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		CampaignStatusCounts: map[string]int{"completed": 1},
		GeneratedAt:          now,
	}); err != nil {
		t.Fatalf("SaveDashboardProjection: %v", err)
	}
	if err := s.SaveToolCallInspection(ctx, evaluation.ToolCallInspection{
		InspectionID:    "inspection_eval_1",
		TenantID:        "ten_eval",
		CampaignID:      campaign.CampaignID,
		CampaignItemID:  item.CampaignItemID,
		ToolCallRef:     "tool_call_eval_1",
		Classification:  "matched",
		RedactionStatus: evaluation.RedactionStatusRedacted,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("SaveToolCallInspection: %v", err)
	}

	fixtures, err := s.ListProductFixtures(ctx, evaluation.ProductListFilter{TenantID: "ten_eval"})
	if err != nil {
		t.Fatalf("ListProductFixtures: %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].FixtureID != fixture.FixtureID {
		t.Fatalf("unexpected fixtures: %+v", fixtures)
	}
	campaigns, err := s.ListReplayCampaigns(ctx, evaluation.ProductListFilter{TenantID: "ten_eval"})
	if err != nil {
		t.Fatalf("ListReplayCampaigns: %v", err)
	}
	if len(campaigns) != 1 || campaigns[0].CampaignID != campaign.CampaignID {
		t.Fatalf("unexpected campaigns: %+v", campaigns)
	}
	projections, err := s.ListDashboardProjections(ctx, evaluation.ProductListFilter{TenantID: "ten_eval"})
	if err != nil {
		t.Fatalf("ListDashboardProjections: %v", err)
	}
	if len(projections) != 1 || projections[0].CampaignStatusCounts["completed"] != 1 {
		t.Fatalf("unexpected projections: %+v", projections)
	}
	inspections, err := s.ListToolCallInspections(ctx, evaluation.ProductListFilter{TenantID: "ten_eval"}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListToolCallInspections: %v", err)
	}
	if len(inspections) != 1 || inspections[0].InspectionID != "inspection_eval_1" {
		t.Fatalf("unexpected inspections: %+v", inspections)
	}
}
