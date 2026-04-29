package livevalidation

import "context"

func (m *Manager) RecordAmbiguousCommit(ctx context.Context, item AmbiguousCommit) (AmbiguousCommit, error) {
	now := m.clock()
	if item.AmbiguousCommitID == "" {
		item.AmbiguousCommitID = newID("lv_ambiguous")
	}
	if item.Cause == "" {
		item.Cause = AmbiguousCauseOther
	}
	item.AutomaticRetryStopped = true
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	if m.store != nil {
		if err := m.store.SaveLiveValidationAmbiguousCommit(ctx, item); err != nil {
			return AmbiguousCommit{}, err
		}
	}
	if item.LedgerEntryID != "" {
		if err := m.UpdateLedgerOutcome(ctx, item.LedgerEntryID, LedgerOutcomeOperatorActionNeeded, "live_validation.ambiguous_commit"); err != nil {
			return AmbiguousCommit{}, err
		}
	}
	return item, nil
}
