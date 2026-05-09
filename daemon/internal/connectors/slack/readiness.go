package slack

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

type OAuthState string

const (
	OAuthNotStarted          OAuthState = "not_started"
	OAuthStarted             OAuthState = "started"
	OAuthCallbackReceived    OAuthState = "callback_received"
	OAuthGrantValid          OAuthState = "grant_valid"
	OAuthGrantMissing        OAuthState = "grant_missing"
	OAuthScopeMissing        OAuthState = "scope_missing"
	OAuthApprovalRequired    OAuthState = "approval_required"
	OAuthRevoked             OAuthState = "revoked"
	OAuthRedactionSuppressed OAuthState = "redaction_suppressed"
)

type RoutePolicyState string

const (
	RoutePolicyStateNone    RoutePolicyState = "none"
	RoutePolicyStatePartial RoutePolicyState = "partial"
	RoutePolicyStateValid   RoutePolicyState = "valid"
	RoutePolicyStateStale   RoutePolicyState = "stale"
)

type WorkspaceBinding struct {
	TenantID           string                         `json:"tenantId,omitempty"`
	ConnectorID        string                         `json:"connectorId,omitempty"`
	WorkspaceBindingID string                         `json:"workspaceBindingId,omitempty"`
	WorkspaceID        string                         `json:"workspaceId"`
	WorkspaceLabel     string                         `json:"workspaceLabel,omitempty"`
	InstallationID     string                         `json:"installationId"`
	OAuthGrantState    string                         `json:"oauthGrantState"`
	RequiredScopeState string                         `json:"requiredScopeState"`
	ValidatedAt        time.Time                      `json:"validatedAt"`
	RedactionStatus    baseconnectors.RedactionStatus `json:"redactionStatus"`
	SafeEvidence       map[string]string              `json:"safeEvidence,omitempty"`
}

type HostedSetupInput struct {
	TenantID            string
	ConnectorID         string
	DisplayName         string
	WorkspaceBinding    WorkspaceBinding
	ExpectedWorkspaceID string
	OAuthState          OAuthState
	RoutePolicy         RoutePolicy
	ProviderAvailable   bool
	NetworkAvailable    bool
	Cancelled           bool
	RedactionReliable   bool
	RedactionSuppressed bool
	StartedAt           time.Time
	SetupTimeout        time.Duration
	ValidatedAt         time.Time
}

type HostedSetup struct {
	TenantID           string                         `json:"tenantId,omitempty"`
	ConnectorID        string                         `json:"connectorId"`
	ConnectorKind      string                         `json:"connectorKind"`
	DisplayName        string                         `json:"displayName"`
	Status             baseconnectors.LifecycleState  `json:"status"`
	TerminalState      TerminalState                  `json:"terminalState"`
	OAuthState         OAuthState                     `json:"oauthState"`
	RoutePolicyState   RoutePolicyState               `json:"routePolicyState"`
	DeliveryEligible   bool                           `json:"deliveryEligible"`
	WorkspaceBindingID string                         `json:"workspaceBindingId"`
	ReasonCode         string                         `json:"reasonCode,omitempty"`
	WorkspaceBinding   WorkspaceBinding               `json:"workspaceBinding,omitempty"`
	RoutePolicy        RoutePolicy                    `json:"routePolicy,omitempty"`
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
	policy := NormalizeRoutePolicy(input.RoutePolicy, now)
	binding := normalizeWorkspaceBinding(input.TenantID, input.ConnectorID, input.WorkspaceBinding, now)
	setup := HostedSetup{
		TenantID:           strings.TrimSpace(input.TenantID),
		ConnectorID:        strings.TrimSpace(input.ConnectorID),
		ConnectorKind:      "slack",
		DisplayName:        strings.TrimSpace(input.DisplayName),
		Status:             baseconnectors.LifecycleStateDegraded,
		TerminalState:      TerminalActionRequired,
		OAuthState:         input.OAuthState,
		RoutePolicyState:   RoutePolicyStateNone,
		WorkspaceBindingID: binding.WorkspaceBindingID,
		WorkspaceBinding:   binding,
		RoutePolicy:        policy,
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	if setup.OAuthState == "" {
		setup.OAuthState = OAuthGrantMissing
	}
	timeout := input.SetupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if input.RedactionSuppressed || setup.OAuthState == OAuthRedactionSuppressed {
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.TerminalState = TerminalActionRequired
		setup.OAuthState = OAuthRedactionSuppressed
		setup.RedactionStatus = baseconnectors.RedactionStatusSuppressed
		setup.ReasonCode = string(baseconnectors.DiagnosticUnknownConnectorFailure)
		return setup
	}
	if input.Cancelled {
		setup.Status = baseconnectors.LifecycleStateDisabled
		setup.TerminalState = TerminalCancelled
		setup.ReasonCode = "user_cancelled"
		return setup
	}
	if !input.StartedAt.IsZero() && now.Sub(input.StartedAt) > timeout {
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.TerminalState = TerminalUnavailable
		setup.ReasonCode = "setup_timeout"
		return setup
	}
	if setup.OAuthState != OAuthGrantValid {
		setup.ReasonCode = reasonForOAuthState(setup.OAuthState)
		return setup
	}
	if !input.ProviderAvailable {
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.TerminalState = TerminalUnavailable
		setup.ReasonCode = string(baseconnectors.DiagnosticProviderUnavailable)
		return setup
	}
	if !input.NetworkAvailable {
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.TerminalState = TerminalUnavailable
		setup.ReasonCode = string(baseconnectors.DiagnosticNetworkFailed)
		return setup
	}
	if strings.TrimSpace(input.ExpectedWorkspaceID) != "" &&
		strings.TrimSpace(binding.WorkspaceID) != "" &&
		strings.TrimSpace(input.ExpectedWorkspaceID) != strings.TrimSpace(binding.WorkspaceID) {
		setup.ReasonCode = "workspace_mismatch"
		return setup
	}
	if !workspaceBindingReady(binding) {
		setup.ReasonCode = reasonForOAuthState(setup.OAuthState)
		return setup
	}
	if HasReadyRoutePolicy(policy) {
		setup.Status = baseconnectors.LifecycleStateHealthy
		setup.TerminalState = TerminalReady
		setup.RoutePolicyState = RoutePolicyStateValid
		setup.DeliveryEligible = true
		setup.ReasonCode = "healthy"
		return setup
	}
	setup.RoutePolicyState = RoutePolicyStateNone
	setup.ReasonCode = string(baseconnectors.DiagnosticBlockedRoute)
	return setup
}

func normalizeWorkspaceBinding(tenantID, connectorID string, binding WorkspaceBinding, now time.Time) WorkspaceBinding {
	binding.TenantID = firstNonEmpty(strings.TrimSpace(binding.TenantID), strings.TrimSpace(tenantID))
	binding.ConnectorID = firstNonEmpty(strings.TrimSpace(binding.ConnectorID), strings.TrimSpace(connectorID))
	if binding.WorkspaceBindingID == "" && binding.ConnectorID != "" {
		binding.WorkspaceBindingID = "slack_workspace_" + binding.ConnectorID
	}
	binding.OAuthGrantState = firstNonEmpty(strings.TrimSpace(binding.OAuthGrantState), "missing")
	binding.RequiredScopeState = firstNonEmpty(strings.TrimSpace(binding.RequiredScopeState), "unknown")
	if binding.ValidatedAt.IsZero() {
		binding.ValidatedAt = now
	}
	if binding.RedactionStatus == "" {
		binding.RedactionStatus = baseconnectors.RedactionStatusRedacted
	}
	return binding
}

func workspaceBindingReady(binding WorkspaceBinding) bool {
	return strings.TrimSpace(binding.WorkspaceID) != "" &&
		strings.TrimSpace(binding.InstallationID) != "" &&
		binding.OAuthGrantState == "valid" &&
		binding.RequiredScopeState == "valid"
}

func reasonForOAuthState(state OAuthState) string {
	switch state {
	case OAuthGrantValid:
		return string(baseconnectors.DiagnosticBlockedRoute)
	case OAuthScopeMissing:
		return string(baseconnectors.DiagnosticPermissionMissing)
	case OAuthApprovalRequired:
		return string(baseconnectors.DiagnosticPermissionMissing)
	case OAuthRevoked, OAuthGrantMissing, OAuthNotStarted, OAuthStarted, OAuthCallbackReceived:
		return string(baseconnectors.DiagnosticAuthMissing)
	default:
		return string(baseconnectors.DiagnosticUnknownConnectorFailure)
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
