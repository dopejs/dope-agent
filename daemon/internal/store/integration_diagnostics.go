package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

func (s *SQLiteStore) SaveIntegrationDiagnosticRun(ctx context.Context, item integrations.DiagnosticRun) error {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(item.TenantID) == "" {
		if tenantID, ok := s.ResolveActiveTenantBinding(ctx).(string); ok {
			item.TenantID = tenantID
		}
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal integration diagnostic run: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO integration_diagnostic_runs (
			diagnostic_run_id, tenant_id, integration_id, integration_account_id,
			domain_kind, provider_kind, requested_by, trigger, status, started_at,
			completed_at, failure_reason_code, redaction_status, retention_expires_at,
			idempotency_key, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(diagnostic_run_id) DO UPDATE SET
			tenant_id = COALESCE(integration_diagnostic_runs.tenant_id, excluded.tenant_id),
			status = excluded.status,
			completed_at = excluded.completed_at,
			failure_reason_code = excluded.failure_reason_code,
			redaction_status = excluded.redaction_status,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, item.DiagnosticRunID, item.TenantID, item.IntegrationID, nullString(item.IntegrationAccountID),
		nullString(item.DomainKind), nullString(item.ProviderKind), item.RequestedBy, item.Trigger, string(item.Status),
		formatProductTime(item.StartedAt), nullableTimeString(item.CompletedAt), nullString(string(item.FailureReasonCode)),
		string(item.RedactionStatus), formatProductTime(item.RetentionExpiresAt), nullString(item.IdempotencyKey), string(document))
	if err != nil {
		return fmt.Errorf("save integration diagnostic run %s: %w", item.DiagnosticRunID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveIntegrationDiagnosticResult(ctx context.Context, item integrations.DiagnosticResult) error {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(item.TenantID) == "" {
		if tenantID, ok := s.ResolveActiveTenantBinding(ctx).(string); ok {
			item.TenantID = tenantID
		}
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal integration diagnostic result: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO integration_diagnostic_results (
			diagnostic_result_id, tenant_id, integration_id, integration_account_id,
			domain_kind, provider_kind, capability, status, reason_code,
			remediation_owner, retry_safety, checked_at, stale_after, freshness_state,
			run_id, redaction_status, retention_expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(diagnostic_result_id) DO UPDATE SET
			tenant_id = COALESCE(integration_diagnostic_results.tenant_id, excluded.tenant_id),
			status = excluded.status,
			reason_code = excluded.reason_code,
			remediation_owner = excluded.remediation_owner,
			retry_safety = excluded.retry_safety,
			checked_at = excluded.checked_at,
			stale_after = excluded.stale_after,
			freshness_state = excluded.freshness_state,
			redaction_status = excluded.redaction_status,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, item.DiagnosticResultID, item.TenantID, item.IntegrationID, nullString(item.IntegrationAccountID),
		item.DomainKind, item.ProviderKind, item.Capability, string(item.Status), string(item.ReasonCode),
		string(item.RemediationOwner), string(item.RetrySafety), formatProductTime(item.CheckedAt),
		formatProductTime(item.StaleAfter), string(item.FreshnessState), nullString(item.RunID),
		string(item.RedactionStatus), formatProductTime(item.RetentionExpiresAt), string(document))
	if err != nil {
		return fmt.Errorf("save integration diagnostic result %s: %w", item.DiagnosticResultID, err)
	}
	return nil
}

func (s *SQLiteStore) LatestIntegrationDiagnosticResults(ctx context.Context, filter integrations.DiagnosticResultFilter) ([]integrations.DiagnosticResult, error) {
	tenantID := strings.TrimSpace(filter.TenantID)
	if tenantID == "" {
		if resolved, ok := s.ResolveActiveTenantBinding(ctx).(string); ok {
			tenantID = resolved
		}
	}
	query := `SELECT document_json FROM integration_diagnostic_results WHERE tenant_id = ?`
	args := []any{tenantID}
	if filter.IntegrationID != "" {
		query += ` AND integration_id = ?`
		args = append(args, filter.IntegrationID)
	}
	if filter.DomainKind != "" {
		query += ` AND domain_kind = ?`
		args = append(args, filter.DomainKind)
	}
	if filter.ProviderKind != "" {
		query += ` AND provider_kind = ?`
		args = append(args, filter.ProviderKind)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	if filter.ReasonCode != "" {
		query += ` AND reason_code = ?`
		args = append(args, string(filter.ReasonCode))
	}
	if !filter.IncludeExpired {
		query += ` AND retention_expires_at > ?`
		args = append(args, formatProductTime(time.Now().UTC()))
	}
	query += ` ORDER BY checked_at DESC, diagnostic_result_id DESC LIMIT ?`
	args = append(args, normalizeDiagnosticLimit(filter.Limit))
	items, err := scanDiagnosticDocuments[integrations.DiagnosticResult](ctx, s.db, query, args, "integration diagnostic results")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for index := range items {
		items[index] = integrations.RefreshDiagnosticResultFreshness(items[index], now)
	}
	return items, nil
}

func (s *SQLiteStore) ListIntegrationDiagnosticRuns(ctx context.Context, filter integrations.DiagnosticRunFilter) ([]integrations.DiagnosticRun, error) {
	tenantID := strings.TrimSpace(filter.TenantID)
	if tenantID == "" {
		if resolved, ok := s.ResolveActiveTenantBinding(ctx).(string); ok {
			tenantID = resolved
		}
	}
	query := `SELECT document_json FROM integration_diagnostic_runs WHERE tenant_id = ?`
	args := []any{tenantID}
	if filter.IntegrationID != "" {
		query += ` AND integration_id = ?`
		args = append(args, filter.IntegrationID)
	}
	if filter.DomainKind != "" {
		query += ` AND domain_kind = ?`
		args = append(args, filter.DomainKind)
	}
	if filter.ProviderKind != "" {
		query += ` AND provider_kind = ?`
		args = append(args, filter.ProviderKind)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	if filter.ReasonCode != "" {
		query += ` AND failure_reason_code = ?`
		args = append(args, string(filter.ReasonCode))
	}
	if !filter.IncludeExpired {
		query += ` AND retention_expires_at > ?`
		args = append(args, formatProductTime(time.Now().UTC()))
	}
	query += ` ORDER BY started_at DESC, diagnostic_run_id DESC LIMIT ?`
	args = append(args, normalizeDiagnosticLimit(filter.Limit))
	return scanDiagnosticDocuments[integrations.DiagnosticRun](ctx, s.db, query, args, "integration diagnostic runs")
}

func (s *SQLiteStore) GetIntegrationDiagnosticRun(ctx context.Context, tenantID, runID string, includeExpired bool) (integrations.DiagnosticRun, bool, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		if resolved, ok := s.ResolveActiveTenantBinding(ctx).(string); ok {
			tenantID = resolved
		}
	}
	query := `SELECT document_json FROM integration_diagnostic_runs WHERE tenant_id = ? AND diagnostic_run_id = ?`
	args := []any{tenantID, strings.TrimSpace(runID)}
	if !includeExpired {
		query += ` AND retention_expires_at > ?`
		args = append(args, formatProductTime(time.Now().UTC()))
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return integrations.DiagnosticRun{}, false, nil
		}
		return integrations.DiagnosticRun{}, false, fmt.Errorf("get integration diagnostic run %s: %w", runID, err)
	}
	var item integrations.DiagnosticRun
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return integrations.DiagnosticRun{}, false, fmt.Errorf("decode integration diagnostic run %s: %w", runID, err)
	}
	return item, true, nil
}

func (s *SQLiteStore) SaveProviderClassification(ctx context.Context, item integrations.ProviderErrorClassification) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal integration provider classification: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO integration_provider_classifications (
			classification_id, tenant_id, provider_kind, domain_kind, integration_id,
			operation_class, reason_code, retry_safety, remediation_owner,
			redaction_status, created_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(classification_id) DO UPDATE SET
			reason_code = excluded.reason_code,
			retry_safety = excluded.retry_safety,
			remediation_owner = excluded.remediation_owner,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, item.ClassificationID, item.TenantID, item.ProviderKind, item.DomainKind, nullString(item.IntegrationID),
		nullString(item.OperationClass), string(item.ReasonCode), string(item.RetrySafety),
		string(item.RemediationOwner), string(item.RedactionStatus), formatProductTime(item.CreatedAt), string(document))
	if err != nil {
		return fmt.Errorf("save provider classification %s: %w", item.ClassificationID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveSmokeMatrixReport(ctx context.Context, item opsreadiness.SmokeMatrixReport) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal smoke matrix report: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO integration_smoke_reports (
			smoke_report_id, tenant_id, report_kind, requested_by, status, started_at,
			completed_at, published_at, retention_expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(smoke_report_id) DO UPDATE SET
			status = excluded.status,
			completed_at = excluded.completed_at,
			published_at = excluded.published_at,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, item.SmokeReportID, item.TenantID, item.ReportKind, item.RequestedBy, string(item.Status),
		formatProductTime(item.StartedAt), nullableTimeString(item.CompletedAt), nullableTimeString(item.PublishedAt),
		formatProductTime(item.RetentionExpiresAt), string(document))
	if err != nil {
		return fmt.Errorf("save smoke matrix report %s: %w", item.SmokeReportID, err)
	}
	for _, outcome := range item.ProbeOutcomes {
		if err := s.SaveSmokeProbeOutcome(ctx, outcome); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) SaveSmokeProbeOutcome(ctx context.Context, item opsreadiness.SmokeProbeOutcome) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal smoke probe outcome: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO integration_smoke_probe_outcomes (
			probe_outcome_id, tenant_id, smoke_report_id, integration_id,
			integration_account_id, domain_kind, provider_kind, probe_action,
			result, reason_code, retry_safety, checked_at, redaction_status,
			retention_expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(probe_outcome_id) DO UPDATE SET
			result = excluded.result,
			reason_code = excluded.reason_code,
			retry_safety = excluded.retry_safety,
			redaction_status = excluded.redaction_status,
			retention_expires_at = excluded.retention_expires_at,
			document_json = excluded.document_json
	`, item.ProbeOutcomeID, item.TenantID, item.SmokeReportID, item.IntegrationID,
		nullString(item.IntegrationAccountID), item.DomainKind, item.ProviderKind, item.ProbeAction,
		string(item.Result), item.ReasonCode, item.RetrySafety, formatProductTime(item.CheckedAt),
		item.RedactionStatus, formatProductTime(item.RetentionExpiresAt), string(document))
	if err != nil {
		return fmt.Errorf("save smoke probe outcome %s: %w", item.ProbeOutcomeID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveDiagnosticRetentionRecord(ctx context.Context, item integrations.DiagnosticRetentionRecord) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal diagnostic retention record: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO integration_diagnostic_retention (
			retention_record_id, tenant_id, target_kind, target_id, policy_ref,
			default_expires_at, effective_expires_at, retention_state, applied_at,
			created_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(retention_record_id) DO UPDATE SET
			effective_expires_at = excluded.effective_expires_at,
			retention_state = excluded.retention_state,
			applied_at = excluded.applied_at,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.RetentionRecordID, item.TenantID, item.TargetKind, item.TargetID, nullString(item.PolicyRef),
		formatProductTime(item.DefaultExpiresAt), formatProductTime(item.EffectiveExpiresAt), string(item.RetentionState),
		nullableTimeString(item.AppliedAt), formatProductTime(item.CreatedAt), formatProductTime(item.UpdatedAt), string(document))
	if err != nil {
		return fmt.Errorf("save diagnostic retention record %s: %w", item.RetentionRecordID, err)
	}
	return nil
}

func (s *SQLiteStore) ExpiredDiagnosticRetentionRecords(ctx context.Context, tenantID string, now time.Time, limit int) ([]integrations.DiagnosticRetentionRecord, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	query := `SELECT document_json FROM integration_diagnostic_retention WHERE tenant_id = ? AND effective_expires_at <= ? AND retention_state = ? ORDER BY effective_expires_at DESC, retention_record_id DESC LIMIT ?`
	return scanDiagnosticDocuments[integrations.DiagnosticRetentionRecord](ctx, s.db, query, []any{tenantID, formatProductTime(now), string(integrations.DiagnosticRetentionActive), normalizeDiagnosticLimit(limit)}, "diagnostic retention records")
}

func (s *SQLiteStore) ApplyExpiredDiagnosticRetentionRecords(ctx context.Context, tenantID string, now time.Time, limit int) ([]integrations.DiagnosticRetentionRecord, error) {
	if s == nil {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	items, err := s.ExpiredDiagnosticRetentionRecords(ctx, tenantID, now, limit)
	if err != nil {
		return nil, err
	}
	for idx := range items {
		appliedAt := now.UTC()
		items[idx].RetentionState = integrations.DiagnosticRetentionExpired
		items[idx].AppliedAt = &appliedAt
		items[idx].UpdatedAt = appliedAt
		if err := s.SaveDiagnosticRetentionRecord(ctx, items[idx]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func normalizeDiagnosticLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func scanDiagnosticDocuments[T any](ctx context.Context, db *sql.DB, query string, args []any, label string) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", label, err)
	}
	defer rows.Close()
	items := []T{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan %s: %w", label, err)
		}
		var item T
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode %s: %w", label, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", label, err)
	}
	return items, nil
}
