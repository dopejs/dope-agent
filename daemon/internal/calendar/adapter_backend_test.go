package calendar

import (
	"bufio"
	"io"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// referenceResponder mimics the reference adapter skeleton over pipes: object payload for
// most operations, array payload for list operations, echoing requestId + contract version.
func referenceResponder(t *testing.T) *adapterrpc.Client {
	t.Helper()
	adapterReader, daemonWriter := io.Pipe()
	daemonReader, adapterWriter := io.Pipe()

	arrayOps := map[string]bool{"ListEvents": true}
	go func() {
		br := bufio.NewReader(adapterReader)
		for {
			req, err := adapterrpc.ReadRequest(br)
			if err != nil {
				return
			}
			payload := []byte("{}")
			if arrayOps[req.Operation] {
				payload = []byte("[]")
			}
			resp := adapterrpc.Response{
				RequestID:       req.RequestID,
				ContractVersion: adapterrpc.ContractVersion,
				Status:          adapterrpc.StatusOK,
				Payload:         payload,
			}
			if werr := adapterrpc.WriteMessage(adapterWriter, resp); werr != nil {
				return
			}
		}
	}()

	t.Cleanup(func() { _ = daemonWriter.Close(); _ = adapterWriter.Close() })
	return adapterrpc.NewClient(daemonWriter, daemonReader)
}

func testResource() integrations.Resource {
	return integrations.Resource{
		IntegrationID:    "int-1",
		DomainKind:       "calendar",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindAdapterRPC},
	}
}

func TestAdapterBackendDispatchesOperations(t *testing.T) {
	backend := NewAdapterBackend(referenceResponder(t), 2*time.Second)
	resource := testResource()

	if _, err := backend.ProjectAccount(resource); err != nil {
		t.Fatalf("ProjectAccount: %v", err)
	}
	acct, err := backend.ProjectAccount(resource)
	if err != nil {
		t.Fatalf("ProjectAccount: %v", err)
	}
	if _, err := backend.ListEvents(resource, acct, ListEventsInput{}); err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if _, err := backend.CreateEvent(resource, acct, CreateEventInput{}); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
}

func TestRegisterAdapterBackendSelectable(t *testing.T) {
	m := NewManager("test")
	backend := NewAdapterBackend(referenceResponder(t), 2*time.Second)
	m.RegisterBackend(integrations.BackendKindAdapterRPC, backend)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.backends[integrations.BackendKindAdapterRPC] == nil {
		t.Fatal("adapter_rpc backend not registered")
	}
	if m.backends[integrations.BackendKindFakeLocal] == nil {
		t.Fatal("fake backend must remain registered alongside adapter_rpc")
	}
}
