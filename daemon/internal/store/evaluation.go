package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func (s *SQLiteStore) UpsertLiveValidationAttempt(ctx context.Context, item livevalidation.Attempt) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation attempt: %w", err)
	}
	tenantID := coalesceString(item.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_attempts (
			validation_id, tenant_id, candidate_id, source_attempt_id, environment_scope, status,
			comparison_id, created_at, started_at, completed_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(validation_id) DO UPDATE SET
			tenant_id = COALESCE(live_validation_attempts.tenant_id, excluded.tenant_id),
			candidate_id = excluded.candidate_id,
			source_attempt_id = excluded.source_attempt_id,
			environment_scope = excluded.environment_scope,
			status = excluded.status,
			comparison_id = excluded.comparison_id,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.ValidationID, tenantID, item.CandidateID, nullString(item.SourceAttemptID), item.EnvironmentScope, string(item.Status),
		nullString(item.ComparisonID), item.CreatedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(item.StartedAt),
		nullableTimeString(item.CompletedAt), item.UpdatedAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("upsert live validation attempt %s: %w", item.ValidationID, err)
	}
	return nil
}

func (s *SQLiteStore) GetLiveValidationAttempt(ctx context.Context, tenantID, validationID string) (livevalidation.Attempt, bool, error) {
	if s == nil || validationID == "" {
		return livevalidation.Attempt{}, false, nil
	}
	query := `SELECT document_json FROM live_validation_attempts WHERE validation_id = ?`
	args := []any{validationID}
	if tenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, tenantID)
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return livevalidation.Attempt{}, false, nil
		}
		return livevalidation.Attempt{}, false, fmt.Errorf("get live validation attempt %s: %w", validationID, err)
	}
	var item livevalidation.Attempt
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return livevalidation.Attempt{}, false, fmt.Errorf("decode live validation attempt %s: %w", validationID, err)
	}
	return item, true, nil
}

func (s *SQLiteStore) ListLiveValidationAttempts(ctx context.Context, filter livevalidation.AttemptFilter) ([]livevalidation.Attempt, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM live_validation_attempts WHERE 1 = 1`
	args := make([]any, 0, 5)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.EnvironmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, filter.EnvironmentScope)
	}
	if filter.CandidateID != "" {
		query += ` AND candidate_id = ?`
		args = append(args, filter.CandidateID)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	query += ` ORDER BY updated_at DESC, validation_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list live validation attempts: %w", err)
	}
	defer rows.Close()
	items := []livevalidation.Attempt{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item livevalidation.Attempt
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode live validation attempt: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertLiveValidationScope(ctx context.Context, item livevalidation.SideEffectScope, tenantID string) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation scope: %w", err)
	}
	tenantID = coalesceString(tenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_scopes (
			scope_id, validation_id, tenant_id, approval_mode, declared_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(scope_id) DO UPDATE SET
			validation_id = excluded.validation_id,
			tenant_id = COALESCE(live_validation_scopes.tenant_id, excluded.tenant_id),
			approval_mode = excluded.approval_mode,
			declared_at = excluded.declared_at,
			document_json = excluded.document_json
	`, item.ScopeID, item.ValidationID, tenantID, string(item.ApprovalMode), item.DeclaredAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("upsert live validation scope %s: %w", item.ScopeID, err)
	}
	return nil
}

func (s *SQLiteStore) UpsertLiveValidationApproval(ctx context.Context, item livevalidation.FreshApproval) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation approval: %w", err)
	}
	tenantID := coalesceString(item.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_approvals (
			approval_id, validation_id, tenant_id, approval_target, tool_class, action_ref,
			status, requested_at, resolved_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(approval_id) DO UPDATE SET
			validation_id = excluded.validation_id,
			tenant_id = COALESCE(live_validation_approvals.tenant_id, excluded.tenant_id),
			approval_target = excluded.approval_target,
			tool_class = excluded.tool_class,
			action_ref = excluded.action_ref,
			status = excluded.status,
			resolved_at = excluded.resolved_at,
			document_json = excluded.document_json
	`, item.ApprovalID, item.ValidationID, tenantID, string(item.Target), string(item.ToolClass), nullString(item.ActionRef),
		string(item.Status), item.RequestedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(item.ResolvedAt), string(document))
	if err != nil {
		return fmt.Errorf("upsert live validation approval %s: %w", item.ApprovalID, err)
	}
	return nil
}

func (s *SQLiteStore) UpsertLiveValidationSupportMatrixSnapshot(ctx context.Context, tenantID, snapshotID string, rows []livevalidation.MatrixRow) error {
	if s == nil {
		return nil
	}
	tenantID = coalesceString(tenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	now := time.Now().UTC()
	document := struct {
		SnapshotID string                     `json:"snapshotId"`
		TenantID   string                     `json:"tenantId"`
		Rows       []livevalidation.MatrixRow `json:"rows"`
		CreatedAt  time.Time                  `json:"createdAt"`
	}{
		SnapshotID: snapshotID,
		TenantID:   tenantID,
		Rows:       rows,
		CreatedAt:  now,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal live validation support matrix snapshot: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_support_matrix_snapshots (
			snapshot_id, tenant_id, version, created_at, document_json
		) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(snapshot_id) DO UPDATE SET
			tenant_id = COALESCE(live_validation_support_matrix_snapshots.tenant_id, excluded.tenant_id),
			version = excluded.version,
			document_json = excluded.document_json
	`, snapshotID, tenantID, "v1", now.Format(time.RFC3339Nano), string(encoded))
	if err != nil {
		return fmt.Errorf("upsert live validation support matrix snapshot %s: %w", snapshotID, err)
	}
	return nil
}

func (s *SQLiteStore) AppendLiveValidationLedgerEntry(ctx context.Context, item livevalidation.SideEffectLedgerEntry) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation ledger entry: %w", err)
	}
	tenantID := coalesceString(item.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_ledger_entries (
			ledger_entry_id, validation_id, tenant_id, candidate_id, tool_class, safety_class,
			action_ref, outcome, attempted_at, completed_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ledger_entry_id) DO UPDATE SET
			tenant_id = COALESCE(live_validation_ledger_entries.tenant_id, excluded.tenant_id),
			outcome = excluded.outcome,
			completed_at = excluded.completed_at,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.LedgerEntryID, item.ValidationID, tenantID, item.CandidateID, string(item.ToolClass), string(item.SafetyClass),
		item.ActionRef, string(item.Outcome), nullableTimeString(item.AttemptedAt), nullableTimeString(item.CompletedAt),
		item.UpdatedAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("append live validation ledger entry %s: %w", item.LedgerEntryID, err)
	}
	return nil
}

func (s *SQLiteStore) UpdateLiveValidationLedgerEntryOutcome(ctx context.Context, ledgerEntryID string, outcome livevalidation.LedgerOutcome, reasonCode string) error {
	if s == nil || ledgerEntryID == "" {
		return nil
	}
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT document_json FROM live_validation_ledger_entries WHERE ledger_entry_id = ?`, ledgerEntryID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("get live validation ledger entry %s: %w", ledgerEntryID, err)
	}
	var item livevalidation.SideEffectLedgerEntry
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return fmt.Errorf("decode live validation ledger entry %s: %w", ledgerEntryID, err)
	}
	if err := livevalidation.ValidateLedgerTransition(item.Outcome, outcome); err != nil {
		return err
	}
	now := time.Now().UTC()
	item.Outcome = outcome
	item.ReasonCode = reasonCode
	item.UpdatedAt = now
	if livevalidation.IsTerminalLedgerOutcome(outcome) && outcome != livevalidation.LedgerOutcomeSkipped && outcome != livevalidation.LedgerOutcomeDenied && item.CompletedAt == nil {
		item.CompletedAt = &now
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation ledger entry %s: %w", ledgerEntryID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE live_validation_ledger_entries
		SET outcome = ?, completed_at = ?, updated_at = ?, document_json = ?
		WHERE ledger_entry_id = ?
	`, string(outcome), nullableTimeString(item.CompletedAt), now.Format(time.RFC3339Nano), string(document), ledgerEntryID)
	if err != nil {
		return fmt.Errorf("update live validation ledger entry %s: %w", ledgerEntryID, err)
	}
	return nil
}

func (s *SQLiteStore) ListLiveValidationLedgerEntries(ctx context.Context, filter livevalidation.LedgerFilter) ([]livevalidation.SideEffectLedgerEntry, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM live_validation_ledger_entries WHERE 1 = 1`
	args := make([]any, 0, 6)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.ValidationID != "" {
		query += ` AND validation_id = ?`
		args = append(args, filter.ValidationID)
	}
	if filter.CandidateID != "" {
		query += ` AND candidate_id = ?`
		args = append(args, filter.CandidateID)
	}
	if filter.ToolClass != "" {
		query += ` AND tool_class = ?`
		args = append(args, string(filter.ToolClass))
	}
	if filter.Outcome != "" {
		query += ` AND outcome = ?`
		args = append(args, string(filter.Outcome))
	}
	query += ` ORDER BY updated_at DESC, ledger_entry_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list live validation ledger entries: %w", err)
	}
	defer rows.Close()
	items := []livevalidation.SideEffectLedgerEntry{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item livevalidation.SideEffectLedgerEntry
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode live validation ledger entry: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) UpsertLiveValidationKillSwitch(ctx context.Context, item livevalidation.KillSwitch) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation kill switch: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_kill_switches (
			kill_switch_id, scope, tenant_id, enabled, changed_at, expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(kill_switch_id) DO UPDATE SET
			scope = excluded.scope,
			tenant_id = excluded.tenant_id,
			enabled = excluded.enabled,
			changed_at = excluded.changed_at,
			expires_at = excluded.expires_at,
			document_json = excluded.document_json
	`, item.KillSwitchID, string(item.Scope), nullString(item.TenantID), boolToInt(item.Enabled),
		item.ChangedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(item.ExpiresAt), string(document))
	if err != nil {
		return fmt.Errorf("upsert live validation kill switch %s: %w", item.KillSwitchID, err)
	}
	return nil
}

func (s *SQLiteStore) ListLiveValidationKillSwitches(ctx context.Context, filter livevalidation.KillSwitchFilter) ([]livevalidation.KillSwitch, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM live_validation_kill_switches WHERE 1 = 1`
	args := make([]any, 0, 4)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.Scope != "" {
		query += ` AND scope = ?`
		args = append(args, string(filter.Scope))
	}
	if filter.Enabled != nil {
		query += ` AND enabled = ?`
		args = append(args, boolToInt(*filter.Enabled))
	}
	query += ` ORDER BY changed_at DESC, kill_switch_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list live validation kill switches: %w", err)
	}
	defer rows.Close()
	items := []livevalidation.KillSwitch{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item livevalidation.KillSwitch
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode live validation kill switch: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveLiveValidationAmbiguousCommit(ctx context.Context, item livevalidation.AmbiguousCommit) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation ambiguous commit: %w", err)
	}
	tenantID := coalesceString(item.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_ambiguous_commits (
			ambiguous_commit_id, ledger_entry_id, validation_id, tenant_id, cause,
			automatic_retry_stopped, created_at, updated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ambiguous_commit_id) DO UPDATE SET
			tenant_id = COALESCE(live_validation_ambiguous_commits.tenant_id, excluded.tenant_id),
			cause = excluded.cause,
			automatic_retry_stopped = excluded.automatic_retry_stopped,
			updated_at = excluded.updated_at,
			document_json = excluded.document_json
	`, item.AmbiguousCommitID, item.LedgerEntryID, item.ValidationID, tenantID, string(item.Cause),
		boolToInt(item.AutomaticRetryStopped), item.CreatedAt.UTC().Format(time.RFC3339Nano), item.UpdatedAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("save live validation ambiguous commit %s: %w", item.AmbiguousCommitID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveLiveValidationReconciliationResolution(ctx context.Context, item livevalidation.ReconciliationResolution) error {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(item.Reason) == "" {
		return fmt.Errorf("save live validation reconciliation %s: reason is required", item.ReconciliationID)
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation reconciliation: %w", err)
	}
	tenantID := coalesceString(item.TenantID, tenantBindingString(s.ResolveActiveTenantBinding(ctx)), tenantBindingString(s.ResolveDefaultTenantBinding(ctx)))
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_reconciliation_resolutions (
			reconciliation_id, ambiguous_commit_id, tenant_id, resolved_by, resolution, resolved_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(reconciliation_id) DO UPDATE SET
			tenant_id = COALESCE(live_validation_reconciliation_resolutions.tenant_id, excluded.tenant_id),
			resolution = excluded.resolution,
			resolved_at = excluded.resolved_at,
			document_json = excluded.document_json
	`, item.ReconciliationID, item.AmbiguousCommitID, tenantID, item.ResolvedBy, string(item.Resolution), item.ResolvedAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("save live validation reconciliation %s: %w", item.ReconciliationID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveLiveValidationComparison(ctx context.Context, item livevalidation.Comparison) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation comparison: %w", err)
	}
	tenantID := tenantBindingString(s.ResolveActiveTenantBinding(ctx))
	if tenantID == "" {
		tenantID = tenantBindingString(s.ResolveDefaultTenantBinding(ctx))
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_comparisons (
			comparison_id, validation_id, tenant_id, candidate_id, terminal_status, generated_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(comparison_id) DO UPDATE SET
			tenant_id = COALESCE(live_validation_comparisons.tenant_id, excluded.tenant_id),
			terminal_status = excluded.terminal_status,
			generated_at = excluded.generated_at,
			document_json = excluded.document_json
	`, item.ComparisonID, item.ValidationID, tenantID, item.CandidateID, string(item.TerminalStatus),
		item.GeneratedAt.UTC().Format(time.RFC3339Nano), string(document))
	if err != nil {
		return fmt.Errorf("save live validation comparison %s: %w", item.ComparisonID, err)
	}
	return nil
}

func (s *SQLiteStore) ListLiveValidationComparisons(ctx context.Context, filter livevalidation.ComparisonFilter) ([]livevalidation.Comparison, error) {
	if s == nil {
		return nil, nil
	}
	query := `SELECT document_json FROM live_validation_comparisons WHERE 1 = 1`
	args := make([]any, 0, 5)
	if filter.TenantID != "" {
		query += ` AND tenant_id = ?`
		args = append(args, filter.TenantID)
	}
	if filter.ValidationID != "" {
		query += ` AND validation_id = ?`
		args = append(args, filter.ValidationID)
	}
	if filter.CandidateID != "" {
		query += ` AND candidate_id = ?`
		args = append(args, filter.CandidateID)
	}
	if filter.TerminalStatus != "" {
		query += ` AND terminal_status = ?`
		args = append(args, string(filter.TerminalStatus))
	}
	query += ` ORDER BY generated_at DESC, comparison_id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list live validation comparisons: %w", err)
	}
	defer rows.Close()
	items := []livevalidation.Comparison{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item livevalidation.Comparison
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode live validation comparison: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) SaveLiveValidationRetentionPolicy(ctx context.Context, item livevalidation.RetentionPolicy) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal live validation retention policy: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO live_validation_retention_policies (
			policy_id, tenant_id, applies_to, retention_mode, created_by_principal_id, created_at, expires_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(policy_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			applies_to = excluded.applies_to,
			retention_mode = excluded.retention_mode,
			expires_at = excluded.expires_at,
			document_json = excluded.document_json
	`, item.PolicyID, nullString(item.TenantID), string(item.AppliesTo), string(item.Mode), item.CreatedByPrincipalID,
		item.CreatedAt.UTC().Format(time.RFC3339Nano), nullableTimeString(item.ExpiresAt), string(document))
	if err != nil {
		return fmt.Errorf("save live validation retention policy %s: %w", item.PolicyID, err)
	}
	return nil
}
