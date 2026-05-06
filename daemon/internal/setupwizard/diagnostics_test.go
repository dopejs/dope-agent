package setupwizard

import (
	"context"
	"testing"
	"time"
)

func TestDiagnosticReasonMappingCoversRepresentativeSetupFailures(t *testing.T) {
	cases := []struct {
		reason string
		state  SetupState
		owner  RemediationOwner
		retry  RetrySafety
	}{
		{ReasonCredentialMissing, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonScopeMissing, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonTenantApprovalPending, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonTokenMissing, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonTokenExpired, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonTokenRevoked, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonOAuthDenied, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonOAuthAbandoned, StateUnavailable, OwnerOperator, RetryRetryable},
		{ReasonTenantMismatch, StateActionRequired, OwnerTenantAdmin, RetryRetryable},
		{ReasonProviderUnavailable, StateUnavailable, OwnerProvider, RetryRetryable},
		{ReasonNetworkFailed, StateUnavailable, OwnerProvider, RetryRetryable},
		{ReasonRateLimited, StateUnavailable, OwnerProvider, RetryRetryable},
		{ReasonUnsupportedTarget, StateActionRequired, OwnerOperator, RetryBlocked},
		{ReasonRedactionFailedClosed, StateActionRequired, OwnerOperator, RetryBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			state, owner, retry := ClassifyDiagnosticReason(tc.reason)
			if state != tc.state || owner != tc.owner || retry != tc.retry {
				t.Fatalf("ClassifyDiagnosticReason(%s) = %s/%s/%s, want %s/%s/%s", tc.reason, state, owner, retry, tc.state, tc.owner, tc.retry)
			}
		})
	}
}

func TestDiagnosticsProjectionUsesRedactedSessionStateAndAllowedUse(t *testing.T) {
	service := NewService(ServiceDependencies{
		Store: NewMemoryStore(),
		Now:   func() time.Time { return time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC) },
	})
	actor := setupActor("ten_diag")
	session, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetFeishuLark, SetupStyle: SetupStyleOAuth, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	session.State = StateDegraded
	session.ReasonCode = ReasonRateLimited
	session.AllowedCapabilities = []string{"metadata_read"}
	session.DiagnosticAllowedUse = []string{"metadata_read"}
	session.DiagnosticResultID = "diag_lark_limited"
	session.DiagnosticRunID = "run_lark_limited"
	session.DiagnosticStage = "integration_probe"
	session.DiagnosticSourceKind = "integration_diagnostic"
	session.DiagnosticSourceID = TargetFeishuLark
	if err := service.store.SaveSetupSession(context.Background(), session); err != nil {
		t.Fatalf("SaveSetupSession returned error: %v", err)
	}
	items, err := service.Diagnostics(context.Background(), actor, session.SetupSessionID)
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if len(items) != 1 || items[0].Status != StateDegraded || items[0].RetrySafety != RetryRetryable || len(items[0].AllowedCapabilities) != 1 {
		t.Fatalf("unexpected diagnostics: %+v", items)
	}
	if items[0].DiagnosticRunID != "run_lark_limited" || items[0].DiagnosticStage != "integration_probe" || items[0].DiagnosticSourceKind != "integration_diagnostic" || items[0].DiagnosticSourceID != TargetFeishuLark {
		t.Fatalf("diagnostics did not expose source/stage details: %+v", items[0])
	}
	if ContainsForbiddenEvidence(items, []string{"authorization_code", "access_token", "refresh_token"}) {
		t.Fatalf("diagnostics leaked OAuth evidence: %+v", items)
	}
}

func TestNormalizeProbeResultClassifiesReasonWhenStateIsAbsent(t *testing.T) {
	got := normalizeProbeResult(SetupSession{
		SetupSessionID: "setup_probe_classify",
		TargetID:       TargetOpenAICompatible,
		TargetKind:     TargetKindProvider,
	}, OperationSubmitSecret, SetupDiagnosticProbeResult{ReasonCode: ReasonCredentialMissing})
	if got.State != StateActionRequired || got.RemediationOwner != OwnerTenantAdmin || got.RetrySafety != RetryRetryable {
		t.Fatalf("normalizeProbeResult classified %+v, want action_required/tenant_admin/retryable", got)
	}
	if got.DiagnosticResultID == "" || got.DiagnosticRunID == "" || got.DiagnosticStage != "credential_probe" || got.DiagnosticSource.Kind != "provider_check" {
		t.Fatalf("normalizeProbeResult did not fill diagnostic linkage: %+v", got)
	}
}
