package capabilities

import (
	"context"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// KindIntegrationAdapter is the capability kind for supervised integration adapter processes
// (Roadmap 59). Adapters register as capabilities so their readiness, health, and restart
// history are observable as daemon operational truth alongside other capabilities.
const KindIntegrationAdapter = "integration_adapter"

// Readiness is the integration-facing readiness derived from supervisor status.
type Readiness string

const (
	ReadinessPending     Readiness = "pending"
	ReadinessReady       Readiness = "ready"
	ReadinessUnavailable Readiness = "unavailable"
)

// AdapterRuntime bridges an integration adapter's RPC client into the capability supervisor:
// it gates readiness on the contract-version handshake, reports health/failure (driving the
// supervisor's backoff and circuit-break), and projects adapter health for observability.
type AdapterRuntime struct {
	sup             *Supervisor
	capabilityID    string
	domain          string
	contractVersion string
	client          *adapterrpc.Client
}

// StartAdapterRuntime registers the adapter as a capability and returns its runtime. Call
// Probe to run the readiness handshake and begin reporting health.
func StartAdapterRuntime(sup *Supervisor, capabilityID, domain string, client *adapterrpc.Client) *AdapterRuntime {
	_, _, _ = sup.Register(RegisterInput{
		CapabilityID: capabilityID,
		Kind:         KindIntegrationAdapter,
		DisplayName:  domain + " integration adapter",
	})
	return &AdapterRuntime{
		sup:             sup,
		capabilityID:    capabilityID,
		domain:          domain,
		contractVersion: adapterrpc.ContractVersion,
		client:          client,
	}
}

// Probe runs the contract-version readiness/heartbeat handshake and updates supervisor state.
// On contract mismatch or an unreachable adapter it reports failure, which drives the
// supervisor's exponential backoff and, once the restart bound is exceeded, circuit-breaks to
// StatusFailed — surfaced here as ReadinessUnavailable (FR-004a, FR-008, spec Q2/Q3).
func (rt *AdapterRuntime) Probe(ctx context.Context) error {
	if err := rt.client.Ready(ctx); err != nil {
		_, _ = rt.sup.ReportFailure(rt.capabilityID, ReportFailureInput{Reason: err.Error()})
		return err
	}
	_, _ = rt.sup.ReportHealth(rt.capabilityID, ReportHealthInput{Status: StatusHealthy})
	return nil
}

// Restart asks the supervisor to restart the adapter (resetting backoff) and re-probes.
func (rt *AdapterRuntime) Restart(ctx context.Context) error {
	if _, err := rt.sup.Restart(rt.capabilityID); err != nil {
		return err
	}
	return rt.Probe(ctx)
}

// CapabilityID returns the supervised capability id.
func (rt *AdapterRuntime) CapabilityID() string { return rt.capabilityID }

// Readiness maps supervisor status to integration readiness. A circuit-broken adapter
// (StatusFailed) is reported unavailable (Q2).
func (rt *AdapterRuntime) Readiness() Readiness {
	capability, ok := rt.sup.Get(rt.capabilityID)
	if !ok {
		return ReadinessUnavailable
	}
	switch capability.Status {
	case StatusHealthy:
		return ReadinessReady
	case StatusFailed:
		return ReadinessUnavailable
	default:
		return ReadinessPending
	}
}

// Available reports whether the adapter should receive operations.
func (rt *AdapterRuntime) Available() bool { return rt.Readiness() == ReadinessReady }

// AdapterHealthEvent projects current state onto schemas/events/integrations/adapter-health.json.
type AdapterHealthEvent struct {
	CapabilityID    string `json:"capabilityId"`
	Domain          string `json:"domain"`
	Status          string `json:"status"`
	Readiness       string `json:"readiness"`
	RestartCount    int    `json:"restartCount"`
	FailureCount    int    `json:"failureCount"`
	ContractVersion string `json:"contractVersion"`
	Reason          string `json:"reason,omitempty"`
}

// HealthEvent returns the current adapter health projection for observability/event fan-out.
func (rt *AdapterRuntime) HealthEvent() AdapterHealthEvent {
	capability, _ := rt.sup.Get(rt.capabilityID)
	return AdapterHealthEvent{
		CapabilityID:    rt.capabilityID,
		Domain:          rt.domain,
		Status:          string(capability.Status),
		Readiness:       string(rt.Readiness()),
		RestartCount:    capability.RestartCount,
		FailureCount:    capability.FailureCount,
		ContractVersion: rt.contractVersion,
		Reason:          capability.LastFailureReason,
	}
}
