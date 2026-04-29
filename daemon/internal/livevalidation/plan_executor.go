package livevalidation

import (
	"context"
	"fmt"
)

type SideEffectPlanStep struct {
	ToolClass        ToolClass
	ActionRef        string
	SourceRef        string
	ApprovalID       string
	DownstreamRef    string
	RequestedOutcome LedgerOutcome
	AmbiguousCause   AmbiguousCommitCause
}

type SideEffectPlanResult struct {
	Attempt Attempt                 `json:"attempt"`
	Ledger  []SideEffectLedgerEntry `json:"ledger"`
}

func (m *Manager) RunSideEffectPlan(ctx context.Context, validationID string, steps []SideEffectPlanStep) (SideEffectPlanResult, error) {
	attempt, ok, err := m.GetAttempt(ctx, validationID)
	if err != nil {
		return SideEffectPlanResult{}, err
	}
	if !ok {
		return SideEffectPlanResult{}, fmt.Errorf("live validation %s not found", validationID)
	}
	if attempt.Status != AttemptStatusRunning {
		return SideEffectPlanResult{}, fmt.Errorf("live validation %s is not running", validationID)
	}
	matrix, err := m.SupportMatrix()
	if err != nil {
		return SideEffectPlanResult{}, err
	}
	included := toolClassSet(attempt.RequestedScope.IncludedToolClasses)
	excluded := toolClassSet(attempt.RequestedScope.ExcludedToolClasses)
	hasExplicitIncludes := len(included) > 0

	ledger := make([]SideEffectLedgerEntry, 0, len(steps))
	summary := LedgerSummary{}
	for _, step := range steps {
		if excluded[step.ToolClass] || (hasExplicitIncludes && !included[step.ToolClass]) {
			entry, err := m.appendSkippedPlanStep(ctx, attempt, matrix, step)
			if err != nil {
				return SideEffectPlanResult{}, err
			}
			ledger = append(ledger, entry)
			summary[entry.Outcome]++
			continue
		}
		entry, err := m.ExecuteSideEffect(ctx, SideEffectExecutionInput{
			Attempt:          attempt,
			ToolClass:        step.ToolClass,
			ActionRef:        step.ActionRef,
			SourceRef:        step.SourceRef,
			ApprovalID:       step.ApprovalID,
			DownstreamRef:    step.DownstreamRef,
			RequestedOutcome: step.RequestedOutcome,
			AmbiguousCause:   step.AmbiguousCause,
		})
		if err != nil {
			return SideEffectPlanResult{}, err
		}
		ledger = append(ledger, entry)
		summary[entry.Outcome]++
	}

	now := m.clock()
	attempt.LedgerSummary = summary
	attempt.UpdatedAt = now
	attempt.CompletedAt = &now
	switch {
	case summary[LedgerOutcomeOperatorActionNeeded] > 0:
		attempt.Status = AttemptStatusOperatorActionNeeded
	case summary[LedgerOutcomeFailed] > 0:
		attempt.Status = AttemptStatusFailed
	default:
		attempt.Status = AttemptStatusCompleted
	}
	if err := m.persistAttempt(ctx, attempt); err != nil {
		return SideEffectPlanResult{}, err
	}
	return SideEffectPlanResult{Attempt: attempt, Ledger: ledger}, nil
}

func (m *Manager) appendSkippedPlanStep(ctx context.Context, attempt Attempt, matrix Matrix, step SideEffectPlanStep) (SideEffectLedgerEntry, error) {
	row, ok := matrix.Row(step.ToolClass)
	safetyClass := SafetyClassUnsupported
	if ok {
		safetyClass = row.SafetyClass
	}
	entry, err := m.AppendLedgerEntry(ctx, SideEffectLedgerEntry{
		ValidationID:  attempt.ValidationID,
		TenantID:      attempt.TenantID,
		CandidateID:   attempt.CandidateID,
		SourceRef:     step.SourceRef,
		ToolClass:     step.ToolClass,
		SafetyClass:   safetyClass,
		ActionRef:     step.ActionRef,
		ApprovalID:    step.ApprovalID,
		DownstreamRef: step.DownstreamRef,
		Outcome:       LedgerOutcomeSkipped,
		ReasonCode:    "live_validation.scope_excluded",
	})
	if err == nil {
		m.emitLedgerEvent(ctx, LedgerEventSideEffectRecorded, entry)
	}
	return entry, err
}
