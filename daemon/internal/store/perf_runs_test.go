package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Roadmap 35 (US3 / T087, post-review C5) — runs list latency
// regression on the SAME database before and after the migration.
//
// The earlier shape compared two independent stores (clean v21 vs.
// clean head); that measured "two empty-history clones" rather than
// the migration's actual cost. The rewrite below:
//
//  1. Opens v21, seeds N rows, measures pre p95 of the legacy list.
//  2. Runs ApplyHeadMigrations → backfill → enforce-not-null on the
//     SAME database, using the same migration entry points the daemon
//     boot path invokes (so a regression in the real migration is
//     observable here).
//  3. Re-measures p95 on the migrated DB using the tenant-filtered
//     read path.
//
// SC-008: post p95 ≤ 1.2× pre p95.
//
// Skipped under `-short` for fast local iteration; CI runs without
// `-short` and exercises the full N=10000 seed.
func TestPerfRunsListLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("perf test skipped under -short")
	}

	ctx := context.Background()
	dir := t.TempDir()

	// PRE: open at v21, seed N rows with NO tenant_id column on the
	// table, measure p95 of the legacy list path.
	s, err := NewSQLiteStoreAtVersion(dir, 21)
	if err != nil {
		t.Fatalf("NewSQLiteStoreAtVersion v21: %v", err)
	}
	defer s.Close()
	seedRunsBulkV21(t, s, perfRowCount)

	preP95 := measureP95(t, s.DB(), perfIterations,
		`SELECT run_id FROM runs ORDER BY created_at DESC LIMIT 50`)

	// MIGRATE: bring the SAME database to head and run backfill +
	// enforcement for the runs table. This is the actual migration
	// entry point the daemon boot uses; any regression here is the
	// regression we care about.
	if err := s.ApplyHeadMigrations(ctx); err != nil {
		t.Fatalf("ApplyHeadMigrations: %v", err)
	}
	// Backfill the runs table to a single tenant (matches the
	// single-operator default-personal-tenant binding pattern).
	bs := BackfillStep{
		Name:      RuntimeBackfillRunsStepName,
		Table:     "runs",
		PKColumn:  "run_id",
		ChunkSize: 1000,
	}
	if err := s.RunBackfill(ctx, bs, perfTenantA); err != nil {
		t.Fatalf("RunBackfill runs: %v", err)
	}
	// Enforce NOT NULL + per-tenant index. Goes through the same
	// EnforcementSpec path the daemon runner uses.
	var spec EnforcementSpec
	for _, sp := range RuntimeEnforcementSpecs() {
		if sp.Table == "runs" {
			spec = sp
			break
		}
	}
	if spec.Table == "" {
		t.Fatalf("runs spec missing from RuntimeEnforcementSpecs()")
	}
	if err := s.EnforceTenantNotNullSpec(ctx, spec); err != nil {
		t.Fatalf("EnforceTenantNotNullSpec runs: %v", err)
	}

	// POST: same DB, tenant-filtered read.
	postP95 := measureP95(t, s.DB(), perfIterations,
		`SELECT run_id FROM runs WHERE tenant_id = ? ORDER BY created_at DESC, run_id DESC LIMIT 50`,
		perfTenantA)

	assertNoRegression(t, "runs list (same-DB migration)", preP95, postP95)
}

func seedRunsBulkV21(t *testing.T, s *SQLiteStore, n int) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO runs (run_id, entrypoint, status, goal, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("prepare: %v", err)
	}
	base := time.Now().UTC()
	for i := 0; i < n; i++ {
		ts := base.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		if _, err := stmt.ExecContext(ctx, fmt.Sprintf("run_perf_%06d", i), "test", "queued", "perf", ts, ts); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
