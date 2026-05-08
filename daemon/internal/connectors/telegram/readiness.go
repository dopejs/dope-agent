package telegram

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type TerminalState string

const (
	TerminalReady          TerminalState = "ready"
	TerminalDegraded       TerminalState = "degraded"
	TerminalUnavailable    TerminalState = "unavailable"
	TerminalCancelled      TerminalState = "cancelled"
	TerminalActionRequired TerminalState = "action-required"
)

type CredentialState string

const (
	CredentialMissing             CredentialState = "missing"
	CredentialSubmitted           CredentialState = "submitted"
	CredentialValid               CredentialState = "valid"
	CredentialInvalid             CredentialState = "invalid"
	CredentialRevoked             CredentialState = "revoked"
	CredentialRedactionSuppressed CredentialState = "redaction_suppressed"
)

type PermissionState string

const (
	PermissionValid               PermissionState = "valid"
	PermissionMissing             PermissionState = "missing_permission"
	PermissionRateLimited         PermissionState = "rate_limited"
	PermissionProviderUnavailable PermissionState = "provider_unavailable"
	PermissionNetworkFailed       PermissionState = "network_failed"
	PermissionUnknown             PermissionState = "unknown"
)

type GroupBehavior string

const (
	GroupBehaviorDisabled                 GroupBehavior = "disabled"
	GroupBehaviorMentionOrCommandRequired GroupBehavior = "mention_or_command_required"
)

type AccountBinding struct {
	TenantID             string                         `json:"tenantId,omitempty"`
	ConnectorID          string                         `json:"connectorId,omitempty"`
	ConnectorAccountID   string                         `json:"connectorAccountId"`
	ProviderAccountLabel string                         `json:"providerAccountLabel,omitempty"`
	PermissionState      PermissionState                `json:"permissionState"`
	ValidatedAt          time.Time                      `json:"validatedAt,omitempty"`
	RedactionStatus      baseconnectors.RedactionStatus `json:"redactionStatus"`
	SafeEvidence         map[string]string              `json:"safeEvidence,omitempty"`
}

type HostedSetupInput struct {
	TenantID         string
	ConnectorID      string
	DisplayName      string
	Credential       CredentialState
	AccountBinding   AccountBinding
	Allowments       []AllowmentValidation
	GroupBehavior    GroupBehavior
	DeliveryEligible bool
	StartedAt        time.Time
	ValidatedAt      time.Time
	Cancelled        bool
}

type HostedSetup struct {
	TenantID           string                         `json:"tenantId,omitempty"`
	ConnectorID        string                         `json:"connectorId"`
	ConnectorKind      string                         `json:"connectorKind"`
	DisplayName        string                         `json:"displayName"`
	Status             baseconnectors.LifecycleState  `json:"status"`
	TerminalState      TerminalState                  `json:"terminalState"`
	HostedReady        bool                           `json:"hostedReady"`
	CredentialState    CredentialState                `json:"credentialState"`
	AllowmentState     AllowmentState                 `json:"allowmentState"`
	GroupBehavior      GroupBehavior                  `json:"groupBehavior"`
	DeliveryEligible   bool                           `json:"deliveryEligible"`
	ReasonCode         string                         `json:"reasonCode,omitempty"`
	AccountBinding     AccountBinding                 `json:"accountBinding,omitempty"`
	Allowments         []AllowmentValidation          `json:"allowments,omitempty"`
	CreatedAt          time.Time                      `json:"createdAt,omitempty"`
	UpdatedAt          time.Time                      `json:"updatedAt,omitempty"`
	ValidatedAt        time.Time                      `json:"validatedAt,omitempty"`
	RedactionStatus    baseconnectors.RedactionStatus `json:"redactionStatus"`
	RetentionExpiresAt time.Time                      `json:"retentionExpiresAt,omitempty"`
}

func EvaluateHostedSetup(input HostedSetupInput) HostedSetup {
	now := input.ValidatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	started := input.StartedAt
	if started.IsZero() {
		started = now
	}
	groupBehavior := input.GroupBehavior
	if groupBehavior == "" {
		groupBehavior = GroupBehaviorDisabled
	}
	allowments := normalizeAllowments(input.TenantID, input.ConnectorID, input.Allowments, now)
	binding := normalizeAccountBinding(input.TenantID, input.ConnectorID, input.AccountBinding, now)
	setup := HostedSetup{
		TenantID:           strings.TrimSpace(input.TenantID),
		ConnectorID:        strings.TrimSpace(input.ConnectorID),
		ConnectorKind:      "telegram",
		DisplayName:        strings.TrimSpace(input.DisplayName),
		Status:             baseconnectors.LifecycleStateHealthy,
		TerminalState:      TerminalReady,
		HostedReady:        true,
		CredentialState:    input.Credential,
		AllowmentState:     allowmentState(allowments),
		GroupBehavior:      groupBehavior,
		DeliveryEligible:   input.DeliveryEligible,
		ReasonCode:         "healthy",
		AccountBinding:     binding,
		Allowments:         allowments,
		CreatedAt:          started,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	if setup.CredentialState == "" {
		setup.CredentialState = CredentialMissing
	}
	if input.Cancelled {
		return setup.notReady(baseconnectors.LifecycleStateDisabled, TerminalCancelled, "user_cancelled")
	}
	if !started.IsZero() && now.Sub(started) > 5*time.Minute && setup.CredentialState == CredentialSubmitted {
		return setup.notReady(baseconnectors.LifecycleStateDegraded, TerminalActionRequired, "telegram_setup_timeout")
	}
	switch setup.CredentialState {
	case CredentialValid:
		// Continue below.
	case CredentialMissing, CredentialSubmitted, CredentialInvalid, CredentialRevoked:
		return setup.notReady(baseconnectors.LifecycleStateFailed, TerminalActionRequired, string(baseconnectors.DiagnosticAuthMissing))
	case CredentialRedactionSuppressed:
		setup.RedactionStatus = baseconnectors.RedactionStatusSuppressed
		return setup.notReady(baseconnectors.LifecycleStateFailed, TerminalActionRequired, string(baseconnectors.DiagnosticUnknownConnectorFailure))
	default:
		return setup.notReady(baseconnectors.LifecycleStateFailed, TerminalActionRequired, string(baseconnectors.DiagnosticUnknownConnectorFailure))
	}
	switch binding.PermissionState {
	case PermissionValid:
	case PermissionMissing:
		return setup.notReady(baseconnectors.LifecycleStatePermissionBlocked, TerminalActionRequired, string(baseconnectors.DiagnosticPermissionMissing))
	case PermissionRateLimited:
		return setup.notReady(baseconnectors.LifecycleStateRateLimited, TerminalDegraded, string(baseconnectors.DiagnosticRateLimited))
	case PermissionProviderUnavailable:
		return setup.notReady(baseconnectors.LifecycleStateDegraded, TerminalUnavailable, string(baseconnectors.DiagnosticProviderUnavailable))
	case PermissionNetworkFailed:
		return setup.notReady(baseconnectors.LifecycleStateDegraded, TerminalUnavailable, string(baseconnectors.DiagnosticNetworkFailed))
	default:
		return setup.notReady(baseconnectors.LifecycleStateDegraded, TerminalActionRequired, string(baseconnectors.DiagnosticUnknownConnectorFailure))
	}
	if !hasValidAllowment(allowments) {
		return setup.notReady(baseconnectors.LifecycleStateDegraded, TerminalActionRequired, "telegram_allowment_missing")
	}
	setup.DeliveryEligible = true
	return setup
}

func (setup HostedSetup) notReady(status baseconnectors.LifecycleState, terminal TerminalState, reason string) HostedSetup {
	setup.Status = status
	setup.TerminalState = terminal
	setup.HostedReady = false
	setup.DeliveryEligible = false
	setup.ReasonCode = reason
	return setup
}

func normalizeAccountBinding(tenantID, connectorID string, binding AccountBinding, now time.Time) AccountBinding {
	binding.TenantID = firstNonEmpty(binding.TenantID, tenantID)
	binding.ConnectorID = firstNonEmpty(binding.ConnectorID, connectorID)
	if binding.PermissionState == "" {
		binding.PermissionState = PermissionUnknown
	}
	if binding.ValidatedAt.IsZero() {
		binding.ValidatedAt = now
	}
	if binding.RedactionStatus == "" {
		binding.RedactionStatus = baseconnectors.RedactionStatusRedacted
	}
	return binding
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
