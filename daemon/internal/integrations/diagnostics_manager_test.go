package integrations_test

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
)

func TestDiagnosticManagerProjectsReadinessOwnerWithinInspectionTarget(t *testing.T) {
	t.Parallel()

	manager := integrations.NewDiagnosticManager()
	resource := integrations.Resource{
		TenantID:               "ten_diag",
		IntegrationID:          "integration_feishu",
		DomainKind:             "calendar",
		DisplayName:            "Feishu Calendar",
		ReadinessStatus:        integrations.ReadinessStatusDegraded,
		ReadinessReason:        "scope missing for calendar.read",
		RequiredOperatorAction: "grant scope",
		AccountBinding:         integrations.AccountBinding{AccountKey: "acct_feishu"},
		BackendBinding:         integrations.BackendBinding{BackendKind: integrations.BackendKind("feishu_lark")},
	}

	started := time.Now()
	result := manager.Inspect(integrations.DiagnosticInspectionInput{
		Resource:   resource,
		Capability: "calendar.read",
		CheckedAt:  time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC),
	})
	if elapsed := time.Since(started); elapsed > 2*time.Minute {
		t.Fatalf("inspection exceeded SC-001 target: %s", elapsed)
	}
	if result.Status != integrations.DiagnosticStatusBlocked || result.ReasonCode != integrations.ReasonScopeMissing {
		t.Fatalf("unexpected diagnostic classification: %+v", result)
	}
	if result.RemediationOwner != integrations.RemediationOwnerTenantAdmin {
		t.Fatalf("expected tenant admin remediation owner, got %s", result.RemediationOwner)
	}
}

func TestDiagnosticManagerProjectsLimitedUnsupportedAndRedactionFailure(t *testing.T) {
	t.Parallel()

	manager := integrations.NewDiagnosticManager()
	limited := manager.Inspect(integrations.DiagnosticInspectionInput{
		Resource: integrations.Resource{
			TenantID:        "ten_diag",
			IntegrationID:   "integration_mail",
			DomainKind:      "mail",
			BackendBinding:  integrations.BackendBinding{BackendKind: integrations.BackendKindFakeLocal, SupportsProbeRead: true},
			ReadinessStatus: integrations.ReadinessStatusHealthy,
		},
		CheckedAt: time.Now().UTC(),
	})
	if limited.ReasonCode != integrations.ReasonLimitedDiagnostic {
		t.Fatalf("expected limited diagnostic projection, got %+v", limited)
	}

	unsupported := manager.Inspect(integrations.DiagnosticInspectionInput{
		Resource: integrations.Resource{
			TenantID:       "ten_diag",
			IntegrationID:  "integration_custom",
			DomainKind:     "custom",
			BackendBinding: integrations.BackendBinding{BackendKind: integrations.BackendKindNative},
		},
		CheckedAt: time.Now().UTC(),
	})
	if unsupported.Status != integrations.DiagnosticStatusUnsupported || unsupported.ReasonCode != integrations.ReasonUnsupportedDiagnostic {
		t.Fatalf("expected unsupported diagnostic projection, got %+v", unsupported)
	}

	failedClosed := manager.Inspect(integrations.DiagnosticInspectionInput{
		Resource: integrations.Resource{
			TenantID:       "ten_diag",
			IntegrationID:  "integration_custom",
			DomainKind:     "custom",
			BackendBinding: integrations.BackendBinding{BackendKind: integrations.BackendKindNative},
		},
		CheckedAt:    time.Now().UTC(),
		ForceGeneric: true,
	})
	if failedClosed.RedactionStatus != integrations.RedactionStatusFailedClosed || failedClosed.ReasonCode != integrations.ReasonRedactionFailedClosed {
		t.Fatalf("expected fail-closed redaction projection, got %+v", failedClosed)
	}
}
