package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestApplyManualAdjustmentValidation(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	definition, _ := DefinitionFor(CategoryRunLaunches)
	period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{
		UsageCounterID:  "counter_adjustment",
		TenantID:        fixtureTenantA,
		Category:        CategoryRunLaunches,
		QuotaPeriodID:   period.QuotaPeriodID,
		CommittedAmount: 2,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatal(err)
	}

	err = manager.ApplyManualAdjustment(ctx, ManualAdjustment{
		AdjustmentID:  "adjustment_without_reason",
		TenantID:      fixtureTenantA,
		Category:      CategoryRunLaunches,
		QuotaPeriodID: period.QuotaPeriodID,
		AmountDelta:   1,
	})
	if !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("expected reason required, got %v", err)
	}

	err = manager.ApplyManualAdjustment(ctx, ManualAdjustment{
		AdjustmentID:  "adjustment_negative_effective_usage",
		TenantID:      fixtureTenantA,
		Category:      CategoryRunLaunches,
		QuotaPeriodID: period.QuotaPeriodID,
		AmountDelta:   -3,
		Reason:        "operator correction",
	})
	if !errors.Is(err, ErrNegativeEffectiveUsage) {
		t.Fatalf("expected negative effective usage error, got %v", err)
	}

	err = manager.ApplyManualAdjustment(ctx, ManualAdjustment{
		AdjustmentID:  "adjustment_unknown_category",
		TenantID:      fixtureTenantA,
		Category:      Category("unknown"),
		QuotaPeriodID: period.QuotaPeriodID,
		AmountDelta:   1,
		Reason:        "operator correction",
	})
	if err == nil {
		t.Fatal("expected unknown category error")
	}
}

func TestApplyManualAdjustmentUpdatesCounterAndEvidence(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	definition, _ := DefinitionFor(CategoryRunLaunches)
	period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{
		UsageCounterID:  "counter_adjustment",
		TenantID:        fixtureTenantA,
		Category:        CategoryRunLaunches,
		QuotaPeriodID:   period.QuotaPeriodID,
		CommittedAmount: 2,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatal(err)
	}

	err = manager.ApplyManualAdjustment(ctx, ManualAdjustment{
		AdjustmentID:         "adjustment_valid",
		TenantID:             fixtureTenantA,
		Category:             CategoryRunLaunches,
		QuotaPeriodID:        period.QuotaPeriodID,
		AmountDelta:          -1,
		Reason:               "operator correction",
		CreatedByPrincipalID: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	counter, ok, err := repo.UsageCounter(ctx, fixtureTenantA, CategoryRunLaunches, period.QuotaPeriodID)
	if err != nil || !ok {
		t.Fatalf("counter err=%v ok=%v", err, ok)
	}
	if counter.AdjustedAmount != -1 {
		t.Fatalf("expected adjusted amount -1, got %+v", counter)
	}
	if len(repo.adjustments) != 1 {
		t.Fatalf("expected one adjustment record, got %d", len(repo.adjustments))
	}
	if len(repo.events) != 1 || repo.events[0].EventKind != UsageEventManualAdjustment || repo.events[0].Reason == "" {
		t.Fatalf("expected manual adjustment evidence, got %+v", repo.events)
	}
}
