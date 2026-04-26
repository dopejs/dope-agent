package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
)

// Roadmap 35 (US2 / T067) — backfill engine regression tests.
//
// Verifies the generic engine guarantees independent of any specific
// per-domain step:
//   - top-level (no parent): rows fanned out to defaultTenantID
//   - parent-derived: child rows pick up parent's tenant_id
//   - orphan rule: child whose parent FK is missing OR parent.tenant_id
//     is NULL surfaces an OrphanError, the chunk is rolled back, no
//     row is half-bound
//   - resume: a chunk-mid crash leaves last_processed_key set, the
//     next invocation continues without skipping or duplicating work
//   - idempotence: re-running a `completed` step is a no-op

func newBackfillTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seedNullTenantSession bypasses the tenant-aware helpers and writes
// a sessions row with NULL tenant_id (the pre-roadmap-35 state). The
// helper is test-only and does not exist in the production code path.
func seedNullTenantRow(t *testing.T, s *SQLiteStore, table, pkColumn, pkValue string, extra map[string]any) {
	t.Helper()
	cols := []string{pkColumn}
	placeholders := []string{"?"}
	args := []any{pkValue}
	for k, v := range extra {
		cols = append(cols, k)
		placeholders = append(placeholders, "?")
		args = append(args, v)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, joinCommas(cols), joinCommas(placeholders))
	if _, err := s.db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("seed %s: %v", table, err)
	}
}

func joinCommas(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

func tenantOf(t *testing.T, s *SQLiteStore, table, pkColumn, pkValue string) string {
	t.Helper()
	var v sql.NullString
	q := fmt.Sprintf("SELECT tenant_id FROM %s WHERE %s = ?", table, pkColumn)
	if err := s.db.QueryRowContext(context.Background(), q, pkValue).Scan(&v); err != nil {
		t.Fatalf("read %s.%s tenant_id: %v", table, pkColumn, err)
	}
	return v.String
}

func TestRunBackfill_TopLevel_BindsDefaultTenant(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed three NULL-tenant sessions. Sessions require kind/status/channel/peer_id/routing_key/generation/timestamps.
	for i, id := range []string{"sess_aaa", "sess_bbb", "sess_ccc"} {
		seedNullTenantRow(t, s, "sessions", "session_id", id, map[string]any{
			"kind": "chat", "status": "active", "channel": "test",
			"peer_id":        "peer_" + id,
			"routing_key":    fmt.Sprintf("rk_%s_%d", id, i),
			"generation":     1,
			"created_at":     "2025-01-01T00:00:00Z",
			"updated_at":     "2025-01-01T00:00:00Z",
			"last_active_at": "2025-01-01T00:00:00Z",
		})
	}
	if err := s.RegisterMigrationStep(ctx, RuntimeBackfillSessionsStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	step := BackfillStep{
		Name:      RuntimeBackfillSessionsStepName,
		Table:     "sessions",
		PKColumn:  "session_id",
		ChunkSize: 50,
	}
	if err := s.RunBackfill(ctx, step, "ten_default"); err != nil {
		t.Fatalf("RunBackfill: %v", err)
	}
	for _, id := range []string{"sess_aaa", "sess_bbb", "sess_ccc"} {
		if got := tenantOf(t, s, "sessions", "session_id", id); got != "ten_default" {
			t.Fatalf("session %s tenant_id=%q, want ten_default", id, got)
		}
	}
	progress, err := s.GetMigrationStep(ctx, RuntimeBackfillSessionsStepName)
	if err != nil {
		t.Fatalf("GetMigrationStep: %v", err)
	}
	if progress.Status != MigrationStepCompleted {
		t.Fatalf("expected step completed, got %q", progress.Status)
	}
}

func TestRunBackfill_ParentDerived_InheritsParentTenant(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed parent runs with explicit tenant_id, then child steps with NULL tenant.
	for _, run := range []struct{ id, tenant string }{
		{"run_a", "ten_aaaa"},
		{"run_b", "ten_bbbb"},
	} {
		seedNullTenantRow(t, s, "runs", "run_id", run.id, map[string]any{
			"entrypoint": "t", "status": "queued", "goal": "g", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
		})
		if _, err := s.db.ExecContext(ctx, `UPDATE runs SET tenant_id = ? WHERE run_id = ?`, run.tenant, run.id); err != nil {
			t.Fatalf("bind parent tenant: %v", err)
		}
	}
	for _, step := range []struct{ id, runID string }{
		{"step_a1", "run_a"},
		{"step_a2", "run_a"},
		{"step_b1", "run_b"},
	} {
		seedNullTenantRow(t, s, "steps", "step_id", step.id, map[string]any{
			"run_id": step.runID, "title": "t", "kind": "model", "status": "queued",
			"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
		})
	}
	if err := s.RegisterMigrationStep(ctx, RuntimeBackfillStepsStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	bs := BackfillStep{
		Name:           RuntimeBackfillStepsStepName,
		Table:          "steps",
		PKColumn:       "step_id",
		ParentTable:    "runs",
		ParentFKColumn: "run_id",
		ParentTablePK:  "run_id",
		ChunkSize:      50,
	}
	if err := s.RunBackfill(ctx, bs, "ten_irrelevant"); err != nil {
		t.Fatalf("RunBackfill: %v", err)
	}
	for _, step := range []struct{ id, want string }{
		{"step_a1", "ten_aaaa"},
		{"step_a2", "ten_aaaa"},
		{"step_b1", "ten_bbbb"},
	} {
		if got := tenantOf(t, s, "steps", "step_id", step.id); got != step.want {
			t.Fatalf("step %s tenant_id=%q, want %q", step.id, got, step.want)
		}
	}
}

func TestRunBackfill_OrphanParent_RefusesAndPreservesChunk(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Parent run exists but has NULL tenant_id (parent backfill not run yet).
	seedNullTenantRow(t, s, "runs", "run_id", "run_orphan", map[string]any{
		"entrypoint": "t", "status": "queued", "goal": "g", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})
	seedNullTenantRow(t, s, "steps", "step_id", "step_orphan", map[string]any{
		"run_id": "run_orphan", "title": "t", "kind": "model", "status": "queued",
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})
	if err := s.RegisterMigrationStep(ctx, RuntimeBackfillStepsStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	bs := BackfillStep{
		Name:           RuntimeBackfillStepsStepName,
		Table:          "steps",
		PKColumn:       "step_id",
		ParentTable:    "runs",
		ParentFKColumn: "run_id",
		ParentTablePK:  "run_id",
		ChunkSize:      50,
	}
	err := s.RunBackfill(ctx, bs, "ten_default")
	var orphan *OrphanError
	if !errors.As(err, &orphan) {
		t.Fatalf("expected OrphanError, got %v", err)
	}
	if orphan.Table != "steps" || orphan.ParentTable != "runs" || orphan.ParentValue != "run_orphan" {
		t.Fatalf("unexpected orphan payload: %+v", orphan)
	}
	// Child row MUST NOT have been half-bound.
	if got := tenantOf(t, s, "steps", "step_id", "step_orphan"); got != "" {
		t.Fatalf("step_orphan tenant_id=%q, want empty (chunk rolled back)", got)
	}
}

func TestRunBackfill_Idempotent_OnCompletedStep(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	if err := s.RegisterMigrationStep(ctx, RuntimeBackfillRunsStepName); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.CompleteMigrationStep(ctx, RuntimeBackfillRunsStepName); err != nil {
		t.Fatalf("complete: %v", err)
	}
	// Add a NULL row AFTER completion — engine MUST NOT touch it.
	seedNullTenantRow(t, s, "runs", "run_id", "run_post", map[string]any{
		"entrypoint": "t", "status": "queued", "goal": "g", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})

	bs := BackfillStep{
		Name: RuntimeBackfillRunsStepName, Table: "runs", PKColumn: "run_id", ChunkSize: 50,
	}
	if err := s.RunBackfill(ctx, bs, "ten_default"); err != nil {
		t.Fatalf("RunBackfill (completed): %v", err)
	}
	if got := tenantOf(t, s, "runs", "run_id", "run_post"); got != "" {
		t.Fatalf("completed step MUST be a no-op; got tenant_id=%q", got)
	}
}

func TestRunBackfill_ResumeFromCursor_NoSkipNoDuplicate(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed 7 NULL-tenant runs in lexical order.
	for i := 0; i < 7; i++ {
		seedNullTenantRow(t, s, "runs", "run_id", fmt.Sprintf("run_%02d", i), map[string]any{
			"entrypoint": "t", "status": "queued", "goal": "g", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
		})
	}
	if err := s.RegisterMigrationStep(ctx, RuntimeBackfillRunsStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Simulate a partial run by seeding the progress row directly.
	if _, err := s.db.ExecContext(ctx, `UPDATE tenant_migration_progress SET status = ?, last_processed_key = ? WHERE step_name = ?`,
		string(MigrationStepRunning), "run_03", RuntimeBackfillRunsStepName); err != nil {
		t.Fatalf("seed running cursor: %v", err)
	}
	// Pre-bind rows 0..3 as if the prior chunk had succeeded.
	for i := 0; i <= 3; i++ {
		if _, err := s.db.ExecContext(ctx, `UPDATE runs SET tenant_id = ? WHERE run_id = ?`,
			"ten_default", fmt.Sprintf("run_%02d", i)); err != nil {
			t.Fatalf("pre-bind: %v", err)
		}
	}

	bs := BackfillStep{
		Name: RuntimeBackfillRunsStepName, Table: "runs", PKColumn: "run_id", ChunkSize: 50,
	}
	if err := s.RunBackfill(ctx, bs, "ten_default"); err != nil {
		t.Fatalf("resume RunBackfill: %v", err)
	}
	for i := 0; i < 7; i++ {
		if got := tenantOf(t, s, "runs", "run_id", fmt.Sprintf("run_%02d", i)); got != "ten_default" {
			t.Fatalf("run_%02d tenant_id=%q, want ten_default", i, got)
		}
	}
}
