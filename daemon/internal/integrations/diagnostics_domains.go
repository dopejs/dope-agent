package integrations

import "strings"

const FeishuLarkProviderKind = "feishu_lark"

func DiagnosticProjectionReason(resource Resource) DiagnosticReasonCode {
	providerKind := strings.TrimSpace(string(resource.BackendBinding.BackendKind))
	if strings.EqualFold(providerKind, FeishuLarkProviderKind) || strings.Contains(strings.ToLower(providerKind), "feishu") || strings.Contains(strings.ToLower(providerKind), "lark") {
		return readinessReason(resource)
	}
	if supportsLimitedDiagnostic(resource) {
		return ReasonLimitedDiagnostic
	}
	return ReasonUnsupportedDiagnostic
}

func supportsLimitedDiagnostic(resource Resource) bool {
	if resource.BackendBinding.SupportsProbeRead {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(resource.DomainKind)) {
	case "calendar", "mail", "reminders", "delivery":
		return true
	default:
		return false
	}
}

func readinessReason(resource Resource) DiagnosticReasonCode {
	switch resource.ReadinessStatus {
	case ReadinessStatusHealthy:
		return ReasonHealthy
	case ReadinessStatusAuthPending:
		if resource.AuthState == AuthStatePending || resource.AuthState == AuthStateNotStarted {
			return ReasonUserAuthorizationMissing
		}
		return ReasonAppAuthorizationMissing
	case ReadinessStatusNotConfigured:
		return ReasonAppAuthorizationMissing
	case ReadinessStatusUnavailable:
		switch resource.AuthState {
		case AuthStateExpired:
			return ReasonTokenExpired
		case AuthStateRevoked:
			return ReasonTokenRevoked
		case AuthStateNotStarted, AuthStatePending:
			return ReasonUserAuthorizationMissing
		}
		if resource.HealthState == HealthStateUnavailable {
			return ReasonProviderUnavailable
		}
		return ReasonUnknownProviderError
	case ReadinessStatusDegraded:
		reason := strings.ToLower(resource.ReadinessReason + " " + resource.RequiredOperatorAction + " " + resource.DisabledReason)
		switch {
		case strings.Contains(reason, "scope"):
			return ReasonScopeMissing
		case strings.Contains(reason, "tenant") && strings.Contains(reason, "approval"):
			return ReasonTenantApprovalPending
		case strings.Contains(reason, "rate"):
			return ReasonRateLimited
		case strings.Contains(reason, "network"):
			return ReasonNetworkFailed
		case strings.Contains(reason, "refresh"):
			return ReasonTokenRefreshFailed
		case strings.Contains(reason, "expired"):
			return ReasonTokenExpired
		case strings.Contains(reason, "revoked"):
			return ReasonTokenRevoked
		case strings.Contains(reason, "auth"):
			return ReasonUserAuthorizationMissing
		default:
			return ReasonUnknownProviderError
		}
	default:
		return ReasonUnknownProviderError
	}
}
