package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Events is the tenant-aware accessor for the mixed events table.
//
// Usage rules:
//   - For tenant-owned categories (every category not in
//     globalEventCategories), call AppendEventForTenant — it requires a
//     resolved tenant context and binds tenant_id on the persisted row.
//   - For global categories (mcp, provider, system, daemon.migration,
//     connector_global, capability_global), the legacy
//     store.AppendEvent path is preserved — it writes tenant_id NULL.
//
// AppendEventForTenant returns ErrTenantContextRequired when the
// context lacks a tenant; this is fail-closed by design.
type Events struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewEvents(s *store.SQLiteStore, emitter *audit.Emitter) *Events {
	return &Events{store: s, emitter: emitter}
}

func (a *Events) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

// IsGlobalCategory reports whether the given event category is global
// (NULL tenant_id allowed). Delegates to events.IsGlobalCategory so the
// global set has a single source of truth in the events package.
func IsGlobalCategory(category string) bool {
	return events.IsGlobalCategory(category)
}

// AppendEventForTenant persists a tenant-owned event with tenant_id
// pre-bound. Returns the persisted event including the assigned
// sequence and the bound tenantId.
func (a *Events) AppendEventForTenant(ctx context.Context, event events.Event) (events.Event, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return events.Event{}, err
	}
	if IsGlobalCategory(event.Category) {
		return events.Event{}, errors.New("tenancy: refused to bind tenant on a global event category — use store.AppendEvent")
	}
	event.TenantID = tenantID
	return a.store.AppendEventForTenantRaw(ctx, event, tenantID)
}

// ListEventsForTenant returns persisted events whose tenant_id matches
// the caller. Global rows (tenant_id NULL) are NOT returned.
func (a *Events) ListEventsForTenant(ctx context.Context, filter events.Filter) ([]events.Event, error) {
	tenantID, err := Require(ctx)
	if err != nil {
		return nil, err
	}
	return a.store.ListEventsForTenantRaw(ctx, tenantID, filter)
}
