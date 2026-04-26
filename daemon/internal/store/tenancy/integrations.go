package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Integrations is the tenant-aware accessor for the integrations table.
//
// Roadmap 37 boundary (binding constraint, enforced by T089c): this
// helper exposes ONLY ownership wiring and query isolation. It MUST NOT
// add, remove, or alter any function that participates in credential
// storage, OAuth state, secret-reference resolution, or readiness
// probing. Those surfaces remain owned by Roadmap 37.
type Integrations struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewIntegrations(s *store.SQLiteStore, emitter *audit.Emitter) *Integrations {
	return &Integrations{store: s, emitter: emitter}
}

func (a *Integrations) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

// UpsertIntegrationForTenant persists an integration row and binds its
// tenant_id. Pass A only wires ownership; readiness/credential probing
// remains owned by Roadmap 37 (untouched here).
func (a *Integrations) UpsertIntegrationForTenant(ctx context.Context, item integrations.Resource) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertIntegration(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "integrations", "integration_id", item.IntegrationID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertIntegrationForTenant", "integration")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
