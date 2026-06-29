package capabilities_test

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
)

func TestAdapterRuntimeReadinessGateAndObservability(t *testing.T) {
	sup := capabilities.NewSupervisor()
	client, stop := adapterref.NewPipeClient()
	defer stop()
	rt := capabilities.StartAdapterRuntime(sup, "cap-cal", "calendar", client)

	if rt.Readiness() != capabilities.ReadinessPending {
		t.Fatalf("before probe: readiness = %q, want pending", rt.Readiness())
	}
	if err := rt.Probe(context.Background()); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !rt.Available() || rt.Readiness() != capabilities.ReadinessReady {
		t.Fatalf("after probe: not ready (readiness=%q)", rt.Readiness())
	}

	// Observable as daemon operational truth on the capability surface.
	found := false
	for _, c := range sup.List() {
		if c.CapabilityID == "cap-cal" && c.Kind == capabilities.KindIntegrationAdapter {
			found = true
		}
	}
	if !found {
		t.Fatal("adapter not visible on the capability surface")
	}

	ev := rt.HealthEvent()
	if ev.Domain != "calendar" || ev.Readiness != "ready" || ev.ContractVersion == "" {
		t.Fatalf("unexpected health event: %+v", ev)
	}
}

func TestAdapterRuntimeCircuitBreaksOnRepeatedFailures(t *testing.T) {
	sup := capabilities.NewSupervisor()
	// A version-mismatched adapter fails every readiness handshake without crashing the pipe.
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{ContractVer: "999"})
	defer stop()
	rt := capabilities.StartAdapterRuntime(sup, "cap-mail", "mail", client)

	for i := 0; i < 5; i++ {
		_ = rt.Probe(context.Background())
	}
	if rt.Available() {
		t.Fatal("circuit-broken adapter must not be available")
	}
	if rt.Readiness() != capabilities.ReadinessUnavailable {
		t.Fatalf("readiness = %q, want unavailable after circuit-break", rt.Readiness())
	}
}

func TestAdapterRuntimeRestartReprobes(t *testing.T) {
	sup := capabilities.NewSupervisor()
	client, stop := adapterref.NewPipeClient()
	defer stop()
	rt := capabilities.StartAdapterRuntime(sup, "cap-cal", "calendar", client)
	if err := rt.Probe(context.Background()); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if err := rt.Restart(context.Background()); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !rt.Available() {
		t.Fatal("adapter should be available after a successful restart re-probe")
	}
	cap, _ := sup.Get("cap-cal")
	if cap.RestartCount < 1 {
		t.Fatalf("restart not recorded: %+v", cap)
	}
}
