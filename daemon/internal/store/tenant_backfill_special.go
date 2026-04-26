package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Roadmap 35 (US2 / T076a + T076b) — specialized backfill helpers.
//
// Three tables don't fit the generic engine in tenant_backfill.go:
//
//   1. mcp_tool_exposure_rules has a COMPOSITE PK
//      (server_id, tool_name, runtime_surface). The generic engine
//      assumes a single TEXT PK. T076a says this table is top-level →
//      default personal tenant, so a single-shot UPDATE is safe and
//      restart-safe (subsequent runs find no NULL rows).
//
//   2. sandbox_executions has an OPTIONAL parent FK (run_id may be
//      NULL). When present, derive from runs.tenant_id and apply the
//      orphan rule. When NULL, fall back to default personal tenant.
//
//   3. connector_messages has TWO optional parent FKs in priority
//      order: session_id → sessions, then run_id → runs, else default.
//      Apply the orphan rule for cases where the FK is non-NULL but
//      the parent row is missing or has NULL tenant_id.
//
// All three helpers register/transition the migration step and respect
// the resume contract (`MigrationStepCompleted` → no-op).

// RunSimpleBulkBackfill binds tenant_id = defaultTenantID for every
// NULL-tenant row in `table` in a single statement. Used for top-level
// tables whose PK doesn't fit the chunked engine — currently only
// mcp_tool_exposure_rules. Restart-safe by virtue of the WHERE
// tenant_id IS NULL filter.
func (s *SQLiteStore) RunSimpleBulkBackfill(ctx context.Context, stepName, table, defaultTenantID string) error {
	if s == nil {
		return errors.New("RunSimpleBulkBackfill: nil store")
	}
	if defaultTenantID == "" {
		return fmt.Errorf("RunSimpleBulkBackfill: %s requires defaultTenantID", stepName)
	}
	_, err := s.BeginMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	current, err := s.GetMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	if current.Status == MigrationStepCompleted {
		return nil
	}
	query := fmt.Sprintf(`UPDATE %s SET tenant_id = ? WHERE tenant_id IS NULL`, table)
	if _, err := s.db.ExecContext(ctx, query, defaultTenantID); err != nil {
		return fmt.Errorf("simple bulk backfill %s: %w", stepName, err)
	}
	return s.CompleteMigrationStep(ctx, stepName)
}

// RunOptionalParentBackfill binds tenant_id for `table` where:
//   - rows with non-NULL parent FK derive from parent.tenant_id (orphan
//     rule applies — missing parent or NULL parent.tenant_id surfaces
//     OrphanError);
//   - rows with NULL parent FK fall back to defaultTenantID.
//
// Used by sandbox_executions (run_id optional). Single-shot per-step
// because the table is small relative to runtime/events; if size grows
// the helper can be migrated to chunking later. Restart-safe via the
// MigrationStepCompleted check.
func (s *SQLiteStore) RunOptionalParentBackfill(ctx context.Context, stepName, table, pkColumn, parentTable, parentFKColumn, parentTablePK, defaultTenantID string) error {
	if s == nil {
		return errors.New("RunOptionalParentBackfill: nil store")
	}
	if defaultTenantID == "" {
		return fmt.Errorf("RunOptionalParentBackfill: %s requires defaultTenantID", stepName)
	}
	_, err := s.BeginMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	current, err := s.GetMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	if current.Status == MigrationStepCompleted {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("optional-parent backfill begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Orphan probe: rows where parent FK is non-NULL but parent missing or parent.tenant_id NULL.
	orphanQuery := fmt.Sprintf(
		`SELECT c.%s, c.%s
		FROM %s AS c
		LEFT JOIN %s AS p ON p.%s = c.%s
		WHERE c.tenant_id IS NULL AND c.%s IS NOT NULL
		AND (p.%s IS NULL OR p.tenant_id IS NULL)
		LIMIT 1`,
		pkColumn, parentFKColumn,
		table,
		parentTable, parentTablePK, parentFKColumn,
		parentFKColumn,
		parentTablePK,
	)
	var orphanPK, orphanFK sql.NullString
	if err := tx.QueryRowContext(ctx, orphanQuery).Scan(&orphanPK, &orphanFK); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("optional-parent orphan probe %s: %w", stepName, err)
	}
	if orphanPK.Valid {
		return &OrphanError{
			Table:       table,
			ParentTable: parentTable,
			ParentFK:    parentFKColumn,
			ParentValue: orphanFK.String,
		}
	}

	// Bind parent-derived rows.
	parentBind := fmt.Sprintf(
		`UPDATE %s SET tenant_id = (
			SELECT p.tenant_id FROM %s AS p WHERE p.%s = %s.%s
		) WHERE tenant_id IS NULL AND %s IS NOT NULL`,
		table, parentTable, parentTablePK, table, parentFKColumn, parentFKColumn,
	)
	if _, err := tx.ExecContext(ctx, parentBind); err != nil {
		return fmt.Errorf("optional-parent bind %s: %w", stepName, err)
	}
	// Fall back: NULL FK → default tenant.
	defaultBind := fmt.Sprintf(`UPDATE %s SET tenant_id = ? WHERE tenant_id IS NULL`, table)
	if _, err := tx.ExecContext(ctx, defaultBind, defaultTenantID); err != nil {
		return fmt.Errorf("optional-parent default %s: %w", stepName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("optional-parent commit %s: %w", stepName, err)
	}
	return s.CompleteMigrationStep(ctx, stepName)
}

// RunEventsBackfill — Roadmap 35 (T077). The mixed events table has
// rows whose tenant_id resolves through any of seven parent FKs in
// priority order, plus two reclassification rules for legacy
// connector / capability events.
//
// Resolution policy (per US2 spec):
//
//  1. Skip rows whose category is already global
//     (system, mcp, provider, daemon.migration, connector_global,
//     capability_global). Their tenant_id stays NULL.
//
//  2. Connector orphan reclassification: rows whose category is in
//     the connector family (any non-global category that is paired
//     with a non-NULL connector_id but lacks a usable
//     resource_kind/resource_id pointer to connector_messages) are
//     reclassified to category='connector_global' with NULL tenant_id.
//
//  3. Capability orphan reclassification: rows whose only resolvable
//     parent FK is capability_id (capabilities are global, no tenant)
//     are reclassified to category='capability_global' with NULL
//     tenant_id.
//
//  4. Bind tenant_id from parent in priority order: run_id, then
//     session_id, then step_id, then workflow_id, then
//     workflow_step_id, then schedule_id, then schedule_attempt_id,
//     then connector_messages via resource_id.
//
//  5. Orphan rule: any tenant-owned-category row that still has NULL
//     tenant_id after step 4 surfaces an OrphanError carrying the
//     event_id (used as parent_value for audit) so the operator knows
//     a parent backfill missed a row.
//
// All operations run inside a single transaction so a failure rolls
// back cleanly.
func (s *SQLiteStore) RunEventsBackfill(ctx context.Context, stepName, defaultTenantID string) error {
	if s == nil {
		return errors.New("RunEventsBackfill: nil store")
	}
	if defaultTenantID == "" {
		return fmt.Errorf("RunEventsBackfill: %s requires defaultTenantID for fallback bind", stepName)
	}
	_, err := s.BeginMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	current, err := s.GetMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	if current.Status == MigrationStepCompleted {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("events backfill begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const globalCategoryFilter = `category NOT IN ('system','mcp','provider','daemon.migration','connector_global','capability_global')`

	// Step 2: reclassify connector orphans.
	if _, err := tx.ExecContext(ctx, `
		UPDATE events SET category = 'connector_global'
		WHERE tenant_id IS NULL
		AND `+globalCategoryFilter+`
		AND connector_id IS NOT NULL
		AND (
			resource_kind != 'connector_message'
			OR resource_id IS NULL OR resource_id = ''
			OR NOT EXISTS (SELECT 1 FROM connector_messages cm WHERE cm.delivery_id = events.resource_id)
		)
	`); err != nil {
		return fmt.Errorf("events reclassify connector_global: %w", err)
	}

	// Step 3: reclassify capability-only events.
	if _, err := tx.ExecContext(ctx, `
		UPDATE events SET category = 'capability_global'
		WHERE tenant_id IS NULL
		AND `+globalCategoryFilter+`
		AND capability_id IS NOT NULL
		AND run_id IS NULL AND session_id IS NULL AND step_id IS NULL
		AND workflow_id IS NULL AND workflow_step_id IS NULL
		AND schedule_id IS NULL AND schedule_attempt_id IS NULL
		AND connector_id IS NULL
	`); err != nil {
		return fmt.Errorf("events reclassify capability_global: %w", err)
	}

	// Step 4: sequential bind via priority cascade. Each statement
	// only touches rows still NULL with a non-global category.
	binds := []struct {
		fk          string
		parentTable string
		parentPK    string
	}{
		{"run_id", "runs", "run_id"},
		{"session_id", "sessions", "session_id"},
		{"step_id", "steps", "step_id"},
		{"workflow_id", "workflows", "workflow_id"},
		{"workflow_step_id", "workflow_steps", "workflow_step_id"},
		{"schedule_id", "schedules", "schedule_id"},
		{"schedule_attempt_id", "schedule_dispatch_attempts", "attempt_id"},
	}
	for _, b := range binds {
		query := fmt.Sprintf(`
			UPDATE events SET tenant_id = (
				SELECT p.tenant_id FROM %s AS p WHERE p.%s = events.%s
			) WHERE tenant_id IS NULL
			AND %s
			AND %s IS NOT NULL
			AND EXISTS (SELECT 1 FROM %s AS p WHERE p.%s = events.%s AND p.tenant_id IS NOT NULL)
		`, b.parentTable, b.parentPK, b.fk, globalCategoryFilter, b.fk, b.parentTable, b.parentPK, b.fk)
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("events bind via %s: %w", b.fk, err)
		}
	}

	// connector_messages bind via resource_id.
	if _, err := tx.ExecContext(ctx, `
		UPDATE events SET tenant_id = (
			SELECT cm.tenant_id FROM connector_messages cm WHERE cm.delivery_id = events.resource_id
		) WHERE tenant_id IS NULL
		AND `+globalCategoryFilter+`
		AND resource_kind = 'connector_message' AND resource_id IS NOT NULL AND resource_id != ''
		AND EXISTS (SELECT 1 FROM connector_messages cm WHERE cm.delivery_id = events.resource_id AND cm.tenant_id IS NOT NULL)
	`); err != nil {
		return fmt.Errorf("events bind via connector_messages: %w", err)
	}

	// Resource-pointer cascade: many tenant-owned event categories
	// (reminder, calendar, mail, evaluation, computer-use, …) carry
	// their tenant linkage via the (resource_kind, resource_id) pair
	// rather than via a typed FK column. Bind via the resource pointer
	// as a final pass before the orphan probe so legitimate
	// resource-pointed events aren't treated as orphans.
	resourcePointers := []struct {
		resourceKind string
		table        string
		pkColumn     string
	}{
		{"reminder", "reminders", "reminder_id"},
		{"reminder_occurrence", "reminder_occurrences", "occurrence_id"},
		{"reminder_action", "reminder_actions", "action_id"},
		{"calendar_account", "calendar_accounts", "calendar_account_id"},
		{"calendar_operation", "calendar_operations", "operation_id"},
		{"calendar_artifact", "calendar_artifacts", "artifact_id"},
		{"mail_account", "mail_accounts", "mail_account_id"},
		{"mail_operation", "mail_operations", "operation_id"},
		{"mail_artifact", "mail_artifacts", "artifact_id"},
		{"computer_use_session", "computer_use_sessions", "computer_use_session_id"},
		{"computer_use_action", "computer_use_actions", "computer_use_action_id"},
		{"computer_use_artifact", "computer_use_artifacts", "artifact_id"},
		{"approval", "approvals", "approval_id"},
		{"decision", "decisions", "decision_id"},
		{"integration", "integrations", "integration_id"},
		{"delivery", "delivery_outcomes", "delivery_id"},
		{"delivery_target", "delivery_targets", "target_id"},
		{"delivery_attempt", "delivery_attempts", "attempt_id"},
		{"evaluation_replay_candidate", "evaluation_replay_candidates", "candidate_id"},
		{"evaluation_replay_attempt", "evaluation_replay_attempts", "attempt_id"},
	}
	for _, rp := range resourcePointers {
		query := fmt.Sprintf(`
			UPDATE events SET tenant_id = (
				SELECT p.tenant_id FROM %s AS p WHERE p.%s = events.resource_id
			) WHERE tenant_id IS NULL
			AND %s
			AND resource_kind = ? AND resource_id IS NOT NULL AND resource_id != ''
			AND EXISTS (SELECT 1 FROM %s AS p WHERE p.%s = events.resource_id AND p.tenant_id IS NOT NULL)
		`, rp.table, rp.pkColumn, globalCategoryFilter, rp.table, rp.pkColumn)
		if _, err := tx.ExecContext(ctx, query, rp.resourceKind); err != nil {
			return fmt.Errorf("events bind via resource %s: %w", rp.resourceKind, err)
		}
	}

	// Step 5a: default-personal-tenant fallback for legitimate
	// tenant-owned events whose typed FK columns are NULL AND whose
	// resource pointer references a resource_kind not in the cascade
	// above OR a resource_id that no longer exists. In a single-
	// operator system every such event came from the operator, so the
	// safe lossless choice is the default personal tenant. The
	// inventory marks this fallback explicitly: events with at least
	// one populated linkage column (typed FK or resource pointer) are
	// considered legitimate emissions, not corruption.
	if _, err := tx.ExecContext(ctx, `
		UPDATE events SET tenant_id = ?
		WHERE tenant_id IS NULL
		AND `+globalCategoryFilter+`
		AND (run_id IS NOT NULL OR session_id IS NOT NULL OR step_id IS NOT NULL
		     OR workflow_id IS NOT NULL OR workflow_step_id IS NOT NULL
		     OR schedule_id IS NOT NULL OR schedule_attempt_id IS NOT NULL
		     OR connector_id IS NOT NULL OR capability_id IS NOT NULL
		     OR (resource_kind != '' AND resource_id != ''))
	`, defaultTenantID); err != nil {
		return fmt.Errorf("events default-tenant fallback: %w", err)
	}

	// Step 5b: orphan probe — only true orphans remain (no typed FK,
	// no resource pointer at all, non-global category). Surface with
	// event_id so audit captures the case without leaking other data.
	var orphanID, orphanCategory string
	err = tx.QueryRowContext(ctx, `
		SELECT event_id, category FROM events
		WHERE tenant_id IS NULL
		AND `+globalCategoryFilter+`
		LIMIT 1
	`).Scan(&orphanID, &orphanCategory)
	if err == nil {
		return &OrphanError{
			Table:       "events",
			ParentTable: "(no parent FK or resource pointer)",
			ParentFK:    "category=" + orphanCategory,
			ParentValue: orphanID,
		}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("events orphan probe: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("events commit: %w", err)
	}
	return s.CompleteMigrationStep(ctx, stepName)
}

// RunConnectorMessagesBackfill resolves tenant_id for connector_messages
// in priority order per inventory: (1) session_id → sessions.tenant_id;
// (2) else run_id → runs.tenant_id; (3) else default personal tenant.
// Apply the orphan rule whenever a non-NULL FK fails to resolve.
//
// Single-statement orchestration:
//   - probe orphans across both FKs;
//   - bind via sessions for rows with session_id;
//   - bind via runs for the remainder with run_id;
//   - bind defaultTenantID for everything still NULL.
func (s *SQLiteStore) RunConnectorMessagesBackfill(ctx context.Context, stepName, defaultTenantID string) error {
	if s == nil {
		return errors.New("RunConnectorMessagesBackfill: nil store")
	}
	if defaultTenantID == "" {
		return fmt.Errorf("RunConnectorMessagesBackfill: %s requires defaultTenantID", stepName)
	}
	_, err := s.BeginMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	current, err := s.GetMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	if current.Status == MigrationStepCompleted {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("connector_messages begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Orphan probe via session_id (when present): session row missing or NULL tenant.
	var orphanPK, orphanFK sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT c.delivery_id, c.session_id
		FROM connector_messages AS c
		LEFT JOIN sessions AS s ON s.session_id = c.session_id
		WHERE c.tenant_id IS NULL AND c.session_id IS NOT NULL
		AND (s.session_id IS NULL OR s.tenant_id IS NULL)
		LIMIT 1
	`).Scan(&orphanPK, &orphanFK); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("connector_messages session orphan probe: %w", err)
	}
	if orphanPK.Valid {
		return &OrphanError{
			Table:       "connector_messages",
			ParentTable: "sessions",
			ParentFK:    "session_id",
			ParentValue: orphanFK.String,
		}
	}
	// Orphan probe via run_id (when session_id is NULL but run_id present).
	if err := tx.QueryRowContext(ctx, `
		SELECT c.delivery_id, c.run_id
		FROM connector_messages AS c
		LEFT JOIN runs AS r ON r.run_id = c.run_id
		WHERE c.tenant_id IS NULL AND c.session_id IS NULL AND c.run_id IS NOT NULL
		AND (r.run_id IS NULL OR r.tenant_id IS NULL)
		LIMIT 1
	`).Scan(&orphanPK, &orphanFK); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("connector_messages run orphan probe: %w", err)
	}
	if orphanPK.Valid {
		return &OrphanError{
			Table:       "connector_messages",
			ParentTable: "runs",
			ParentFK:    "run_id",
			ParentValue: orphanFK.String,
		}
	}

	// Bind via session.
	if _, err := tx.ExecContext(ctx, `
		UPDATE connector_messages SET tenant_id = (
			SELECT s.tenant_id FROM sessions AS s WHERE s.session_id = connector_messages.session_id
		) WHERE tenant_id IS NULL AND session_id IS NOT NULL
	`); err != nil {
		return fmt.Errorf("connector_messages bind via session: %w", err)
	}
	// Bind via run for the remainder.
	if _, err := tx.ExecContext(ctx, `
		UPDATE connector_messages SET tenant_id = (
			SELECT r.tenant_id FROM runs AS r WHERE r.run_id = connector_messages.run_id
		) WHERE tenant_id IS NULL AND run_id IS NOT NULL
	`); err != nil {
		return fmt.Errorf("connector_messages bind via run: %w", err)
	}
	// Default for rows with no parent at all.
	if _, err := tx.ExecContext(ctx, `
		UPDATE connector_messages SET tenant_id = ? WHERE tenant_id IS NULL
	`, defaultTenantID); err != nil {
		return fmt.Errorf("connector_messages bind default: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("connector_messages commit: %w", err)
	}
	return s.CompleteMigrationStep(ctx, stepName)
}
