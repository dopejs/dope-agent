package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Delivery is the tenant-aware accessor for the delivery family —
// delivery_targets, delivery_preferences, delivery_outcomes,
// delivery_attempts, delivery_summary_windows.
type Delivery struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewDelivery(s *store.SQLiteStore, emitter *audit.Emitter) *Delivery {
	return &Delivery{store: s, emitter: emitter}
}

func (a *Delivery) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *Delivery) UpsertTargetForTenant(ctx context.Context, record store.DeliveryTargetRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertDeliveryTarget(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "delivery_targets", "target_id", record.TargetID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertDeliveryTargetForTenant", "delivery_target")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Delivery) UpsertPreferenceForTenant(ctx context.Context, record store.DeliveryPreferenceRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertDeliveryPreference(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "delivery_preferences", "preference_id", record.PreferenceID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertDeliveryPreferenceForTenant", "delivery_preference")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Delivery) UpsertOutcomeForTenant(ctx context.Context, record store.DeliveryOutcomeRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertDeliveryOutcome(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "delivery_outcomes", "delivery_id", record.DeliveryID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertDeliveryOutcomeForTenant", "delivery_outcome")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Delivery) UpsertAttemptForTenant(ctx context.Context, record store.DeliveryAttemptRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertDeliveryAttempt(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "delivery_attempts", "attempt_id", record.AttemptID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertDeliveryAttemptForTenant", "delivery_attempt")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Delivery) UpsertSummaryWindowForTenant(ctx context.Context, record store.DeliverySummaryWindowRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertDeliverySummaryWindow(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "delivery_summary_windows", "summary_window_id", record.SummaryWindowID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertDeliverySummaryWindowForTenant", "delivery_summary_window")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
