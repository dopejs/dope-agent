package integrations

import "fmt"

type FakeBackend struct{}

func (FakeBackend) SupportedDomainKinds() []string {
	return []string{"calendar", "mail"}
}

func (FakeBackend) RunProbe(resource Resource, probeKind ProbeKind, input map[string]any) (ProbeResult, error) {
	result := ProbeResult{
		ProbeKind: probeKind,
		Status:    "completed",
		ResultSummary: map[string]any{
			"integrationId": resource.IntegrationID,
			"domainKind":    resource.DomainKind,
			"backendKind":   resource.BackendBinding.BackendKind,
			"probeKind":     probeKind,
		},
	}
	if input != nil && len(input) > 0 {
		result.ResultSummary["input"] = input
	}
	switch probeKind {
	case ProbeKindInspect:
		result.ResultSummary["message"] = fmt.Sprintf("fake inspect probe for %s", resource.DisplayName)
		return result, nil
	case ProbeKindMutate:
		result.ResultSummary["message"] = fmt.Sprintf("fake mutate probe for %s", resource.DisplayName)
		return result, nil
	default:
		return ProbeResult{}, ErrProbeUnsupported
	}
}

func (FakeBackend) supportsDomainKind(domainKind string) bool {
	for _, item := range (FakeBackend{}).SupportedDomainKinds() {
		if item == domainKind {
			return true
		}
	}
	return false
}
