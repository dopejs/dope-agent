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
)

type Backend interface {
	ProjectAccount(resource integrations.Resource) (AccountProjection, error)
	ListEvents(resource integrations.Resource, account AccountProjection, input ListEventsInput) ([]Event, error)
	GetEvent(resource integrations.Resource, account AccountProjection, eventID string) (Event, error)
	BusyFree(resource integrations.Resource, account AccountProjection, input BusyFreeInput) (AvailabilityQuery, error)
	CreateEvent(resource integrations.Resource, account AccountProjection, input CreateEventInput) (Event, error)
	UpdateEvent(resource integrations.Resource, account AccountProjection, input UpdateEventInput) (Event, error)
	CancelEvent(resource integrations.Resource, account AccountProjection, input CancelEventInput) (Event, error)
	RestoreIntegrationState(integrationID string, events []Event)
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
