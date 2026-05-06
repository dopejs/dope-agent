package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
)

func TestSetupWizardStorePersistsCurrentSessionAndAppendOnlyAttempts(t *testing.T) {
	dataDir := t.TempDir()
	s, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)

	first := setupwizard.SetupSession{
		SetupSessionID:      "setup_ten_a_provider_openai_compatible_submitted_secret",
		TenantID:            "ten_a",
		ActorPrincipalID:    "prn_a",
		TargetID:            setupwizard.TargetOpenAICompatible,
		TargetKind:          setupwizard.TargetKindProvider,
		SetupStyle:          setupwizard.SetupStyleSubmittedSecret,
		State:               setupwizard.StateInProgress,
		RemediationOwner:    setupwizard.OwnerProductUser,
		SafeUseMode:         setupwizard.SafeUseBlocked,
		RedactionStatus:     setupwizard.RedactionRedacted,
		CreatedAt:           now,
		UpdatedAt:           now,
		LastTransitionAt:    now,
		CurrentAttemptID:    "attempt_start",
		RedactedEvidence:    map[string]string{"redactionRule": "metadata_only"},
		AllowedCapabilities: []string{"metadata_read"},
	}
	if err := s.SaveSetupSession(ctx, first); err != nil {
		t.Fatalf("SaveSetupSession first: %v", err)
	}
	if err := s.AppendSetupAttempt(ctx, setupwizard.SetupAttempt{
		AttemptID:        "attempt_start",
		SetupSessionID:   first.SetupSessionID,
		TenantID:         first.TenantID,
		Operation:        setupwizard.OperationStart,
		ToState:          setupwizard.StateInProgress,
		RedactionStatus:  setupwizard.RedactionRedacted,
		RedactedEvidence: map[string]string{"redactionRule": "metadata_only"},
		CreatedAt:        now,
	}); err != nil {
		t.Fatalf("AppendSetupAttempt start: %v", err)
	}

	first.State = setupwizard.StateReady
	first.CurrentAttemptID = "attempt_ready"
	first.DiagnosticResultID = "diag_ready"
	first.UpdatedAt = now.Add(time.Minute)
	if err := s.SaveSetupSession(ctx, first); err != nil {
		t.Fatalf("SaveSetupSession ready: %v", err)
	}
	if err := s.AppendSetupAttempt(ctx, setupwizard.SetupAttempt{
		AttemptID:          "attempt_ready",
		SetupSessionID:     first.SetupSessionID,
		TenantID:           first.TenantID,
		Operation:          setupwizard.OperationSubmitSecret,
		FromState:          setupwizard.StateInProgress,
		ToState:            setupwizard.StateReady,
		DiagnosticResultID: "diag_ready",
		RedactionStatus:    setupwizard.RedactionRedacted,
		CreatedAt:          now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("AppendSetupAttempt ready: %v", err)
	}

	got, ok, err := s.GetSetupSession(ctx, "ten_a", first.SetupSessionID)
	if err != nil {
		t.Fatalf("GetSetupSession: %v", err)
	}
	if !ok || got.State != setupwizard.StateReady || got.DiagnosticResultID != "diag_ready" {
		t.Fatalf("stored session = %+v, ok=%v", got, ok)
	}
	if _, ok, err := s.GetSetupSession(ctx, "ten_b", first.SetupSessionID); err != nil || ok {
		t.Fatalf("cross-tenant GetSetupSession ok=%v err=%v, want not found", ok, err)
	}
	attempts, err := s.ListSetupAttempts(ctx, "ten_a", first.SetupSessionID)
	if err != nil {
		t.Fatalf("ListSetupAttempts: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempt count=%d, want 2", len(attempts))
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("reopen NewSQLiteStore: %v", err)
	}
	defer reopened.Close()
	recovered, ok, err := reopened.GetSetupSession(ctx, "ten_a", first.SetupSessionID)
	if err != nil || !ok || recovered.State != setupwizard.StateReady {
		t.Fatalf("reopened session = %+v ok=%v err=%v", recovered, ok, err)
	}
	recoveredAttempts, err := reopened.ListSetupAttempts(ctx, "ten_a", first.SetupSessionID)
	if err != nil || len(recoveredAttempts) != 2 {
		t.Fatalf("reopened attempts count=%d err=%v", len(recoveredAttempts), err)
	}
}

func TestSetupWizardServicePersistsTenantAuditEvents(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	actor := identity.TenantContext{
		TenantID:    "ten_audit_setup",
		PrincipalID: "prn_audit_setup",
		Permissions: []identity.Permission{
			identity.PermissionSecretsManage,
			identity.PermissionIntegrationsManage,
			identity.PermissionCredentialsInspect,
		},
	}
	service := setupwizard.NewService(setupwizard.ServiceDependencies{
		Store: s,
		Audit: setupwizard.NewTenantAuditRecorder(s),
		Now:   func() time.Time { return time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC) },
	})

	session, err := service.Start(ctx, setupwizard.StartInput{TenantContext: actor, TargetID: setupwizard.TargetOpenAICompatible, SetupStyle: setupwizard.SetupStyleSubmittedSecret, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	session, err = service.SubmitSecret(ctx, setupwizard.SubmitSecretInput{
		TenantContext: actor,
		SessionID:     session.SetupSessionID,
		SecretRef:     "OPENAI_COMPATIBLE_API_KEY",
		Value:         "R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK",
		DisplayName:   "OpenAI-compatible API key",
	})
	if err != nil {
		t.Fatalf("SubmitSecret returned error: %v", err)
	}
	if session.LastTransitionAuditID == "" {
		t.Fatalf("expected session audit event id, got %+v", session)
	}

	events, err := s.ListTenantAuditEvents(ctx, identity.AuditEventFilter{TenantID: actor.TenantID, EventKind: "credential_setup.secret_submitted", Limit: 10})
	if err != nil {
		t.Fatalf("ListTenantAuditEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].Document["diagnosticResultId"] == "" || events[0].Document["targetId"] != setupwizard.TargetOpenAICompatible {
		t.Fatalf("unexpected setup audit events: %+v", events)
	}
}
