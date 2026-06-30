package execprofile

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestExecProfilePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := NewManager("test", nil, nil, nil).WithStore(s)
	seed(t, m)
	if _, err := m.SelectProfile(context.Background(), "ten_a", "p_subproc", "op"); err != nil {
		t.Fatalf("SelectProfile: %v", err)
	}
	_ = s.Close()

	s2, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	m2 := NewManager("test", nil, nil, nil).WithStore(s2)
	if err := m2.LoadFromStore(context.Background()); err != nil {
		t.Fatalf("LoadFromStore: %v", err)
	}
	if _, err := m2.GetProfile(context.Background(), "p_subproc"); err != nil {
		t.Fatalf("profile did not survive restart: %v", err)
	}
	if sel, ok := m2.SelectionForTenant("ten_a"); !ok || sel.ProfileID != "p_subproc" {
		t.Fatalf("selection did not survive restart: ok=%v sel=%+v", ok, sel)
	}
}
