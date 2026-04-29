package livevalidation

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

var ErrReconciliationPermissionDenied = errors.New("live validation reconciliation permission denied")

func (m *Manager) ResolveReconciliation(ctx context.Context, item ReconciliationResolution) (ReconciliationResolution, error) {
	tenantContext, ok := tenantctx.FromContext(ctx)
	if !ok || !identity.CanResolveLiveValidationReconciliation(tenantContext) {
		return ReconciliationResolution{}, ErrReconciliationPermissionDenied
	}
	now := m.clock()
	if item.ReconciliationID == "" {
		item.ReconciliationID = newID("lv_reconcile")
	}
	if item.TenantID == "" {
		item.TenantID = tenantContext.TenantID
	}
	if item.ResolvedBy == "" {
		item.ResolvedBy = tenantContext.PrincipalID
	}
	if item.ResolvedAt.IsZero() {
		item.ResolvedAt = now
	}
	if m.store != nil {
		if err := m.store.SaveLiveValidationReconciliationResolution(ctx, item); err != nil {
			return ReconciliationResolution{}, err
		}
	}
	return item, nil
}
