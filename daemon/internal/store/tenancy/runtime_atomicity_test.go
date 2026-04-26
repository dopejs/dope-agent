package tenancy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

// Roadmap 35 review remediation — Finding #2.
//
// The earlier Pass A pattern composed `legacy Upsert` + `BindRowTenant`
// in two SQL statements. That pattern leaked: tenant B writing the
// same primary key as tenant A's row would clobber A's columns BEFORE
// the bind helper noticed the mismatch. These tests prove the new
// atomic INSERT path refuses cross-tenant writes WITHOUT modifying
// any of A's columns.

func TestRuntimePassA_CrossTenantWriteLeavesOriginalRowIntact(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bus := events.NewBus()
	em := audit.NewEmitter(bus, nil)
	r := tenancy.NewRuntime(s, em)

	ctxA := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	ctxB := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_b"})
	now := time.Now().UTC()

	originalGoal := "tenant A's protected goal"
	if err := r.UpsertRunForTenant(ctxA, runtime.Run{
		RunID: "run_collision", Entrypoint: "test", Status: runtime.RunStatusQueued,
		Goal: originalGoal, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert A: %v", err)
	}

	// B attempts to UPSERT the same run id with completely different
	// payload. Pre-fix: this would have clobbered A's `goal` column
	// before the bind step detected the mismatch. Post-fix: the
	// statement is rejected atomically — A's row stays intact.
	err = r.UpsertRunForTenant(ctxB, runtime.Run{
		RunID: "run_collision", Entrypoint: "EVIL", Status: runtime.RunStatusFailed,
		Goal: "TENANT B OVERWROTE THIS", CreatedAt: now, UpdatedAt: now,
	})
	if !errors.Is(err, tenancy.ErrCrossTenantWrite) {
		t.Fatalf("expected ErrCrossTenantWrite, got %v", err)
	}

	// Read A's row back via tenant A's accessor and assert nothing
	// changed.
	got, ok, err := r.GetRunForTenant(ctxA, "run_collision")
	if err != nil || !ok {
		t.Fatalf("tenant A lost ownership of its own run: ok=%v err=%v", ok, err)
	}
	if got.Goal != originalGoal {
		t.Fatalf("tenant A's goal was clobbered: got %q, want %q", got.Goal, originalGoal)
	}
	if got.Entrypoint != "test" {
		t.Fatalf("tenant A's entrypoint was clobbered: got %q, want %q", got.Entrypoint, "test")
	}
	if got.Status != runtime.RunStatusQueued {
		t.Fatalf("tenant A's status was clobbered: got %q, want %q", got.Status, runtime.RunStatusQueued)
	}
}

func TestRuntimePassA_SameTenantUpsertStillUpdates(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	r := tenancy.NewRuntime(s, audit.NewEmitter(events.NewBus(), nil))

	ctxA := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	now := time.Now().UTC()

	if err := r.UpsertRunForTenant(ctxA, runtime.Run{
		RunID: "run_iter", Entrypoint: "test", Status: runtime.RunStatusQueued,
		Goal: "v1", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("v1: %v", err)
	}
	if err := r.UpsertRunForTenant(ctxA, runtime.Run{
		RunID: "run_iter", Entrypoint: "test", Status: runtime.RunStatusRunning,
		Goal: "v2", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("v2 (same tenant must succeed): %v", err)
	}

	got, ok, err := r.GetRunForTenant(ctxA, "run_iter")
	if err != nil || !ok {
		t.Fatalf("read back: ok=%v err=%v", ok, err)
	}
	if got.Goal != "v2" || got.Status != runtime.RunStatusRunning {
		t.Fatalf("same-tenant UPSERT did not advance: got goal=%q status=%q", got.Goal, got.Status)
	}
}
