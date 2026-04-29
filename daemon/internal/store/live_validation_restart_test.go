package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestLiveValidationAmbiguousCommitLedgerPersistsAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	if err := s.UpsertLiveValidationAttempt(ctx, livevalidation.Attempt{
		ValidationID:     "lv_restart",
		TenantID:         "ten_restart",
		CandidateID:      "candidate_restart",
		EnvironmentScope: "test",
		Status:           livevalidation.AttemptStatusRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		t.Fatalf("UpsertLiveValidationAttempt: %v", err)
	}
	entry := livevalidation.SideEffectLedgerEntry{
		LedgerEntryID:   "ledger_restart",
		ValidationID:    "lv_restart",
		TenantID:        "ten_restart",
		CandidateID:     "candidate_restart",
		ToolClass:       livevalidation.ToolClassMailSend,
		SafetyClass:     livevalidation.SafetyClassNonIdempotentMutation,
		ActionRef:       "send_restart",
		Outcome:         livevalidation.LedgerOutcomeOperatorActionNeeded,
		AmbiguousCommit: true,
		UpdatedAt:       now,
	}
	if err := s.AppendLiveValidationLedgerEntry(ctx, entry); err != nil {
		t.Fatalf("AppendLiveValidationLedgerEntry: %v", err)
	}
	if err := s.SaveLiveValidationAmbiguousCommit(ctx, livevalidation.AmbiguousCommit{
		AmbiguousCommitID:     "amb_restart",
		LedgerEntryID:         entry.LedgerEntryID,
		ValidationID:          entry.ValidationID,
		TenantID:              entry.TenantID,
		Cause:                 livevalidation.AmbiguousCauseDaemonRestart,
		AutomaticRetryStopped: true,
		CreatedAt:             now,
		UpdatedAt:             now,
	}); err != nil {
		t.Fatalf("SaveLiveValidationAmbiguousCommit: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	items, err := reopened.ListLiveValidationLedgerEntries(ctx, livevalidation.LedgerFilter{TenantID: "ten_restart", ValidationID: "lv_restart"})
	if err != nil {
		t.Fatalf("ListLiveValidationLedgerEntries: %v", err)
	}
	if len(items) != 1 || !items[0].AmbiguousCommit || items[0].Outcome != livevalidation.LedgerOutcomeOperatorActionNeeded {
		t.Fatalf("items=%+v, want persisted ambiguous operator-action-needed ledger", items)
	}
}
