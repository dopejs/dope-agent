package tenancy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestEvaluationProductTenantHelpersRejectCrossTenantWrites(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	accessor := tenancy.NewEvaluation(s, nil)
	ctxA := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_eval_a", PrincipalID: "prn_a"})
	ctxB := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_eval_b", PrincipalID: "prn_b"})
	now := time.Now().UTC()
	policy := evaluation.DiscoveryPolicy{
		PolicyID:             "policy_cross_eval",
		Enabled:              true,
		SourceKinds:          []evaluation.SourceKind{evaluation.SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := accessor.UpsertDiscoveryPolicyForTenant(ctxA, policy); err != nil {
		t.Fatalf("UpsertDiscoveryPolicyForTenant A: %v", err)
	}
	policy.MaxInspectedRecords = 99
	if err := accessor.UpsertDiscoveryPolicyForTenant(ctxB, policy); !errors.Is(err, tenancy.ErrCrossTenantWrite) {
		t.Fatalf("cross-tenant policy write err=%v, want ErrCrossTenantWrite", err)
	}
	got, err := s.ListDiscoveryPolicies(context.Background(), evaluation.DiscoveryPolicyFilter{ProductListFilter: evaluation.ProductListFilter{TenantID: "ten_eval_a"}})
	if err != nil {
		t.Fatalf("ListDiscoveryPolicies: %v", err)
	}
	if len(got) != 1 || got[0].MaxInspectedRecords != 10 || got[0].TenantID != "ten_eval_a" {
		t.Fatalf("cross-tenant write mutated tenant A row: %+v", got)
	}
}

func TestEvaluationProductTenantHelpersBindRowsAndLists(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	accessor := tenancy.NewEvaluation(s, nil)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_eval", PrincipalID: "prn_eval"})
	now := time.Now().UTC()
	policy := evaluation.DiscoveryPolicy{
		PolicyID:             "policy_bind_eval",
		Enabled:              true,
		SourceKinds:          []evaluation.SourceKind{evaluation.SourceKindRun},
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		MaxInspectedRecords:  10,
		MaxEmittedCandidates: 2,
		CostBudget:           5,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := accessor.UpsertDiscoveryPolicyForTenant(ctx, policy); err != nil {
		t.Fatalf("UpsertDiscoveryPolicyForTenant: %v", err)
	}
	policies, err := accessor.ListDiscoveryPoliciesForTenant(ctx, evaluation.DiscoveryPolicyFilter{})
	if err != nil {
		t.Fatalf("ListDiscoveryPoliciesForTenant: %v", err)
	}
	if len(policies) != 1 || policies[0].TenantID != "ten_eval" {
		t.Fatalf("unexpected policies: %+v", policies)
	}
	tenantID, ok, err := s.LookupRowTenant(context.Background(), "evaluation_discovery_policies", "policy_id", policy.PolicyID)
	if err != nil {
		t.Fatalf("LookupRowTenant: %v", err)
	}
	if !ok || tenantID != "ten_eval" {
		t.Fatalf("policy row tenant=%q ok=%v, want ten_eval", tenantID, ok)
	}

	fixture := evaluation.ProductManagedFixture{
		FixtureID:         "product_fixture_bind_eval",
		DisplayName:       "Product Fixture",
		DomainClass:       evaluation.FixtureDomainSchedule,
		SourceKind:        string(evaluation.ProductResourceDiscoveredCandidate),
		CurrentRevisionID: "revision_bind_eval_1",
		ReviewState:       evaluation.ProductStatusDraft,
		SuppressionState:  evaluation.SuppressionStateNone,
		RetentionState:    evaluation.RetentionStateActive,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := accessor.UpsertProductFixtureForTenant(ctx, fixture); err != nil {
		t.Fatalf("UpsertProductFixtureForTenant: %v", err)
	}
	revision := evaluation.FixtureRevision{
		RevisionID:      "revision_bind_eval_1",
		FixtureID:       fixture.FixtureID,
		RevisionNumber:  1,
		FixturePayload:  map[string]any{"goal": "safe"},
		RedactionStatus: evaluation.RedactionStatusClean,
		CreatedAt:       now,
	}
	if err := accessor.SaveFixtureRevisionForTenant(ctx, revision); err != nil {
		t.Fatalf("SaveFixtureRevisionForTenant: %v", err)
	}
	fixtures, err := accessor.ListProductFixturesForTenant(ctx, evaluation.ProductListFilter{})
	if err != nil {
		t.Fatalf("ListProductFixturesForTenant: %v", err)
	}
	if len(fixtures) != 1 || fixtures[0].TenantID != "ten_eval" {
		t.Fatalf("unexpected fixtures: %+v", fixtures)
	}
	gotFixture, ok, err := accessor.GetProductFixtureForTenant(ctx, fixture.FixtureID)
	if err != nil {
		t.Fatalf("GetProductFixtureForTenant: %v", err)
	}
	if !ok || gotFixture.FixtureID != fixture.FixtureID {
		t.Fatalf("fixture=%+v ok=%v, want product fixture", gotFixture, ok)
	}
	revisions, err := accessor.ListFixtureRevisionsForTenant(ctx, fixture.FixtureID, 10)
	if err != nil {
		t.Fatalf("ListFixtureRevisionsForTenant: %v", err)
	}
	if len(revisions) != 1 || revisions[0].TenantID != "ten_eval" {
		t.Fatalf("unexpected revisions: %+v", revisions)
	}

	campaign := evaluation.ReplayCampaign{
		CampaignID:     "campaign_bind_eval",
		DisplayName:    "Campaign",
		Status:         evaluation.ProductStatusCompleted,
		CreatedAt:      now,
		RetentionState: evaluation.RetentionStateActive,
	}
	if err := accessor.SaveReplayCampaignForTenant(ctx, campaign); err != nil {
		t.Fatalf("SaveReplayCampaignForTenant: %v", err)
	}
	item := evaluation.CampaignItem{
		CampaignItemID:       "campaign_bind_eval_item_001",
		CampaignID:           campaign.CampaignID,
		SourceType:           evaluation.ProductResourceProductFixture,
		SourceID:             fixture.FixtureID,
		SuppressionCheckedAt: now,
		CreatedAt:            now,
	}
	if err := accessor.SaveCampaignItemForTenant(ctx, item); err != nil {
		t.Fatalf("SaveCampaignItemForTenant: %v", err)
	}
	group := evaluation.CampaignAttemptGroup{
		AttemptGroupID:    "attempt_group_bind_eval",
		CampaignID:        campaign.CampaignID,
		CampaignItemID:    item.CampaignItemID,
		Status:            evaluation.ProductStatusCompleted,
		LiveValidationIDs: []string{"ledger_bind_eval"},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := accessor.SaveCampaignAttemptGroupForTenant(ctx, group); err != nil {
		t.Fatalf("SaveCampaignAttemptGroupForTenant: %v", err)
	}
	campaigns, err := accessor.ListReplayCampaignsForTenant(ctx, evaluation.ProductListFilter{})
	if err != nil {
		t.Fatalf("ListReplayCampaignsForTenant: %v", err)
	}
	if len(campaigns) != 1 || campaigns[0].TenantID != "ten_eval" {
		t.Fatalf("unexpected campaigns: %+v", campaigns)
	}
	gotCampaign, ok, err := accessor.GetReplayCampaignForTenant(ctx, campaign.CampaignID)
	if err != nil {
		t.Fatalf("GetReplayCampaignForTenant: %v", err)
	}
	if !ok || gotCampaign.CampaignID != campaign.CampaignID {
		t.Fatalf("campaign=%+v ok=%v, want campaign", gotCampaign, ok)
	}
	items, err := accessor.ListCampaignItemsForTenant(ctx, evaluation.ProductListFilter{}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListCampaignItemsForTenant: %v", err)
	}
	if len(items) != 1 || items[0].TenantID != "ten_eval" {
		t.Fatalf("unexpected campaign items: %+v", items)
	}
	groups, err := accessor.ListCampaignAttemptGroupsForTenant(ctx, evaluation.ProductListFilter{}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListCampaignAttemptGroupsForTenant: %v", err)
	}
	if len(groups) != 1 || groups[0].TenantID != "ten_eval" || len(groups[0].LiveValidationIDs) != 1 {
		t.Fatalf("unexpected campaign groups: %+v", groups)
	}

	projection := evaluation.DashboardProjection{
		ProjectionID:         "projection_bind_eval",
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		CampaignStatusCounts: map[string]int{"completed": 1},
		GeneratedAt:          now,
	}
	if err := accessor.SaveDashboardProjectionForTenant(ctx, projection); err != nil {
		t.Fatalf("SaveDashboardProjectionForTenant: %v", err)
	}
	projections, err := accessor.ListDashboardProjectionsForTenant(ctx, evaluation.ProductListFilter{})
	if err != nil {
		t.Fatalf("ListDashboardProjectionsForTenant: %v", err)
	}
	if len(projections) != 1 || projections[0].TenantID != "ten_eval" {
		t.Fatalf("unexpected projections: %+v", projections)
	}

	inspection := evaluation.ToolCallInspection{
		InspectionID:             "inspection_bind_eval",
		CampaignID:               campaign.CampaignID,
		CampaignItemID:           item.CampaignItemID,
		ToolCallRef:              "tool_call_bind_eval",
		OriginalEvidenceRef:      "original_bind_eval",
		NonLiveReplayEvidenceRef: "replay_bind_eval",
		Classification:           evaluation.InspectionMatched,
		RedactionStatus:          evaluation.RedactionStatusClean,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := accessor.SaveToolCallInspectionForTenant(ctx, inspection); err != nil {
		t.Fatalf("SaveToolCallInspectionForTenant: %v", err)
	}
	inspections, err := accessor.ListToolCallInspectionsForTenant(ctx, evaluation.ProductListFilter{}, campaign.CampaignID)
	if err != nil {
		t.Fatalf("ListToolCallInspectionsForTenant: %v", err)
	}
	if len(inspections) != 1 || inspections[0].TenantID != "ten_eval" {
		t.Fatalf("unexpected inspections: %+v", inspections)
	}
	gotInspection, ok, err := accessor.GetToolCallInspectionForTenant(ctx, inspection.InspectionID)
	if err != nil {
		t.Fatalf("GetToolCallInspectionForTenant: %v", err)
	}
	if !ok || gotInspection.InspectionID != inspection.InspectionID {
		t.Fatalf("inspection=%+v ok=%v, want inspection", gotInspection, ok)
	}
}
