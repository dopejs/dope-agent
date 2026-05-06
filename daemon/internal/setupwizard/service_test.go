package setupwizard

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

func TestServiceCompletesProofTargetsWithoutLeakingSecrets(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(ServiceDependencies{Store: store})
	actor := setupActor("ten_setup")

	targets, err := service.ListTargets(context.Background(), ListTargetsInput{TenantContext: actor})
	if err != nil {
		t.Fatalf("ListTargets returned error: %v", err)
	}
	if len(targets) < 2 {
		t.Fatalf("expected proof targets, got %d", len(targets))
	}

	submitted, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetOpenAICompatible, SetupStyle: SetupStyleSubmittedSecret, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start submitted-secret returned error: %v", err)
	}
	submitted, err = service.SubmitSecret(context.Background(), SubmitSecretInput{
		TenantContext: actor,
		SessionID:     submitted.SetupSessionID,
		SecretRef:     "OPENAI_COMPATIBLE_API_KEY",
		Value:         "R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK",
		DisplayName:   "OpenAI-compatible API key",
	})
	if err != nil {
		t.Fatalf("SubmitSecret returned error: %v", err)
	}
	if submitted.State != StateReady {
		t.Fatalf("submitted-secret state=%s, want %s", submitted.State, StateReady)
	}
	if ContainsForbiddenEvidence(submitted, []string{"R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK"}) {
		t.Fatalf("session leaked submitted secret: %+v", submitted)
	}

	oauth, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetFeishuLark, SetupStyle: SetupStyleOAuth, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start OAuth returned error: %v", err)
	}
	started, err := service.StartOAuth(context.Background(), OAuthStartInput{TenantContext: actor, SessionID: oauth.SetupSessionID, RedirectRoute: "/setup/oauth/feishu-lark/callback"})
	if err != nil {
		t.Fatalf("StartOAuth returned error: %v", err)
	}
	completed, err := service.CompleteOAuth(context.Background(), OAuthCallbackInput{TenantContext: actor, SessionID: oauth.SetupSessionID, State: started.StateRef, Result: OAuthResultCompleted})
	if err != nil {
		t.Fatalf("CompleteOAuth returned error: %v", err)
	}
	if completed.State != StateReady {
		t.Fatalf("OAuth state=%s, want %s", completed.State, StateReady)
	}
}

func TestSubmitSecretReadinessComesFromDiagnosticProbe(t *testing.T) {
	store := NewMemoryStore()
	probe := &recordingDiagnosticProbe{
		result: SetupDiagnosticProbeResult{
			State:               StateActionRequired,
			ReasonCode:          ReasonCredentialMissing,
			RemediationOwner:    OwnerTenantAdmin,
			RetrySafety:         RetryRetryable,
			DiagnosticResultID:  "diag_secret_missing",
			DiagnosticRunID:     "run_secret_missing",
			DiagnosticStage:     "credential_probe",
			DiagnosticSource:    DiagnosticSource{Kind: "provider_check", ID: TargetOpenAICompatible},
			AllowedCapabilities: []string{"metadata_read"},
		},
	}
	audit := &recordingSetupAuditSink{}
	service := NewService(ServiceDependencies{Store: store, Diagnostics: probe, Audit: audit})
	actor := setupActor("ten_probe")

	session, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetOpenAICompatible, SetupStyle: SetupStyleSubmittedSecret, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	session, err = service.SubmitSecret(context.Background(), SubmitSecretInput{
		TenantContext: actor,
		SessionID:     session.SetupSessionID,
		SecretRef:     "OPENAI_COMPATIBLE_API_KEY",
		Value:         "R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK",
		DisplayName:   "OpenAI-compatible API key",
	})
	if err != nil {
		t.Fatalf("SubmitSecret returned error: %v", err)
	}
	if session.State != StateActionRequired || session.ReasonCode != ReasonCredentialMissing {
		t.Fatalf("session=%s/%s, want action_required/credential_missing", session.State, session.ReasonCode)
	}
	if session.DiagnosticResultID != "diag_secret_missing" || session.DiagnosticRunID != "run_secret_missing" || session.DiagnosticStage != "credential_probe" {
		t.Fatalf("session did not retain diagnostic linkage: %+v", session)
	}
	if len(probe.calls) != 1 || probe.calls[0].Operation != OperationSubmitSecret {
		t.Fatalf("expected submit_secret probe call, got %+v", probe.calls)
	}
	if len(audit.records) < 2 || audit.records[len(audit.records)-1].Operation != OperationSubmitSecret {
		t.Fatalf("expected setup transition audit records, got %+v", audit.records)
	}
	if audit.records[len(audit.records)-1].DiagnosticResultID != "diag_secret_missing" {
		t.Fatalf("audit did not capture diagnostic id: %+v", audit.records[len(audit.records)-1])
	}
}

func TestDefaultDiagnosticProbeRequiresReadableSubmittedSecret(t *testing.T) {
	service := NewService(ServiceDependencies{
		Store:   NewMemoryStore(),
		Secrets: missingSecretManager{},
	})
	actor := setupActor("ten_default_probe")

	session, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetOpenAICompatible, SetupStyle: SetupStyleSubmittedSecret, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	session.ResourceRefs = []ResourceRef{{Kind: "tenant_secret", ID: "OPENAI_COMPATIBLE_API_KEY"}}
	session, state, reason, err := service.probeReadiness(context.Background(), session, OperationSubmitSecret)
	if err != nil {
		t.Fatalf("probeReadiness returned error: %v", err)
	}
	if state != StateActionRequired || reason != ReasonCredentialMissing || session.DiagnosticResultID == "" {
		t.Fatalf("probe state=%s reason=%s session=%+v, want credential_missing diagnostic", state, reason, session)
	}
}

func TestCompleteOAuthReadyRequiresDiagnosticProbeConfirmation(t *testing.T) {
	probe := &recordingDiagnosticProbe{
		result: SetupDiagnosticProbeResult{
			State:              StateReady,
			ReasonCode:         ReasonHealthy,
			RemediationOwner:   OwnerNoneRequired,
			RetrySafety:        RetryNoActionNeeded,
			DiagnosticResultID: "diag_oauth_ready",
			DiagnosticRunID:    "run_oauth_ready",
			DiagnosticStage:    "oauth_probe",
			DiagnosticSource:   DiagnosticSource{Kind: "integration_probe", ID: TargetFeishuLark},
		},
	}
	service := NewService(ServiceDependencies{Store: NewMemoryStore(), Diagnostics: probe})
	actor := setupActor("ten_oauth_probe")

	session, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetFeishuLark, SetupStyle: SetupStyleOAuth, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	started, err := service.StartOAuth(context.Background(), OAuthStartInput{TenantContext: actor, SessionID: session.SetupSessionID, RedirectRoute: "/callback"})
	if err != nil {
		t.Fatalf("StartOAuth returned error: %v", err)
	}
	session, err = service.CompleteOAuth(context.Background(), OAuthCallbackInput{TenantContext: actor, SessionID: session.SetupSessionID, State: started.StateRef, Result: OAuthResultCompleted})
	if err != nil {
		t.Fatalf("CompleteOAuth returned error: %v", err)
	}
	if session.State != StateReady || session.DiagnosticResultID != "diag_oauth_ready" || session.DiagnosticSourceKind != "integration_probe" {
		t.Fatalf("OAuth completion did not use probe result: %+v", session)
	}
}

func TestOAuthNegativeOutcomesNeverCreateFailedState(t *testing.T) {
	service := NewService(ServiceDependencies{Store: NewMemoryStore()})
	actor := setupActor("ten_oauth_negative")

	cases := []struct {
		name string
		in   OAuthResult
		want SetupState
	}{
		{name: "denied", in: OAuthResultDenied, want: StateActionRequired},
		{name: "abandoned", in: OAuthResultAbandoned, want: StateCancelled},
		{name: "expired", in: OAuthResultExpired, want: StateActionRequired},
		{name: "replay", in: OAuthResultReplay, want: StateActionRequired},
		{name: "tenant mismatch", in: OAuthResultTenantMismatch, want: StateActionRequired},
		{name: "provider error", in: OAuthResultProviderError, want: StateUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetFeishuLark, SetupStyle: SetupStyleOAuth, Source: "wizard"})
			if err != nil {
				t.Fatalf("Start returned error: %v", err)
			}
			started, err := service.StartOAuth(context.Background(), OAuthStartInput{TenantContext: actor, SessionID: session.SetupSessionID, RedirectRoute: "/callback"})
			if err != nil {
				t.Fatalf("StartOAuth returned error: %v", err)
			}
			session, err = service.CompleteOAuth(context.Background(), OAuthCallbackInput{TenantContext: actor, SessionID: session.SetupSessionID, State: started.StateRef, Result: tc.in})
			if err != nil {
				t.Fatalf("CompleteOAuth returned error: %v", err)
			}
			if session.State != tc.want {
				t.Fatalf("state=%s, want %s", session.State, tc.want)
			}
			if session.State == SetupState("failed") {
				t.Fatalf("terminal failed state is forbidden")
			}
		})
	}
}

func TestMutationPermissionsDifferentiateSecretAndOAuthManagement(t *testing.T) {
	tenantID := "ten_perm_split"
	secretOnly := setupActor(tenantID)
	secretOnly.Permissions = []identity.Permission{identity.PermissionCredentialsInspect, identity.PermissionSecretsManage}
	oauthOnly := setupActor(tenantID)
	oauthOnly.Permissions = []identity.Permission{identity.PermissionCredentialsInspect, identity.PermissionIntegrationsManage}

	service := NewService(ServiceDependencies{Store: NewMemoryStore()})
	secretSession, err := service.Start(context.Background(), StartInput{TenantContext: setupActor(tenantID), TargetID: TargetOpenAICompatible, SetupStyle: SetupStyleSubmittedSecret, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start secret session returned error: %v", err)
	}
	if _, err := service.SubmitSecret(context.Background(), SubmitSecretInput{TenantContext: oauthOnly, SessionID: secretSession.SetupSessionID, SecretRef: "OPENAI_COMPATIBLE_API_KEY", Value: "secret", DisplayName: "key"}); err == nil {
		t.Fatal("SubmitSecret without secrets.manage succeeded")
	}

	oauthSession, err := service.Start(context.Background(), StartInput{TenantContext: setupActor(tenantID), TargetID: TargetFeishuLark, SetupStyle: SetupStyleOAuth, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start OAuth session returned error: %v", err)
	}
	if _, err := service.StartOAuth(context.Background(), OAuthStartInput{TenantContext: secretOnly, SessionID: oauthSession.SetupSessionID, RedirectRoute: "/callback"}); err == nil {
		t.Fatal("StartOAuth without integrations.manage succeeded")
	}
}

func TestDependentUseAllowsOnlyDiagnosticConfirmedDegradedCapabilities(t *testing.T) {
	service := NewService(ServiceDependencies{Store: NewMemoryStore()})
	session := SetupSession{
		TenantID:             "ten_degraded",
		TargetID:             TargetOpenAICompatible,
		State:                StateDegraded,
		SafeUseMode:          SafeUseLimited,
		AllowedCapabilities:  []string{"metadata_read", "chat"},
		DiagnosticResultID:   "diag_1",
		DiagnosticAllowedUse: []string{"metadata_read"},
	}
	decision := service.DependentUseDecision(context.Background(), session, "metadata_read")
	if decision.SafeUseMode != SafeUseLimited {
		t.Fatalf("metadata_read mode=%s, want %s", decision.SafeUseMode, SafeUseLimited)
	}
	decision = service.DependentUseDecision(context.Background(), session, "chat")
	if decision.SafeUseMode != SafeUseBlocked {
		t.Fatalf("chat mode=%s, want blocked without diagnostic confirmation", decision.SafeUseMode)
	}
}

type diagnosticProbeCall struct {
	SessionID string
	Operation SetupOperation
}

type recordingDiagnosticProbe struct {
	result SetupDiagnosticProbeResult
	calls  []diagnosticProbeCall
}

func (p *recordingDiagnosticProbe) ProbeSetup(_ context.Context, session SetupSession, operation SetupOperation) (SetupDiagnosticProbeResult, error) {
	p.calls = append(p.calls, diagnosticProbeCall{SessionID: session.SetupSessionID, Operation: operation})
	return p.result, nil
}

type recordingSetupAuditSink struct {
	records []SetupAuditRecord
}

func (s *recordingSetupAuditSink) RecordSetupAudit(_ context.Context, record SetupAuditRecord) (string, error) {
	id := "audit_" + record.SetupSessionID + "_" + string(record.Operation)
	s.records = append(s.records, record)
	return id, nil
}

type missingSecretManager struct{}

func (missingSecretManager) Create(context.Context, secrets.CreateInput) (secrets.TenantSecret, error) {
	return secrets.TenantSecret{}, secrets.ErrSecretNotFound
}

func (missingSecretManager) Rotate(context.Context, secrets.RotateInput) (secrets.TenantSecret, error) {
	return secrets.TenantSecret{}, secrets.ErrSecretNotFound
}

func (missingSecretManager) Get(context.Context, string, string) (secrets.TenantSecret, error) {
	return secrets.TenantSecret{}, secrets.ErrSecretNotFound
}

func (missingSecretManager) Disable(context.Context, secrets.DisableInput) (secrets.TenantSecret, error) {
	return secrets.TenantSecret{}, secrets.ErrSecretNotFound
}

func setupActor(tenantID string) identity.TenantContext {
	return identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: "prn_" + tenantID,
		Permissions: []identity.Permission{
			identity.PermissionSecretsManage,
			identity.PermissionIntegrationsManage,
			identity.PermissionCredentialsInspect,
		},
	}
}
