package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
)

// Roadmap 35 (T064) — API isolation regression.
//
// Exercises the request-scope tenant guard installed by NewServer for
// every tenant_owned by-id route. The full route table is verified
// transitively by the per-domain handler tests; this file verifies the
// guard's contract directly so adding a new tenant_owned route under
// `/v1/<resource>/{id}` MUST be paired with a corresponding wrapper
// (otherwise the guard does not fire and cross-tenant access leaks).
//
// Pass A semantics:
//   - cross-tenant by-id GET → 404 + audit emit, body "not found";
//   - same-tenant by-id GET → guard returns true, request proceeds;
//   - missing tenant context → guard returns true (legacy/anonymous
//     traffic preserved during the staged rollout).

func TestWithByIDTenantGuard_BlocksCrossTenantAndEmitsAudit(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	bus := events.NewBus()
	emitter := audit.NewEmitter(bus, nil)
	rt := tenancy.NewRuntime(s, emitter)

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_aaaa1111"})
	ctxB := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_bbbb2222"})

	if err := rt.UpsertRunForTenant(ctxA, runtime.Run{
		RunID: "run_protected", Entrypoint: "test", Status: runtime.RunStatusQueued, Goal: "hidden",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	innerCalled := false
	inner := func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	}
	guarded := withByIDTenantGuard(s, emitter, "/v1/runs/", "runs", "run_id", "run", inner)

	auditCh, cancel := bus.Subscribe(events.Filter{Category: audit.EventCategory})
	defer cancel()

	// Cross-tenant request: B asks for A's run.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/run_protected", nil).WithContext(ctxB)
	guarded(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant request must 404, got %d", rec.Code)
	}
	if innerCalled {
		t.Fatalf("inner handler must NOT run on cross-tenant denial")
	}
	select {
	case ev := <-auditCh:
		if ev.Name != audit.EventName {
			t.Fatalf("expected %q audit event, got %q", audit.EventName, ev.Name)
		}
		if surface, _ := ev.Payload["surface"].(string); surface != "api:GET /v1/runs/run_protected" {
			t.Fatalf("unexpected audit surface: %v", ev.Payload)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected audit.cross_tenant_access_denied event")
	}

	// Same-tenant request: A asks for A's run — guard passes through.
	innerCalled = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/runs/run_protected", nil).WithContext(ctxA)
	guarded(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-tenant request must succeed, got %d", rec.Code)
	}
	if !innerCalled {
		t.Fatalf("inner handler MUST run for same-tenant request")
	}
}

func TestWithByIDTenantGuard_AllowsAnonymousTraffic(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	innerCalled := false
	inner := func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	}
	guarded := withByIDTenantGuard(s, audit.NewEmitter(events.NewBus(), nil), "/v1/runs/", "runs", "run_id", "run", inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/runs/anything", nil) // no tenant in ctx
	guarded(rec, req)
	if rec.Code != http.StatusOK || !innerCalled {
		t.Fatalf("anonymous traffic must pass through the guard, got code=%d called=%v", rec.Code, innerCalled)
	}
}
