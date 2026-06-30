package routine

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestRoutinePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := NewManager("test", &fakeScheduler{}).WithStore(s)
	r, err := m.Create(context.Background(), dailyDef())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, _ = m.Pause(context.Background(), r.RoutineID)
	_ = s.Close()

	s2, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	m2 := NewManager("test", &fakeScheduler{}).WithStore(s2)
	if err := m2.LoadFromStore(context.Background()); err != nil {
		t.Fatalf("LoadFromStore: %v", err)
	}
	got, ok := m2.Get(r.RoutineID)
	if !ok || got.State != StatePaused {
		t.Fatalf("routine state did not survive restart: ok=%v got=%+v", ok, got)
	}
}
