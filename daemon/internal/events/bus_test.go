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
