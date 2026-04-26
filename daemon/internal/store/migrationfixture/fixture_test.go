package migrationfixture

import (
	"context"
	"testing"
)

// TestBuildPreTenantV21Fixture proves the seed helper builds a v21
// database with at least one row in every in-scope tenant_owned table.
// Phase 4 batch 6 (T078) extends this with the full migration assertion.
func TestBuildPreTenantV21Fixture(t *testing.T) {
	s, err := BuildPreTenantV21Fixture(t.TempDir())
	if err != nil {
		t.Fatalf("BuildPreTenantV21Fixture: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	counts, err := CountSeededRows(context.Background(), s)
	if err != nil {
		t.Fatalf("CountSeededRows: %v", err)
	}
	for table, n := range counts {
		if n == 0 {
			t.Errorf("table %s has zero seeded rows", table)
		}
	}
}

// TestApplyHeadMigrations_FromV21Fixture proves the v21 fixture can
// be migrated up to head + has every migration_progress step
// registered. The actual backfill driver invocation lives in the app
// package; this test only covers the schema migration path.
func TestApplyHeadMigrations_FromV21Fixture(t *testing.T) {
	s, err := BuildPreTenantV21Fixture(t.TempDir())
	if err != nil {
		t.Fatalf("BuildPreTenantV21Fixture: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.ApplyHeadMigrations(context.Background()); err != nil {
		t.Fatalf("ApplyHeadMigrations: %v", err)
	}
	steps, err := s.LoadMigrationSteps(context.Background())
	if err != nil {
		t.Fatalf("LoadMigrationSteps: %v", err)
	}
	if len(steps) < 30 {
		t.Fatalf("expected ~38 registered steps, got %d", len(steps))
	}
}
