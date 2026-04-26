// Package audit emits operator-visible audit events for tenant-related
// security failures. The cross-tenant access denial event introduced by
// Roadmap 35 is published on the existing in-process event bus and logged
// via a structured stable log code so operators can grep failures.
package audit

import (
	"context"
	"errors"
	"log/slog"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

// LogCode is the stable structured log code emitted alongside the event.
const LogCode = "audit.cross_tenant_access_denied"

// EventCategory and EventName are the runtime-event envelope identifiers for
// the new R35 audit event. They must stay in sync with
// schemas/events/audit-cross-tenant-access-denied.event.schema.json.
const (
	EventCategory = "audit"
	EventName     = "audit.cross_tenant_access_denied"
)

// ErrNoActingTenant is returned by Emit when the caller's context carries no
// resolved tenant. This should never happen in practice — callers reach Emit
// only after RequireTenant succeeded — but is reported defensively rather
// than emitting a malformed event.
var ErrNoActingTenant = errors.New("audit: cannot emit tenant breach without acting tenant context")

// Emitter publishes the audit event on the bus and writes the structured
// log line. It carries no per-call state; instances are cheap to allocate.
type Emitter struct {
	bus *events.Bus
	log *slog.Logger
}

// NewEmitter constructs an Emitter wired to the daemon's shared event bus
// and logger.
func NewEmitter(bus *events.Bus, log *slog.Logger) *Emitter {
	if log == nil {
		log = slog.Default()
	}
	return &Emitter{bus: bus, log: log}
}

// Emit publishes the audit event for a denied cross-tenant access. The
// caller's tenant context is read from ctx; the surface (e.g. "api:GET
// /v1/runs/{id}", "store:ListSchedulesForTenant") and the resourceKind
// (e.g. "run", "schedule") are passed in. Per Roadmap 35's Q3 decision the
// payload MUST NOT include the target tenant id, the target resource id,
// or any row data; the function signature enforces this by only accepting
// the two opaque strings.
func (e *Emitter) Emit(ctx context.Context, surface, resourceKind string) error {
	tc, ok := tenantctx.FromContext(ctx)
	if !ok || tc.TenantID == "" {
		return ErrNoActingTenant
	}
	if e == nil {
		return nil
	}

	payload := map[string]any{
		"actingTenantId": tc.TenantID,
		"principalId":    tc.PrincipalID,
		"surface":        surface,
		"resourceKind":   resourceKind,
	}
	if e.bus != nil {
		e.bus.Publish(events.Event{
			TenantID: tc.TenantID,
			Category: EventCategory,
			Name:     EventName,
			Resource: events.Resource{Kind: "tenant", ID: tc.TenantID},
			Payload:  payload,
		})
	}
	e.log.LogAttrs(ctx, slog.LevelWarn, LogCode,
		slog.String("acting_tenant_id", tc.TenantID),
		slog.String("principal_id", tc.PrincipalID),
		slog.String("surface", surface),
		slog.String("resource_kind", resourceKind),
	)
	return nil
}
