package secrets

import (
	"encoding/json"
	"strings"
)

const RedactedValue = "[REDACTED]"

type RedactedSecretSummary struct {
	SecretRef       string           `json:"secretRef,omitempty"`
	SecretVersionID string           `json:"secretVersionId,omitempty"`
	Resolution      ResolutionStatus `json:"resolution,omitempty"`
	Status          SecretStatus     `json:"status,omitempty"`
	DisabledReason  string           `json:"disabledReason,omitempty"`
	RedactionRule   string           `json:"redactionRule,omitempty"`
}

func RedactSecretValue(value string) string {
	if value == "" {
		return ""
	}
	return RedactedValue
}

func RedactSecretRefs(secretRefs []string) []RedactedSecretSummary {
	items := make([]RedactedSecretSummary, 0, len(secretRefs))
	for _, ref := range secretRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		items = append(items, RedactedSecretSummary{
			SecretRef:      ref,
			Resolution:     ResolutionStatusUnavailable,
			RedactionRule:  "secret_ref_only",
			DisabledReason: "",
		})
	}
	return items
}

func ContainsAnyLeakSentinel(value string, sentinels []string) bool {
	for _, sentinel := range sentinels {
		if sentinel != "" && strings.Contains(value, sentinel) {
			return true
		}
	}
	return false
}

func JSONContainsAnyLeakSentinel(value any, sentinels []string) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	return ContainsAnyLeakSentinel(string(data), sentinels), nil
}
