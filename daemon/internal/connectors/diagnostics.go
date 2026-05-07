package connectors

import (
	"errors"
	"strings"
	"time"
)

type DiagnosticReasonCode string

const (
	DiagnosticAuthMissing             DiagnosticReasonCode = "auth_missing"
	DiagnosticPermissionMissing       DiagnosticReasonCode = "permission_missing"
	DiagnosticRateLimited             DiagnosticReasonCode = "rate_limited"
	DiagnosticProviderUnavailable     DiagnosticReasonCode = "provider_unavailable"
	DiagnosticNetworkFailed           DiagnosticReasonCode = "network_failed"
	DiagnosticUnsupportedCapability   DiagnosticReasonCode = "unsupported_capability"
	DiagnosticBlockedRoute            DiagnosticReasonCode = "blocked_route"
	DiagnosticDuplicateInbound        DiagnosticReasonCode = "duplicate_inbound"
	DiagnosticReplyFailed             DiagnosticReasonCode = "reply_failed"
	DiagnosticUnknownConnectorFailure DiagnosticReasonCode = "unknown_connector_failure"
)

type RemediationOwner string

const (
	RemediationOwnerUser         RemediationOwner = "product_user"
	RemediationOwnerAdmin        RemediationOwner = "tenant_admin"
	RemediationOwnerOperator     RemediationOwner = "operator"
	RemediationOwnerProvider     RemediationOwner = "provider"
	RemediationOwnerNoneRequired RemediationOwner = "none_required"
	RemediationOwnerNone         RemediationOwner = RemediationOwnerNoneRequired
)

type RetrySafety string

const (
	RetrySafetyNoActionNeeded RetrySafety = "no_action_needed"
	RetrySafetyRetryable      RetrySafety = "retryable"
	RetrySafetyRetryAfter     RetrySafety = "retry_after"
	RetrySafetyBlocked        RetrySafety = "blocked"
	RetrySafetyUnsafe         RetrySafety = "unsafe"
)

type FreshnessState string

const (
	FreshnessFresh FreshnessState = "fresh"
	FreshnessStale FreshnessState = "stale"
)

type ConnectorDiagnosticState struct {
	DiagnosticStateID   string               `json:"diagnosticStateId"`
	TenantID            string               `json:"tenantId,omitempty"`
	ConnectorID         string               `json:"connectorId"`
	ConnectorAccountID  string               `json:"connectorAccountId,omitempty"`
	Status              LifecycleState       `json:"status"`
	ReasonCode          DiagnosticReasonCode `json:"reasonCode"`
	RemediationOwner    RemediationOwner     `json:"remediationOwner"`
	UserVisibleSeverity string               `json:"userVisibleSeverity"`
	RetrySafety         RetrySafety          `json:"retrySafety"`
	EvidenceTimestamp   time.Time            `json:"evidenceTimestamp"`
	FreshnessState      FreshnessState       `json:"freshnessState"`
	RedactionStatus     RedactionStatus      `json:"redactionStatus"`
	RetentionExpiresAt  time.Time            `json:"retentionExpiresAt"`
	SafeEvidence        map[string]string    `json:"safeEvidence,omitempty"`
	RedactionFailureID  string               `json:"redactionFailureId,omitempty"`
}

type DiagnosticInput struct {
	DiagnosticStateID  string
	TenantID           string
	ConnectorID        string
	ConnectorAccountID string
	ReasonCode         DiagnosticReasonCode
	EvidenceTimestamp  time.Time
	RedactionReliable  bool
	SafeEvidence       map[string]string
}

var (
	ErrDiagnosticConnectorRequired = errors.New("connector id is required")
	ErrDiagnosticReasonRequired    = errors.New("diagnostic reason code is required")
)

func ClassifyDiagnostic(input DiagnosticInput) (ConnectorDiagnosticState, error) {
	if strings.TrimSpace(input.ConnectorID) == "" {
		return ConnectorDiagnosticState{}, ErrDiagnosticConnectorRequired
	}
	if input.ReasonCode == "" {
		return ConnectorDiagnosticState{}, ErrDiagnosticReasonRequired
	}
	now := input.EvidenceTimestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	redaction := RedactionStatusRedacted
	evidence := input.SafeEvidence
	redactionFailureID := ""
	if !input.RedactionReliable {
		redaction = RedactionStatusSuppressed
		evidence = nil
		redactionFailureID = "redaction_failed_" + input.ConnectorID
	}
	id := input.DiagnosticStateID
	if strings.TrimSpace(id) == "" {
		id = "diag_" + input.ConnectorID + "_" + string(input.ReasonCode)
	}
	return ConnectorDiagnosticState{
		DiagnosticStateID:   id,
		TenantID:            input.TenantID,
		ConnectorID:         input.ConnectorID,
		ConnectorAccountID:  input.ConnectorAccountID,
		Status:              statusForDiagnostic(input.ReasonCode),
		ReasonCode:          input.ReasonCode,
		RemediationOwner:    remediationForDiagnostic(input.ReasonCode),
		UserVisibleSeverity: severityForDiagnostic(input.ReasonCode),
		RetrySafety:         retrySafetyForDiagnostic(input.ReasonCode),
		EvidenceTimestamp:   now,
		FreshnessState:      FreshnessAt(now, now),
		RedactionStatus:     redaction,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		SafeEvidence:        evidence,
		RedactionFailureID:  redactionFailureID,
	}, nil
}

func FreshnessAt(evidenceTimestamp, now time.Time) FreshnessState {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if evidenceTimestamp.IsZero() || now.Sub(evidenceTimestamp) > 15*time.Minute {
		return FreshnessStale
	}
	return FreshnessFresh
}

func CurrentDiagnosticTruth(input DiagnosticInput, failureTime time.Time) (ConnectorDiagnosticState, error) {
	input.EvidenceTimestamp = failureTime
	return ClassifyDiagnostic(input)
}

func statusForDiagnostic(reason DiagnosticReasonCode) LifecycleState {
	switch reason {
	case DiagnosticAuthMissing:
		return LifecycleStateFailed
	case DiagnosticPermissionMissing:
		return LifecycleStatePermissionBlocked
	case DiagnosticRateLimited:
		return LifecycleStateRateLimited
	case DiagnosticProviderUnavailable, DiagnosticNetworkFailed:
		return LifecycleStateDegraded
	case DiagnosticUnsupportedCapability:
		return LifecycleStateUnsupportedCapability
	case DiagnosticBlockedRoute, DiagnosticDuplicateInbound:
		return LifecycleStateDegraded
	case DiagnosticReplyFailed, DiagnosticUnknownConnectorFailure:
		return LifecycleStateFailed
	default:
		return LifecycleStateFailed
	}
}

func remediationForDiagnostic(reason DiagnosticReasonCode) RemediationOwner {
	switch reason {
	case DiagnosticAuthMissing:
		return RemediationOwnerUser
	case DiagnosticPermissionMissing, DiagnosticBlockedRoute:
		return RemediationOwnerAdmin
	case DiagnosticRateLimited, DiagnosticProviderUnavailable:
		return RemediationOwnerProvider
	case DiagnosticNetworkFailed, DiagnosticReplyFailed, DiagnosticUnknownConnectorFailure:
		return RemediationOwnerOperator
	case DiagnosticUnsupportedCapability, DiagnosticDuplicateInbound:
		return RemediationOwnerNoneRequired
	default:
		return RemediationOwnerOperator
	}
}

func retrySafetyForDiagnostic(reason DiagnosticReasonCode) RetrySafety {
	switch reason {
	case DiagnosticRateLimited:
		return RetrySafetyRetryAfter
	case DiagnosticProviderUnavailable, DiagnosticNetworkFailed:
		return RetrySafetyRetryable
	case DiagnosticAuthMissing, DiagnosticPermissionMissing, DiagnosticBlockedRoute:
		return RetrySafetyBlocked
	case DiagnosticDuplicateInbound, DiagnosticUnsupportedCapability:
		return RetrySafetyNoActionNeeded
	default:
		return RetrySafetyUnsafe
	}
}

func severityForDiagnostic(reason DiagnosticReasonCode) string {
	switch reason {
	case DiagnosticDuplicateInbound, DiagnosticUnsupportedCapability:
		return "info"
	case DiagnosticBlockedRoute, DiagnosticRateLimited:
		return "warning"
	default:
		return "error"
	}
}
