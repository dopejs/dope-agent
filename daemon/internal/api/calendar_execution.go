package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

type calendarExecutionResult struct {
	Account   calendar.AccountProjection
	Operation calendar.Operation
	Artifacts []calendar.Artifact
	Output    any
}

func buildCalendarAction(request *CalendarWorkflowActionRequest) (*calendar.Action, error) {
	if request == nil {
		return nil, nil
	}
	action := &calendar.Action{
		OperationClass:  request.OperationClass,
		IntegrationID:   strings.TrimSpace(request.IntegrationID),
		ExternalEventID: strings.TrimSpace(request.ExternalEventID),
		Title:           strings.TrimSpace(request.Title),
		Description:     strings.TrimSpace(request.Description),
		Location:        strings.TrimSpace(request.Location),
		Timezone:        strings.TrimSpace(request.Timezone),
		CalendarRef:     strings.TrimSpace(request.CalendarRef),
		AllDay:          request.AllDay,
		Recurring:       request.Recurring,
		Attendees:       make([]string, 0, len(request.Attendees)),
		Reason:          strings.TrimSpace(request.Reason),
	}
	for _, attendee := range request.Attendees {
		if email := strings.TrimSpace(attendee); email != "" {
			action.Attendees = append(action.Attendees, email)
		}
	}
	var err error
	if action.WindowStart, err = parseOptionalCalendarActionTime(request.WindowStart); err != nil {
		return nil, fmt.Errorf("parse windowStart: %w", err)
	}
	if action.WindowEnd, err = parseOptionalCalendarActionTime(request.WindowEnd); err != nil {
		return nil, fmt.Errorf("parse windowEnd: %w", err)
	}
	if action.StartsAt, err = parseOptionalCalendarActionTime(request.StartsAt); err != nil {
		return nil, fmt.Errorf("parse startsAt: %w", err)
	}
	if action.EndsAt, err = parseOptionalCalendarActionTime(request.EndsAt); err != nil {
		return nil, fmt.Errorf("parse endsAt: %w", err)
	}
	if strings.TrimSpace(string(action.OperationClass)) == "" {
		return nil, errors.New("calendarAction.operationClass is required")
	}
	return action, nil
}

func parseOptionalCalendarActionTime(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func decodeCalendarAction(value any) (calendar.Action, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return calendar.Action{}, err
	}
	var action calendar.Action
	if err := json.Unmarshal(payload, &action); err != nil {
		return calendar.Action{}, err
	}
	if action.WindowStart != nil {
		normalized := action.WindowStart.UTC()
		action.WindowStart = &normalized
	}
	if action.WindowEnd != nil {
		normalized := action.WindowEnd.UTC()
		action.WindowEnd = &normalized
	}
	if action.StartsAt != nil {
		normalized := action.StartsAt.UTC()
		action.StartsAt = &normalized
	}
	if action.EndsAt != nil {
		normalized := action.EndsAt.UTC()
		action.EndsAt = &normalized
	}
	return action, nil
}

func executeCalendarAction(manager *calendar.Manager, integrationsManager *integrations.Manager, action calendar.Action, source calendar.SourceLinkage) (calendarExecutionResult, error) {
	if manager == nil || integrationsManager == nil {
		return calendarExecutionResult{}, errors.New("calendar dependencies are not configured")
	}
	switch action.OperationClass {
	case calendar.OperationClassListEvents:
		account, items, operation, artifacts, err := manager.ListEvents(integrationsManager.List(), calendar.ListEventsInput{
			Selection: calendar.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			StartsAt:  action.WindowStart,
			EndsAt:    action.WindowEnd,
			Source:    source,
		})
		if err != nil {
			return calendarExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return calendarExecutionResult{
			Account:   account,
			Operation: operation,
			Artifacts: artifacts,
			Output: CalendarEventListResponse{
				Account:   account,
				Items:     items,
				Operation: operation,
				Artifacts: artifacts,
			},
		}, nil
	case calendar.OperationClassGetEvent:
		account, item, operation, artifacts, err := manager.GetEvent(integrationsManager.List(), calendar.GetEventInput{
			Selection:       calendar.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			ExternalEventID: strings.TrimSpace(action.ExternalEventID),
			Source:          source,
		})
		if err != nil {
			return calendarExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return calendarExecutionResult{
			Account:   account,
			Operation: operation,
			Artifacts: artifacts,
			Output: CalendarEventResponse{
				Account:   account,
				Event:     item,
				Operation: operation,
				Artifacts: artifacts,
			},
		}, nil
	case calendar.OperationClassBusyFree:
		if action.WindowStart == nil || action.WindowEnd == nil {
			return calendarExecutionResult{}, errors.New("calendarAction.windowStart and calendarAction.windowEnd are required for busy_free")
		}
		account, query, operation, artifacts, err := manager.BusyFree(integrationsManager.List(), calendar.BusyFreeInput{
			Selection:   calendar.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			WindowStart: action.WindowStart.UTC(),
			WindowEnd:   action.WindowEnd.UTC(),
			Timezone:    strings.TrimSpace(action.Timezone),
			Source:      source,
		})
		if err != nil {
			return calendarExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return calendarExecutionResult{
			Account:   account,
			Operation: operation,
			Artifacts: artifacts,
			Output: CalendarAvailabilityQueryResponse{
				Account:   account,
				Query:     query,
				Operation: operation,
				Artifacts: artifacts,
			},
		}, nil
	case calendar.OperationClassCreateEvent:
		if err := rejectUnsupportedCalendarMutation(action.CalendarRef); err != nil {
			return calendarExecutionResult{}, err
		}
		if action.StartsAt == nil || action.EndsAt == nil {
			return calendarExecutionResult{}, errors.New("calendarAction.startsAt and calendarAction.endsAt are required for create_event")
		}
		account, item, operation, artifacts, err := manager.CreateEvent(integrationsManager.List(), calendar.CreateEventInput{
			Selection:   calendar.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			Title:       strings.TrimSpace(action.Title),
			Description: strings.TrimSpace(action.Description),
			Location:    strings.TrimSpace(action.Location),
			StartsAt:    action.StartsAt.UTC(),
			EndsAt:      action.EndsAt.UTC(),
			Timezone:    strings.TrimSpace(action.Timezone),
			AllDay:      action.AllDay,
			Recurring:   action.Recurring,
			Attendees:   append([]string(nil), action.Attendees...),
			Source:      source,
		})
		if err != nil {
			return calendarExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return calendarExecutionResult{
			Account:   account,
			Operation: operation,
			Artifacts: artifacts,
			Output: CalendarEventResponse{
				Account:   account,
				Event:     item,
				Operation: operation,
				Artifacts: artifacts,
			},
		}, nil
	case calendar.OperationClassUpdateEvent:
		if err := rejectUnsupportedCalendarMutation(action.CalendarRef); err != nil {
			return calendarExecutionResult{}, err
		}
		if action.StartsAt == nil || action.EndsAt == nil {
			return calendarExecutionResult{}, errors.New("calendarAction.startsAt and calendarAction.endsAt are required for update_event")
		}
		account, item, operation, artifacts, err := manager.UpdateEvent(integrationsManager.List(), calendar.UpdateEventInput{
			Selection:       calendar.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			ExternalEventID: strings.TrimSpace(action.ExternalEventID),
			Title:           strings.TrimSpace(action.Title),
			Description:     strings.TrimSpace(action.Description),
			Location:        strings.TrimSpace(action.Location),
			StartsAt:        action.StartsAt.UTC(),
			EndsAt:          action.EndsAt.UTC(),
			Timezone:        strings.TrimSpace(action.Timezone),
			AllDay:          action.AllDay,
			Recurring:       action.Recurring,
			Attendees:       append([]string(nil), action.Attendees...),
			Source:          source,
		})
		if err != nil {
			return calendarExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return calendarExecutionResult{
			Account:   account,
			Operation: operation,
			Artifacts: artifacts,
			Output: CalendarEventResponse{
				Account:   account,
				Event:     item,
				Operation: operation,
				Artifacts: artifacts,
			},
		}, nil
	case calendar.OperationClassCancelEvent:
		if strings.TrimSpace(action.CalendarRef) != "" {
			return calendarExecutionResult{}, calendar.ErrCalendarAlternateCalendarDeny
		}
		account, item, operation, artifacts, err := manager.CancelEvent(integrationsManager.List(), calendar.CancelEventInput{
			Selection:       calendar.Selection{IntegrationID: strings.TrimSpace(action.IntegrationID)},
			ExternalEventID: strings.TrimSpace(action.ExternalEventID),
			Reason:          strings.TrimSpace(action.Reason),
			Source:          source,
		})
		if err != nil {
			return calendarExecutionResult{Account: account, Operation: operation, Artifacts: artifacts}, err
		}
		return calendarExecutionResult{
			Account:   account,
			Operation: operation,
			Artifacts: artifacts,
			Output: CalendarEventResponse{
				Account:   account,
				Event:     item,
				Operation: operation,
				Artifacts: artifacts,
			},
		}, nil
	default:
		return calendarExecutionResult{}, fmt.Errorf("unsupported calendar action %q", action.OperationClass)
	}
}
