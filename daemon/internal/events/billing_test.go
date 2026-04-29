package events

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

func TestBillingEventProjection(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	event := BillingUsageEvent(BillingUsageReservedName, billing.UsageEvent{
		UsageEventID:  "usage_event_1",
		TenantID:      "ten_r38_a",
		Category:      billing.CategoryRunLaunches,
		QuotaPeriodID: "period_1",
		OperationKey:  "tenant:ten_r38_a:run:client_1",
		Amount:        1,
		ReasonCode:    "usage_reserved",
		Outcome:       "reserved",
		CreatedAt:     createdAt,
	})
	if event.Category != "billing" || event.Name != BillingUsageReservedName || event.TenantID != "ten_r38_a" {
		t.Fatalf("unexpected billing event identity: %#v", event)
	}
	if event.Payload["operationKey"] != "tenant:ten_r38_a:run:client_1" || event.Payload["amount"] != int64(1) {
		t.Fatalf("billing payload lost accounting fields: %#v", event.Payload)
	}
}

func TestBillingRecoveryDecisionEventProjection(t *testing.T) {
	updatedAt := time.Date(2026, 4, 28, 10, 10, 0, 0, time.UTC)
	event := BillingRecoveryDecisionEvent(billing.RecoveryDecision{
		Reservation: billing.UsageReservation{
			ReservationID:  "reservation_1",
			TenantID:       "ten_r38_a",
			Category:       billing.CategoryRunLaunches,
			QuotaPeriodID:  "period_1",
			OperationKey:   "tenant:ten_r38_a:run:client_1",
			Status:         billing.ReservationStatusOperatorActionNeeded,
			UpdatedAt:      updatedAt,
			RecoveryReason: "restart outcome could not be proven",
		},
		Outcome: billing.ReservationStatusOperatorActionNeeded,
		Reason:  "restart outcome could not be proven",
	})
	if event.Category != "billing" || event.Name != BillingReservationRecoveryDecidedName || event.Resource.Kind != "billing_reservation" {
		t.Fatalf("unexpected recovery decision event identity: %#v", event)
	}
	if event.Payload["outcome"] != string(billing.ReservationStatusOperatorActionNeeded) || event.Payload["reason"] == "" {
		t.Fatalf("recovery decision payload lost evidence: %#v", event.Payload)
	}
}
