package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestLiveValidationPreflightGateAllowedDeniedAndRetry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_r38_live_validation"
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{PlanID: "plan_" + tenantID, TenantID: tenantID, PlanKey: "finite", Status: billing.PlanStatusActive, EnforcementMode: billing.EnforcementModeEnforced, EffectiveAt: now.Add(-time.Minute)}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(1)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{QuotaOverrideID: "override_live_validation", TenantID: tenantID, Category: billing.CategoryLiveValidationAttempts, Limit: &limit, EffectiveAt: now.Add(-time.Minute), Reason: "test limit"}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	manager := billing.NewManager(sqliteStore)

	first, err := reserveLiveValidationPreflight(ctx, manager, tenantID, "validation_1", "", true)
	if err != nil || !first.Allowed || first.Reservation.ReservationID == "" {
		t.Fatalf("expected first live validation preflight allowed, err=%v result=%+v", err, first)
	}
	retry, err := reserveLiveValidationPreflight(ctx, manager, tenantID, "validation_1", "", true)
	if err != nil || !retry.Allowed || retry.Reservation.ReservationID != first.Reservation.ReservationID {
		t.Fatalf("expected retry to reuse reservation, err=%v result=%+v", err, retry)
	}
	denied, err := reserveLiveValidationPreflight(ctx, manager, tenantID, "validation_2", "", true)
	if !errors.Is(err, billing.ErrQuotaDenied) || denied.Denial == nil || denied.Denial.ReasonCode != "quota_denied:live_validation_attempts_exhausted" {
		t.Fatalf("expected live validation quota denial, err=%v result=%+v", err, denied)
	}
}

func TestLiveValidationPreflightGateFailsClosedWhenHostedStateUnavailable(t *testing.T) {
	t.Parallel()

	result, err := reserveLiveValidationPreflight(context.Background(), nil, "ten_r38_live_validation_unavailable", "validation_1", "", true)
	if !errors.Is(err, billing.ErrQuotaStateUnavailable) || result.Allowed || result.Denial == nil {
		t.Fatalf("expected hosted fail-closed result, err=%v result=%+v", err, result)
	}
}

func TestLiveValidationPreflightGateAllowsLocalWithoutExecutor(t *testing.T) {
	t.Parallel()

	result, err := reserveLiveValidationPreflight(context.Background(), nil, "ten_r38_local", "validation_1", "", false)
	if err != nil || !result.Allowed || result.Reservation.ReservationID != "" {
		t.Fatalf("expected local preflight allow without mounted executor or reservation, err=%v result=%+v", err, result)
	}
}
