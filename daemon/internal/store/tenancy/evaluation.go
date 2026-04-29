package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Evaluation is the tenant-aware accessor for the evaluation +
// regression-harness family: evaluation_replay_candidates,
// evaluation_replay_attempts, evaluation_comparisons,
// evaluation_regression_fixtures.
type Evaluation struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewEvaluation(s *store.SQLiteStore, emitter *audit.Emitter) *Evaluation {
	return &Evaluation{store: s, emitter: emitter}
}

func (a *Evaluation) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *Evaluation) ensureTenantRow(ctx context.Context, table, pkColumn, pk, tenantID, surface, resourceKind string) error {
	existing, ok, err := a.store.LookupRowTenant(ctx, table, pkColumn, pk)
	if err != nil {
		return err
	}
	if ok && existing != "" && existing != tenantID {
		a.emit(ctx, surface, resourceKind)
		return ErrCrossTenantWrite
	}
	return nil
}

func (a *Evaluation) UpsertReplayCandidateForTenant(ctx context.Context, item evaluation.ReplayCandidate) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertReplayCandidate(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "evaluation_replay_candidates", "candidate_id", item.CandidateID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertReplayCandidateForTenant", "evaluation_replay_candidate")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertReplayAttemptForTenant(ctx context.Context, item evaluation.ReplayAttempt) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertReplayAttempt(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "evaluation_replay_attempts", "attempt_id", item.AttemptID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertReplayAttemptForTenant", "evaluation_replay_attempt")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertComparisonResultForTenant(ctx context.Context, item evaluation.ComparisonResult) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertComparisonResult(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "evaluation_comparisons", "comparison_id", item.ComparisonID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertComparisonResultForTenant", "evaluation_comparison")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertRegressionFixtureForTenant(ctx context.Context, item evaluation.RegressionFixture) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertRegressionFixture(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "evaluation_regression_fixtures", "fixture_id", item.FixtureID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertRegressionFixtureForTenant", "evaluation_regression_fixture")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertLiveValidationAttemptForTenant(ctx context.Context, item livevalidation.Attempt) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "live_validation_attempts", "validation_id", item.ValidationID, tenantID, "store:UpsertLiveValidationAttemptForTenant", "live_validation_attempt"); err != nil {
		return err
	}
	if err := a.store.UpsertLiveValidationAttempt(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_attempts", "validation_id", item.ValidationID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertLiveValidationAttemptForTenant", "live_validation_attempt")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertLiveValidationScopeForTenant(ctx context.Context, item livevalidation.SideEffectScope) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.ensureTenantRow(ctx, "live_validation_scopes", "scope_id", item.ScopeID, tenantID, "store:UpsertLiveValidationScopeForTenant", "live_validation_scope"); err != nil {
		return err
	}
	if err := a.store.UpsertLiveValidationScope(ctx, item, tenantID); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_scopes", "scope_id", item.ScopeID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertLiveValidationScopeForTenant", "live_validation_scope")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertLiveValidationApprovalForTenant(ctx context.Context, item livevalidation.FreshApproval) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "live_validation_approvals", "approval_id", item.ApprovalID, tenantID, "store:UpsertLiveValidationApprovalForTenant", "live_validation_approval"); err != nil {
		return err
	}
	if err := a.store.UpsertLiveValidationApproval(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_approvals", "approval_id", item.ApprovalID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertLiveValidationApprovalForTenant", "live_validation_approval")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertLiveValidationSupportMatrixSnapshotForTenant(ctx context.Context, snapshotID string, rows []livevalidation.MatrixRow) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.ensureTenantRow(ctx, "live_validation_support_matrix_snapshots", "snapshot_id", snapshotID, tenantID, "store:UpsertLiveValidationSupportMatrixSnapshotForTenant", "live_validation_support_matrix_snapshot"); err != nil {
		return err
	}
	if err := a.store.UpsertLiveValidationSupportMatrixSnapshot(ctx, tenantID, snapshotID, rows); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_support_matrix_snapshots", "snapshot_id", snapshotID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertLiveValidationSupportMatrixSnapshotForTenant", "live_validation_support_matrix_snapshot")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) AppendLiveValidationLedgerEntryForTenant(ctx context.Context, item livevalidation.SideEffectLedgerEntry) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "live_validation_ledger_entries", "ledger_entry_id", item.LedgerEntryID, tenantID, "store:AppendLiveValidationLedgerEntryForTenant", "live_validation_ledger_entry"); err != nil {
		return err
	}
	if err := a.store.AppendLiveValidationLedgerEntry(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_ledger_entries", "ledger_entry_id", item.LedgerEntryID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:AppendLiveValidationLedgerEntryForTenant", "live_validation_ledger_entry")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) UpsertLiveValidationKillSwitchForTenant(ctx context.Context, item livevalidation.KillSwitch) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.Scope = livevalidation.KillSwitchScopeTenant
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "live_validation_kill_switches", "kill_switch_id", item.KillSwitchID, tenantID, "store:UpsertLiveValidationKillSwitchForTenant", "live_validation_kill_switch"); err != nil {
		return err
	}
	if err := a.store.UpsertLiveValidationKillSwitch(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_kill_switches", "kill_switch_id", item.KillSwitchID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertLiveValidationKillSwitchForTenant", "live_validation_kill_switch")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) SaveLiveValidationAmbiguousCommitForTenant(ctx context.Context, item livevalidation.AmbiguousCommit) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "live_validation_ambiguous_commits", "ambiguous_commit_id", item.AmbiguousCommitID, tenantID, "store:SaveLiveValidationAmbiguousCommitForTenant", "live_validation_ambiguous_commit"); err != nil {
		return err
	}
	if err := a.store.SaveLiveValidationAmbiguousCommit(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_ambiguous_commits", "ambiguous_commit_id", item.AmbiguousCommitID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveLiveValidationAmbiguousCommitForTenant", "live_validation_ambiguous_commit")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) SaveLiveValidationReconciliationResolutionForTenant(ctx context.Context, item livevalidation.ReconciliationResolution) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "live_validation_reconciliation_resolutions", "reconciliation_id", item.ReconciliationID, tenantID, "store:SaveLiveValidationReconciliationResolutionForTenant", "live_validation_reconciliation_resolution"); err != nil {
		return err
	}
	if err := a.store.SaveLiveValidationReconciliationResolution(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_reconciliation_resolutions", "reconciliation_id", item.ReconciliationID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveLiveValidationReconciliationResolutionForTenant", "live_validation_reconciliation_resolution")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) SaveLiveValidationComparisonForTenant(ctx context.Context, item livevalidation.Comparison) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.ensureTenantRow(ctx, "live_validation_comparisons", "comparison_id", item.ComparisonID, tenantID, "store:SaveLiveValidationComparisonForTenant", "live_validation_comparison"); err != nil {
		return err
	}
	if err := a.store.SaveLiveValidationComparison(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_comparisons", "comparison_id", item.ComparisonID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveLiveValidationComparisonForTenant", "live_validation_comparison")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Evaluation) SaveLiveValidationRetentionPolicyForTenant(ctx context.Context, item livevalidation.RetentionPolicy) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.ensureTenantRow(ctx, "live_validation_retention_policies", "policy_id", item.PolicyID, tenantID, "store:SaveLiveValidationRetentionPolicyForTenant", "live_validation_retention_policy"); err != nil {
		return err
	}
	if err := a.store.SaveLiveValidationRetentionPolicy(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "live_validation_retention_policies", "policy_id", item.PolicyID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveLiveValidationRetentionPolicyForTenant", "live_validation_retention_policy")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
