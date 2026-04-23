package calendar

import (
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func calendarTestResources() []integrations.Resource {
	now := time.Now().UTC()
	return []integrations.Resource{
		{
			IntegrationID:    "calendar-a",
			DomainKind:       "calendar",
			DisplayName:      "Calendar A",
			EnvironmentScope: "test",
			ReadinessStatus:  integrations.ReadinessStatusHealthy,
			CanonicalDefault: true,
			AccountBinding: integrations.AccountBinding{
				AccountKey:   "acct_primary",
				AccountLabel: "Primary",
			},
			BackendBinding: integrations.BackendBinding{
				BackendKind: integrations.BackendKindFakeLocal,
			},
			CreatedAt:        now,
			UpdatedAt:        now,
			LastTransitionAt: now,
		},
		{
			IntegrationID:    "calendar-b",
			DomainKind:       "calendar",
			DisplayName:      "Calendar B",
			EnvironmentScope: "test",
			ReadinessStatus:  integrations.ReadinessStatusHealthy,
			CanonicalDefault: false,
			AccountBinding: integrations.AccountBinding{
				AccountKey:   "acct_primary",
				AccountLabel: "Primary",
			},
			BackendBinding: integrations.BackendBinding{
				BackendKind: integrations.BackendKindFakeLocal,
			},
			CreatedAt:        now,
			UpdatedAt:        now,
			LastTransitionAt: now,
		},
	}
}

func TestCalendarManagerUsesExplicitSelectionAndCanonicalDefaultFallback(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	resources := calendarTestResources()

	accounts, err := manager.ListAccounts(resources, Selection{})
	if err != nil {
		t.Fatalf("ListAccounts(default) returned error: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 account projections, got %d", len(accounts))
	}

	account, events, operation, _, err := manager.ListEvents(resources, ListEventsInput{})
	if err != nil {
		t.Fatalf("ListEvents(default) returned error: %v", err)
	}
	if account.IntegrationID != "calendar-a" || operation.SelectionMode != "canonical_default" {
		t.Fatalf("expected canonical-default selection, got account=%+v operation=%+v", account, operation)
	}
	if len(events) == 0 {
		t.Fatal("expected seeded fake event")
	}

	explicitAccount, _, explicitOperation, _, err := manager.ListEvents(resources, ListEventsInput{
		Selection: Selection{IntegrationID: "calendar-b"},
	})
	if err != nil {
		t.Fatalf("ListEvents(explicit) returned error: %v", err)
	}
	if explicitAccount.IntegrationID != "calendar-b" || explicitOperation.SelectionMode != "explicit" {
		t.Fatalf("expected explicit selection, got account=%+v operation=%+v", explicitAccount, explicitOperation)
	}
}

func TestCalendarManagerRejectsRecurringAllDayAndAttendeeMutation(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	resources := calendarTestResources()
	now := time.Now().UTC()

	_, _, _, _, err := manager.CreateEvent(resources, CreateEventInput{
		Title:     "Recurring",
		StartsAt:  now.Add(time.Hour),
		EndsAt:    now.Add(2 * time.Hour),
		Recurring: true,
	})
	if !errors.Is(err, ErrCalendarRecurringUnsupported) {
		t.Fatalf("expected recurring unsupported, got %v", err)
	}

	_, _, _, _, err = manager.CreateEvent(resources, CreateEventInput{
		Title:    "All Day",
		StartsAt: now.Add(time.Hour),
		EndsAt:   now.Add(2 * time.Hour),
		AllDay:   true,
	})
	if !errors.Is(err, ErrCalendarAllDayUnsupported) {
		t.Fatalf("expected all-day unsupported, got %v", err)
	}

	_, _, _, _, err = manager.CreateEvent(resources, CreateEventInput{
		Title:     "With Attendee",
		StartsAt:  now.Add(time.Hour),
		EndsAt:    now.Add(2 * time.Hour),
		Attendees: []string{"bob@example.com"},
	})
	if !errors.Is(err, ErrCalendarAttendeesUnsupported) {
		t.Fatalf("expected attendee unsupported, got %v", err)
	}
}

func TestCalendarManagerPreservesEventIdentityAcrossUpdateAndCancel(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	resources := calendarTestResources()
	now := time.Now().UTC()

	_, created, _, _, err := manager.CreateEvent(resources, CreateEventInput{
		Selection: Selection{IntegrationID: "calendar-a"},
		Title:     "Created",
		StartsAt:  now.Add(time.Hour),
		EndsAt:    now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateEvent returned error: %v", err)
	}

	_, updated, _, _, err := manager.UpdateEvent(resources, UpdateEventInput{
		Selection:       Selection{IntegrationID: "calendar-a"},
		ExternalEventID: created.ExternalEventID,
		Title:           "Updated",
		StartsAt:        now.Add(2 * time.Hour),
		EndsAt:          now.Add(3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("UpdateEvent returned error: %v", err)
	}
	if updated.ExternalEventID != created.ExternalEventID {
		t.Fatalf("expected stable event identity, got create=%s update=%s", created.ExternalEventID, updated.ExternalEventID)
	}

	_, cancelled, _, _, err := manager.CancelEvent(resources, CancelEventInput{
		Selection:       Selection{IntegrationID: "calendar-a"},
		ExternalEventID: created.ExternalEventID,
	})
	if err != nil {
		t.Fatalf("CancelEvent returned error: %v", err)
	}
	if cancelled.ExternalEventID != created.ExternalEventID {
		t.Fatalf("expected stable event identity on cancel, got create=%s cancel=%s", created.ExternalEventID, cancelled.ExternalEventID)
	}
	if cancelled.LifecycleState != EventLifecycleStateCancelled {
		t.Fatalf("expected cancelled lifecycle state, got %+v", cancelled)
	}
}
