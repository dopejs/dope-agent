package api

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Roadmap 35 review remediation — Finding #1.
//
// Even after the tenancy.* helpers existed, the production list paths
// (handleRuns / handleSessions) were still calling the in-memory
// runtime.Manager / SessionRouter and writing the raw enumeration to
// the response — every tenant saw every other tenant's runs and
// sessions. These tests prove filterRunsByTenant /
// filterSessionsByTenant DO scope the response, both for the resolved
// tenant case and for legacy/anonymous traffic.

func seedRunWithTenant(t *testing.T, s *store.SQLiteStore, runID, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if tenantID == "" {
		if err := s.UpsertRun(context.Background(), runtime.Run{
			RunID: runID, Entrypoint: "t", Status: runtime.RunStatusQueued,
			Goal: "g", CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed run %s: %v", runID, err)
		}
		return
	}
	if err := s.UpsertRunForTenantSafe(context.Background(), runtime.Run{
		RunID: runID, Entrypoint: "t", Status: runtime.RunStatusQueued,
		Goal: "g", CreatedAt: now, UpdatedAt: now,
	}, tenantID); err != nil {
		t.Fatalf("seed run %s tenant %s: %v", runID, tenantID, err)
	}
}

func seedSessionWithTenant(t *testing.T, s *store.SQLiteStore, sessID, tenantID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := s.UpsertSessionForTenantSafe(context.Background(), router.Session{
		SessionID:    sessID,
		Kind:         router.SessionKindDirect,
		Status:       "active",
		Channel:      "test",
		PeerID:       "peer_" + sessID,
		RoutingKey:   "rk_" + sessID,
		Generation:   1,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActiveAt: now,
	}, tenantID); err != nil {
		t.Fatalf("seed session %s tenant %s: %v", sessID, tenantID, err)
	}
}

func TestFilterRunsByTenant_ScopesAndPreservesLegacyRows(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	seedRunWithTenant(t, s, "run_a", "ten_aaaa1111")
	seedRunWithTenant(t, s, "run_b", "ten_bbbb2222")
	seedRunWithTenant(t, s, "run_legacy", "") // pre-backfill, NULL tenant_id

	// In-memory enumeration as returned by runtime.Manager.ListRuns().
	allRuns := []runtime.Run{
		{RunID: "run_a"}, {RunID: "run_b"}, {RunID: "run_legacy"},
	}

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	gotA, err := filterRunsByTenant(ctxA, s, allRuns)
	if err != nil {
		t.Fatalf("filter A: %v", err)
	}
	if !containsRun(gotA, "run_a") {
		t.Fatalf("tenant A must see its own run, got %+v", gotA)
	}
	if containsRun(gotA, "run_b") {
		t.Fatalf("tenant A leaked tenant B's run: %+v", gotA)
	}
	if !containsRun(gotA, "run_legacy") {
		t.Fatalf("tenant A must see legacy NULL-tenant rows during the staged rollout, got %+v", gotA)
	}

	ctxB := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_bbbb2222", PrincipalID: "prn_b"})
	gotB, err := filterRunsByTenant(ctxB, s, allRuns)
	if err != nil {
		t.Fatalf("filter B: %v", err)
	}
	if containsRun(gotB, "run_a") {
		t.Fatalf("tenant B leaked tenant A's run: %+v", gotB)
	}
	if !containsRun(gotB, "run_b") {
		t.Fatalf("tenant B must see its own run, got %+v", gotB)
	}

	// No tenant context (anonymous / legacy traffic) — pass-through.
	got, err := filterRunsByTenant(context.Background(), s, allRuns)
	if err != nil {
		t.Fatalf("anonymous filter: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("anonymous traffic must pass through unfiltered, got %d rows: %+v", len(got), got)
	}
}

func TestFilterSessionsByTenant_ScopesProperly(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	seedSessionWithTenant(t, s, "sess_a", "ten_aaaa1111")
	seedSessionWithTenant(t, s, "sess_b", "ten_bbbb2222")

	all := []router.Session{{SessionID: "sess_a"}, {SessionID: "sess_b"}}

	ctxA := withTenantContext(context.Background(), identity.TenantContext{TenantID: "ten_aaaa1111", PrincipalID: "prn_a"})
	gotA, err := filterSessionsByTenant(ctxA, s, all)
	if err != nil {
		t.Fatalf("filter A: %v", err)
	}
	if len(gotA) != 1 || gotA[0].SessionID != "sess_a" {
		t.Fatalf("tenant A leak/loss: %+v", gotA)
	}
}

func containsRun(rs []runtime.Run, id string) bool {
	for _, r := range rs {
		if r.RunID == id {
			return true
		}
	}
	return false
}
