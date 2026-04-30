package audit_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func TestBuildIntegrationDiagnosticAuditEvent(t *testing.T) {
	t.Parallel()

	event := audit.BuildIntegrationDiagnosticAuditEvent(audit.IntegrationDiagnosticAuditInput{
		TenantID:        "ten_r42",
		PrincipalID:     "prn_operator",
		Action:          "diagnostic_run.completed",
		TargetKind:      "integration",
		TargetID:        "integration_feishu",
		Outcome:         identity.AuditOutcomeSucceeded,
		ReasonCode:      integrations.ReasonScopeMissing,
		DiagnosticRunID: "diag_run_1",
		RedactionStatus: integrations.RedactionStatusRedacted,
		CreatedAt:       time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC),
	})
	if event.EventKind != audit.IntegrationDiagnosticAuditEventKind || event.TenantID != "ten_r42" || event.PrincipalID != "prn_operator" {
		t.Fatalf("unexpected audit event identity: %+v", event)
	}
	if event.ReasonCode != string(integrations.ReasonScopeMissing) || event.Document["diagnosticRunId"] != "diag_run_1" {
		t.Fatalf("unexpected audit document: %+v", event)
	}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal audit event: %v", err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "bearer") || strings.Contains(strings.ToLower(string(raw)), "token-secret") {
		t.Fatalf("audit event leaked credential material: %+v", event)
	}
}
