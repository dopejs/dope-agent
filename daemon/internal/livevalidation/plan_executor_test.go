package livevalidation

import (
	"context"
	"testing"
)

func TestRunSideEffectPlanRecordsSkippedAndCompletesAttempt(t *testing.T) {
	store := &memoryStore{}
	manager := NewManager(Dependencies{Enabled: true, Store: store, Clock: fixedClock})
	attempt := Attempt{
		ValidationID: "lv_plan",
		TenantID:     "ten_1",
		CandidateID:  "candidate_1",
		Status:       AttemptStatusRunning,
		RequestedScope: SideEffectScope{
			ScopeID:             "scope_1",
			IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
			ExcludedToolClasses: []ToolClass{ToolClassMCPToolCall},
		},
		CreatedAt: fixedClock(),
		UpdatedAt: fixedClock(),
	}
	if err := store.UpsertLiveValidationAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("UpsertLiveValidationAttempt: %v", err)
	}

	result, err := manager.RunSideEffectPlan(context.Background(), "lv_plan", []SideEffectPlanStep{
		{ToolClass: ToolClassDaemonInspectionRead, ActionRef: "inspect_1", SourceRef: "tool_call_1", RequestedOutcome: LedgerOutcomeCompleted},
		{ToolClass: ToolClassMCPToolCall, ActionRef: "mcp_1", SourceRef: "tool_call_2"},
	})
	if err != nil {
		t.Fatalf("RunSideEffectPlan returned error: %v", err)
	}
	if result.Attempt.Status != AttemptStatusCompleted {
		t.Fatalf("Status=%s, want completed", result.Attempt.Status)
	}
	if result.Attempt.LedgerSummary[LedgerOutcomeCompleted] != 1 || result.Attempt.LedgerSummary[LedgerOutcomeSkipped] != 1 {
		t.Fatalf("LedgerSummary=%+v, want completed=1 skipped=1", result.Attempt.LedgerSummary)
	}
	if len(result.Ledger) != 2 || result.Ledger[1].Outcome != LedgerOutcomeSkipped {
		t.Fatalf("ledger=%+v, want second skipped", result.Ledger)
	}
}

func TestRunSideEffectPlanMarksAttemptOperatorActionNeeded(t *testing.T) {
	store := &memoryStore{}
	manager := NewManager(Dependencies{Enabled: true, Store: store, Clock: fixedClock})
	attempt := Attempt{
		ValidationID: "lv_ambiguous_plan",
		TenantID:     "ten_1",
		CandidateID:  "candidate_1",
		Status:       AttemptStatusRunning,
		RequestedScope: SideEffectScope{
			ScopeID:             "scope_1",
			IncludedToolClasses: []ToolClass{ToolClassMailSend},
		},
		CreatedAt: fixedClock(),
		UpdatedAt: fixedClock(),
	}
	if err := store.UpsertLiveValidationAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("UpsertLiveValidationAttempt: %v", err)
	}

	result, err := manager.RunSideEffectPlan(context.Background(), "lv_ambiguous_plan", []SideEffectPlanStep{
		{ToolClass: ToolClassMailSend, ActionRef: "send_1", SourceRef: "tool_call_1", AmbiguousCause: AmbiguousCauseTimeout},
	})
	if err != nil {
		t.Fatalf("RunSideEffectPlan returned error: %v", err)
	}
	if result.Attempt.Status != AttemptStatusOperatorActionNeeded {
		t.Fatalf("Status=%s, want operator_action_needed", result.Attempt.Status)
	}
	if result.Attempt.LedgerSummary[LedgerOutcomeOperatorActionNeeded] != 1 {
		t.Fatalf("LedgerSummary=%+v, want operator_action_needed=1", result.Attempt.LedgerSummary)
	}
}
