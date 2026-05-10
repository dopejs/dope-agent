package matrix

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func EvaluateHostedSetup(input HostedSetupInput) HostedSetup {
	now := input.ValidatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	timeout := input.SetupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	binding := NormalizeHomeserverBinding(input.TenantID, input.ConnectorID, input.HomeserverBinding)
	policy := NormalizeRoutePolicy(input.RoutePolicy, now)
	setup := HostedSetup{
		TenantID:             strings.TrimSpace(input.TenantID),
		ConnectorID:          strings.TrimSpace(input.ConnectorID),
		ConnectorKind:        ConnectorKind,
		DisplayName:          strings.TrimSpace(input.DisplayName),
		Status:               baseconnectors.LifecycleStateDegraded,
		TerminalState:        TerminalActionRequired,
		BotCredentialState:   input.BotCredentialState,
		HomeserverState:      homeserverState(binding),
		RoutePolicyState:     RoutePolicyNone,
		HomeserverBindingID:  binding.HomeserverBindingID,
		HomeserverBinding:    binding,
		RoutePolicy:          policy,
		CreatedAt:            now,
		UpdatedAt:            now,
		ValidatedAt:          now,
		RedactionStatus:      baseconnectors.RedactionStatusRedacted,
		RetentionExpiresAt:   now.Add(90 * 24 * time.Hour),
		SetupCompletedWithin: timeout,
	}
	if setup.BotCredentialState == "" {
		setup.BotCredentialState = BotCredentialUnknown
	}
	if input.RedactionSuppressed || setup.BotCredentialState == BotCredentialRedactionSuppressed {
		setup.Status = baseconnectors.LifecycleStateFailed
		setup.ReasonCode = string(baseconnectors.DiagnosticUnknownConnectorFailure)
		setup.RedactionStatus = baseconnectors.RedactionStatusSuppressed
		return setup
	}
	if input.Cancelled {
		setup.Status = baseconnectors.LifecycleStateDisabled
		setup.TerminalState = TerminalCancelled
		setup.ReasonCode = "user_cancelled"
		return setup
	}
	if input.RequestedHostedHomeserver || input.RequestedAccountProvision {
		setup.Status = baseconnectors.LifecycleStateUnsupportedCapability
		setup.ReasonCode = string(baseconnectors.DiagnosticUnsupportedCapability)
		return setup
	}
	if setup.BotCredentialState != BotCredentialValid {
		setup.ReasonCode = reasonForBotCredential(setup.BotCredentialState)
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
	if err := ValidateHomeserverBinding(binding); err != nil {
		setup.ReasonCode = string(baseconnectors.DiagnosticPermissionMissing)
		return setup
	}
	if !input.ConformancePassed {
		setup.ReasonCode = "conformance_not_ready"
		return setup
	}
	if HasReadyRoutePolicy(policy) {
		setup.Status = baseconnectors.LifecycleStateHealthy
		setup.TerminalState = TerminalReady
		setup.RoutePolicyState = RoutePolicyValid
		setup.DeliveryEligible = true
		setup.ReasonCode = "healthy"
		return setup
	}
	setup.RoutePolicyState = RoutePolicyNone
	setup.ReasonCode = string(baseconnectors.DiagnosticBlockedRoute)
	return setup
}

func reasonForBotCredential(state BotCredentialState) string {
	switch state {
	case BotCredentialPermissionMissing:
		return string(baseconnectors.DiagnosticPermissionMissing)
	case BotCredentialInvalid, BotCredentialRevoked, BotCredentialNotStarted, BotCredentialSubmitted, BotCredentialUnknown:
		return string(baseconnectors.DiagnosticAuthMissing)
	default:
		return string(baseconnectors.DiagnosticUnknownConnectorFailure)
	}
}
