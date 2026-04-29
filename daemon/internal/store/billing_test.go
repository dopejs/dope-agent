package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

func TestSQLiteStoreBillingUsageSummaryProjectsCounterAmounts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	tenantID := "ten_billing_projection"
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_projection",
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     time.Now().UTC().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	definition, ok := billing.DefinitionFor(billing.CategoryRunLaunches)
	if !ok {
		t.Fatal("expected run launch definition")
	}
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	if err := sqliteStore.SaveUsageCounter(ctx, billing.UsageCounter{
		UsageCounterID:  "counter_projection",
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		QuotaPeriodID:   period.QuotaPeriodID,
		CommittedAmount: 5,
		ReservedAmount:  2,
		AdjustedAmount:  -1,
		CarryoverAmount: 3,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("SaveUsageCounter returned error: %v", err)
	}
	limit := int64(10)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_projection",
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test projection",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}

	manager := billing.NewManagerWithClock(sqliteStore, func() time.Time { return now })
	summary, err := manager.UsageSummary(ctx, tenantID, true)
	if err != nil {
		t.Fatalf("UsageSummary returned error: %v", err)
	}
	var quota *billing.EffectiveQuota
	for index := range summary.Quotas {
		if summary.Quotas[index].Category == billing.CategoryRunLaunches {
			quota = &summary.Quotas[index]
			break
		}
	}
	if quota == nil {
		t.Fatal("expected run launch quota in usage summary")
	}
	if quota.ConsumedAmount != 5 || quota.ReservedAmount != 2 || quota.AdjustedAmount != -1 || quota.CarryoverApplied != 3 || quota.RemainingAmount != 7 {
		t.Fatalf("unexpected projected quota: %+v", *quota)
	}
}

func TestSQLiteStoreReserveUsageSerializesConcurrentLastUnit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := NewSQLiteStore(t.TempDir())
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
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	tenantID := "ten_billing_concurrent_store"
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_concurrent_store",
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	limit := int64(1)
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_concurrent_store",
		TenantID:        tenantID,
		Category:        billing.CategoryRunLaunches,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test last unit",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, operationKey := range []string{"tenant:" + tenantID + ":run:one", "tenant:" + tenantID + ":run:two"} {
		wg.Add(1)
		go func(operationKey string) {
			defer wg.Done()
			<-start
			_, err := billing.NewManagerWithClock(sqliteStore, func() time.Time { return now }).Reserve(ctx, billing.ReserveInput{
				TenantID:          tenantID,
				Category:          billing.CategoryRunLaunches,
				Amount:            1,
				OperationKey:      operationKey,
				ReservationPoint:  "test concurrent reserve",
				GuardedEntryPoint: "test concurrent reserve",
			})
			results <- err
		}(operationKey)
	}
	close(start)
	wg.Wait()
	close(results)

	var allowed, denied int
	for err := range results {
		switch {
		case err == nil:
			allowed++
		case errors.Is(err, billing.ErrQuotaDenied):
			denied++
		default:
			t.Fatalf("unexpected reserve error: %v", err)
		}
	}
	if allowed != 1 || denied != 1 {
		t.Fatalf("expected one allowed and one denied reservation, got allowed=%d denied=%d", allowed, denied)
	}
	definition, ok := billing.DefinitionFor(billing.CategoryRunLaunches)
	if !ok {
		t.Fatal("expected run launch definition")
	}
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter, ok, err := sqliteStore.UsageCounter(ctx, tenantID, billing.CategoryRunLaunches, period.QuotaPeriodID)
	if err != nil {
		t.Fatalf("UsageCounter returned error: %v", err)
	}
	if !ok || counter.ReservedAmount != 1 || counter.CommittedAmount != 0 {
		t.Fatalf("expected exactly one reserved last unit, ok=%v counter=%+v", ok, counter)
	}
}

func TestSQLiteStoreBillingReservationOperationKeyUniqueness(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	now := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	tenantID := "ten_billing_unique_operation"
	definition, ok := billing.DefinitionFor(billing.CategoryRunLaunches)
	if !ok {
		t.Fatal("expected run launch definition")
	}
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	operationKey := "tenant:" + tenantID + ":run:client_1"
	first := billing.UsageReservation{
		ReservationID:  "reservation_first",
		TenantID:       tenantID,
		Category:       billing.CategoryRunLaunches,
		QuotaPeriodID:  period.QuotaPeriodID,
		OperationKey:   operationKey,
		AmountReserved: 1,
		Status:         billing.ReservationStatusReserved,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := sqliteStore.SaveReservation(ctx, first); err != nil {
		t.Fatalf("SaveReservation(first) returned error: %v", err)
	}
	second := first
	second.ReservationID = "reservation_second"
	second.AmountCommitted = 1
	second.Status = billing.ReservationStatusCommitted
	second.UpdatedAt = now.Add(time.Minute)
	if err := sqliteStore.SaveReservation(ctx, second); err != nil {
		t.Fatalf("SaveReservation(second) returned error: %v", err)
	}
	loaded, found, err := sqliteStore.ReservationByOperation(ctx, tenantID, billing.CategoryRunLaunches, operationKey)
	if err != nil {
		t.Fatalf("ReservationByOperation returned error: %v", err)
	}
	if !found || loaded.ReservationID != first.ReservationID || loaded.Status != billing.ReservationStatusCommitted || loaded.AmountCommitted != 1 {
		t.Fatalf("expected operation-key upsert to preserve original reservation id and update lifecycle, found=%v loaded=%#v", found, loaded)
	}
	pending, err := sqliteStore.ListPendingReservations(ctx)
	if err != nil {
		t.Fatalf("ListPendingReservations returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected committed upsert to remove pending reservation, got %#v", pending)
	}
}

func TestSQLiteStoreResolveUsageCommitsCounterReservationAndEventInOneTransaction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := NewSQLiteStore(t.TempDir())
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
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	tenantID := "ten_billing_resolve_tx"
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_resolve_tx",
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	operationKey := "tenant:" + tenantID + ":run:resolve_tx"
	reserved, err := sqliteStore.ReserveUsage(ctx, billing.ReserveInput{
		TenantID:          tenantID,
		Category:          billing.CategoryRunLaunches,
		Amount:            1,
		OperationKey:      operationKey,
		ReservationPoint:  "test before runtime.CreateRun",
		GuardedEntryPoint: "test",
		Hosted:            true,
	}, now)
	if err != nil || !reserved.Allowed {
		t.Fatalf("ReserveUsage err=%v result=%+v", err, reserved)
	}

	reservation, err := sqliteStore.ResolveUsage(ctx, billing.ResolveInput{
		TenantID:     tenantID,
		Category:     billing.CategoryRunLaunches,
		OperationKey: operationKey,
		ReasonCode:   "billing.test_committed",
		Reason:       "test commit",
	}, billing.ReservationStatusCommitted, billing.UsageEventCommit, now.Add(time.Second))
	if err != nil {
		t.Fatalf("ResolveUsage returned error: %v", err)
	}
	if reservation.Status != billing.ReservationStatusCommitted || reservation.AmountCommitted != 1 {
		t.Fatalf("unexpected resolved reservation: %+v", reservation)
	}
	definition, _ := billing.DefinitionFor(billing.CategoryRunLaunches)
	period, err := sqliteStore.OpenPeriod(ctx, tenantID, definition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter, ok, err := sqliteStore.UsageCounter(ctx, tenantID, billing.CategoryRunLaunches, period.QuotaPeriodID)
	if err != nil || !ok {
		t.Fatalf("UsageCounter err=%v ok=%v", err, ok)
	}
	if counter.ReservedAmount != 0 || counter.CommittedAmount != 1 {
		t.Fatalf("expected committed counter with no reservation, got %+v", counter)
	}
	var eventCount int
	if err := sqliteStore.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM billing_usage_events
		WHERE tenant_id = ? AND category = ? AND operation_key = ? AND event_kind = ?
	`, tenantID, string(billing.CategoryRunLaunches), operationKey, string(billing.UsageEventCommit)).Scan(&eventCount); err != nil {
		t.Fatalf("count usage events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one commit usage event, got %d", eventCount)
	}
}

func TestSQLiteStoreReserveAllUsageDeniesAtomicallyWithoutPriorCategoryReservation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := NewSQLiteStore(t.TempDir())
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
	now := time.Date(2026, 4, 29, 12, 30, 0, 0, time.UTC)
	tenantID := "ten_billing_reserve_all_tx"
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_reserve_all_tx",
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	toolDefinition, _ := billing.DefinitionFor(billing.CategoryRuntimeToolCalls)
	toolPeriod, err := sqliteStore.OpenPeriod(ctx, tenantID, toolDefinition, now)
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	if err := sqliteStore.SaveUsageCounter(ctx, billing.UsageCounter{
		UsageCounterID:  "counter_reserve_all_tool_exhausted",
		TenantID:        tenantID,
		Category:        billing.CategoryRuntimeToolCalls,
		QuotaPeriodID:   toolPeriod.QuotaPeriodID,
		CommittedAmount: toolDefinition.DefaultLimit,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("SaveUsageCounter returned error: %v", err)
	}

	integrationOperationKey := billing.IntegrationOperationKey(tenantID, "mail", "op_atomic", "")
	result, err := sqliteStore.ReserveAllUsage(ctx, []billing.ReserveInput{
		{
			TenantID:          tenantID,
			Category:          billing.CategoryIntegrationOperations,
			Amount:            1,
			OperationKey:      integrationOperationKey,
			ReservationPoint:  "integration preflight",
			GuardedEntryPoint: "mail operation",
			Hosted:            true,
		},
		{
			TenantID:          tenantID,
			Category:          billing.CategoryRuntimeToolCalls,
			Amount:            1,
			OperationKey:      billing.ToolCallOperationKey(tenantID, "run_atomic", "step_atomic", "tool_atomic", ""),
			ReservationPoint:  "tool call creation",
			GuardedEntryPoint: "tool call",
			Hosted:            true,
		},
	}, now)
	if !errors.Is(err, billing.ErrQuotaDenied) || result.Allowed || result.Denial == nil {
		t.Fatalf("expected atomic multi-category denial, err=%v result=%+v", err, result)
	}
	if reservation, ok, err := sqliteStore.ReservationByOperation(ctx, tenantID, billing.CategoryIntegrationOperations, integrationOperationKey); err != nil || ok {
		t.Fatalf("expected no prior-category reservation, err=%v ok=%v reservation=%+v", err, ok, reservation)
	}
	integrationDefinition, _ := billing.DefinitionFor(billing.CategoryIntegrationOperations)
	integrationPeriod, err := sqliteStore.OpenPeriod(ctx, tenantID, integrationDefinition, now)
	if err != nil {
		t.Fatalf("OpenPeriod(integration) returned error: %v", err)
	}
	counter, ok, err := sqliteStore.UsageCounter(ctx, tenantID, billing.CategoryIntegrationOperations, integrationPeriod.QuotaPeriodID)
	if err != nil {
		t.Fatalf("UsageCounter(integration) returned error: %v", err)
	}
	if ok && counter.ReservedAmount != 0 {
		t.Fatalf("expected no reserved amount for prior category, got %+v", counter)
	}
}
