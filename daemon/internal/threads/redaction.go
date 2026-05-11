package threads

import (
	"errors"
	"regexp"
	"strings"
)

type RedactionStatus string

const (
	RedactionStatusRedacted        RedactionStatus = "redacted"
	RedactionStatusSuppressed      RedactionStatus = "suppressed"
	RedactionStatusRedactionFailed RedactionStatus = "redaction_failed"
)

var (
	ErrAuditEvidenceRequired         = errors.New("required audit evidence must be recorded before lifecycle mutation")
	ErrLifecycleTransitionNotAllowed = errors.New("lifecycle transition is not allowed")
	ErrLifecycleMutationConflict     = errors.New("lifecycle mutation conflicted with concurrent thread update")
	ErrLifecycleReopenNotEligible    = errors.New("thread source or session is not eligible for reopen")
)

type SafeEvidenceSummary struct {
	Text   string
	Status RedactionStatus
}

func SafeSummary(text string, allowed bool) SafeEvidenceSummary {
	if !allowed {
		return SafeEvidenceSummary{Text: "suppressed", Status: RedactionStatusSuppressed}
	}
	return SafeEvidenceSummary{Text: strings.TrimSpace(text), Status: RedactionStatusRedacted}
}

var continuityUnsafePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bauthorization\s*:\s*bearer\s+\S+`),
	regexp.MustCompile(`(?i)\b(api[_-]?key|password|secret|access[_-]?token|refresh[_-]?token)\s*[:=]\s*\S+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9][A-Za-z0-9_-]{12,}\b`),
}

func SafeContinuityContent(text string) SafeEvidenceSummary {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return SafeEvidenceSummary{Status: RedactionStatusRedacted}
	}
	if looksLikeRawProviderPayload(trimmed) {
		return SafeEvidenceSummary{Text: "suppressed", Status: RedactionStatusSuppressed}
	}
	for _, pattern := range continuityUnsafePatterns {
		if pattern.MatchString(trimmed) {
			return SafeEvidenceSummary{Text: "suppressed", Status: RedactionStatusSuppressed}
		}
	}
	return SafeSummary(trimmed, true)
}

func looksLikeRawProviderPayload(text string) bool {
	lower := strings.ToLower(text)
	if !(strings.HasPrefix(strings.TrimSpace(text), "{") || strings.HasPrefix(strings.TrimSpace(text), "[")) {
		return false
	}
	return strings.Contains(lower, `"choices"`) && strings.Contains(lower, `"usage"`) ||
		strings.Contains(lower, `"messages"`) && strings.Contains(lower, `"model"`) ||
		strings.Contains(lower, `"raw_provider_payload"`)
}
