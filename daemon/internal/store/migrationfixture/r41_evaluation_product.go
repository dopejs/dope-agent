package migrationfixture

import (
	"context"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// R41EvaluationProductTableNames lists the product-evaluation tables expected
// from the Roadmap 41 storage migration.
var R41EvaluationProductTableNames = []string{
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

// R41EvaluationProductFixtureNotes documents the minimum migration fixture
// coverage expected when Roadmap 41 tables are implemented.
var R41EvaluationProductFixtureNotes = []string{
	"seed at least two tenants to prove tenant-scoped indexes and accessors",
	"seed discovery policy, run, candidate, evidence, and suppression rows",
	"seed product fixture head plus immutable revisions",
	"seed campaigns with immutable source snapshots and attempt groups",
	"seed dashboard and inspection rows with retention-state variants",
	"seed redaction-failed and retention-applied audit/event examples",
}

type R41EvaluationProductFixture struct {
	TenantIDs        []string
	ExpectedRowCount map[string]int
}

func BuildR41EvaluationProductFixture() R41EvaluationProductFixture {
	counts := map[string]int{}
	for _, table := range R41EvaluationProductTableNames {
		counts[table] = 2
	}
	return R41EvaluationProductFixture{
		TenantIDs:        []string{"ten_eval_alpha", "ten_eval_beta"},
		ExpectedRowCount: counts,
	}
}

func SeedR41EvaluationProductRows(ctx context.Context, s *store.SQLiteStore) (R41EvaluationProductFixture, error) {
	fixture := BuildR41EvaluationProductFixture()
	for i, tenantID := range fixture.TenantIDs {
		suffix := fmt.Sprintf("%d", i+1)
		policyID := "r41_policy_" + suffix
		runID := "r41_discovery_run_" + suffix
		candidateID := "r41_discovered_candidate_" + suffix
		evidenceID := "r41_evidence_" + suffix
		fixtureID := "r41_fixture_" + suffix
		revisionID := "r41_revision_" + suffix
		campaignID := "r41_campaign_" + suffix
		itemID := "r41_campaign_item_" + suffix
		groupID := "r41_attempt_group_" + suffix
		projectionID := "r41_projection_" + suffix
		inspectionID := "r41_inspection_" + suffix
		retentionID := "r41_retention_" + suffix

		if err := exec(ctx, s, `INSERT INTO evaluation_discovery_policies (policy_id, tenant_id, enabled, window_start, window_end, max_inspected_records, max_emitted_candidates, cost_budget, created_by, created_at, updated_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			policyID, tenantID, 1, ts, ts, 50, 10, 100, "prn_"+suffix, ts, ts, `{"redactionStatus":"clean"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_discovery_runs (discovery_run_id, tenant_id, policy_id, status, cursor, window_start, window_end, max_inspected_records, max_emitted_candidates, cost_budget, inspected_records, emitted_candidates, started_by, started_at, completed_at, updated_at, idempotency_key, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			runID, tenantID, policyID, "completed", "cursor_"+suffix, ts, ts, 50, 10, 100, 12, 1, "prn_"+suffix, ts, ts, ts, "idem_"+suffix, `{"status":"completed"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_discovered_candidates (discovered_candidate_id, tenant_id, discovery_run_id, source_kind, source_id, score, score_band, redaction_status, evidence_ref, readiness_status, suppression_state, retention_state, created_at, updated_at, expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			candidateID, tenantID, runID, "run", "run_seed", 0.92, "high", "redacted", evidenceID, "ready", "none", "active", ts, ts, nil, `{"evidence":"redacted"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_candidate_evidence (evidence_id, tenant_id, discovered_candidate_id, redaction_status, materialization_allowed, retention_state, created_at, expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			evidenceID, tenantID, candidateID, "redacted", 1, "active", ts, nil, `{"redactedPayload":{"token":"[REDACTED]"}}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_suppressions (suppression_id, tenant_id, target_kind, target_id, target_source_ref, reason_code, created_by, active, created_at, expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			"r41_suppression_"+suffix, tenantID, "discovered_candidate", candidateID, nil, "operator_hidden", "prn_"+suffix, i%2, ts, nil, `{"active":true}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_product_fixtures (fixture_id, tenant_id, display_name, domain_class, source_kind, source_candidate_id, current_revision_id, review_state, suppression_state, retention_state, created_by, created_at, updated_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			fixtureID, tenantID, "R41 fixture "+suffix, "runtime", "run", candidateID, revisionID, "approved", "none", "active", "prn_"+suffix, ts, ts, `{"displayName":"R41 fixture"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_fixture_revisions (revision_id, fixture_id, tenant_id, revision_number, redaction_status, created_by, created_at, document_json)
			VALUES (?,?,?,?,?,?,?,?)`,
			revisionID, fixtureID, tenantID, 1, "redacted", "prn_"+suffix, ts, `{"payload":{"secret":"[REDACTED]"}}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_campaigns (campaign_id, tenant_id, display_name, status, created_at, started_at, completed_at, published_at, retention_state, idempotency_key, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			campaignID, tenantID, "R41 campaign "+suffix, "completed", ts, ts, ts, nil, "active", "campaign_idem_"+suffix, `{"status":"completed"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_campaign_items (campaign_item_id, campaign_id, tenant_id, source_type, source_id, suppression_checked_at, created_at, document_json)
			VALUES (?,?,?,?,?,?,?,?)`,
			itemID, campaignID, tenantID, "product_fixture", fixtureID, ts, ts, `{"sourceSnapshot":{"fixtureId":"redacted"}}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_campaign_attempt_groups (attempt_group_id, campaign_id, campaign_item_id, tenant_id, status, drift_count, failure_count, unsupported_count, operator_action_needed_count, created_at, updated_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			groupID, campaignID, itemID, tenantID, "completed", i, 0, 0, 0, ts, ts, `{"summary":"ok"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_dashboard_projections (projection_id, tenant_id, window_start, window_end, generated_at, cursor, document_json)
			VALUES (?,?,?,?,?,?,?)`,
			projectionID, tenantID, ts, ts, ts, "cursor_"+suffix, `{"campaignStatusCounts":{"completed":1}}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_tool_call_inspections (inspection_id, tenant_id, campaign_id, campaign_item_id, tool_call_ref, classification, redaction_status, created_at, updated_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			inspectionID, tenantID, campaignID, itemID, "tool_call_"+suffix, "matched", "redacted", ts, ts, `{"diffSummary":"redacted"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO evaluation_retention_applications (application_id, tenant_id, resource_kind, resource_id, dry_run, outcome, applied_at, document_json)
			VALUES (?,?,?,?,?,?,?,?)`,
			retentionID, tenantID, "campaign", campaignID, i%2, "retained", ts, `{"outcome":"retained"}`); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

func CountR41EvaluationProductRows(ctx context.Context, s *store.SQLiteStore) (map[string]int, error) {
	counts := map[string]int{}
	for _, table := range R41EvaluationProductTableNames {
		var count int
		if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}
