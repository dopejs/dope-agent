package integrations

import "fmt"

type FakeFaultType string

const (
	FakeFaultTransient5xx        FakeFaultType = "transient_5xx"
	FakeFaultRateLimit           FakeFaultType = "rate_limit"
	FakeFaultAuthExpiry          FakeFaultType = "auth_expiry"
	FakeFaultProviderUnavailable FakeFaultType = "provider_unavailable"
	FakeFaultSlowResponse        FakeFaultType = "slow_response"
	FakeFaultMalformedResponse   FakeFaultType = "malformed_response"
)

type FakeFaultDrillResult struct {
	FaultType              FakeFaultType
	DomainKind             string
	ObservedClassification string
	OperatorActionNeeded   bool
}

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

func (FakeBackend) RunFaultDrill(resource Resource, faultType FakeFaultType) FakeFaultDrillResult {
	classification := "recovered"
	operatorActionNeeded := false
	switch faultType {
	case FakeFaultAuthExpiry, FakeFaultMalformedResponse:
		classification = "operator_action_needed"
		operatorActionNeeded = true
	case FakeFaultProviderUnavailable:
		classification = "retry_exhausted"
		operatorActionNeeded = true
	}
	return FakeFaultDrillResult{
		FaultType:              faultType,
		DomainKind:             resource.DomainKind,
		ObservedClassification: classification,
		OperatorActionNeeded:   operatorActionNeeded,
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
