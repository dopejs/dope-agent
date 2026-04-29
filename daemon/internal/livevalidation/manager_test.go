package livevalidation

import (
	"context"
	"time"
)

func fixedClock() time.Time {
	return time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
}

type memoryStore struct {
	attempts     []Attempt
	ledger       []SideEffectLedgerEntry
	killSwitches []KillSwitch
	comparisons  []Comparison
}

func (s *memoryStore) UpsertLiveValidationAttempt(_ context.Context, item Attempt) error {
	for i := range s.attempts {
		if s.attempts[i].ValidationID == item.ValidationID {
			s.attempts[i] = item
			return nil
		}
	}
	s.attempts = append(s.attempts, item)
	return nil
}

func (s *memoryStore) GetLiveValidationAttempt(_ context.Context, tenantID, validationID string) (Attempt, bool, error) {
	for _, item := range s.attempts {
		if item.ValidationID == validationID && (tenantID == "" || item.TenantID == tenantID) {
			return item, true, nil
		}
	}
	return Attempt{}, false, nil
}

func (s *memoryStore) ListLiveValidationAttempts(_ context.Context, filter AttemptFilter) ([]Attempt, error) {
	return append([]Attempt(nil), s.attempts...), nil
}

func (s *memoryStore) UpsertLiveValidationScope(context.Context, SideEffectScope, string) error {
	return nil
}

func (s *memoryStore) UpsertLiveValidationApproval(context.Context, FreshApproval) error {
	return nil
}

func (s *memoryStore) AppendLiveValidationLedgerEntry(_ context.Context, item SideEffectLedgerEntry) error {
	s.ledger = append(s.ledger, item)
	return nil
}

func (s *memoryStore) UpdateLiveValidationLedgerEntryOutcome(_ context.Context, ledgerEntryID string, outcome LedgerOutcome, reasonCode string) error {
	for i := range s.ledger {
		if s.ledger[i].LedgerEntryID == ledgerEntryID {
			if err := ValidateLedgerTransition(s.ledger[i].Outcome, outcome); err != nil {
				return err
			}
			s.ledger[i].Outcome = outcome
			s.ledger[i].ReasonCode = reasonCode
			return nil
		}
	}
	return nil
}

func (s *memoryStore) ListLiveValidationLedgerEntries(context.Context, LedgerFilter) ([]SideEffectLedgerEntry, error) {
	return append([]SideEffectLedgerEntry(nil), s.ledger...), nil
}

func (s *memoryStore) UpsertLiveValidationKillSwitch(_ context.Context, item KillSwitch) error {
	s.killSwitches = append(s.killSwitches, item)
	return nil
}

func (s *memoryStore) ListLiveValidationKillSwitches(_ context.Context, filter KillSwitchFilter) ([]KillSwitch, error) {
	items := make([]KillSwitch, 0, len(s.killSwitches))
	for _, item := range s.killSwitches {
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *memoryStore) UpsertLiveValidationSupportMatrixSnapshot(context.Context, string, string, []MatrixRow) error {
	return nil
}

func (s *memoryStore) SaveLiveValidationAmbiguousCommit(context.Context, AmbiguousCommit) error {
	return nil
}

func (s *memoryStore) SaveLiveValidationReconciliationResolution(context.Context, ReconciliationResolution) error {
	return nil
}

func (s *memoryStore) SaveLiveValidationComparison(_ context.Context, item Comparison) error {
	s.comparisons = append(s.comparisons, item)
	return nil
}

func (s *memoryStore) ListLiveValidationComparisons(context.Context, ComparisonFilter) ([]Comparison, error) {
	return append([]Comparison(nil), s.comparisons...), nil
}

func (s *memoryStore) SaveLiveValidationRetentionPolicy(context.Context, RetentionPolicy) error {
	return nil
}
