package integrations

import (
	"fmt"
	"strings"
	"time"
)

type FeishuLarkDiagnosticBackend struct{}

func (FeishuLarkDiagnosticBackend) SupportedDomainKinds() []string {
	return []string{"calendar", "mail", "reminders", "delivery"}
}

func (b FeishuLarkDiagnosticBackend) RunProbe(resource Resource, probeKind ProbeKind, input map[string]any) (ProbeResult, error) {
	if !b.supportsDomainKind(resource.DomainKind) {
		return ProbeResult{}, ErrProbeUnsupported
	}
	if probeKind != ProbeKindInspect && probeKind != ProbeKindMutate {
		return ProbeResult{}, ErrProbeUnsupported
	}
	evidence := feishuLarkEvidenceFromProbe(resource, input)
	classification := ClassifyProviderEvidence(evidence)
	status := "completed"
	if classification.ReasonCode != ReasonHealthy {
		status = "failed"
	}
	summary := map[string]any{
		"integrationId":      resource.IntegrationID,
		"domainKind":         resource.DomainKind,
		"backendKind":        string(resource.BackendBinding.BackendKind),
		"probeKind":          string(probeKind),
		"operationClass":     evidence.OperationClass,
		"reasonCode":         string(classification.ReasonCode),
		"retrySafety":        string(classification.RetrySafety),
		"remediationOwner":   string(classification.RemediationOwner),
		"redactionStatus":    string(classification.RedactionStatus),
		"evidenceConfidence": classification.EvidenceConfidence,
	}
	return ProbeResult{
		ProbeKind:     probeKind,
		Status:        status,
		FailureClass:  firstNonEmptyString(classification.RedactedProviderCode, string(classification.ReasonCode)),
		ResultSummary: summary,
	}, nil
}

func (FeishuLarkDiagnosticBackend) supportsDomainKind(domainKind string) bool {
	switch strings.ToLower(strings.TrimSpace(domainKind)) {
	case "calendar", "mail", "reminders", "delivery":
		return true
	default:
		return false
	}
}

func feishuLarkEvidenceFromProbe(resource Resource, input map[string]any) ProviderDiagnosticEvidence {
	rawEvidence := map[string]any{}
	if nested, ok := input["providerEvidence"].(map[string]any); ok {
		for key, value := range nested {
			rawEvidence[key] = value
		}
	}
	if code := strings.TrimSpace(fmt.Sprint(input["code"])); code != "" && code != "<nil>" {
		rawEvidence["code"] = code
	}
	if status := strings.TrimSpace(fmt.Sprint(input["status"])); status != "" && status != "<nil>" {
		rawEvidence["status"] = status
	}
	if message := strings.TrimSpace(fmt.Sprint(input["message"])); message != "" && message != "<nil>" {
		rawEvidence["message"] = message
	}
	if len(rawEvidence) == 0 && strings.TrimSpace(resource.ReadinessReason) != "" {
		rawEvidence["message"] = resource.ReadinessReason
	}
	evidence := ProviderEvidenceFromMap(string(BackendKindFeishuLark), resource.DomainKind, rawEvidence)
	evidence.IntegrationID = resource.IntegrationID
	evidence.OperationClass = strings.TrimSpace(fmt.Sprint(input["operationClass"]))
	if evidence.OperationClass == "" || evidence.OperationClass == "<nil>" {
		evidence.OperationClass = "integration.readiness"
	}
	evidence.CreatedAt = time.Now().UTC()
	return evidence
}
