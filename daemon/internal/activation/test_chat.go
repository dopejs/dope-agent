package activation

import (
	"context"
	"strings"
)

const defaultActivationTestChatMessage = "Run a safe hosted activation test."

func (s *Service) RunTestChat(ctx context.Context, input RunTestChatInput) (State, TestChatMetadata, error) {
	if s == nil || s.stateStore == nil {
		return State{}, TestChatMetadata{}, activationError(ReasonUnexpectedFailed, FailureStageUnexpected, false, RemediationOwnerOperator, "activation service is not configured")
	}
	principalID := strings.TrimSpace(input.TenantContext.PrincipalID)
	if principalID == "" {
		principalID = strings.TrimSpace(input.Token.PrincipalID)
	}
	tenantID := strings.TrimSpace(input.TenantContext.TenantID)
	if principalID == "" || tenantID == "" {
		return State{}, TestChatMetadata{}, activationError(ReasonTenantAccessRevoked, FailureStageAuthorization, false, RemediationOwnerProductUser, "activation tenant context is required")
	}
	state, ok, err := s.stateStore.GetActivationStateForPrincipalTenant(ctx, principalID, tenantID)
	if err != nil {
		return State{}, TestChatMetadata{}, err
	}
	if !ok {
		return State{}, TestChatMetadata{}, activationError(ReasonTenantAccessRevoked, FailureStageAuthorization, false, RemediationOwnerProductUser, "activation state is not available for this tenant")
	}
	if state.Status == StatusBlocked || len(state.BlockingReasonCodes) > 0 || !state.FirstAction.Available {
		reason := ReasonTestChatUnavailable
		stage := FailureStageTestChat
		if hasBlockingReason(state, ReasonQuotaBaselineUnavailable) {
			reason = ReasonQuotaBaselineUnavailable
			stage = FailureStageQuotaBaseline
		}
		return State{}, TestChatMetadata{}, activationError(reason, stage, true, RemediationOwnerOperator, "activation readiness blocks test chat")
	}
	if s.chat == nil {
		return State{}, TestChatMetadata{}, activationError(ReasonTestChatUnavailable, FailureStageTestChat, true, RemediationOwnerOperator, "activation test chat runner is not configured")
	}
	now := s.now()
	message := strings.TrimSpace(input.Message)
	if message == "" {
		message = defaultActivationTestChatMessage
	}
	result, err := s.chat.RunActivationTestChat(ctx, TestChatInput{
		ActivationID:     state.ActivationID,
		PrincipalID:      principalID,
		TenantID:         tenantID,
		EnvironmentScope: state.EnvironmentScope,
		Message:          message,
	})
	completedAt := result.CompletedAt
	if completedAt.IsZero() {
		completedAt = now
	}
	metadata := TestChatMetadata{
		ActivationID: state.ActivationID,
		TenantID:     tenantID,
		DispatchID:   strings.TrimSpace(result.DispatchID),
		Status:       result.Status,
		Provider:     strings.TrimSpace(result.Provider),
		Model:        strings.TrimSpace(result.Model),
		Usage:        sanitizeUsageMetadata(result.Usage),
		FinishReason: strings.TrimSpace(result.FinishReason),
		CompletedAt:  &completedAt,
	}
	if metadata.Status == "" {
		metadata.Status = TestChatStatusCompleted
	}
	if err != nil || metadata.Status == TestChatStatusFailed || metadata.Status == TestChatStatusCancelled {
		metadata.ReasonCode = ReasonTestChatFailed
		state.Status = StatusActive
		state.CurrentStepID = StepTestChat
		state.BlockingReasonCodes = []ReasonCode{}
		state.FirstAction = DefaultTestChatFirstAction(true, nil)
		state.TestChat = &metadata
		state.FailureReason = &FailureReason{
			ReasonCode:       ReasonTestChatFailed,
			Stage:            FailureStageTestChat,
			Retryable:        true,
			RemediationOwner: RemediationOwnerOperator,
			Message:          firstNonEmpty(errString(err), "activation test chat failed"),
		}
		state.UpdatedAt = now
		state.LastEvaluatedAt = now
		if persistErr := s.stateStore.UpsertActivationState(ctx, state); persistErr != nil {
			return State{}, metadata, activationError(ReasonPersistenceFailed, FailureStagePersistence, true, RemediationOwnerOperator, persistErr.Error())
		}
		if auditErr := s.recordAudit(ctx, auditRecord{
			EventKind:         "tenant.activation_failed",
			ActivationID:      state.ActivationID,
			TenantID:          tenantID,
			PrincipalID:       principalID,
			TokenID:           input.Token.TokenID,
			Outcome:           "failed",
			ReasonCode:        ReasonTestChatFailed,
			Stage:             FailureStageTestChat,
			FromStatus:        StatusActive,
			ToStatus:          state.Status,
			Retryable:         true,
			RemediationOwner:  RemediationOwnerOperator,
			TestChat:          &metadata,
			CompletedStepIDs:  state.CompletedStepIDs,
			ReadinessItemIDs:  readinessItemIDsForState(state),
			QuotaBaselineStat: quotaBaselineStatusForAudit(state),
		}); auditErr != nil {
			return State{}, metadata, auditErr
		}
		return State{}, metadata, activationError(ReasonTestChatFailed, FailureStageTestChat, true, RemediationOwnerOperator, firstNonEmpty(errString(err), "activation test chat failed"))
	}

	state.Status = StatusFirstActionCompleted
	state.CurrentStepID = StepCompleted
	state.CompletedStepIDs = appendUniqueStep(state.CompletedStepIDs, StepTestChatCompleted)
	state.BlockingReasonCodes = []ReasonCode{}
	state.TestChat = &metadata
	state.FailureReason = nil
	state.FirstActionCompletedAt = &completedAt
	state.UpdatedAt = now
	state.LastEvaluatedAt = now
	if err := s.stateStore.UpsertActivationState(ctx, state); err != nil {
		return State{}, metadata, err
	}
	if err := s.recordAudit(ctx, auditRecord{
		EventKind:         "tenant.activation_test_chat_completed",
		ActivationID:      state.ActivationID,
		TenantID:          tenantID,
		PrincipalID:       principalID,
		TokenID:           input.Token.TokenID,
		Outcome:           "succeeded",
		Stage:             FailureStageTestChat,
		FromStatus:        StatusActive,
		ToStatus:          state.Status,
		RemediationOwner:  RemediationOwnerNoneRequired,
		TestChat:          &metadata,
		CompletedStepIDs:  state.CompletedStepIDs,
		ReadinessItemIDs:  readinessItemIDsForState(state),
		QuotaBaselineStat: quotaBaselineStatusForAudit(state),
	}); err != nil {
		return State{}, metadata, err
	}
	return state, metadata, nil
}

func quotaBaselineStatusForAudit(state State) string {
	if state.QuotaBaseline == nil {
		return ""
	}
	return string(state.QuotaBaseline.Status)
}

func hasBlockingReason(state State, reason ReasonCode) bool {
	for _, item := range state.BlockingReasonCodes {
		if item == reason {
			return true
		}
	}
	if state.FailureReason != nil && state.FailureReason.ReasonCode == reason {
		return true
	}
	return false
}

func appendUniqueStep(items []string, step string) []string {
	for _, item := range items {
		if item == step {
			return items
		}
	}
	return append(append([]string(nil), items...), step)
}

func sanitizeUsageMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := map[string]any{}
	for key, value := range input {
		if forbiddenActivationEvidenceKey(key) {
			continue
		}
		switch typed := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
			output[key] = typed
		}
	}
	return output
}

func forbiddenActivationEvidenceKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	switch normalized {
	case "query", "reply", "transcript", "delta", "prompt", "rawproviderpayload", "authorization", "accesstoken", "refreshtoken", "secret":
		return true
	default:
		return strings.Contains(normalized, "secret")
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
