package store

import (
	"context"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

func (s *SQLiteStore) EnsureBillingCatalog(ctx context.Context) error {
	if s == nil {
		return nil
	}
	now := time.Now().UTC()
	for _, definition := range billing.InitialDefinitions(now) {
		if err := s.SaveQuotaDefinition(ctx, definition); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) EnsureDevelopmentBillingPlan(ctx context.Context, tenantID string) error {
	if s == nil || tenantID == "" {
		return nil
	}
	if _, found, err := s.ActivePlan(ctx, tenantID); err != nil {
		return err
	} else if found {
		return nil
	}
	return s.SavePlan(ctx, billing.DevelopmentPlan(tenantID, time.Now().UTC()))
}

func (s *SQLiteStore) EnsureBillingDefaults(ctx context.Context, tenantID string) error {
	if err := s.EnsureBillingCatalog(ctx); err != nil {
		return err
	}
	return s.EnsureDevelopmentBillingPlan(ctx, tenantID)
}
