package livevalidation

import (
	"context"

	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func (m *Manager) DefaultRetentionPolicy(ctx context.Context) RetentionPolicy {
	now := m.clock()
	tenantID := ""
	createdBy := ""
	if tenantContext, ok := tenantctx.FromContext(ctx); ok {
		tenantID = tenantContext.TenantID
		createdBy = tenantContext.PrincipalID
	}
	return RetentionPolicy{
		PolicyID:             "live_validation_retention_default",
		TenantID:             tenantID,
		AppliesTo:            RetentionAppliesAll,
		Mode:                 RetentionModeIndefinite,
		CreatedByPrincipalID: createdBy,
		CreatedAt:            now,
	}
}

func (m *Manager) SaveRetentionPolicy(ctx context.Context, item RetentionPolicy) (RetentionPolicy, error) {
	if item.PolicyID == "" {
		item.PolicyID = newID("lv_retention")
	}
	if item.Mode == "" {
		item.Mode = RetentionModeIndefinite
	}
	if item.AppliesTo == "" {
		item.AppliesTo = RetentionAppliesAll
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = m.clock()
	}
	if m.store != nil {
		if err := m.store.SaveLiveValidationRetentionPolicy(ctx, item); err != nil {
			return RetentionPolicy{}, err
		}
	}
	return item, nil
}
