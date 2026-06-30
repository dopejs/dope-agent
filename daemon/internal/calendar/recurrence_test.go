package calendar

import (
	"errors"
	"testing"
	"time"
)

// US1 (FR-005, SC-001): an all-day create preserves date boundaries.
func TestAllDayCreatePreservesDateBoundaries(t *testing.T) {
	t.Parallel()
	m := NewManager("test")
	resources := calendarTestResources()

	_, ev, _, _, err := m.CreateEvent(resources, CreateEventInput{
		Selection: Selection{IntegrationID: "calendar-a"},
		Title:     "Company holiday",
		AllDay:    true,
		StartDate: "2026-03-08", // US DST spring-forward day
		EndDate:   "2026-03-09",
	})
	if err != nil {
		t.Fatalf("all-day create: %v", err)
	}
	if !ev.AllDay || ev.StartDate != "2026-03-08" || ev.EndDate != "2026-03-09" {
		t.Fatalf("all-day boundaries not preserved: %+v", ev)
	}
	if !ev.StartsAt.Equal(time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("absolute start wrong for all-day: %v", ev.StartsAt)
	}
}

// US2 (FR-002, FR-004, SC-002/SC-004): recurring mutations require scope, reject invalid/ambiguous
// scope, and record scope + original/resulting identities.
func TestRecurrenceScopeRulesAndIdentities(t *testing.T) {
	t.Parallel()
	m := NewManager("test")
	resources := calendarTestResources()
	now := time.Now().UTC()

	_, rec, _, _, err := m.CreateEvent(resources, CreateEventInput{
		Selection:      Selection{IntegrationID: "calendar-a"},
		Title:          "Weekly sync",
		StartsAt:       now.Add(time.Hour),
		EndsAt:         now.Add(2 * time.Hour),
		Recurring:      true,
		RecurrenceRule: "FREQ=WEEKLY;BYDAY=MO",
	})
	if err != nil {
		t.Fatalf("recurring create: %v", err)
	}
	if !rec.Recurring || rec.SeriesID == "" {
		t.Fatalf("series identity not set: %+v", rec)
	}

	// Missing scope on a recurring update is rejected backend-side.
	if _, _, _, _, err := m.UpdateEvent(resources, UpdateEventInput{
		Selection: Selection{IntegrationID: "calendar-a"}, ExternalEventID: rec.ExternalEventID,
		Title: "x", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
	}); !errors.Is(err, ErrCalendarRecurrenceScopeRequired) {
		t.Fatalf("expected scope-required, got %v", err)
	}

	// Invalid scope is rejected at the manager.
	if _, _, _, _, err := m.UpdateEvent(resources, UpdateEventInput{
		Selection: Selection{IntegrationID: "calendar-a"}, ExternalEventID: rec.ExternalEventID,
		Title: "x", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
		RecurrenceScope: RecurrenceScope("bogus"),
	}); !errors.Is(err, ErrCalendarRecurrenceScopeInvalid) {
		t.Fatalf("expected scope-invalid, got %v", err)
	}

	// this_occurrence update records scope + occurrence identity + original id.
	_, occ, op, _, err := m.UpdateEvent(resources, UpdateEventInput{
		Selection: Selection{IntegrationID: "calendar-a"}, ExternalEventID: rec.ExternalEventID,
		Title: "moved", StartsAt: now.Add(3 * time.Hour), EndsAt: now.Add(4 * time.Hour),
		RecurrenceScope: RecurrenceScopeThisOccurrence,
	})
	if err != nil {
		t.Fatalf("occurrence update: %v", err)
	}
	if op.RecurrenceScope != RecurrenceScopeThisOccurrence || op.OriginalExternalEventID != rec.ExternalEventID {
		t.Fatalf("operation identity not recorded: %+v", op)
	}
	if occ.OccurrenceID == "" || occ.OriginalStartsAt == nil {
		t.Fatalf("occurrence identity not set: %+v", occ)
	}

	// entire_series cancel records scope and cancels the series.
	_, can, cop, _, err := m.CancelEvent(resources, CancelEventInput{
		Selection: Selection{IntegrationID: "calendar-a"}, ExternalEventID: rec.ExternalEventID,
		RecurrenceScope: RecurrenceScopeEntireSeries,
	})
	if err != nil {
		t.Fatalf("series cancel: %v", err)
	}
	if can.LifecycleState != EventLifecycleStateCancelled || cop.RecurrenceScope != RecurrenceScopeEntireSeries {
		t.Fatalf("series cancel not recorded: event=%+v op=%+v", can, cop)
	}
}
