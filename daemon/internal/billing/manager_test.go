package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerReserveCommitRefundIdempotency(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	input := ReserveInput{
		TenantID:          fixtureTenantA,
		Category:          CategoryRunLaunches,
		Amount:            1,
		OperationKey:      RunOperationKey(fixtureTenantA, fixtureClient, fixtureRunID),
		ReservationPoint:  "test",
		GuardedEntryPoint: "POST /v1/runs",
		Hosted:            true,
	}
	first, err := manager.Reserve(ctx, input)
	if err != nil || !first.Allowed {
		t.Fatalf("reserve err=%v result=%+v", err, first)
	}
	second, err := manager.Reserve(ctx, input)
	if err != nil || !second.Allowed || second.Reservation.ReservationID != first.Reservation.ReservationID {
		t.Fatalf("idempotent reserve err=%v result=%+v", err, second)
	}
	committed, err := manager.Commit(ctx, ResolveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, OperationKey: input.OperationKey})
	if err != nil || committed.Status != ReservationStatusCommitted {
		t.Fatalf("commit err=%v reservation=%+v", err, committed)
	}
	committedAgain, err := manager.Commit(ctx, ResolveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, OperationKey: input.OperationKey})
	if err != nil || committedAgain.AmountCommitted != committed.AmountCommitted {
		t.Fatalf("idempotent commit err=%v reservation=%+v", err, committedAgain)
	}
}

func TestManagerLifecycleReplayIsIdempotent(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	tests := []struct {
		name    string
		resolve func(context.Context, *Manager, ResolveInput) (UsageReservation, error)
		status  ReservationStatus
	}{
		{
			name: "commit",
			resolve: func(ctx context.Context, manager *Manager, input ResolveInput) (UsageReservation, error) {
				return manager.Commit(ctx, input)
			},
			status: ReservationStatusCommitted,
		},
		{
			name: "refund",
			resolve: func(ctx context.Context, manager *Manager, input ResolveInput) (UsageReservation, error) {
				return manager.Refund(ctx, input)
			},
			status: ReservationStatusRefunded,
		},
		{
			name: "release",
			resolve: func(ctx context.Context, manager *Manager, input ResolveInput) (UsageReservation, error) {
				return manager.Release(ctx, input)
			},
			status: ReservationStatusReleased,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFixtureRepo(t, now)
			manager := NewManagerWithClock(repo, func() time.Time { return now })
			key := RunOperationKey(fixtureTenantA, fixtureClient, "run_"+tt.name)
			if result, err := manager.Reserve(ctx, ReserveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, Amount: 1, OperationKey: key, Hosted: true}); err != nil || !result.Allowed {
				t.Fatalf("reserve err=%v result=%+v", err, result)
			}
			input := ResolveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, OperationKey: key}
			first, err := tt.resolve(ctx, manager, input)
			if err != nil || first.Status != tt.status {
				t.Fatalf("first resolve err=%v reservation=%+v", err, first)
			}
			second, err := tt.resolve(ctx, manager, input)
			if err != nil || second.Status != tt.status {
				t.Fatalf("second resolve err=%v reservation=%+v", err, second)
			}
			definition, _ := DefinitionFor(CategoryRunLaunches)
			period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
			if err != nil {
				t.Fatal(err)
			}
			counter, ok, err := repo.UsageCounter(ctx, fixtureTenantA, CategoryRunLaunches, period.QuotaPeriodID)
			if err != nil || !ok {
				t.Fatalf("counter err=%v ok=%v", err, ok)
			}
			if counter.ReservedAmount != 0 {
				t.Fatalf("reserved amount changed on replay: %+v", counter)
			}
			if tt.status == ReservationStatusCommitted && counter.CommittedAmount != 1 {
				t.Fatalf("commit replay double-counted or failed to count: %+v", counter)
			}
			if tt.status != ReservationStatusCommitted && counter.CommittedAmount != 0 {
				t.Fatalf("non-commit lifecycle committed usage: %+v", counter)
			}
		})
	}
}

func TestManagerDeniesWhenQuotaExhausted(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	definition, _ := DefinitionFor(CategoryRunLaunches)
	period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{UsageCounterID: "counter_1", TenantID: fixtureTenantA, Category: CategoryRunLaunches, QuotaPeriodID: period.QuotaPeriodID, CommittedAmount: definition.DefaultLimit, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	result, err := manager.Reserve(ctx, ReserveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, Amount: 1, OperationKey: "op_over", Hosted: true})
	if !errors.Is(err, ErrQuotaDenied) || result.Denial == nil || result.Denial.ReasonCode != "quota_denied:run_launches_exhausted" {
		t.Fatalf("expected stable quota denial, err=%v result=%+v", err, result)
	}
	if len(repo.denials) != 1 {
		t.Fatalf("expected recorded denial, got %d", len(repo.denials))
	}
}

func TestManagerDeniedReservationReplayIsIdempotent(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	definition, _ := DefinitionFor(CategoryRunLaunches)
	period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{UsageCounterID: "counter_1", TenantID: fixtureTenantA, Category: CategoryRunLaunches, QuotaPeriodID: period.QuotaPeriodID, CommittedAmount: definition.DefaultLimit, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	input := ReserveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, Amount: 1, OperationKey: "op_denied_replay", Hosted: true}
	first, err := manager.Reserve(ctx, input)
	if !errors.Is(err, ErrQuotaDenied) || first.Reservation.Status != ReservationStatusDenied {
		t.Fatalf("first denial err=%v result=%+v", err, first)
	}
	second, err := manager.Reserve(ctx, input)
	if !errors.Is(err, ErrQuotaDenied) || second.Reservation.ReservationID != first.Reservation.ReservationID {
		t.Fatalf("second denial err=%v result=%+v", err, second)
	}
	if len(repo.denials) != 1 {
		t.Fatalf("expected one recorded denial after replay, got %d", len(repo.denials))
	}
}

func TestManagerReserveAllRollsBackPartialReservationsOnDeniedCategory(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	toolDefinition, _ := DefinitionFor(CategoryRuntimeToolCalls)
	toolPeriod, err := repo.OpenPeriod(ctx, fixtureTenantA, toolDefinition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{
		UsageCounterID:  "counter_tool_exhausted",
		TenantID:        fixtureTenantA,
		Category:        CategoryRuntimeToolCalls,
		QuotaPeriodID:   toolPeriod.QuotaPeriodID,
		CommittedAmount: toolDefinition.DefaultLimit,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := manager.ReserveAll(ctx, []ReserveInput{
		{
			TenantID:          fixtureTenantA,
			Category:          CategoryIntegrationOperations,
			Amount:            1,
			OperationKey:      IntegrationOperationKey(fixtureTenantA, "mail", "op_1", ""),
			ReservationPoint:  "integration preflight",
			GuardedEntryPoint: "mail operation",
			Hosted:            true,
		},
		{
			TenantID:          fixtureTenantA,
			Category:          CategoryRuntimeToolCalls,
			Amount:            1,
			OperationKey:      ToolCallOperationKey(fixtureTenantA, fixtureRunID, fixtureStepID, "tool_1", ""),
			ReservationPoint:  "tool call creation",
			GuardedEntryPoint: "tool call",
			Hosted:            true,
		},
	})
	if !errors.Is(err, ErrQuotaDenied) || result.Allowed || result.Denial == nil {
		t.Fatalf("expected multi-category denial, err=%v result=%+v", err, result)
	}
	integrationDefinition, _ := DefinitionFor(CategoryIntegrationOperations)
	integrationPeriod, err := repo.OpenPeriod(ctx, fixtureTenantA, integrationDefinition, now)
	if err != nil {
		t.Fatal(err)
	}
	counter, ok, err := repo.UsageCounter(ctx, fixtureTenantA, CategoryIntegrationOperations, integrationPeriod.QuotaPeriodID)
	if err != nil || !ok {
		t.Fatalf("counter err=%v ok=%v", err, ok)
	}
	if counter.ReservedAmount != 0 {
		t.Fatalf("expected integration reservation rollback, got %+v", counter)
	}
	reservation, ok, err := repo.ReservationByOperation(ctx, fixtureTenantA, CategoryIntegrationOperations, IntegrationOperationKey(fixtureTenantA, "mail", "op_1", ""))
	if err != nil || !ok {
		t.Fatalf("reservation err=%v ok=%v", err, ok)
	}
	if reservation.Status != ReservationStatusReleased {
		t.Fatalf("expected rolled-back reservation to be released, got %+v", reservation)
	}
}

func TestManagerLoweredQuotaDeniesNewWorkImmediately(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	definition, _ := DefinitionFor(CategoryRunLaunches)
	period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{UsageCounterID: "counter_1", TenantID: fixtureTenantA, Category: CategoryRunLaunches, QuotaPeriodID: period.QuotaPeriodID, CommittedAmount: 1, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	loweredLimit := int64(0)
	if err := manager.ApplyQuotaOverride(ctx, QuotaOverride{QuotaOverrideID: "override_lowered", TenantID: fixtureTenantA, Category: CategoryRunLaunches, Limit: &loweredLimit, Reason: "downgrade", CreatedByPrincipalID: "admin"}); err != nil {
		t.Fatal(err)
	}
	result, err := manager.Reserve(ctx, ReserveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, Amount: 1, OperationKey: "op_after_lowered_quota", Hosted: true})
	if !errors.Is(err, ErrQuotaDenied) || result.Denial == nil || !result.Quota.OverLimit {
		t.Fatalf("expected lowered quota denial, err=%v result=%+v", err, result)
	}
}
