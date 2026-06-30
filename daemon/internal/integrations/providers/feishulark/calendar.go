package feishulark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterprovider"
)

// CalendarProvider implements the adapterprovider.Handler for the Feishu/Lark calendar domain
// (Roadmap 60). It maps the Feishu Open Platform Calendar API onto the existing calendar domain
// resources. It is stateless and records no ledger state; the daemon owns that.
type CalendarProvider struct {
	client *Client
}

// NewCalendarProvider builds a calendar provider over the given client.
func NewCalendarProvider(client *Client) *CalendarProvider {
	return &CalendarProvider{client: client}
}

var _ adapterprovider.Handler = (*CalendarProvider)(nil)

// Handle dispatches one calendar operation. Faults are converted to the serve-harness Fault;
// unconfirmed write outcomes return adapterprovider.ErrAmbiguous (FR-008).
func (p *CalendarProvider) Handle(ctx context.Context, op adapterprovider.Operation) (json.RawMessage, error) {
	if op.Domain != "calendar" {
		return nil, (&providerFault{kind: faultInternal, code: "unsupported_domain", message: "adapter serves the calendar domain only"}).toAdapterFault()
	}
	token, fault := parseToken(op.Credential)
	if fault != nil {
		return nil, fault.toAdapterFault()
	}
	var resource integrations.Resource
	if len(op.Resource) > 0 {
		_ = json.Unmarshal(op.Resource, &resource)
	}

	result, pf := p.route(ctx, token, resource, op)
	if pf != nil {
		if pf.isAmbiguous() {
			return nil, adapterprovider.ErrAmbiguous
		}
		return nil, pf.toAdapterFault()
	}
	return result, nil
}

func (p *CalendarProvider) route(ctx context.Context, token scopedToken, resource integrations.Resource, op adapterprovider.Operation) (json.RawMessage, *providerFault) {
	switch op.Operation {
	case "ProjectAccount":
		return marshalResult(p.projectAccount(ctx, token, resource))
	case "ListEvents":
		var in struct {
			Account calendar.AccountProjection `json:"account"`
			Input   calendar.ListEventsInput   `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.listEvents(ctx, token, in.Account, in.Input))
	case "GetEvent":
		var in struct {
			Account calendar.AccountProjection `json:"account"`
			EventID string                     `json:"eventId"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.getEvent(ctx, token, in.Account, in.EventID))
	case "BusyFree":
		var in struct {
			Account calendar.AccountProjection `json:"account"`
			Input   calendar.BusyFreeInput     `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.busyFree(ctx, token, in.Account, in.Input))
	case "CreateEvent":
		var in struct {
			Account calendar.AccountProjection `json:"account"`
			Input   calendar.CreateEventInput  `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.createEvent(ctx, token, in.Account, in.Input))
	case "UpdateEvent":
		var in struct {
			Account calendar.AccountProjection `json:"account"`
			Input   calendar.UpdateEventInput  `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.updateEvent(ctx, token, in.Account, in.Input))
	case "CancelEvent":
		var in struct {
			Account calendar.AccountProjection `json:"account"`
			Input   calendar.CancelEventInput  `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.cancelEvent(ctx, token, in.Account, in.Input))
	case "UpdateAttendees":
		var in struct {
			Account calendar.AccountProjection    `json:"account"`
			Input   calendar.UpdateAttendeesInput `json:"input"`
		}
		if pf := decodePayload(op.Payload, &in); pf != nil {
			return nil, pf
		}
		return marshalResult(p.updateAttendees(ctx, token, in.Account, in.Input))
	default:
		return nil, &providerFault{kind: faultInternal, code: "unsupported_operation", message: "unsupported calendar operation"}
	}
}

// ---- Feishu API response shapes ----

type feishuTimeInfo struct {
	Timestamp string `json:"timestamp"` // unix seconds as string
	Date      string `json:"date"`      // all-day date (YYYY-MM-DD); not mutated in this phase
	Timezone  string `json:"timezone"`
}

type feishuEvent struct {
	EventID     string         `json:"event_id"`
	Summary     string         `json:"summary"`
	Description string         `json:"description"`
	StartTime   feishuTimeInfo `json:"start_time"`
	EndTime     feishuTimeInfo `json:"end_time"`
	Status      string         `json:"status"` // tentative | confirmed | cancelled
	Location    struct {
		Name string `json:"name"`
	} `json:"location"`
	Recurrence string           `json:"recurrence"`
	Attendees  []feishuAttendee `json:"attendees"`
}

// feishuAttendee mirrors a Feishu calendar event attendee. RSVP is carried in rsvp_status.
type feishuAttendee struct {
	Type            string `json:"type"`
	AttendeeID      string `json:"attendee_id"`
	RsvpStatus      string `json:"rsvp_status"` // needs_action | accept | decline | tentative
	DisplayName     string `json:"display_name"`
	IsOptional      bool   `json:"is_optional"`
	ThirdPartyEmail string `json:"third_party_email"`
	UserID          string `json:"user_id"`
}

type feishuPrimaryResp struct {
	Calendars []struct {
		Calendar struct {
			CalendarID string `json:"calendar_id"`
			Summary    string `json:"summary"`
			Role       string `json:"role"`
		} `json:"calendar"`
		UserID string `json:"user_id"`
	} `json:"calendars"`
}

func (p *CalendarProvider) projectAccount(ctx context.Context, token scopedToken, resource integrations.Resource) (calendar.AccountProjection, *providerFault) {
	var out feishuPrimaryResp
	if pf := p.client.call(ctx, "POST", "/open-apis/calendar/v4/calendars/primary?user_id_type=open_id", token.AccessToken, map[string]any{}, &out, false); pf != nil {
		return calendar.AccountProjection{}, pf
	}
	if len(out.Calendars) == 0 {
		return calendar.AccountProjection{}, &providerFault{kind: faultUnavailable, code: "primary_calendar_missing", message: "no primary calendar returned"}
	}
	primary := out.Calendars[0]
	now := time.Now().UTC()
	return calendar.AccountProjection{
		CalendarAccountID:       "fl_" + primary.UserID,
		IntegrationID:           resource.IntegrationID,
		DomainKind:              "calendar",
		EnvironmentScope:        resource.EnvironmentScope,
		AccountKey:              primary.UserID,
		AccountLabel:            primary.Calendar.Summary,
		ReadinessStatus:         string(integrations.ReadinessStatusHealthy),
		CanonicalDefault:        resource.AccountBinding.AccountType == "primary",
		PrimaryCalendarRef:      primary.Calendar.CalendarID,
		PrimaryCalendarLabel:    primary.Calendar.Summary,
		PrimaryTimezone:         normalizeTZ(resource),
		SupportsEventInspection: true,
		SupportsBusyFree:        true,
		SupportsTimedMutation:   true,
		LastSyncedAt:            now,
		UpdatedAt:               now,
	}, nil
}

func (p *CalendarProvider) listEvents(ctx context.Context, token scopedToken, account calendar.AccountProjection, input calendar.ListEventsInput) ([]calendar.Event, *providerFault) {
	q := url.Values{}
	if input.StartsAt != nil {
		q.Set("start_time", strconv.FormatInt(input.StartsAt.Unix(), 10))
	}
	if input.EndsAt != nil {
		q.Set("end_time", strconv.FormatInt(input.EndsAt.Unix(), 10))
	}
	q.Set("page_size", "100")
	path := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events?%s", url.PathEscape(account.PrimaryCalendarRef), q.Encode())
	var out struct {
		Items []feishuEvent `json:"items"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return nil, pf
	}
	events := make([]calendar.Event, 0, len(out.Items))
	for _, item := range out.Items {
		events = append(events, mapEvent(account, item))
	}
	return events, nil
}

func (p *CalendarProvider) getEvent(ctx context.Context, token scopedToken, account calendar.AccountProjection, eventID string) (calendar.Event, *providerFault) {
	path := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events/%s", url.PathEscape(account.PrimaryCalendarRef), url.PathEscape(eventID))
	var out struct {
		Event feishuEvent `json:"event"`
	}
	if pf := p.client.call(ctx, "GET", path, token.AccessToken, nil, &out, false); pf != nil {
		return calendar.Event{}, pf
	}
	return mapEvent(account, out.Event), nil
}

func (p *CalendarProvider) busyFree(ctx context.Context, token scopedToken, account calendar.AccountProjection, input calendar.BusyFreeInput) (calendar.AvailabilityQuery, *providerFault) {
	body := map[string]any{
		"time_min": input.WindowStart.UTC().Format(time.RFC3339),
		"time_max": input.WindowEnd.UTC().Format(time.RFC3339),
		"user_id":  account.AccountKey,
	}
	var out struct {
		FreebusyList []struct {
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
		} `json:"freebusy_list"`
	}
	if pf := p.client.call(ctx, "POST", "/open-apis/calendar/v4/freebusy/list?user_id_type=open_id", token.AccessToken, body, &out, false); pf != nil {
		return calendar.AvailabilityQuery{}, pf
	}
	intervals := make([]calendar.BusyInterval, 0, len(out.FreebusyList))
	for _, fb := range out.FreebusyList {
		start, err1 := time.Parse(time.RFC3339, fb.StartTime)
		end, err2 := time.Parse(time.RFC3339, fb.EndTime)
		if err1 != nil || err2 != nil {
			continue
		}
		intervals = append(intervals, calendar.BusyInterval{StartsAt: start.UTC(), EndsAt: end.UTC()})
	}
	return calendar.AvailabilityQuery{
		IntegrationID:     account.IntegrationID,
		CalendarAccountID: account.CalendarAccountID,
		WindowStart:       input.WindowStart.UTC(),
		WindowEnd:         input.WindowEnd.UTC(),
		Timezone:          firstNonEmpty(input.Timezone, account.PrimaryTimezone),
		BusyIntervals:     intervals,
		ConflictCount:     len(intervals),
		ResultSummary:     fmt.Sprintf("%d busy interval(s)", len(intervals)),
	}, nil
}

func (p *CalendarProvider) createEvent(ctx context.Context, token scopedToken, account calendar.AccountProjection, input calendar.CreateEventInput) (calendar.Event, *providerFault) {
	requests := resolveRequests(input.AttendeeRequests, input.Attendees)
	body := writeEventBody(eventBodyInput{
		title: input.Title, description: input.Description, location: input.Location,
		tz:       firstNonEmpty(input.Timezone, account.PrimaryTimezone),
		startsAt: input.StartsAt, endsAt: input.EndsAt,
		allDay: input.AllDay, startDate: input.StartDate, endDate: input.EndDate,
		recurrenceRule: input.RecurrenceRule, attendees: requests,
	})
	path := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events?need_notification=%t", url.PathEscape(account.PrimaryCalendarRef), input.NotifyAttendees)
	var out struct {
		Event feishuEvent `json:"event"`
	}
	if pf := p.client.call(ctx, "POST", path, token.AccessToken, body, &out, true); pf != nil {
		return calendar.Event{}, pf
	}
	ev := mapEvent(account, out.Event)
	applyInvitationStatus(&ev, requests, input.NotifyAttendees)
	return ev, nil
}

func (p *CalendarProvider) updateEvent(ctx context.Context, token scopedToken, account calendar.AccountProjection, input calendar.UpdateEventInput) (calendar.Event, *providerFault) {
	requests := resolveRequests(input.AttendeeRequests, input.Attendees)
	body := writeEventBody(eventBodyInput{
		title: input.Title, description: input.Description, location: input.Location,
		tz:       firstNonEmpty(input.Timezone, account.PrimaryTimezone),
		startsAt: input.StartsAt, endsAt: input.EndsAt,
		allDay: input.AllDay, startDate: input.StartDate, endDate: input.EndDate,
		recurrenceRule: input.RecurrenceRule, attendees: requests,
	})
	path := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events/%s?need_notification=%t", url.PathEscape(account.PrimaryCalendarRef), url.PathEscape(input.ExternalEventID), input.NotifyAttendees)
	var out struct {
		Event feishuEvent `json:"event"`
	}
	if pf := p.client.call(ctx, "PATCH", path, token.AccessToken, body, &out, true); pf != nil {
		return calendar.Event{}, pf
	}
	ev := mapEvent(account, out.Event)
	if ev.ExternalEventID == "" {
		ev.ExternalEventID = input.ExternalEventID // identity preserved across update (FR-004)
	}
	applyInvitationStatus(&ev, requests, input.NotifyAttendees)
	return ev, nil
}

func (p *CalendarProvider) updateAttendees(ctx context.Context, token scopedToken, account calendar.AccountProjection, input calendar.UpdateAttendeesInput) (calendar.Event, *providerFault) {
	eventPath := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events/%s", url.PathEscape(account.PrimaryCalendarRef), url.PathEscape(input.ExternalEventID))
	if len(input.AddAttendees) > 0 {
		body := map[string]any{"attendees": attendeeBody(resolveRequests(input.AddAttendees, nil)), "need_notification": input.Notify}
		if pf := p.client.call(ctx, "POST", eventPath+"/attendees", token.AccessToken, body, nil, true); pf != nil {
			return calendar.Event{}, pf
		}
	}
	if len(input.RemoveAttendees) > 0 {
		// Resolve emails to attendee ids from the current event, then batch-delete.
		var current struct {
			Event feishuEvent `json:"event"`
		}
		if pf := p.client.call(ctx, "GET", eventPath, token.AccessToken, nil, &current, false); pf != nil {
			return calendar.Event{}, pf
		}
		ids := resolveAttendeeIDs(current.Event.Attendees, input.RemoveAttendees)
		if len(ids) > 0 {
			body := map[string]any{"attendee_ids": ids, "need_notification": input.Notify}
			if pf := p.client.call(ctx, "POST", eventPath+"/attendees/batch_delete", token.AccessToken, body, nil, true); pf != nil {
				return calendar.Event{}, pf
			}
		}
	}
	var out struct {
		Event feishuEvent `json:"event"`
	}
	if pf := p.client.call(ctx, "GET", eventPath, token.AccessToken, nil, &out, false); pf != nil {
		return calendar.Event{}, pf
	}
	ev := mapEvent(account, out.Event)
	if ev.ExternalEventID == "" {
		ev.ExternalEventID = input.ExternalEventID
	}
	return ev, nil
}

func (p *CalendarProvider) cancelEvent(ctx context.Context, token scopedToken, account calendar.AccountProjection, input calendar.CancelEventInput) (calendar.Event, *providerFault) {
	path := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events/%s", url.PathEscape(account.PrimaryCalendarRef), url.PathEscape(input.ExternalEventID))
	if pf := p.client.call(ctx, "DELETE", path, token.AccessToken, nil, nil, true); pf != nil {
		return calendar.Event{}, pf
	}
	now := time.Now().UTC()
	return calendar.Event{
		ExternalEventID:   input.ExternalEventID,
		IntegrationID:     account.IntegrationID,
		CalendarAccountID: account.CalendarAccountID,
		CalendarRef:       account.PrimaryCalendarRef,
		LifecycleState:    calendar.EventLifecycleStateCancelled,
		CancelledAt:       &now,
		UpdatedAt:         now,
	}, nil
}

// ---- mapping helpers ----

type eventBodyInput struct {
	title, description, location, tz string
	startsAt, endsAt                 time.Time
	allDay                           bool
	startDate, endDate               string
	recurrenceRule                   string
	attendees                        []calendar.AttendeeRequest
}

func writeEventBody(in eventBodyInput) map[string]any {
	start := map[string]any{"timezone": in.tz}
	end := map[string]any{"timezone": in.tz}
	if in.allDay {
		// All-day events use date boundaries (timezone-independent) rather than timestamps.
		start["date"] = firstNonEmpty(in.startDate, in.startsAt.UTC().Format("2006-01-02"))
		end["date"] = firstNonEmpty(in.endDate, in.endsAt.UTC().Format("2006-01-02"))
	} else {
		start["timestamp"] = strconv.FormatInt(in.startsAt.Unix(), 10)
		end["timestamp"] = strconv.FormatInt(in.endsAt.Unix(), 10)
	}
	body := map[string]any{
		"summary":     in.title,
		"description": in.description,
		"start_time":  start,
		"end_time":    end,
	}
	if strings.TrimSpace(in.location) != "" {
		body["location"] = map[string]any{"name": in.location}
	}
	if strings.TrimSpace(in.recurrenceRule) != "" {
		body["recurrence"] = in.recurrenceRule
	}
	if items := attendeeBody(in.attendees); len(items) > 0 {
		body["attendees"] = items
	}
	return body
}

// attendeeBody builds the Feishu attendee payload from attendee requests.
func attendeeBody(requests []calendar.AttendeeRequest) []map[string]any {
	if len(requests) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(requests))
	for _, r := range requests {
		out = append(out, map[string]any{
			"type":              "third_party",
			"third_party_email": r.Email,
			"is_optional":       r.Role == calendar.AttendeeRoleOptional,
		})
	}
	return out
}

// resolveRequests mirrors the calendar package's attendee normalization for the provider side:
// prefer explicit requests, otherwise synthesize from the legacy email list (required role).
func resolveRequests(requests []calendar.AttendeeRequest, emails []string) []calendar.AttendeeRequest {
	if len(requests) > 0 {
		out := make([]calendar.AttendeeRequest, 0, len(requests))
		for _, r := range requests {
			if strings.TrimSpace(r.Email) == "" {
				continue
			}
			if r.Role == "" {
				r.Role = calendar.AttendeeRoleRequired
			}
			out = append(out, r)
		}
		return out
	}
	out := make([]calendar.AttendeeRequest, 0, len(emails))
	for _, e := range emails {
		if strings.TrimSpace(e) == "" {
			continue
		}
		out = append(out, calendar.AttendeeRequest{Email: strings.TrimSpace(e), Role: calendar.AttendeeRoleRequired})
	}
	return out
}

// mapAttendees projects Feishu attendees onto the calendar attendee resource, preserving RSVP.
func mapAttendees(items []feishuAttendee) []calendar.Attendee {
	if len(items) == 0 {
		return nil
	}
	out := make([]calendar.Attendee, 0, len(items))
	for _, a := range items {
		role := calendar.AttendeeRoleRequired
		if a.IsOptional {
			role = calendar.AttendeeRoleOptional
		}
		out = append(out, calendar.Attendee{
			Email:       firstNonEmpty(a.ThirdPartyEmail, a.DisplayName),
			DisplayName: a.DisplayName,
			Role:        role,
			RSVP:        mapRSVP(a.RsvpStatus),
		})
	}
	return out
}

func mapRSVP(s string) calendar.RSVPStatus {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "accept", "accepted":
		return calendar.RSVPStatusAccepted
	case "decline", "declined":
		return calendar.RSVPStatusDeclined
	case "tentative":
		return calendar.RSVPStatusTentative
	case "needs_action", "":
		return calendar.RSVPStatusNeedsAction
	default:
		return calendar.RSVPStatusUnknown
	}
}

// applyInvitationStatus records, per attendee returned by a write, whether the externally-visible
// invitation was sent (notify requested) or only added without notification.
func applyInvitationStatus(ev *calendar.Event, requests []calendar.AttendeeRequest, notify bool) {
	if len(ev.AttendeeDetails) == 0 && len(requests) > 0 {
		// Provider did not echo attendees; project from the request so the outcome is truthful.
		ev.AttendeeDetails = make([]calendar.Attendee, 0, len(requests))
		for _, r := range requests {
			ev.AttendeeDetails = append(ev.AttendeeDetails, calendar.Attendee{Email: r.Email, DisplayName: r.DisplayName, Role: r.Role, RSVP: calendar.RSVPStatusNeedsAction})
		}
	}
	status := calendar.InvitationStatusNotRequested
	if notify {
		status = calendar.InvitationStatusSent
	}
	for i := range ev.AttendeeDetails {
		ev.AttendeeDetails[i].InvitationStatus = status
	}
	ev.Attendees = attendeeEmailList(ev.AttendeeDetails)
}

func attendeeEmailList(details []calendar.Attendee) []string {
	if len(details) == 0 {
		return nil
	}
	emails := make([]string, 0, len(details))
	for _, a := range details {
		emails = append(emails, a.Email)
	}
	return emails
}

// resolveAttendeeIDs maps emails to Feishu attendee ids for removal.
func resolveAttendeeIDs(current []feishuAttendee, removeEmails []string) []string {
	want := make(map[string]bool, len(removeEmails))
	for _, e := range removeEmails {
		want[strings.ToLower(strings.TrimSpace(e))] = true
	}
	ids := make([]string, 0, len(removeEmails))
	for _, a := range current {
		if want[strings.ToLower(firstNonEmpty(a.ThirdPartyEmail, a.DisplayName))] && a.AttendeeID != "" {
			ids = append(ids, a.AttendeeID)
		}
	}
	return ids
}

func mapEvent(account calendar.AccountProjection, item feishuEvent) calendar.Event {
	lifecycle := calendar.EventLifecycleStateActive
	if strings.EqualFold(item.Status, "cancelled") {
		lifecycle = calendar.EventLifecycleStateCancelled
	}
	now := time.Now().UTC()
	attendees := mapAttendees(item.Attendees)
	allDay := item.StartTime.Date != ""
	recurring := strings.TrimSpace(item.Recurrence) != ""
	ev := calendar.Event{
		ExternalEventID:         item.EventID,
		IntegrationID:           account.IntegrationID,
		CalendarAccountID:       account.CalendarAccountID,
		CalendarRef:             account.PrimaryCalendarRef,
		Title:                   item.Summary,
		Description:             item.Description,
		Location:                item.Location.Name,
		StartsAt:                parseFeishuTime(item.StartTime),
		EndsAt:                  parseFeishuTime(item.EndTime),
		Timezone:                firstNonEmpty(item.StartTime.Timezone, account.PrimaryTimezone),
		AllDay:                  allDay,
		StartDate:               item.StartTime.Date,
		EndDate:                 item.EndTime.Date,
		Recurring:               recurring,
		RecurrenceSummary:       item.Recurrence,
		RecurrenceRule:          item.Recurrence,
		Attendees:               attendeeEmailList(attendees),
		AttendeeDetails:         attendees,
		MutationEligibleInPhase: true,
		LifecycleState:          lifecycle,
		UpdatedAt:               now,
	}
	if recurring {
		ev.SeriesID = item.EventID // Feishu series identity is the root event id
	}
	return ev
}

// parseFeishuTime preserves the absolute instant: timestamps are unix seconds (UTC), so DST and
// timezone-boundary events keep correct absolute start/end when mapped onto the event resource.
func parseFeishuTime(t feishuTimeInfo) time.Time {
	if ts := strings.TrimSpace(t.Timestamp); ts != "" {
		if sec, err := strconv.ParseInt(ts, 10, 64); err == nil {
			return time.Unix(sec, 0).UTC()
		}
	}
	if d := strings.TrimSpace(t.Date); d != "" {
		if parsed, err := time.Parse("2006-01-02", d); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func normalizeTZ(resource integrations.Resource) string {
	// The Feishu primary-calendar object carries no timezone; per-event timezones are preserved
	// on each Event. Default the account timezone to UTC so absolute timing stays unambiguous.
	_ = resource
	return "UTC"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func decodePayload(raw json.RawMessage, out any) *providerFault {
	if len(raw) == 0 {
		return &providerFault{kind: faultInternal, code: "empty_payload", message: "operation payload missing"}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &providerFault{kind: faultInternal, code: "payload_decode_failed", message: "operation payload unreadable"}
	}
	return nil
}

func marshalResult[T any](value T, pf *providerFault) (json.RawMessage, *providerFault) {
	if pf != nil {
		return nil, pf
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, &providerFault{kind: faultInternal, code: "result_encode_failed", message: "result encode failed"}
	}
	return raw, nil
}
