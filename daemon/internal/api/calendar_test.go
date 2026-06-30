package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func seedHealthyCalendarIntegration(t *testing.T, manager *integrations.Manager, sqliteStore *store.SQLiteStore, integrationID string, canonicalDefault bool) integrations.Resource {
	t.Helper()

	resource, err := manager.Create(integrations.CreateInput{
		IntegrationID:    integrationID,
		DomainKind:       "calendar",
		DisplayName:      integrationID,
		EnvironmentScope: "test",
		CanonicalDefault: canonicalDefault,
		AccountBinding: integrations.AccountBinding{
			AccountKey:   "acct_calendar",
			AccountLabel: "Primary Calendar",
		},
		BackendBinding: integrations.BackendBinding{
			BackendKind:           integrations.BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	resource, err = manager.UpdateReadiness(resource.IntegrationID, integrations.UpdateReadinessInput{
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		AuthState:        integrations.AuthStateAuthorized,
		HealthState:      integrations.HealthStateHealthy,
		SecretResolution: "resolved",
	})
	if err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}
	if sqliteStore != nil {
		if err := sqliteStore.UpsertIntegration(context.Background(), resource); err != nil {
			t.Fatalf("UpsertIntegration returned error: %v", err)
		}
	}
	return resource
}

func TestCalendarRoutesSupportSelectionFallbackAndAvailability(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	t.Cleanup(eventBus.Close)

	integrationManager := integrations.NewManager("test")
	seedHealthyCalendarIntegration(t, integrationManager, sqliteStore, "calendar-a", true)
	seedHealthyCalendarIntegration(t, integrationManager, sqliteStore, "calendar-b", false)
	calendarManager := calendar.NewManager("test")

	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     t.TempDir(),
			LogLevel:    "info",
			Version:     "test",
		},
		Logger:       telemetry.New("error").Slog(),
		EventBus:     eventBus,
		Integrations: integrationManager,
		Calendar:     calendarManager,
		Store:        sqliteStore,
	})

	accountRec := httptest.NewRecorder()
	accountReq := httptest.NewRequest(http.MethodGet, "/v1/calendar/accounts?canonicalDefault=true", nil)
	server.Handler().ServeHTTP(accountRec, accountReq)
	if accountRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for accounts, got %d body=%s", accountRec.Code, accountRec.Body.String())
	}
	accounts := decodeStrictResponse[CalendarAccountListResponse](t, accountRec.Body.Bytes())
	if len(accounts.Items) != 1 || accounts.Items[0].IntegrationID != "calendar-a" {
		t.Fatalf("expected canonical default account projection, got %+v", accounts.Items)
	}
	if accounts.Items[0].PrimaryTimezone == "" || accounts.Items[0].PrimaryCalendarRef == "" {
		t.Fatalf("expected projected primary calendar metadata, got %+v", accounts.Items[0])
	}

	explicitRec := httptest.NewRecorder()
	explicitReq := httptest.NewRequest(http.MethodGet, "/v1/calendar/events?integrationId=calendar-b", nil)
	server.Handler().ServeHTTP(explicitRec, explicitReq)
	if explicitRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for explicit event list, got %d body=%s", explicitRec.Code, explicitRec.Body.String())
	}
	explicit := decodeStrictResponse[CalendarEventListResponse](t, explicitRec.Body.Bytes())
	if explicit.Account.IntegrationID != "calendar-b" || explicit.Operation.SelectionMode != "explicit" {
		t.Fatalf("expected explicit calendar selection, got account=%+v operation=%+v", explicit.Account, explicit.Operation)
	}
	if len(explicit.Items) == 0 || len(explicit.Artifacts) == 0 {
		t.Fatalf("expected fake events and artifacts, got %+v", explicit)
	}

	defaultRec := httptest.NewRecorder()
	defaultReq := httptest.NewRequest(http.MethodGet, "/v1/calendar/events", nil)
	server.Handler().ServeHTTP(defaultRec, defaultReq)
	if defaultRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for default event list, got %d body=%s", defaultRec.Code, defaultRec.Body.String())
	}
	defaults := decodeStrictResponse[CalendarEventListResponse](t, defaultRec.Body.Bytes())
	if defaults.Account.IntegrationID != "calendar-a" || defaults.Operation.SelectionMode != "canonical_default" {
		t.Fatalf("expected canonical default fallback, got account=%+v operation=%+v", defaults.Account, defaults.Operation)
	}

	queryRec := httptest.NewRecorder()
	queryReq := httptest.NewRequest(http.MethodPost, "/v1/calendar/availability/queries", strings.NewReader(`{
		"integrationId":"calendar-b",
		"windowStart":"2026-04-23T09:00:00-07:00",
		"windowEnd":"2026-04-23T11:00:00-07:00"
	}`))
	queryReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for availability query, got %d body=%s", queryRec.Code, queryRec.Body.String())
	}
	query := decodeStrictResponse[CalendarAvailabilityQueryResponse](t, queryRec.Body.Bytes())
	if query.Query.Timezone != "America/Los_Angeles" {
		t.Fatalf("expected primary timezone default, got %+v", query.Query)
	}
	if query.Operation.OperationClass != calendar.OperationClassBusyFree || query.Operation.Status != calendar.OperationStatusCompleted {
		t.Fatalf("expected completed busy_free operation, got %+v", query.Operation)
	}

	persistedOps, err := sqliteStore.ListCalendarOperations(context.Background(), "test", store.CalendarOperationFilter{IntegrationID: "calendar-b"})
	if err != nil {
		t.Fatalf("ListCalendarOperations returned error: %v", err)
	}
	if len(persistedOps) < 2 {
		t.Fatalf("expected persisted explicit list and busy_free operations, got %+v", persistedOps)
	}
}

func TestCalendarMutationRoutesPreserveIdentityAndRejectUnsupportedRequests(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	t.Cleanup(eventBus.Close)

	integrationManager := integrations.NewManager("test")
	seedHealthyCalendarIntegration(t, integrationManager, sqliteStore, "calendar-a", true)
	calendarManager := calendar.NewManager("test")

	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     t.TempDir(),
			LogLevel:    "info",
			Version:     "test",
		},
		Logger:       telemetry.New("error").Slog(),
		EventBus:     eventBus,
		Integrations: integrationManager,
		Calendar:     calendarManager,
		Store:        sqliteStore,
	})

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/calendar/events", strings.NewReader(`{
		"integrationId":"calendar-a",
		"title":"Phase 29 event",
		"startsAt":"2026-04-23T13:00:00-07:00",
		"endsAt":"2026-04-23T13:30:00-07:00",
		"location":"Desk"
	}`))
	createReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	created := decodeStrictResponse[CalendarEventResponse](t, createRec.Body.Bytes())

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPost, "/v1/calendar/events/"+created.Event.ExternalEventID+"/update", strings.NewReader(`{
		"title":"Phase 29 moved",
		"startsAt":"2026-04-23T14:00:00-07:00",
		"endsAt":"2026-04-23T14:30:00-07:00"
	}`))
	updateReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}
	updated := decodeStrictResponse[CalendarEventResponse](t, updateRec.Body.Bytes())

	cancelRec := httptest.NewRecorder()
	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/calendar/events/"+created.Event.ExternalEventID+"/cancel", strings.NewReader(`{}`))
	cancelReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for cancel, got %d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	cancelled := decodeStrictResponse[CalendarEventResponse](t, cancelRec.Body.Bytes())

	if created.Event.ExternalEventID == "" || updated.Event.ExternalEventID != created.Event.ExternalEventID || cancelled.Event.ExternalEventID != created.Event.ExternalEventID {
		t.Fatalf("expected stable event identity, got create=%+v update=%+v cancel=%+v", created.Event, updated.Event, cancelled.Event)
	}
	if cancelled.Event.LifecycleState != calendar.EventLifecycleStateCancelled {
		t.Fatalf("expected cancelled lifecycle state, got %+v", cancelled.Event)
	}

	// Roadmap 62: all-day creates are now accepted with date boundaries.
	allDayRec := httptest.NewRecorder()
	allDayReq := httptest.NewRequest(http.MethodPost, "/v1/calendar/events", strings.NewReader(`{
		"title":"All day event",
		"startsAt":"2026-04-24T00:00:00Z",
		"endsAt":"2026-04-25T00:00:00Z",
		"allDay":true,
		"startDate":"2026-04-24",
		"endDate":"2026-04-25"
	}`))
	allDayReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(allDayRec, allDayReq)
	if allDayRec.Code != http.StatusCreated || !strings.Contains(allDayRec.Body.String(), `"allDay":true`) {
		t.Fatalf("expected all-day acceptance, got %d body=%s", allDayRec.Code, allDayRec.Body.String())
	}

	// Roadmap 61: attendee-bearing updates are now accepted and record an attendee outcome.
	attendeeRec := httptest.NewRecorder()
	attendeeReq := httptest.NewRequest(http.MethodPost, "/v1/calendar/events/"+created.Event.ExternalEventID+"/update", strings.NewReader(`{
		"title":"Attendee update",
		"startsAt":"2026-04-23T15:00:00-07:00",
		"endsAt":"2026-04-23T15:30:00-07:00",
		"attendees":[{"email":"bob@example.com","role":"required"}],
		"notifyAttendees":true
	}`))
	attendeeReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(attendeeRec, attendeeReq)
	if attendeeRec.Code != http.StatusOK {
		t.Fatalf("expected attendee update accepted, got %d body=%s", attendeeRec.Code, attendeeRec.Body.String())
	}
	if !strings.Contains(attendeeRec.Body.String(), "bob@example.com") || !strings.Contains(attendeeRec.Body.String(), "attendeeOutcome") {
		t.Fatalf("expected attendee details + outcome in response, got %s", attendeeRec.Body.String())
	}

	alternateRec := httptest.NewRecorder()
	alternateReq := httptest.NewRequest(http.MethodPost, "/v1/calendar/events/"+created.Event.ExternalEventID+"/cancel", strings.NewReader(`{"calendarRef":"secondary"}`))
	alternateReq.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(alternateRec, alternateReq)
	if alternateRec.Code != http.StatusBadRequest || !strings.Contains(alternateRec.Body.String(), calendar.ErrCalendarAlternateCalendarDeny.Error()) {
		t.Fatalf("expected alternate-calendar rejection, got %d body=%s", alternateRec.Code, alternateRec.Body.String())
	}

	artifacts, err := sqliteStore.ListCalendarArtifacts(context.Background(), "test", "")
	if err != nil {
		t.Fatalf("ListCalendarArtifacts returned error: %v", err)
	}
	if len(artifacts) < 3 {
		t.Fatalf("expected persisted create/update/cancel artifacts, got %+v", artifacts)
	}

	timezoneUsed := created.Operation.TimezoneUsed
	if timezoneUsed != "America/Los_Angeles" {
		t.Fatalf("expected primary timezone default on create, got %s", timezoneUsed)
	}
}
