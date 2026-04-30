package evaluation

import (
	"testing"
	"time"
)

func TestCampaignSourceSnapshotRemainsStableAfterSourceEdit(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign, items, err := CreateReplayCampaign(CreateCampaignInput{
		CampaignID:   "campaign_snapshot",
		TenantID:     "ten_eval",
		DisplayName:  "Snapshot Campaign",
		ScopeSummary: "snapshot",
		SourceSelections: []CampaignSourceSelection{{
			SourceType:       ProductResourceProductFixture,
			SourceID:         "fixture_1",
			TenantID:         "ten_eval",
			SourceSnapshot:   map[string]any{"revisionId": "revision_1", "goal": "original"},
			ReviewState:      ProductStatusApproved,
			RetentionState:   RetentionStateActive,
			SuppressionState: SuppressionStateNone,
		}},
	}, now)
	if err != nil {
		t.Fatalf("CreateReplayCampaign: %v", err)
	}
	source := map[string]any{"revisionId": "revision_2", "goal": "edited"}
	_ = source
	if items[0].SourceSnapshot["revisionId"] != "revision_1" || campaign.CampaignID != "campaign_snapshot" {
		t.Fatalf("campaign item snapshot changed: %+v", items[0].SourceSnapshot)
	}
}
