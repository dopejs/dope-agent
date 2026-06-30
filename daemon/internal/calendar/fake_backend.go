package calendar

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

type fakeState struct {
	account AccountProjection
	events  map[string]Event
}

type FakeBackend struct {
	mu     sync.RWMutex
	states map[string]*fakeState
}

func NewFakeBackend() *FakeBackend {
	return &FakeBackend{states: make(map[string]*fakeState)}
}

func (b *FakeBackend) RunLiveValidationOutcome(outcome livevalidation.FakeOutcome) livevalidation.FakeOutcomeResult {
	return livevalidation.FakeOutcomeResultFor(outcome, livevalidation.SafetyClassNonIdempotentMutation)
}

func (b *FakeBackend) ProjectAccount(resource integrations.Resource) (AccountProjection, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	now := time.Now().UTC()
	state.account.ReadinessStatus = string(resource.ReadinessStatus)
	state.account.CanonicalDefault = resource.CanonicalDefault
	state.account.AccountKey = resource.AccountBinding.AccountKey
	state.account.AccountLabel = resource.AccountBinding.AccountLabel
	state.account.UpdatedAt = now
	state.account.LastSyncedAt = now
	return state.account, nil
}

func (b *FakeBackend) ListEvents(resource integrations.Resource, account AccountProjection, input ListEventsInput) ([]Event, error) {
	b.mu.RLock()
	state, ok := b.states[resource.IntegrationID]
	b.mu.RUnlock()
	if !ok {
		b.mu.Lock()
		state = b.ensureStateLocked(resource)
		b.mu.Unlock()
	}
	items := make([]Event, 0, len(state.events))
	for _, item := range state.events {
		if input.StartsAt != nil && item.EndsAt.Before(*input.StartsAt) {
			continue
		}
		if input.EndsAt != nil && item.StartsAt.After(*input.EndsAt) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartsAt.Equal(items[j].StartsAt) {
			return items[i].ExternalEventID < items[j].ExternalEventID
		}
		return items[i].StartsAt.Before(items[j].StartsAt)
	})
	return cloneEvents(items), nil
}

func (b *FakeBackend) GetEvent(resource integrations.Resource, account AccountProjection, eventID string) (Event, error) {
	b.mu.RLock()
	state, ok := b.states[resource.IntegrationID]
	b.mu.RUnlock()
	if !ok {
		b.mu.Lock()
		state = b.ensureStateLocked(resource)
		b.mu.Unlock()
	}
	item, ok := state.events[strings.TrimSpace(eventID)]
	if !ok {
		return Event{}, ErrCalendarEventNotFound
	}
	return item, nil
}

func (b *FakeBackend) BusyFree(resource integrations.Resource, account AccountProjection, input BusyFreeInput) (AvailabilityQuery, error) {
	b.mu.RLock()
	state, ok := b.states[resource.IntegrationID]
	b.mu.RUnlock()
	if !ok {
		b.mu.Lock()
		state = b.ensureStateLocked(resource)
		b.mu.Unlock()
	}
	items := make([]BusyInterval, 0)
	for _, item := range state.events {
		if item.LifecycleState == EventLifecycleStateCancelled {
			continue
		}
		if item.EndsAt.Before(input.WindowStart) || item.StartsAt.After(input.WindowEnd) {
			continue
		}
		items = append(items, BusyInterval{StartsAt: item.StartsAt, EndsAt: item.EndsAt})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartsAt.Before(items[j].StartsAt) })
	return AvailabilityQuery{
		IntegrationID:     account.IntegrationID,
		CalendarAccountID: account.CalendarAccountID,
		WindowStart:       input.WindowStart.UTC(),
		WindowEnd:         input.WindowEnd.UTC(),
		Timezone:          normalizeTimezone(input.Timezone, account.PrimaryTimezone),
		BusyIntervals:     items,
		ConflictCount:     len(items),
		ResultSummary:     fmt.Sprintf("%d busy interval(s)", len(items)),
		CreatedAt:         time.Now().UTC(),
	}, nil
}

func (b *FakeBackend) CreateEvent(resource integrations.Resource, account AccountProjection, input CreateEventInput) (Event, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	now := time.Now().UTC()
	eventID := fmt.Sprintf("evt_%s_%d", strings.ReplaceAll(resource.IntegrationID, "-", "_"), now.UnixNano())
	item := Event{
		ExternalEventID:         eventID,
		IntegrationID:           account.IntegrationID,
		CalendarAccountID:       account.CalendarAccountID,
		CalendarRef:             account.PrimaryCalendarRef,
		Title:                   strings.TrimSpace(input.Title),
		Description:             strings.TrimSpace(input.Description),
		Location:                strings.TrimSpace(input.Location),
		StartsAt:                input.StartsAt.UTC(),
		EndsAt:                  input.EndsAt.UTC(),
		Timezone:                normalizeTimezone(input.Timezone, account.PrimaryTimezone),
		AllDay:                  false,
		Recurring:               false,
		MutationEligibleInPhase: true,
		LifecycleState:          EventLifecycleStateActive,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	item.AttendeeDetails = fakeInvite(resolveAttendeeRequests(input.AttendeeRequests, input.Attendees), input.NotifyAttendees)
	item.Attendees = attendeeEmails(item.AttendeeDetails)
	state.events[item.ExternalEventID] = item
	return item, nil
}

func (b *FakeBackend) UpdateEvent(resource integrations.Resource, account AccountProjection, input UpdateEventInput) (Event, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	item, ok := state.events[strings.TrimSpace(input.ExternalEventID)]
	if !ok {
		return Event{}, ErrCalendarEventNotFound
	}
	item.Title = strings.TrimSpace(input.Title)
	item.Description = strings.TrimSpace(input.Description)
	item.Location = strings.TrimSpace(input.Location)
	item.StartsAt = input.StartsAt.UTC()
	item.EndsAt = input.EndsAt.UTC()
	item.Timezone = normalizeTimezone(input.Timezone, account.PrimaryTimezone)
	item.UpdatedAt = time.Now().UTC()
	if requests := resolveAttendeeRequests(input.AttendeeRequests, input.Attendees); len(requests) > 0 {
		item.AttendeeDetails = fakeInvite(requests, input.NotifyAttendees)
		item.Attendees = attendeeEmails(item.AttendeeDetails)
	}
	state.events[item.ExternalEventID] = item
	return item, nil
}

// UpdateAttendees adds/removes attendees on an existing event and simulates invitation results.
func (b *FakeBackend) UpdateAttendees(resource integrations.Resource, account AccountProjection, input UpdateAttendeesInput) (Event, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	item, ok := state.events[strings.TrimSpace(input.ExternalEventID)]
	if !ok {
		return Event{}, ErrCalendarEventNotFound
	}
	// Index existing attendees by email so add/remove is idempotent and preserves RSVP state.
	byEmail := make(map[string]Attendee, len(item.AttendeeDetails))
	for _, a := range item.AttendeeDetails {
		byEmail[strings.ToLower(a.Email)] = a
	}
	for _, email := range input.RemoveAttendees {
		delete(byEmail, strings.ToLower(strings.TrimSpace(email)))
	}
	for _, added := range fakeInvite(resolveAttendeeRequests(input.AddAttendees, nil), input.Notify) {
		byEmail[strings.ToLower(added.Email)] = added
	}
	details := make([]Attendee, 0, len(byEmail))
	for _, a := range byEmail {
		details = append(details, a)
	}
	sort.Slice(details, func(i, j int) bool { return details[i].Email < details[j].Email })
	item.AttendeeDetails = details
	item.Attendees = attendeeEmails(details)
	item.UpdatedAt = time.Now().UTC()
	state.events[item.ExternalEventID] = item
	return item, nil
}

// fakeInvite simulates per-attendee invitation + RSVP results for the fake backend: an
// invitation is "sent" only when notification was requested; RSVP starts at needs_action.
func fakeInvite(requests []AttendeeRequest, notify bool) []Attendee {
	if len(requests) == 0 {
		return nil
	}
	details := make([]Attendee, 0, len(requests))
	for _, r := range requests {
		invitation := InvitationStatusNotRequested
		if notify {
			invitation = InvitationStatusSent
		}
		details = append(details, Attendee{
			Email:            strings.TrimSpace(r.Email),
			DisplayName:      r.DisplayName,
			Role:             r.Role,
			RSVP:             RSVPStatusNeedsAction,
			InvitationStatus: invitation,
		})
	}
	return details
}

func (b *FakeBackend) CancelEvent(resource integrations.Resource, account AccountProjection, input CancelEventInput) (Event, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureStateLocked(resource)
	item, ok := state.events[strings.TrimSpace(input.ExternalEventID)]
	if !ok {
		return Event{}, ErrCalendarEventNotFound
	}
	now := time.Now().UTC()
	item.LifecycleState = EventLifecycleStateCancelled
	item.UpdatedAt = now
	item.CancelledAt = &now
	state.events[item.ExternalEventID] = item
	return item, nil
}

func (b *FakeBackend) RestoreIntegrationState(integrationID string, events []Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	trimmed := strings.TrimSpace(integrationID)
	state, ok := b.states[trimmed]
	if !ok {
		state = &fakeState{
			account: AccountProjection{
				CalendarAccountID: "acct_" + trimmed,
				IntegrationID:     trimmed,
				DomainKind:        "calendar",
			},
			events: make(map[string]Event, len(events)),
		}
		b.states[trimmed] = state
	}
	state.events = make(map[string]Event, len(events))
	for _, item := range events {
		state.events[item.ExternalEventID] = item
	}
}

func (b *FakeBackend) ensureStateLocked(resource integrations.Resource) *fakeState {
	if state, ok := b.states[resource.IntegrationID]; ok {
		return state
	}
	now := time.Now().UTC()
	state := &fakeState{
		account: AccountProjection{
			CalendarAccountID:       "acct_" + resource.IntegrationID,
			IntegrationID:           resource.IntegrationID,
			DomainKind:              resource.DomainKind,
			EnvironmentScope:        resource.EnvironmentScope,
			AccountKey:              resource.AccountBinding.AccountKey,
			AccountLabel:            resource.AccountBinding.AccountLabel,
			ReadinessStatus:         string(resource.ReadinessStatus),
			CanonicalDefault:        resource.CanonicalDefault,
			SelectionMode:           "explicit",
			PrimaryCalendarRef:      "primary",
			PrimaryCalendarLabel:    "Primary Calendar",
			PrimaryTimezone:         "America/Los_Angeles",
			SupportsEventInspection: true,
			SupportsBusyFree:        true,
			SupportsTimedMutation:   true,
			LastSyncedAt:            now,
			UpdatedAt:               now,
		},
		events: map[string]Event{
			"fake_event_seed": {
				ExternalEventID:         "fake_event_seed",
				IntegrationID:           resource.IntegrationID,
				CalendarAccountID:       "acct_" + resource.IntegrationID,
				CalendarRef:             "primary",
				Title:                   "Seed Calendar Event",
				StartsAt:                time.Date(2026, 4, 23, 16, 0, 0, 0, time.UTC),
				EndsAt:                  time.Date(2026, 4, 23, 16, 30, 0, 0, time.UTC),
				Timezone:                "America/Los_Angeles",
				MutationEligibleInPhase: true,
				LifecycleState:          EventLifecycleStateActive,
				CreatedAt:               now,
				UpdatedAt:               now,
			},
		},
	}
	b.states[resource.IntegrationID] = state
	return state
}

func cloneEvents(items []Event) []Event {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]Event, len(items))
	copy(cloned, items)
	return cloned
}
