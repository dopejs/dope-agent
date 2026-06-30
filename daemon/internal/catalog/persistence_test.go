package catalog

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestCatalogPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := NewManager("test", nil, nil).WithStore(s)
	item, _ := m.RegisterItem(sampleItem())
	_, _ = m.Enable(context.Background(), "ten_a", item.ItemID, "1.0.0", "op")
	_ = s.Close()

	s2, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	m2 := NewManager("test", nil, nil).WithStore(s2)
	if err := m2.LoadFromStore(context.Background()); err != nil {
		t.Fatalf("LoadFromStore: %v", err)
	}
	if _, ok := m2.GetItem(item.ItemID); !ok {
		t.Fatalf("catalog item did not survive restart")
	}
	if v, ok := m2.ActiveVersion(context.Background(), "ten_a", item.ItemID); !ok || v != "1.0.0" {
		t.Fatalf("enablement did not survive restart: v=%q ok=%v", v, ok)
	}
}
