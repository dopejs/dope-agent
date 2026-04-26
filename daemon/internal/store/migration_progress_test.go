package store

import (
	"context"
	"errors"
	"testing"
)

func newTestStoreForMigrationProgress(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestMigrationProgressLifecycle(t *testing.T) {
	s := newTestStoreForMigrationProgress(t)
	ctx := context.Background()

	if err := s.RegisterMigrationStep(ctx, "tenant_migration:backfill:runs"); err != nil {
		t.Fatalf("RegisterMigrationStep: %v", err)
	}

	step, err := s.GetMigrationStep(ctx, "tenant_migration:backfill:runs")
	if err != nil {
		t.Fatalf("GetMigrationStep: %v", err)
	}
	if step.Status != MigrationStepPending {
		t.Fatalf("expected pending, got %q", step.Status)
	}

	resume, err := s.BeginMigrationStep(ctx, "tenant_migration:backfill:runs")
	if err != nil {
		t.Fatalf("BeginMigrationStep: %v", err)
	}
	if resume {
		t.Fatalf("first BeginMigrationStep should not signal resume")
	}

	if err := s.RecordMigrationChunk(ctx, "tenant_migration:backfill:runs", "run_500"); err != nil {
		t.Fatalf("RecordMigrationChunk: %v", err)
	}

	// A second BeginMigrationStep on a `running` step should signal resume
	// without overwriting the cursor.
	resume, err = s.BeginMigrationStep(ctx, "tenant_migration:backfill:runs")
	if err != nil {
		t.Fatalf("second BeginMigrationStep: %v", err)
	}
	if !resume {
		t.Fatalf("expected resume=true while step is still running")
	}

	step, _ = s.GetMigrationStep(ctx, "tenant_migration:backfill:runs")
	if step.LastProcessedKey != "run_500" {
		t.Fatalf("cursor lost across resume: %q", step.LastProcessedKey)
	}

	if err := s.CompleteMigrationStep(ctx, "tenant_migration:backfill:runs"); err != nil {
		t.Fatalf("CompleteMigrationStep: %v", err)
	}

	step, _ = s.GetMigrationStep(ctx, "tenant_migration:backfill:runs")
	if step.Status != MigrationStepCompleted {
		t.Fatalf("expected completed, got %q", step.Status)
	}
	if step.CompletedAt == nil {
		t.Fatalf("expected completed_at to be set")
	}

	// Begin on a completed step is a no-op (resume=false, no error) and the
	// caller must skip the work.
	resume, err = s.BeginMigrationStep(ctx, "tenant_migration:backfill:runs")
	if err != nil {
		t.Fatalf("BeginMigrationStep on completed: %v", err)
	}
	if resume {
		t.Fatalf("BeginMigrationStep on completed should not signal resume")
	}
}

func TestMigrationProgressFailedRefusesBegin(t *testing.T) {
	s := newTestStoreForMigrationProgress(t)
	ctx := context.Background()

	const name = "tenant_migration:backfill:integrations"
	if err := s.RegisterMigrationStep(ctx, name); err != nil {
		t.Fatalf("RegisterMigrationStep: %v", err)
	}
	if _, err := s.BeginMigrationStep(ctx, name); err != nil {
		t.Fatalf("BeginMigrationStep: %v", err)
	}
	if err := s.FailMigrationStep(ctx, name, errors.New("disk full")); err != nil {
		t.Fatalf("FailMigrationStep: %v", err)
	}

	if _, err := s.BeginMigrationStep(ctx, name); !errors.Is(err, ErrMigrationStepFailed) {
		t.Fatalf("expected ErrMigrationStepFailed, got %v", err)
	}

	if err := s.ResetMigrationStep(ctx, name); err != nil {
		t.Fatalf("ResetMigrationStep: %v", err)
	}

	step, _ := s.GetMigrationStep(ctx, name)
	if step.Status != MigrationStepPending {
		t.Fatalf("after reset, expected pending, got %q", step.Status)
	}
	if step.Error != "" {
		t.Fatalf("after reset, error should be cleared, got %q", step.Error)
	}
}

func TestMigrationProgressIdempotentRegistration(t *testing.T) {
	s := newTestStoreForMigrationProgress(t)
	ctx := context.Background()

	const name = "tenant_migration:backfill:schedules"
	if err := s.RegisterMigrationStep(ctx, name); err != nil {
		t.Fatalf("RegisterMigrationStep: %v", err)
	}
	if _, err := s.BeginMigrationStep(ctx, name); err != nil {
		t.Fatalf("BeginMigrationStep: %v", err)
	}
	if err := s.CompleteMigrationStep(ctx, name); err != nil {
		t.Fatalf("CompleteMigrationStep: %v", err)
	}

	// Re-register MUST be a no-op and MUST NOT reset status.
	if err := s.RegisterMigrationStep(ctx, name); err != nil {
		t.Fatalf("RegisterMigrationStep (again): %v", err)
	}
	step, _ := s.GetMigrationStep(ctx, name)
	if step.Status != MigrationStepCompleted {
		t.Fatalf("re-registration overwrote completed status: %q", step.Status)
	}
}

func TestMigrationProgressIsolationByName(t *testing.T) {
	s := newTestStoreForMigrationProgress(t)
	ctx := context.Background()

	for _, name := range []string{"a", "b", "c"} {
		if err := s.RegisterMigrationStep(ctx, name); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	if _, err := s.BeginMigrationStep(ctx, "b"); err != nil {
		t.Fatalf("begin b: %v", err)
	}
	if err := s.RecordMigrationChunk(ctx, "b", "key_b"); err != nil {
		t.Fatalf("record b: %v", err)
	}

	a, _ := s.GetMigrationStep(ctx, "a")
	c, _ := s.GetMigrationStep(ctx, "c")
	if a.Status != MigrationStepPending || c.Status != MigrationStepPending {
		t.Fatalf("unrelated steps should remain pending: a=%q c=%q", a.Status, c.Status)
	}
	if a.LastProcessedKey != "" || c.LastProcessedKey != "" {
		t.Fatalf("cursor leaked across step names")
	}
}

func TestLoadMigrationStepsOrdered(t *testing.T) {
	s := newTestStoreForMigrationProgress(t)
	ctx := context.Background()

	for _, name := range []string{"zzz_step:c", "zzz_step:a", "zzz_step:b"} {
		if err := s.RegisterMigrationStep(ctx, name); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	steps, err := s.LoadMigrationSteps(ctx)
	if err != nil {
		t.Fatalf("LoadMigrationSteps: %v", err)
	}
	// NewSQLiteStore auto-registers the events backfill + enforce_check
	// rows; filter to just the rows this test added so the assertion is
	// stable as more steps are registered at startup.
	var got []MigrationStep
	for _, step := range steps {
		if step.Name == "zzz_step:a" || step.Name == "zzz_step:b" || step.Name == "zzz_step:c" {
			got = append(got, step)
		}
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 zzz_step rows, got %d (all rows: %+v)", len(got), steps)
	}
	if got[0].Name != "zzz_step:a" || got[1].Name != "zzz_step:b" || got[2].Name != "zzz_step:c" {
		t.Fatalf("steps not ordered by name: %+v", got)
	}
}
