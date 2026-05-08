package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type TelegramSetup struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewTelegramSetup(s *store.SQLiteStore, emitter *audit.Emitter) *TelegramSetup {
	return &TelegramSetup{store: s, emitter: emitter}
}

func (a *TelegramSetup) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *TelegramSetup) SaveHostedSetupForTenant(ctx context.Context, item store.TelegramHostedSetupRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	item.TenantID = tenantID
	if err := a.store.SaveTelegramHostedSetup(ctx, item); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:SaveTelegramHostedSetupForTenant", "telegram_hosted_setup")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *TelegramSetup) GetHostedSetupForTenant(ctx context.Context, connectorID string) (store.TelegramHostedSetupRecord, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return store.TelegramHostedSetupRecord{}, false, err
	}
	return a.store.GetTelegramHostedSetup(ctx, tenantID, connectorID)
}
