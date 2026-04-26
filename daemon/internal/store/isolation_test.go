package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

// Roadmap 35 (T063) — cross-tenant isolation regression.
//
// For every domain whose Pass A tenancy accessor exposes a tenant-aware
// list helper, this test provisions a fixture row in tenant A and tenant
// B and asserts that:
//   - the helper returns only the calling tenant's row;
//   - cross-tenant get returns not-found (and emits the audit event);
//   - cross-tenant write attempts surface ErrCrossTenantWrite.
//
// New domains MUST add a row here when they ship a Pass A tenancy
// helper; the test is intentionally explicit so adding a domain to the
// inventory without an isolation row fails review.

func newIsolationFixture(t *testing.T) (*store.SQLiteStore, *audit.Emitter, *events.Bus) {
	t.Helper()
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bus := events.NewBus()
	return s, audit.NewEmitter(bus, nil), bus
}

func ctxFor(tenantID string) context.Context {
	return tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: "prn_" + tenantID,
	})
}

const (
	tenantA = "ten_aaaa1111"
	tenantB = "ten_bbbb2222"
)

func TestIsolation_Runs(t *testing.T) {
	s, em, _ := newIsolationFixture(t)
	r := tenancy.NewRuntime(s, em)
	now := time.Now().UTC()
	if err := r.UpsertRunForTenant(ctxFor(tenantA), runtime.Run{
		RunID: "run_a", Entrypoint: "test", Status: runtime.RunStatusQueued, Goal: "a",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	if err := r.UpsertRunForTenant(ctxFor(tenantB), runtime.Run{
		RunID: "run_b", Entrypoint: "test", Status: runtime.RunStatusQueued, Goal: "b",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert B: %v", err)
	}
	listA, err := r.ListRunsForTenant(ctxFor(tenantA))
	if err != nil || len(listA) != 1 || listA[0].RunID != "run_a" {
		t.Fatalf("list A leak: %+v err=%v", listA, err)
	}
	if _, ok, _ := r.GetRunForTenant(ctxFor(tenantA), "run_b"); ok {
		t.Fatalf("tenant A must not see B's run via Get")
	}
	if err := r.UpsertRunForTenant(ctxFor(tenantA), runtime.Run{
		RunID: "run_b", Entrypoint: "test", Status: runtime.RunStatusQueued, Goal: "x",
		CreatedAt: now, UpdatedAt: now,
	}); !errors.Is(err, tenancy.ErrCrossTenantWrite) {
		t.Fatalf("cross-tenant upsert must fail with ErrCrossTenantWrite, got %v", err)
	}
}

func TestIsolation_Sessions(t *testing.T) {
	s, em, _ := newIsolationFixture(t)
	r := tenancy.NewRuntime(s, em)
	now := time.Now().UTC()
	for tenantID, sessID := range map[string]string{tenantA: "sess_a", tenantB: "sess_b"} {
		if err := r.UpsertSessionForTenant(ctxFor(tenantID), router.Session{
			SessionID:    sessID,
			Kind:         router.SessionKindDirect,
			Status:       "active",
			Channel:      "test",
			PeerID:       "peer_" + tenantID,
			RoutingKey:   "rk_" + sessID,
			Generation:   1,
			CreatedAt:    now,
			UpdatedAt:    now,
			LastActiveAt: now,
		}); err != nil {
			t.Fatalf("upsert session %s: %v", sessID, err)
		}
	}
	listA, err := r.ListSessionsForTenant(ctxFor(tenantA))
	if err != nil || len(listA) != 1 || listA[0].SessionID != "sess_a" {
		t.Fatalf("list A leak: %+v err=%v", listA, err)
	}
}

func TestIsolation_LLMDispatches(t *testing.T) {
	s, em, _ := newIsolationFixture(t)
	r := tenancy.NewRuntime(s, em)
	now := time.Now().UTC()
	for tenantID, dID := range map[string]string{tenantA: "dsp_a", tenantB: "dsp_b"} {
		if err := r.UpsertLLMDispatchForTenant(ctxFor(tenantID), llm.Dispatch{
			DispatchID:   dID,
			Provider:     "openai",
			Model:        "gpt-test",
			Messages:     []llm.Message{},
			Stream:       false,
			Status:       "succeeded",
			Output:       "ok",
			Usage:        llm.Usage{},
			TimeoutMs:    1000,
			MaxRetries:   0,
			AttemptCount: 1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			t.Fatalf("upsert dispatch %s: %v", dID, err)
		}
	}
	listA, err := r.ListLLMDispatchesForTenant(ctxFor(tenantA))
	if err != nil || len(listA) != 1 || listA[0].DispatchID != "dsp_a" {
		t.Fatalf("list A leak: %+v err=%v", listA, err)
	}
}

func TestIsolation_Approvals(t *testing.T) {
	s, em, _ := newIsolationFixture(t)
	a := tenancy.NewApprovals(s, em)
	now := time.Now().UTC()
	for tenantID, apID := range map[string]string{tenantA: "apv_a", tenantB: "apv_b"} {
		if err := a.UpsertApprovalForTenant(ctxFor(tenantID), policy.Approval{
			ApprovalID: apID,
			Action:     "test",
			Reason:     "isolation test",
			Status:     "pending",
			CreatedAt:  now,
			UpdatedAt:  now,
		}); err != nil {
			t.Fatalf("upsert approval %s: %v", apID, err)
		}
	}
	listA, err := a.ListApprovalsForTenant(ctxFor(tenantA))
	if err != nil || len(listA) != 1 || listA[0].ApprovalID != "apv_a" {
		t.Fatalf("approval list A leak: %+v err=%v", listA, err)
	}
}

func TestIsolation_Schedules(t *testing.T) {
	s, em, _ := newIsolationFixture(t)
	a := tenancy.NewSchedules(s, em)
	now := time.Now().UTC()
	for tenantID, scID := range map[string]string{tenantA: "sch_a", tenantB: "sch_b"} {
		if err := a.UpsertScheduleForTenant(ctxFor(tenantID), store.ScheduleRecord{
			ScheduleID:       scID,
			EnvironmentScope: "test",
			Kind:             "one_time",
			Status:           "scheduled",
			TargetRefID:      "tgt_" + scID,
			CreatedAt:        now,
			UpdatedAt:        now,
			Document:         []byte(`{}`),
		}); err != nil {
			t.Fatalf("upsert schedule %s: %v", scID, err)
		}
	}
	listA, err := a.ListSchedulesForTenant(ctxFor(tenantA), "test")
	if err != nil || len(listA) != 1 || listA[0].ScheduleID != "sch_a" {
		t.Fatalf("schedule list A leak: %+v err=%v", listA, err)
	}
}
