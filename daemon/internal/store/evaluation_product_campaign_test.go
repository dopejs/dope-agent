package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func TestEvaluationProductCampaignStoreDetailItemsAndAttemptGroups(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign := evaluation.ReplayCampaign{
		CampaignID:     "campaign_store",
		TenantID:       "ten_campaign",
		DisplayName:    "Campaign Store",
		Status:         evaluation.ProductStatusRunning,
		CreatedAt:      now,
		StartedAt:      &now,
		RetentionState: evaluation.RetentionStateActive,
	}
	if err := s.SaveReplayCampaign(ctx, campaign); err != nil {
		t.Fatalf("SaveReplayCampaign: %v", err)
	}
	item := evaluation.CampaignItem{
		CampaignItemID:       "campaign_store_item_001",
		CampaignID:           campaign.CampaignID,
		TenantID:             campaign.TenantID,
		SourceType:           evaluation.ProductResourceProductFixture,
		SourceID:             "fixture_store",
		SourceSnapshot:       map[string]any{"fixtureId": "fixture_store", "revision": "rev_1"},
		SelectionReason:      "approved fixture",
		SuppressionCheckedAt: now,
		CreatedAt:            now,
	}
	if err := s.SaveCampaignItem(ctx, item); err != nil {
		t.Fatalf("SaveCampaignItem: %v", err)
	}
	group := evaluation.CampaignAttemptGroup{
		AttemptGroupID:            "attempt_group_store",
		CampaignID:                campaign.CampaignID,
		CampaignItemID:            item.CampaignItemID,
		TenantID:                  campaign.TenantID,
		ReplayAttemptIDs:          []string{"attempt_1"},
		ComparisonIDs:             []string{"comparison_1"},
		LiveValidationIDs:         []string{"ledger_1"},
		Status:                    evaluation.ProductStatusCompleted,
		DriftCount:                1,
		FailureCount:              2,
		UnsupportedCount:          3,
		OperatorActionNeededCount: 4,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := s.SaveCampaignAttemptGroup(ctx, group); err != nil {
		t.Fatalf("SaveCampaignAttemptGroup: %v", err)
	}

	got, ok, err := s.GetReplayCampaign(ctx, campaign.TenantID, campaign.CampaignID)
	if err != nil {
		t.Fatalf("GetReplayCampaign: %v", err)
	}
	if !ok || got.CampaignID != campaign.CampaignID {
		t.Fatalf("campaign=%+v ok=%v, want saved campaign", got, ok)
	}
	items, err := s.ListCampaignItems(ctx, evaluation.ProductListFilter{TenantID: campaign.TenantID}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListCampaignItems: %v", err)
	}
	if len(items) != 1 || items[0].SourceSnapshot["revision"] != "rev_1" {
		t.Fatalf("items=%+v, want immutable source snapshot", items)
	}
	groups, err := s.ListCampaignAttemptGroups(ctx, evaluation.ProductListFilter{TenantID: campaign.TenantID}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListCampaignAttemptGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].OperatorActionNeededCount != 4 || len(groups[0].LiveValidationIDs) != 1 {
		t.Fatalf("groups=%+v, want aggregate result links", groups)
	}

	if err := s.ApplyRetention(ctx, evaluation.RetentionApplicationFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: campaign.TenantID}, ResourceKinds: []evaluation.ProductResourceKind{evaluation.ProductResourceCampaign}}); err != nil {
		t.Fatalf("ApplyRetention(campaign): %v", err)
	}
	got, ok, err = s.GetReplayCampaign(ctx, campaign.TenantID, campaign.CampaignID)
	if err != nil {
		t.Fatalf("GetReplayCampaign(after retention): %v", err)
	}
	if !ok || got.RetentionState != evaluation.RetentionStateExpired {
		t.Fatalf("campaign after retention=%+v ok=%v, want expired", got, ok)
	}
}
