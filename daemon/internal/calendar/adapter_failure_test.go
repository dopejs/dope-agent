package calendar

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
)

// US2 / FR-007a: when an adapter dies mid side-effecting write, the operation is recorded
// exactly once as failed with ambiguous-commit classification, and the daemon stays usable.
func TestCreateEventAmbiguousCommitOnAdapterCrash(t *testing.T) {
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{
		FailMode:       adapterref.FailCrash,
		FailOperations: map[string]bool{"CreateEvent": true}, // ProjectAccount still succeeds
	})
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
		Title:     "ambiguous write",
		StartsAt:  start,
		EndsAt:    start.Add(time.Hour),
	})

	if err == nil {
		t.Fatal("expected an error from the crashed adapter")
	}
	if op.Status != OperationStatusFailed {
		t.Fatalf("operation status = %q, want failed", op.Status)
	}
	if op.FailureClass != "ambiguous_commit" {
		t.Fatalf("FailureClass = %q, want ambiguous_commit", op.FailureClass)
	}

	// Single ledger: exactly one operation recorded, and the manager remains usable.
	ops := m.ListOperations(OperationFilter{})
	if len(ops) != 1 {
		t.Fatalf("operations recorded = %d, want 1 (single ledger)", len(ops))
	}
}
