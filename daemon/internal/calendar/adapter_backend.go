package calendar

import (
	"context"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// domainCalendar is the RPC domain key for calendar operations.
const domainCalendar = "calendar"

// AdapterBackend must satisfy the calendar Backend interface.
var _ Backend = (*AdapterBackend)(nil)

// AdapterBackend implements calendar.Backend by dispatching each operation to an
// out-of-process integration adapter over the capability RPC contract (Roadmap 59). It does
// provider request/response mapping only; the calendar Manager retains the operation ledger,
// idempotency, artifacts, and live-validation classification. The adapter is stateless, so
// RestoreIntegrationState is a no-op here.
type AdapterBackend struct {
	client   *adapterrpc.Client
	deadline time.Duration
}

// NewAdapterBackend builds a calendar adapter backend over the given RPC client. The deadline
// bounds each operation (spec clarification Q1 / FR-007b); zero uses the client default.
func NewAdapterBackend(client *adapterrpc.Client, deadline time.Duration) *AdapterBackend {
	return &AdapterBackend{client: client, deadline: deadline}
}

func (b *AdapterBackend) op() (context.Context, context.CancelFunc) {
	if b.deadline > 0 {
		return context.WithTimeout(context.Background(), b.deadline)
	}
	return context.WithCancel(context.Background())
}

func (b *AdapterBackend) ProjectAccount(resource integrations.Resource) (AccountProjection, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out AccountProjection
	err := b.client.Dispatch(ctx, domainCalendar, "ProjectAccount", resource, nil, &out)
	return out, mapAdapterError(err)
}

func (b *AdapterBackend) ListEvents(resource integrations.Resource, account AccountProjection, input ListEventsInput) ([]Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out []Event
	err := b.client.Dispatch(ctx, domainCalendar, "ListEvents", resource, map[string]any{"account": account, "input": input}, &out)
	return out, mapAdapterError(err)
}

func (b *AdapterBackend) GetEvent(resource integrations.Resource, account AccountProjection, eventID string) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "GetEvent", resource, map[string]any{"account": account, "eventId": eventID}, &out)
	return out, mapAdapterError(err)
}

func (b *AdapterBackend) BusyFree(resource integrations.Resource, account AccountProjection, input BusyFreeInput) (AvailabilityQuery, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out AvailabilityQuery
	err := b.client.Dispatch(ctx, domainCalendar, "BusyFree", resource, map[string]any{"account": account, "input": input}, &out)
	return out, mapAdapterError(err)
}

func (b *AdapterBackend) CreateEvent(resource integrations.Resource, account AccountProjection, input CreateEventInput) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "CreateEvent", resource, map[string]any{"account": account, "input": input}, &out)
	return out, mapAdapterError(err)
}

func (b *AdapterBackend) UpdateEvent(resource integrations.Resource, account AccountProjection, input UpdateEventInput) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "UpdateEvent", resource, map[string]any{"account": account, "input": input}, &out)
	return out, mapAdapterError(err)
}

func (b *AdapterBackend) CancelEvent(resource integrations.Resource, account AccountProjection, input CancelEventInput) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "CancelEvent", resource, map[string]any{"account": account, "input": input}, &out)
	return out, mapAdapterError(err)
}

// RestoreIntegrationState is a no-op: the adapter holds no durable state; restore is daemon-owned.
func (b *AdapterBackend) RestoreIntegrationState(integrationID string, events []Event) {}

// writeFailureReason classifies a side-effecting write failure. Unconfirmed outcomes
// (deadline expiry, transport break, undecodable response) are recorded as ambiguous-commit
// in the single operation ledger (FR-007a) rather than assumed committed or failed.
func writeFailureReason(defaultReason string, err error) string {
	if adapterrpc.IsAmbiguous(err) {
		return "ambiguous_commit"
	}
	return defaultReason
}

// mapAdapterError translates an unavailable adapter into the calendar-domain unavailable
// error; other adapter and transport errors are returned as-is for the manager to classify.
func mapAdapterError(err error) error {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*adapterrpc.AdapterError); ok && ae.Kind == adapterrpc.FailureUnavailable {
		return ErrCalendarUnavailable
	}
	return err
}
