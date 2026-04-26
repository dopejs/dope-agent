package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Roadmap 35 (US3 / T089, post-review C5) — mail_artifacts list
// latency regression on the SAME database before and after the
// migration. Runs the actual backfill + enforcement on the seeded
// v21 data (rather than comparing two clean independent stores).
func TestPerfMailArtifactsListLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("perf test skipped under -short")
	}

	ctx := context.Background()
	dir := t.TempDir()
	const opID = "op_perf_001"

	s, err := NewSQLiteStoreAtVersion(dir, 21)
	if err != nil {
		t.Fatalf("NewSQLiteStoreAtVersion v21: %v", err)
	}
	defer s.Close()
	seedMailArtifactsBulkV21(t, s, perfRowCount, opID)

	preP95 := measureP95(t, s.DB(), perfIterations,
		`SELECT artifact_id FROM mail_artifacts WHERE operation_id = ? ORDER BY created_at ASC, artifact_id ASC LIMIT 50`,
		opID)

	if err := s.ApplyHeadMigrations(ctx); err != nil {
		t.Fatalf("ApplyHeadMigrations: %v", err)
	}
	if err := s.RunSimpleBulkBackfill(ctx, MailArtifactsBackfillStepName, "mail_artifacts", perfTenantA); err != nil {
		t.Fatalf("RunSimpleBulkBackfill mail_artifacts: %v", err)
	}
	// The mail_artifacts spec lives under ExtendedEnforcementSpecs;
	// pick it out so we exercise the same EnforcementSpec the daemon
	// boot path uses.
	var spec EnforcementSpec
	for _, sp := range ExtendedEnforcementSpecs() {
		if sp.Table == "mail_artifacts" {
			spec = sp
			break
		}
	}
	if spec.Table == "" {
		t.Fatalf("mail_artifacts spec missing from ExtendedEnforcementSpecs()")
	}
	if err := s.EnforceTenantNotNullSpec(ctx, spec); err != nil {
		t.Fatalf("EnforceTenantNotNullSpec mail_artifacts: %v", err)
	}

	postP95 := measureP95(t, s.DB(), perfIterations,
		`SELECT artifact_id FROM mail_artifacts WHERE tenant_id = ? AND operation_id = ? ORDER BY created_at ASC, artifact_id ASC LIMIT 50`,
		perfTenantA, opID)

	assertNoRegression(t, "mail_artifacts list (same-DB migration)", preP95, postP95)
}

func seedMailArtifactsBulkV21(t *testing.T, s *SQLiteStore, n int, opID string) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO mail_artifacts (artifact_id, operation_id, integration_id, environment_scope, kind, created_at, document_json) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("prepare: %v", err)
	}
	base := time.Now().UTC()
	for i := 0; i < n; i++ {
		// Half the rows belong to the target operation; the rest belong
		// to an unrelated op so the operation filter is meaningful.
		op := opID
		if i%2 == 1 {
			op = "op_perf_other"
		}
		ts := base.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano)
		if _, err := stmt.ExecContext(ctx,
			fmt.Sprintf("art_perf_%06d", i), op, "int_perf", "test", "thread", ts, "{}"); err != nil {
			_ = tx.Rollback()
			t.Fatalf("insert: %v", err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
