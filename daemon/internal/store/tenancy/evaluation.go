package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
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
