package livevalidation

import (
	"context"
	"testing"
)

func TestCreateComparisonSummarizesDeniedAndOperatorActionNeededOutcomes(t *testing.T) {
	store := &memoryStore{}
	manager := NewManager(Dependencies{Enabled: true, Store: store, Clock: fixedClock})
	attempt := Attempt{ValidationID: "lv_1", TenantID: "ten_1", CandidateID: "candidate_1"}
	if err := store.UpsertLiveValidationAttempt(context.Background(), attempt); err != nil {
		t.Fatalf("UpsertLiveValidationAttempt: %v", err)
	}
	store.ledger = append(store.ledger,
		SideEffectLedgerEntry{LedgerEntryID: "ledger_1", ValidationID: "lv_1", TenantID: "ten_1", CandidateID: "candidate_1", ToolClass: ToolClassMCPToolCall, SafetyClass: SafetyClassUnsupported, Outcome: LedgerOutcomeDenied},
		SideEffectLedgerEntry{LedgerEntryID: "ledger_2", ValidationID: "lv_1", TenantID: "ten_1", CandidateID: "candidate_1", ToolClass: ToolClassMailSend, SafetyClass: SafetyClassNonIdempotentMutation, Outcome: LedgerOutcomeOperatorActionNeeded, AmbiguousCommit: true},
	)
	comparison, err := manager.CreateComparison(context.Background(), "lv_1")
	if err != nil {
		t.Fatalf("CreateComparison returned error: %v", err)
	}
	if comparison.TerminalStatus != ComparisonStatusOperatorActionNeeded {
		t.Fatalf("TerminalStatus=%s, want operator_action_needed", comparison.TerminalStatus)
	}
	if comparison.LedgerSummary[LedgerOutcomeDenied] != 1 || comparison.LedgerSummary[LedgerOutcomeOperatorActionNeeded] != 1 {
		t.Fatalf("LedgerSummary=%+v", comparison.LedgerSummary)
	}
}
