package audit

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

const IntegrationDiagnosticAuditEventKind = "integration_diagnostic.audit_recorded"

type IntegrationDiagnosticAuditInput struct {
	TenantID        string
	PrincipalID     string
	Action          string
	TargetKind      string
	TargetID        string
	Outcome         string
	ReasonCode      integrations.DiagnosticReasonCode
	DiagnosticRunID string
	SmokeReportID   string
	RedactionStatus integrations.RedactionStatus
	CreatedAt       time.Time
}

func BuildIntegrationDiagnosticAuditEvent(input IntegrationDiagnosticAuditInput) identity.TenantAuditEvent {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	outcome := input.Outcome
	if outcome == "" {
		outcome = identity.AuditOutcomeSucceeded
	}
	document := map[string]any{
		"action": input.Action,
	}
	if input.TargetKind != "" {
		document["targetKind"] = input.TargetKind
	}
	if input.TargetID != "" {
		document["targetId"] = input.TargetID
	}
	if input.DiagnosticRunID != "" {
		document["diagnosticRunId"] = input.DiagnosticRunID
	}
	if input.SmokeReportID != "" {
		document["smokeReportId"] = input.SmokeReportID
	}
	if input.RedactionStatus != "" {
		document["redactionStatus"] = string(input.RedactionStatus)
	}
	return identity.TenantAuditEvent{
		EventKind:   IntegrationDiagnosticAuditEventKind,
		TenantID:    input.TenantID,
		PrincipalID: input.PrincipalID,
		Outcome:     outcome,
		ReasonCode:  string(input.ReasonCode),
		CreatedAt:   createdAt.UTC(),
		Document:    document,
	}
}
