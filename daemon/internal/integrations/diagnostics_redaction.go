package integrations

import (
	"regexp"
	"strings"
)

type DiagnosticRedactionResult struct {
	Status  RedactionStatus `json:"status"`
	Summary string          `json:"summary,omitempty"`
}

var diagnosticSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)(access|refresh|id)_token["'=:\s]+[a-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)(app_?secret|client_?secret)["'=:\s]+[a-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)authorization["'=:\s]+[a-z0-9._~+/=-]+`),
}

func RedactDiagnosticSummary(input string) DiagnosticRedactionResult {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return DiagnosticRedactionResult{Status: RedactionStatusRedacted}
	}
	redacted := trimmed
	for _, pattern := range diagnosticSecretPatterns {
		redacted = pattern.ReplaceAllString(redacted, "[REDACTED]")
	}
	if strings.Contains(redacted, "[REDACTED]") {
		return DiagnosticRedactionResult{Status: RedactionStatusSuppressed, Summary: "diagnostic detail suppressed"}
	}
	return DiagnosticRedactionResult{Status: RedactionStatusRedacted, Summary: redacted}
}

func FailClosedDiagnosticRedaction() DiagnosticRedactionResult {
	return DiagnosticRedactionResult{Status: RedactionStatusFailedClosed, Summary: "diagnostic detail suppressed"}
}
