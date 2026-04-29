package events

import "github.com/dopejs/dope-agent/daemon/internal/livevalidation"

const (
	LiveValidationStartedName                = "live_validation.started"
	LiveValidationBlockedName                = "live_validation.blocked"
	LiveValidationAwaitingApprovalName       = "live_validation.awaiting_approval"
	LiveValidationSideEffectRecordedName     = "live_validation.side_effect_recorded"
	LiveValidationOperatorActionNeededName   = "live_validation.operator_action_needed"
	LiveValidationCompletedName              = "live_validation.completed"
	LiveValidationComparisonCompletedName    = "live_validation.comparison_completed"
	LiveValidationReconciliationResolvedName = "live_validation.reconciliation_resolved"
	LiveValidationKillSwitchChangedName      = "live_validation.kill_switch_changed"
	LiveValidationAbortedName                = "live_validation.aborted"
)

func LiveValidationAttemptEvent(name string, attempt livevalidation.Attempt, denials []livevalidation.Denial) Event {
	payload := map[string]any{
		"validationId":     attempt.ValidationID,
		"tenantId":         attempt.TenantID,
		"candidateId":      attempt.CandidateID,
		"environmentScope": attempt.EnvironmentScope,
		"status":           string(attempt.Status),
	}
	if len(denials) > 0 {
		payload["denials"] = denials
		payload["gate"] = denials[0].Gate
		payload["reasonCode"] = denials[0].ReasonCode
	}
	return Event{
		TenantID:   attempt.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: attempt.UpdatedAt,
		Resource: Resource{
			Kind: "live_validation",
			ID:   attempt.ValidationID,
		},
		Payload: payload,
	}
}

func LiveValidationLedgerEvent(name string, entry livevalidation.SideEffectLedgerEntry) Event {
	payload := map[string]any{
		"validationId":    entry.ValidationID,
		"tenantId":        entry.TenantID,
		"ledgerEntryId":   entry.LedgerEntryID,
		"toolClass":       entry.ToolClass,
		"actionRef":       entry.ActionRef,
		"outcome":         entry.Outcome,
		"ambiguousCommit": entry.AmbiguousCommit,
	}
	return Event{
		TenantID:   entry.TenantID,
		Category:   "evaluation",
		Name:       name,
		OccurredAt: entry.UpdatedAt,
		Resource: Resource{
			Kind: "live_validation_ledger_entry",
			ID:   entry.LedgerEntryID,
		},
		Payload: payload,
	}
}

func LiveValidationComparisonEvent(comparison livevalidation.Comparison) Event {
	return Event{
		Category:   "evaluation",
		Name:       LiveValidationComparisonCompletedName,
		OccurredAt: comparison.GeneratedAt,
		Resource: Resource{
			Kind: "live_validation_comparison",
			ID:   comparison.ComparisonID,
		},
		Payload: map[string]any{
			"validationId":   comparison.ValidationID,
			"comparisonId":   comparison.ComparisonID,
			"terminalStatus": comparison.TerminalStatus,
		},
	}
}

func LiveValidationReconciliationEvent(resolution livevalidation.ReconciliationResolution) Event {
	return Event{
		TenantID:   resolution.TenantID,
		Category:   "evaluation",
		Name:       LiveValidationReconciliationResolvedName,
		OccurredAt: resolution.ResolvedAt,
		Resource: Resource{
			Kind: "live_validation_reconciliation",
			ID:   resolution.ReconciliationID,
		},
		Payload: map[string]any{
			"tenantId":          resolution.TenantID,
			"ambiguousCommitId": resolution.AmbiguousCommitID,
			"reconciliationId":  resolution.ReconciliationID,
			"resolution":        resolution.Resolution,
			"resolvedBy":        resolution.ResolvedBy,
		},
	}
}
