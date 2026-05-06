package setupwizard

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

var unsafeIDChars = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sessionID(tenantID, targetID string, style SetupStyle) string {
	return "setup_" + sanitizeID(tenantID) + "_" + sanitizeID(targetID) + "_" + sanitizeID(string(style))
}

func attemptID(sessionID string, operation SetupOperation, at time.Time) string {
	return "attempt_" + sanitizeID(sessionID) + "_" + sanitizeID(string(operation)) + "_" + fmt.Sprintf("%d", at.UnixNano())
}

func oauthStateRef(session SetupSession) string {
	return "oauth_state_" + sanitizeID(session.TenantID) + "_" + sanitizeID(session.SetupSessionID)
}

func sanitizeID(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.ReplaceAll(trimmed, ".", "_")
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	trimmed = unsafeIDChars.ReplaceAllString(trimmed, "_")
	trimmed = strings.Trim(trimmed, "_")
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

func ensureDiagnosticID(session SetupSession) string {
	if session.DiagnosticResultID != "" {
		return session.DiagnosticResultID
	}
	return "diag_" + sanitizeID(session.SetupSessionID)
}

func safeUseForState(session SetupSession) SafeUseMode {
	switch session.State {
	case StateReady:
		return SafeUseNormal
	case StateDegraded:
		if len(session.AllowedCapabilities) > 0 && len(session.DiagnosticAllowedUse) > 0 {
			return SafeUseLimited
		}
		return SafeUseBlocked
	default:
		return SafeUseBlocked
	}
}

func retryableForState(state SetupState) bool {
	switch state {
	case StateActionRequired, StateUnavailable, StateCancelled, StateDisabled, StateInProgress:
		return true
	default:
		return false
	}
}

func retrySafetyForState(state SetupState) RetrySafety {
	switch state {
	case StateReady:
		return RetryNoActionNeeded
	case StateDegraded, StateActionRequired, StateUnavailable, StateCancelled, StateDisabled:
		return RetryRetryable
	default:
		return RetryBlocked
	}
}

func remediationOwnerForState(state SetupState, reason string) RemediationOwner {
	if reason == ReasonProviderUnavailable || reason == ReasonNetworkFailed || reason == ReasonRateLimited {
		return OwnerProvider
	}
	if reason == ReasonRedactionFailedClosed || reason == ReasonUnsupportedTarget {
		return OwnerOperator
	}
	switch state {
	case StateReady:
		return OwnerNoneRequired
	case StateUnavailable:
		return OwnerProvider
	case StateActionRequired:
		return OwnerProductUser
	default:
		return OwnerProductUser
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstRedaction(value RedactionStatus) RedactionStatus {
	if value == "" {
		return RedactionRedacted
	}
	return value
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneSession(in SetupSession) SetupSession {
	in.AllowedCapabilities = append([]string(nil), in.AllowedCapabilities...)
	in.DiagnosticAllowedUse = append([]string(nil), in.DiagnosticAllowedUse...)
	in.ResourceRefs = append([]ResourceRef(nil), in.ResourceRefs...)
	in.RedactedEvidence = cloneStringMap(in.RedactedEvidence)
	return in
}

func cloneAttempt(in SetupAttempt) SetupAttempt {
	in.ResourceRefs = append([]ResourceRef(nil), in.ResourceRefs...)
	in.RedactedEvidence = cloneStringMap(in.RedactedEvidence)
	return in
}

func upsertResourceRef(items []ResourceRef, next ResourceRef) []ResourceRef {
	for i, item := range items {
		if item.Kind == next.Kind && item.ID == next.ID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func contains(items []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}

func auditEventSuffix(operation SetupOperation, state SetupState) string {
	switch operation {
	case OperationStart:
		return "started"
	case OperationSubmitSecret:
		return "secret_submitted"
	case OperationOAuthStart:
		return "oauth_started"
	case OperationOAuthCallback:
		if state == StateReady {
			return "oauth_completed"
		}
		return string(state)
	case OperationRetry:
		return "retried"
	case OperationReplace:
		return "replaced"
	case OperationCancel:
		return "cancelled"
	case OperationDisable:
		return "disabled"
	default:
		return string(state)
	}
}

func auditOutcome(state SetupState, redaction RedactionStatus) string {
	if redaction == RedactionFailedClosed {
		return "failed_closed"
	}
	switch state {
	case StateReady, StateDegraded:
		return "succeeded"
	case StateCancelled:
		return "cancelled"
	default:
		return "blocked"
	}
}

func secretsDisableInput(tenantID, secretRef, reason string) secrets.DisableInput {
	return secrets.DisableInput{TenantID: tenantID, SecretRef: secretRef, DisabledReason: reason}
}
