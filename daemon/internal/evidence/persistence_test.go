package evidence

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestEvidencePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := NewManager("test", fakeCollector{sections: []Section{{Kind: "routine", Summary: map[string]string{"state": "failed"}}}}, nil).WithStore(s)
	bundle, err := m.Generate(context.Background(), "ten_a", "support@dope", routineScope())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
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
	if _, err := m2.Get(context.Background(), "ten_a", "support@dope", bundle.BundleID); err != nil {
		t.Fatalf("bundle did not survive restart: %v", err)
	}
}
