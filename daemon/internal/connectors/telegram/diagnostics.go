package telegram

import (
	"errors"
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type classifiedTelegramError interface {
	ErrorClass() string
}

func DiagnosticReasonForError(err error) baseconnectors.DiagnosticReasonCode {
	if err == nil {
		return ""
	}
	var classified classifiedTelegramError
	if errors.As(err, &classified) {
		switch classified.ErrorClass() {
		case "auth_error":
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
	case strings.Contains(message, "401"),
		strings.Contains(message, "unauthorized"),
		strings.Contains(message, "token"):
		return baseconnectors.DiagnosticAuthMissing
	case strings.Contains(message, "403"),
		strings.Contains(message, "forbidden"),
		strings.Contains(message, "permission"):
		return baseconnectors.DiagnosticPermissionMissing
	case strings.Contains(message, "429"),
		strings.Contains(message, "rate limit"),
		strings.Contains(message, "too many requests"):
		return baseconnectors.DiagnosticRateLimited
	case strings.Contains(message, "unsupported"),
		strings.Contains(message, "attachment"),
		strings.Contains(message, "voice"),
		strings.Contains(message, "payment"),
		strings.Contains(message, "mini app"):
		return baseconnectors.DiagnosticUnsupportedCapability
	case strings.Contains(message, "unavailable"),
		strings.Contains(message, "5xx"):
		return baseconnectors.DiagnosticProviderUnavailable
	case strings.Contains(message, "network"),
		strings.Contains(message, "connection"),
		strings.Contains(message, "reconnect"):
		return baseconnectors.DiagnosticNetworkFailed
	default:
		return baseconnectors.DiagnosticUnknownConnectorFailure
	}
}

func BuildDiagnosticState(tenantID, connectorID, connectorAccountID string, reason baseconnectors.DiagnosticReasonCode, evidence map[string]string, now time.Time) (baseconnectors.ConnectorDiagnosticState, error) {
	return baseconnectors.ClassifyDiagnostic(baseconnectors.DiagnosticInput{
		TenantID:           strings.TrimSpace(tenantID),
		ConnectorID:        strings.TrimSpace(connectorID),
		ConnectorAccountID: strings.TrimSpace(connectorAccountID),
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
			strings.Contains(lowerKey, "authorization") ||
			strings.Contains(lowerValue, "secret") ||
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
