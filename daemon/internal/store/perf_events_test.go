package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Roadmap 35 (US3 / T088, post-review C5) — events list latency
// regression on the SAME database before and after the migration.
//
// Mirrors perf_runs_test.go: open at v21, seed N rows, measure pre,
// run the actual migration (ApplyHeadMigrations + RunEventsBackfill +
// EnforceEventsPartialIndexes), re-measure on the migrated DB.
func TestPerfEventsListLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("perf test skipped under -short")
	}

	ctx := context.Background()
	dir := t.TempDir()

	s, err := NewSQLiteStoreAtVersion(dir, 21)
	if err != nil {
		t.Fatalf("NewSQLiteStoreAtVersion v21: %v", err)
	}
	defer s.Close()
	seedEventsBulkV21(t, s, perfRowCount)

	preP95 := measureP95(t, s.DB(), perfIterations,
		`SELECT event_id FROM events ORDER BY occurred_at DESC LIMIT 50`)

	if err := s.ApplyHeadMigrations(ctx); err != nil {
		t.Fatalf("ApplyHeadMigrations: %v", err)
	}
	if err := s.RunEventsBackfill(ctx, EventsBackfillStepName, perfTenantA); err != nil {
		t.Fatalf("RunEventsBackfill: %v", err)
	}
	if err := s.EnforceEventsPartialIndexes(ctx, EventsEnforceCheckStepName); err != nil {
		t.Fatalf("EnforceEventsPartialIndexes: %v", err)
	}

	postP95 := measureP95(t, s.DB(), perfIterations,
		`SELECT event_id FROM events WHERE tenant_id = ? ORDER BY occurred_at DESC LIMIT 50`,
		perfTenantA)

	assertNoRegression(t, "events list (same-DB migration)", preP95, postP95)
}

func seedEventsBulkV21(t *testing.T, s *SQLiteStore, n int) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// runs FK is required by events backfill's run-id linkage. Seed
	// matching runs so RunEventsBackfill's priority cascade can resolve
	// each event back to a tenant in the same DB.
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO runs (run_id, entrypoint, status, goal, created_at, updated_at)
		SELECT 'run_' || printf('%06d', value), 'test', 'queued', 'g', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z'
		FROM (WITH RECURSIVE seq(value) AS (SELECT 0 UNION ALL SELECT value+1 FROM seq WHERE value+1 < ?) SELECT value FROM seq)`, n); err != nil {
		_ = tx.Rollback()
		t.Fatalf("seed runs FK: %v", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO events (event_id, category, name, occurred_at, run_id, resource_kind, resource_id, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("prepare: %v", err)
	}
	base := time.Now().UTC()
	for i := 0; i < n; i++ {
		ts := base.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		runID := fmt.Sprintf("run_%06d", i)
		if _, err := stmt.ExecContext(ctx,
			fmt.Sprintf("ev_perf_%06d", i), "runtime", "run-created", ts, runID, "run",
			runID, "{}"); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
