package integrations

import "time"

func DiagnosticRemediationHint(reason DiagnosticReasonCode) string {
	switch reason {
	case ReasonHealthy:
		return "No operator action is required."
	case ReasonAppAuthorizationMissing, ReasonBotAuthorizationMissing:
		return "Reconnect the provider application or bot credentials."
	case ReasonUserAuthorizationMissing, ReasonTokenMissing, ReasonTokenExpired, ReasonTokenRevoked:
		return "Ask the affected user to reauthorize the integration account."
	case ReasonTenantApprovalPending:
		return "Ask a tenant administrator to approve the provider application."
	case ReasonScopeMissing:
		return "Ask a tenant administrator to grant the missing provider scope."
	case ReasonRefreshCredentialsMissing, ReasonTokenRefreshFailed, ReasonTenantMismatch:
		return "Review integration credential binding and reconnect the account if needed."
	case ReasonRateLimited:
		return "Wait for the provider quota window to recover before retrying."
	case ReasonProviderUnavailable, ReasonTransientProviderFailure:
		return "Retry after provider health recovers."
	case ReasonNetworkFailed:
		return "Check local network reachability from the daemon environment."
	case ReasonAmbiguousDownstreamCommit, ReasonUnsafeToRetry:
		return "Do not retry automatically; review downstream commit evidence."
	case ReasonOperatorActionNeeded:
		return "An operator must inspect the integration before retrying."
	case ReasonLimitedDiagnostic:
		return "Only limited diagnostic dimensions are available for this domain."
	case ReasonUnsupportedDiagnostic:
		return "Diagnostics are not yet supported for this domain."
	case ReasonRedactionFailedClosed:
		return "Diagnostic evidence was suppressed because redaction could not be proven."
	default:
		return "Inspect provider evidence and integration configuration."
	}
}

func DiagnosticDefaults(reason DiagnosticReasonCode) (DiagnosticStatus, RemediationOwner, RetrySafety) {
	for _, definition := range DefaultDiagnosticReasonCodeCatalog() {
		if definition.ReasonCode == reason {
			status := DiagnosticStatusBlocked
			switch reason {
			case ReasonHealthy:
				status = DiagnosticStatusHealthy
			case ReasonLimitedDiagnostic:
				status = DiagnosticStatusDegraded
			case ReasonUnsupportedDiagnostic:
				status = DiagnosticStatusUnsupported
			case ReasonProviderUnavailable, ReasonTransientProviderFailure, ReasonRateLimited, ReasonNetworkFailed:
				status = DiagnosticStatusDegraded
			case ReasonUnknownProviderError, ReasonOperatorActionNeeded, ReasonRedactionFailedClosed:
				status = DiagnosticStatusUnknown
			}
			return status, definition.DefaultRemediationOwner, definition.DefaultRetrySafety
		}
	}
	return DiagnosticStatusUnknown, RemediationOwnerOperator, RetrySafetyOperatorActionNeeded
}

func DiagnosticFailureFromResult(result DiagnosticResult) DiagnosticFailureProjection {
	return DiagnosticFailureProjection{
		ReasonCode:       result.ReasonCode,
		RemediationOwner: result.RemediationOwner,
		RemediationHint:  result.RemediationHint,
		RetrySafety:      result.RetrySafety,
		FreshnessState:   result.FreshnessState,
		CheckedAt:        result.CheckedAt,
		RedactionStatus:  result.RedactionStatus,
	}
}

func DiagnosticFailureForReason(reason DiagnosticReasonCode, checkedAt time.Time) DiagnosticFailureProjection {
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	_, owner, retrySafety := DiagnosticDefaults(reason)
	return DiagnosticFailureProjection{
		ReasonCode:       reason,
		RemediationOwner: owner,
		RemediationHint:  DiagnosticRemediationHint(reason),
		RetrySafety:      retrySafety,
		FreshnessState:   FreshnessStateFresh,
		CheckedAt:        checkedAt.UTC(),
		RedactionStatus:  RedactionStatusRedacted,
	}
}
