package evaluation

import (
	"errors"
	"testing"
	"time"
)

func TestReplayCampaignLifecycleTransitions(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign, items, err := CreateReplayCampaign(CreateCampaignInput{
		CampaignID:     "campaign_1",
		TenantID:       "ten_eval",
		DisplayName:    "Release Campaign",
		ScopeSummary:   "release smoke",
		IdempotencyKey: "idem_1",
		SourceSelections: []CampaignSourceSelection{{
			SourceType:       ProductResourceProductFixture,
			SourceID:         "fixture_1",
			TenantID:         "ten_eval",
			ReviewState:      ProductStatusApproved,
			RetentionState:   RetentionStateActive,
			SuppressionState: SuppressionStateNone,
		}},
	}, now)
	if err != nil {
		t.Fatalf("CreateReplayCampaign: %v", err)
	}
	if campaign.Status != ProductStatusDraft || len(items) != 1 || CampaignIdempotencyScope(campaign) != "ten_eval:idem_1" {
		t.Fatalf("unexpected campaign: %+v items=%+v", campaign, items)
	}
	campaign, err = TransitionReplayCampaign(campaign, CampaignTransitionStart, now.Add(time.Minute))
	if err != nil || campaign.Status != ProductStatusRunning || campaign.StartedAt == nil {
		t.Fatalf("start campaign=%+v err=%v", campaign, err)
	}
	campaign, err = TransitionReplayCampaign(campaign, CampaignTransitionComplete, now.Add(2*time.Minute))
	if err != nil || campaign.Status != ProductStatusCompleted || campaign.CompletedAt == nil {
		t.Fatalf("complete campaign=%+v err=%v", campaign, err)
	}
	campaign, err = TransitionReplayCampaign(campaign, CampaignTransitionPublish, now.Add(3*time.Minute))
	if err != nil || campaign.Status != ProductStatusPublished || campaign.PublishedAt == nil {
		t.Fatalf("publish campaign=%+v err=%v", campaign, err)
	}
	if _, err := TransitionReplayCampaign(campaign, CampaignTransitionCancel, now.Add(4*time.Minute)); !errors.Is(err, ErrEvaluationCampaignTransitionInvalid) {
		t.Fatalf("published cancel err=%v, want invalid transition", err)
	}
}
