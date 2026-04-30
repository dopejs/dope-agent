package events_test

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func TestIntegrationDiagnosticEvents(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	run := integrations.DiagnosticRun{
		DiagnosticRunID: "diag_run_1",
		TenantID:        "ten_r42",
		IntegrationID:   "integration_feishu",
		RequestedBy:     "prn_operator",
		Status:          integrations.DiagnosticRunCompleted,
		StartedAt:       now,
		CompletedAt:     &now,
		RedactionStatus: integrations.RedactionStatusRedacted,
	}
	runEvent := events.IntegrationDiagnosticRunEvent(events.IntegrationDiagnosticRunCompletedName, run)
	if runEvent.Name != events.IntegrationDiagnosticRunCompletedName || runEvent.Payload["diagnosticRunId"] != "diag_run_1" {
		t.Fatalf("unexpected run event: %+v", runEvent)
	}

	result := integrations.DiagnosticResult{
		DiagnosticResultID: "diag_result_1",
		TenantID:           "ten_r42",
		IntegrationID:      "integration_feishu",
		Status:             integrations.DiagnosticStatusUnknown,
		ReasonCode:         integrations.ReasonRedactionFailedClosed,
		RedactionStatus:    integrations.RedactionStatusFailedClosed,
		CheckedAt:          now,
	}
	redactionEvent := events.IntegrationDiagnosticRedactionFailedEvent(result)
	if redactionEvent.Name != events.IntegrationDiagnosticRedactionFailedName || redactionEvent.Payload["redactionStatus"] != string(integrations.RedactionStatusFailedClosed) {
		t.Fatalf("unexpected redaction event: %+v", redactionEvent)
	}

	appliedAt := now.Add(time.Minute)
	record := integrations.NewDiagnosticRetentionRecord("ten_r42", "diagnostic_run", "diag_run_1", now)
	record.RetentionState = integrations.DiagnosticRetentionExpired
	record.AppliedAt = &appliedAt
	retentionEvent := events.IntegrationDiagnosticRetentionAppliedEvent(record)
	if retentionEvent.Name != events.IntegrationDiagnosticRetentionAppliedName || retentionEvent.Payload["retentionState"] != string(integrations.DiagnosticRetentionExpired) {
		t.Fatalf("unexpected retention event: %+v", retentionEvent)
	}
}
