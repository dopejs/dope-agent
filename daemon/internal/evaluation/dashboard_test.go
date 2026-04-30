package evaluation

import (
	"testing"
	"time"
)

func TestBuildDashboardProjectionAggregatesTenantScopedProductSignals(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	projection, err := BuildDashboardProjection(DashboardProjectionInput{
		ProjectionID: "projection_eval",
		TenantID:     "ten_eval",
		WindowStart:  now.Add(-time.Hour),
		WindowEnd:    now,
		GeneratedAt:  now,
		Campaigns: []ReplayCampaign{
			{CampaignID: "campaign_complete", TenantID: "ten_eval", Status: ProductStatusCompleted},
			{CampaignID: "campaign_failed", TenantID: "ten_eval", Status: ProductStatusFailed},
			{CampaignID: "campaign_other", TenantID: "ten_other", Status: ProductStatusCompleted},
		},
		Candidates: []DiscoveredCandidate{
			{TenantID: "ten_eval", RetentionState: RetentionStateActive, SuppressionState: SuppressionStateNone},
			{TenantID: "ten_eval", RetentionState: RetentionStateExpired, SuppressionState: SuppressionStateSuppressed},
			{TenantID: "ten_other", RetentionState: RetentionStateActive, SuppressionState: SuppressionStateNone},
		},
		Fixtures: []ProductManagedFixture{
			{TenantID: "ten_eval", ReviewState: ProductStatusApproved, RetentionState: RetentionStateActive},
			{TenantID: "ten_eval", ReviewState: ProductStatusDraft, RetentionState: RetentionStateExpired},
			{TenantID: "ten_other", ReviewState: ProductStatusApproved, RetentionState: RetentionStateActive},
		},
		AttemptGroups: []CampaignAttemptGroup{
			{
				TenantID:                  "ten_eval",
				DriftCount:                2,
				FailureCount:              1,
				UnsupportedCount:          3,
				OperatorActionNeededCount: 4,
				LiveValidationIDs:         []string{"ledger_1", "ledger_2"},
			},
			{
				TenantID:          "ten_other",
				DriftCount:        100,
				FailureCount:      100,
				UnsupportedCount:  100,
				LiveValidationIDs: []string{"ledger_other"},
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildDashboardProjection: %v", err)
	}

	if projection.CampaignStatusCounts[string(ProductStatusCompleted)] != 1 || projection.CampaignStatusCounts[string(ProductStatusFailed)] != 1 {
		t.Fatalf("campaign counts=%+v, want completed=1 failed=1", projection.CampaignStatusCounts)
	}
	if projection.DriftSummary["total"] != 2 || projection.FailureSummary["total"] != 1 || projection.UnsupportedSummary["total"] != 3 {
		t.Fatalf("unexpected drift/failure/unsupported summaries: drift=%+v failure=%+v unsupported=%+v", projection.DriftSummary, projection.FailureSummary, projection.UnsupportedSummary)
	}
	if projection.OperatorActionNeededSummary["total"] != 4 || projection.LiveValidationSummary["linked"] != 2 {
		t.Fatalf("unexpected operator/live summaries: operator=%+v live=%+v", projection.OperatorActionNeededSummary, projection.LiveValidationSummary)
	}
	if projection.CandidateSummary[string(RetentionStateActive)] != 1 || projection.CandidateSummary[string(SuppressionStateSuppressed)] != 1 {
		t.Fatalf("candidate summary=%+v, want active and suppressed tenant counts", projection.CandidateSummary)
	}
	if projection.FixtureSummary[string(ProductStatusApproved)] != 1 || projection.FixtureSummary[string(RetentionStateExpired)] != 1 {
		t.Fatalf("fixture summary=%+v, want approved and expired tenant counts", projection.FixtureSummary)
	}
}
