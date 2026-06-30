package feishulark_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterprovider"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/providers/feishulark"
)

// feishuMux returns a handler that always answers the primary-calendar probe (the Manager
// projects the account before each operation) and delegates the operation path to op.
func feishuMux(op http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open-apis/calendar/v4/calendars/primary" {
			writeFeishu(w, map[string]any{
				"calendars": []map[string]any{
					{"user_id": "ou_user", "calendar": map[string]any{"calendar_id": "cal_primary", "summary": "Primary", "role": "owner"}},
				},
			})
			return
		}
		op(w, r)
	}
}

func writeFeishu(w http.ResponseWriter, data any) {
	raw, _ := json.Marshal(data)
	env, _ := json.Marshal(map[string]any{"code": 0, "msg": "success", "data": json.RawMessage(raw)})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(env)
}

func writeFeishuCode(w http.ResponseWriter, code int, msg string) {
	env, _ := json.Marshal(map[string]any{"code": code, "msg": msg})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(env)
}

func wiredManager(t *testing.T, h http.HandlerFunc) (*calendar.Manager, []integrations.Resource) {
	t.Helper()
	srv := httptest.NewServer(h)
	provider := feishulark.NewCalendarProvider(feishulark.NewClient(srv.URL, srv.Client()))

	adapterReader, daemonWriter := io.Pipe() // daemon -> adapter
	daemonReader, adapterWriter := io.Pipe() // adapter -> daemon
	go func() {
		_ = adapterprovider.Serve(adapterReader, adapterWriter, provider)
		_ = adapterWriter.Close()
	}()

	resolver := adapterrpc.ScopedResolver(func(ctx context.Context, integrationID string) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"accessToken": "scoped-token", "grantedScopes": []string{"calendar:calendar"}})
	})
	client := adapterrpc.NewClient(daemonWriter, daemonReader).WithCredentials(resolver)

	m := calendar.NewManager("test")
	m.RegisterBackend(integrations.BackendKindAdapterRPC, calendar.NewAdapterBackend(client, 2*time.Second).WithProviderKind(string(integrations.BackendKindFeishuLark)))

	t.Cleanup(func() {
		_ = daemonWriter.Close()
		_ = adapterWriter.Close()
		srv.Close()
	})
	resource := integrations.Resource{
		IntegrationID:    "int-cal-1",
		DomainKind:       "calendar",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		CanonicalDefault: true,
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindAdapterRPC},
	}
	return m, []integrations.Resource{resource}
}

func sel() calendar.Selection { return calendar.Selection{IntegrationID: "int-cal-1"} }

// US1 (FR-001/FR-002, SC-001): read closure maps real provider responses onto existing
// calendar resources, preserving identity and absolute timing across timezone boundaries.
func TestReadClosureMapsEvents(t *testing.T) {
	start := time.Date(2026, 3, 8, 1, 30, 0, 0, time.UTC) // US DST spring-forward boundary
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events"):
			writeFeishu(w, map[string]any{"items": []map[string]any{{
				"event_id":   "evt-1",
				"summary":    "Standup",
				"start_time": map[string]any{"timestamp": "1741397400", "timezone": "America/New_York"},
				"end_time":   map[string]any{"timestamp": "1741401000", "timezone": "America/New_York"},
				"status":     "confirmed",
			}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))

	account, events, op, _, err := m.ListEvents(resources, calendar.ListEventsInput{
		Selection: sel(),
		StartsAt:  &start,
		EndsAt:    ptr(start.Add(24 * time.Hour)),
	})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if account.PrimaryCalendarRef != "cal_primary" {
		t.Fatalf("primary calendar ref = %q", account.PrimaryCalendarRef)
	}
	if op.Status != calendar.OperationStatusCompleted {
		t.Fatalf("op status = %q", op.Status)
	}
	if len(events) != 1 || events[0].ExternalEventID != "evt-1" {
		t.Fatalf("events = %+v", events)
	}
	if !events[0].StartsAt.Equal(time.Unix(1741397400, 0).UTC()) {
		t.Fatalf("absolute start not preserved: %v", events[0].StartsAt)
	}
	if events[0].Timezone != "America/New_York" {
		t.Fatalf("timezone not preserved: %q", events[0].Timezone)
	}
}

func TestBusyFreeMapsIntervals(t *testing.T) {
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/freebusy/list") {
			writeFeishu(w, map[string]any{"freebusy_list": []map[string]any{
				{"start_time": "2026-03-08T09:00:00Z", "end_time": "2026-03-08T10:00:00Z"},
			}})
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	win := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	_, q, op, _, err := m.BusyFree(resources, calendar.BusyFreeInput{Selection: sel(), WindowStart: win, WindowEnd: win.Add(24 * time.Hour)})
	if err != nil {
		t.Fatalf("BusyFree: %v", err)
	}
	if op.Status != calendar.OperationStatusCompleted {
		t.Fatalf("op status = %q", op.Status)
	}
	if q.ConflictCount != 1 || len(q.BusyIntervals) != 1 {
		t.Fatalf("availability = %+v", q)
	}
}

// US2 (FR-002, SC-002): create/update/cancel preserve event identity as distinct operations.
func TestWriteClosurePreservesIdentity(t *testing.T) {
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events"):
			writeFeishu(w, map[string]any{"event": map[string]any{
				"event_id":   "evt-100",
				"summary":    "Review",
				"start_time": map[string]any{"timestamp": "1741397400", "timezone": "UTC"},
				"end_time":   map[string]any{"timestamp": "1741401000", "timezone": "UTC"},
				"status":     "confirmed",
			}})
		case r.Method == http.MethodPatch:
			writeFeishu(w, map[string]any{"event": map[string]any{
				"event_id":   "evt-100",
				"summary":    "Review (moved)",
				"start_time": map[string]any{"timestamp": "1741404600", "timezone": "UTC"},
				"end_time":   map[string]any{"timestamp": "1741408200", "timezone": "UTC"},
				"status":     "confirmed",
			}})
		case r.Method == http.MethodDelete:
			writeFeishu(w, map[string]any{})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))

	base := time.Date(2026, 3, 8, 1, 30, 0, 0, time.UTC)
	_, created, createOp, _, err := m.CreateEvent(resources, calendar.CreateEventInput{Selection: sel(), Title: "Review", StartsAt: base, EndsAt: base.Add(time.Hour)})
	if err != nil || created.ExternalEventID != "evt-100" || createOp.OperationClass != calendar.OperationClassCreateEvent {
		t.Fatalf("create: %v op=%+v ev=%+v", err, createOp, created)
	}
	_, updated, updateOp, _, err := m.UpdateEvent(resources, calendar.UpdateEventInput{Selection: sel(), ExternalEventID: "evt-100", Title: "Review (moved)", StartsAt: base.Add(time.Hour), EndsAt: base.Add(2 * time.Hour)})
	if err != nil || updated.ExternalEventID != "evt-100" || updateOp.OperationClass != calendar.OperationClassUpdateEvent {
		t.Fatalf("update: %v op=%+v ev=%+v", err, updateOp, updated)
	}
	_, cancelled, cancelOp, _, err := m.CancelEvent(resources, calendar.CancelEventInput{Selection: sel(), ExternalEventID: "evt-100"})
	if err != nil || cancelled.LifecycleState != calendar.EventLifecycleStateCancelled || cancelOp.OperationClass != calendar.OperationClassCancelEvent {
		t.Fatalf("cancel: %v op=%+v ev=%+v", err, cancelOp, cancelled)
	}
	if got := len(m.ListOperations(calendar.OperationFilter{})); got != 3 {
		t.Fatalf("operations recorded = %d, want 3 distinct on single ledger", got)
	}
}

// US3 (FR-006, SC-003, SC-005): provider OAuth/scope/token failures map to stable diagnostics
// reason codes via the feishu_lark provider kind; no raw provider message leaks.
func TestDiagnosticsMapStableReasons(t *testing.T) {
	cases := []struct {
		name       string
		handler    http.HandlerFunc
		wantReason integrations.DiagnosticReasonCode
	}{
		{"scope", feishuMux(func(w http.ResponseWriter, r *http.Request) {
			writeFeishuCode(w, 99991669, "scope not granted secret-detail")
		}), integrations.ReasonScopeMissing},
		{"user_token", feishuMux(func(w http.ResponseWriter, r *http.Request) {
			writeFeishuCode(w, 99991668, "user token invalid secret-detail")
		}), integrations.ReasonUserAuthorizationMissing},
		{"expired", feishuMux(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{}`))
		}), integrations.ReasonTokenExpired},
		{"rate", feishuMux(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{}`))
		}), integrations.ReasonRateLimited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, resources := wiredManager(t, tc.handler)
			start := time.Date(2026, 3, 8, 1, 0, 0, 0, time.UTC)
			_, _, op, _, err := m.ListEvents(resources, calendar.ListEventsInput{Selection: sel(), StartsAt: &start, EndsAt: ptr(start.Add(time.Hour))})
			if err == nil {
				t.Fatal("expected provider failure")
			}
			if op.Status != calendar.OperationStatusFailed {
				t.Fatalf("op status = %q", op.Status)
			}
			if op.DiagnosticFailure == nil || op.DiagnosticFailure.ReasonCode != tc.wantReason {
				t.Fatalf("diagnostic = %+v, want reason %s", op.DiagnosticFailure, tc.wantReason)
			}
			if strings.Contains(op.FailureReason, "secret-detail") {
				t.Fatalf("raw provider message leaked: %q", op.FailureReason)
			}
		})
	}
}

// US2 (FR-008, SC-004): an ambiguous provider write acknowledgement is recorded as
// ambiguous-commit, not coerced to success or failure.
func TestAmbiguousWriteRecordedAsAmbiguousCommit(t *testing.T) {
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events") {
			w.WriteHeader(http.StatusInternalServerError) // server error after a write was submitted
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	base := time.Date(2026, 3, 8, 1, 30, 0, 0, time.UTC)
	_, _, op, _, err := m.CreateEvent(resources, calendar.CreateEventInput{Selection: sel(), Title: "ambiguous", StartsAt: base, EndsAt: base.Add(time.Hour)})
	if err == nil {
		t.Fatal("expected an error for the ambiguous write")
	}
	if op.Status != calendar.OperationStatusFailed || op.FailureClass != "ambiguous_commit" {
		t.Fatalf("op = status:%q class:%q, want failed/ambiguous_commit", op.Status, op.FailureClass)
	}
}

// US2 (FR-010, AS6): out-of-scope mutations are rejected before any real provider call.
func TestOutOfScopeMutationsRejectedBeforeProviderCall(t *testing.T) {
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("out-of-scope mutation must not reach the provider: %s %s", r.Method, r.URL.Path)
	}))
	base := time.Date(2026, 3, 8, 1, 30, 0, 0, time.UTC)
	cases := []struct {
		name  string
		input calendar.CreateEventInput
	}{
		{"recurring", calendar.CreateEventInput{Selection: sel(), Title: "x", StartsAt: base, EndsAt: base.Add(time.Hour), Recurring: true}},
		{"all_day", calendar.CreateEventInput{Selection: sel(), Title: "x", StartsAt: base, EndsAt: base.Add(time.Hour), AllDay: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, _, err := m.CreateEvent(resources, tc.input); err == nil {
				t.Fatal("expected out-of-scope rejection")
			}
		})
	}
}

// US1 (FR-001/FR-002, SC-001): attendee-bearing create records event-field mutation plus
// per-attendee invitation results and the requested notification behavior.
func TestCreateWithAttendeesRecordsOutcome(t *testing.T) {
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events") {
			writeFeishu(w, map[string]any{"event": map[string]any{
				"event_id":   "evt-att",
				"summary":    "Sync",
				"start_time": map[string]any{"timestamp": "1741397400", "timezone": "UTC"},
				"end_time":   map[string]any{"timestamp": "1741401000", "timezone": "UTC"},
				"status":     "confirmed",
				"attendees": []map[string]any{
					{"type": "third_party", "third_party_email": "a@example.com", "rsvp_status": "needs_action"},
				},
			}})
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	base := time.Date(2026, 3, 8, 1, 30, 0, 0, time.UTC)
	_, ev, op, _, err := m.CreateEvent(resources, calendar.CreateEventInput{
		Selection:        sel(),
		Title:            "Sync",
		StartsAt:         base,
		EndsAt:           base.Add(time.Hour),
		AttendeeRequests: []calendar.AttendeeRequest{{Email: "a@example.com", Role: calendar.AttendeeRoleRequired}},
		NotifyAttendees:  true,
	})
	if err != nil {
		t.Fatalf("create with attendees: %v", err)
	}
	if len(ev.AttendeeDetails) != 1 || ev.AttendeeDetails[0].InvitationStatus != calendar.InvitationStatusSent {
		t.Fatalf("attendee invitation not recorded as sent: %+v", ev.AttendeeDetails)
	}
	if op.AttendeeOutcome == nil || op.AttendeeOutcome.NotificationBehavior != calendar.NotificationBehaviorNotify {
		t.Fatalf("attendee outcome = %+v, want notify", op.AttendeeOutcome)
	}
}

// US2 (FR-001): an attendee-only update is a distinct operation; field mutation untouched.
func TestUpdateAttendeesDistinctOperation(t *testing.T) {
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/attendees"):
			writeFeishu(w, map[string]any{})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events/"):
			writeFeishu(w, map[string]any{"event": map[string]any{
				"event_id":   "evt-1",
				"summary":    "Sync",
				"start_time": map[string]any{"timestamp": "1741397400", "timezone": "UTC"},
				"end_time":   map[string]any{"timestamp": "1741401000", "timezone": "UTC"},
				"status":     "confirmed",
				"attendees": []map[string]any{
					{"type": "third_party", "third_party_email": "a@example.com", "rsvp_status": "accept"},
				},
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	_, ev, op, _, err := m.UpdateAttendees(resources, calendar.UpdateAttendeesInput{
		Selection:       sel(),
		ExternalEventID: "evt-1",
		AddAttendees:    []calendar.AttendeeRequest{{Email: "a@example.com"}},
		Notify:          true,
	})
	if err != nil {
		t.Fatalf("UpdateAttendees: %v", err)
	}
	if op.OperationClass != calendar.OperationClassUpdateAttendees {
		t.Fatalf("op class = %q, want update_attendees", op.OperationClass)
	}
	if len(ev.AttendeeDetails) != 1 || ev.AttendeeDetails[0].RSVP != calendar.RSVPStatusAccepted {
		t.Fatalf("RSVP not projected: %+v", ev.AttendeeDetails)
	}
}

// US3 (FR-003): RSVP state is projected from the provider on the read path.
func TestGetEventProjectsRSVP(t *testing.T) {
	m, resources := wiredManager(t, feishuMux(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/events/") {
			writeFeishu(w, map[string]any{"event": map[string]any{
				"event_id":   "evt-7",
				"summary":    "Review",
				"start_time": map[string]any{"timestamp": "1741397400", "timezone": "UTC"},
				"end_time":   map[string]any{"timestamp": "1741401000", "timezone": "UTC"},
				"status":     "confirmed",
				"attendees": []map[string]any{
					{"third_party_email": "a@example.com", "rsvp_status": "decline"},
					{"third_party_email": "b@example.com", "rsvp_status": "tentative", "is_optional": true},
				},
			}})
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	_, ev, _, _, err := m.GetEvent(resources, calendar.GetEventInput{Selection: sel(), ExternalEventID: "evt-7"})
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if len(ev.AttendeeDetails) != 2 {
		t.Fatalf("attendees not projected: %+v", ev.AttendeeDetails)
	}
	got := map[string]calendar.RSVPStatus{}
	for _, a := range ev.AttendeeDetails {
		got[a.Email] = a.RSVP
	}
	if got["a@example.com"] != calendar.RSVPStatusDeclined || got["b@example.com"] != calendar.RSVPStatusTentative {
		t.Fatalf("RSVP states wrong: %+v", got)
	}
}

func ptr[T any](v T) *T { return &v }
