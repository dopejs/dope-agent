package store

import (
	"context"
	"testing"
)

func TestEvaluationProductSchemaTablesAndIndexes(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	requiredColumns := map[string][]string{
		"evaluation_discovery_policies": {
			"policy_id", "tenant_id", "enabled", "window_start", "window_end",
			"max_inspected_records", "max_emitted_candidates", "cost_budget",
			"created_at", "updated_at", "document_json",
		},
		"evaluation_discovery_runs": {
			"discovery_run_id", "tenant_id", "policy_id", "status", "cursor",
			"started_at", "completed_at", "updated_at", "document_json",
		},
		"evaluation_discovered_candidates": {
			"discovered_candidate_id", "tenant_id", "discovery_run_id", "source_kind",
			"source_id", "score_band", "readiness_status", "suppression_state",
			"retention_state", "created_at", "updated_at", "expires_at", "document_json",
		},
		"evaluation_candidate_evidence": {
			"evidence_id", "tenant_id", "discovered_candidate_id", "redaction_status",
			"materialization_allowed", "retention_state", "created_at", "expires_at", "document_json",
		},
		"evaluation_suppressions": {
			"suppression_id", "tenant_id", "target_kind", "target_id", "target_source_ref",
			"reason_code", "active", "created_at", "expires_at", "document_json",
		},
		"evaluation_product_fixtures": {
			"fixture_id", "tenant_id", "display_name", "domain_class", "source_kind",
			"source_candidate_id", "current_revision_id", "review_state",
			"suppression_state", "retention_state", "created_at", "updated_at", "document_json",
		},
		"evaluation_fixture_revisions": {
			"revision_id", "fixture_id", "tenant_id", "revision_number",
			"redaction_status", "created_at", "document_json",
		},
		"evaluation_campaigns": {
			"campaign_id", "tenant_id", "display_name", "status", "created_at",
			"started_at", "completed_at", "published_at", "retention_state",
			"idempotency_key", "document_json",
		},
		"evaluation_campaign_items": {
			"campaign_item_id", "campaign_id", "tenant_id", "source_type", "source_id",
			"suppression_checked_at", "created_at", "document_json",
		},
		"evaluation_campaign_attempt_groups": {
			"attempt_group_id", "campaign_id", "campaign_item_id", "tenant_id",
			"status", "drift_count", "failure_count", "unsupported_count",
			"operator_action_needed_count", "created_at", "updated_at", "document_json",
		},
		"evaluation_dashboard_projections": {
			"projection_id", "tenant_id", "window_start", "window_end",
			"generated_at", "cursor", "retention_state", "document_json",
		},
		"evaluation_tool_call_inspections": {
			"inspection_id", "tenant_id", "campaign_id", "campaign_item_id",
			"tool_call_ref", "classification", "redaction_status", "retention_state",
			"created_at", "updated_at", "document_json",
		},
		"evaluation_retention_applications": {
			"application_id", "tenant_id", "resource_kind", "resource_id",
			"dry_run", "outcome", "applied_at", "document_json",
		},
	}

	r41Tables := []string{
		"evaluation_discovery_policies",
		"evaluation_discovery_runs",
		"evaluation_discovered_candidates",
		"evaluation_candidate_evidence",
		"evaluation_suppressions",
		"evaluation_product_fixtures",
		"evaluation_fixture_revisions",
		"evaluation_campaigns",
		"evaluation_campaign_items",
		"evaluation_campaign_attempt_groups",
		"evaluation_dashboard_projections",
		"evaluation_tool_call_inspections",
		"evaluation_retention_applications",
	}
	for _, table := range r41Tables {
		columns, ok := requiredColumns[table]
		if !ok {
			t.Fatalf("missing schema assertions for R41 table %s", table)
		}
		got := loadStoreColumns(t, s, ctx, table)
		for _, column := range columns {
			if !got[column] {
				t.Fatalf("table %s missing column %s", table, column)
			}
		}
	}

	indexes := []string{
		"idx_eval_discovery_policies_tenant_enabled",
		"idx_eval_discovery_runs_tenant_status",
		"idx_eval_discovered_candidates_tenant_ready",
		"idx_eval_discovered_candidates_tenant_source",
		"idx_eval_candidate_evidence_tenant_candidate",
		"idx_eval_suppressions_tenant_target",
		"idx_eval_product_fixtures_tenant_review",
		"idx_eval_fixture_revisions_tenant_fixture",
		"idx_eval_campaigns_tenant_status",
		"idx_eval_campaign_items_tenant_campaign",
		"idx_eval_campaign_attempt_groups_tenant_campaign",
		"idx_eval_dashboard_projections_tenant_generated",
		"idx_eval_tool_call_inspections_tenant_campaign",
		"idx_eval_retention_applications_tenant_resource",
	}
	for _, indexName := range indexes {
		if !storeIndexExists(t, s, ctx, indexName) {
			t.Fatalf("missing evaluation product index %s", indexName)
		}
	}
}
