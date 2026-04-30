package evaluation

import (
	"testing"
	"time"
)

func TestCampaignAttemptGroupAggregatesReplayComparisonAndLiveValidationSignals(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	group, err := BuildCampaignAttemptGroup(CampaignAttemptAggregationInput{
		TenantID:                  "ten_eval",
		CampaignID:                "campaign_1",
		CampaignItemID:            "campaign_1_item_001",
		ReplayAttemptIDs:          []string{"attempt_1"},
		ComparisonIDs:             []string{"comparison_1"},
		LiveValidationIDs:         []string{"lv_1"},
		DriftCount:                2,
		FailureCount:              0,
		UnsupportedCount:          1,
		OperatorActionNeededCount: 1,
	}, now)
	if err != nil {
		t.Fatalf("BuildCampaignAttemptGroup: %v", err)
	}
	if group.Status != ProductStatusCompleted || group.DriftCount != 2 || group.UnsupportedCount != 1 || group.OperatorActionNeededCount != 1 {
		t.Fatalf("unexpected group: %+v", group)
	}
	if len(group.ReplayAttemptIDs) != 1 || len(group.ComparisonIDs) != 1 || len(group.LiveValidationIDs) != 1 {
		t.Fatalf("lost evidence links: %+v", group)
	}
}

func TestCampaignReplayLaunchPlanUsesNonLiveAttempts(t *testing.T) {
	plans := BuildCampaignReplayLaunchPlan(ReplayCampaign{CampaignID: "campaign_1"}, []CampaignItem{{CampaignItemID: "item_1"}})
	if len(plans) != 1 || plans[0].Mode != string(ReplayModeNonLive) || len(plans[0].ReplayAttemptIDs) != 1 {
		t.Fatalf("unexpected launch plans: %+v", plans)
	}
}

func TestBuildCampaignRunnerPlanLaunchesNonLiveAttemptsAndCarriesLiveValidationLinks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign := ReplayCampaign{CampaignID: "campaign_runner", TenantID: "ten_runner"}
	items := []CampaignItem{{CampaignItemID: "campaign_runner_item_001", CampaignID: campaign.CampaignID, TenantID: campaign.TenantID}}
	plan, err := BuildCampaignRunnerPlan(CampaignRunnerInput{
		Campaign:          campaign,
		Items:             items,
		LiveValidationIDs: map[string][]string{"campaign_runner_item_001": {"ledger_runner"}},
		Now:               now,
	})
	if err != nil {
		t.Fatalf("BuildCampaignRunnerPlan: %v", err)
	}
	if len(plan.Launches) != 1 || plan.Launches[0].Mode != string(ReplayModeNonLive) {
		t.Fatalf("launches=%+v, want non-live launch", plan.Launches)
	}
	if len(plan.Groups) != 1 || len(plan.Groups[0].LiveValidationIDs) != 1 || plan.Groups[0].LiveValidationIDs[0] != "ledger_runner" {
		t.Fatalf("groups=%+v, want live-validation link", plan.Groups)
	}
}
