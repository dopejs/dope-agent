package livevalidation

import "strings"

func CorrelationKey(validationID, ledgerEntryID, actionRef string) string {
	parts := []string{"live_validation", validationID, ledgerEntryID, actionRef}
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, ":")
}
