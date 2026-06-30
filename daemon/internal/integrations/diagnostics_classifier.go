package integrations

import (
	"fmt"
	"strings"
	"time"
)

type ProviderDiagnosticEvidence struct {
	ProviderKind         string
	DomainKind           string
	IntegrationID        string
	OperationClass       string
	ProviderErrorClass   string
	ProviderStatusCode   string
	RedactedProviderCode string
	Message              string
	RedactionConfidence  string
	SideEffecting        bool
	CommitAmbiguous      bool
	CreatedAt            time.Time
}

func ClassifyProviderEvidence(evidence ProviderDiagnosticEvidence) ProviderErrorClassification {
	now := evidence.CreatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	reason := classifyReason(evidence)
	_, owner, retrySafety := DiagnosticDefaults(reason)
	redactionStatus := RedactionStatusRedacted
	ambiguous := false
	if strings.EqualFold(evidence.RedactionConfidence, "uncertain") {
		reason = ReasonRedactionFailedClosed
		_, owner, retrySafety = DiagnosticDefaults(reason)
		redactionStatus = RedactionStatusFailedClosed
	}
	if reason == ReasonUnknownProviderError || reason == ReasonAmbiguousDownstreamCommit {
		ambiguous = true
	}
	return ProviderErrorClassification{
		ClassificationID:     diagnosticID("diag_class", evidence.ProviderKind, evidence.DomainKind, evidence.IntegrationID, evidence.OperationClass, evidence.ProviderErrorClass, evidence.ProviderStatusCode, string(reason), now.Format(time.RFC3339Nano)),
		TenantID:             "",
		ProviderKind:         firstNonEmptyString(evidence.ProviderKind, "unknown"),
		DomainKind:           strings.TrimSpace(evidence.DomainKind),
		IntegrationID:        strings.TrimSpace(evidence.IntegrationID),
		OperationClass:       strings.TrimSpace(evidence.OperationClass),
		ProviderErrorClass:   strings.TrimSpace(evidence.ProviderErrorClass),
		ProviderStatusCode:   strings.TrimSpace(evidence.ProviderStatusCode),
		RedactedProviderCode: strings.TrimSpace(evidence.RedactedProviderCode),
		ReasonCode:           reason,
		RetrySafety:          retrySafety,
		RemediationOwner:     owner,
		EvidenceConfidence:   "high",
		Ambiguous:            ambiguous,
		RedactionStatus:      redactionStatus,
		CreatedAt:            now,
	}
}

func ClassifyFakeBackendFault(faultType FakeFaultType, domainKind string) ProviderErrorClassification {
	reason := ReasonHealthy
	switch faultType {
	case FakeFaultTransient5xx:
		reason = ReasonTransientProviderFailure
	case FakeFaultRateLimit:
		reason = ReasonRateLimited
	case FakeFaultAuthExpiry:
		reason = ReasonTokenExpired
	case FakeFaultProviderUnavailable:
		reason = ReasonProviderUnavailable
	case FakeFaultSlowResponse:
		reason = ReasonNetworkFailed
	case FakeFaultMalformedResponse:
		reason = ReasonUnknownProviderError
	}
	return ClassifyProviderEvidence(ProviderDiagnosticEvidence{
		ProviderKind:         string(BackendKindFakeLocal),
		DomainKind:           domainKind,
		ProviderErrorClass:   string(faultType),
		RedactedProviderCode: string(faultType),
		Message:              string(reason),
	})
}

func classifyReason(evidence ProviderDiagnosticEvidence) DiagnosticReasonCode {
	combined := strings.ToLower(strings.Join(strings.Fields(strings.Join([]string{
		evidence.ProviderErrorClass,
		evidence.ProviderStatusCode,
		evidence.RedactedProviderCode,
		evidence.Message,
	}, " ")), " "))
	if strings.Contains(strings.ToLower(evidence.ProviderKind), "feishu") || strings.Contains(strings.ToLower(evidence.ProviderKind), "lark") {
		if reason, ok := classifyFeishuLarkReason(combined); ok {
			return reason
		}
	}
	if evidence.CommitAmbiguous {
		return ReasonAmbiguousDownstreamCommit
	}
	if evidence.SideEffecting && strings.Contains(combined, "send_permission_required") {
		return ReasonUnsafeToRetry
	}
	if evidence.SideEffecting && strings.Contains(combined, "retry") && strings.Contains(combined, "unsafe") {
		return ReasonUnsafeToRetry
	}
	switch {
	case combined == "" || combined == "ok" || combined == "healthy" || strings.Contains(combined, "status:ok"):
		return ReasonHealthy
	case strings.Contains(combined, "ambiguous") && (strings.Contains(combined, "permission") || strings.Contains(combined, "authorization") || strings.Contains(combined, "scope")):
		return ReasonUnknownProviderError
	case strings.Contains(combined, "transient"):
		return ReasonTransientProviderFailure
	case strings.Contains(combined, "app_auth") || strings.Contains(combined, "app authorization"):
		return ReasonAppAuthorizationMissing
	case strings.Contains(combined, "bot_auth") || strings.Contains(combined, "bot authorization"):
		return ReasonBotAuthorizationMissing
	case strings.Contains(combined, "user_auth") || strings.Contains(combined, "user authorization") || strings.Contains(combined, "auth missing"):
		return ReasonUserAuthorizationMissing
	case strings.Contains(combined, "tenant") && strings.Contains(combined, "approval"):
		return ReasonTenantApprovalPending
	case strings.Contains(combined, "scope"):
		return ReasonScopeMissing
	case strings.Contains(combined, "refresh") && strings.Contains(combined, "missing"):
		return ReasonRefreshCredentialsMissing
	case strings.Contains(combined, "refresh"):
		return ReasonTokenRefreshFailed
	case strings.Contains(combined, "token") && strings.Contains(combined, "missing"):
		return ReasonTokenMissing
	case strings.Contains(combined, "expired") || strings.Contains(combined, "expiry") || strings.Contains(combined, "auth_expiry"):
		return ReasonTokenExpired
	case strings.Contains(combined, "revoked"):
		return ReasonTokenRevoked
	case strings.Contains(combined, "tenant_mismatch"):
		return ReasonTenantMismatch
	case strings.Contains(combined, "rate") || strings.Contains(combined, "429"):
		return ReasonRateLimited
	case strings.Contains(combined, "network") || strings.Contains(combined, "timeout") || strings.Contains(combined, "slow_response"):
		return ReasonNetworkFailed
	case strings.Contains(combined, "5xx") || strings.Contains(combined, "unavailable"):
		return ReasonProviderUnavailable
	case strings.Contains(combined, "operator_action"):
		return ReasonOperatorActionNeeded
	case strings.Contains(combined, "unsupported"):
		// Provider does not support a requested capability (e.g. RSVP inspection or attendee
		// notification control, Roadmap 61 FR-005) — surface it explicitly, not as a silent drop.
		return ReasonUnsupportedDiagnostic
	default:
		return ReasonUnknownProviderError
	}
}

func classifyFeishuLarkReason(combined string) (DiagnosticReasonCode, bool) {
	switch {
	case strings.Contains(combined, "99991663") || strings.Contains(combined, "tenant_access_token_invalid") && strings.Contains(combined, "approval"):
		return ReasonTenantApprovalPending, true
	case strings.Contains(combined, "99991664") || strings.Contains(combined, "app_access_token_invalid") || strings.Contains(combined, "app_ticket_invalid"):
		return ReasonAppAuthorizationMissing, true
	case strings.Contains(combined, "99991665") || strings.Contains(combined, "bot_not_installed") || strings.Contains(combined, "bot_auth"):
		return ReasonBotAuthorizationMissing, true
	case strings.Contains(combined, "99991668") || strings.Contains(combined, "user_access_token_invalid") || strings.Contains(combined, "user_auth"):
		return ReasonUserAuthorizationMissing, true
	case strings.Contains(combined, "99991669") || strings.Contains(combined, "scope_not_granted") || strings.Contains(combined, "missing_scope"):
		return ReasonScopeMissing, true
	case strings.Contains(combined, "refresh_token_missing") || strings.Contains(combined, "refresh_credentials_missing"):
		return ReasonRefreshCredentialsMissing, true
	case strings.Contains(combined, "refresh_token_failed") || strings.Contains(combined, "refresh_failed"):
		return ReasonTokenRefreshFailed, true
	case strings.Contains(combined, "token_missing") || strings.Contains(combined, "access_token_missing"):
		return ReasonTokenMissing, true
	case strings.Contains(combined, "token_expired") || strings.Contains(combined, "access_token_expired"):
		return ReasonTokenExpired, true
	case strings.Contains(combined, "token_revoked") || strings.Contains(combined, "user_token_revoked"):
		return ReasonTokenRevoked, true
	case strings.Contains(combined, "tenant_mismatch"):
		return ReasonTenantMismatch, true
	case strings.Contains(combined, "rate_limited") || strings.Contains(combined, "too_many_requests"):
		return ReasonRateLimited, true
	case strings.Contains(combined, "provider_unavailable") || strings.Contains(combined, "service_unavailable"):
		return ReasonProviderUnavailable, true
	case strings.Contains(combined, "network_failed") || strings.Contains(combined, "dns_failed") || strings.Contains(combined, "connect_timeout"):
		return ReasonNetworkFailed, true
	case strings.Contains(combined, "transient_provider_failure") || strings.Contains(combined, "temporary_failure"):
		return ReasonTransientProviderFailure, true
	default:
		return "", false
	}
}

func DiagnosticFailureForOperationFailure(domainKind, providerKind, integrationID, operationClass, failureClass, reason string, sideEffecting bool, checkedAt time.Time) DiagnosticFailureProjection {
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	classification := ClassifyProviderEvidence(ProviderDiagnosticEvidence{
		ProviderKind:       providerKind,
		DomainKind:         domainKind,
		IntegrationID:      integrationID,
		OperationClass:     operationClass,
		ProviderErrorClass: failureClass,
		Message:            reason,
		SideEffecting:      sideEffecting,
		CreatedAt:          checkedAt,
	})
	return DiagnosticFailureForReason(classification.ReasonCode, checkedAt)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ProviderEvidenceFromMap(providerKind, domainKind string, evidence map[string]any) ProviderDiagnosticEvidence {
	item := ProviderDiagnosticEvidence{ProviderKind: providerKind, DomainKind: domainKind}
	for key, value := range evidence {
		text := strings.TrimSpace(fmt.Sprint(value))
		switch key {
		case "code":
			item.RedactedProviderCode = text
		case "status", "statusClass":
			item.ProviderStatusCode = text
		case "errorClass":
			item.ProviderErrorClass = text
		case "message":
			item.Message += " " + text
		case "approval":
			item.Message += " approval " + text
		case "redactionConfidence":
			item.RedactionConfidence = text
		case "commitAmbiguous":
			item.CommitAmbiguous = text == "true"
		}
	}
	return item
}
