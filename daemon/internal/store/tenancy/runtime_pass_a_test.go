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

func newRuntimeFixture(t *testing.T) (*tenancy.Runtime, *store.SQLiteStore, *events.Bus) {
	t.Helper()
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bus := events.NewBus()
	emitter := audit.NewEmitter(bus, nil)
	return tenancy.NewRuntime(s, emitter), s, bus
}

func ctxFor(tenantID, principalID string) context.Context {
	return tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: principalID,
	})
}

func TestRuntimePassA_RequiresTenantContext(t *testing.T) {
	r, _, _ := newRuntimeFixture(t)
	if _, err := r.ListRunsForTenant(context.Background()); !errors.Is(err, tenancy.ErrTenantContextRequired) {
		t.Fatalf("ListRunsForTenant without ctx = %v, want ErrTenantContextRequired", err)
	}
	if err := r.UpsertRunForTenant(context.Background(), runtime.Run{RunID: "run_a"}); !errors.Is(err, tenancy.ErrTenantContextRequired) {
		t.Fatalf("UpsertRunForTenant without ctx = %v, want ErrTenantContextRequired", err)
	}
}

func TestRuntimePassA_TwoTenantsAreIsolated(t *testing.T) {
	r, _, _ := newRuntimeFixture(t)
	now := time.Now().UTC()

	ctxA := ctxFor("ten_aaaa1111", "prn_aaaa1111")
	ctxB := ctxFor("ten_bbbb2222", "prn_bbbb2222")

	runA := runtime.Run{
		RunID:      "run_a_001",
		Entrypoint: "test",
		Status:     runtime.RunStatusQueued,
		Goal:       "tenant a goal",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	runB := runtime.Run{
		RunID:      "run_b_001",
		Entrypoint: "test",
		Status:     runtime.RunStatusQueued,
		Goal:       "tenant b goal",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := r.UpsertRunForTenant(ctxA, runA); err != nil {
		t.Fatalf("UpsertRunForTenant A: %v", err)
	}
	if err := r.UpsertRunForTenant(ctxB, runB); err != nil {
		t.Fatalf("UpsertRunForTenant B: %v", err)
	}

	listA, err := r.ListRunsForTenant(ctxA)
	if err != nil {
		t.Fatalf("ListRunsForTenant A: %v", err)
	}
	if len(listA) != 1 || listA[0].RunID != runA.RunID {
		t.Fatalf("tenant A should see only its run, got %+v", listA)
	}

	listB, err := r.ListRunsForTenant(ctxB)
	if err != nil {
		t.Fatalf("ListRunsForTenant B: %v", err)
	}
	if len(listB) != 1 || listB[0].RunID != runB.RunID {
		t.Fatalf("tenant B should see only its run, got %+v", listB)
	}
}

func TestRuntimePassA_GetRunInOtherTenantIsHiddenAndAudited(t *testing.T) {
	r, _, bus := newRuntimeFixture(t)
	now := time.Now().UTC()
	ctxA := ctxFor("ten_aaaa1111", "prn_aaaa1111")
	ctxB := ctxFor("ten_bbbb2222", "prn_bbbb2222")

	if err := r.UpsertRunForTenant(ctxA, runtime.Run{
		RunID: "run_secret", Entrypoint: "test", Status: runtime.RunStatusQueued,
		Goal: "secret", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("UpsertRunForTenant A: %v", err)
	}

	ch, cancel := bus.Subscribe(events.Filter{Category: audit.EventCategory})
	defer cancel()

	got, ok, err := r.GetRunForTenant(ctxB, "run_secret")
	if err != nil {
		t.Fatalf("GetRunForTenant B for A's run: %v", err)
	}
	if ok {
		t.Fatalf("cross-tenant lookup MUST return not-found, got %+v", got)
	}

	select {
	case ev := <-ch:
		if ev.Name != audit.EventName {
			t.Fatalf("expected audit event %q, got %q", audit.EventName, ev.Name)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected audit.cross_tenant_access_denied event on cross-tenant read")
	}
}

func TestRuntimePassA_UpsertRefusesOtherTenantsRow(t *testing.T) {
	r, _, _ := newRuntimeFixture(t)
	now := time.Now().UTC()
	ctxA := ctxFor("ten_aaaa1111", "prn_aaaa1111")
	ctxB := ctxFor("ten_bbbb2222", "prn_bbbb2222")

	if err := r.UpsertRunForTenant(ctxA, runtime.Run{
		RunID: "run_shared", Entrypoint: "test", Status: runtime.RunStatusQueued,
		Goal: "a", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("UpsertRunForTenant A: %v", err)
	}

	err := r.UpsertRunForTenant(ctxB, runtime.Run{
		RunID: "run_shared", Entrypoint: "test", Status: runtime.RunStatusQueued,
		Goal: "b", CreatedAt: now, UpdatedAt: now,
	})
	if !errors.Is(err, tenancy.ErrCrossTenantWrite) {
		t.Fatalf("UpsertRunForTenant B for A's run = %v, want ErrCrossTenantWrite", err)
	}
}

func TestRuntimePassA_DeleteRespectsTenantOwnership(t *testing.T) {
	r, _, _ := newRuntimeFixture(t)
	now := time.Now().UTC()
	ctxA := ctxFor("ten_aaaa1111", "prn_aaaa1111")
	ctxB := ctxFor("ten_bbbb2222", "prn_bbbb2222")

	if err := r.UpsertRunForTenant(ctxA, runtime.Run{
		RunID: "run_kept", Entrypoint: "test", Status: runtime.RunStatusQueued,
		Goal: "keep", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("UpsertRunForTenant A: %v", err)
	}

	deleted, err := r.DeleteRunForTenant(ctxB, "run_kept")
	if err != nil {
		t.Fatalf("DeleteRunForTenant cross-tenant: %v", err)
	}
	if deleted {
		t.Fatalf("cross-tenant delete must not remove the row")
	}

	listA, err := r.ListRunsForTenant(ctxA)
	if err != nil || len(listA) != 1 {
		t.Fatalf("tenant A should still own run_kept after cross-tenant delete attempt: list=%v err=%v", listA, err)
	}

	deleted, err = r.DeleteRunForTenant(ctxA, "run_kept")
	if err != nil || !deleted {
		t.Fatalf("DeleteRunForTenant A own row: deleted=%v err=%v", deleted, err)
	}
}
