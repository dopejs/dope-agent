package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
)

func TestEvaluationProductInspectionStorePersistsDetailAndCampaignList(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	campaign := evaluation.ReplayCampaign{
		CampaignID:     "campaign_inspection",
		TenantID:       "ten_inspection",
		DisplayName:    "Inspection Campaign",
		Status:         evaluation.ProductStatusCompleted,
		CreatedAt:      now,
		RetentionState: evaluation.RetentionStateActive,
	}
	if err := s.SaveReplayCampaign(ctx, campaign); err != nil {
		t.Fatalf("SaveReplayCampaign: %v", err)
	}
	item := evaluation.CampaignItem{
		CampaignItemID:       "campaign_inspection_item_001",
		CampaignID:           campaign.CampaignID,
		TenantID:             campaign.TenantID,
		SourceType:           evaluation.ProductResourceDiscoveredCandidate,
		SourceID:             "candidate_inspection",
		SuppressionCheckedAt: now,
		CreatedAt:            now,
	}
	if err := s.SaveCampaignItem(ctx, item); err != nil {
		t.Fatalf("SaveCampaignItem: %v", err)
	}
	inspection := evaluation.ToolCallInspection{
		InspectionID:             "inspection_store",
		TenantID:                 campaign.TenantID,
		CampaignID:               campaign.CampaignID,
		CampaignItemID:           item.CampaignItemID,
		ToolCallRef:              "tool_call_store",
		OriginalEvidenceRef:      "original_store",
		NonLiveReplayEvidenceRef: "replay_store",
		LiveValidationLedgerRefs: []string{"ledger_store"},
		Classification:           evaluation.InspectionDrifted,
		DiffSummary:              "redacted drift",
		RedactionStatus:          evaluation.RedactionStatusRedacted,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := s.SaveToolCallInspection(ctx, inspection); err != nil {
		t.Fatalf("SaveToolCallInspection: %v", err)
	}

	got, ok, err := s.GetToolCallInspection(ctx, campaign.TenantID, inspection.InspectionID)
	if err != nil {
		t.Fatalf("GetToolCallInspection: %v", err)
	}
	if !ok || got.OriginalEvidenceRef != "original_store" || len(got.LiveValidationLedgerRefs) != 1 {
		t.Fatalf("inspection=%+v ok=%v, want stable evidence links", got, ok)
	}
	items, err := s.ListToolCallInspections(ctx, evaluation.ProductListFilter{TenantID: campaign.TenantID}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListToolCallInspections: %v", err)
	}
	if len(items) != 1 || items[0].Classification != evaluation.InspectionDrifted {
		t.Fatalf("items=%+v, want campaign inspection list", items)
	}
	otherTenant, ok, err := s.GetToolCallInspection(ctx, "ten_other", inspection.InspectionID)
	if err != nil {
		t.Fatalf("GetToolCallInspection(other tenant): %v", err)
	}
	if ok || otherTenant.InspectionID != "" {
		t.Fatalf("other tenant inspection=%+v ok=%v, want isolated", otherTenant, ok)
	}

	if err := s.ApplyRetention(ctx, evaluation.RetentionApplicationFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: campaign.TenantID}, ResourceKinds: []evaluation.ProductResourceKind{evaluation.ProductResourceToolCallInspection}}); err != nil {
		t.Fatalf("ApplyRetention(inspection): %v", err)
	}
	expired, ok, err := s.GetToolCallInspection(ctx, campaign.TenantID, inspection.InspectionID)
	if err != nil {
		t.Fatalf("GetToolCallInspection(after retention): %v", err)
	}
	if !ok || expired.RetentionState != evaluation.RetentionStateExpired || expired.OriginalEvidenceRef != "original_store" {
		t.Fatalf("inspection after retention=%+v ok=%v, want expired detail with evidence link", expired, ok)
	}
	items, err = s.ListToolCallInspections(ctx, evaluation.ProductListFilter{TenantID: campaign.TenantID}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListToolCallInspections(after retention): %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items after retention=%+v, want retention-filtered list", items)
	}
}
