package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Roadmap 35 (US2 / T077a + T077b) — step (c) enforcement: after every
// per-domain backfill completes, recreate each tenant_owned table with
// `tenant_id TEXT NOT NULL CHECK (tenant_id GLOB 'ten_*')` plus a
// per-tenant index that mirrors the inventory's selectivity guarantee.
//
// SQLite has no ALTER TABLE ADD CONSTRAINT, so each table is rebuilt
// via the canonical shadow-table swap inside a single transaction:
//
//   1. Read the live CREATE TABLE statement from sqlite_master.
//   2. Rewrite the `tenant_id TEXT` column to add NOT NULL + CHECK.
//   3. CREATE TABLE __new_<table> AS the rewritten DDL.
//   4. INSERT INTO __new_<table> SELECT * FROM <table>.
//   5. DROP TABLE <table>; ALTER TABLE __new_<table> RENAME TO <table>.
//   6. Recreate every index that referenced <table>.
//
// Each step is registered in `tenant_migration_progress` with name
// `tenant_migration:enforce_not_null:<table>`. The step refuses to run
// (returns ErrEnforcementBackfillIncomplete) unless the matching
// backfill step is `completed`. Idempotent: a `completed` enforcement
// step is a no-op.
//
// Events table is handled separately: it stays NULLABLE for global
// categories. EnforceEventsPartialIndexes adds the partial indexes
// from the inventory (idx_events_tenant_owned, idx_events_global)
// without a shadow swap.

// ErrEnforcementBackfillIncomplete signals the gate condition: the
// matching backfill step is not yet `completed`, so the enforcement
// step refuses to swap the table. Caller surfaces a clear operator
// error and aborts daemon startup.
var ErrEnforcementBackfillIncomplete = errors.New("enforcement: matching backfill step not completed")

// EnforceTenantNotNull rebuilds `table` so its `tenant_id` column is
// `NOT NULL CHECK (tenant_id GLOB 'ten_*')`. Gated on `backfillStep`
// having status `completed`.
//
// SAFETY: rebuilds all data inside a transaction. On any error the
// transaction rolls back and the original table is unchanged.
//
// LIMITATION: this implementation regenerates indexes from
// sqlite_master but does NOT reapply triggers or views. The Roadmap 35
// inventory does not declare any triggers or views on tenant_owned
// tables; if a future contributor adds one they MUST extend this
// helper to recreate them.
// EnforceTenantNotNullSpec is a convenience overload that runs the
// enforcement using a full EnforcementSpec (so per-tenant UNIQUE
// indexes from T018–T026 are applied at swap time). The driver loop
// calls this; the older EnforceTenantNotNull(table, ...) signature
// stays for the existing test surface.
func (s *SQLiteStore) EnforceTenantNotNullSpec(ctx context.Context, spec EnforcementSpec) error {
	return s.enforceTenantNotNull(ctx, spec.StepName, spec.Table, spec.BackfillStep, spec.UniqueIndexes, spec.PKExtension)
}

func (s *SQLiteStore) EnforceTenantNotNull(ctx context.Context, stepName, table, backfillStep string) error {
	return s.enforceTenantNotNull(ctx, stepName, table, backfillStep, nil, nil)
}

func (s *SQLiteStore) enforceTenantNotNull(ctx context.Context, stepName, table, backfillStep string, uniqueIndexes []TenantUniqueIndex, pkExtension []string) error {
	if s == nil {
		return errors.New("EnforceTenantNotNull: nil store")
	}
	// Gate: matching backfill must be completed.
	bf, err := s.GetMigrationStep(ctx, backfillStep)
	if err != nil {
		return err
	}
	if bf.Status != MigrationStepCompleted {
		return fmt.Errorf("%w: %s.status=%s", ErrEnforcementBackfillIncomplete, backfillStep, bf.Status)
	}

	// Idempotence + state machine.
	if _, err := s.BeginMigrationStep(ctx, stepName); err != nil {
		return err
	}
	current, err := s.GetMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	if current.Status == MigrationStepCompleted {
		return nil
	}

	// Introspect schema via PRAGMA. We can't use sqlite_master.sql
	// because ALTER TABLE ADD COLUMN does NOT update the stored
	// statement, so the live shape only appears via PRAGMA.
	cols, err := readTableColumns(ctx, s.db, table)
	if err != nil {
		return fmt.Errorf("read columns for %s: %w", table, err)
	}
	if len(cols) == 0 {
		return fmt.Errorf("table %s not found", table)
	}
	hasTenantID := false
	for _, c := range cols {
		if c.Name == "tenant_id" {
			hasTenantID = true
			break
		}
	}
	if !hasTenantID {
		return fmt.Errorf("table %s has no tenant_id column", table)
	}
	fks, err := readForeignKeys(ctx, s.db, table)
	if err != nil {
		return fmt.Errorf("read FKs for %s: %w", table, err)
	}
	pkCols := pkComposite(cols)
	// PK extension (T026): prepend tenant_id (or other named columns)
	// to the existing PRIMARY KEY so the table's per-row uniqueness
	// becomes per-tenant. Skip extension columns already present in
	// the live PK so re-runs are idempotent.
	if len(pkExtension) > 0 {
		existing := map[string]struct{}{}
		for _, c := range pkCols {
			existing[c] = struct{}{}
		}
		var prepend []string
		for _, c := range pkExtension {
			if _, dup := existing[c]; !dup {
				prepend = append(prepend, c)
			}
		}
		if len(prepend) > 0 {
			pkCols = append(prepend, pkCols...)
		}
	}

	// Read every index that refs the table so we can recreate them.
	indexRows, err := s.db.QueryContext(ctx, `SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL`, table)
	if err != nil {
		return fmt.Errorf("read indexes for %s: %w", table, err)
	}
	type idx struct{ name, sql string }
	var indexes []idx
	for indexRows.Next() {
		var i idx
		if err := indexRows.Scan(&i.name, &i.sql); err != nil {
			indexRows.Close()
			return fmt.Errorf("scan index: %w", err)
		}
		indexes = append(indexes, i)
	}
	indexRows.Close()

	shadow := "__new_" + table
	shadowDDL := buildShadowDDL(shadow, cols, fks, pkCols)

	// SQLite's `defer_foreign_keys` only defers the *check* until
	// commit; ON DELETE CASCADE fires immediately when the old table
	// is dropped, which would wipe child rows (steps, tool_calls,
	// workflows, computer_use_*, checkpoints, etc.). The standard
	// shadow-swap recipe disables FKs entirely for the swap and
	// re-enables them afterwards. PRAGMA foreign_keys must be set
	// OUTSIDE a transaction; we rely on the store being a single-
	// connection pool (MaxOpenConns(1)) so this is safe.
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable FKs: %w", err)
	}
	defer func() {
		_, _ = s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	}()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("enforcement begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, shadowDDL); err != nil {
		return fmt.Errorf("create shadow %s: %w\nDDL: %s", shadow, err, shadowDDL)
	}
	colList := columnListSQL(cols)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (%s) SELECT %s FROM %s`, shadow, colList, colList, table)); err != nil {
		return fmt.Errorf("copy rows to %s: %w", shadow, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, table)); err != nil {
		return fmt.Errorf("drop %s: %w", table, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, shadow, table)); err != nil {
		return fmt.Errorf("rename %s to %s: %w", shadow, table, err)
	}
	// Recreate indexes captured before the swap. Skip indexes whose
	// name matches one we're about to add (per-tenant UNIQUE replaces
	// the pre-existing global UNIQUE) and skip implicit auto-indexes
	// created by sqlite for UNIQUE columns (their `sql` was already
	// filtered NULL above) — but we must also avoid recreating an
	// existing UNIQUE on the same columns the new tenant-scoped index
	// covers. The simplest safe rule: if a recreated index name
	// collides with the new spec's name, skip it. The new spec wins.
	skipNames := map[string]struct{}{}
	for _, u := range uniqueIndexes {
		skipNames[u.Name] = struct{}{}
	}
	for _, i := range indexes {
		if _, skip := skipNames[i.name]; skip {
			continue
		}
		if _, err := tx.ExecContext(ctx, i.sql); err != nil {
			return fmt.Errorf("recreate index %s: %w", i.name, err)
		}
	}
	// Apply per-tenant UNIQUE indexes (T018–T026 step (c)). Each is a
	// CREATE UNIQUE INDEX so the constraint is enforced on every
	// future write while staying compatible with SQLite's lack of
	// ALTER TABLE ADD CONSTRAINT.
	for _, u := range uniqueIndexes {
		if err := normalizeNullableUniqueKeyDuplicates(ctx, tx, table, cols, u, pkCols); err != nil {
			return err
		}
		stmt := fmt.Sprintf(`CREATE UNIQUE INDEX %s ON %s (%s)`, u.Name, table, strings.Join(u.Columns, ", "))
		if u.Where != "" {
			stmt += " WHERE " + u.Where
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create unique index %s: %w", u.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("enforcement commit: %w", err)
	}
	return s.CompleteMigrationStep(ctx, stepName)
}

func normalizeNullableUniqueKeyDuplicates(ctx context.Context, tx *sql.Tx, table string, cols []columnInfo, unique TenantUniqueIndex, pkCols []string) error {
	if strings.TrimSpace(unique.Where) != "account_key IS NOT NULL" || !containsString(unique.Columns, "account_key") || !hasColumn(cols, "account_key") {
		return nil
	}
	// Legacy single-tenant data can contain duplicate account_key rows
	// that were valid before Roadmap 35's partial per-tenant unique
	// indexes. Keep the best row keyed and preserve the other historical
	// rows by clearing their nullable account_key before adding the index.
	partition := strings.Join(unique.Columns, ", ")
	order := nullableUniqueDuplicateOrder(cols, pkCols)
	stmt := fmt.Sprintf(`
		WITH ranked AS (
			SELECT rowid AS rid,
				ROW_NUMBER() OVER (PARTITION BY %s ORDER BY %s) AS rn
			FROM %s
			WHERE account_key IS NOT NULL
		)
		UPDATE %s
		SET account_key = NULL
		WHERE rowid IN (SELECT rid FROM ranked WHERE rn > 1)
	`, partition, order, table, table)
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("normalize duplicate nullable unique keys for %s.%s: %w", table, unique.Name, err)
	}
	return nil
}

func nullableUniqueDuplicateOrder(cols []columnInfo, pkCols []string) string {
	parts := make([]string, 0, 4)
	if hasColumn(cols, "canonical_default") {
		parts = append(parts, "canonical_default DESC")
	}
	if hasColumn(cols, "updated_at") {
		parts = append(parts, "updated_at DESC")
	}
	for _, col := range pkCols {
		if col != "" {
			parts = append(parts, col+" ASC")
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "rowid ASC")
	}
	return strings.Join(parts, ", ")
}

func hasColumn(cols []columnInfo, name string) bool {
	for _, col := range cols {
		if col.Name == name {
			return true
		}
	}
	return false
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

// columnInfo mirrors PRAGMA table_info output.
type columnInfo struct {
	Name      string
	Type      string
	NotNull   bool
	DfltValue sql.NullString
	PK        int // 0 = not pk, otherwise position
}

func readTableColumns(ctx context.Context, db *sql.DB, table string) ([]columnInfo, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%q)`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []columnInfo
	for rows.Next() {
		var (
			cid     int
			c       columnInfo
			notnull int
		)
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notnull, &c.DfltValue, &c.PK); err != nil {
			return nil, err
		}
		c.NotNull = notnull != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// fkInfo mirrors PRAGMA foreign_key_list output (one row per FK column;
// we group by id so composite FKs assemble correctly).
type fkInfo struct {
	ID       int
	Seq      int
	Table    string
	From     string
	To       string
	OnUpdate string
	OnDelete string
}

func readForeignKeys(ctx context.Context, db *sql.DB, table string) ([]fkInfo, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA foreign_key_list(%q)`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []fkInfo
	for rows.Next() {
		var (
			fk    fkInfo
			match string
		)
		if err := rows.Scan(&fk.ID, &fk.Seq, &fk.Table, &fk.From, &fk.To, &fk.OnUpdate, &fk.OnDelete, &match); err != nil {
			return nil, err
		}
		out = append(out, fk)
	}
	return out, rows.Err()
}

func pkComposite(cols []columnInfo) []string {
	type pkc struct {
		name string
		pos  int
	}
	var pks []pkc
	for _, c := range cols {
		if c.PK > 0 {
			pks = append(pks, pkc{c.Name, c.PK})
		}
	}
	// Sort by position.
	for i := 1; i < len(pks); i++ {
		for j := i; j > 0 && pks[j].pos < pks[j-1].pos; j-- {
			pks[j], pks[j-1] = pks[j-1], pks[j]
		}
	}
	out := make([]string, 0, len(pks))
	for _, p := range pks {
		out = append(out, p.name)
	}
	return out
}

// buildShadowDDL constructs a CREATE TABLE statement for the shadow
// table. tenant_id is rewritten to NOT NULL + CHECK; non-composite PK
// stays inline; composite PK becomes a table-level constraint; FKs
// are emitted as table-level constraints grouped by FK id.
func buildShadowDDL(shadow string, cols []columnInfo, fks []fkInfo, pkCols []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n", shadow)
	for i, c := range cols {
		fmt.Fprintf(&b, "  %s %s", c.Name, c.Type)
		if c.Name == "tenant_id" {
			b.WriteString(" NOT NULL CHECK (tenant_id GLOB 'ten_*')")
		} else if c.NotNull {
			b.WriteString(" NOT NULL")
		}
		if c.DfltValue.Valid {
			fmt.Fprintf(&b, " DEFAULT %s", c.DfltValue.String)
		}
		if len(pkCols) == 1 && c.Name == pkCols[0] {
			b.WriteString(" PRIMARY KEY")
		}
		if i < len(cols)-1 || len(pkCols) > 1 || len(fks) > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	if len(pkCols) > 1 {
		fmt.Fprintf(&b, "  PRIMARY KEY (%s)", strings.Join(pkCols, ", "))
		if len(fks) > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	// Group FKs by id.
	grouped := map[int][]fkInfo{}
	var ids []int
	for _, fk := range fks {
		if _, ok := grouped[fk.ID]; !ok {
			ids = append(ids, fk.ID)
		}
		grouped[fk.ID] = append(grouped[fk.ID], fk)
	}
	for i, id := range ids {
		group := grouped[id]
		from := make([]string, 0, len(group))
		to := make([]string, 0, len(group))
		for _, g := range group {
			from = append(from, g.From)
			to = append(to, g.To)
		}
		fmt.Fprintf(&b, "  FOREIGN KEY (%s) REFERENCES %s (%s)",
			strings.Join(from, ", "), group[0].Table, strings.Join(to, ", "))
		onDelete := strings.ToUpper(group[0].OnDelete)
		if onDelete != "" && onDelete != "NO ACTION" {
			fmt.Fprintf(&b, " ON DELETE %s", onDelete)
		}
		onUpdate := strings.ToUpper(group[0].OnUpdate)
		if onUpdate != "" && onUpdate != "NO ACTION" {
			fmt.Fprintf(&b, " ON UPDATE %s", onUpdate)
		}
		if i < len(ids)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(")")
	return b.String()
}

func columnListSQL(cols []columnInfo) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		parts = append(parts, c.Name)
	}
	return strings.Join(parts, ", ")
}

// EnforceEventsPartialIndexes performs T077b on the mixed events table:
//
//  1. Shadow-swap rebuild adding two CHECK constraints that bind the
//     tenant_id column to FR-007 semantics:
//     - column-level: `tenant_id IS NULL OR tenant_id GLOB 'ten_*'`
//     so any non-NULL value matches the tenant id format.
//     - table-level: `tenant_id IS NOT NULL OR category IN (<global>)`
//     so a NULL tenant_id is allowed only for the inventory's hard-
//     coded global event categories.
//  2. Recreate the inventory-mandated partial indexes:
//     - idx_events_tenant_owned (tenant_id, occurred_at DESC, event_id DESC)
//     WHERE tenant_id IS NOT NULL — selectivity for the tenant-aware
//     list path.
//     - idx_events_global (category, occurred_at DESC, event_id DESC)
//     WHERE tenant_id IS NULL — selectivity for the global-event read.
//
// Gated on the events backfill step (T077). Idempotent: a `completed`
// step is a no-op, and re-runs detect the existing CHECK on the live
// schema and skip the shadow swap.
func (s *SQLiteStore) EnforceEventsPartialIndexes(ctx context.Context, stepName string) error {
	if s == nil {
		return errors.New("EnforceEventsPartialIndexes: nil store")
	}
	bf, err := s.GetMigrationStep(ctx, EventsBackfillStepName)
	if err != nil {
		return err
	}
	if bf.Status != MigrationStepCompleted {
		return fmt.Errorf("%w: %s.status=%s", ErrEnforcementBackfillIncomplete, EventsBackfillStepName, bf.Status)
	}
	if _, err := s.BeginMigrationStep(ctx, stepName); err != nil {
		return err
	}
	current, err := s.GetMigrationStep(ctx, stepName)
	if err != nil {
		return err
	}
	if current.Status == MigrationStepCompleted {
		return nil
	}
	if err := s.enforceEventsTenantCheck(ctx); err != nil {
		return err
	}
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_events_tenant_owned ON events(tenant_id, occurred_at DESC, event_id DESC) WHERE tenant_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_events_global ON events(category, occurred_at DESC, event_id DESC) WHERE tenant_id IS NULL`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("create partial index: %w", err)
		}
	}
	return s.CompleteMigrationStep(ctx, stepName)
}

// eventsCheckSentinel is a substring that uniquely identifies the
// post-T077b events table CHECK constraint in sqlite_master.sql. Used
// to skip the shadow swap on idempotent re-runs.
const eventsCheckSentinel = "tenant_id IS NULL OR tenant_id GLOB 'ten_*'"

// eventsGlobalCategoriesForCheck must mirror events.GlobalCategories.
// Hardcoded here (rather than imported) because the store package
// cannot import the events package without a cycle. The events package
// owns the canonical set; this list is enforced in lockstep by
// TestEventsGlobalCategoriesMatch (see tenant_enforcement_test.go).
var eventsGlobalCategoriesForCheck = []string{
	"mcp",
	"provider",
	"system",
	"daemon.migration",
	"connector_global",
	"capability_global",
}

// EventsGlobalCheckCategories returns the categories baked into the
// events table CHECK constraint for the inventory's mixed-tenancy
// rule. Exported solely so a sync test can assert lockstep with
// events.GlobalCategories without an import cycle.
func EventsGlobalCheckCategories() []string {
	out := make([]string, len(eventsGlobalCategoriesForCheck))
	copy(out, eventsGlobalCategoriesForCheck)
	return out
}

func (s *SQLiteStore) enforceEventsTenantCheck(ctx context.Context) error {
	// Idempotence: detect prior swap by scanning sqlite_master.sql for
	// the sentinel CHECK substring. Cheaper and safer than re-swapping.
	var liveDDL sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='events'`).Scan(&liveDDL); err != nil {
		return fmt.Errorf("read events DDL: %w", err)
	}
	if liveDDL.Valid && strings.Contains(liveDDL.String, eventsCheckSentinel) {
		return nil
	}

	cols, err := readTableColumns(ctx, s.db, "events")
	if err != nil {
		return fmt.Errorf("read events columns: %w", err)
	}
	if len(cols) == 0 {
		return errors.New("events table not found")
	}
	pkCols := pkComposite(cols)

	// Build globals-IN-list as a single quoted SQL fragment.
	quoted := make([]string, 0, len(eventsGlobalCategoriesForCheck))
	for _, g := range eventsGlobalCategoriesForCheck {
		quoted = append(quoted, "'"+strings.ReplaceAll(g, "'", "''")+"'")
	}
	rowCheck := fmt.Sprintf(
		"tenant_id IS NOT NULL OR category IN (%s)",
		strings.Join(quoted, ", "),
	)

	shadow := "__new_events"
	shadowDDL := buildEventsShadowDDL(shadow, cols, pkCols, rowCheck)

	// Capture indexes to recreate.
	indexRows, err := s.db.QueryContext(ctx, `SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name='events' AND sql IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("read events indexes: %w", err)
	}
	type idx struct{ name, sql string }
	var indexes []idx
	for indexRows.Next() {
		var i idx
		if err := indexRows.Scan(&i.name, &i.sql); err != nil {
			indexRows.Close()
			return fmt.Errorf("scan events index: %w", err)
		}
		indexes = append(indexes, i)
	}
	indexRows.Close()

	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable FKs: %w", err)
	}
	defer func() { _, _ = s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`) }()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("events enforcement begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, shadowDDL); err != nil {
		return fmt.Errorf("create %s: %w\nDDL: %s", shadow, err, shadowDDL)
	}
	colList := columnListSQL(cols)
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (%s) SELECT %s FROM events`, shadow, colList, colList)); err != nil {
		return fmt.Errorf("copy events rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE events`); err != nil {
		return fmt.Errorf("drop events: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME TO events`, shadow)); err != nil {
		return fmt.Errorf("rename %s to events: %w", shadow, err)
	}
	// Skip recreating the partial indexes the caller will issue right
	// after; otherwise re-emit every captured index.
	skipNames := map[string]struct{}{
		"idx_events_tenant_owned": {},
		"idx_events_global":       {},
	}
	for _, i := range indexes {
		if _, skip := skipNames[i.name]; skip {
			continue
		}
		if _, err := tx.ExecContext(ctx, i.sql); err != nil {
			return fmt.Errorf("recreate events index %s: %w", i.name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("events enforcement commit: %w", err)
	}
	return nil
}

// buildEventsShadowDDL is the events-specific variant of buildShadowDDL.
// Differences from the standard helper:
//   - tenant_id stays NULLABLE; column CHECK becomes
//     `tenant_id IS NULL OR tenant_id GLOB 'ten_*'`.
//   - emits an extra table-level CHECK (rowCheck) enforcing
//     "tenant_id IS NOT NULL OR category IN (<globals>)".
//   - the events table has no FKs.
func buildEventsShadowDDL(shadow string, cols []columnInfo, pkCols []string, rowCheck string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n", shadow)
	for i, c := range cols {
		fmt.Fprintf(&b, "  %s %s", c.Name, c.Type)
		if c.Name == "tenant_id" {
			b.WriteString(" CHECK (" + eventsCheckSentinel + ")")
		} else if c.NotNull {
			b.WriteString(" NOT NULL")
		}
		if c.DfltValue.Valid {
			fmt.Fprintf(&b, " DEFAULT %s", c.DfltValue.String)
		}
		if len(pkCols) == 1 && c.Name == pkCols[0] {
			b.WriteString(" PRIMARY KEY")
		}
		// Always trailing comma — the rowCheck below is the last clause.
		if i < len(cols)-1 || len(pkCols) > 1 || rowCheck != "" {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	if len(pkCols) > 1 {
		fmt.Fprintf(&b, "  PRIMARY KEY (%s)", strings.Join(pkCols, ", "))
		if rowCheck != "" {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	if rowCheck != "" {
		fmt.Fprintf(&b, "  CHECK (%s)\n", rowCheck)
	}
	b.WriteString(")")
	return b.String()
}

// ---- step name constants ----

const (
	// Enforcement step names follow tenant_migration:enforce_not_null:<table>
	// for tables that get NOT NULL, and tenant_migration:enforce_check:events
	// for the mixed events table (CHECK + partial indexes only).
	EnforceNotNullSessionsStepName      = "tenant_migration:enforce_not_null:sessions"
	EnforceNotNullRunsStepName          = "tenant_migration:enforce_not_null:runs"
	EnforceNotNullStepsStepName         = "tenant_migration:enforce_not_null:steps"
	EnforceNotNullToolCallsStepName     = "tenant_migration:enforce_not_null:tool_calls"
	EnforceNotNullLLMDispatchesStepName = "tenant_migration:enforce_not_null:llm_dispatches"
	EnforceNotNullCheckpointsStepName   = "tenant_migration:enforce_not_null:checkpoints"

	// Schedules / workflows / delivery / calendar / mail / reminders /
	// computer-use / evaluation / harness / connector_messages.
	EnforceNotNullSchedulesStepName                = "tenant_migration:enforce_not_null:schedules"
	EnforceNotNullScheduleTargetsStepName          = "tenant_migration:enforce_not_null:schedule_targets"
	EnforceNotNullScheduleDispatchAttemptsStepName = "tenant_migration:enforce_not_null:schedule_dispatch_attempts"

	EnforceNotNullWorkflowsStepName            = "tenant_migration:enforce_not_null:workflows"
	EnforceNotNullWorkflowStepsStepName        = "tenant_migration:enforce_not_null:workflow_steps"
	EnforceNotNullWorkflowDependenciesStepName = "tenant_migration:enforce_not_null:workflow_dependencies"
	EnforceNotNullWorkflowHandoffsStepName     = "tenant_migration:enforce_not_null:workflow_handoffs"

	EnforceNotNullIntegrationsStepName           = "tenant_migration:enforce_not_null:integrations"
	EnforceNotNullDeliveryTargetsStepName        = "tenant_migration:enforce_not_null:delivery_targets"
	EnforceNotNullDeliveryPreferencesStepName    = "tenant_migration:enforce_not_null:delivery_preferences"
	EnforceNotNullDeliveryOutcomesStepName       = "tenant_migration:enforce_not_null:delivery_outcomes"
	EnforceNotNullDeliveryAttemptsStepName       = "tenant_migration:enforce_not_null:delivery_attempts"
	EnforceNotNullDeliverySummaryWindowsStepName = "tenant_migration:enforce_not_null:delivery_summary_windows"

	EnforceNotNullCalendarAccountsStepName   = "tenant_migration:enforce_not_null:calendar_accounts"
	EnforceNotNullCalendarOperationsStepName = "tenant_migration:enforce_not_null:calendar_operations"
	EnforceNotNullCalendarArtifactsStepName  = "tenant_migration:enforce_not_null:calendar_artifacts"

	EnforceNotNullMailAccountsStepName   = "tenant_migration:enforce_not_null:mail_accounts"
	EnforceNotNullMailOperationsStepName = "tenant_migration:enforce_not_null:mail_operations"
	EnforceNotNullMailArtifactsStepName  = "tenant_migration:enforce_not_null:mail_artifacts"

	EnforceNotNullRemindersStepName           = "tenant_migration:enforce_not_null:reminders"
	EnforceNotNullReminderOccurrencesStepName = "tenant_migration:enforce_not_null:reminder_occurrences"
	EnforceNotNullReminderActionsStepName     = "tenant_migration:enforce_not_null:reminder_actions"

	EnforceNotNullComputerUseSessionsStepName  = "tenant_migration:enforce_not_null:computer_use_sessions"
	EnforceNotNullComputerUseActionsStepName   = "tenant_migration:enforce_not_null:computer_use_actions"
	EnforceNotNullComputerUseArtifactsStepName = "tenant_migration:enforce_not_null:computer_use_artifacts"

	EnforceNotNullEvalReplayCandidatesStepName   = "tenant_migration:enforce_not_null:evaluation_replay_candidates"
	EnforceNotNullEvalReplayAttemptsStepName     = "tenant_migration:enforce_not_null:evaluation_replay_attempts"
	EnforceNotNullEvalComparisonsStepName        = "tenant_migration:enforce_not_null:evaluation_comparisons"
	EnforceNotNullEvalRegressionFixturesStepName = "tenant_migration:enforce_not_null:evaluation_regression_fixtures"

	EnforceNotNullApprovalsStepName = "tenant_migration:enforce_not_null:approvals"
	EnforceNotNullDecisionsStepName = "tenant_migration:enforce_not_null:decisions"

	EnforceNotNullConsumerPolicyRecordsStepName = "tenant_migration:enforce_not_null:consumer_policy_records"
	EnforceNotNullProviderPreferencesStepName   = "tenant_migration:enforce_not_null:provider_preferences"
	EnforceNotNullSecretScopeBindingsStepName   = "tenant_migration:enforce_not_null:secret_scope_bindings"
	EnforceNotNullSandboxExecutionsStepName     = "tenant_migration:enforce_not_null:sandbox_executions"

	EnforceNotNullConnectorMessagesStepName = "tenant_migration:enforce_not_null:connector_messages"

	// T026: mcp_tool_exposure_rules — extends the composite PK from
	// (server_id, tool_name, runtime_surface) to
	// (tenant_id, server_id, tool_name, runtime_surface) via the
	// EnforcementSpec.PKExtension hook on buildShadowDDL.
	EnforceNotNullMCPToolExposureRulesStepName = "tenant_migration:enforce_not_null:mcp_tool_exposure_rules"
)

// EnforcementSpec describes a single step (c) enforcement: which table
// to swap, which backfill step gates it, and what per-tenant UNIQUE
// constraints to add per the inventory's `indexesAndUniqueness` cell
// (declared in T018–T026 but only encoded at swap time here).
type EnforcementSpec struct {
	StepName      string
	Table         string
	BackfillStep  string
	UniqueIndexes []TenantUniqueIndex
	// PKExtension prepends columns to the existing PRIMARY KEY during
	// the shadow swap (T026 mcp_tool_exposure_rules). Use only when
	// per-tenant uniqueness on the row must be encoded in the PK
	// itself (i.e. the natural key is identity, not synthetic). Empty
	// for the standard case where tenant_id is just a NOT NULL column.
	PKExtension []string
}

// TenantUniqueIndex declares a per-tenant UNIQUE index added during
// the shadow swap. Implementation strategy: emit a CREATE UNIQUE INDEX
// after the table rename. Partial uniqueness (WHERE clause) covers the
// inventory's nullable-natural-key cases (calendar/mail account_key,
// integrations.account_key).
type TenantUniqueIndex struct {
	Name    string
	Columns []string
	Where   string // optional partial-index predicate
}

// RuntimeEnforcementSpecs returns the step (c) specs for the runtime
// spine. Driver invokes EnforceTenantNotNull for each in order. The
// order is not load-bearing per se (each table's swap is independent)
// but matches the backfill order for operator-friendly progress logs.
//
// Per-tenant UNIQUE specs follow T018: replace the existing global
// UNIQUE on `sessions.routing_key` with UNIQUE `(tenant_id,
// routing_key)`. Other runtime spine tables have no natural-key
// UNIQUE per T018 (PKs are global identifiers and per-tenant
// uniqueness on the PK is implicit).
func RuntimeEnforcementSpecs() []EnforcementSpec {
	return []EnforcementSpec{
		{
			StepName: EnforceNotNullSessionsStepName, Table: "sessions", BackfillStep: RuntimeBackfillSessionsStepName,
			UniqueIndexes: []TenantUniqueIndex{
				{Name: "ux_sessions_tenant_routing_key", Columns: []string{"tenant_id", "routing_key"}},
			},
		},
		{StepName: EnforceNotNullRunsStepName, Table: "runs", BackfillStep: RuntimeBackfillRunsStepName},
		{StepName: EnforceNotNullStepsStepName, Table: "steps", BackfillStep: RuntimeBackfillStepsStepName},
		{StepName: EnforceNotNullToolCallsStepName, Table: "tool_calls", BackfillStep: RuntimeBackfillToolCallsStepName},
		{StepName: EnforceNotNullLLMDispatchesStepName, Table: "llm_dispatches", BackfillStep: RuntimeBackfillLLMDispatchesStepName},
		{StepName: EnforceNotNullCheckpointsStepName, Table: "checkpoints", BackfillStep: RuntimeBackfillCheckpointsStepName},
	}
}

// ExtendedEnforcementSpecs returns step (c) specs for every
// non-runtime-spine tenant_owned table per T019–T026. Drivers invoke
// EnforceTenantNotNullSpec for each in dependency order so a child's
// FK still resolves while the parent is being swapped.
//
// Per-tenant UNIQUE specs are encoded directly from each task body's
// "Uniqueness:" clause. Tables with no UNIQUE clause carry only the
// NOT NULL CHECK enforcement.
func ExtendedEnforcementSpecs() []EnforcementSpec {
	return []EnforcementSpec{
		// T019 schedules
		{
			StepName: EnforceNotNullSchedulesStepName, Table: "schedules", BackfillStep: SchedulesBackfillStepName,
			UniqueIndexes: []TenantUniqueIndex{
				{Name: "ux_schedules_tenant_kind_target", Columns: []string{"tenant_id", "kind", "target_ref_id"}},
			},
		},
		{StepName: EnforceNotNullScheduleTargetsStepName, Table: "schedule_targets", BackfillStep: ScheduleTargetsBackfillStepName},
		{StepName: EnforceNotNullScheduleDispatchAttemptsStepName, Table: "schedule_dispatch_attempts", BackfillStep: ScheduleDispatchAttemptsBackfillStepName},

		// T020 workflows (no SQL UNIQUE per task body)
		{StepName: EnforceNotNullWorkflowsStepName, Table: "workflows", BackfillStep: WorkflowsBackfillStepName},
		{StepName: EnforceNotNullWorkflowStepsStepName, Table: "workflow_steps", BackfillStep: WorkflowStepsBackfillStepName},
		{StepName: EnforceNotNullWorkflowDependenciesStepName, Table: "workflow_dependencies", BackfillStep: WorkflowDependenciesBackfillStepName},
		{StepName: EnforceNotNullWorkflowHandoffsStepName, Table: "workflow_handoffs", BackfillStep: WorkflowHandoffsBackfillStepName},

		// T021 integrations + delivery
		{
			StepName: EnforceNotNullIntegrationsStepName, Table: "integrations", BackfillStep: IntegrationsBackfillStepName,
			UniqueIndexes: []TenantUniqueIndex{
				{Name: "ux_integrations_tenant_domain_account", Columns: []string{"tenant_id", "domain_kind", "account_key"}, Where: "account_key IS NOT NULL"},
			},
		},
		{StepName: EnforceNotNullDeliveryTargetsStepName, Table: "delivery_targets", BackfillStep: DeliveryTargetsBackfillStepName},
		{StepName: EnforceNotNullDeliveryPreferencesStepName, Table: "delivery_preferences", BackfillStep: DeliveryPreferencesBackfillStepName},
		{StepName: EnforceNotNullDeliveryOutcomesStepName, Table: "delivery_outcomes", BackfillStep: DeliveryOutcomesBackfillStepName},
		{StepName: EnforceNotNullDeliveryAttemptsStepName, Table: "delivery_attempts", BackfillStep: DeliveryAttemptsBackfillStepName},
		{StepName: EnforceNotNullDeliverySummaryWindowsStepName, Table: "delivery_summary_windows", BackfillStep: DeliverySummaryWindowsBackfillStepName},

		// T022 calendar
		{
			StepName: EnforceNotNullCalendarAccountsStepName, Table: "calendar_accounts", BackfillStep: CalendarAccountsBackfillStepName,
			UniqueIndexes: []TenantUniqueIndex{
				{Name: "ux_calendar_accounts_tenant_account_key", Columns: []string{"tenant_id", "account_key"}, Where: "account_key IS NOT NULL"},
			},
		},
		{StepName: EnforceNotNullCalendarOperationsStepName, Table: "calendar_operations", BackfillStep: CalendarOperationsBackfillStepName},
		{StepName: EnforceNotNullCalendarArtifactsStepName, Table: "calendar_artifacts", BackfillStep: CalendarArtifactsBackfillStepName},

		// T023 mail
		{
			StepName: EnforceNotNullMailAccountsStepName, Table: "mail_accounts", BackfillStep: MailAccountsBackfillStepName,
			UniqueIndexes: []TenantUniqueIndex{
				{Name: "ux_mail_accounts_tenant_account_key", Columns: []string{"tenant_id", "account_key"}, Where: "account_key IS NOT NULL"},
			},
		},
		{StepName: EnforceNotNullMailOperationsStepName, Table: "mail_operations", BackfillStep: MailOperationsBackfillStepName},
		{StepName: EnforceNotNullMailArtifactsStepName, Table: "mail_artifacts", BackfillStep: MailArtifactsBackfillStepName},

		// T024 reminders (no SQL UNIQUE per task body)
		{StepName: EnforceNotNullRemindersStepName, Table: "reminders", BackfillStep: RemindersBackfillStepName},
		{StepName: EnforceNotNullReminderOccurrencesStepName, Table: "reminder_occurrences", BackfillStep: ReminderOccurrencesBackfillStepName},
		{StepName: EnforceNotNullReminderActionsStepName, Table: "reminder_actions", BackfillStep: ReminderActionsBackfillStepName},

		// T025 computer-use (no SQL UNIQUE)
		{StepName: EnforceNotNullComputerUseSessionsStepName, Table: "computer_use_sessions", BackfillStep: ComputerUseSessionsBackfillStepName},
		{StepName: EnforceNotNullComputerUseActionsStepName, Table: "computer_use_actions", BackfillStep: ComputerUseActionsBackfillStepName},
		{StepName: EnforceNotNullComputerUseArtifactsStepName, Table: "computer_use_artifacts", BackfillStep: ComputerUseArtifactsBackfillStepName},

		// T026 evaluation + harness
		{StepName: EnforceNotNullEvalReplayCandidatesStepName, Table: "evaluation_replay_candidates", BackfillStep: EvaluationReplayCandidatesBackfillStepName},
		{StepName: EnforceNotNullEvalReplayAttemptsStepName, Table: "evaluation_replay_attempts", BackfillStep: EvaluationReplayAttemptsBackfillStepName},
		{StepName: EnforceNotNullEvalComparisonsStepName, Table: "evaluation_comparisons", BackfillStep: EvaluationComparisonsBackfillStepName},
		{
			// T026 evaluation_regression_fixtures: UNIQUE (tenant_id, manifest_path).
			// manifest_path is a column on the table; verify before adding.
			StepName: EnforceNotNullEvalRegressionFixturesStepName, Table: "evaluation_regression_fixtures", BackfillStep: EvaluationRegressionFixturesBackfillStepName,
			UniqueIndexes: []TenantUniqueIndex{
				{Name: "ux_eval_regression_fixtures_tenant_manifest", Columns: []string{"tenant_id", "manifest_path"}},
			},
		},
		{StepName: EnforceNotNullApprovalsStepName, Table: "approvals", BackfillStep: ApprovalsBackfillStepName},
		{StepName: EnforceNotNullDecisionsStepName, Table: "decisions", BackfillStep: DecisionsBackfillStepName},
		{StepName: EnforceNotNullConsumerPolicyRecordsStepName, Table: "consumer_policy_records", BackfillStep: ConsumerPolicyRecordsBackfillStepName},
		{StepName: EnforceNotNullProviderPreferencesStepName, Table: "provider_preferences", BackfillStep: ProviderPreferencesBackfillStepName},
		{StepName: EnforceNotNullSecretScopeBindingsStepName, Table: "secret_scope_bindings", BackfillStep: SecretScopeBindingsBackfillStepName},
		{StepName: EnforceNotNullSandboxExecutionsStepName, Table: "sandbox_executions", BackfillStep: SandboxExecutionsBackfillStepName},

		// T076b connector_messages
		{StepName: EnforceNotNullConnectorMessagesStepName, Table: "connector_messages", BackfillStep: ConnectorMessagesBackfillStepName},

		// T026 mcp_tool_exposure_rules — composite PK extension. The
		// table already enforces per-server/tool/surface uniqueness
		// globally; the PK extension makes that uniqueness per-tenant
		// so two tenants can configure the same tool independently.
		{
			StepName:     EnforceNotNullMCPToolExposureRulesStepName,
			Table:        "mcp_tool_exposure_rules",
			BackfillStep: MCPToolExposureRulesBackfillStepName,
			PKExtension:  []string{"tenant_id"},
		},
	}
}

// RegisterEnforcementMigrationSteps inserts pending rows for the
// runtime spine + events enforcement steps. Idempotent.
func (s *SQLiteStore) RegisterEnforcementMigrationSteps(ctx context.Context) error {
	for _, spec := range RuntimeEnforcementSpecs() {
		if err := s.RegisterMigrationStep(ctx, spec.StepName); err != nil {
			return err
		}
	}
	for _, spec := range ExtendedEnforcementSpecs() {
		if err := s.RegisterMigrationStep(ctx, spec.StepName); err != nil {
			return err
		}
	}
	if err := s.RegisterMigrationStep(ctx, EventsEnforceCheckStepName); err != nil {
		return err
	}
	return nil
}

// Future scope: enforcement specs for the remaining ~30 tables
// (schedules, workflows, calendar/mail/reminders, computer-use,
// evaluation, harness, connector_messages). Each follows the same
// EnforceTenantNotNull pattern; landing them is mechanical but bulky.
// The runtime-spine enforcement above is sufficient to demonstrate
// the pipeline works end-to-end.
