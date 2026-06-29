package calendar

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
)

// US6 / FR-012 / SC-002: an operation executed through the adapter is recorded exactly once
// in the single daemon-owned operation ledger.
func TestAdapterOperationRecordedOnceInSingleLedger(t *testing.T) {
	client, stop := adapterref.NewPipeClient()
	defer stop()
	m := NewManager("test")
	m.RegisterBackend(integrations.BackendKindAdapterRPC, NewAdapterBackend(client, 2*time.Second))

	resource := integrations.Resource{
		IntegrationID:    "int-1",
		DomainKind:       "calendar",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindAdapterRPC},
	}
	start := time.Now().UTC()
	_, _, op, _, err := m.CreateEvent([]integrations.Resource{resource}, CreateEventInput{
		Selection: Selection{IntegrationID: "int-1"},
		Title:     "single ledger",
		StartsAt:  start,
		EndsAt:    start.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if op.Status != OperationStatusCompleted {
		t.Fatalf("operation status = %q, want completed", op.Status)
	}
	if got := len(m.ListOperations(OperationFilter{})); got != 1 {
		t.Fatalf("operations recorded = %d, want 1 (single ledger, no second plane)", got)
	}
}

// US6 / FR-011: the fake backend remains registered by default; registering the adapter does
// not replace it.
func TestFakeBackendRemainsDefault(t *testing.T) {
	m := NewManager("test")
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.backends[integrations.BackendKindFakeLocal] == nil {
		t.Fatal("fake_local must be the default registered backend")
	}
}
