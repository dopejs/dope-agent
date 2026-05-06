package integrations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
)

func TestManagerTracksReadinessTransitionsAndCanonicalDefault(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")

	first, err := manager.Create(CreateInput{
		IntegrationID:    "calendar-a",
		DomainKind:       "calendar",
		DisplayName:      "Calendar A",
		EnvironmentScope: "test",
		CanonicalDefault: true,
		AccountBinding: AccountBinding{
			AccountKey:   "acct_primary",
			AccountLabel: "Primary Calendar",
		},
		BackendBinding: BackendBinding{
			BackendKind: BackendKindFakeLocal,
		},
	})
	if err != nil {
		t.Fatalf("Create(first) returned error: %v", err)
	}
	if !first.CanonicalDefault {
		t.Fatal("expected first integration to start as canonical default")
	}

	second, err := manager.Create(CreateInput{
		IntegrationID:    "calendar-b",
		DomainKind:       "calendar",
		DisplayName:      "Calendar B",
		EnvironmentScope: "test",
		CanonicalDefault: true,
		AccountBinding: AccountBinding{
			AccountKey:   "acct_primary",
			AccountLabel: "Primary Calendar",
		},
		BackendBinding: BackendBinding{
			BackendKind: BackendKindFakeLocal,
		},
	})
	if err != nil {
		t.Fatalf("Create(second) returned error: %v", err)
	}
	if !second.CanonicalDefault {
		t.Fatal("expected second integration to become canonical default")
	}

	first, ok := manager.Get(first.IntegrationID)
	if !ok {
		t.Fatal("expected first integration to remain addressable")
	}
	if first.CanonicalDefault {
		t.Fatalf("expected first integration to be demoted after replacement, got %+v", first)
	}

	updated, err := manager.UpdateReadiness(second.IntegrationID, UpdateReadinessInput{
		ReadinessStatus:        ReadinessStatusHealthy,
		AuthState:              AuthStateAuthorized,
		HealthState:            HealthStateHealthy,
		ReadinessReason:        "probe passed",
		RequiredOperatorAction: "none",
		SecretResolution:       "resolved",
	})
	if err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}
	if updated.ReadinessStatus != ReadinessStatusHealthy || updated.AuthState != AuthStateAuthorized || updated.HealthState != HealthStateHealthy {
		t.Fatalf("expected healthy readiness projection, got %+v", updated)
	}
	if updated.LastReadyAt == nil {
		t.Fatal("expected LastReadyAt to be set when readiness becomes healthy")
	}
	if updated.Provenance.SecretResolution != "resolved" || !updated.Provenance.SecretMaterialPresent {
		t.Fatalf("expected provenance secret resolution to be projected, got %+v", updated.Provenance)
	}

	swapped, err := manager.SetCanonicalDefault(first.IntegrationID)
	if err != nil {
		t.Fatalf("SetCanonicalDefault returned error: %v", err)
	}
	if !swapped.CanonicalDefault {
		t.Fatal("expected selected integration to become canonical default")
	}
	second, ok = manager.Get(second.IntegrationID)
	if !ok {
		t.Fatal("expected second integration after canonical default swap")
	}
	if second.CanonicalDefault {
		t.Fatalf("expected second integration to be demoted after swap, got %+v", second)
	}
}

func TestManagerSetupDependentUseDecisionBlocksDisabledAndUnconfirmedDegradedUse(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	disabled := manager.setupDependentUseDecision(context.Background(), setupwizard.SetupSession{
		TenantID:         "ten_setup",
		TargetID:         setupwizard.TargetFeishuLark,
		State:            setupwizard.StateDisabled,
		ReasonCode:       setupwizard.ReasonDisabledByUser,
		RedactionStatus:  setupwizard.RedactionRedacted,
		RemediationOwner: setupwizard.OwnerProductUser,
	}, "metadata_read")
	if disabled.SafeUseMode != setupwizard.SafeUseBlocked {
		t.Fatalf("expected disabled integration setup to block dependent use, got %+v", disabled)
	}

	limited := manager.setupDependentUseDecision(context.Background(), setupwizard.SetupSession{
		TenantID:             "ten_setup",
		TargetID:             setupwizard.TargetFeishuLark,
		State:                setupwizard.StateDegraded,
		AllowedCapabilities:  []string{"metadata_read"},
		DiagnosticAllowedUse: []string{"metadata_read"},
		ReasonCode:           setupwizard.ReasonScopeMissing,
		RedactionStatus:      setupwizard.RedactionRedacted,
	}, "metadata_read")
	if limited.SafeUseMode != setupwizard.SafeUseLimited {
		t.Fatalf("expected diagnostic-confirmed degraded integration setup to allow limited use, got %+v", limited)
	}
}

func TestManagerRunProbeWithSetupGateBlocksUnsafeIntegrationUse(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	created, err := manager.Create(CreateInput{
		TenantID:      "ten_setup",
		IntegrationID: "lark-a",
		DomainKind:    "mail",
		DisplayName:   "Lark A",
		BackendBinding: BackendBinding{
			BackendKind:           BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	_, err = manager.UpdateReadinessForTenant(created.IntegrationID, "ten_setup", UpdateReadinessInput{
		ReadinessStatus: ReadinessStatusHealthy,
		AuthState:       AuthStateAuthorized,
		HealthState:     HealthStateHealthy,
	})
	if err != nil {
		t.Fatalf("UpdateReadinessForTenant returned error: %v", err)
	}

	_, _, decision, err := manager.runProbeWithSetupGate(created.IntegrationID, ProbeKindInspect, nil, setupwizard.SetupSession{
		TenantID:        "ten_setup",
		TargetID:        setupwizard.TargetFeishuLark,
		TargetKind:      setupwizard.TargetKindIntegration,
		State:           setupwizard.StateDisabled,
		ReasonCode:      setupwizard.ReasonDisabledByUser,
		RedactionStatus: setupwizard.RedactionRedacted,
	}, "metadata_read")
	if err == nil || !errors.Is(err, ErrProbeBlocked) {
		t.Fatalf("runProbeWithSetupGate error=%v, want ErrProbeBlocked", err)
	}
	if decision.SafeUseMode != setupwizard.SafeUseBlocked {
		t.Fatalf("unexpected blocked decision: %+v", decision)
	}

	_, result, decision, err := manager.runProbeWithSetupGate(created.IntegrationID, ProbeKindInspect, nil, setupwizard.SetupSession{
		TenantID:             "ten_setup",
		TargetID:             setupwizard.TargetFeishuLark,
		TargetKind:           setupwizard.TargetKindIntegration,
		State:                setupwizard.StateDegraded,
		AllowedCapabilities:  []string{"metadata_read"},
		DiagnosticAllowedUse: []string{"metadata_read"},
		ReasonCode:           setupwizard.ReasonRateLimited,
		RedactionStatus:      setupwizard.RedactionRedacted,
	}, "metadata_read")
	if err != nil {
		t.Fatalf("runProbeWithSetupGate limited returned error: %v", err)
	}
	if decision.SafeUseMode != setupwizard.SafeUseLimited || result.ProbeKind != ProbeKindInspect {
		t.Fatalf("unexpected limited result=%+v decision=%+v", result, decision)
	}
}

func TestManagerRunProbeAllowsDegradedAndBlocksUnavailable(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	created, err := manager.Create(CreateInput{
		IntegrationID:    "mail-a",
		DomainKind:       "mail",
		DisplayName:      "Mail A",
		EnvironmentScope: "test",
		AccountBinding: AccountBinding{
			AccountKey: "acct_mail",
		},
		BackendBinding: BackendBinding{
			BackendKind:           BackendKindFakeLocal,
			SupportsProbeRead:     true,
			SupportsProbeMutation: true,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := manager.UpdateReadiness(created.IntegrationID, UpdateReadinessInput{
		ReadinessStatus: ReadinessStatusDegraded,
		AuthState:       AuthStateAuthorized,
		HealthState:     HealthStateDegraded,
		ReadinessReason: "upstream latency",
	}); err != nil {
		t.Fatalf("UpdateReadiness(degraded) returned error: %v", err)
	}

	resource, result, summary, err := manager.RunProbe(created.IntegrationID, ProbeKindInspect, map[string]any{"mode": "readonly"})
	if err != nil {
		t.Fatalf("RunProbe(degraded) returned error: %v", err)
	}
	if resource.ReadinessStatus != ReadinessStatusDegraded {
		t.Fatalf("expected degraded resource snapshot, got %+v", resource)
	}
	if result.Status != "completed" || result.ResultSummary["message"] == "" {
		t.Fatalf("expected fake backend completion summary, got %+v", result)
	}
	if summary.ReadinessAtInvocation != ReadinessStatusDegraded || summary.IntegrationID != created.IntegrationID {
		t.Fatalf("expected summary to capture degraded invocation state, got %+v", summary)
	}

	if _, err := manager.UpdateReadiness(created.IntegrationID, UpdateReadinessInput{
		ReadinessStatus: ReadinessStatusUnavailable,
		AuthState:       AuthStateExpired,
		HealthState:     HealthStateUnavailable,
		ReadinessReason: "token revoked",
	}); err != nil {
		t.Fatalf("UpdateReadiness(unavailable) returned error: %v", err)
	}

	if _, _, _, err := manager.RunProbe(created.IntegrationID, ProbeKindInspect, nil); !errors.Is(err, ErrProbeBlocked) {
		t.Fatalf("expected ErrProbeBlocked, got %v", err)
	}
}

func TestManagerRunProbeUsesFeishuLarkDiagnosticBackend(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	created, err := manager.Create(CreateInput{
		IntegrationID:    "calendar-feishu",
		DomainKind:       "calendar",
		DisplayName:      "Feishu Calendar",
		EnvironmentScope: "test",
		AccountBinding: AccountBinding{
			AccountKey: "acct_feishu",
		},
		BackendBinding: BackendBinding{
			BackendKind:       BackendKindFeishuLark,
			SupportsProbeRead: true,
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := manager.UpdateReadiness(created.IntegrationID, UpdateReadinessInput{
		ReadinessStatus: ReadinessStatusHealthy,
		AuthState:       AuthStateAuthorized,
		HealthState:     HealthStateHealthy,
	}); err != nil {
		t.Fatalf("UpdateReadiness returned error: %v", err)
	}

	_, result, summary, err := manager.RunProbe(created.IntegrationID, ProbeKindInspect, map[string]any{
		"providerEvidence": map[string]any{"code": "scope_not_granted"},
		"operationClass":   "calendar.read",
	})
	if err != nil {
		t.Fatalf("RunProbe returned error: %v", err)
	}
	if result.Status != "failed" || result.FailureClass != "scope_not_granted" {
		t.Fatalf("expected Feishu/Lark probe failure evidence, got %+v", result)
	}
	if summary.BackendKind != BackendKindFeishuLark || summary.IntegrationID != created.IntegrationID {
		t.Fatalf("expected Feishu/Lark binding summary, got %+v", summary)
	}
}

func TestManagerNormalizesBackendBindingAndBindingSummary(t *testing.T) {
	t.Parallel()

	manager := NewManager("test")
	created, err := manager.Create(CreateInput{
		IntegrationID:    "calendar-managed",
		DomainKind:       "calendar",
		DisplayName:      "Managed Calendar",
		EnvironmentScope: "test",
		CanonicalDefault: true,
		AccountBinding: AccountBinding{
			AccountKey: "acct_calendar",
		},
		BackendBinding: BackendBinding{
			BackendKind:        BackendKindManagedProvider,
			BackendRefID:       " managed-calendar ",
			BackendDisplayName: " Managed Calendar ",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.BackendBinding.SourceKind != string(BackendKindManagedProvider) {
		t.Fatalf("expected source kind default to backend kind, got %+v", created.BackendBinding)
	}
	if created.BackendBinding.BackendRefID != "managed-calendar" || created.BackendBinding.BackendDisplayName != "Managed Calendar" {
		t.Fatalf("expected backend binding to be trimmed, got %+v", created.BackendBinding)
	}

	capturedAt := time.Now().UTC()
	summary, err := manager.BindingSummary(created.IntegrationID, capturedAt)
	if err != nil {
		t.Fatalf("BindingSummary returned error: %v", err)
	}
	if summary.BackendKind != BackendKindManagedProvider || summary.EnvironmentScope != "test" || summary.CapturedAt != capturedAt {
		t.Fatalf("expected binding summary to reflect normalized backend state, got %+v", summary)
	}
}
