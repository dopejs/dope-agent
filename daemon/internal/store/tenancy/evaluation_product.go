package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func (a *Evaluation) UpsertDiscoveryPolicyForTenant(ctx context.Context, item evaluation.DiscoveryPolicy) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_discovery_policies", "policy_id", item.PolicyID, tenantID, "store:UpsertDiscoveryPolicyForTenant", "evaluation_discovery_policy"); err != nil {
		return err
	}
	if err := a.store.UpsertDiscoveryPolicy(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_discovery_policies", "policy_id", item.PolicyID, tenantID, "store:UpsertDiscoveryPolicyForTenant", "evaluation_discovery_policy")
}

func (a *Evaluation) ListDiscoveryPoliciesForTenant(ctx context.Context, filter evaluation.DiscoveryPolicyFilter) ([]evaluation.DiscoveryPolicy, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListDiscoveryPolicies(ctx, filter)
}

func (a *Evaluation) SaveDiscoveryRunForTenant(ctx context.Context, item evaluation.DiscoveryRun) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_discovery_runs", "discovery_run_id", item.DiscoveryRunID, tenantID, "store:SaveDiscoveryRunForTenant", "evaluation_discovery_run"); err != nil {
		return err
	}
	if err := a.store.SaveDiscoveryRun(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_discovery_runs", "discovery_run_id", item.DiscoveryRunID, tenantID, "store:SaveDiscoveryRunForTenant", "evaluation_discovery_run")
}

func (a *Evaluation) ListDiscoveryRunsForTenant(ctx context.Context, filter evaluation.DiscoveryRunFilter) ([]evaluation.DiscoveryRun, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListDiscoveryRuns(ctx, filter)
}

func (a *Evaluation) SaveDiscoveredCandidateForTenant(ctx context.Context, item evaluation.DiscoveredCandidate, evidence evaluation.CandidateEvidence) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	evidence.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_discovered_candidates", "discovered_candidate_id", item.DiscoveredCandidateID, tenantID, "store:SaveDiscoveredCandidateForTenant", "evaluation_discovered_candidate"); err != nil {
		return err
	}
	if evidence.EvidenceID != "" {
		if err := a.ensureTenantRow(ctx, "evaluation_candidate_evidence", "evidence_id", evidence.EvidenceID, tenantID, "store:SaveDiscoveredCandidateForTenant", "evaluation_candidate_evidence"); err != nil {
			return err
		}
	}
	if err := a.store.SaveDiscoveredCandidate(ctx, item, evidence); err != nil {
		return err
	}
	if err := a.bindEvaluationProductRow(ctx, "evaluation_discovered_candidates", "discovered_candidate_id", item.DiscoveredCandidateID, tenantID, "store:SaveDiscoveredCandidateForTenant", "evaluation_discovered_candidate"); err != nil {
		return err
	}
	if evidence.EvidenceID != "" {
		return a.bindEvaluationProductRow(ctx, "evaluation_candidate_evidence", "evidence_id", evidence.EvidenceID, tenantID, "store:SaveDiscoveredCandidateForTenant", "evaluation_candidate_evidence")
	}
	return nil
}

func (a *Evaluation) ListDiscoveredCandidatesForTenant(ctx context.Context, filter evaluation.DiscoveredCandidateFilter) ([]evaluation.DiscoveredCandidate, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListDiscoveredCandidates(ctx, filter)
}

func (a *Evaluation) CreateSuppressionForTenant(ctx context.Context, item evaluation.SuppressionRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_suppressions", "suppression_id", item.SuppressionID, tenantID, "store:CreateSuppressionForTenant", "evaluation_suppression"); err != nil {
		return err
	}
	if err := a.store.CreateSuppression(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_suppressions", "suppression_id", item.SuppressionID, tenantID, "store:CreateSuppressionForTenant", "evaluation_suppression")
}

func (a *Evaluation) UpsertProductFixtureForTenant(ctx context.Context, item evaluation.ProductManagedFixture) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_product_fixtures", "fixture_id", item.FixtureID, tenantID, "store:UpsertProductFixtureForTenant", "evaluation_product_fixture"); err != nil {
		return err
	}
	if err := a.store.UpsertProductFixture(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_product_fixtures", "fixture_id", item.FixtureID, tenantID, "store:UpsertProductFixtureForTenant", "evaluation_product_fixture")
}

func (a *Evaluation) ListProductFixturesForTenant(ctx context.Context, filter evaluation.ProductListFilter) ([]evaluation.ProductManagedFixture, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListProductFixtures(ctx, filter)
}

func (a *Evaluation) GetProductFixtureForTenant(ctx context.Context, fixtureID string) (evaluation.ProductManagedFixture, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return evaluation.ProductManagedFixture{}, false, err
	}
	return a.store.GetProductFixture(ctx, tenantID, fixtureID)
}

func (a *Evaluation) SaveFixtureRevisionForTenant(ctx context.Context, item evaluation.FixtureRevision) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_fixture_revisions", "revision_id", item.RevisionID, tenantID, "store:SaveFixtureRevisionForTenant", "evaluation_fixture_revision"); err != nil {
		return err
	}
	if err := a.store.SaveFixtureRevision(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_fixture_revisions", "revision_id", item.RevisionID, tenantID, "store:SaveFixtureRevisionForTenant", "evaluation_fixture_revision")
}

func (a *Evaluation) ListFixtureRevisionsForTenant(ctx context.Context, fixtureID string, limit int) ([]evaluation.FixtureRevision, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return a.store.ListFixtureRevisions(ctx, tenantID, fixtureID, limit)
}

func (a *Evaluation) SaveReplayCampaignForTenant(ctx context.Context, item evaluation.ReplayCampaign) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_campaigns", "campaign_id", item.CampaignID, tenantID, "store:SaveReplayCampaignForTenant", "evaluation_campaign"); err != nil {
		return err
	}
	if err := a.store.SaveReplayCampaign(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_campaigns", "campaign_id", item.CampaignID, tenantID, "store:SaveReplayCampaignForTenant", "evaluation_campaign")
}

func (a *Evaluation) ListReplayCampaignsForTenant(ctx context.Context, filter evaluation.ProductListFilter) ([]evaluation.ReplayCampaign, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListReplayCampaigns(ctx, filter)
}

func (a *Evaluation) GetReplayCampaignForTenant(ctx context.Context, campaignID string) (evaluation.ReplayCampaign, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return evaluation.ReplayCampaign{}, false, err
	}
	return a.store.GetReplayCampaign(ctx, tenantID, campaignID)
}

func (a *Evaluation) SaveCampaignItemForTenant(ctx context.Context, item evaluation.CampaignItem) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_campaign_items", "campaign_item_id", item.CampaignItemID, tenantID, "store:SaveCampaignItemForTenant", "evaluation_campaign_item"); err != nil {
		return err
	}
	if err := a.store.SaveCampaignItem(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_campaign_items", "campaign_item_id", item.CampaignItemID, tenantID, "store:SaveCampaignItemForTenant", "evaluation_campaign_item")
}

func (a *Evaluation) ListCampaignItemsForTenant(ctx context.Context, filter evaluation.ProductListFilter, campaignID string) ([]evaluation.CampaignItem, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListCampaignItems(ctx, filter, campaignID)
}

func (a *Evaluation) SaveCampaignAttemptGroupForTenant(ctx context.Context, item evaluation.CampaignAttemptGroup) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_campaign_attempt_groups", "attempt_group_id", item.AttemptGroupID, tenantID, "store:SaveCampaignAttemptGroupForTenant", "evaluation_campaign_attempt_group"); err != nil {
		return err
	}
	if err := a.store.SaveCampaignAttemptGroup(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_campaign_attempt_groups", "attempt_group_id", item.AttemptGroupID, tenantID, "store:SaveCampaignAttemptGroupForTenant", "evaluation_campaign_attempt_group")
}

func (a *Evaluation) ListCampaignAttemptGroupsForTenant(ctx context.Context, filter evaluation.ProductListFilter, campaignID string) ([]evaluation.CampaignAttemptGroup, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListCampaignAttemptGroups(ctx, filter, campaignID)
}

func (a *Evaluation) SaveDashboardProjectionForTenant(ctx context.Context, item evaluation.DashboardProjection) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_dashboard_projections", "projection_id", item.ProjectionID, tenantID, "store:SaveDashboardProjectionForTenant", "evaluation_dashboard_projection"); err != nil {
		return err
	}
	if err := a.store.SaveDashboardProjection(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_dashboard_projections", "projection_id", item.ProjectionID, tenantID, "store:SaveDashboardProjectionForTenant", "evaluation_dashboard_projection")
}

func (a *Evaluation) ListDashboardProjectionsForTenant(ctx context.Context, filter evaluation.ProductListFilter) ([]evaluation.DashboardProjection, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListDashboardProjections(ctx, filter)
}

func (a *Evaluation) SaveToolCallInspectionForTenant(ctx context.Context, item evaluation.ToolCallInspection) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "evaluation_tool_call_inspections", "inspection_id", item.InspectionID, tenantID, "store:SaveToolCallInspectionForTenant", "evaluation_tool_call_inspection"); err != nil {
		return err
	}
	if err := a.store.SaveToolCallInspection(ctx, item); err != nil {
		return err
	}
	return a.bindEvaluationProductRow(ctx, "evaluation_tool_call_inspections", "inspection_id", item.InspectionID, tenantID, "store:SaveToolCallInspectionForTenant", "evaluation_tool_call_inspection")
}

func (a *Evaluation) ListToolCallInspectionsForTenant(ctx context.Context, filter evaluation.ProductListFilter, campaignID string) ([]evaluation.ToolCallInspection, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	filter.TenantID = tenantID
	return a.store.ListToolCallInspections(ctx, filter, campaignID)
}

func (a *Evaluation) GetToolCallInspectionForTenant(ctx context.Context, inspectionID string) (evaluation.ToolCallInspection, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return evaluation.ToolCallInspection{}, false, err
	}
	return a.store.GetToolCallInspection(ctx, tenantID, inspectionID)
}

func (a *Evaluation) ApplyProductRetentionForTenant(ctx context.Context, filter evaluation.RetentionApplicationFilter) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	filter.TenantID = tenantID
	return a.store.ApplyRetention(ctx, filter)
}

func (a *Evaluation) bindEvaluationProductRow(ctx context.Context, table, pkColumn, pk, tenantID, surface, resourceKind string) error {
	if err := a.store.BindRowTenant(ctx, table, pkColumn, pk, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, surface, resourceKind)
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
