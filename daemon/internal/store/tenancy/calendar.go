package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Calendar is the tenant-aware accessor for calendar_accounts,
// calendar_operations, calendar_artifacts.
type Calendar struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewCalendar(s *store.SQLiteStore, emitter *audit.Emitter) *Calendar {
	return &Calendar{store: s, emitter: emitter}
}

func (a *Calendar) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *Calendar) UpsertAccountForTenant(ctx context.Context, item calendar.AccountProjection) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertCalendarAccount(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "calendar_accounts", "calendar_account_id", item.CalendarAccountID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertCalendarAccountForTenant", "calendar_account")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Calendar) UpsertOperationForTenant(ctx context.Context, item calendar.Operation) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertCalendarOperation(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "calendar_operations", "operation_id", item.OperationID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertCalendarOperationForTenant", "calendar_operation")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Calendar) UpsertArtifactForTenant(ctx context.Context, item calendar.Artifact) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertCalendarArtifact(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "calendar_artifacts", "artifact_id", item.ArtifactID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertCalendarArtifactForTenant", "calendar_artifact")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
