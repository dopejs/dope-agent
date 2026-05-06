package activation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestActivationAuditFailClosedWithStableRetryableReason(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_audit_fail"] = activePrincipal("prn_audit_fail", now)
	svc := NewService(Dependencies{
		StateStore:       newMemoryStateStore(),
		Identity:         repo,
		Audit:            failingActivationAuditSink{},
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	_, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_audit_fail", PrincipalID: "prn_audit_fail", Status: identity.StatusActive}})
	if got := ReasonCodeFromError(err); got != ReasonAuditWriteFailed {
		t.Fatalf("expected audit write reason, got %q err=%v", got, err)
	}
	var activationErr *Error
	if !errors.As(err, &activationErr) || !activationErr.Retryable || activationErr.RemediationOwner != RemediationOwnerOperator {
		t.Fatalf("expected retryable operator audit error, got %#v", err)
	}
}

func TestActivationAuditRecordsMetadataOnlyTestChatCompletion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_audit_metadata"] = activePrincipal("prn_audit_metadata", now)
	auditSink := &recordingAuditSink{}
	svc := NewService(Dependencies{
		StateStore:       newMemoryStateStore(),
		Identity:         repo,
		Chat:             &recordingActivationChatRunner{result: TestChatResult{DispatchID: "dispatch_audit", Status: TestChatStatusCompleted, Provider: "test", Model: "test-chat", Usage: map[string]any{"totalTokens": 2}, CompletedAt: now}},
		Audit:            auditSink,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})
	state, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_audit_metadata", PrincipalID: "prn_audit_metadata", Status: identity.StatusActive}})
	if err != nil {
		t.Fatalf("Activate returned error: %v", err)
	}
	if _, _, err := svc.RunTestChat(ctx, RunTestChatInput{
		Token:         identity.TokenAuthority{TokenID: "tok_audit_metadata", PrincipalID: "prn_audit_metadata", Status: identity.StatusActive},
		TenantContext: identity.TenantContext{PrincipalID: "prn_audit_metadata", TenantID: state.TenantID, TokenID: "tok_audit_metadata"},
		Message:       "Do not audit this prompt.",
	}); err != nil {
		t.Fatalf("RunTestChat returned error: %v", err)
	}
	payload := mustActivationJSON(t, auditSink.events)
	if !strings.Contains(payload, "tenant.activation_test_chat_completed") {
		t.Fatalf("expected completion audit event, got %s", payload)
	}
	if !strings.Contains(payload, "dispatch_audit") || !strings.Contains(payload, "test-chat") {
		t.Fatalf("expected metadata-only test chat audit fields, got %s", payload)
	}
	for _, forbidden := range []string{"Do not audit this prompt", "query", "reply", "transcript", "rawProviderPayload", "authorization", "accessToken", "refreshToken"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("audit retained forbidden evidence %q in %s", forbidden, payload)
		}
	}
}

func TestActivationTestChatFailurePersistsRecoverableStateAndAudit(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_chat_fail"] = activePrincipal("prn_chat_fail", now)
	stateStore := newMemoryStateStore()
	auditSink := &recordingAuditSink{}
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Chat:             failingActivationChatRunner{},
		Audit:            auditSink,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})
	state, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_chat_fail", PrincipalID: "prn_chat_fail", Status: identity.StatusActive}})
	if err != nil {
		t.Fatalf("Activate returned error: %v", err)
	}

	_, metadata, err := svc.RunTestChat(ctx, RunTestChatInput{
		Token:         identity.TokenAuthority{TokenID: "tok_chat_fail", PrincipalID: "prn_chat_fail", Status: identity.StatusActive},
		TenantContext: identity.TenantContext{PrincipalID: "prn_chat_fail", TenantID: state.TenantID, TokenID: "tok_chat_fail"},
		Message:       "Do not persist this failed prompt.",
	})
	if got := ReasonCodeFromError(err); got != ReasonTestChatFailed {
		t.Fatalf("expected test chat failure reason, got %q err=%v", got, err)
	}
	if metadata.Status != TestChatStatusFailed || metadata.ReasonCode != ReasonTestChatFailed {
		t.Fatalf("expected failed metadata, got %#v", metadata)
	}
	persisted, ok, err := stateStore.GetActivationStateForPrincipalTenant(ctx, "prn_chat_fail", state.TenantID)
	if err != nil || !ok {
		t.Fatalf("expected persisted recoverable activation state, ok=%v err=%v", ok, err)
	}
	if persisted.Status != StatusActive || persisted.CurrentStepID != StepTestChat || persisted.FirstAction.Available == false {
		t.Fatalf("expected active retryable activation state, got %#v", persisted)
	}
	if persisted.FailureReason == nil || persisted.FailureReason.ReasonCode != ReasonTestChatFailed || !persisted.FailureReason.Retryable {
		t.Fatalf("expected retryable test chat failure reason, got %#v", persisted.FailureReason)
	}
	if persisted.TestChat == nil || persisted.TestChat.Status != TestChatStatusFailed {
		t.Fatalf("expected failed test chat metadata to persist, got %#v", persisted.TestChat)
	}
	diagnostics, err := svc.Diagnostics(ctx, GetInput{TenantContext: identity.TenantContext{PrincipalID: "prn_chat_fail", TenantID: state.TenantID, TokenID: "tok_chat_fail"}})
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Stage != FailureStageTestChat || diagnostics[0].ReasonCode != ReasonTestChatFailed || diagnostics[0].TestChat == nil {
		t.Fatalf("expected test chat diagnostic, got %#v", diagnostics)
	}
	payload := mustActivationJSON(t, auditSink.events)
	if !strings.Contains(payload, "tenant.activation_failed") || !strings.Contains(payload, "activation_failed:test_chat") || !strings.Contains(payload, "dispatch_failed") {
		t.Fatalf("expected failed activation audit metadata, got %s", payload)
	}
	if strings.Contains(payload, "Do not persist this failed prompt.") {
		t.Fatalf("audit retained failed test chat prompt: %s", payload)
	}
}

type failingActivationAuditSink struct{}

func (failingActivationAuditSink) AppendTenantAuditEvent(context.Context, identity.TenantAuditEvent) (identity.TenantAuditEvent, error) {
	return identity.TenantAuditEvent{}, errors.New("audit unavailable")
}

type failingActivationChatRunner struct{}

func (failingActivationChatRunner) RunActivationTestChat(context.Context, TestChatInput) (TestChatResult, error) {
	return TestChatResult{
		DispatchID:   "dispatch_failed",
		Status:       TestChatStatusFailed,
		Provider:     "test",
		Model:        "test-chat",
		Usage:        map[string]any{"totalTokens": 1, "prompt": "forbidden"},
		FinishReason: "error",
	}, errors.New("upstream test chat failed")
}

func mustDecodeActivationAuditDocuments(t *testing.T, events []identity.TenantAuditEvent) []map[string]any {
	t.Helper()
	out := make([]map[string]any, 0, len(events))
	for _, event := range events {
		if len(event.Document) == 0 {
			continue
		}
		var document map[string]any
		encoded, err := json.Marshal(event.Document)
		if err != nil {
			t.Fatalf("marshal audit document: %v", err)
		}
		if err := json.Unmarshal(encoded, &document); err != nil {
			t.Fatalf("decode audit document: %v", err)
		}
		out = append(out, document)
	}
	return out
}
