package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
)

// Roadmap 35 (T065) — live event isolation across two tenants. The
// in-process bus fan-out must respect tenantOwnedTenantID filters: a
// subscriber bound to tenant A MUST never receive tenant B's events,
// and a list operation scoped to tenant A MUST omit tenant B's events.
// Backfilled-events isolation (post-T077) is covered by T081a.

func tenantEvent(tenantID, category, name string) events.Event {
	return events.Event{
		EventID:    "evt_" + tenantID + "_" + name,
		Category:   category,
		Name:       name,
		OccurredAt: time.Now().UTC(),
		Resource:   events.Resource{Kind: "test", ID: "res_" + tenantID},
		TenantID:   tenantID,
		Payload:    map[string]any{"tenantId": tenantID},
	}
}

func drainWithin(t *testing.T, ch <-chan events.Event, limit time.Duration) []events.Event {
	t.Helper()
	out := make([]events.Event, 0, 4)
	deadline := time.After(limit)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
}

func TestSubscribersReceiveOnlyTheirTenantsEvents(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	chA, cancelA := bus.Subscribe(events.Filter{TenantOwnedTenantID: "ten_aaaa1111"})
	defer cancelA()
	chB, cancelB := bus.Subscribe(events.Filter{TenantOwnedTenantID: "ten_bbbb2222"})
	defer cancelB()

	bus.Publish(tenantEvent("ten_aaaa1111", "run", "run.created"))
	bus.Publish(tenantEvent("ten_bbbb2222", "run", "run.created"))
	bus.Publish(tenantEvent("ten_aaaa1111", "run", "run.completed"))

	// Allow fan-out to settle.
	gotA := drainWithin(t, chA, 50*time.Millisecond)
	gotB := drainWithin(t, chB, 50*time.Millisecond)

	if len(gotA) != 2 {
		t.Fatalf("tenant A should see 2 events, got %d: %+v", len(gotA), gotA)
	}
	for _, ev := range gotA {
		if ev.TenantID != "ten_aaaa1111" {
			t.Fatalf("tenant A subscriber received cross-tenant event: %+v", ev)
		}
	}
	if len(gotB) != 1 {
		t.Fatalf("tenant B should see 1 event, got %d: %+v", len(gotB), gotB)
	}
	if gotB[0].TenantID != "ten_bbbb2222" {
		t.Fatalf("tenant B subscriber received cross-tenant event: %+v", gotB[0])
	}
}

func TestListWithTenantFilterOmitsOtherTenant(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	bus.Publish(tenantEvent("ten_aaaa1111", "run", "run.created"))
	bus.Publish(tenantEvent("ten_bbbb2222", "run", "run.created"))
	bus.Publish(tenantEvent("ten_aaaa1111", "run", "run.completed"))

	listA := bus.List(events.Filter{TenantOwnedTenantID: "ten_aaaa1111"})
	if len(listA) != 2 {
		t.Fatalf("List for tenant A should return 2 events, got %d: %+v", len(listA), listA)
	}
	for _, ev := range listA {
		if ev.TenantID != "ten_aaaa1111" {
			t.Fatalf("List leaked cross-tenant event into tenant A result: %+v", ev)
		}
	}

	listB := bus.List(events.Filter{TenantOwnedTenantID: "ten_bbbb2222"})
	if len(listB) != 1 || listB[0].TenantID != "ten_bbbb2222" {
		t.Fatalf("List for tenant B should return its single event, got %+v", listB)
	}
}

func TestIncludeGlobalDoesNotLeakTenantOwnedEvents(t *testing.T) {
	bus := events.NewBus()
	defer bus.Close()

	chGlobal, cancel := bus.Subscribe(events.Filter{IncludeGlobal: true})
	defer cancel()

	// A tenant-owned event MUST NOT reach an IncludeGlobal-only subscriber.
	bus.Publish(tenantEvent("ten_aaaa1111", "run", "run.created"))
	// A global event (no tenant id) MUST reach it.
	bus.Publish(events.Event{
		EventID:    "evt_global",
		Category:   "system",
		Name:       "system.heartbeat",
		OccurredAt: time.Now().UTC(),
		Resource:   events.Resource{Kind: "system", ID: "heartbeat"},
		Payload:    map[string]any{"ok": true},
	})

	got := drainWithin(t, chGlobal, 50*time.Millisecond)
	if len(got) != 1 {
		t.Fatalf("global subscriber should see exactly 1 (the global) event, got %d: %+v", len(got), got)
	}
	if got[0].TenantID != "" {
		t.Fatalf("global subscriber received tenant-owned event: %+v", got[0])
	}
}

// Compile-time guard: ensure the test compiles against the events
// package's exported surface (FromContext is unrelated but kept here so
// `events_test` retains an import for context if extended).
var _ = context.Background
