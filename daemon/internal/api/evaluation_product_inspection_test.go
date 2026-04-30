package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestEvaluationProductInspectionAPIRoutes(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_inspection_api",
		PrincipalID: "prn_inspection",
		Permissions: []identity.Permission{identity.PermissionEvaluationInspectionRead},
	})
	campaign := evaluation.ReplayCampaign{
		CampaignID:     "campaign_inspection_api",
		TenantID:       "ten_inspection_api",
		DisplayName:    "Inspection Campaign",
		Status:         evaluation.ProductStatusCompleted,
		CreatedAt:      now,
		RetentionState: evaluation.RetentionStateActive,
	}
	if err := sqliteStore.SaveReplayCampaign(ctx, campaign); err != nil {
		t.Fatalf("SaveReplayCampaign: %v", err)
	}
	item := evaluation.CampaignItem{
		CampaignItemID:       "campaign_inspection_api_item_001",
		CampaignID:           campaign.CampaignID,
		TenantID:             "ten_inspection_api",
		SourceType:           evaluation.ProductResourceDiscoveredCandidate,
		SourceID:             "candidate_inspection_api",
		SuppressionCheckedAt: now,
		CreatedAt:            now,
	}
	if err := sqliteStore.SaveCampaignItem(ctx, item); err != nil {
		t.Fatalf("SaveCampaignItem: %v", err)
	}
	inspection := evaluation.ToolCallInspection{
		InspectionID:             "inspection_api",
		TenantID:                 "ten_inspection_api",
		CampaignID:               campaign.CampaignID,
		CampaignItemID:           item.CampaignItemID,
		ToolCallRef:              "tool_call_api",
		OriginalEvidenceRef:      "original_api",
		NonLiveReplayEvidenceRef: "replay_api",
		LiveValidationLedgerRefs: []string{"ledger_api"},
		Classification:           evaluation.InspectionLiveValidationCompleted,
		RedactionStatus:          evaluation.RedactionStatusRedacted,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := sqliteStore.SaveToolCallInspection(ctx, inspection); err != nil {
		t.Fatalf("SaveToolCallInspection: %v", err)
	}

	list := httptest.NewRequest(http.MethodGet, "/v1/evaluation/campaigns/campaign_inspection_api/tool-call-inspections", nil).WithContext(ctx)
	listRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, listRec, list)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "inspection_api") || !strings.Contains(listRec.Body.String(), "ledger_api") {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	detail := httptest.NewRequest(http.MethodGet, "/v1/evaluation/tool-call-inspections/inspection_api", nil).WithContext(ctx)
	detailRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, detailRec, detail)
	if detailRec.Code != http.StatusOK || !strings.Contains(detailRec.Body.String(), "live_validation_completed") {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
}
