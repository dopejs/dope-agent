package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type SlackSetup struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewSlackSetup(s *store.SQLiteStore, emitter *audit.Emitter) *SlackSetup {
	return &SlackSetup{store: s, emitter: emitter}
}

func (a *SlackSetup) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *SlackSetup) SaveHostedSetupForTenant(ctx context.Context, item store.SlackHostedSetupRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.store.SaveSlackHostedSetup(ctx, item); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveSlackHostedSetupForTenant", "slack_hosted_setup")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *SlackSetup) GetHostedSetupForTenant(ctx context.Context, connectorID string) (store.SlackHostedSetupRecord, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return store.SlackHostedSetupRecord{}, false, err
	}
	return a.store.GetSlackHostedSetup(ctx, tenantID, connectorID)
}
