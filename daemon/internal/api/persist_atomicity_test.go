package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Roadmap 35 review remediation — Finding #2 + Finding #1 end-to-end.
//
// `persistRun` is the production write path used by handleRuns,
// schedule_workflow_launcher, im/loop, evaluation/runtime_recorder,
// and checkpoints/manager. Pre-fix: even after the tenancy.* helpers
// landed, the production handler still called `sqliteStore.UpsertRun`
// directly, so cross-tenant ID collision would clobber tenant A's
// columns. Post-fix: when ctx carries tenant context, `persistRun`
// routes through the atomic UpsertRunForTenantSafe path and surfaces
// `ErrTenantOwnershipDenied` instead of mutating shared state.

func TestPersistRun_CrossTenantCollisionIsAtomicallyRefused(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	now := time.Now().UTC()

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	ctxB := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_b"})

	original := runtime.Run{
		RunID: "run_collide", Entrypoint: "alpha", Status: runtime.RunStatusQueued,
		Goal: "owned by A", CreatedAt: now, UpdatedAt: now,
	}
	if err := persistRun(ctxA, s, original); err != nil {
		t.Fatalf("persistRun A: %v", err)
	}

	collision := runtime.Run{
		RunID: "run_collide", Entrypoint: "EVIL", Status: runtime.RunStatusFailed,
		Goal: "B clobbered this", CreatedAt: now, UpdatedAt: now,
	}
	err = persistRun(ctxB, s, collision)
	if !errors.Is(err, ErrTenantOwnershipDenied) {
		t.Fatalf("cross-tenant persistRun must return ErrTenantOwnershipDenied, got %v", err)
	}

	// Verify A's row is untouched. Use the legacy ListRuns path to
	// avoid coupling this assertion to the tenancy helper under
	// test — we are explicitly checking the raw row state.
	rows, err := s.ListRunsAllTenantsForTest(context.Background())
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	var got runtime.Run
	for _, r := range rows {
		if r.RunID == "run_collide" {
			got = r
		}
	}
	if got.RunID == "" {
		t.Fatalf("row vanished after refused write")
	}
	if got.Goal != "owned by A" {
		t.Fatalf("tenant A's goal was clobbered by refused B write: got %q", got.Goal)
	}
	if got.Entrypoint != "alpha" {
		t.Fatalf("tenant A's entrypoint was clobbered by refused B write: got %q", got.Entrypoint)
	}
	if got.Status != runtime.RunStatusQueued {
		t.Fatalf("tenant A's status was clobbered by refused B write: got %q", got.Status)
	}
}
