package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
)

const (
	BillingUsageReservedName              = "billing.usage_reserved"
	BillingUsageCommittedName             = "billing.usage_committed"
	BillingUsageRefundedName              = "billing.usage_refunded"
	BillingQuotaDeniedName                = "billing.quota_denied"
	BillingManualAdjustmentCreatedName    = "billing.manual_adjustment_created"
	BillingReservationRecoveryDecidedName = "billing.reservation_recovery_decided"
)

func BillingUsageEvent(name string, event billing.UsageEvent) Event {
	occurredAt := event.CreatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   event.TenantID,
		Category:   "billing",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: "billing_usage_event",
			ID:   event.UsageEventID,
		},
		Payload: map[string]any{
			"tenantId":      event.TenantID,
			"category":      string(event.Category),
			"quotaPeriodId": event.QuotaPeriodID,
			"operationKey":  event.OperationKey,
			"amount":        event.Amount,
			"reasonCode":    event.ReasonCode,
			"outcome":       event.Outcome,
		},
	}
}

func BillingQuotaDeniedEvent(denial billing.QuotaDenial) Event {
	occurredAt := denial.CreatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   denial.TenantID,
		Category:   "billing",
		Name:       BillingQuotaDeniedName,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: "billing_denial",
			ID:   denial.DenialID,
		},
		Payload: map[string]any{
			"tenantId":        denial.TenantID,
			"category":        string(denial.Category),
			"quotaPeriodId":   denial.QuotaPeriodID,
			"operationKey":    denial.OperationKey,
			"reasonCode":      denial.ReasonCode,
			"requestedAmount": denial.RequestedAmount,
			"remainingAmount": denial.RemainingAmount,
		},
	}
}

func BillingRecoveryDecisionEvent(decision billing.RecoveryDecision) Event {
	reservation := decision.Reservation
	occurredAt := reservation.UpdatedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	reason := decision.Reason
	if reason == "" {
		reason = reservation.RecoveryReason
	}
	outcome := decision.Outcome
	if outcome == "" {
		outcome = reservation.Status
	}
	return Event{
		TenantID:   reservation.TenantID,
		Category:   "billing",
		Name:       BillingReservationRecoveryDecidedName,
		OccurredAt: occurredAt.UTC(),
		Resource: Resource{
			Kind: "billing_reservation",
			ID:   reservation.ReservationID,
		},
		Payload: map[string]any{
			"tenantId":      reservation.TenantID,
			"category":      string(reservation.Category),
			"quotaPeriodId": reservation.QuotaPeriodID,
			"operationKey":  reservation.OperationKey,
			"outcome":       string(outcome),
			"reason":        reason,
		},
	}
}
