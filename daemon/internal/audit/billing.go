package audit

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

const BillingAuditEventKind = "billing.audit_recorded"

type BillingAuditInput struct {
	TenantID        string
	PrincipalID     string
	Category        billing.Category
	OperationKey    string
	ReservationID   string
	AdjustmentID    string
	Action          string
	Outcome         string
	ReasonCode      string
	Reason          string
	Amount          int64
	RemainingAmount int64
	CreatedAt       time.Time
}

func BuildBillingAuditEvent(input BillingAuditInput) identity.TenantAuditEvent {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	outcome := input.Outcome
	if outcome == "" {
		outcome = identity.AuditOutcomeSucceeded
	}
	document := map[string]any{
		"action": stringOrDefault(input.Action, "billing.usage_event"),
	}
	if input.Category != "" {
		document["category"] = string(input.Category)
	}
	if input.OperationKey != "" {
		document["operationKey"] = input.OperationKey
	}
	if input.ReservationID != "" {
		document["reservationId"] = input.ReservationID
	}
	if input.AdjustmentID != "" {
		document["adjustmentId"] = input.AdjustmentID
	}
	if input.Reason != "" {
		document["reason"] = input.Reason
	}
	if input.Amount != 0 {
		document["amount"] = input.Amount
	}
	if input.RemainingAmount != 0 {
		document["remainingAmount"] = input.RemainingAmount
	}
	return identity.TenantAuditEvent{
		EventKind:   BillingAuditEventKind,
		TenantID:    input.TenantID,
		PrincipalID: input.PrincipalID,
		Outcome:     outcome,
		ReasonCode:  input.ReasonCode,
		CreatedAt:   createdAt.UTC(),
		Document:    document,
	}
}

func DefaultBillingAuditRetentionPolicy(tenantID string) billing.AuditRetentionPolicy {
	return billing.AuditRetentionPolicy{
		TenantID:        tenantID,
		RetentionMode:   "indefinite",
		RetentionPeriod: "",
		CreatedAt:       time.Now().UTC(),
	}
}

func stringOrDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
