package audit_test

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestBillingAuditEventConstruction(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)
	event := audit.BuildBillingAuditEvent(audit.BillingAuditInput{
		TenantID:        "ten_r38_a",
		PrincipalID:     "prn_admin",
		Category:        billing.CategoryRunLaunches,
		OperationKey:    "tenant:ten_r38_a:run:client_1",
		ReservationID:   "reservation_1",
		Action:          "billing.usage_reserved",
		Outcome:         identity.AuditOutcomeSucceeded,
		ReasonCode:      "usage_reserved",
		Amount:          1,
		RemainingAmount: 0,
		CreatedAt:       createdAt,
	})
	if event.EventKind != audit.BillingAuditEventKind {
		t.Fatalf("EventKind=%q, want %q", event.EventKind, audit.BillingAuditEventKind)
	}
	if event.TenantID != "ten_r38_a" || event.PrincipalID != "prn_admin" {
		t.Fatalf("tenant/principal not preserved: %#v", event)
	}
	if event.Outcome != identity.AuditOutcomeSucceeded || event.ReasonCode != "usage_reserved" {
		t.Fatalf("outcome/reason not preserved: %#v", event)
	}
	if event.Document["operationKey"] != "tenant:ten_r38_a:run:client_1" || event.Document["amount"] != int64(1) {
		t.Fatalf("document did not preserve billing evidence: %#v", event.Document)
	}
}

func TestDefaultBillingAuditRetentionIsIndefinite(t *testing.T) {
	policy := audit.DefaultBillingAuditRetentionPolicy("ten_r38_a")
	if policy.RetentionMode != "indefinite" || policy.TenantID != "ten_r38_a" {
		t.Fatalf("unexpected default retention policy: %#v", policy)
	}
}

func TestBillingRecoveryAuditEventConstruction(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 10, 5, 0, 0, time.UTC)
	event := audit.BuildBillingAuditEvent(audit.BillingAuditInput{
		TenantID:      "ten_r38_a",
		PrincipalID:   "operator",
		Category:      billing.CategoryRunLaunches,
		OperationKey:  "tenant:ten_r38_a:run:client_1",
		ReservationID: "reservation_1",
		Action:        "billing.reservation_recovery_decided",
		Outcome:       identity.AuditOutcomeSucceeded,
		ReasonCode:    "billing.reservation_recovery_decided",
		Reason:        "restart outcome could not be proven",
		Amount:        1,
		CreatedAt:     createdAt,
	})
	if event.EventKind != audit.BillingAuditEventKind || event.ReasonCode != "billing.reservation_recovery_decided" {
		t.Fatalf("unexpected recovery audit event: %#v", event)
	}
	if event.Document["action"] != "billing.reservation_recovery_decided" || event.Document["reservationId"] != "reservation_1" || event.Document["reason"] == "" {
		t.Fatalf("recovery audit event lost evidence: %#v", event.Document)
	}
}
