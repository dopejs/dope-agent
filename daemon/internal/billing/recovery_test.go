package billing

import (
	"context"
	"testing"
	"time"
)

func TestRecoverPendingReservationsAppliesDecisions(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	definition, _ := DefinitionFor(CategoryIntegrationOperations)
	period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{
		UsageCounterID: "counter_recovery",
		TenantID:       fixtureTenantA,
		Category:       CategoryIntegrationOperations,
		QuotaPeriodID:  period.QuotaPeriodID,
		ReservedAmount: 4,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"op_commit", "op_release", "op_refund", "op_operator"} {
		if err := repo.SaveReservation(ctx, UsageReservation{
			ReservationID:  "reservation_" + key,
			TenantID:       fixtureTenantA,
			Category:       CategoryIntegrationOperations,
			QuotaPeriodID:  period.QuotaPeriodID,
			OperationKey:   key,
			AmountReserved: 1,
			Status:         ReservationStatusReserved,
			CreatedAt:      now,
			UpdatedAt:      now,
		}); err != nil {
			t.Fatal(err)
		}
	}

	decisions, err := manager.RecoverPendingReservations(ctx, func(reservation UsageReservation) RecoveryDecision {
		switch reservation.OperationKey {
		case "op_commit":
			return RecoveryDecision{Reservation: reservation, Outcome: ReservationStatusCommitted, Reason: "commit proven"}
		case "op_release":
			return RecoveryDecision{Reservation: reservation, Outcome: ReservationStatusReleased, Reason: "not started"}
		case "op_refund":
			return RecoveryDecision{Reservation: reservation, Outcome: ReservationStatusRefunded, Reason: "canceled"}
		default:
			return RecoveryDecision{Reservation: reservation, Outcome: ReservationStatusOperatorActionNeeded, Reason: "ambiguous"}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 4 {
		t.Fatalf("expected four recovery decisions, got %d", len(decisions))
	}
	counter, ok, err := repo.UsageCounter(ctx, fixtureTenantA, CategoryIntegrationOperations, period.QuotaPeriodID)
	if err != nil || !ok {
		t.Fatalf("counter err=%v ok=%v", err, ok)
	}
	if counter.CommittedAmount != 1 || counter.ReservedAmount != 1 {
		t.Fatalf("unexpected recovered counter: %+v", counter)
	}
	wantStatuses := map[string]ReservationStatus{
		"op_commit":   ReservationStatusCommitted,
		"op_release":  ReservationStatusReleased,
		"op_refund":   ReservationStatusRefunded,
		"op_operator": ReservationStatusOperatorActionNeeded,
	}
	for key, want := range wantStatuses {
		reservation, ok, err := repo.ReservationByOperation(ctx, fixtureTenantA, CategoryIntegrationOperations, key)
		if err != nil || !ok {
			t.Fatalf("reservation %s err=%v ok=%v", key, err, ok)
		}
		if reservation.Status != want {
			t.Fatalf("reservation %s status=%s, want %s", key, reservation.Status, want)
		}
	}
}

func TestRecoverPendingReservationsDefaultsAmbiguousToOperatorActionNeeded(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	definition, _ := DefinitionFor(CategoryRunLaunches)
	period, err := repo.OpenPeriod(ctx, fixtureTenantA, definition, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveUsageCounter(ctx, UsageCounter{UsageCounterID: "counter_recovery_default", TenantID: fixtureTenantA, Category: CategoryRunLaunches, QuotaPeriodID: period.QuotaPeriodID, ReservedAmount: 1, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveReservation(ctx, UsageReservation{
		ReservationID:  "reservation_ambiguous",
		TenantID:       fixtureTenantA,
		Category:       CategoryRunLaunches,
		QuotaPeriodID:  period.QuotaPeriodID,
		OperationKey:   "op_ambiguous",
		AmountReserved: 1,
		Status:         ReservationStatusReserved,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.RecoverPendingReservations(ctx, nil); err != nil {
		t.Fatal(err)
	}
	reservation, ok, err := repo.ReservationByOperation(ctx, fixtureTenantA, CategoryRunLaunches, "op_ambiguous")
	if err != nil || !ok {
		t.Fatalf("reservation err=%v ok=%v", err, ok)
	}
	if reservation.Status != ReservationStatusOperatorActionNeeded || reservation.RecoveryReason == "" {
		t.Fatalf("expected operator action needed with reason, got %+v", reservation)
	}
	result, err := manager.Reserve(ctx, ReserveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, OperationKey: "op_ambiguous", Hosted: true})
	if err != ErrOperatorActionRequired || result.Denial == nil || result.Reservation.Status != ReservationStatusOperatorActionNeeded {
		t.Fatalf("expected duplicate work denial until operator resolution, err=%v result=%+v", err, result)
	}
}
