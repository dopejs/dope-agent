package app

import (
	"context"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
)

type activationChatRunner struct {
	service *chat.Service
}

func (r activationChatRunner) RunActivationTestChat(ctx context.Context, input activation.TestChatInput) (activation.TestChatResult, error) {
	if r.service == nil {
		return activation.TestChatResult{}, &activation.Error{
			ReasonCode:       activation.ReasonTestChatUnavailable,
			Stage:            activation.FailureStageTestChat,
			Retryable:        true,
			RemediationOwner: activation.RemediationOwnerOperator,
			Message:          "chat service is not configured",
		}
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		message = "Run a safe hosted activation test."
	}
	result, err := r.service.Query(ctx, chat.QueryInput{
		Query:    message,
		Provider: "echo",
		Model:    "echo-v1",
	})
	status := activation.TestChatStatusCompleted
	if result.Dispatch.Status == llm.DispatchStatusCancelled {
		status = activation.TestChatStatusCancelled
	} else if err != nil || result.Dispatch.Status == llm.DispatchStatusFailed || result.Dispatch.Status == llm.DispatchStatusPartialFailed {
		status = activation.TestChatStatusFailed
	}
	completedAt := time.Now().UTC()
	if result.Dispatch.CompletedAt != nil {
		completedAt = result.Dispatch.CompletedAt.UTC()
	}
	return activation.TestChatResult{
		DispatchID: result.Dispatch.DispatchID,
		Status:     status,
		Provider:   result.Dispatch.Provider,
		Model:      result.Dispatch.Model,
		Usage: map[string]any{
			"inputTokens":  result.Dispatch.Usage.InputTokens,
			"outputTokens": result.Dispatch.Usage.OutputTokens,
			"totalTokens":  result.Dispatch.Usage.TotalTokens,
		},
		FinishReason: result.Dispatch.FinishReason,
		CompletedAt:  completedAt,
	}, err
}
