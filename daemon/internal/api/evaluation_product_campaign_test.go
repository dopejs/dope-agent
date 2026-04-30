package api

import (
	"context"
	"encoding/json"
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

func TestEvaluationProductCampaignAPIRoutes(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_campaign_api",
		PrincipalID: "prn_campaign_api",
		Permissions: []identity.Permission{
			identity.PermissionEvaluationCampaignRead,
			identity.PermissionEvaluationCampaignManage,
			identity.PermissionEvaluationInspectionRead,
		},
	})
	fixture := evaluation.ProductManagedFixture{
		FixtureID:         "fixture_campaign_api",
		TenantID:          "ten_campaign_api",
		DisplayName:       "Campaign Fixture",
		DomainClass:       evaluation.FixtureDomainSchedule,
		CurrentRevisionID: "revision_campaign_api",
		ReviewState:       evaluation.ProductStatusApproved,
		SuppressionState:  evaluation.SuppressionStateNone,
		RetentionState:    evaluation.RetentionStateActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := sqliteStore.UpsertProductFixture(ctx, fixture); err != nil {
		t.Fatalf("UpsertProductFixture: %v", err)
	}

	create := httptest.NewRequest(http.MethodPost, "/v1/evaluation/campaigns", jsonBody(map[string]any{
		"campaignId":       "campaign_api",
		"displayName":      "Campaign API",
		"scopeSummary":     "approved fixture",
		"startImmediately": true,
		"sourceSelections": []map[string]any{{"sourceType": "product_fixture", "sourceId": fixture.FixtureID, "selectionReason": "release gate"}},
	})).WithContext(ctx)
	createRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, createRec, create)
	if createRec.Code != http.StatusCreated || !strings.Contains(createRec.Body.String(), `"status":"queued"`) {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	start := httptest.NewRequest(http.MethodPost, "/v1/evaluation/campaigns/campaign_api/start", nil).WithContext(ctx)
	startRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, startRec, start)
	if startRec.Code != http.StatusOK || !strings.Contains(startRec.Body.String(), `"status":"running"`) {
		t.Fatalf("start status=%d body=%s", startRec.Code, startRec.Body.String())
	}
	complete := httptest.NewRequest(http.MethodPost, "/v1/evaluation/campaigns/campaign_api/complete", nil).WithContext(ctx)
	completeRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, completeRec, complete)
	if completeRec.Code != http.StatusOK || !strings.Contains(completeRec.Body.String(), `"status":"completed"`) {
		t.Fatalf("complete status=%d body=%s", completeRec.Code, completeRec.Body.String())
	}
	publish := httptest.NewRequest(http.MethodPost, "/v1/evaluation/campaigns/campaign_api/publish-results", nil).WithContext(ctx)
	publishRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, publishRec, publish)
	if publishRec.Code != http.StatusOK || !strings.Contains(publishRec.Body.String(), `"status":"published"`) {
		t.Fatalf("publish status=%d body=%s", publishRec.Code, publishRec.Body.String())
	}

	items := httptest.NewRequest(http.MethodGet, "/v1/evaluation/campaigns/campaign_api/items", nil).WithContext(ctx)
	itemsRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, itemsRec, items)
	if itemsRec.Code != http.StatusOK || !strings.Contains(itemsRec.Body.String(), `"currentRevisionId":"revision_campaign_api"`) {
		t.Fatalf("items status=%d body=%s", itemsRec.Code, itemsRec.Body.String())
	}

	campaignItems, err := sqliteStore.ListCampaignItems(ctx, evaluation.ProductListFilter{TenantID: "ten_campaign_api"}, "campaign_api")
	if err != nil {
		t.Fatalf("ListCampaignItems: %v", err)
	}
	group := evaluation.CampaignAttemptGroup{
		AttemptGroupID:            "attempt_group_campaign_api",
		CampaignID:                "campaign_api",
		CampaignItemID:            campaignItems[0].CampaignItemID,
		TenantID:                  "ten_campaign_api",
		Status:                    evaluation.ProductStatusCompleted,
		ReplayAttemptIDs:          []string{"attempt_api"},
		ComparisonIDs:             []string{"comparison_api"},
		LiveValidationIDs:         []string{"ledger_api"},
		OperatorActionNeededCount: 1,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := sqliteStore.SaveCampaignAttemptGroup(ctx, group); err != nil {
		t.Fatalf("SaveCampaignAttemptGroup: %v", err)
	}
	attemptGroups := httptest.NewRequest(http.MethodGet, "/v1/evaluation/campaigns/campaign_api/attempt-groups", nil).WithContext(ctx)
	attemptGroupsRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, attemptGroupsRec, attemptGroups)
	if attemptGroupsRec.Code != http.StatusOK || !strings.Contains(attemptGroupsRec.Body.String(), "ledger_api") {
		t.Fatalf("attempt groups status=%d body=%s", attemptGroupsRec.Code, attemptGroupsRec.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/v1/evaluation/campaigns/campaign_api", nil).WithContext(ctx)
	getRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, getRec, get)
	if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), `"campaignId":"campaign_api"`) {
		t.Fatalf("get status=%d body=%s", getRec.Code, getRec.Body.String())
	}
}

func TestEvaluationProductCampaignCreateDeniedWithoutManagePermission(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_campaign_api",
		PrincipalID: "prn_viewer",
		Permissions: []identity.Permission{identity.PermissionEvaluationCampaignRead},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/evaluation/campaigns", jsonBody(map[string]any{"displayName": "Denied"})).WithContext(ctx)
	rec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "evaluation.campaign.manage") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func decodeCampaignResponse(t *testing.T, body []byte) evaluation.ReplayCampaign {
	t.Helper()
	var campaign evaluation.ReplayCampaign
	if err := json.Unmarshal(body, &campaign); err != nil {
		t.Fatalf("decode campaign: %v", err)
	}
	return campaign
}
