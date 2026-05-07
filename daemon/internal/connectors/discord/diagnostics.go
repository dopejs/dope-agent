package discord

import (
	"errors"
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type discordClassifiedError interface {
	ErrorClass() string
}

func DiagnosticReasonForError(err error) baseconnectors.DiagnosticReasonCode {
	if err == nil {
		return ""
	}
	var classified discordClassifiedError
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
		}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "401"),
		strings.Contains(message, "unauthorized"),
		strings.Contains(message, "token"),
		strings.Contains(message, "invalid session"):
		return baseconnectors.DiagnosticAuthMissing
	case strings.Contains(message, "403"),
		strings.Contains(message, "forbidden"),
		strings.Contains(message, "permission"),
		strings.Contains(message, "message content"):
		return baseconnectors.DiagnosticPermissionMissing
	case strings.Contains(message, "429"),
		strings.Contains(message, "rate limit"):
		return baseconnectors.DiagnosticRateLimited
	case strings.Contains(message, "gateway"),
		strings.Contains(message, "network"),
		strings.Contains(message, "connection"):
		return baseconnectors.DiagnosticNetworkFailed
	case strings.Contains(message, "unavailable"),
		strings.Contains(message, "5xx"):
		return baseconnectors.DiagnosticProviderUnavailable
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
		RedactionReliable:  true,
		SafeEvidence:       evidence,
	})
}
