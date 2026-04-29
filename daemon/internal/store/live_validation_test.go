package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestLiveValidationStoreRoundTripAndRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_live", PrincipalID: "prn_live"})
	now := time.Now().UTC().Truncate(time.Microsecond)
	attempt := livevalidation.Attempt{
		ValidationID:     "lv_attempt_1",
		TenantID:         "ten_live",
		CandidateID:      "candidate_1",
		RequestedBy:      "prn_live",
		EnvironmentScope: "test",
		RequestedScope: livevalidation.SideEffectScope{
			ScopeID:             "lv_scope_1",
			ValidationID:        "lv_attempt_1",
			IncludedToolClasses: []livevalidation.ToolClass{livevalidation.ToolClassDaemonInspectionRead},
			ApprovalMode:        livevalidation.ApprovalModeScopeLevel,
			DeclaredBy:          "prn_live",
			DeclaredAt:          now,
		},
		Status:    livevalidation.AttemptStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.UpsertLiveValidationAttempt(ctx, attempt); err != nil {
		t.Fatalf("UpsertLiveValidationAttempt: %v", err)
	}
	if err := s.UpsertLiveValidationScope(ctx, attempt.RequestedScope, "ten_live"); err != nil {
		t.Fatalf("UpsertLiveValidationScope: %v", err)
	}
	if err := s.UpsertLiveValidationApproval(ctx, livevalidation.FreshApproval{
		ApprovalID:   "lv_approval_1",
		ValidationID: "lv_attempt_1",
		TenantID:     "ten_live",
		Target:       livevalidation.ApprovalTargetScope,
		ToolClass:    livevalidation.ToolClassDaemonInspectionRead,
		SafetyClass:  livevalidation.SafetyClassReadOnly,
		Status:       livevalidation.ApprovalStatusApproved,
		RequestedBy:  "prn_live",
		ResolvedBy:   "prn_owner",
		RequestedAt:  now,
		ResolvedAt:   &now,
	}); err != nil {
		t.Fatalf("UpsertLiveValidationApproval: %v", err)
	}
	if err := s.UpsertLiveValidationSupportMatrixSnapshot(ctx, "ten_live", "lv_matrix_1", livevalidation.DefaultMatrixRows()); err != nil {
		t.Fatalf("UpsertLiveValidationSupportMatrixSnapshot: %v", err)
	}
	ledger := livevalidation.SideEffectLedgerEntry{
		LedgerEntryID: "lv_ledger_1",
		ValidationID:  "lv_attempt_1",
		TenantID:      "ten_live",
		CandidateID:   "candidate_1",
		SourceRef:     "tool_call_1",
		ToolClass:     livevalidation.ToolClassDaemonInspectionRead,
		SafetyClass:   livevalidation.SafetyClassReadOnly,
		ActionRef:     "action_1",
		Outcome:       livevalidation.LedgerOutcomeAttempted,
		AttemptedAt:   &now,
		UpdatedAt:     now,
	}
	if err := s.AppendLiveValidationLedgerEntry(ctx, ledger); err != nil {
		t.Fatalf("AppendLiveValidationLedgerEntry: %v", err)
	}
	if err := s.UpdateLiveValidationLedgerEntryOutcome(ctx, ledger.LedgerEntryID, livevalidation.LedgerOutcomeCompleted, "completed"); err != nil {
		t.Fatalf("UpdateLiveValidationLedgerEntryOutcome: %v", err)
	}
	if err := s.UpsertLiveValidationKillSwitch(ctx, livevalidation.KillSwitch{
		KillSwitchID: "lv_kill_1",
		Scope:        livevalidation.KillSwitchScopeTenant,
		TenantID:     "ten_live",
		Enabled:      false,
		Reason:       "test",
		ChangedBy:    "prn_live",
		ChangedAt:    now,
	}); err != nil {
		t.Fatalf("UpsertLiveValidationKillSwitch: %v", err)
	}
	if err := s.SaveLiveValidationAmbiguousCommit(ctx, livevalidation.AmbiguousCommit{
		AmbiguousCommitID:     "lv_ambiguous_1",
		LedgerEntryID:         ledger.LedgerEntryID,
		ValidationID:          attempt.ValidationID,
		TenantID:              "ten_live",
		Cause:                 livevalidation.AmbiguousCauseTimeout,
		AutomaticRetryStopped: true,
		CreatedAt:             now,
		UpdatedAt:             now,
	}); err != nil {
		t.Fatalf("SaveLiveValidationAmbiguousCommit: %v", err)
	}
	if err := s.SaveLiveValidationReconciliationResolution(ctx, livevalidation.ReconciliationResolution{
		ReconciliationID:  "lv_reconcile_1",
		AmbiguousCommitID: "lv_ambiguous_1",
		TenantID:          "ten_live",
		ResolvedBy:        "prn_owner",
		Resolution:        livevalidation.ResolutionConfirmedCommitted,
		Reason:            "verified provider state",
		ResolvedAt:        now,
	}); err != nil {
		t.Fatalf("SaveLiveValidationReconciliationResolution: %v", err)
	}
	if err := s.SaveLiveValidationComparison(ctx, livevalidation.Comparison{
		ComparisonID:   "lv_comparison_1",
		ValidationID:   attempt.ValidationID,
		CandidateID:    attempt.CandidateID,
		BaselineRef:    "baseline_1",
		TerminalStatus: livevalidation.ComparisonStatusMatched,
		LedgerSummary:  livevalidation.LedgerSummary{livevalidation.LedgerOutcomeCompleted: 1},
		GeneratedAt:    now,
	}); err != nil {
		t.Fatalf("SaveLiveValidationComparison: %v", err)
	}
	if err := s.SaveLiveValidationRetentionPolicy(ctx, livevalidation.RetentionPolicy{
		PolicyID:             "lv_retention_1",
		TenantID:             "ten_live",
		AppliesTo:            livevalidation.RetentionAppliesAll,
		Mode:                 livevalidation.RetentionModeIndefinite,
		CreatedByPrincipalID: "prn_live",
		CreatedAt:            now,
	}); err != nil {
		t.Fatalf("SaveLiveValidationRetentionPolicy: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	got, ok, err := reopened.GetLiveValidationAttempt(ctx, "ten_live", attempt.ValidationID)
	if err != nil {
		t.Fatalf("GetLiveValidationAttempt: %v", err)
	}
	if !ok || got.ValidationID != attempt.ValidationID {
		t.Fatalf("expected restarted attempt %s, got ok=%v item=%+v", attempt.ValidationID, ok, got)
	}
	entries, err := reopened.ListLiveValidationLedgerEntries(ctx, livevalidation.LedgerFilter{TenantID: "ten_live", ValidationID: attempt.ValidationID})
	if err != nil {
		t.Fatalf("ListLiveValidationLedgerEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].Outcome != livevalidation.LedgerOutcomeCompleted {
		t.Fatalf("expected completed ledger after restart, got %+v", entries)
	}
	comparisons, err := reopened.ListLiveValidationComparisons(ctx, livevalidation.ComparisonFilter{TenantID: "ten_live", ValidationID: attempt.ValidationID})
	if err != nil {
		t.Fatalf("ListLiveValidationComparisons: %v", err)
	}
	if len(comparisons) != 1 || comparisons[0].TerminalStatus != livevalidation.ComparisonStatusMatched {
		t.Fatalf("expected comparison after restart, got %+v", comparisons)
	}
}
