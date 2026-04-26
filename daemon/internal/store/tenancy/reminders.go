package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Reminders is the tenant-aware accessor for reminders,
// reminder_occurrences, reminder_actions.
type Reminders struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewReminders(s *store.SQLiteStore, emitter *audit.Emitter) *Reminders {
	return &Reminders{store: s, emitter: emitter}
}

func (a *Reminders) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *Reminders) UpsertReminderForTenant(ctx context.Context, record store.ReminderRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertReminder(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "reminders", "reminder_id", record.ReminderID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertReminderForTenant", "reminder")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Reminders) UpsertOccurrenceForTenant(ctx context.Context, record store.ReminderOccurrenceRecord) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertReminderOccurrence(ctx, record); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "reminder_occurrences", "occurrence_id", record.OccurrenceID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertReminderOccurrenceForTenant", "reminder_occurrence")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
