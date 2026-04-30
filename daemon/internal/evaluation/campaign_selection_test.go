package evaluation

import (
	"errors"
	"testing"
	"time"
)

func TestCampaignSelectionRejectsSuppressedExpiredDraftAndCrossTenantSources(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign := ReplayCampaign{CampaignID: "campaign_selection", TenantID: "ten_eval"}
	cases := []CampaignSourceSelection{
		{SourceType: ProductResourceDiscoveredCandidate, SourceID: "candidate_1", TenantID: "ten_other", RetentionState: RetentionStateActive},
		{SourceType: ProductResourceDiscoveredCandidate, SourceID: "candidate_1", TenantID: "ten_eval", RetentionState: RetentionStateActive, SuppressionState: SuppressionStateSuppressed},
		{SourceType: ProductResourceDiscoveredCandidate, SourceID: "candidate_1", TenantID: "ten_eval", RetentionState: RetentionStateExpired},
		{SourceType: ProductResourceProductFixture, SourceID: "fixture_1", TenantID: "ten_eval", RetentionState: RetentionStateActive, ReviewState: ProductStatusDraft},
	}
	for _, tc := range cases {
		_, err := CampaignItemFromSelection(campaign, tc, 1, now)
		if !errors.Is(err, ErrEvaluationCampaignSelectionInvalid) && !errors.Is(err, ErrEvaluationProductCrossTenantSource) {
			t.Fatalf("selection %+v err=%v, want rejection", tc, err)
		}
	}
}
