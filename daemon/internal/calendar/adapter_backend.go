package calendar

import (
	"context"
	"errors"
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
	client       *adapterrpc.Client
	deadline     time.Duration
	providerKind string
}

// NewAdapterBackend builds a calendar adapter backend over the given RPC client. The deadline
// bounds each operation (spec clarification Q1 / FR-007b); zero uses the client default.
func NewAdapterBackend(client *adapterrpc.Client, deadline time.Duration) *AdapterBackend {
	return &AdapterBackend{client: client, deadline: deadline}
}

// WithProviderKind records the integration diagnostics provider kind served by this adapter
// (e.g. "feishu_lark", Roadmap 60). It is surfaced on operation-failure classification so
// OAuth/scope/token failures land on the existing provider diagnostics reason vocabulary
// (FR-006). The empty default keeps the generic (provider-agnostic) classification path.
func (b *AdapterBackend) WithProviderKind(kind string) *AdapterBackend {
	b.providerKind = kind
	return b
}

// ProviderKind reports the diagnostics provider kind this adapter serves (may be empty).
func (b *AdapterBackend) ProviderKind() string { return b.providerKind }

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
	return out, b.mapErr(err)
}

func (b *AdapterBackend) ListEvents(resource integrations.Resource, account AccountProjection, input ListEventsInput) ([]Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out []Event
	err := b.client.Dispatch(ctx, domainCalendar, "ListEvents", resource, map[string]any{"account": account, "input": input}, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) GetEvent(resource integrations.Resource, account AccountProjection, eventID string) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "GetEvent", resource, map[string]any{"account": account, "eventId": eventID}, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) BusyFree(resource integrations.Resource, account AccountProjection, input BusyFreeInput) (AvailabilityQuery, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out AvailabilityQuery
	err := b.client.Dispatch(ctx, domainCalendar, "BusyFree", resource, map[string]any{"account": account, "input": input}, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) CreateEvent(resource integrations.Resource, account AccountProjection, input CreateEventInput) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "CreateEvent", resource, map[string]any{"account": account, "input": input}, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) UpdateEvent(resource integrations.Resource, account AccountProjection, input UpdateEventInput) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "UpdateEvent", resource, map[string]any{"account": account, "input": input}, &out)
	return out, b.mapErr(err)
}

func (b *AdapterBackend) CancelEvent(resource integrations.Resource, account AccountProjection, input CancelEventInput) (Event, error) {
	ctx, cancel := b.op()
	defer cancel()
	var out Event
	err := b.client.Dispatch(ctx, domainCalendar, "CancelEvent", resource, map[string]any{"account": account, "input": input}, &out)
	return out, b.mapErr(err)
}

// RestoreIntegrationState is a no-op: the adapter holds no durable state; restore is daemon-owned.
func (b *AdapterBackend) RestoreIntegrationState(integrationID string, events []Event) {}

// AdapterFailure wraps an out-of-process adapter failure with the stable, redacted failure
// class and provider kind the calendar Manager records on the single operation ledger and
// feeds into integration diagnostics classification (FR-006, FR-008). Detail carries only the
// stable, non-secret token the adapter returned; no credential or token material is included.
type AdapterFailure struct {
	class        string // stable failure-class token (e.g. token_expired, scope_not_granted, ambiguous_commit)
	providerKind string // diagnostics provider kind (e.g. feishu_lark); may be empty
	detail       string // redacted, non-secret detail
	ambiguous    bool   // unconfirmed write outcome (FR-007a)
	unavailable  bool   // adapter/provider unavailable
}

func (e *AdapterFailure) Error() string {
	if e.detail != "" {
		return e.detail
	}
	return e.class
}

// Is lets existing callers keep matching ErrCalendarUnavailable for unavailable outcomes so
// the API surface and readiness semantics are preserved.
func (e *AdapterFailure) Is(target error) bool {
	return e.unavailable && target == ErrCalendarUnavailable
}

// FailureClass returns the stable failure-class token for the operation ledger.
func (e *AdapterFailure) FailureClass() string { return e.class }

// mapErr translates a transport/adapter error into an *AdapterFailure carrying the stable
// failure class and provider kind. Unconfirmed outcomes (deadline expiry, transport break,
// undecodable response) are classified as ambiguous-commit (FR-007a) rather than assumed
// committed or failed. A clean StatusFailure response is a confirmed non-commit.
func (b *AdapterBackend) mapErr(err error) error {
	if err == nil {
		return nil
	}
	if adapterrpc.IsAmbiguous(err) {
		return &AdapterFailure{class: "ambiguous_commit", providerKind: b.providerKind, detail: err.Error(), ambiguous: true}
	}
	var ae *adapterrpc.AdapterError
	if errors.As(err, &ae) {
		return &AdapterFailure{
			class:        stableFailureClass(ae),
			providerKind: b.providerKind,
			detail:       ae.Detail,
			unavailable:  ae.Kind == adapterrpc.FailureUnavailable,
		}
	}
	return err
}

// stableFailureClass derives a stable, redacted failure-class token from a typed adapter
// failure. It prefers the adapter's own redacted detail token (already shaped to the stable
// provider vocabulary, e.g. "scope_not_granted") and otherwise falls back to the failure kind.
func stableFailureClass(ae *adapterrpc.AdapterError) string {
	if token := ae.Detail; token != "" {
		return token
	}
	switch ae.Kind {
	case adapterrpc.FailureAuth:
		return "user_access_token_invalid"
	case adapterrpc.FailureScope:
		return "scope_not_granted"
	case adapterrpc.FailureRateLimited:
		return "rate_limited"
	case adapterrpc.FailureUnavailable:
		return "service_unavailable"
	case adapterrpc.FailureMalformed:
		return "malformed_provider_response"
	default:
		return "provider_internal_error"
	}
}

// failureClassAndProvider extracts the stable failure class and diagnostics provider kind from
// a backend error so the operation ledger and DiagnosticFailure carry stable reasons (FR-006).
// For non-adapter errors it returns the supplied default class and no provider kind.
func failureClassAndProvider(defaultClass string, err error) (class, providerKind string) {
	var af *AdapterFailure
	if errors.As(err, &af) {
		return af.FailureClass(), af.providerKind
	}
	if adapterrpc.IsAmbiguous(err) {
		return "ambiguous_commit", ""
	}
	return defaultClass, ""
}
