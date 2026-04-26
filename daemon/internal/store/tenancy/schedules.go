package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Schedules is the tenant-aware accessor for the schedules family
// (schedules, schedule_targets, schedule_dispatch_attempts). Pass A
// semantics mirror tenancy.Runtime: fail-closed reads, write-then-bind
// upserts, audit emission on cross-tenant lookup.
type Schedules struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

// NewSchedules constructs the accessor.
func NewSchedules(s *store.SQLiteStore, emitter *audit.Emitter) *Schedules {
	return &Schedules{store: s, emitter: emitter}
}

func (a *Schedules) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

// ----- schedules -----

func (a *Schedules) ListSchedulesForTenant(ctx context.Context, environmentScope string) ([]store.ScheduleRecord, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return a.store.ListSchedulesForTenantRaw(ctx, tenantID, environmentScope)
}

func (a *Schedules) GetScheduleForTenant(ctx context.Context, environmentScope, scheduleID string) (store.ScheduleRecord, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return store.ScheduleRecord{}, false, err
	}
	owner, ok, err := a.store.LookupRowTenant(ctx, "schedules", "schedule_id", scheduleID)
	if err != nil {
		return store.ScheduleRecord{}, false, err
	}
	if !ok {
		return store.ScheduleRecord{}, false, nil
	}
	if owner != "" && owner != tenantID {
		a.emit(ctx, "store:GetScheduleForTenant", "schedule")
		return store.ScheduleRecord{}, false, nil
	}
	return a.store.GetSchedule(ctx, environmentScope, scheduleID)
}

func (a *Schedules) UpsertScheduleForTenant(ctx context.Context, record store.ScheduleRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertSchedule(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "schedules", "schedule_id", record.ScheduleID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertScheduleForTenant", "schedule")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

// ----- schedule_targets -----

func (a *Schedules) UpsertScheduleTargetForTenant(ctx context.Context, record store.ScheduleTargetRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertScheduleTarget(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "schedule_targets", "target_ref_id", record.TargetRefID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertScheduleTargetForTenant", "schedule_target")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Schedules) GetScheduleTargetForTenant(ctx context.Context, scheduleID, targetRefID string) (store.ScheduleTargetRecord, bool, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return store.ScheduleTargetRecord{}, false, err
	}
	owner, ok, err := a.store.LookupRowTenant(ctx, "schedule_targets", "target_ref_id", targetRefID)
	if err != nil {
		return store.ScheduleTargetRecord{}, false, err
	}
	if !ok {
		return store.ScheduleTargetRecord{}, false, nil
	}
	if owner != "" && owner != tenantID {
		a.emit(ctx, "store:GetScheduleTargetForTenant", "schedule_target")
		return store.ScheduleTargetRecord{}, false, nil
	}
	return a.store.GetScheduleTarget(ctx, scheduleID, targetRefID)
}

// ----- schedule_dispatch_attempts -----

func (a *Schedules) UpsertScheduleDispatchAttemptForTenant(ctx context.Context, record store.ScheduleDispatchAttemptRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertScheduleDispatchAttempt(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "schedule_dispatch_attempts", "attempt_id", record.AttemptID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertScheduleDispatchAttemptForTenant", "schedule_dispatch_attempt")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Schedules) ListScheduleDispatchAttemptsForTenant(ctx context.Context, scheduleID string) ([]store.ScheduleDispatchAttemptRecord, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	owner, ok, err := a.store.LookupRowTenant(ctx, "schedules", "schedule_id", scheduleID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	if owner != "" && owner != tenantID {
		a.emit(ctx, "store:ListScheduleDispatchAttemptsForTenant", "schedule")
		return nil, nil
	}
	return a.store.ListScheduleDispatchAttempts(ctx, scheduleID)
}
