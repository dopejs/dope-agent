package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Roadmap 35 (US2 / T067+) — generic backfill engine.
//
// Each per-domain backfill step backfills `tenant_id` for rows that were
// inserted before Roadmap 35 added the column and now carry NULL. The
// per-domain implementations specify a `BackfillStep` describing which
// table to backfill, how to derive the row's tenant id (default
// personal tenant OR parent-FK lookup), and which primary key column
// drives the chunked cursor.
//
// Engine guarantees:
//   - chunked transactions of `ChunkSize` rows so a partial run leaves
//     a coherent intermediate state and the resume cursor never leads
//     the data;
//   - resume from `last_processed_key` after a crash mid-step;
//   - parent-child orphan rule (FR-009): if a child row's parent FK
//     does not resolve to a tenant_owned row, the engine surfaces an
//     OrphanError with table + parent FK so the caller can emit
//     `daemon.migration.orphan_detected` and fail the step;
//   - per-step idempotence: re-running a `completed` step is a no-op;
//   - state-machine transitions via `migration_progress.go`.
//
// Each step is a `BackfillStep`; the engine never embeds business logic
// about a specific table.

// BackfillStep describes one per-domain backfill operation. The engine
// uses these fields to construct the chunked SQL and to enforce the
// parent-child orphan rule.
type BackfillStep struct {
	// Name is the canonical step name (e.g.
	// "tenant_migration:backfill:runs"). Used as the
	// `tenant_migration_progress.step_name` key.
	Name string

	// Table is the SQL table being backfilled.
	Table string

	// PKColumn is the primary key column name. The engine uses it for
	// the chunk cursor (`ORDER BY <PKColumn> ASC`) and for the resume
	// `last_processed_key` lookup. Must be a single TEXT column —
	// composite PKs are not supported by this engine and any composite-PK
	// table must implement its own backfill helper.
	PKColumn string

	// ParentTable is the parent table for derived `tenant_id`. When
	// empty, the engine assigns `DefaultTenantID` to every row in the
	// chunk (top-level domain). When non-empty, the engine looks up
	// the parent row's tenant_id via `ParentFKColumn` →
	// `ParentTable.<ParentTablePK>`.
	ParentTable string

	// ParentFKColumn is the FK column on `Table` that points at the
	// parent row. Required when `ParentTable` is set.
	ParentFKColumn string

	// ParentTablePK is the primary key column on `ParentTable`. Used to
	// build the JOIN. Required when `ParentTable` is set.
	ParentTablePK string

	// ChunkSize is the number of rows committed per transaction. The
	// engine clamps to a sane minimum (50) and a maximum (5000).
	// Callers should pass 500 per the US2 contract unless tuning.
	ChunkSize int
}

// OrphanError signals that a child row in the chunk references a
// parent row that either does not exist OR exists but has a NULL
// tenant_id. The caller emits `daemon.migration.orphan_detected` with
// the carried table + parent FK and fails the step. The error never
// carries row data, only the orphan FK value, per the audit/payload
// minimization rule.
type OrphanError struct {
	Table       string
	ParentTable string
	ParentFK    string
	ParentValue string
}

func (e *OrphanError) Error() string {
	return fmt.Sprintf("backfill orphan: %s.%s=%q has no resolvable %s.tenant_id",
		e.Table, e.ParentFK, e.ParentValue, e.ParentTable)
}

// RunBackfill executes a single backfill step end-to-end against the
// store. The function is restart-safe: on a crash the step's status is
// `running` with `last_processed_key` set, and a subsequent invocation
// resumes from there. Returns nil when the step completes (or was
// already complete); returns an OrphanError when a parent FK can't be
// resolved (caller must FailMigrationStep on that error); returns any
// other error unchanged for the caller to log + FailMigrationStep.
//
// `defaultTenantID` is the personal tenant for top-level (parent-less)
// rows. May be empty only when every step in the suite has a parent
// table — the engine returns an error if a top-level step is invoked
// without a default.
func (s *SQLiteStore) RunBackfill(ctx context.Context, step BackfillStep, defaultTenantID string) error {
	if s == nil {
		return errors.New("RunBackfill: nil store")
	}
	if step.Name == "" || step.Table == "" || step.PKColumn == "" {
		return fmt.Errorf("RunBackfill: invalid step %+v", step)
	}
	if step.ParentTable != "" && (step.ParentFKColumn == "" || step.ParentTablePK == "") {
		return fmt.Errorf("RunBackfill: parent table %q requires ParentFKColumn + ParentTablePK", step.ParentTable)
	}
	if step.ParentTable == "" && defaultTenantID == "" {
		return fmt.Errorf("RunBackfill: top-level step %q requires defaultTenantID", step.Name)
	}
	if step.ChunkSize < 50 {
		step.ChunkSize = 500
	}
	if step.ChunkSize > 5000 {
		step.ChunkSize = 5000
	}

	resume, err := s.BeginMigrationStep(ctx, step.Name)
	if err != nil {
		return err
	}
	current, err := s.GetMigrationStep(ctx, step.Name)
	if err != nil {
		return err
	}
	if current.Status == MigrationStepCompleted {
		return nil
	}

	cursor := ""
	if resume {
		cursor = current.LastProcessedKey
	}

	for {
		processed, lastKey, err := s.runBackfillChunk(ctx, step, defaultTenantID, cursor)
		if err != nil {
			return err
		}
		if processed == 0 {
			break
		}
		cursor = lastKey
		if err := s.RecordMigrationChunk(ctx, step.Name, lastKey); err != nil {
			return err
		}
		if processed < step.ChunkSize {
			break
		}
	}
	return s.CompleteMigrationStep(ctx, step.Name)
}

// runBackfillChunk processes one chunk inside a single transaction.
// Returns (rowsProcessed, lastKey, err). rowsProcessed=0 means no more
// NULL-tenant rows remain.
func (s *SQLiteStore) runBackfillChunk(ctx context.Context, step BackfillStep, defaultTenantID, cursor string) (int, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", fmt.Errorf("backfill begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := s.selectChunkKeys(ctx, tx, step, cursor)
	if err != nil {
		return 0, "", err
	}
	if len(rows) == 0 {
		return 0, cursor, nil
	}

	if step.ParentTable == "" {
		if err := updateRowsToDefault(ctx, tx, step, defaultTenantID, rows); err != nil {
			return 0, "", err
		}
	} else {
		if err := updateRowsFromParent(ctx, tx, step, rows); err != nil {
			return 0, "", err
		}
	}

	lastKey := rows[len(rows)-1]
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("backfill commit: %w", err)
	}
	return len(rows), lastKey, nil
}

// selectChunkKeys returns up to ChunkSize PK values that still need
// backfilling, ordered ascending so the cursor advances monotonically.
func (s *SQLiteStore) selectChunkKeys(ctx context.Context, tx *sql.Tx, step BackfillStep, cursor string) ([]string, error) {
	query := fmt.Sprintf(
		`SELECT %s FROM %s WHERE tenant_id IS NULL AND %s > ? ORDER BY %s ASC LIMIT ?`,
		step.PKColumn, step.Table, step.PKColumn, step.PKColumn,
	)
	res, err := tx.QueryContext(ctx, query, cursor, step.ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("backfill select chunk %s: %w", step.Name, err)
	}
	defer res.Close()
	keys := make([]string, 0, step.ChunkSize)
	for res.Next() {
		var k string
		if err := res.Scan(&k); err != nil {
			return nil, fmt.Errorf("backfill scan chunk key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, res.Err()
}

// updateRowsToDefault binds tenant_id=defaultTenantID for the chunk.
func updateRowsToDefault(ctx context.Context, tx *sql.Tx, step BackfillStep, defaultTenantID string, keys []string) error {
	placeholders, args := buildInClause(keys)
	args = append([]any{defaultTenantID}, args...)
	query := fmt.Sprintf(
		`UPDATE %s SET tenant_id = ? WHERE tenant_id IS NULL AND %s IN (%s)`,
		step.Table, step.PKColumn, placeholders,
	)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("backfill update default %s: %w", step.Name, err)
	}
	return nil
}

// updateRowsFromParent binds tenant_id from the parent row. Detects
// orphans (parent missing OR parent.tenant_id NULL) and surfaces an
// OrphanError so the caller can emit `daemon.migration.orphan_detected`
// and fail the step.
func updateRowsFromParent(ctx context.Context, tx *sql.Tx, step BackfillStep, keys []string) error {
	// First detect orphans BEFORE the UPDATE so we don't half-bind a
	// chunk and then fail. The orphan detection joins child→parent and
	// returns the first row in the chunk whose parent.tenant_id is NULL
	// (or the parent does not exist).
	placeholders, args := buildInClause(keys)
	orphanQuery := fmt.Sprintf(
		`SELECT c.%s, c.%s
		FROM %s AS c
		LEFT JOIN %s AS p ON p.%s = c.%s
		WHERE c.tenant_id IS NULL AND c.%s IN (%s)
		AND (p.%s IS NULL OR p.tenant_id IS NULL)
		ORDER BY c.%s ASC LIMIT 1`,
		step.PKColumn, step.ParentFKColumn,
		step.Table,
		step.ParentTable, step.ParentTablePK, step.ParentFKColumn,
		step.PKColumn, placeholders,
		step.ParentTablePK,
		step.PKColumn,
	)
	row := tx.QueryRowContext(ctx, orphanQuery, args...)
	var orphanPK, orphanFK sql.NullString
	if err := row.Scan(&orphanPK, &orphanFK); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("backfill orphan probe %s: %w", step.Name, err)
	}
	if orphanPK.Valid {
		return &OrphanError{
			Table:       step.Table,
			ParentTable: step.ParentTable,
			ParentFK:    step.ParentFKColumn,
			ParentValue: orphanFK.String,
		}
	}

	// No orphans; bind tenant_id from parent in one SQL statement.
	updateQuery := fmt.Sprintf(
		`UPDATE %s SET tenant_id = (
			SELECT p.tenant_id FROM %s AS p WHERE p.%s = %s.%s
		) WHERE tenant_id IS NULL AND %s IN (%s)`,
		step.Table,
		step.ParentTable, step.ParentTablePK, step.Table, step.ParentFKColumn,
		step.PKColumn, placeholders,
	)
	if _, err := tx.ExecContext(ctx, updateQuery, args...); err != nil {
		return fmt.Errorf("backfill update from parent %s: %w", step.Name, err)
	}
	return nil
}

// buildInClause returns "?,?,?,..." and the matching args slice for an
// IN-list of string values.
func buildInClause(keys []string) (string, []any) {
	if len(keys) == 0 {
		return "", nil
	}
	placeholders := make([]byte, 0, 2*len(keys)-1)
	args := make([]any, 0, len(keys))
	for i, k := range keys {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, k)
	}
	return string(placeholders), args
}
