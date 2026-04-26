package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// ComputerUse is the tenant-aware accessor for computer_use_sessions,
// computer_use_actions, computer_use_artifacts. Tenant ownership is
// derived from the parent run via the NOT NULL run_id FK during the
// US2 backfill (T067 + per-domain dependency in T077).
type ComputerUse struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewComputerUse(s *store.SQLiteStore, emitter *audit.Emitter) *ComputerUse {
	return &ComputerUse{store: s, emitter: emitter}
}

func (a *ComputerUse) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *ComputerUse) UpsertSessionForTenant(ctx context.Context, session computeruse.Session) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertComputerUseSession(ctx, session); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "computer_use_sessions", "computer_use_session_id", session.ComputerUseSessionID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertComputerUseSessionForTenant", "computer_use_session")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *ComputerUse) UpsertActionForTenant(ctx context.Context, action computeruse.Action) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertComputerUseAction(ctx, action); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "computer_use_actions", "computer_use_action_id", action.ComputerUseActionID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertComputerUseActionForTenant", "computer_use_action")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *ComputerUse) UpsertArtifactForTenant(ctx context.Context, artifact computeruse.Artifact) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertComputerUseArtifact(ctx, artifact); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "computer_use_artifacts", "artifact_id", artifact.ArtifactID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertComputerUseArtifactForTenant", "computer_use_artifact")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
