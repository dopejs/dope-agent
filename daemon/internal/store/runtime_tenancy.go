package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

// Roadmap 35 (T028 Pass A) — low-level tenant-aware primitives for the
// runtime spine. The tenancy subpackage (daemon/internal/store/tenancy)
// composes these into the user-facing `*ForTenant` helpers and layers
// audit emission on top.
//
// During Pass A, the existing tenantless helpers (UpsertRun, ListRuns, …)
// continue to serve all callers. The primitives here add a separate
// tenant-binding path that lets new tenant-aware code coexist with the
// pre-migration baseline. Pass B (post-backfill) deletes the tenantless
// helpers and the wrapper write-then-bind pattern below collapses into a
// single tenant-aware INSERT.

// ErrCrossTenantRow is returned by the lookup primitives when a row exists
// but its tenant_id does not match the caller's tenant. The tenancy
// subpackage maps this to a 404 + audit emission so the resource's
// existence is not leaked across tenants.
var ErrCrossTenantRow = errors.New("row belongs to a different tenant")

// LookupRowTenant returns the tenant_id of a single row keyed by primary
// key column. Returns (id, true, nil) when the row exists, ("", false,
// nil) when it does not, and (id.String, true, ErrCrossTenantRow) is NOT
// produced here — callers compare against their resolved tenant.
//
// `pkColumn` and `table` are NOT bind-parameter eligible in SQLite, so
// callers MUST pass trusted compile-time constants. The tenancy package
// owns the small set of allowed (table, pkColumn) pairs.
func (s *SQLiteStore) LookupRowTenant(ctx context.Context, table, pkColumn, pk string) (string, bool, error) {
	if s == nil {
		return "", false, nil
	}
	query := fmt.Sprintf("SELECT tenant_id FROM %s WHERE %s = ?", table, pkColumn)
	var tenant sql.NullString
	if err := s.db.QueryRowContext(ctx, query, pk).Scan(&tenant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("lookup tenant for %s.%s=%s: %w", table, pkColumn, pk, err)
	}
	return tenant.String, true, nil
}

// BindRowTenant sets tenant_id on a single row keyed by primary key,
// refusing to overwrite a previously-bound non-matching tenant. Returns
// ErrCrossTenantRow when the existing tenant_id is non-NULL and differs
// from `tenantID`. Idempotent for the matching-tenant case.
//
// Called by every Pass A write helper after the tenantless upsert
// completes. Pass B replaces this with a single tenant-aware INSERT.
func (s *SQLiteStore) BindRowTenant(ctx context.Context, table, pkColumn, pk, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("BindRowTenant: empty tenantID")
	}
	query := fmt.Sprintf("UPDATE %s SET tenant_id = ? WHERE %s = ? AND (tenant_id IS NULL OR tenant_id = ?)", table, pkColumn)
	res, err := s.db.ExecContext(ctx, query, tenantID, pk, tenantID)
	if err != nil {
		return fmt.Errorf("bind tenant for %s.%s=%s: %w", table, pkColumn, pk, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("bind tenant rows-affected for %s.%s=%s: %w", table, pkColumn, pk, err)
	}
	if rows == 0 {
		// Either the row does not exist (nothing to bind, caller's prior
		// upsert may still be in flight at a different layer) or the row
		// is owned by another tenant. Disambiguate so callers can audit.
		existing, ok, lookupErr := s.LookupRowTenant(ctx, table, pkColumn, pk)
		if lookupErr != nil {
			return lookupErr
		}
		if !ok {
			return nil
		}
		if existing != "" && existing != tenantID {
			return ErrCrossTenantRow
		}
	}
	return nil
}

// DeleteRowForTenant removes a row only if its tenant_id matches the
// caller's tenant. Returns (true, nil) on a successful delete, (false,
// nil) when no row exists, and (false, ErrCrossTenantRow) when the row
// is owned by a different tenant.
func (s *SQLiteStore) DeleteRowForTenant(ctx context.Context, table, pkColumn, pk, tenantID string) (bool, error) {
	if s == nil {
		return false, nil
	}
	if tenantID == "" {
		return false, errors.New("DeleteRowForTenant: empty tenantID")
	}
	existing, ok, err := s.LookupRowTenant(ctx, table, pkColumn, pk)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if existing != "" && existing != tenantID {
		return false, ErrCrossTenantRow
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ? AND (tenant_id IS NULL OR tenant_id = ?)", table, pkColumn)
	res, err := s.db.ExecContext(ctx, query, pk, tenantID)
	if err != nil {
		return false, fmt.Errorf("delete %s.%s=%s for tenant: %w", table, pkColumn, pk, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// ListRunsAllTenantsForTest scans the runs table without a tenant
// filter. Test-only: production callers MUST use the tenancy layer
// (tenancy.Runtime.ListRunsForTenant) so cross-tenant rows do not leak
// out of the read path. Replaces the legacy Store.ListRuns method that
// was deleted during T077a (FR-006 leak fix).
func (s *SQLiteStore) ListRunsAllTenantsForTest(ctx context.Context) ([]runtime.Run, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, schedule_id, schedule_attempt_id, reminder_id, reminder_occurrence_id, entrypoint, status, goal, created_at, updated_at
		FROM runs
		ORDER BY created_at ASC, run_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("ListRunsAllTenantsForTest: %w", err)
	}
	defer rows.Close()
	out := make([]runtime.Run, 0)
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("ListRunsAllTenantsForTest scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RunExistsForTenant reports whether a run with the given id exists AND
// belongs to `tenantID`. Returns (true, nil) when the row is present and
// owned by the caller's tenant; (false, nil) when no such row exists OR
// the row is owned by a different tenant. The two cases are
// indistinguishable by design — callers MUST treat "not visible to my
// tenant" as "does not exist", which is what the cross-tenant isolation
// invariant requires (FR-006). Used by the reminder follow-up
// projection to decide whether a `FollowUpLinkKindRun` link is stale.
func (s *SQLiteStore) RunExistsForTenant(ctx context.Context, runID, tenantID string) (bool, error) {
	if s == nil {
		return false, nil
	}
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM runs WHERE run_id = ? AND tenant_id = ? LIMIT 1`,
		runID, tenantID).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("run exists for tenant %s/%s: %w", runID, tenantID, err)
	}
	return n == 1, nil
}

// ListRunsForTenantRaw returns all runs whose tenant_id matches `tenantID`
// in ascending creation order. The tenancy subpackage wraps this with the
// caller-facing API. Pass A semantics: pre-backfill rows whose tenant_id
// is still NULL are NOT returned.
func (s *SQLiteStore) ListRunsForTenantRaw(ctx context.Context, tenantID string) ([]runtime.Run, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, schedule_id, schedule_attempt_id, reminder_id, reminder_occurrence_id, entrypoint, status, goal, created_at, updated_at
		FROM runs
		WHERE tenant_id = ?
		ORDER BY created_at ASC, run_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list runs for tenant: %w", err)
	}
	defer rows.Close()
	out := make([]runtime.Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// GetRunForTenantRaw returns a single run by id IF its tenant_id matches
// `tenantID`. Returns (run, true, nil) on match, (zero, false, nil) on
// missing row, and (zero, false, ErrCrossTenantRow) on mismatch.
func (s *SQLiteStore) GetRunForTenantRaw(ctx context.Context, runID, tenantID string) (runtime.Run, bool, error) {
	if s == nil {
		return runtime.Run{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT run_id, session_id, schedule_id, schedule_attempt_id, reminder_id, reminder_occurrence_id, entrypoint, status, goal, created_at, updated_at, tenant_id
		FROM runs
		WHERE run_id = ?
	`, runID)
	var (
		rowTenant sql.NullString
		// We split the scan into two phases so we can call scanRun on the
		// first 11 columns while inspecting tenant_id ourselves.
	)
	// Buffer scan via a local row scanner.
	var (
		runRow                                                                     runtime.Run
		statusStr                                                                  string
		sessionID, scheduleID, scheduleAttemptID, reminderID, reminderOccurrenceID sql.NullString
		createdAt, updatedAt                                                       string
	)
	if err := row.Scan(
		&runRow.RunID,
		&sessionID,
		&scheduleID,
		&scheduleAttemptID,
		&reminderID,
		&reminderOccurrenceID,
		&runRow.Entrypoint,
		&statusStr,
		&runRow.Goal,
		&createdAt,
		&updatedAt,
		&rowTenant,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return runtime.Run{}, false, nil
		}
		return runtime.Run{}, false, fmt.Errorf("get run for tenant: %w", err)
	}
	if rowTenant.Valid && rowTenant.String != "" && rowTenant.String != tenantID {
		return runtime.Run{}, false, ErrCrossTenantRow
	}
	runRow.SessionID = sessionID.String
	runRow.ScheduleID = scheduleID.String
	runRow.ScheduleAttemptID = scheduleAttemptID.String
	runRow.ReminderID = reminderID.String
	runRow.ReminderOccurrenceID = reminderOccurrenceID.String
	runRow.Status = runtime.RunStatus(statusStr)
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return runtime.Run{}, false, fmt.Errorf("parse run created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return runtime.Run{}, false, fmt.Errorf("parse run updated_at: %w", err)
	}
	runRow.CreatedAt = parsedCreatedAt
	runRow.UpdatedAt = parsedUpdatedAt
	return runRow, true, nil
}

// ListSessionsForTenantRaw mirrors ListSessions but filtered by tenant.
func (s *SQLiteStore) ListSessionsForTenantRaw(ctx context.Context, tenantID string) ([]router.Session, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, kind, status, channel, account_id, peer_id, thread_id, routing_key, generation, created_at, updated_at, last_active_at, last_reset_at
		FROM sessions
		WHERE tenant_id = ?
		ORDER BY created_at ASC, session_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list sessions for tenant: %w", err)
	}
	defer rows.Close()
	out := make([]router.Session, 0)
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// ListStepsForTenantRaw mirrors ListSteps but filtered by tenant.
func (s *SQLiteStore) ListStepsForTenantRaw(ctx context.Context, tenantID, runID string) ([]runtime.Step, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT step_id, run_id, workflow_id, workflow_step_id, attempt, title, kind, status, input_json, output_json, created_at, updated_at
		FROM steps
		WHERE tenant_id = ? AND run_id = ?
		ORDER BY created_at ASC, step_id ASC
	`, tenantID, runID)
	if err != nil {
		return nil, fmt.Errorf("list steps for tenant: %w", err)
	}
	defer rows.Close()
	out := make([]runtime.Step, 0)
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, step)
	}
	return out, rows.Err()
}

// ListToolCallsForTenantRaw mirrors ListToolCalls but filtered by tenant.
func (s *SQLiteStore) ListToolCallsForTenantRaw(ctx context.Context, tenantID, runID, stepID string) ([]runtime.ToolCall, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT tool_call_id, run_id, step_id, workflow_id, workflow_step_id, attempt, computer_use_session_id, computer_use_action_id, invocation_kind, capability_id, skill_id, mcp_server_id, mcp_server_name, mcp_tool_name, mcp_transport_kind, mcp_session_id, authorization_result, tool_name, status, sandbox_execution_id, failure_class, input_json, output_json, sandbox_json, integration_bindings_json, error_text, created_at, updated_at
		FROM tool_calls
		WHERE tenant_id = ? AND run_id = ? AND step_id = ?
		ORDER BY created_at ASC, tool_call_id ASC
	`, tenantID, runID, stepID)
	if err != nil {
		return nil, fmt.Errorf("list tool_calls for tenant: %w", err)
	}
	defer rows.Close()
	out := make([]runtime.ToolCall, 0)
	for rows.Next() {
		tc, err := scanToolCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tc)
	}
	return out, rows.Err()
}

// ListLLMDispatchesForTenantRaw mirrors ListLLMDispatches but filtered by tenant.
func (s *SQLiteStore) ListLLMDispatchesForTenantRaw(ctx context.Context, tenantID string) ([]llm.Dispatch, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT dispatch_id, provider, model, messages_json, stream, status, output_text, finish_reason, usage_json, error_code, error_text, timeout_ms, max_retries, attempt_count, created_at, updated_at, started_at, completed_at
		FROM llm_dispatches
		WHERE tenant_id = ?
		ORDER BY created_at DESC, dispatch_id DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list llm_dispatches for tenant: %w", err)
	}
	defer rows.Close()
	out := make([]llm.Dispatch, 0)
	for rows.Next() {
		d, err := scanLLMDispatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetLLMDispatchForTenantRaw mirrors GetLLMDispatch but enforces tenant
// ownership. Returns ErrCrossTenantRow when the dispatch exists in a
// different tenant.
func (s *SQLiteStore) GetLLMDispatchForTenantRaw(ctx context.Context, dispatchID, tenantID string) (llm.Dispatch, bool, error) {
	if s == nil {
		return llm.Dispatch{}, false, nil
	}
	owner, ok, err := s.LookupRowTenant(ctx, "llm_dispatches", "dispatch_id", dispatchID)
	if err != nil {
		return llm.Dispatch{}, false, err
	}
	if !ok {
		return llm.Dispatch{}, false, nil
	}
	if owner != "" && owner != tenantID {
		return llm.Dispatch{}, false, ErrCrossTenantRow
	}
	return s.GetLLMDispatch(ctx, dispatchID)
}

// BindMCPToolExposureRuleTenant sets tenant_id on the row keyed by the
// composite PK (server_id, tool_name, runtime_surface). Mirrors
// BindRowTenant but for the only single-row helper that does not have a
// scalar primary key.
func (s *SQLiteStore) BindMCPToolExposureRuleTenant(ctx context.Context, serverID, toolName, runtimeSurface, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("BindMCPToolExposureRuleTenant: empty tenantID")
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE mcp_tool_exposure_rules
		SET tenant_id = ?
		WHERE server_id = ? AND tool_name = ? AND runtime_surface = ? AND (tenant_id IS NULL OR tenant_id = ?)
	`, tenantID, serverID, toolName, runtimeSurface, tenantID)
	if err != nil {
		return fmt.Errorf("bind tenant for mcp_tool_exposure_rules: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		// Determine whether the row exists with a different tenant id
		// (cross-tenant write attempt) versus simply being absent.
		var existing sql.NullString
		err := s.db.QueryRowContext(ctx, `
			SELECT tenant_id FROM mcp_tool_exposure_rules
			WHERE server_id = ? AND tool_name = ? AND runtime_surface = ?
		`, serverID, toolName, runtimeSurface).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lookup tenant for mcp_tool_exposure_rules: %w", err)
		}
		if existing.Valid && existing.String != "" && existing.String != tenantID {
			return ErrCrossTenantRow
		}
	}
	return nil
}

// BindRunCheckpointsTenant sets tenant_id on every checkpoint row whose
// run_id matches `runID` and whose tenant_id is currently NULL or already
// equal to `tenantID`. Used by the Pass A SaveCheckpointForTenant path
// because checkpoint_id is generated server-side and not exposed.
func (s *SQLiteStore) BindRunCheckpointsTenant(ctx context.Context, runID, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("BindRunCheckpointsTenant: empty tenantID")
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE checkpoints
		SET tenant_id = ?
		WHERE run_id = ? AND (tenant_id IS NULL OR tenant_id = ?)
	`, tenantID, runID, tenantID)
	if err != nil {
		return fmt.Errorf("bind run checkpoints tenant for run %s: %w", runID, err)
	}
	return nil
}

// ListLatestCheckpointsForTenantRaw mirrors ListLatestCheckpoints but
// returns only checkpoints whose tenant_id matches the caller. The
// checkpoints table's row id is generated server-side and not surfaced on
// runtime.RunCheckpoint, so the tenant filter goes through the
// checkpoint row's own tenant_id column rather than via the parent run.
func (s *SQLiteStore) ListLatestCheckpointsForTenantRaw(ctx context.Context, tenantID string) ([]runtime.RunCheckpoint, error) {
	if s == nil {
		return nil, nil
	}
	all, err := s.ListLatestCheckpoints(ctx)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return all, nil
	}
	// Pull every {run_id, captured_at} that already has a tenant binding
	// in this tenant. Pass A: pre-backfill rows (tenant_id IS NULL) are
	// excluded, matching the same fail-closed semantics as the other
	// tenant-aware reads.
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, captured_at
		FROM checkpoints
		WHERE tenant_id = ?
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list checkpoints for tenant: %w", err)
	}
	defer rows.Close()
	type key struct{ runID, capturedAt string }
	owned := make(map[key]struct{})
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.runID, &k.capturedAt); err != nil {
			return nil, fmt.Errorf("scan checkpoint tenant row: %w", err)
		}
		owned[k] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]runtime.RunCheckpoint, 0, len(all))
	for _, cp := range all {
		k := key{runID: cp.Run.RunID, capturedAt: cp.CapturedAt.UTC().Format(time.RFC3339Nano)}
		if _, ok := owned[k]; ok {
			out = append(out, cp)
		}
	}
	return out, nil
}
