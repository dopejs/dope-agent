package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
)

// Roadmap 35 (T039 Pass A) — events table tenant-aware writes.
//
// The events table is mixed: tenant-owned event categories carry a
// non-NULL tenant_id while a small set of global categories
// (mcp-*, provider-*, system-*, daemon.migration.*, connector_global,
// capability_global) leave tenant_id NULL. AppendEventForTenantRaw is
// used by the new tenant-aware emission path; the existing AppendEvent
// remains for legacy callers and global-event emitters.

// AppendEventForTenantRaw appends a tenant-owned event with tenant_id
// bound atomically in the same INSERT. Returns the persisted event
// including the assigned sequence. Refuses (returns ErrCrossTenantRow)
// when an event row with the same `event_id` already exists owned by a
// different tenant — the existing row is preserved and the caller
// learns the write was rejected.
//
// Replaces the earlier "AppendEvent then UPDATE tenant_id" pattern
// which (a) could silently leave the row unbound when the conflicting
// row was owned by another tenant or was global, and (b) did not
// inspect RowsAffected so the caller had no way to detect the leak.
func (s *SQLiteStore) AppendEventForTenantRaw(ctx context.Context, event events.Event, tenantID string) (events.Event, error) {
	if s == nil {
		return event, nil
	}
	if tenantID == "" {
		return events.Event{}, errors.New("AppendEventForTenantRaw: empty tenantID")
	}
	prepared := event
	if prepared.EventID == "" {
		return events.Event{}, errors.New("AppendEventForTenantRaw: empty event_id (caller must pre-fill via ensureEventDefaults)")
	}
	if prepared.OccurredAt.IsZero() {
		prepared.OccurredAt = time.Now().UTC()
	}
	if prepared.Payload == nil {
		prepared.Payload = map[string]any{}
	}
	if prepared.EnvironmentScope == "" {
		prepared.EnvironmentScope = events.EnvironmentScopeFromContext(ctx)
	}
	payloadJSON, err := marshalJSON(prepared.Payload)
	if err != nil {
		return events.Event{}, fmt.Errorf("marshal event payload: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO events (
			event_id,
			environment_scope,
			category,
			name,
			occurred_at,
			session_id,
			run_id,
			workflow_id,
			workflow_step_id,
			schedule_id,
			schedule_attempt_id,
			step_id,
			connector_id,
			capability_id,
			resource_kind,
			resource_id,
			payload_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id) DO NOTHING
	`,
		prepared.EventID,
		prepared.EnvironmentScope,
		prepared.Category,
		prepared.Name,
		prepared.OccurredAt.UTC().Format(time.RFC3339Nano),
		nullString(prepared.Scope.SessionID),
		nullString(prepared.Scope.RunID),
		nullString(prepared.Scope.WorkflowID),
		nullString(prepared.Scope.WorkflowStepID),
		nullString(prepared.Scope.ScheduleID),
		nullString(prepared.Scope.ScheduleAttemptID),
		nullString(prepared.Scope.StepID),
		nullString(prepared.Scope.ConnectorID),
		nullString(prepared.Scope.CapabilityID),
		prepared.Resource.Kind,
		prepared.Resource.ID,
		payloadJSON,
		tenantID,
	)
	if err != nil {
		return events.Event{}, fmt.Errorf("append event for tenant: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return events.Event{}, fmt.Errorf("append event rows-affected: %w", err)
	}
	if rowsAffected == 0 {
		// A row with the same event_id already exists. Disambiguate so
		// callers can audit cross-tenant collisions AND so we refuse to
		// claim NULL-tenant rows that belong to a global category.
		var (
			existingTenant   sql.NullString
			existingCategory string
		)
		err := s.db.QueryRowContext(ctx, `SELECT tenant_id, category FROM events WHERE event_id = ?`, prepared.EventID).Scan(&existingTenant, &existingCategory)
		if err != nil {
			return events.Event{}, fmt.Errorf("append event lookup existing: %w", err)
		}
		if existingTenant.Valid && existingTenant.String != "" && existingTenant.String != tenantID {
			return events.Event{}, ErrCrossTenantRow
		}
		// Row exists with NULL tenant_id. Two cases:
		//   - Global category (system, mcp, provider, daemon.migration,
		//     connector_global, capability_global): row is owned by the
		//     daemon-wide global pool. NEVER claim — refuse so a tenant
		//     emit cannot silently retag a system event as their own.
		//   - Non-global (tenant-owned) category: legitimate pre-backfill
		//     compatibility. Claim atomically so the bound state is
		//     reachable. Pass B drops this branch when the schema swap
		//     guarantees every tenant-owned row is bound at INSERT.
		if !existingTenant.Valid || existingTenant.String == "" {
			if events.IsGlobalCategory(existingCategory) {
				return events.Event{}, ErrCrossTenantRow
			}
			if _, err := s.db.ExecContext(ctx, `
				UPDATE events SET tenant_id = ? WHERE event_id = ? AND tenant_id IS NULL
			`, tenantID, prepared.EventID); err != nil {
				return events.Event{}, fmt.Errorf("claim tenant on existing event: %w", err)
			}
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT rowid FROM events WHERE event_id = ?`, prepared.EventID).Scan(&prepared.Sequence); err != nil {
		return events.Event{}, fmt.Errorf("load event sequence %s: %w", prepared.EventID, err)
	}
	prepared.TenantID = tenantID
	return prepared, nil
}

// ListEventsForTenantRaw returns events whose tenant_id matches
// `tenantID`. Reads tenant_id directly from the events table (the
// existing ListEvents helper does not surface it). Use ListEvents (no
// tenant filter) to query global events.
func (s *SQLiteStore) ListEventsForTenantRaw(ctx context.Context, tenantID string, filter events.Filter) ([]events.Event, error) {
	if s == nil {
		return nil, nil
	}
	query := `
		SELECT rowid, event_id, environment_scope, category, name, occurred_at, session_id, run_id, workflow_id, workflow_step_id, schedule_id, schedule_attempt_id, step_id, connector_id, capability_id, resource_kind, resource_id, payload_json, tenant_id
		FROM events
		WHERE tenant_id = ?
	`
	args := []any{tenantID}
	if filter.EnvironmentScope != "" {
		query += ` AND environment_scope = ?`
		args = append(args, filter.EnvironmentScope)
	}
	if filter.Category != "" {
		query += ` AND category = ?`
		args = append(args, filter.Category)
	}
	if filter.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, filter.RunID)
	}
	if filter.SessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, filter.SessionID)
	}
	if filter.ScheduleID != "" {
		query += ` AND schedule_id = ?`
		args = append(args, filter.ScheduleID)
	}
	if filter.ScheduleAttemptID != "" {
		query += ` AND schedule_attempt_id = ?`
		args = append(args, filter.ScheduleAttemptID)
	}
	if filter.ResourceKind != "" {
		query += ` AND resource_kind = ?`
		args = append(args, filter.ResourceKind)
	}
	if filter.Cursor > 0 {
		query += ` AND rowid > ?`
		args = append(args, filter.Cursor)
	}
	query += ` ORDER BY rowid ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events for tenant: %w", err)
	}
	defer rows.Close()
	items := make([]events.Event, 0)
	for rows.Next() {
		ev, tenantOwned, err := scanEventWithTenant(rows)
		if err != nil {
			return nil, err
		}
		if !tenantOwned {
			continue
		}
		items = append(items, ev)
	}
	return items, rows.Err()
}

// scanEventWithTenant reads the same columns as scanEvent plus the
// tenant_id column. Returns (event, true, nil) when the row's tenant_id
// is non-NULL; (event, false, nil) for global rows (NULL tenant_id) so
// the caller can filter them out.
func scanEventWithTenant(scanner interface {
	Scan(dest ...any) error
}) (events.Event, bool, error) {
	var (
		ev          events.Event
		envScope    sql.NullString
		sessionID   sql.NullString
		runID       sql.NullString
		workflowID  sql.NullString
		workflowSID sql.NullString
		scheduleID  sql.NullString
		attemptID   sql.NullString
		stepID      sql.NullString
		connectorID sql.NullString
		capabilID   sql.NullString
		occurredAt  string
		payload     sql.NullString
		tenantID    sql.NullString
	)
	if err := scanner.Scan(
		&ev.Sequence,
		&ev.EventID,
		&envScope,
		&ev.Category,
		&ev.Name,
		&occurredAt,
		&sessionID,
		&runID,
		&workflowID,
		&workflowSID,
		&scheduleID,
		&attemptID,
		&stepID,
		&connectorID,
		&capabilID,
		&ev.Resource.Kind,
		&ev.Resource.ID,
		&payload,
		&tenantID,
	); err != nil {
		return events.Event{}, false, fmt.Errorf("scan tenant event: %w", err)
	}
	if envScope.Valid {
		ev.EnvironmentScope = envScope.String
	}
	parsed, err := time.Parse(time.RFC3339Nano, occurredAt)
	if err != nil {
		return events.Event{}, false, fmt.Errorf("parse event occurred_at: %w", err)
	}
	ev.OccurredAt = parsed
	ev.Scope.SessionID = sessionID.String
	ev.Scope.RunID = runID.String
	ev.Scope.WorkflowID = workflowID.String
	ev.Scope.WorkflowStepID = workflowSID.String
	ev.Scope.ScheduleID = scheduleID.String
	ev.Scope.ScheduleAttemptID = attemptID.String
	ev.Scope.StepID = stepID.String
	ev.Scope.ConnectorID = connectorID.String
	ev.Scope.CapabilityID = capabilID.String
	if err := unmarshalNullableJSON(payload, &ev.Payload); err != nil {
		return events.Event{}, false, fmt.Errorf("decode event payload: %w", err)
	}
	if ev.Payload == nil {
		ev.Payload = map[string]any{}
	}
	if tenantID.Valid && tenantID.String != "" {
		ev.TenantID = tenantID.String
		return ev, true, nil
	}
	return ev, false, nil
}
