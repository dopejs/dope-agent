package opsreadiness

import (
	"strconv"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

type SmokeProbeInput struct {
	TenantID                 string
	IntegrationID            string
	IntegrationAccountID     string
	DomainKind               string
	ProviderKind             string
	ProbeAction              string
	RequestedBy              string
	SafeCredentialsAvailable bool
	TenantApprovalAvailable  bool
	ProviderAvailable        bool
	Supported                bool
	ReadOnlyOrReversible     bool
	TenantAdminApproved      bool
	OperatorApproved         bool
	OperatorDeferred         bool
	ReasonCode               integrations.DiagnosticReasonCode
	ArtifactRefs             []string
	CheckedAt                time.Time
}

func BuildIntegrationDiagnosticSmokeReport(reportID string, requestedBy string, probes []SmokeProbeInput, startedAt time.Time) SmokeMatrixReport {
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	completedAt := startedAt
	report := SmokeMatrixReport{
		SmokeReportID:      reportID,
		ReportKind:         "diagnostic",
		RequestedBy:        requestedBy,
		Status:             SmokeReportCompleted,
		DomainSummary:      map[string]string{},
		StartedAt:          startedAt.UTC(),
		CompletedAt:        &completedAt,
		ArtifactRefs:       []string{},
		RetentionExpiresAt: integrations.DiagnosticRetentionExpiry(startedAt),
		ProbeOutcomes:      []SmokeProbeOutcome{},
	}
	for index, probe := range probes {
		if report.TenantID == "" {
			report.TenantID = probe.TenantID
		}
		outcome := BuildSmokeProbeOutcome(reportID, index, probe, startedAt)
		report.ProbeOutcomes = append(report.ProbeOutcomes, outcome)
		report.DomainSummary[outcome.DomainKind] = string(outcome.Result)
		report.ArtifactRefs = append(report.ArtifactRefs, outcome.ArtifactRefs...)
		if outcome.Result == SmokeProbeFailed {
			report.Status = SmokeReportFailed
		}
		if outcome.Result == SmokeProbeBlocked && report.Status == SmokeReportCompleted {
			report.Status = SmokeReportBlocked
		}
	}
	return report
}

func BuildSmokeProbeOutcome(reportID string, index int, probe SmokeProbeInput, fallbackTime time.Time) SmokeProbeOutcome {
	checkedAt := probe.CheckedAt.UTC()
	if checkedAt.IsZero() {
		checkedAt = fallbackTime.UTC()
	}
	result := SmokeProbePassed
	blockedReason := SmokeBlockedReason("")
	reasonCode := probe.ReasonCode
	if reasonCode == "" {
		reasonCode = integrations.ReasonHealthy
	}
	switch {
	case probe.OperatorDeferred:
		result = SmokeProbeSkipped
		blockedReason = SmokeReasonOperatorDeferred
		reasonCode = integrations.ReasonOperatorActionNeeded
	case !probe.Supported:
		result = SmokeProbeSkipped
		blockedReason = SmokeReasonUnsupportedDomain
		reasonCode = integrations.ReasonUnsupportedDiagnostic
	case !probe.SafeCredentialsAvailable:
		result = SmokeProbeBlocked
		blockedReason = SmokeReasonMissingSafeCredentials
		reasonCode = integrations.ReasonTokenMissing
	case !probe.TenantApprovalAvailable:
		result = SmokeProbeBlocked
		blockedReason = SmokeReasonTenantApprovalUnavailable
		reasonCode = integrations.ReasonTenantApprovalPending
	case !probe.ProviderAvailable:
		result = SmokeProbeBlocked
		blockedReason = SmokeReasonProviderOutage
		reasonCode = integrations.ReasonProviderUnavailable
	case !probe.ReadOnlyOrReversible && !probe.TenantAdminApproved:
		result = SmokeProbeBlocked
		blockedReason = SmokeReasonMissingTenantAdminApproval
		reasonCode = integrations.ReasonUnsafeToRetry
	case !probe.ReadOnlyOrReversible && !probe.OperatorApproved:
		result = SmokeProbeBlocked
		blockedReason = SmokeReasonMissingOperatorApproval
		reasonCode = integrations.ReasonUnsafeToRetry
	case reasonCode != integrations.ReasonHealthy:
		result = SmokeProbeFailed
	}
	_, owner, retrySafety := integrations.DiagnosticDefaults(reasonCode)
	_ = owner
	return SmokeProbeOutcome{
		ProbeOutcomeID:         diagnosticSmokeOutcomeID(reportID, index),
		TenantID:               probe.TenantID,
		SmokeReportID:          reportID,
		IntegrationID:          probe.IntegrationID,
		IntegrationAccountID:   probe.IntegrationAccountID,
		DomainKind:             probe.DomainKind,
		ProviderKind:           probe.ProviderKind,
		ProbeAction:            probe.ProbeAction,
		Result:                 result,
		ReasonCode:             string(reasonCode),
		RemediationHint:        integrations.DiagnosticRemediationHint(reasonCode),
		RetrySafety:            string(retrySafety),
		BlockedOrSkippedReason: blockedReason,
		ArtifactRefs:           probe.ArtifactRefs,
		CheckedAt:              checkedAt,
		RedactionStatus:        string(integrations.RedactionStatusRedacted),
		RetentionExpiresAt:     integrations.DiagnosticRetentionExpiry(checkedAt),
	}
}

func diagnosticSmokeOutcomeID(reportID string, index int) string {
	return reportID + "_probe_" + strconv.Itoa(index+1)
}
