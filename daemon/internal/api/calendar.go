package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleCalendarAccounts(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "calendar dependencies are not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	items, err := manager.ListAccounts(integrationsManager.List(), calendar.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))})
	if err != nil {
		writeCalendarError(w, r, err)
		return
	}
	items = filterCalendarAccounts(items, r)
	if err := recordCalendarAccounts(r.Context(), eventBus, sqliteStore, items); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, CalendarAccountListResponse{Items: items})
}

func handleCalendarAccountRoutes(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "calendar dependencies are not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	integrationID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/calendar/accounts/"))
	if integrationID == "" {
		http.NotFound(w, r)
		return
	}
	items, err := manager.ListAccounts(integrationsManager.List(), calendar.Selection{IntegrationID: integrationID})
	if err != nil {
		writeCalendarError(w, r, err)
		return
	}
	if len(items) == 0 {
		http.NotFound(w, r)
		return
	}
	if err := recordCalendarAccounts(r.Context(), eventBus, sqliteStore, items[:1]); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items[0])
}

func handleCalendarEvents(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "calendar dependencies are not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		handleCalendarEventList(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r)
	case http.MethodPost:
		handleCalendarEventCreate(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleCalendarEventRoutes(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "calendar dependencies are not configured")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/calendar/events/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		handleCalendarEventGet(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "update" && r.Method == http.MethodPost {
		handleCalendarEventUpdate(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
		handleCalendarEventCancel(cfg, manager, integrationsManager, eventBus, billingManager, sqliteStore, w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func handleCalendarAvailabilityQueries(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil || integrationsManager == nil {
		writeError(w, http.StatusInternalServerError, "calendar dependencies are not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request CreateCalendarAvailabilityQueryRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	windowStart, err := parseCalendarTimestamp(request.WindowStart)
	if err != nil {
		writeError(w, http.StatusBadRequest, "windowStart must be RFC3339")
		return
	}
	windowEnd, err := parseCalendarTimestamp(request.WindowEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "windowEnd must be RFC3339")
		return
	}

	operationID := calendar.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "calendar", operationID, "POST /v1/calendar/availability/queries", w, r)
	if !ok {
		return
	}
	account, query, operation, artifacts, err := manager.BusyFree(integrationsManager.List(), calendar.BusyFreeInput{
		Selection:   calendar.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Timezone:    strings.TrimSpace(request.Timezone),
		Source:      calendarSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordCalendarActivity(r.Context(), eventBus, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
		if commitErr := commitBillingReservation(r.Context(), billingManager, reservation, "billing.integration_operation_committed", "calendar operation recorded after backend attempt"); commitErr != nil {
			writeError(w, http.StatusInternalServerError, commitErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "calendar operation failed before backend attempt")
		}
		writeCalendarError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, CalendarAvailabilityQueryResponse{
		Account:   account,
		Query:     query,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleCalendarAvailabilityQueryRoutes(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "calendar manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	queryID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/calendar/availability/queries/"))
	if queryID == "" {
		http.NotFound(w, r)
		return
	}
	operation, ok := manager.GetOperation(queryID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var query calendar.AvailabilityQuery
	artifacts := manager.ListArtifacts(operation.OperationID)
	found := false
	for _, item := range artifacts {
		if item.Kind == calendar.ArtifactKindAvailabilityQuery && item.AvailabilityQuery != nil {
			query = *item.AvailabilityQuery
			found = true
			break
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	account, ok := calendarAccountForOperation(manager, operation)
	if !ok {
		writeError(w, http.StatusInternalServerError, "calendar account projection is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, CalendarAvailabilityQueryResponse{
		Account:   account,
		Query:     query,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleCalendarOperations(cfg config.Config, manager *calendar.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "calendar manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	filter := calendar.OperationFilter{
		IntegrationID:   strings.TrimSpace(r.URL.Query().Get("integrationId")),
		RunID:           strings.TrimSpace(r.URL.Query().Get("runId")),
		WorkflowID:      strings.TrimSpace(r.URL.Query().Get("workflowId")),
		ScheduleID:      strings.TrimSpace(r.URL.Query().Get("scheduleId")),
		DeliveryID:      strings.TrimSpace(r.URL.Query().Get("deliveryId")),
		OperationClass:  calendar.OperationClass(strings.TrimSpace(r.URL.Query().Get("operationClass"))),
		Status:          calendar.OperationStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		ExternalEventID: strings.TrimSpace(r.URL.Query().Get("externalEventId")),
	}
	writeJSON(w, http.StatusOK, CalendarOperationListResponse{Items: manager.ListOperations(filter)})
}

func handleCalendarOperationRoutes(cfg config.Config, manager *calendar.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "calendar manager is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	operationID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/calendar/operations/"))
	if operationID == "" {
		http.NotFound(w, r)
		return
	}
	operation, ok := manager.GetOperation(operationID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, CalendarOperationResponse{
		Operation: operation,
		Artifacts: manager.ListArtifacts(operationID),
	})
}

func handleCalendarEventList(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	startsAt, err := parseOptionalCalendarTimestamp(r.URL.Query().Get("startsAt"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "startsAt must be RFC3339")
		return
	}
	endsAt, err := parseOptionalCalendarTimestamp(r.URL.Query().Get("endsAt"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "endsAt must be RFC3339")
		return
	}
	operationID := calendar.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "calendar", operationID, "GET /v1/calendar/events", w, r)
	if !ok {
		return
	}
	account, items, operation, artifacts, err := manager.ListEvents(integrationsManager.List(), calendar.ListEventsInput{
		Selection: calendar.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))},
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Source:    calendar.SourceLinkage{OperationID: operationID},
	})
	if operation.OperationID != "" {
		if recordErr := recordCalendarActivity(r.Context(), eventBus, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
		if commitErr := commitBillingReservation(r.Context(), billingManager, reservation, "billing.integration_operation_committed", "calendar operation recorded after backend attempt"); commitErr != nil {
			writeError(w, http.StatusInternalServerError, commitErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "calendar operation failed before backend attempt")
		}
		writeCalendarError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, CalendarEventListResponse{
		Account:   account,
		Items:     items,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleCalendarEventGet(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, externalEventID string) {
	operationID := calendar.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "calendar", operationID, "GET /v1/calendar/events/{externalEventId}", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.GetEvent(integrationsManager.List(), calendar.GetEventInput{
		Selection:       calendar.Selection{IntegrationID: strings.TrimSpace(r.URL.Query().Get("integrationId"))},
		ExternalEventID: strings.TrimSpace(externalEventID),
		Source:          calendar.SourceLinkage{OperationID: operationID},
	})
	if operation.OperationID != "" {
		if recordErr := recordCalendarActivity(r.Context(), eventBus, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
		if commitErr := commitBillingReservation(r.Context(), billingManager, reservation, "billing.integration_operation_committed", "calendar operation recorded after backend attempt"); commitErr != nil {
			writeError(w, http.StatusInternalServerError, commitErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "calendar operation failed before backend attempt")
		}
		writeCalendarError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, CalendarEventResponse{
		Account:   account,
		Event:     item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleCalendarEventCreate(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	var request CreateCalendarEventRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := rejectUnsupportedCalendarMutation(request.CalendarRef); err != nil {
		writeCalendarError(w, r, err)
		return
	}
	startsAt, err := parseCalendarTimestamp(request.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "startsAt must be RFC3339")
		return
	}
	endsAt, err := parseCalendarTimestamp(request.EndsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "endsAt must be RFC3339")
		return
	}

	operationID := calendar.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "calendar", operationID, "POST /v1/calendar/events", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.CreateEvent(integrationsManager.List(), calendar.CreateEventInput{
		Selection:        calendar.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		Title:            strings.TrimSpace(request.Title),
		Description:      strings.TrimSpace(request.Description),
		Location:         strings.TrimSpace(request.Location),
		StartsAt:         startsAt,
		EndsAt:           endsAt,
		Timezone:         strings.TrimSpace(request.Timezone),
		AllDay:           request.AllDay,
		StartDate:        strings.TrimSpace(request.StartDate),
		EndDate:          strings.TrimSpace(request.EndDate),
		Recurring:        request.Recurring,
		RecurrenceRule:   strings.TrimSpace(request.RecurrenceRule),
		AttendeeRequests: calendarAttendeeRequests(request.Attendees),
		NotifyAttendees:  request.NotifyAttendees,
		Source:           calendarSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordCalendarActivity(r.Context(), eventBus, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
		if commitErr := commitBillingReservation(r.Context(), billingManager, reservation, "billing.integration_operation_committed", "calendar operation recorded after backend attempt"); commitErr != nil {
			writeError(w, http.StatusInternalServerError, commitErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "calendar operation failed before backend attempt")
		}
		writeCalendarError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, CalendarEventResponse{
		Account:   account,
		Event:     item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleCalendarEventUpdate(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, externalEventID string) {
	var request UpdateCalendarEventRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := rejectUnsupportedCalendarMutation(request.CalendarRef); err != nil {
		writeCalendarError(w, r, err)
		return
	}
	startsAt, err := parseCalendarTimestamp(request.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "startsAt must be RFC3339")
		return
	}
	endsAt, err := parseCalendarTimestamp(request.EndsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "endsAt must be RFC3339")
		return
	}

	operationID := calendar.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "calendar", operationID, "POST /v1/calendar/events/{externalEventId}/update", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.UpdateEvent(integrationsManager.List(), calendar.UpdateEventInput{
		Selection:        calendar.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		ExternalEventID:  strings.TrimSpace(externalEventID),
		Title:            strings.TrimSpace(request.Title),
		Description:      strings.TrimSpace(request.Description),
		Location:         strings.TrimSpace(request.Location),
		StartsAt:         startsAt,
		EndsAt:           endsAt,
		Timezone:         strings.TrimSpace(request.Timezone),
		AllDay:           request.AllDay,
		StartDate:        strings.TrimSpace(request.StartDate),
		EndDate:          strings.TrimSpace(request.EndDate),
		Recurring:        request.Recurring,
		RecurrenceRule:   strings.TrimSpace(request.RecurrenceRule),
		RecurrenceScope:  calendarRecurrenceScope(request.RecurrenceScope),
		AttendeeRequests: calendarAttendeeRequests(request.Attendees),
		NotifyAttendees:  request.NotifyAttendees,
		Source:           calendarSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordCalendarActivity(r.Context(), eventBus, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
		if commitErr := commitBillingReservation(r.Context(), billingManager, reservation, "billing.integration_operation_committed", "calendar operation recorded after backend attempt"); commitErr != nil {
			writeError(w, http.StatusInternalServerError, commitErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "calendar operation failed before backend attempt")
		}
		writeCalendarError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, CalendarEventResponse{
		Account:   account,
		Event:     item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func handleCalendarEventCancel(cfg config.Config, manager *calendar.Manager, integrationsManager *integrations.Manager, eventBus *events.Bus, billingManager *billing.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, externalEventID string) {
	var request CancelCalendarEventRequest
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.CalendarRef) != "" {
		writeCalendarError(w, r, calendar.ErrCalendarAlternateCalendarDeny)
		return
	}
	operationID := calendar.NewOperationID()
	reservation, ok := beginIntegrationOperationQuota(r.Context(), cfg, billingManager, "calendar", operationID, "POST /v1/calendar/events/{externalEventId}/cancel", w, r)
	if !ok {
		return
	}
	account, item, operation, artifacts, err := manager.CancelEvent(integrationsManager.List(), calendar.CancelEventInput{
		Selection:       calendar.Selection{IntegrationID: strings.TrimSpace(request.IntegrationID)},
		ExternalEventID: strings.TrimSpace(externalEventID),
		Reason:          strings.TrimSpace(request.Reason),
		RecurrenceScope: calendarRecurrenceScope(request.RecurrenceScope),
		Source:          calendarSourceLinkageWithOperation(request.Source, operationID),
	})
	if operation.OperationID != "" {
		if recordErr := recordCalendarActivity(r.Context(), eventBus, sqliteStore, account, operation, artifacts); recordErr != nil {
			writeError(w, http.StatusInternalServerError, recordErr.Error())
			return
		}
		if commitErr := commitBillingReservation(r.Context(), billingManager, reservation, "billing.integration_operation_committed", "calendar operation recorded after backend attempt"); commitErr != nil {
			writeError(w, http.StatusInternalServerError, commitErr.Error())
			return
		}
	}
	if err != nil {
		if operation.OperationID == "" {
			releaseBillingReservation(r.Context(), billingManager, reservation, "calendar operation failed before backend attempt")
		}
		writeCalendarError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, CalendarEventResponse{
		Account:   account,
		Event:     item,
		Operation: operation,
		Artifacts: artifacts,
	})
}

func recordCalendarAccounts(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, items []calendar.AccountProjection) error {
	for _, item := range items {
		if err := persistCalendarAccount(ctx, sqliteStore, item); err != nil {
			return err
		}
		if err := publishCalendarAccountProjected(ctx, eventBus, sqliteStore, item); err != nil {
			return err
		}
	}
	return nil
}

func recordCalendarActivity(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, account calendar.AccountProjection, operation calendar.Operation, artifacts []calendar.Artifact) error {
	if account.IntegrationID != "" {
		if err := persistCalendarAccount(ctx, sqliteStore, account); err != nil {
			return err
		}
		if err := publishCalendarAccountProjected(ctx, eventBus, sqliteStore, account); err != nil {
			return err
		}
	}
	if operation.OperationID == "" {
		return nil
	}
	if err := persistCalendarOperation(ctx, sqliteStore, operation); err != nil {
		return err
	}
	if err := publishCalendarOperationRequested(ctx, eventBus, sqliteStore, operation); err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := persistCalendarArtifact(ctx, sqliteStore, artifact); err != nil {
			return err
		}
		if err := publishCalendarArtifactRecorded(ctx, eventBus, sqliteStore, artifact, operation); err != nil {
			return err
		}
	}
	switch operation.Status {
	case calendar.OperationStatusCompleted:
		return publishCalendarOperationCompleted(ctx, eventBus, sqliteStore, operation)
	case calendar.OperationStatusFailed, calendar.OperationStatusBlocked, calendar.OperationStatusCancelled:
		return publishCalendarOperationFailed(ctx, eventBus, sqliteStore, operation)
	default:
		return nil
	}
}

func persistCalendarAccount(ctx context.Context, sqliteStore *store.SQLiteStore, item calendar.AccountProjection) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertCalendarAccount(ctx, item)
}

func persistCalendarOperation(ctx context.Context, sqliteStore *store.SQLiteStore, item calendar.Operation) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertCalendarOperation(ctx, item)
}

func persistCalendarArtifact(ctx context.Context, sqliteStore *store.SQLiteStore, item calendar.Artifact) error {
	if sqliteStore == nil {
		return nil
	}
	return sqliteStore.UpsertCalendarArtifact(ctx, item)
}

func publishCalendarAccountProjected(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, account calendar.AccountProjection) error {
	if eventBus == nil {
		return nil
	}
	_, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "calendar",
		Name:     "calendar.account_projected",
		Resource: events.Resource{Kind: "calendar_account", ID: account.CalendarAccountID},
		Payload: map[string]any{
			"integrationId":      account.IntegrationID,
			"accountKey":         account.AccountKey,
			"primaryCalendarRef": account.PrimaryCalendarRef,
			"primaryTimezone":    account.PrimaryTimezone,
			"readinessStatus":    account.ReadinessStatus,
			"canonicalDefault":   account.CanonicalDefault,
		},
	})
	return err
}

func publishCalendarOperationRequested(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, operation calendar.Operation) error {
	return publishCalendarOperationEvent(ctx, eventBus, sqliteStore, "calendar.operation_requested", operation)
}

func publishCalendarOperationCompleted(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, operation calendar.Operation) error {
	return publishCalendarOperationEvent(ctx, eventBus, sqliteStore, "calendar.operation_completed", operation)
}

func publishCalendarOperationFailed(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, operation calendar.Operation) error {
	return publishCalendarOperationEvent(ctx, eventBus, sqliteStore, "calendar.operation_failed", operation)
}

func publishCalendarOperationEvent(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, name string, operation calendar.Operation) error {
	if eventBus == nil {
		return nil
	}
	_, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "calendar",
		Name:     name,
		Scope: events.Scope{
			RunID:      operation.RunID,
			WorkflowID: operation.WorkflowID,
			ScheduleID: operation.ScheduleID,
		},
		Resource: events.Resource{Kind: "calendar_operation", ID: operation.OperationID},
		Payload: map[string]any{
			"operationId":     operation.OperationID,
			"operationClass":  operation.OperationClass,
			"integrationId":   operation.IntegrationID,
			"runId":           operation.RunID,
			"workflowId":      operation.WorkflowID,
			"scheduleId":      operation.ScheduleID,
			"deliveryId":      operation.DeliveryID,
			"status":          operation.Status,
			"timezoneUsed":    operation.TimezoneUsed,
			"externalEventId": operation.ExternalEventID,
			"failureClass":    operation.FailureClass,
		},
	})
	return err
}

func publishCalendarArtifactRecorded(ctx context.Context, eventBus *events.Bus, sqliteStore *store.SQLiteStore, artifact calendar.Artifact, operation calendar.Operation) error {
	if eventBus == nil {
		return nil
	}
	_, err := publishEvent(ctx, eventBus, sqliteStore, events.Event{
		Category: "calendar",
		Name:     "calendar.artifact_recorded",
		Scope: events.Scope{
			RunID:      operation.RunID,
			WorkflowID: operation.WorkflowID,
			ScheduleID: operation.ScheduleID,
		},
		Resource: events.Resource{Kind: "calendar_artifact", ID: artifact.ArtifactID},
		Payload: map[string]any{
			"artifactId":      artifact.ArtifactID,
			"operationId":     artifact.OperationID,
			"externalEventId": artifact.ExternalEventID,
			"calendarRef":     artifact.CalendarRef,
			"lifecycleState":  artifact.LifecycleState,
		},
	})
	return err
}

func calendarSourceLinkage(source *CalendarSourceLinkageRequest) calendar.SourceLinkage {
	if source == nil {
		return calendar.SourceLinkage{}
	}
	return calendar.SourceLinkage{
		RunID:             strings.TrimSpace(source.RunID),
		StepID:            strings.TrimSpace(source.StepID),
		ToolCallID:        strings.TrimSpace(source.ToolCallID),
		WorkflowID:        strings.TrimSpace(source.WorkflowID),
		WorkflowStepID:    strings.TrimSpace(source.WorkflowStepID),
		ScheduleID:        strings.TrimSpace(source.ScheduleID),
		ScheduleAttemptID: strings.TrimSpace(source.ScheduleAttemptID),
		DeliveryID:        strings.TrimSpace(source.DeliveryID),
	}
}

func calendarSourceLinkageWithOperation(source *CalendarSourceLinkageRequest, operationID string) calendar.SourceLinkage {
	linkage := calendarSourceLinkage(source)
	linkage.OperationID = strings.TrimSpace(operationID)
	return linkage
}

func filterCalendarAccounts(items []calendar.AccountProjection, r *http.Request) []calendar.AccountProjection {
	readinessStatus := strings.TrimSpace(r.URL.Query().Get("readinessStatus"))
	canonicalDefault := strings.TrimSpace(r.URL.Query().Get("canonicalDefault"))
	wantDefault := false
	filterDefault := false
	if canonicalDefault != "" {
		if parsed, err := strconv.ParseBool(canonicalDefault); err == nil {
			filterDefault = true
			wantDefault = parsed
		}
	}
	filtered := make([]calendar.AccountProjection, 0, len(items))
	for _, item := range items {
		if readinessStatus != "" && item.ReadinessStatus != readinessStatus {
			continue
		}
		if filterDefault && item.CanonicalDefault != wantDefault {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// rejectUnsupportedCalendarMutation rejects mutations still out of scope. Attendee writes are
// supported from Roadmap 61 (spec 046); all-day and recurrence from Roadmap 62 (spec 047). Only
// alternate-calendar targeting remains out of scope.
func rejectUnsupportedCalendarMutation(calendarRef string) error {
	if strings.TrimSpace(calendarRef) != "" {
		return calendar.ErrCalendarAlternateCalendarDeny
	}
	return nil
}

// calendarRecurrenceScope maps the API recurrence-scope string to the calendar model.
func calendarRecurrenceScope(raw string) calendar.RecurrenceScope {
	return calendar.RecurrenceScope(strings.TrimSpace(raw))
}

// calendarAttendeeRequests maps API attendee requests to the calendar attendee model.
func calendarAttendeeRequests(items []CalendarAttendeeRequest) []calendar.AttendeeRequest {
	if len(items) == 0 {
		return nil
	}
	out := make([]calendar.AttendeeRequest, 0, len(items))
	for _, a := range items {
		if strings.TrimSpace(a.Email) == "" {
			continue
		}
		role := calendar.AttendeeRoleRequired
		if strings.EqualFold(strings.TrimSpace(a.Role), string(calendar.AttendeeRoleOptional)) {
			role = calendar.AttendeeRoleOptional
		}
		out = append(out, calendar.AttendeeRequest{Email: strings.TrimSpace(a.Email), DisplayName: strings.TrimSpace(a.DisplayName), Role: role})
	}
	return out
}

func calendarAccountForOperation(manager *calendar.Manager, operation calendar.Operation) (calendar.AccountProjection, bool) {
	return manager.GetAccount(operation.IntegrationID)
}

func writeCalendarError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, calendar.ErrCalendarIntegrationNotFound), errors.Is(err, calendar.ErrCalendarEventNotFound), errors.Is(err, calendar.ErrCalendarOperationNotFound), errors.Is(err, calendar.ErrCalendarAccountNotFound):
		http.NotFound(w, r)
	case errors.Is(err, calendar.ErrCalendarUnavailable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, calendar.ErrCalendarSelectionInvalid), errors.Is(err, calendar.ErrCalendarRecurringUnsupported), errors.Is(err, calendar.ErrCalendarAllDayUnsupported), errors.Is(err, calendar.ErrCalendarAttendeesUnsupported), errors.Is(err, calendar.ErrCalendarAlternateCalendarDeny), errors.Is(err, calendar.ErrCalendarInvalidTimeRange), errors.Is(err, calendar.ErrCalendarRecurrenceScopeRequired), errors.Is(err, calendar.ErrCalendarRecurrenceScopeInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func parseCalendarTimestamp(raw string) (time.Time, error) {
	return time.Parse(time.RFC3339, strings.TrimSpace(raw))
}

func parseOptionalCalendarTimestamp(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := parseCalendarTimestamp(raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
