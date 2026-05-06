package setupwizard

import (
	"context"
	"time"
)

func (s *Service) DependentUseDecision(_ context.Context, session SetupSession, capability string) DependentUseDecision {
	mode := SafeUseBlocked
	allowed := []string(nil)
	reason := session.ReasonCode
	switch session.State {
	case StateReady:
		mode = SafeUseNormal
		reason = ""
	case StateDegraded:
		if contains(session.AllowedCapabilities, capability) && contains(session.DiagnosticAllowedUse, capability) {
			mode = SafeUseLimited
			allowed = []string{capability}
		} else {
			reason = firstNonEmpty(reason, "degraded_capability_not_allowed")
		}
	case StateActionRequired, StateUnavailable, StateCancelled, StateDisabled:
		mode = SafeUseBlocked
	default:
		mode = SafeUseBlocked
	}
	return DependentUseDecision{
		TenantID:            session.TenantID,
		TargetID:            session.TargetID,
		SetupState:          session.State,
		SafeUseMode:         mode,
		AllowedCapabilities: allowed,
		ReasonCode:          reason,
		CheckedAt:           time.Now().UTC(),
	}
}
