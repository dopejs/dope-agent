package livevalidation

import (
	"errors"
	"testing"
)

func TestLedgerTransitions(t *testing.T) {
	valid := []LedgerOutcome{LedgerOutcomeCompleted, LedgerOutcomeFailed, LedgerOutcomeAborted, LedgerOutcomeOperatorActionNeeded}
	for _, outcome := range valid {
		if err := ValidateLedgerTransition(LedgerOutcomeAttempted, outcome); err != nil {
			t.Fatalf("attempted -> %s returned error: %v", outcome, err)
		}
	}
	if err := ValidateLedgerTransition(LedgerOutcomeCompleted, LedgerOutcomeAttempted); !errors.Is(err, ErrLedgerTransitionInvalid) {
		t.Fatalf("completed -> attempted err=%v, want invalid transition", err)
	}
	if err := ValidateLedgerTransition("", LedgerOutcomeSkipped); err != nil {
		t.Fatalf("initial skipped transition returned error: %v", err)
	}
}
