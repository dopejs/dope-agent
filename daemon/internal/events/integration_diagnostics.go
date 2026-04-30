package events

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

const (
	IntegrationDiagnosticRunStartedName       = "integration_diagnostic.run_started"
	IntegrationDiagnosticRunCompletedName     = "integration_diagnostic.run_completed"
	IntegrationDiagnosticStateChangedName     = "integration_diagnostic.state_changed"
	IntegrationDiagnosticRedactionFailedName  = "integration_diagnostic.redaction_failed_closed"
	IntegrationDiagnosticSmokeCompletedName   = "integration_diagnostic.smoke_completed"
	IntegrationDiagnosticRetentionAppliedName = "integration_diagnostic.retention_applied"
)

func IntegrationDiagnosticRunEvent(name string, run integrations.DiagnosticRun) Event {
	occurredAt := run.StartedAt
	if run.CompletedAt != nil {
		occurredAt = *run.CompletedAt
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   run.TenantID,
		Category:   "integration",
		Name:       name,
		OccurredAt: occurredAt.UTC(),
		Resource:   Resource{Kind: "integration_diagnostic_run", ID: run.DiagnosticRunID},
		Payload: map[string]any{
			"tenantId":        run.TenantID,
			"diagnosticRunId": run.DiagnosticRunID,
			"integrationId":   run.IntegrationID,
			"requestedBy":     run.RequestedBy,
			"status":          string(run.Status),
			"redactionStatus": string(run.RedactionStatus),
		},
	}
}

func IntegrationDiagnosticStateChangedEvent(result integrations.DiagnosticResult, previous integrations.DiagnosticStatus) Event {
	occurredAt := result.CheckedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   result.TenantID,
		Category:   "integration",
		Name:       IntegrationDiagnosticStateChangedName,
		OccurredAt: occurredAt.UTC(),
		Resource:   Resource{Kind: "integration_diagnostic_result", ID: result.DiagnosticResultID},
		Payload: map[string]any{
			"tenantId":           result.TenantID,
			"diagnosticResultId": result.DiagnosticResultID,
			"integrationId":      result.IntegrationID,
			"previousStatus":     string(previous),
			"status":             string(result.Status),
			"reasonCode":         string(result.ReasonCode),
			"remediationOwner":   string(result.RemediationOwner),
		},
	}
}

func IntegrationDiagnosticSmokeCompletedEvent(report opsreadiness.SmokeMatrixReport) Event {
	occurredAt := report.CompletedAt
	if occurredAt == nil {
		now := time.Now().UTC()
		occurredAt = &now
	}
	return Event{
		TenantID:   report.TenantID,
		Category:   "integration",
		Name:       IntegrationDiagnosticSmokeCompletedName,
		OccurredAt: occurredAt.UTC(),
		Resource:   Resource{Kind: "integration_diagnostic_smoke_report", ID: report.SmokeReportID},
		Payload: map[string]any{
			"tenantId":      report.TenantID,
			"smokeReportId": report.SmokeReportID,
			"status":        string(report.Status),
			"domainSummary": report.DomainSummary,
			"artifactRefs":  report.ArtifactRefs,
		},
	}
}

func IntegrationDiagnosticRedactionFailedEvent(result integrations.DiagnosticResult) Event {
	occurredAt := result.CheckedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   result.TenantID,
		Category:   "integration",
		Name:       IntegrationDiagnosticRedactionFailedName,
		OccurredAt: occurredAt.UTC(),
		Resource:   Resource{Kind: "integration_diagnostic_result", ID: result.DiagnosticResultID},
		Payload: map[string]any{
			"tenantId":           result.TenantID,
			"targetKind":         "diagnostic_result",
			"targetId":           result.DiagnosticResultID,
			"diagnosticResultId": result.DiagnosticResultID,
			"integrationId":      result.IntegrationID,
			"reasonCode":         string(result.ReasonCode),
			"redactionStatus":    string(result.RedactionStatus),
		},
	}
}

func IntegrationDiagnosticRetentionAppliedEvent(record integrations.DiagnosticRetentionRecord) Event {
	occurredAt := record.UpdatedAt
	if record.AppliedAt != nil {
		occurredAt = *record.AppliedAt
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Event{
		TenantID:   record.TenantID,
		Category:   "integration",
		Name:       IntegrationDiagnosticRetentionAppliedName,
		OccurredAt: occurredAt.UTC(),
		Resource:   Resource{Kind: "integration_diagnostic_retention", ID: record.RetentionRecordID},
		Payload: map[string]any{
			"tenantId":           record.TenantID,
			"targetKind":         record.TargetKind,
			"targetId":           record.TargetID,
			"retentionState":     string(record.RetentionState),
			"effectiveExpiresAt": record.EffectiveExpiresAt,
		},
	}
}
