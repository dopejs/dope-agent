package delivery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func (m *Manager) dispatchImmediate(ctx context.Context, outcome DeliveryOutcome, target DeliveryTarget) (DeliveryOutcome, error) {
	return m.dispatchAttempt(ctx, outcome, target, m.nextAttemptNumber(ctx, outcome.DeliveryID))
}

func (m *Manager) dispatchAttempt(ctx context.Context, outcome DeliveryOutcome, target DeliveryTarget, attemptNumber int) (DeliveryOutcome, error) {
	now := time.Now().UTC()
	outcome.Status = OutcomeStatusDispatching
	outcome.UpdatedAt = now
	if err := m.storeOutcome(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}

	if target.Status != TargetStatusActive {
		return m.failOutcomeWithoutRetry(ctx, outcome, target.TargetID, attemptNumber, "target_unavailable", fmt.Sprintf("target %s is %s", target.TargetID, target.Status))
	}
	if disabled, err := m.connectorDeliveryDisabled(ctx, target); err != nil {
		return DeliveryOutcome{}, err
	} else if disabled {
		connectorID := target.ConnectorBinding.ConnectorID
		failed, err := m.failOutcomeWithoutRetry(ctx, outcome, target.TargetID, attemptNumber, "connector_disabled", fmt.Sprintf("connector %s is disabled", connectorID))
		if err == nil {
			err = m.recordChannelBackgroundDeliveryOutcome(ctx, failed, target, "connector_disabled")
		}
		return failed, err
	}

	adapter := m.adapterFor(target.TargetKind)
	if adapter == nil {
		return m.failOutcomeWithoutRetry(ctx, outcome, target.TargetID, attemptNumber, "adapter_unavailable", fmt.Sprintf("no adapter registered for %s", target.TargetID))
	}

	attempt := DeliveryAttempt{
		AttemptID:     newAttemptID(),
		DeliveryID:    outcome.DeliveryID,
		AttemptNumber: attemptNumber,
		TargetID:      target.TargetID,
		TransportKind: string(target.TargetKind),
		Status:        AttemptStatusRunning,
		StartedAt:     now,
	}
	if err := m.storeAttempt(ctx, attempt); err != nil {
		return DeliveryOutcome{}, err
	}

	result, err := adapter.Send(ctx, target, outcome)
	completedAt := time.Now().UTC()
	attempt.TransportKind = nonEmpty(result.TransportKind, attempt.TransportKind)
	attempt.TransportReceiptSummary = result.ReceiptSummary
	attempt.ConnectorMessageDeliveryID = result.ConnectorMessageDeliveryID
	attempt.CompletedAt = &completedAt

	if err != nil {
		failed, failureErr := m.handleAttemptFailure(ctx, outcome, attempt, err)
		if failureErr == nil {
			failureErr = m.recordChannelBackgroundDeliveryOutcome(ctx, failed, target, attempt.FailureClass)
		}
		return failed, failureErr
	}

	attempt.Status = AttemptStatusDelivered
	if err := m.storeAttempt(ctx, attempt); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.publishAttemptRecorded(ctx, outcome, attempt); err != nil {
		return DeliveryOutcome{}, err
	}

	outcome.Status = OutcomeStatusDelivered
	outcome.UpdatedAt = completedAt
	outcome.FinalizedAt = &completedAt
	if err := m.storeOutcome(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.publishOutcomeStatusChanged(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.recordChannelBackgroundDeliveryOutcome(ctx, outcome, target, "delivered"); err != nil {
		return DeliveryOutcome{}, err
	}
	m.clearRetrySchedule(outcome.DeliveryID)
	return m.attachAttempts(ctx, outcome)
}

func (m *Manager) handleAttemptFailure(ctx context.Context, outcome DeliveryOutcome, attempt DeliveryAttempt, sendErr error) (DeliveryOutcome, error) {
	completedAt := time.Now().UTC()
	attempt.FailureClass = "transport_failed"
	attempt.FailureReason = sendErr.Error()
	if attempt.AttemptNumber < m.maxAttempts {
		nextRetryAt := completedAt.Add(m.retryDelayForAttempt(attempt.AttemptNumber))
		attempt.Status = AttemptStatusRetryableFailure
		attempt.NextRetryAt = &nextRetryAt
		if err := m.storeAttempt(ctx, attempt); err != nil {
			return DeliveryOutcome{}, err
		}
		if err := m.publishAttemptRecorded(ctx, outcome, attempt); err != nil {
			return DeliveryOutcome{}, err
		}
		outcome.Status = OutcomeStatusQueued
		outcome.UpdatedAt = completedAt
		if err := m.storeOutcome(ctx, outcome); err != nil {
			return DeliveryOutcome{}, err
		}
		if err := m.publishOutcomeStatusChanged(ctx, outcome); err != nil {
			return DeliveryOutcome{}, err
		}
		m.scheduleRetry(outcome.DeliveryID, nextRetryAt)
		return m.attachAttempts(ctx, outcome)
	}
	attempt.Status = AttemptStatusTerminalFailure
	attempt.FailureClass = "retry_exhausted"
	attempt.NextRetryAt = nil
	if err := m.storeAttempt(ctx, attempt); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.publishAttemptRecorded(ctx, outcome, attempt); err != nil {
		return DeliveryOutcome{}, err
	}
	outcome.Status = OutcomeStatusFailed
	outcome.UpdatedAt = completedAt
	outcome.FinalizedAt = &completedAt
	outcome.DiagnosticFailure = deliveryDiagnosticFailure(outcome, attempt.FailureClass, attempt.FailureReason, true, completedAt)
	if strings.TrimSpace(outcome.PayloadPreview) == "" {
		outcome.PayloadPreview = sendErr.Error()
	}
	if err := m.storeOutcome(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.publishOutcomeStatusChanged(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	m.clearRetrySchedule(outcome.DeliveryID)
	return m.attachAttempts(ctx, outcome)
}

func (m *Manager) failOutcomeWithoutRetry(ctx context.Context, outcome DeliveryOutcome, targetID string, attemptNumber int, failureClass, failureReason string) (DeliveryOutcome, error) {
	now := time.Now().UTC()
	attempt := DeliveryAttempt{
		AttemptID:     newAttemptID(),
		DeliveryID:    outcome.DeliveryID,
		AttemptNumber: attemptNumber,
		TargetID:      targetID,
		TransportKind: nonEmpty(string(outcome.Mode), "unknown"),
		Status:        AttemptStatusTerminalFailure,
		FailureClass:  failureClass,
		FailureReason: failureReason,
		StartedAt:     now,
		CompletedAt:   &now,
	}
	if err := m.storeAttempt(ctx, attempt); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.publishAttemptRecorded(ctx, outcome, attempt); err != nil {
		return DeliveryOutcome{}, err
	}
	outcome.Status = OutcomeStatusFailed
	outcome.UpdatedAt = now
	outcome.FinalizedAt = &now
	outcome.DiagnosticFailure = deliveryDiagnosticFailure(outcome, failureClass, failureReason, false, now)
	if strings.TrimSpace(outcome.PayloadPreview) == "" {
		outcome.PayloadPreview = failureReason
	}
	if err := m.storeOutcome(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	if err := m.publishOutcomeStatusChanged(ctx, outcome); err != nil {
		return DeliveryOutcome{}, err
	}
	m.clearRetrySchedule(outcome.DeliveryID)
	return m.attachAttempts(ctx, outcome)
}

func deliveryDiagnosticFailure(outcome DeliveryOutcome, failureClass, failureReason string, sideEffecting bool, checkedAt time.Time) *integrations.DiagnosticFailureProjection {
	diagnostic := integrations.DiagnosticFailureForOperationFailure("delivery", "", outcome.IntegrationID, string(outcome.Mode), failureClass, failureReason, sideEffecting, checkedAt)
	return &diagnostic
}

func (m *Manager) nextAttemptNumber(ctx context.Context, deliveryID string) int {
	if m == nil || m.sqliteStore == nil {
		return 1
	}
	attempts, err := m.sqliteStore.ListDeliveryAttempts(ctx, deliveryID)
	if err != nil || len(attempts) == 0 {
		return 1
	}
	maxAttempt := 0
	for _, item := range attempts {
		if item.AttemptNumber > maxAttempt {
			maxAttempt = item.AttemptNumber
		}
	}
	return maxAttempt + 1
}

func (m *Manager) retryDelayForAttempt(attemptNumber int) time.Duration {
	delay := m.baseRetryDelay
	if delay <= 0 {
		delay = 5 * time.Second
	}
	if attemptNumber > 1 {
		for i := 1; i < attemptNumber; i++ {
			delay *= 2
			if m.maxRetryDelay > 0 && delay >= m.maxRetryDelay {
				return m.maxRetryDelay
			}
		}
	}
	if m.maxRetryDelay > 0 && delay > m.maxRetryDelay {
		return m.maxRetryDelay
	}
	return delay
}

func (m *Manager) scheduleRetry(deliveryID string, when time.Time) {
	if m == nil || strings.TrimSpace(deliveryID) == "" {
		return
	}
	m.mu.Lock()
	if _, exists := m.retryScheduled[deliveryID]; exists {
		m.mu.Unlock()
		return
	}
	m.retryScheduled[deliveryID] = struct{}{}
	m.mu.Unlock()

	go func() {
		delay := time.Until(when)
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			<-timer.C
		}
		ctx := context.Background()
		_ = m.resumeOutcome(ctx, deliveryID)
	}()
}

func (m *Manager) clearRetrySchedule(deliveryID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.retryScheduled, deliveryID)
}

func (m *Manager) resumeOutcome(ctx context.Context, deliveryID string) error {
	defer m.clearRetrySchedule(deliveryID)
	outcome, ok, err := m.GetOutcome(ctx, deliveryID)
	if err != nil || !ok {
		return err
	}
	if outcome.Status != OutcomeStatusQueued && outcome.Status != OutcomeStatusDispatching {
		return nil
	}
	target, ok, err := m.GetTarget(ctx, outcome.ChosenTargetID)
	if err != nil {
		return err
	}
	if !ok {
		_, err = m.failOutcomeWithoutRetry(ctx, outcome, outcome.ChosenTargetID, m.nextAttemptNumber(ctx, outcome.DeliveryID), "target_missing", "configured target is missing")
		return err
	}
	_, err = m.dispatchAttempt(ctx, outcome, target, m.nextAttemptNumber(ctx, outcome.DeliveryID))
	return err
}
