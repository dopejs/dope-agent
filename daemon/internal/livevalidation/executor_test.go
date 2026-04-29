package livevalidation

import (
	"context"
	"testing"
	"time"
)

func TestExecutorPropagatesCorrelationKeyAndTerminalOutcome(t *testing.T) {
	store := &memoryStore{}
	events := []string{}
	manager := NewManager(Dependencies{
		Enabled: true,
		Store:   store,
		Clock:   fixedClock,
		LedgerEventSink: func(_ context.Context, eventName string, _ SideEffectLedgerEntry) {
			events = append(events, eventName)
		},
	})
	attempt := Attempt{ValidationID: "lv_1", TenantID: "ten_1", CandidateID: "candidate_1"}
	entry, err := manager.ExecuteSideEffect(context.Background(), SideEffectExecutionInput{
		Attempt:          attempt,
		ToolClass:        ToolClassIntegrationProbeMutation,
		ActionRef:        "action_1",
		RequestedOutcome: LedgerOutcomeCompleted,
	})
	if err != nil {
		t.Fatalf("ExecuteSideEffect returned error: %v", err)
	}
	if entry.CorrelationKey != "live_validation:lv_1:"+entry.LedgerEntryID+":action_1" {
		t.Fatalf("CorrelationKey=%q", entry.CorrelationKey)
	}
	if len(store.ledger) != 1 || store.ledger[0].Outcome != LedgerOutcomeCompleted {
		t.Fatalf("ledger=%+v, want completed persisted", store.ledger)
	}
	if store.ledger[0].CorrelationKey != entry.CorrelationKey {
		t.Fatalf("persisted correlation=%q, want %q", store.ledger[0].CorrelationKey, entry.CorrelationKey)
	}
	if len(events) != 2 || events[0] != LedgerEventSideEffectRecorded || events[1] != LedgerEventSideEffectRecorded {
		t.Fatalf("events=%+v, want attempted and terminal side-effect events", events)
	}
}

func TestExecutorRecordsAmbiguousCommitAndStopsRetry(t *testing.T) {
	store := &memoryStore{}
	events := []string{}
	manager := NewManager(Dependencies{
		Enabled: true,
		Store:   store,
		Clock:   func() time.Time { return time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC) },
		LedgerEventSink: func(_ context.Context, eventName string, _ SideEffectLedgerEntry) {
			events = append(events, eventName)
		},
	})
	entry, err := manager.ExecuteSideEffect(context.Background(), SideEffectExecutionInput{
		Attempt:        Attempt{ValidationID: "lv_1", TenantID: "ten_1", CandidateID: "candidate_1"},
		ToolClass:      ToolClassMailSend,
		ActionRef:      "send_1",
		AmbiguousCause: AmbiguousCauseTimeout,
	})
	if err != nil {
		t.Fatalf("ExecuteSideEffect returned error: %v", err)
	}
	if entry.Outcome != LedgerOutcomeOperatorActionNeeded || !entry.AmbiguousCommit {
		t.Fatalf("entry=%+v, want operator-action-needed ambiguous commit", entry)
	}
	if len(events) != 2 || events[1] != LedgerEventOperatorActionNeeded {
		t.Fatalf("events=%+v, want operator-action-needed terminal event", events)
	}
}
