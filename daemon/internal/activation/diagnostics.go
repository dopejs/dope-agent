package activation

import (
	"context"
	"strings"
)

func (s *Service) Diagnostics(ctx context.Context, input GetInput) ([]Diagnostic, error) {
	if s == nil || s.stateStore == nil {
		return nil, activationError(ReasonUnexpectedFailed, FailureStageUnexpected, false, RemediationOwnerOperator, "activation service is not configured")
	}
	principalID := strings.TrimSpace(input.TenantContext.PrincipalID)
	if principalID == "" {
		principalID = strings.TrimSpace(input.Token.PrincipalID)
	}
	tenantID := strings.TrimSpace(input.TenantContext.TenantID)
	if principalID == "" || tenantID == "" {
		return nil, activationError(ReasonTenantAccessRevoked, FailureStageAuthorization, false, RemediationOwnerProductUser, "activation tenant context is required")
	}
	state, ok, err := s.stateStore.GetActivationStateForPrincipalTenant(ctx, principalID, tenantID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []Diagnostic{}, nil
	}
	diagnostic, ok := diagnosticFromState(state)
	if !ok {
		return []Diagnostic{}, nil
	}
	return []Diagnostic{diagnostic}, nil
}

func diagnosticFromState(state State) (Diagnostic, bool) {
	reason := state.FailureReason
	if reason == nil && len(state.BlockingReasonCodes) == 0 && (state.TestChat == nil || state.TestChat.Status == TestChatStatusCompleted) {
		return Diagnostic{}, false
	}
	stage := FailureStageUnexpected
	reasonCode := ReasonUnexpectedFailed
	retryable := false
	owner := RemediationOwnerOperator
	if reason != nil {
		stage = reason.Stage
		reasonCode = reason.ReasonCode
		retryable = reason.Retryable
		owner = reason.RemediationOwner
	} else if len(state.BlockingReasonCodes) > 0 {
		reasonCode = state.BlockingReasonCodes[0]
		stage = stageForReason(reasonCode)
		retryable = reasonCode == ReasonQuotaBaselineUnavailable || reasonCode == ReasonTestChatUnavailable
	}
	out := Diagnostic{
		ActivationID:     state.ActivationID,
		TenantID:         state.TenantID,
		PrincipalID:      state.PrincipalID,
		Status:           state.Status,
		Stage:            stage,
		ReasonCode:       reasonCode,
		Retryable:        retryable,
		RemediationOwner: owner,
		LastTransitionAt: state.UpdatedAt,
		ReadinessItemIDs: readinessItemIDsForState(state),
	}
	if state.QuotaBaseline != nil {
		out.QuotaBaselineStatus = string(state.QuotaBaseline.Status)
	}
	if state.TestChat != nil && state.TestChat.Status != TestChatStatusCompleted {
		out.TestChat = state.TestChat
	}
	return out, true
}

func readinessItemIDsForState(state State) []string {
	ids := []string{}
	for _, item := range state.ReadinessItems {
		if item.Status == ReadinessStatusBlocked || item.ReasonCode != "" {
			ids = append(ids, item.ItemID)
		}
	}
	return ids
}

func stageForReason(reason ReasonCode) FailureStage {
	switch reason {
	case ReasonQuotaBaselineUnavailable:
		return FailureStageQuotaBaseline
	case ReasonTenantAccessRevoked:
		return FailureStageAuthorization
	case ReasonPrincipalDenied, ReasonPrincipalDisabled:
		return FailureStageEligibility
	case ReasonTestChatFailed, ReasonTestChatUnavailable:
		return FailureStageTestChat
	default:
		return FailureStageUnexpected
	}
}
