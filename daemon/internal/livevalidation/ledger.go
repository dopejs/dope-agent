package livevalidation

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type LedgerOutcome string

const (
	LedgerOutcomeAttempted            LedgerOutcome = "attempted"
	LedgerOutcomeSkipped              LedgerOutcome = "skipped"
	LedgerOutcomeCompleted            LedgerOutcome = "completed"
	LedgerOutcomeFailed               LedgerOutcome = "failed"
	LedgerOutcomeAborted              LedgerOutcome = "aborted"
	LedgerOutcomeDenied               LedgerOutcome = "denied"
	LedgerOutcomeOperatorActionNeeded LedgerOutcome = "operator_action_needed"
)

var (
	ErrLedgerTransitionInvalid = errors.New("invalid live validation ledger transition")
	ErrLedgerOutcomeUnknown    = errors.New("unknown live validation ledger outcome")
)

func IsTerminalLedgerOutcome(outcome LedgerOutcome) bool {
	switch outcome {
	case LedgerOutcomeSkipped, LedgerOutcomeCompleted, LedgerOutcomeAborted, LedgerOutcomeDenied, LedgerOutcomeOperatorActionNeeded:
		return true
	case LedgerOutcomeFailed:
		return true
	default:
		return false
	}
}

func ValidateLedgerTransition(from, to LedgerOutcome) error {
	if !knownLedgerOutcome(to) {
		return fmt.Errorf("%w: %s", ErrLedgerOutcomeUnknown, to)
	}
	if from == "" {
		return nil
	}
	if !knownLedgerOutcome(from) {
		return fmt.Errorf("%w: %s", ErrLedgerOutcomeUnknown, from)
	}
	if from == to {
		return nil
	}
	switch from {
	case LedgerOutcomeAttempted:
		switch to {
		case LedgerOutcomeCompleted, LedgerOutcomeFailed, LedgerOutcomeAborted, LedgerOutcomeOperatorActionNeeded:
			return nil
		}
	case LedgerOutcomeFailed:
		if to == LedgerOutcomeAttempted {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrLedgerTransitionInvalid, from, to)
}

func knownLedgerOutcome(outcome LedgerOutcome) bool {
	switch outcome {
	case LedgerOutcomeAttempted, LedgerOutcomeSkipped, LedgerOutcomeCompleted, LedgerOutcomeFailed, LedgerOutcomeAborted, LedgerOutcomeDenied, LedgerOutcomeOperatorActionNeeded:
		return true
	default:
		return false
	}
}

func (m *Manager) AppendLedgerEntry(ctx context.Context, entry SideEffectLedgerEntry) (SideEffectLedgerEntry, error) {
	if entry.Outcome == "" {
		entry.Outcome = LedgerOutcomeAttempted
	}
	if !knownLedgerOutcome(entry.Outcome) {
		return SideEffectLedgerEntry{}, fmt.Errorf("%w: %s", ErrLedgerOutcomeUnknown, entry.Outcome)
	}
	now := m.clock()
	if entry.LedgerEntryID == "" {
		entry.LedgerEntryID = newID("lv_ledger")
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = now
	}
	if entry.AttemptedAt == nil && entry.Outcome == LedgerOutcomeAttempted {
		entry.AttemptedAt = timePtr(now)
	}
	if IsTerminalLedgerOutcome(entry.Outcome) && entry.CompletedAt == nil && entry.Outcome != LedgerOutcomeSkipped && entry.Outcome != LedgerOutcomeDenied {
		entry.CompletedAt = timePtr(now)
	}
	if m.store != nil {
		if err := m.store.AppendLiveValidationLedgerEntry(ctx, entry); err != nil {
			return SideEffectLedgerEntry{}, err
		}
	}
	return entry, nil
}

func (m *Manager) UpdateLedgerOutcome(ctx context.Context, ledgerEntryID string, outcome LedgerOutcome, reasonCode string) error {
	if !knownLedgerOutcome(outcome) {
		return fmt.Errorf("%w: %s", ErrLedgerOutcomeUnknown, outcome)
	}
	if m.store == nil {
		return nil
	}
	return m.store.UpdateLiveValidationLedgerEntryOutcome(ctx, ledgerEntryID, outcome, reasonCode)
}

func (m *Manager) ListLedgerEntries(ctx context.Context, filter LedgerFilter) ([]SideEffectLedgerEntry, error) {
	if m.store == nil {
		return nil, nil
	}
	return m.store.ListLiveValidationLedgerEntries(ctx, filter)
}

func timePtr(value time.Time) *time.Time {
	return &value
}
