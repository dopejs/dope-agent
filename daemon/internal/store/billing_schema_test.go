package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

func TestSQLiteStoreBillingSchemaPersistsCoreRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	definition, ok := billing.DefinitionFor(billing.CategoryRunLaunches)
	if !ok {
		t.Fatal("expected run launch quota definition")
	}
	if err := store.SaveQuotaDefinition(ctx, definition); err != nil {
		t.Fatalf("SaveQuotaDefinition returned error: %v", err)
	}
	plan := billing.TenantPlan{
		PlanID:          "plan_test_finite",
		TenantID:        "ten_billing_schema",
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Hour),
		Document:        map[string]any{"source": "test"},
	}
	if err := store.SavePlan(ctx, plan); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	activePlan, found, err := store.ActivePlan(ctx, plan.TenantID)
	if err != nil {
		t.Fatalf("ActivePlan returned error: %v", err)
	}
	if !found || activePlan.PlanID != plan.PlanID {
		t.Fatalf("expected active plan %q, got found=%v plan=%q", plan.PlanID, found, activePlan.PlanID)
	}

	limit := int64(3)
	override := billing.QuotaOverride{
		QuotaOverrideID:      "override_test_run_launches",
		TenantID:             plan.TenantID,
		Category:             billing.CategoryRunLaunches,
		Limit:                &limit,
		EffectiveAt:          now.Add(-time.Minute),
		Reason:               "test lowered quota",
		CreatedByPrincipalID: "principal_admin",
	}
	if err := store.SaveQuotaOverride(ctx, override); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	activeOverride, err := store.QuotaOverride(ctx, plan.TenantID, billing.CategoryRunLaunches, now)
	if err != nil {
		t.Fatalf("QuotaOverride returned error: %v", err)
	}
	if activeOverride == nil || activeOverride.Limit == nil || *activeOverride.Limit != limit {
		t.Fatalf("expected active override limit %d, got %#v", limit, activeOverride)
	}

	period, err := store.OpenPeriod(ctx, plan.TenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter := billing.UsageCounter{
		UsageCounterID:  "counter_test_run_launches",
		TenantID:        plan.TenantID,
		Category:        billing.CategoryRunLaunches,
		QuotaPeriodID:   period.QuotaPeriodID,
		CommittedAmount: 1,
		ReservedAmount:  1,
		AdjustedAmount:  1,
		UpdatedAt:       now,
	}
	if err := store.SaveUsageCounter(ctx, counter); err != nil {
		t.Fatalf("SaveUsageCounter returned error: %v", err)
	}
	loadedCounter, found, err := store.UsageCounter(ctx, plan.TenantID, billing.CategoryRunLaunches, period.QuotaPeriodID)
	if err != nil {
		t.Fatalf("UsageCounter returned error: %v", err)
	}
	if !found || loadedCounter.CommittedAmount != 1 || loadedCounter.ReservedAmount != 1 || loadedCounter.AdjustedAmount != 1 {
		t.Fatalf("unexpected counter found=%v value=%#v", found, loadedCounter)
	}

	reservation := billing.UsageReservation{
		ReservationID:    "reservation_test_run_launch",
		TenantID:         plan.TenantID,
		Category:         billing.CategoryRunLaunches,
		QuotaPeriodID:    period.QuotaPeriodID,
		OperationKey:     "tenant:ten_billing_schema:run:client_key",
		AmountReserved:   1,
		Status:           billing.ReservationStatusReserved,
		ReservationPoint: "test reserve",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := store.SaveReservation(ctx, reservation); err != nil {
		t.Fatalf("SaveReservation returned error: %v", err)
	}
	loadedReservation, found, err := store.ReservationByOperation(ctx, plan.TenantID, billing.CategoryRunLaunches, reservation.OperationKey)
	if err != nil {
		t.Fatalf("ReservationByOperation returned error: %v", err)
	}
	if !found || loadedReservation.ReservationID != reservation.ReservationID {
		t.Fatalf("expected reservation %q, got found=%v value=%#v", reservation.ReservationID, found, loadedReservation)
	}
	pending, err := store.ListPendingReservations(ctx)
	if err != nil {
		t.Fatalf("ListPendingReservations returned error: %v", err)
	}
	if len(pending) != 1 || pending[0].ReservationID != reservation.ReservationID {
		t.Fatalf("expected one pending reservation, got %#v", pending)
	}

	if err := store.AppendUsageEvent(ctx, billing.UsageEvent{
		UsageEventID:  "usage_event_test_reserved",
		TenantID:      plan.TenantID,
		Category:      billing.CategoryRunLaunches,
		QuotaPeriodID: period.QuotaPeriodID,
		OperationKey:  reservation.OperationKey,
		EventKind:     billing.UsageEventReservation,
		Amount:        1,
		ReasonCode:    "usage_reserved",
		Outcome:       "reserved",
		CreatedAt:     now,
		Document:      map[string]any{"source": "test"},
	}); err != nil {
		t.Fatalf("AppendUsageEvent returned error: %v", err)
	}
	if err := store.AppendQuotaDenial(ctx, billing.QuotaDenial{
		DenialID:          "denial_test_run_launch",
		TenantID:          plan.TenantID,
		Category:          billing.CategoryRunLaunches,
		QuotaPeriodID:     period.QuotaPeriodID,
		OperationKey:      "tenant:ten_billing_schema:run:denied",
		ReasonCode:        "quota_denied:run_launches_exhausted",
		RequestedAmount:   1,
		RemainingAmount:   0,
		GuardedEntryPoint: "POST /v1/runs",
		CreatedAt:         now,
	}); err != nil {
		t.Fatalf("AppendQuotaDenial returned error: %v", err)
	}
	adjustment := billing.ManualAdjustment{
		AdjustmentID:         "adjustment_test_run_launches",
		TenantID:             plan.TenantID,
		Category:             billing.CategoryRunLaunches,
		QuotaPeriodID:        period.QuotaPeriodID,
		AmountDelta:          -1,
		Reason:               "test correction",
		CreatedByPrincipalID: "principal_admin",
		CreatedAt:            now,
	}
	if err := store.SaveManualAdjustment(ctx, adjustment); err != nil {
		t.Fatalf("SaveManualAdjustment returned error: %v", err)
	}
}
