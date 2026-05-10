package matrix

import (
	"strings"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func RedactEvidence(evidence map[string]string) RedactionResult {
	safe := map[string]string{}
	status := baseconnectors.RedactionStatusRedacted
	for key, value := range evidence {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "rawproviderpayload") || strings.Contains(lower, "payload") || strings.Contains(lower, "authorization") {
			status = baseconnectors.RedactionStatusSuppressed
			continue
		}
		safe[key] = value
	}
	return RedactionResult{Status: status, SafeEvidence: safe}
}
