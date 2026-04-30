package opsreadiness_test

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

func TestIntegrationDiagnosticSmokeReportOutcomes(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	report := opsreadiness.BuildIntegrationDiagnosticSmokeReport("smoke_r42", "operator", []opsreadiness.SmokeProbeInput{
		smokeProbe("calendar", integrations.ReasonHealthy),
		smokeProbe("mail", integrations.ReasonScopeMissing),
		{TenantID: "ten_r42", IntegrationID: "integration_blocked", DomainKind: "calendar", ProviderKind: "feishu_lark", ProbeAction: "calendar.read", SafeCredentialsAvailable: false, TenantApprovalAvailable: true, ProviderAvailable: true, Supported: true, ReadOnlyOrReversible: true},
		{TenantID: "ten_r42", IntegrationID: "integration_skipped", DomainKind: "custom", ProviderKind: "native", ProbeAction: "custom.read", SafeCredentialsAvailable: true, TenantApprovalAvailable: true, ProviderAvailable: true, Supported: false, ReadOnlyOrReversible: true},
	}, startedAt)

	if report.TenantID != "ten_r42" || len(report.ProbeOutcomes) != 4 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.ProbeOutcomes[0].Result != opsreadiness.SmokeProbePassed {
		t.Fatalf("expected passed outcome: %+v", report.ProbeOutcomes[0])
	}
	if report.ProbeOutcomes[1].Result != opsreadiness.SmokeProbeFailed {
		t.Fatalf("expected failed outcome: %+v", report.ProbeOutcomes[1])
	}
	if report.ProbeOutcomes[2].Result != opsreadiness.SmokeProbeBlocked || report.ProbeOutcomes[2].BlockedOrSkippedReason != opsreadiness.SmokeReasonMissingSafeCredentials {
		t.Fatalf("expected blocked missing credentials outcome: %+v", report.ProbeOutcomes[2])
	}
	if report.ProbeOutcomes[3].Result != opsreadiness.SmokeProbeSkipped || report.ProbeOutcomes[3].BlockedOrSkippedReason != opsreadiness.SmokeReasonUnsupportedDomain {
		t.Fatalf("expected skipped unsupported outcome: %+v", report.ProbeOutcomes[3])
	}
}

func TestIntegrationDiagnosticSmokeBlocksRiskyProbeWithoutDualApproval(t *testing.T) {
	t.Parallel()

	base := smokeProbe("calendar", integrations.ReasonHealthy)
	base.ReadOnlyOrReversible = false
	base.TenantAdminApproved = false
	base.OperatorApproved = true
	missingTenantAdmin := opsreadiness.BuildSmokeProbeOutcome("smoke_risky", 0, base, time.Now().UTC())
	if missingTenantAdmin.Result != opsreadiness.SmokeProbeBlocked || missingTenantAdmin.BlockedOrSkippedReason != opsreadiness.SmokeReasonMissingTenantAdminApproval {
		t.Fatalf("expected tenant-admin approval block: %+v", missingTenantAdmin)
	}

	base.TenantAdminApproved = true
	base.OperatorApproved = false
	missingOperator := opsreadiness.BuildSmokeProbeOutcome("smoke_risky", 1, base, time.Now().UTC())
	if missingOperator.Result != opsreadiness.SmokeProbeBlocked || missingOperator.BlockedOrSkippedReason != opsreadiness.SmokeReasonMissingOperatorApproval {
		t.Fatalf("expected operator approval block: %+v", missingOperator)
	}
}

func TestIntegrationDiagnosticSmokeApprovedPathCompletesWithinTarget(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().UTC()
	report := opsreadiness.BuildIntegrationDiagnosticSmokeReport("smoke_fast", "operator", []opsreadiness.SmokeProbeInput{
		smokeProbe("calendar", integrations.ReasonHealthy),
	}, startedAt)
	if report.CompletedAt == nil || report.CompletedAt.Sub(report.StartedAt) > 10*time.Minute {
		t.Fatalf("expected SC-007 smoke target, got %+v", report)
	}
}

func smokeProbe(domain string, reason integrations.DiagnosticReasonCode) opsreadiness.SmokeProbeInput {
	return opsreadiness.SmokeProbeInput{
		TenantID:                 "ten_r42",
		IntegrationID:            "integration_" + domain,
		IntegrationAccountID:     "acct_" + domain,
		DomainKind:               domain,
		ProviderKind:             "feishu_lark",
		ProbeAction:              domain + ".read",
		SafeCredentialsAvailable: true,
		TenantApprovalAvailable:  true,
		ProviderAvailable:        true,
		Supported:                true,
		ReadOnlyOrReversible:     true,
		TenantAdminApproved:      true,
		OperatorApproved:         true,
		ReasonCode:               reason,
	}
}
