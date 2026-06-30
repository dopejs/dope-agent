package webhook

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestWebhookPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	m := NewManager("test", &recordingFirer{}, nil).WithStore(s)
	created, err := m.Create("ten_a", "hook", TargetKindRoutine, "routine_1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_ = s.Close()

	s2, err := store.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	m2 := NewManager("test", &recordingFirer{}, nil).WithStore(s2)
	if err := m2.LoadFromStore(context.Background()); err != nil {
		t.Fatalf("LoadFromStore: %v", err)
	}
	// The signing secret must survive so a signature still verifies after restart.
	payload := []byte(`{"e":1}`)
	rec, err := m2.Trigger(context.Background(), TriggerInput{WebhookID: created.Endpoint.WebhookID, TenantID: "ten_a", Signature: Sign(created.Secret, payload), IdempotencyKey: "k", Payload: payload})
	if err != nil || rec.Status != TriggerStatusFired {
		t.Fatalf("persisted webhook + secret did not verify after restart: rec=%+v err=%v", rec, err)
	}
}
