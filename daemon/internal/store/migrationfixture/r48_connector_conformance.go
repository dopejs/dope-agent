package migrationfixture

import (
	"context"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

var R48ConnectorConformanceTableNames = []string{
	"connector_conformance_results",
	"connector_diagnostic_states",
	"connector_diagnostic_redaction_failures",
	"connector_delivery_boundaries",
}

type R48ConnectorConformanceFixture struct {
	TenantIDs        []string
	ExpectedRowCount map[string]int
}

func BuildR48ConnectorConformanceFixture() R48ConnectorConformanceFixture {
	counts := map[string]int{}
	for _, table := range R48ConnectorConformanceTableNames {
		counts[table] = 2
	}
	return R48ConnectorConformanceFixture{
		TenantIDs:        []string{"ten_connector_alpha", "ten_connector_beta"},
		ExpectedRowCount: counts,
	}
}

func SeedR48ConnectorConformanceRows(ctx context.Context, s *store.SQLiteStore) (R48ConnectorConformanceFixture, error) {
	fixture := BuildR48ConnectorConformanceFixture()
	for i, tenantID := range fixture.TenantIDs {
		suffix := fmt.Sprintf("%d", i+1)
		connectorID := "r48_connector_" + suffix
		resultID := "r48_conformance_result_" + suffix
		diagnosticID := "r48_diagnostic_state_" + suffix
		failureID := "r48_redaction_failure_" + suffix
		boundaryID := "r48_delivery_boundary_" + suffix
		if err := exec(ctx, s, `INSERT INTO connector_conformance_results (conformance_result_id, tenant_id, connector_kind, connector_id, scenario_id, area, result, reason_code, redaction_status, evidence_timestamp, retention_expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			resultID, tenantID, "discord", connectorID, "discord.direct.pass", "direct_message", "pass", nil, "redacted", ts, ts, `{"result":"pass"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO connector_diagnostic_states (diagnostic_state_id, tenant_id, connector_id, connector_account_id, status, reason_code, remediation_owner, user_visible_severity, retry_safety, evidence_timestamp, stale_after, freshness_state, redaction_status, retention_expires_at, redaction_failure_id, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			diagnosticID, tenantID, connectorID, "acct_"+suffix, "healthy", "healthy", "none_required", "info", "no_action_needed", ts, ts, "fresh", "redacted", ts, nil, `{"reasonCode":"healthy"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO connector_diagnostic_redaction_failures (redaction_failure_id, tenant_id, connector_id, diagnostic_state_id, reason_code, occurred_at, retention_expires_at, document_json)
			VALUES (?,?,?,?,?,?,?,?)`,
			failureID, tenantID, connectorID, diagnosticID, "redaction_failed_closed", ts, ts, `{"redactionStatus":"suppressed"}`); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO connector_delivery_boundaries (boundary_id, tenant_id, connector_id, foreground_reply_outcome_id, background_delivery_id, transport_kind, separation_status, created_at, document_json)
			VALUES (?,?,?,?,?,?,?,?,?)`,
			boundaryID, tenantID, connectorID, "foreground_"+suffix, "background_"+suffix, "discord", "separate_truths", ts, `{"separationStatus":"separate_truths"}`); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

func CountR48ConnectorConformanceRows(ctx context.Context, s *store.SQLiteStore) (map[string]int, error) {
	counts := map[string]int{}
	for _, table := range R48ConnectorConformanceTableNames {
		var count int
		if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}
