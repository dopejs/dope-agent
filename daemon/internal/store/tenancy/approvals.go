package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Approvals is the tenant-aware accessor for the approvals + decisions
// pair. Pass A semantics mirror tenancy.Runtime.
type Approvals struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewApprovals(s *store.SQLiteStore, emitter *audit.Emitter) *Approvals {
	return &Approvals{store: s, emitter: emitter}
}

func (a *Approvals) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *Approvals) ListApprovalsForTenant(ctx context.Context) ([]policy.Approval, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return a.store.ListApprovalsForTenantRaw(ctx, tenantID)
}

func (a *Approvals) UpsertApprovalForTenant(ctx context.Context, approval policy.Approval) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertApproval(ctx, approval); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "approvals", "approval_id", approval.ApprovalID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertApprovalForTenant", "approval")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Approvals) ListDecisionsForTenant(ctx context.Context) ([]policy.Decision, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return a.store.ListDecisionsForTenantRaw(ctx, tenantID)
}

func (a *Approvals) UpsertDecisionForTenant(ctx context.Context, decision policy.Decision) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertDecision(ctx, decision); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "decisions", "decision_id", decision.DecisionID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertDecisionForTenant", "decision")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
