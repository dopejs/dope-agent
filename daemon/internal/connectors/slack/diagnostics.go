package slack

import (
	"errors"
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type slackClassifiedError interface {
	ErrorClass() string
}

func DiagnosticReasonForError(err error) baseconnectors.DiagnosticReasonCode {
	if err == nil {
		return ""
	}
	var classified slackClassifiedError
	if errors.As(err, &classified) {
		switch classified.ErrorClass() {
		case "auth_missing":
			return baseconnectors.DiagnosticAuthMissing
		case "permission_missing":
			return baseconnectors.DiagnosticPermissionMissing
		case "rate_limited":
			return baseconnectors.DiagnosticRateLimited
		case "provider_unavailable":
			return baseconnectors.DiagnosticProviderUnavailable
		case "network_failed":
			return baseconnectors.DiagnosticNetworkFailed
		case "unsupported_capability":
			return baseconnectors.DiagnosticUnsupportedCapability
		}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "oauth"),
		strings.Contains(message, "token"),
		strings.Contains(message, "revoked"),
		strings.Contains(message, "invalid_auth"):
		return baseconnectors.DiagnosticAuthMissing
	case strings.Contains(message, "scope"),
		strings.Contains(message, "permission"),
		strings.Contains(message, "not_in_channel"),
		strings.Contains(message, "approval"):
		return baseconnectors.DiagnosticPermissionMissing
	case strings.Contains(message, "rate"),
		strings.Contains(message, "429"):
		return baseconnectors.DiagnosticRateLimited
	case strings.Contains(message, "network"),
		strings.Contains(message, "event delivery"),
		strings.Contains(message, "timeout"):
		return baseconnectors.DiagnosticNetworkFailed
	case strings.Contains(message, "unsupported"):
		return baseconnectors.DiagnosticUnsupportedCapability
	case strings.Contains(message, "unavailable"),
		strings.Contains(message, "5xx"):
		return baseconnectors.DiagnosticProviderUnavailable
	default:
		return baseconnectors.DiagnosticUnknownConnectorFailure
	}
}

func BuildDiagnosticState(tenantID, connectorID, workspaceBindingID string, reason baseconnectors.DiagnosticReasonCode, evidence map[string]string, now time.Time) (baseconnectors.ConnectorDiagnosticState, error) {
	return baseconnectors.ClassifyDiagnostic(baseconnectors.DiagnosticInput{
		TenantID:           strings.TrimSpace(tenantID),
		ConnectorID:        strings.TrimSpace(connectorID),
		ConnectorAccountID: strings.TrimSpace(workspaceBindingID),
		ReasonCode:         reason,
		EvidenceTimestamp:  now,
		RedactionReliable:  !containsUnsafeEvidence(evidence),
		SafeEvidence:       safeEvidence(evidence),
	})
}

func containsUnsafeEvidence(evidence map[string]string) bool {
	for key, value := range evidence {
		lowerKey := strings.ToLower(key)
		lowerValue := strings.ToLower(value)
		if strings.Contains(lowerKey, "token") ||
			strings.Contains(lowerKey, "secret") ||
			strings.Contains(lowerKey, "authorization") ||
			strings.Contains(lowerValue, "xoxb-") ||
			strings.Contains(lowerValue, "secret") ||
			strings.Contains(lowerValue, "signing secret") ||
			strings.Contains(lowerValue, "authorization") ||
			strings.Contains(lowerValue, "bot token") {
			return true
		}
	}
	return false
}

func safeEvidence(evidence map[string]string) map[string]string {
	if containsUnsafeEvidence(evidence) {
		return nil
	}
	out := make(map[string]string, len(evidence))
	for key, value := range evidence {
		out[key] = value
	}
	return out
}
