package livevalidation

import "context"

func (m *Manager) CreateComparison(ctx context.Context, validationID string) (Comparison, error) {
	attempt, ok, err := m.GetAttempt(ctx, validationID)
	if err != nil {
		return Comparison{}, err
	}
	if !ok {
		return Comparison{}, ErrLiveValidationDisabled
	}
	ledger, err := m.ListLedgerEntries(ctx, LedgerFilter{TenantID: attempt.TenantID, ValidationID: validationID})
	if err != nil {
		return Comparison{}, err
	}
	summary := LedgerSummary{}
	unsupported := []ToolClass{}
	ambiguous := []string{}
	for _, entry := range ledger {
		summary[entry.Outcome]++
		if entry.Outcome == LedgerOutcomeDenied || entry.Outcome == LedgerOutcomeSkipped {
			if entry.SafetyClass == SafetyClassUnsupported {
				unsupported = append(unsupported, entry.ToolClass)
			}
		}
		if entry.AmbiguousCommit {
			ambiguous = append(ambiguous, entry.LedgerEntryID)
		}
	}
	status := ComparisonStatusMatched
	if summary[LedgerOutcomeOperatorActionNeeded] > 0 || len(ambiguous) > 0 {
		status = ComparisonStatusOperatorActionNeeded
	} else if summary[LedgerOutcomeDenied] > 0 {
		status = ComparisonStatusBlocked
	} else if len(unsupported) > 0 {
		status = ComparisonStatusUnsupported
	} else if summary[LedgerOutcomeFailed] > 0 {
		status = ComparisonStatusDrifted
	}
	comparison := Comparison{
		ComparisonID:       newID("lv_comparison"),
		ValidationID:       validationID,
		CandidateID:        attempt.CandidateID,
		BaselineRef:        attempt.SourceAttemptID,
		TerminalStatus:     status,
		LedgerSummary:      summary,
		UnsupportedClasses: unsupported,
		AmbiguousCommits:   ambiguous,
		GeneratedAt:        m.clock(),
	}
	if m.store != nil {
		if err := m.store.SaveLiveValidationComparison(ctx, comparison); err != nil {
			return Comparison{}, err
		}
	}
	return comparison, nil
}
