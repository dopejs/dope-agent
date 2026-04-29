package livevalidation

import "context"

type SideEffectExecutionInput struct {
	Attempt          Attempt
	ToolClass        ToolClass
	ActionRef        string
	SourceRef        string
	ApprovalID       string
	DownstreamRef    string
	RequestedOutcome LedgerOutcome
	AmbiguousCause   AmbiguousCommitCause
}

func (m *Manager) ExecuteSideEffect(ctx context.Context, input SideEffectExecutionInput) (SideEffectLedgerEntry, error) {
	now := m.clock()
	decision, denial, err := m.evaluateKillSwitch(ctx, input.Attempt.TenantID, now)
	if err != nil {
		return SideEffectLedgerEntry{}, err
	}
	if !decision.Allowed {
		aborted, err := m.AppendLedgerEntry(ctx, SideEffectLedgerEntry{
			ValidationID:  input.Attempt.ValidationID,
			TenantID:      input.Attempt.TenantID,
			CandidateID:   input.Attempt.CandidateID,
			SourceRef:     input.SourceRef,
			ToolClass:     input.ToolClass,
			ActionRef:     input.ActionRef,
			ApprovalID:    input.ApprovalID,
			DownstreamRef: input.DownstreamRef,
			Outcome:       LedgerOutcomeAborted,
			ReasonCode:    denial.ReasonCode,
			UpdatedAt:     now,
		})
		if err == nil {
			m.emitLedgerEvent(ctx, LedgerEventSideEffectRecorded, aborted)
		}
		return aborted, err
	}
	matrix, err := m.SupportMatrix()
	if err != nil {
		return SideEffectLedgerEntry{}, err
	}
	row, err := matrix.Lookup(input.ToolClass)
	if err != nil {
		row = MatrixRow{ToolClass: input.ToolClass, SafetyClass: SafetyClassUnsupported}
		entry := SideEffectLedgerEntry{
			ValidationID:  input.Attempt.ValidationID,
			TenantID:      input.Attempt.TenantID,
			CandidateID:   input.Attempt.CandidateID,
			SourceRef:     input.SourceRef,
			ToolClass:     input.ToolClass,
			SafetyClass:   row.SafetyClass,
			ActionRef:     input.ActionRef,
			ApprovalID:    input.ApprovalID,
			DownstreamRef: input.DownstreamRef,
			Outcome:       LedgerOutcomeDenied,
			ReasonCode:    "live_validation.unsupported_tool_class",
		}
		denied, err := m.AppendLedgerEntry(ctx, entry)
		if err == nil {
			m.emitLedgerEvent(ctx, LedgerEventSideEffectRecorded, denied)
		}
		return denied, err
	}
	ledgerEntryID := newID("lv_ledger")
	attempted, err := m.AppendLedgerEntry(ctx, SideEffectLedgerEntry{
		LedgerEntryID:  ledgerEntryID,
		ValidationID:   input.Attempt.ValidationID,
		TenantID:       input.Attempt.TenantID,
		CandidateID:    input.Attempt.CandidateID,
		SourceRef:      input.SourceRef,
		ToolClass:      input.ToolClass,
		SafetyClass:    row.SafetyClass,
		ActionRef:      input.ActionRef,
		ApprovalID:     input.ApprovalID,
		DownstreamRef:  input.DownstreamRef,
		Outcome:        LedgerOutcomeAttempted,
		CorrelationKey: CorrelationKey(input.Attempt.ValidationID, ledgerEntryID, input.ActionRef),
	})
	if err != nil {
		return SideEffectLedgerEntry{}, err
	}
	m.emitLedgerEvent(ctx, LedgerEventSideEffectRecorded, attempted)
	if input.RequestedOutcome == "" {
		input.RequestedOutcome = LedgerOutcomeCompleted
	}
	if input.RequestedOutcome == LedgerOutcomeOperatorActionNeeded || input.AmbiguousCause != "" {
		attempted.AmbiguousCommit = true
		if _, err := m.RecordAmbiguousCommit(ctx, AmbiguousCommit{
			LedgerEntryID:       attempted.LedgerEntryID,
			ValidationID:        attempted.ValidationID,
			TenantID:            attempted.TenantID,
			Cause:               input.AmbiguousCause,
			LastKnownRequestRef: attempted.CorrelationKey,
		}); err != nil {
			return SideEffectLedgerEntry{}, err
		}
		attempted.Outcome = LedgerOutcomeOperatorActionNeeded
		m.emitLedgerEvent(ctx, LedgerEventOperatorActionNeeded, attempted)
		return attempted, nil
	}
	if err := m.UpdateLedgerOutcome(ctx, attempted.LedgerEntryID, input.RequestedOutcome, "live_validation.executor_result"); err != nil {
		return SideEffectLedgerEntry{}, err
	}
	attempted.Outcome = input.RequestedOutcome
	m.emitLedgerEvent(ctx, LedgerEventSideEffectRecorded, attempted)
	return attempted, nil
}
