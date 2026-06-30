package triage

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestTriagePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := NewManager("test").WithStore(s)
	p, err := m.CreatePolicy("inbox", []Rule{{Conditions: []Condition{{Field: FieldSender, Operator: OperatorContains, Value: "x"}}, Classification: ClassificationFYI, Outcome: OutcomeNoAction}}, ClassificationFYI)
	if err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}
	_ = s.Close()

	s2, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	m2 := NewManager("test").WithStore(s2)
	if err := m2.LoadFromStore(context.Background()); err != nil {
		t.Fatalf("LoadFromStore: %v", err)
	}
	if _, ok := m2.GetPolicy(p.PolicyID); !ok {
		t.Fatalf("policy %s did not survive restart", p.PolicyID)
	}
}
