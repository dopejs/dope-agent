package events

import "testing"

func TestPublishAndList(t *testing.T) {
	bus := NewBus()

	published := bus.Publish(Event{
		Category: "run",
		Name:     "run.created",
		Scope: Scope{
			RunID: "run_1",
		},
		Resource: Resource{
			Kind: "run",
			ID:   "run_1",
		},
		Payload: map[string]any{
			"entrypoint": "chat",
		},
	})
	if published.Sequence == 0 {
		t.Fatal("expected published event sequence")
	}

	runEvents := bus.List(Filter{RunID: "run_1"})
	if len(runEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(runEvents))
	}
	if runEvents[0].Name != "run.created" {
		t.Fatalf("expected run.created, got %s", runEvents[0].Name)
	}
	if runEvents[0].Sequence != published.Sequence {
		t.Fatalf("expected event sequence %d, got %d", published.Sequence, runEvents[0].Sequence)
	}
}

func TestSubscribeFiltersEvents(t *testing.T) {
	bus := NewBus()

	ch, unsubscribe := bus.Subscribe(Filter{Category: "run"})
	defer unsubscribe()

	bus.Publish(Event{
		Category: "system",
		Name:     "system.started",
		Resource: Resource{
			Kind: "system",
			ID:   "dope",
		},
	})

	bus.Publish(Event{
		Category: "run",
		Name:     "run.created",
		Scope: Scope{
			RunID: "run_2",
		},
		Resource: Resource{
			Kind: "run",
			ID:   "run_2",
		},
	})

	event := <-ch
	if event.Category != "run" {
		t.Fatalf("expected run category, got %s", event.Category)
	}
}

func TestListAndSubscribeRespectCursor(t *testing.T) {
	bus := NewBus()

	first := bus.Publish(Event{
		Category: "run",
		Name:     "run.created",
		Resource: Resource{Kind: "run", ID: "run_1"},
	})
	second := bus.Publish(Event{
		Category: "run",
		Name:     "run.status_changed",
		Resource: Resource{Kind: "run", ID: "run_1"},
	})

	items := bus.List(Filter{Category: "run", Cursor: first.Sequence})
	if len(items) != 1 {
		t.Fatalf("expected 1 event after cursor, got %d", len(items))
	}
	if items[0].Sequence != second.Sequence {
		t.Fatalf("expected sequence %d, got %d", second.Sequence, items[0].Sequence)
	}

	ch, unsubscribe := bus.Subscribe(Filter{Category: "run", Cursor: second.Sequence})
	defer unsubscribe()

	bus.Publish(Event{
		Category: "run",
		Name:     "run.cancelled",
		Resource: Resource{Kind: "run", ID: "run_1"},
	})

	event := <-ch
	if event.Sequence <= second.Sequence {
		t.Fatalf("expected event sequence after %d, got %d", second.Sequence, event.Sequence)
	}
}

// Roadmap 35: tenant-owned event subscribers MUST receive only their tenant's
// events and MUST NOT see global (NULL-tenant) events. The default filter
// continues to see global events for backward compatibility.
func TestTenantOwnedFilterIsolatesAcrossTenants(t *testing.T) {
	bus := NewBus()

	chA, unA := bus.Subscribe(Filter{TenantOwnedTenantID: "ten_a"})
	defer unA()
	chB, unB := bus.Subscribe(Filter{TenantOwnedTenantID: "ten_b"})
	defer unB()

	bus.Publish(Event{TenantID: "ten_a", Category: "run", Name: "run.created", Resource: Resource{Kind: "run", ID: "run_1"}})
	bus.Publish(Event{TenantID: "ten_b", Category: "run", Name: "run.created", Resource: Resource{Kind: "run", ID: "run_2"}})
	// Global event MUST NOT be delivered to tenant-owned subscribers.
	bus.Publish(Event{Category: "system", Name: "system.started", Resource: Resource{Kind: "system", ID: "dope"}})

	gotA := drain(chA, 2)
	gotB := drain(chB, 2)

	if len(gotA) != 1 || gotA[0].TenantID != "ten_a" {
		t.Fatalf("tenant A subscriber received wrong events: %+v", gotA)
	}
	if len(gotB) != 1 || gotB[0].TenantID != "ten_b" {
		t.Fatalf("tenant B subscriber received wrong events: %+v", gotB)
	}
}

func TestIncludeGlobalSubscribesOnlyToGlobal(t *testing.T) {
	bus := NewBus()
	ch, unsub := bus.Subscribe(Filter{IncludeGlobal: true})
	defer unsub()

	bus.Publish(Event{TenantID: "ten_a", Category: "run", Name: "run.created", Resource: Resource{Kind: "run", ID: "run_1"}})
	bus.Publish(Event{Category: "system", Name: "system.started", Resource: Resource{Kind: "system", ID: "dope"}})

	// IncludeGlobal=true (without a TenantOwnedTenantID) MUST deliver
	// ONLY events that have no tenant id — the tenant-owned event above
	// must be filtered out.
	got := drain(ch, 2)
	if len(got) != 1 {
		t.Fatalf("expected only the global event, got %d: %+v", len(got), got)
	}
	if got[0].TenantID != "" {
		t.Fatalf("IncludeGlobal subscriber received tenant-owned event: %+v", got[0])
	}
}

func drain(ch <-chan Event, max int) []Event {
	out := make([]Event, 0, max)
	for i := 0; i < max; i++ {
		select {
		case e, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, e)
		default:
			return out
		}
	}
	return out
}
