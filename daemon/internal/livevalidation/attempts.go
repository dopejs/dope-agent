package livevalidation

import (
	"context"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func (m *Manager) newAttempt(input StartInput, tenantContext identity.TenantContext, now time.Time) Attempt {
	validationID := input.ValidationID
	if validationID == "" {
		validationID = newID("live_validation")
	}
	scope := input.RequestedScope
	scope.ValidationID = firstNonEmpty(scope.ValidationID, validationID)
	return Attempt{
		ValidationID:       validationID,
		TenantID:           tenantContext.TenantID,
		CandidateID:        input.CandidateID,
		SourceAttemptID:    input.SourceAttemptID,
		RequestedBy:        tenantContext.PrincipalID,
		EnvironmentScope:   m.EnvironmentScope(),
		RequestedScope:     scope,
		Status:             AttemptStatusQueued,
		PermissionDecision: GateDecision{Allowed: true, CheckedAt: now},
		QuotaDecision:      GateDecision{Allowed: true, CheckedAt: now},
		KillSwitchDecision: GateDecision{Allowed: true, CheckedAt: now},
		ApprovalSummary:    ApprovalSummary{},
		LedgerSummary:      LedgerSummary{},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (m *Manager) block(ctx context.Context, attempt Attempt, gate, reasonCode, message, reference string) (StartResult, error) {
	attempt.Status = AttemptStatusBlocked
	attempt.UpdatedAt = m.clock()
	if err := m.persistAttempt(ctx, attempt); err != nil {
		return StartResult{}, err
	}
	denial := Denial{Gate: gate, ReasonCode: reasonCode, Message: message, Reference: reference}
	return StartResult{Attempt: attempt, Denials: []Denial{denial}}, ErrLiveValidationBlocked
}

func (m *Manager) persistAttempt(ctx context.Context, attempt Attempt) error {
	if m.store == nil {
		return nil
	}
	return m.store.UpsertLiveValidationAttempt(ctx, attempt)
}

func (m *Manager) ListAttempts(ctx context.Context, filter AttemptFilter) ([]Attempt, error) {
	if m.store == nil {
		return nil, nil
	}
	filter.EnvironmentScope = firstNonEmpty(filter.EnvironmentScope, m.EnvironmentScope())
	return m.store.ListLiveValidationAttempts(ctx, filter)
}

func (m *Manager) GetAttempt(ctx context.Context, validationID string) (Attempt, bool, error) {
	if m.store == nil {
		return Attempt{}, false, nil
	}
	tenantID := ""
	if tenantContext, ok := tenantctx.FromContext(ctx); ok {
		tenantID = tenantContext.TenantID
	}
	return m.store.GetLiveValidationAttempt(ctx, tenantID, validationID)
}

func (m *Manager) Abort(ctx context.Context, validationID string) (Attempt, error) {
	attempt, ok, err := m.GetAttempt(ctx, validationID)
	if err != nil {
		return Attempt{}, err
	}
	if !ok {
		return Attempt{}, fmt.Errorf("live validation %s not found", validationID)
	}
	if attempt.Status == AttemptStatusCompleted || attempt.Status == AttemptStatusAborted || attempt.Status == AttemptStatusFailed {
		return attempt, nil
	}
	now := m.clock()
	attempt.Status = AttemptStatusAborted
	attempt.CompletedAt = &now
	attempt.UpdatedAt = now
	if err := m.persistAttempt(ctx, attempt); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}
