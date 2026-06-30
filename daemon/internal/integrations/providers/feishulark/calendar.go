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
	Recurrence string `json:"recurrence"`
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
	body := writeEventBody(input.Title, input.Description, input.Location, input.StartsAt, input.EndsAt, firstNonEmpty(input.Timezone, account.PrimaryTimezone))
	path := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events", url.PathEscape(account.PrimaryCalendarRef))
	var out struct {
		Event feishuEvent `json:"event"`
	}
	if pf := p.client.call(ctx, "POST", path, token.AccessToken, body, &out, true); pf != nil {
		return calendar.Event{}, pf
	}
	return mapEvent(account, out.Event), nil
}

func (p *CalendarProvider) updateEvent(ctx context.Context, token scopedToken, account calendar.AccountProjection, input calendar.UpdateEventInput) (calendar.Event, *providerFault) {
	body := writeEventBody(input.Title, input.Description, input.Location, input.StartsAt, input.EndsAt, firstNonEmpty(input.Timezone, account.PrimaryTimezone))
	path := fmt.Sprintf("/open-apis/calendar/v4/calendars/%s/events/%s", url.PathEscape(account.PrimaryCalendarRef), url.PathEscape(input.ExternalEventID))
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

func writeEventBody(title, description, location string, startsAt, endsAt time.Time, tz string) map[string]any {
	body := map[string]any{
		"summary":     title,
		"description": description,
		"start_time":  map[string]any{"timestamp": strconv.FormatInt(startsAt.Unix(), 10), "timezone": tz},
		"end_time":    map[string]any{"timestamp": strconv.FormatInt(endsAt.Unix(), 10), "timezone": tz},
	}
	if strings.TrimSpace(location) != "" {
		body["location"] = map[string]any{"name": location}
	}
	return body
}

func mapEvent(account calendar.AccountProjection, item feishuEvent) calendar.Event {
	lifecycle := calendar.EventLifecycleStateActive
	if strings.EqualFold(item.Status, "cancelled") {
		lifecycle = calendar.EventLifecycleStateCancelled
	}
	now := time.Now().UTC()
	return calendar.Event{
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
		AllDay:                  item.StartTime.Date != "",
		Recurring:               strings.TrimSpace(item.Recurrence) != "",
		RecurrenceSummary:       item.Recurrence,
		MutationEligibleInPhase: item.StartTime.Date == "" && strings.TrimSpace(item.Recurrence) == "",
		LifecycleState:          lifecycle,
		UpdatedAt:               now,
	}
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
