package calendar

import (
	"errors"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

var (
	ErrCalendarUnavailable           = errors.New("calendar integration is unavailable")
	ErrCalendarIntegrationNotFound   = errors.New("calendar integration not found")
	ErrCalendarSelectionInvalid      = errors.New("calendar integration selection is invalid")
	ErrCalendarOperationNotFound     = errors.New("calendar operation not found")
	ErrCalendarAccountNotFound       = errors.New("calendar account projection not found")
	ErrCalendarEventNotFound         = errors.New("calendar event not found")
	ErrCalendarRecurringUnsupported  = errors.New("recurring-event mutation is out of scope for phase 29")
	ErrCalendarAllDayUnsupported     = errors.New("all-day-event mutation is out of scope for phase 29")
	ErrCalendarAttendeesUnsupported  = errors.New("attendee mutation semantics are out of scope for phase 29")
	ErrCalendarAlternateCalendarDeny = errors.New("alternate-calendar mutation is out of scope for phase 29")
	ErrCalendarInvalidTimeRange      = errors.New("invalid calendar time range")
	ErrCalendarAttendeeRequestEmpty  = errors.New("attendee update requires at least one add or remove")
	// ErrCalendarRecurrenceScopeRequired is returned when a recurring event is mutated without
	// stating which part of the series the mutation targets (Roadmap 62, FR-002).
	ErrCalendarRecurrenceScopeRequired = errors.New("recurrence scope is required for recurring-event mutation")
	// ErrCalendarRecurrenceScopeInvalid is returned when a stated recurrence scope is not one of
	// this_occurrence / this_and_following / entire_series.
	ErrCalendarRecurrenceScopeInvalid = errors.New("recurrence scope is invalid")
)

type Backend interface {
	ProjectAccount(resource integrations.Resource) (AccountProjection, error)
	ListEvents(resource integrations.Resource, account AccountProjection, input ListEventsInput) ([]Event, error)
	GetEvent(resource integrations.Resource, account AccountProjection, eventID string) (Event, error)
	BusyFree(resource integrations.Resource, account AccountProjection, input BusyFreeInput) (AvailabilityQuery, error)
	CreateEvent(resource integrations.Resource, account AccountProjection, input CreateEventInput) (Event, error)
	UpdateEvent(resource integrations.Resource, account AccountProjection, input UpdateEventInput) (Event, error)
	CancelEvent(resource integrations.Resource, account AccountProjection, input CancelEventInput) (Event, error)
	// UpdateAttendees performs an attendee-only mutation (add/remove + invitation notification)
	// and returns the event with projected attendee details (Roadmap 61). The event-field
	// mutation is unchanged; only attendees and the notification side effect are affected.
	UpdateAttendees(resource integrations.Resource, account AccountProjection, input UpdateAttendeesInput) (Event, error)
	RestoreIntegrationState(integrationID string, events []Event)
}

// resolveAttendeeRequests normalizes attendee inputs: it prefers explicit AttendeeRequests and
// otherwise synthesizes them from the legacy email list, defaulting role to required. This keeps
// the legacy []string attendee field backward compatible (FR-008).
func resolveAttendeeRequests(requests []AttendeeRequest, emails []string) []AttendeeRequest {
	if len(requests) > 0 {
		out := make([]AttendeeRequest, 0, len(requests))
		for _, r := range requests {
			if strings.TrimSpace(r.Email) == "" {
				continue
			}
			if r.Role == "" {
				r.Role = AttendeeRoleRequired
			}
			out = append(out, r)
		}
		return out
	}
	out := make([]AttendeeRequest, 0, len(emails))
	for _, email := range emails {
		if strings.TrimSpace(email) == "" {
			continue
		}
		out = append(out, AttendeeRequest{Email: strings.TrimSpace(email), Role: AttendeeRoleRequired})
	}
	return out
}

// attendeeEmails projects attendee details to the legacy email list for backward compatibility.
func attendeeEmails(details []Attendee) []string {
	if len(details) == 0 {
		return nil
	}
	emails := make([]string, 0, len(details))
	for _, a := range details {
		emails = append(emails, a.Email)
	}
	return emails
}

// buildAttendeeOutcome derives the recorded attendee outcome from a backend result. notify is the
// requested notification behavior; the per-attendee invitation status carries the provider result.
func buildAttendeeOutcome(notify bool, details []Attendee) *AttendeeOutcome {
	if len(details) == 0 && !notify {
		return nil
	}
	out := &AttendeeOutcome{NotificationRequested: notify, Attendees: details}
	behavior := NotificationBehaviorSilent
	if notify {
		behavior = NotificationBehaviorNotify
	}
	for _, a := range details {
		if a.InvitationStatus == InvitationStatusUnsupported {
			out.Unsupported = true
			out.UnsupportedReason = "provider does not support the requested attendee notification behavior"
			behavior = NotificationBehaviorUnsupported
		}
	}
	out.NotificationBehavior = behavior
	return out
}

func normalizeTimezone(requested, fallback string) string {
	if trimmed := strings.TrimSpace(requested); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(fallback); trimmed != "" {
		return trimmed
	}
	return "UTC"
}
