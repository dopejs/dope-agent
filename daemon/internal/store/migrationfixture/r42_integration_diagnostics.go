package migrationfixture

import (
	"context"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

var R42IntegrationDiagnosticTableNames = []string{
	"integration_diagnostic_runs",
	"integration_diagnostic_results",
	"integration_provider_classifications",
	"integration_smoke_reports",
	"integration_smoke_probe_outcomes",
	"integration_diagnostic_retention",
}

type R42IntegrationDiagnosticFixture struct {
	TenantIDs        []string
	ExpectedRowCount map[string]int
}

func BuildR42IntegrationDiagnosticFixture() R42IntegrationDiagnosticFixture {
	counts := map[string]int{}
	for _, table := range R42IntegrationDiagnosticTableNames {
		counts[table] = 2
	}
	return R42IntegrationDiagnosticFixture{
		TenantIDs:        []string{"ten_diag_alpha", "ten_diag_beta"},
		ExpectedRowCount: counts,
	}
}

func SeedR42IntegrationDiagnosticRows(ctx context.Context, s *store.SQLiteStore) (R42IntegrationDiagnosticFixture, error) {
	fixture := BuildR42IntegrationDiagnosticFixture()
	for i, tenantID := range fixture.TenantIDs {
		suffix := fmt.Sprintf("%d", i+1)
		runID := "r42_diag_run_" + suffix
		resultID := "r42_diag_result_" + suffix
		classificationID := "r42_classification_" + suffix
		smokeID := "r42_smoke_" + suffix
		outcomeID := "r42_probe_" + suffix
		retentionID := "r42_retention_" + suffix
		integrationID := "r42_integration_" + suffix
		if err := exec(ctx, s, `INSERT INTO integration_diagnostic_runs (diagnostic_run_id, tenant_id, integration_id, integration_account_id, domain_kind, provider_kind, requested_by, trigger, status, started_at, completed_at, failure_reason_code, redaction_status, retention_expires_at, idempotency_key, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			runID, tenantID, integrationID, "acct_"+suffix, "calendar", "feishu_lark", "operator_"+suffix, "operator_inspection", "completed", ts, ts, nil, "redacted", ts, "idem_"+suffix, `{"status":"completed"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO integration_diagnostic_results (diagnostic_result_id, tenant_id, integration_id, integration_account_id, domain_kind, provider_kind, capability, status, reason_code, remediation_owner, retry_safety, checked_at, stale_after, freshness_state, run_id, redaction_status, retention_expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			resultID, tenantID, integrationID, "acct_"+suffix, "calendar", "feishu_lark", "calendar.read", "healthy", "healthy", "none_required", "no_action_needed", ts, ts, "fresh", runID, "redacted", ts, `{"reasonCode":"healthy"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO integration_provider_classifications (classification_id, tenant_id, provider_kind, domain_kind, integration_id, operation_class, reason_code, retry_safety, remediation_owner, redaction_status, created_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			classificationID, tenantID, "feishu_lark", "calendar", integrationID, "calendar.read", "healthy", "no_action_needed", "none_required", "redacted", ts, `{"reasonCode":"healthy"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO integration_smoke_reports (smoke_report_id, tenant_id, report_kind, requested_by, status, started_at, completed_at, published_at, retention_expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			smokeID, tenantID, "diagnostic", "operator_"+suffix, "completed", ts, ts, nil, ts, `{"status":"completed"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO integration_smoke_probe_outcomes (probe_outcome_id, tenant_id, smoke_report_id, integration_id, integration_account_id, domain_kind, provider_kind, probe_action, result, reason_code, retry_safety, checked_at, redaction_status, retention_expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			outcomeID, tenantID, smokeID, integrationID, "acct_"+suffix, "calendar", "feishu_lark", "calendar.read", "passed", "healthy", "no_action_needed", ts, "redacted", ts, `{"result":"passed"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO integration_diagnostic_retention (retention_record_id, tenant_id, target_kind, target_id, policy_ref, default_expires_at, effective_expires_at, retention_state, applied_at, created_at, updated_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			retentionID, tenantID, "diagnostic_run", runID, nil, ts, ts, "active", nil, ts, ts, `{"retentionState":"active"}`); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

func CountR42IntegrationDiagnosticRows(ctx context.Context, s *store.SQLiteStore) (map[string]int, error) {
	counts := map[string]int{}
	for _, table := range R42IntegrationDiagnosticTableNames {
		var count int
		if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}
