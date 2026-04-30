package evaluation

import (
	"strings"
	"time"
)

type RedactionPolicy struct {
	SensitiveFieldRules []string `json:"sensitiveFieldRules,omitempty"`
}

type RedactedEvidence struct {
	Status                  RedactionStatus `json:"status"`
	Payload                 map[string]any  `json:"payload"`
	RedactionRulesApplied   []string        `json:"redactionRulesApplied,omitempty"`
	SensitiveFieldsExcluded []string        `json:"sensitiveFieldsExcluded,omitempty"`
}

type CandidateEvidenceInput struct {
	EvidenceID            string
	TenantID              string
	DiscoveredCandidateID string
	SourceRefs            []SourceRef
	Summary               string
	Payload               map[string]any
	RedactionPolicy       RedactionPolicy
	Now                   time.Time
	ExpiresAt             *time.Time
}

func FailedClosedRedactedEvidence(reasonCode string) RedactedEvidence {
	if strings.TrimSpace(reasonCode) == "" {
		reasonCode = "evaluation.redaction_failed"
	}
	return RedactedEvidence{
		Status:                  RedactionStatusFailed,
		Payload:                 map[string]any{},
		RedactionRulesApplied:   []string{"failed_closed"},
		SensitiveFieldsExcluded: []string{reasonCode},
	}
}

func RedactEvidencePayload(payload map[string]any, policy RedactionPolicy) RedactedEvidence {
	sensitive := defaultSensitiveFieldSet()
	for _, field := range policy.SensitiveFieldRules {
		if strings.TrimSpace(field) != "" {
			sensitive[normalizeSensitiveField(field)] = struct{}{}
		}
	}
	redacted, excluded := redactMap(payload, sensitive)
	status := RedactionStatusClean
	if len(excluded) > 0 {
		status = RedactionStatusRedacted
	}
	rules := []string{"default_sensitive_fields"}
	if len(policy.SensitiveFieldRules) > 0 {
		rules = append(rules, "tenant_sensitive_fields")
	}
	return RedactedEvidence{
		Status:                  status,
		Payload:                 redacted,
		RedactionRulesApplied:   rules,
		SensitiveFieldsExcluded: excluded,
	}
}

func CandidateEvidenceFromPayload(input CandidateEvidenceInput) (CandidateEvidence, error) {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return CandidateEvidence{}, err
	}
	if strings.TrimSpace(input.DiscoveredCandidateID) == "" {
		return CandidateEvidence{}, ErrEvaluationProductSourceRequired
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	evidenceID := strings.TrimSpace(input.EvidenceID)
	if evidenceID == "" {
		evidenceID = "evidence_" + strings.TrimSpace(input.DiscoveredCandidateID)
	}
	redacted := RedactEvidencePayload(input.Payload, input.RedactionPolicy)
	return CandidateEvidence{
		EvidenceID:              evidenceID,
		TenantID:                strings.TrimSpace(input.TenantID),
		DiscoveredCandidateID:   strings.TrimSpace(input.DiscoveredCandidateID),
		SourceRefs:              append([]SourceRef(nil), input.SourceRefs...),
		Summary:                 strings.TrimSpace(input.Summary),
		RedactedPayload:         redacted.Payload,
		RedactionRulesApplied:   append([]string(nil), redacted.RedactionRulesApplied...),
		SensitiveFieldsExcluded: append([]string(nil), redacted.SensitiveFieldsExcluded...),
		MaterializationAllowed:  redacted.Status != RedactionStatusFailed,
		RetentionState:          RetentionStateActive,
		CreatedAt:               now.UTC(),
		ExpiresAt:               input.ExpiresAt,
	}, nil
}

func defaultSensitiveFieldSet() map[string]struct{} {
	fields := []string{
		"authorization",
		"access_token",
		"refresh_token",
		"session_token",
		"bearer_token",
		"credential",
		"credentials",
		"secret",
		"secrets",
		"token",
	}
	out := map[string]struct{}{}
	for _, field := range fields {
		out[normalizeSensitiveField(field)] = struct{}{}
	}
	return out
}

func redactMap(payload map[string]any, sensitive map[string]struct{}) (map[string]any, []string) {
	out := map[string]any{}
	var excluded []string
	for key, value := range payload {
		if _, ok := sensitive[normalizeSensitiveField(key)]; ok {
			excluded = append(excluded, key)
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			nested, nestedExcluded := redactMap(typed, sensitive)
			out[key] = nested
			excluded = append(excluded, nestedExcluded...)
		case []any:
			nested, nestedExcluded := redactSlice(typed, sensitive)
			out[key] = nested
			excluded = append(excluded, nestedExcluded...)
		default:
			out[key] = value
		}
	}
	return out, excluded
}

func redactSlice(payload []any, sensitive map[string]struct{}) ([]any, []string) {
	out := make([]any, 0, len(payload))
	var excluded []string
	for _, value := range payload {
		switch typed := value.(type) {
		case map[string]any:
			nested, nestedExcluded := redactMap(typed, sensitive)
			out = append(out, nested)
			excluded = append(excluded, nestedExcluded...)
		case []any:
			nested, nestedExcluded := redactSlice(typed, sensitive)
			out = append(out, nested)
			excluded = append(excluded, nestedExcluded...)
		default:
			out = append(out, value)
		}
	}
	return out, excluded
}

func normalizeSensitiveField(field string) string {
	replacer := strings.NewReplacer("_", "", "-", "", ".", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(field)))
}
