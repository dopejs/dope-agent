package opsreadiness

import (
	"fmt"
	"strings"
	"time"
)

func GenerateHostedRunID(profileID string, startedAt time.Time) (string, error) {
	if strings.TrimSpace(profileID) == "" {
		return "", fmt.Errorf("profile id is required")
	}
	if startedAt.IsZero() {
		return "", fmt.Errorf("started at is required")
	}
	return fmt.Sprintf("%s_%s", sanitizeHostedIdentity(profileID), startedAt.UTC().Format("20060102T150405Z")), nil
}

func sanitizeHostedIdentity(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_")
	return replacer.Replace(value)
}
