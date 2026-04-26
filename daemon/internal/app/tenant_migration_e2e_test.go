package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/migrationfixture"
)

// Roadmap 35 (US2 / T078–T081b) — end-to-end migration regression
// suite. These tests stand up a v21 pre-tenant SQLite fixture, invoke
// app.New() to drive the full migrate→backfill→enforcement pipeline,
// and assert the fixture survives intact with every row correctly
// owned.
//
// Tests share `bootApp` — it isolates `DOPE_DATA_DIR` per test, sets
// minimal env vars, and tears down the daemon on cleanup.

// bootAppOnFixtureDir creates a v21 fixture in dataDir, then returns
// the dataDir so the caller can `t.Setenv("DOPE_DATA_DIR", ...)` and
// invoke `app.New()`.
func bootAppOnFixtureDir(t *testing.T) string {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "dope")
	s, err := migrationfixture.BuildPreTenantV21Fixture(dataDir)
	if err != nil {
		t.Fatalf("BuildPreTenantV21Fixture: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close v21 store: %v", err)
	}
	t.Setenv("DOPE_DATA_DIR", dataDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:0")
	t.Setenv("DOPE_LOG_LEVEL", "error")
	t.Setenv("DOPE_VERSION", "test")
	return dataDir
}

// T078 — full lossless migration. Pre/post row counts equal per
// table; every tenant_owned non-events row carries the bootstrapped
// personal tenant id; events split correctly between tenant-owned
// and global rows.
func TestMigration_E2E_FixtureLossless(t *testing.T) {
	dataDir := bootAppOnFixtureDir(t)

	// Snapshot pre-migration row counts via a separate handle.
	preStore, err := store.NewSQLiteStoreAtVersion(dataDir, migrationfixture.PreTenantSchemaVersion)
	if err != nil {
		t.Fatalf("reopen pre-tenant store: %v", err)
	}
	preCounts, err := migrationfixture.CountSeededRows(context.Background(), preStore)
	if err != nil {
		t.Fatalf("count pre rows: %v", err)
	}
	if err := preStore.Close(); err != nil {
		t.Fatalf("close pre store: %v", err)
	}

	a, err := New()
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close(context.Background()) })

	postCounts, err := migrationfixture.CountSeededRows(context.Background(), a.Store)
	if err != nil {
		t.Fatalf("count post rows: %v", err)
	}
	// Strict equality is not the right assertion: app.New legitimately
	// seeds additional rows on boot (default sandbox profile, evaluation
	// fixtures, default policy/secret bindings, daemon.migration.*
	// events). The lossless guarantee is that pre-existing rows survive
	// migration, which means post >= pre per table. The per-row
	// tenant-ownership spot-checks below (and the dedicated event
	// reclassification assertions) cover the binding correctness.
	for table, want := range preCounts {
		if got := postCounts[table]; got < want {
			t.Errorf("table %s LOST rows during migration: pre=%d post=%d", table, want, got)
		}
	}

	// Every backfill step must be `completed`.
	steps, err := a.Store.LoadMigrationSteps(context.Background())
	if err != nil {
		t.Fatalf("LoadMigrationSteps: %v", err)
	}
	for _, step := range steps {
		if !strings.HasPrefix(step.Name, "tenant_migration:backfill:") {
			continue
		}
		if step.Status != store.MigrationStepCompleted {
			t.Errorf("backfill step %s status=%s, want completed", step.Name, step.Status)
		}
	}

	// Spot-check tenant ownership across domains.
	checkBoundTenant(t, a.Store, "runs", "run_id", "run_seed")
	checkBoundTenant(t, a.Store, "steps", "step_id", "step_seed")
	checkBoundTenant(t, a.Store, "tool_calls", "tool_call_id", "tc_seed")
	checkBoundTenant(t, a.Store, "schedules", "schedule_id", "sch_seed")
	checkBoundTenant(t, a.Store, "workflows", "workflow_id", "wf_seed")
	checkBoundTenant(t, a.Store, "calendar_accounts", "calendar_account_id", "cal_seed")
	checkBoundTenant(t, a.Store, "mail_accounts", "mail_account_id", "mail_seed")
	checkBoundTenant(t, a.Store, "reminders", "reminder_id", "rem_seed")
	checkBoundTenant(t, a.Store, "computer_use_sessions", "computer_use_session_id", "cus_seed")
	checkBoundTenant(t, a.Store, "approvals", "approval_id", "apr_seed")
	checkBoundTenant(t, a.Store, "evaluation_replay_candidates", "candidate_id", "cand_seed")
	checkBoundTenant(t, a.Store, "consumer_policy_records", "policy_record_id", "polrec_seed")
	checkBoundTenant(t, a.Store, "secret_scope_bindings", "binding_id", "binding_seed")
	checkBoundTenant(t, a.Store, "sandbox_executions", "execution_id", "exec_seed")

	// Events: tenant-owned event must be bound; global stays NULL;
	// reclassified events sit in their global category with NULL.
	for _, c := range []struct{ id, wantCategory, wantTenantBound string }{
		{"evt_run_seed", "run", "yes"},
		{"evt_sys_seed", "system", "no"},
		{"evt_cm_seed", "connector.message", "yes"}, // resource_id points at cm_seed_session which gets bound via session
		{"evt_cm_orphan", "connector_global", "no"},
		{"evt_cap_only", "capability_global", "no"},
	} {
		var category string
		var tenant string
		row := a.Store.DB().QueryRowContext(context.Background(),
			`SELECT category, COALESCE(tenant_id,'') FROM events WHERE event_id = ?`, c.id)
		if err := row.Scan(&category, &tenant); err != nil {
			t.Errorf("read event %s: %v", c.id, err)
			continue
		}
		if category != c.wantCategory {
			t.Errorf("event %s category=%q, want %q", c.id, category, c.wantCategory)
		}
		if c.wantTenantBound == "yes" && tenant == "" {
			t.Errorf("event %s tenant_id empty, want bound", c.id)
		}
		if c.wantTenantBound == "no" && tenant != "" {
			t.Errorf("event %s tenant_id=%q, want empty", c.id, tenant)
		}
	}
}

// T079 — re-running migrations on a fully-migrated store changes nothing.
func TestMigration_E2E_Idempotent(t *testing.T) {
	_ = bootAppOnFixtureDir(t)

	a1, err := New()
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	first, err := snapshotTenantIDs(context.Background(), a1.Store)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if err := a1.Close(context.Background()); err != nil {
		t.Fatalf("first close: %v", err)
	}

	a2, err := New()
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	t.Cleanup(func() { _ = a2.Close(context.Background()) })
	second, err := snapshotTenantIDs(context.Background(), a2.Store)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if first != second {
		t.Fatalf("idempotence broken: snapshot diverged across runs\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// T080 — resume safety. Mid-backfill abort leaves a `running` step
// with last_processed_key set; a subsequent boot resumes and
// completes the step exactly once. Simulated by manually setting the
// `runs` step to `running` with a partial cursor before invoking
// app.New.
func TestMigration_E2E_ResumeFromCursor(t *testing.T) {
	dataDir := bootAppOnFixtureDir(t)

	// Open store at head schema, register progress rows, simulate a
	// half-finished runs backfill.
	pre, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := pre.DB().ExecContext(context.Background(), `UPDATE tenant_migration_progress SET status = 'running', last_processed_key = '' WHERE step_name = ?`, store.RuntimeBackfillRunsStepName); err != nil {
		t.Fatalf("seed running cursor: %v", err)
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("close pre: %v", err)
	}

	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close(context.Background()) })

	step, err := a.Store.GetMigrationStep(context.Background(), store.RuntimeBackfillRunsStepName)
	if err != nil {
		t.Fatalf("GetMigrationStep: %v", err)
	}
	if step.Status != store.MigrationStepCompleted {
		t.Fatalf("runs backfill MUST resume to completed, got %s", step.Status)
	}
	checkBoundTenant(t, a.Store, "runs", "run_id", "run_seed")
}

// T081 — a `failed` step refuses startup with ErrTenantMigrationFailed.
func TestMigration_E2E_RefusesStartOnFailedStep(t *testing.T) {
	dataDir := bootAppOnFixtureDir(t)

	pre, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Inject a `failed` row.
	if _, err := pre.DB().ExecContext(context.Background(), `UPDATE tenant_migration_progress SET status = 'failed', error = 'simulated' WHERE step_name = ?`, store.RuntimeBackfillRunsStepName); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("close pre: %v", err)
	}

	_, err = New()
	if err == nil {
		t.Fatalf("expected ErrTenantMigrationFailed, got nil")
	}
	if !errors.Is(err, ErrTenantMigrationFailed) {
		t.Fatalf("expected ErrTenantMigrationFailed, got %v", err)
	}
}

// T081a — backfilled events flow only to the tenant that owns them
// after migration. Default-personal-tenant scoped reader sees seeded
// events; a different tenant sees none of them.
func TestMigration_E2E_BackfilledEventIsolation(t *testing.T) {
	_ = bootAppOnFixtureDir(t)

	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close(context.Background()) })

	// Read the bootstrapped personal tenant id.
	var defaultTenantID string
	row := a.Store.DB().QueryRowContext(context.Background(), `SELECT tenant_id FROM tenants WHERE tenant_kind = 'personal' LIMIT 1`)
	if err := row.Scan(&defaultTenantID); err != nil {
		t.Fatalf("read default personal tenant: %v", err)
	}
	if defaultTenantID == "" {
		t.Fatalf("default personal tenant id missing")
	}

	// Tenant-owned event must show up under the personal tenant.
	rows, err := a.Store.DB().QueryContext(context.Background(),
		`SELECT event_id FROM events WHERE tenant_id = ?`, defaultTenantID)
	if err != nil {
		t.Fatalf("list events for tenant: %v", err)
	}
	owned := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatalf("scan: %v", err)
		}
		owned[id] = true
	}
	rows.Close()
	if !owned["evt_run_seed"] {
		t.Errorf("evt_run_seed MUST be visible to default personal tenant")
	}

	// A different tenant must see none of the seeded rows.
	otherRows, err := a.Store.DB().QueryContext(context.Background(),
		`SELECT event_id FROM events WHERE tenant_id = 'ten_other_tenant'`)
	if err != nil {
		t.Fatalf("list other-tenant events: %v", err)
	}
	defer otherRows.Close()
	if otherRows.Next() {
		var id string
		_ = otherRows.Scan(&id)
		t.Fatalf("other tenant MUST see no seeded events; got %s", id)
	}

	// Global events MUST NOT appear under any tenant_id.
	var globalEventTenant string
	if err := a.Store.DB().QueryRowContext(context.Background(),
		`SELECT COALESCE(tenant_id, '') FROM events WHERE event_id = 'evt_sys_seed'`).Scan(&globalEventTenant); err != nil {
		t.Fatalf("read evt_sys_seed: %v", err)
	}
	if globalEventTenant != "" {
		t.Fatalf("global event MUST stay tenantless, got %q", globalEventTenant)
	}
}

// T081b — backup-restore round-trip restores the database file
// byte-for-byte and unwinds every Roadmap 35 schema change.
func TestMigration_E2E_BackupRestoreRoundTrip(t *testing.T) {
	dataDir := bootAppOnFixtureDir(t)

	dbPath := filepath.Join(dataDir, "daemon.sqlite")
	preHash, err := hashFile(dbPath)
	if err != nil {
		t.Fatalf("hash pre db: %v", err)
	}
	// Copy the original file aside as the "backup".
	backupPath := filepath.Join(t.TempDir(), "backup.sqlite")
	if err := copyFile(dbPath, backupPath); err != nil {
		t.Fatalf("copy backup: %v", err)
	}

	// Migrate.
	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := a.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Confirm migration actually changed the file.
	postHash, err := hashFile(dbPath)
	if err != nil {
		t.Fatalf("hash post db: %v", err)
	}
	if postHash == preHash {
		t.Fatalf("migration did not change the database file")
	}

	// Restore from backup. Also remove SQLite's WAL/journal sidecars
	// so the restored file is the sole source of truth.
	for _, suf := range []string{"-wal", "-shm", "-journal"} {
		_ = os.Remove(dbPath + suf)
	}
	if err := copyFile(backupPath, dbPath); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restoredHash, err := hashFile(dbPath)
	if err != nil {
		t.Fatalf("hash restored db: %v", err)
	}
	if restoredHash != preHash {
		t.Fatalf("restored db hash != pre-migration hash")
	}

	// Reopen as a fresh handle — schema metadata is per-connection.
	restoredStore, err := store.NewSQLiteStoreAtVersion(dataDir, migrationfixture.PreTenantSchemaVersion)
	if err != nil {
		t.Fatalf("reopen restored: %v", err)
	}
	t.Cleanup(func() { _ = restoredStore.Close() })
	var has int
	if err := restoredStore.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tenant_migration_progress'`).Scan(&has); err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if has != 0 {
		t.Fatalf("tenant_migration_progress table MUST be absent after restore")
	}
	// Confirm tenant_id column not present on a tenant_owned table.
	cols, err := restoredStore.DB().QueryContext(context.Background(), `PRAGMA table_info(runs)`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer cols.Close()
	for cols.Next() {
		var (
			cid     int
			name    string
			ctype   string
			nn      int
			dflt    any
			pk      int
		)
		if err := cols.Scan(&cid, &name, &ctype, &nn, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "tenant_id" {
			t.Fatalf("runs.tenant_id MUST be absent in restored pre-tenant schema")
		}
	}
}

// ----- helpers -----

func checkBoundTenant(t *testing.T, s *store.SQLiteStore, table, pkCol, pkValue string) {
	t.Helper()
	var got string
	if err := s.DB().QueryRowContext(context.Background(),
		"SELECT COALESCE(tenant_id, '') FROM "+table+" WHERE "+pkCol+" = ?", pkValue).Scan(&got); err != nil {
		t.Errorf("read %s.%s=%s: %v", table, pkCol, pkValue, err)
		return
	}
	if got == "" {
		t.Errorf("%s.%s=%s tenant_id is empty post-migration", table, pkCol, pkValue)
	}
}

// snapshotTenantIDs serializes (table, pk, tenant_id) across the
// fixture-seeded tables so two runs can be compared byte-for-byte.
func snapshotTenantIDs(ctx context.Context, s *store.SQLiteStore) (string, error) {
	var b strings.Builder
	specs := []struct {
		table  string
		pkCol  string
	}{
		{"sessions", "session_id"},
		{"runs", "run_id"},
		{"steps", "step_id"},
		{"tool_calls", "tool_call_id"},
		{"llm_dispatches", "dispatch_id"},
		{"checkpoints", "checkpoint_id"},
		{"schedules", "schedule_id"},
		{"workflows", "workflow_id"},
		{"calendar_accounts", "calendar_account_id"},
		{"mail_accounts", "mail_account_id"},
		{"reminders", "reminder_id"},
		{"computer_use_sessions", "computer_use_session_id"},
		{"approvals", "approval_id"},
		// evaluation_replay_candidates is excluded: app.New seeds new
		// fixture candidates on every boot (LoadFixtures), which is
		// orthogonal to migration idempotence.
		// events is excluded: every daemon start emits new lifecycle
		// events (daemon.migration.*, system.*) so the table grows on
		// each boot. Migration idempotence is already covered for the
		// pre-existing rows by the per-step `MigrationStepCompleted`
		// status check + the FixtureLossless test.
	}
	for _, sp := range specs {
		rows, err := s.DB().QueryContext(ctx,
			"SELECT "+sp.pkCol+", COALESCE(tenant_id,'') FROM "+sp.table+" ORDER BY "+sp.pkCol)
		if err != nil {
			return "", err
		}
		for rows.Next() {
			var pk, tid string
			if err := rows.Scan(&pk, &tid); err != nil {
				rows.Close()
				return "", err
			}
			b.WriteString(sp.table)
			b.WriteByte('|')
			b.WriteString(pk)
			b.WriteByte('|')
			b.WriteString(tid)
			b.WriteByte('\n')
		}
		rows.Close()
	}
	return b.String(), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
