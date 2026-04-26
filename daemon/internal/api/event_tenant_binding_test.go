package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Roadmap 35 review remediation — Finding #3.
//
// publishEvent (the central event-write path used by every API handler
// that emits a runtime event) was writing tenant-owned events with
// tenant_id = NULL. The same handler's SSE/list path filters on
// tenant_id, so the freshly-written event was invisible to the
// tenant — a silent isolation regression. These tests prove the new
// behaviour: tenant-owned categories get tenant_id bound; global
// categories stay tenantless.

func TestPublishEvent_TenantOwnedCategoryBindsTenantID(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bus := events.NewBus()

	ctx := withTenantContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_aaaa1111",
		PrincipalID: "prn_aaaa1111",
	})

	persisted, err := publishEvent(ctx, bus, s, events.Event{
		EventID:    "evt_run_created",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: time.Now().UTC(),
		Resource:   events.Resource{Kind: "run", ID: "run_xyz"},
		Payload:    map[string]any{"status": "queued"},
	})
	if err != nil {
		t.Fatalf("publishEvent: %v", err)
	}
	if persisted.TenantID != "ten_aaaa1111" {
		t.Fatalf("expected persisted event TenantID=ten_aaaa1111, got %q", persisted.TenantID)
	}

	// SSE replay scoped to the same tenant MUST see the new event.
	visible, err := s.ListEventsForTenantRaw(ctx, "ten_aaaa1111", events.Filter{})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw: %v", err)
	}
	found := false
	for _, ev := range visible {
		if ev.EventID == "evt_run_created" {
			found = true
			if ev.TenantID != "ten_aaaa1111" {
				t.Fatalf("event row missing tenant_id binding: %+v", ev)
			}
		}
	}
	if !found {
		t.Fatalf("tenant SSE replay did not surface freshly-written event (silent isolation regression)")
	}
}

func TestAppendEventForTenantRaw_RefusesCrossTenantCollision(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	original := events.Event{
		EventID:    "evt_collide",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: now,
		Resource:   events.Resource{Kind: "run", ID: "run_a"},
		Payload:    map[string]any{"owner": "A"},
	}
	if _, err := s.AppendEventForTenantRaw(context.Background(), original, "ten_aaaa1111"); err != nil {
		t.Fatalf("seed A event: %v", err)
	}

	// Tenant B attempts to write an event with the same event_id.
	// Pre-fix: AppendEvent would no-op via ON CONFLICT DO NOTHING and
	// the subsequent UPDATE would silently leave the row owned by A,
	// then the helper returned a misleading "bound" event. Post-fix:
	// the atomic INSERT detects the conflict, looks up the existing
	// owner, and returns ErrCrossTenantRow.
	collision := events.Event{
		EventID:    "evt_collide",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: now,
		Resource:   events.Resource{Kind: "run", ID: "run_b"},
		Payload:    map[string]any{"owner": "B"},
	}
	_, err = s.AppendEventForTenantRaw(context.Background(), collision, "ten_bbbb2222")
	if !errors.Is(err, store.ErrCrossTenantRow) {
		t.Fatalf("cross-tenant event collision MUST return ErrCrossTenantRow, got %v", err)
	}

	// Verify A still owns the row and B's payload did NOT bleed in.
	visible, err := s.ListEventsForTenantRaw(context.Background(), "ten_aaaa1111", events.Filter{})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw A: %v", err)
	}
	found := false
	for _, ev := range visible {
		if ev.EventID == "evt_collide" {
			found = true
			if owner, _ := ev.Payload["owner"].(string); owner != "A" {
				t.Fatalf("tenant A's event payload was clobbered: %+v", ev.Payload)
			}
			if ev.TenantID != "ten_aaaa1111" {
				t.Fatalf("tenant A lost ownership: %+v", ev)
			}
		}
	}
	if !found {
		t.Fatalf("tenant A's event vanished after refused B write")
	}

	// Tenant B MUST NOT see the row.
	visibleB, err := s.ListEventsForTenantRaw(context.Background(), "ten_bbbb2222", events.Filter{})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw B: %v", err)
	}
	for _, ev := range visibleB {
		if ev.EventID == "evt_collide" {
			t.Fatalf("tenant B saw tenant A's event after refused write: %+v", ev)
		}
	}
}

// Round-3 Finding #2: a NULL-tenant row whose category is GLOBAL
// (system, mcp, provider, daemon.migration, connector_global,
// capability_global) MUST NOT be claimed by a tenant emit, even when
// the tenant's incoming event_id collides with a previously-emitted
// global event. The pre-fix code silently retagged the global row,
// which both leaked daemon-wide events into a tenant scope and broke
// global subscribers that expected NULL ownership.
func TestAppendEventForTenantRaw_RefusesGlobalCategoryClaim(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	// Pre-existing global event (system.heartbeat emitted at startup).
	if _, err := s.AppendEvent(context.Background(), events.Event{
		EventID:    "evt_global_collide",
		Category:   "system",
		Name:       "system.heartbeat",
		OccurredAt: now,
		Resource:   events.Resource{Kind: "system", ID: "heartbeat"},
		Payload:    map[string]any{"global": true},
	}); err != nil {
		t.Fatalf("seed global event: %v", err)
	}

	// Tenant attempts to claim it via a tenant-owned-category emit.
	_, err = s.AppendEventForTenantRaw(context.Background(), events.Event{
		EventID:    "evt_global_collide",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: now,
		Resource:   events.Resource{Kind: "run", ID: "run_x"},
		Payload:    map[string]any{"tenant": "stolen"},
	}, "ten_aaaa1111")
	if !errors.Is(err, store.ErrCrossTenantRow) {
		t.Fatalf("global-category claim MUST return ErrCrossTenantRow, got %v", err)
	}

	// Verify the global row was NOT mutated (still NULL-tenant, original payload).
	tenantVisible, err := s.ListEventsForTenantRaw(context.Background(), "ten_aaaa1111", events.Filter{})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw: %v", err)
	}
	for _, ev := range tenantVisible {
		if ev.EventID == "evt_global_collide" {
			t.Fatalf("global event leaked into tenant scope after refused claim: %+v", ev)
		}
	}
}

// Pre-backfill compat: a NULL-tenant row whose category is TENANT-OWNED
// (e.g., "run") may be claimed. This covers historical events emitted
// before tenant_id binding was wired. Pass B removes this branch.
func TestAppendEventForTenantRaw_ClaimsTenantCategoryNullRow(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	now := time.Now().UTC()
	// Pre-existing tenant-category event with NULL tenant_id (pre-backfill state).
	if _, err := s.AppendEvent(context.Background(), events.Event{
		EventID:    "evt_legacy_run",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: now,
		Resource:   events.Resource{Kind: "run", ID: "run_legacy"},
		Payload:    map[string]any{"legacy": true},
	}); err != nil {
		t.Fatalf("seed legacy event: %v", err)
	}

	persisted, err := s.AppendEventForTenantRaw(context.Background(), events.Event{
		EventID:    "evt_legacy_run",
		Category:   "run",
		Name:       "run.created",
		OccurredAt: now,
		Resource:   events.Resource{Kind: "run", ID: "run_legacy"},
		Payload:    map[string]any{"legacy": true},
	}, "ten_aaaa1111")
	if err != nil {
		t.Fatalf("claim of tenant-category NULL row should succeed (pre-backfill compat), got %v", err)
	}
	if persisted.TenantID != "ten_aaaa1111" {
		t.Fatalf("post-claim event must report bound tenant, got %q", persisted.TenantID)
	}

	// Verify the row is now visible to the claiming tenant.
	visible, err := s.ListEventsForTenantRaw(context.Background(), "ten_aaaa1111", events.Filter{})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw: %v", err)
	}
	found := false
	for _, ev := range visible {
		if ev.EventID == "evt_legacy_run" {
			found = true
		}
	}
	if !found {
		t.Fatalf("claimed legacy event MUST be visible to the new owner")
	}
}

func TestPublishEvent_GlobalCategoryStaysTenantless(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	bus := events.NewBus()

	ctx := withTenantContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_aaaa1111",
		PrincipalID: "prn_aaaa1111",
	})

	persisted, err := publishEvent(ctx, bus, s, events.Event{
		EventID:    "evt_system_heartbeat",
		Category:   "system",
		Name:       "system.heartbeat",
		OccurredAt: time.Now().UTC(),
		Resource:   events.Resource{Kind: "system", ID: "heartbeat"},
		Payload:    map[string]any{"ok": true},
	})
	if err != nil {
		t.Fatalf("publishEvent: %v", err)
	}
	if persisted.TenantID != "" {
		t.Fatalf("global-category event MUST stay tenantless, got TenantID=%q", persisted.TenantID)
	}

	// Tenant-scoped read MUST NOT see it.
	visible, err := s.ListEventsForTenantRaw(ctx, "ten_aaaa1111", events.Filter{})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw: %v", err)
	}
	for _, ev := range visible {
		if ev.EventID == "evt_system_heartbeat" {
			t.Fatalf("global event leaked into tenant scope: %+v", ev)
		}
	}
}
