package activation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestActivationTestChatPersistsMetadataOnly(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	repo := newMemoryIdentityRepository()
	repo.principals["prn_redaction"] = activePrincipal("prn_redaction", now)
	stateStore := newMemoryStateStore()
	auditSink := &recordingAuditSink{}
	chat := &recordingActivationChatRunner{result: TestChatResult{
		DispatchID: "dispatch_redacted",
		Status:     TestChatStatusCompleted,
		Provider:   "test",
		Model:      "test-chat",
		Usage: map[string]any{
			"inputTokens":        1,
			"query":              "forbidden query",
			"reply":              "forbidden reply",
			"transcript":         "forbidden transcript",
			"delta":              "forbidden delta",
			"prompt":             "forbidden prompt",
			"rawProviderPayload": "forbidden raw payload",
			"authorization":      "Bearer forbidden",
			"accessToken":        "token",
			"refreshToken":       "refresh",
			"secret":             "secret",
		},
		FinishReason: "stop",
		CompletedAt:  now,
	}}
	svc := NewService(Dependencies{
		StateStore:       stateStore,
		Identity:         repo,
		Chat:             chat,
		Audit:            auditSink,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	started, err := svc.Activate(ctx, ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_redaction", PrincipalID: "prn_redaction", Status: identity.StatusActive}})
	if err != nil {
		t.Fatalf("Activate returned error: %v", err)
	}
	completed, metadata, err := svc.RunTestChat(ctx, RunTestChatInput{
		Token: identity.TokenAuthority{TokenID: "tok_redaction", PrincipalID: "prn_redaction", Status: identity.StatusActive},
		TenantContext: identity.TenantContext{
			PrincipalID: "prn_redaction",
			TokenID:     "tok_redaction",
			TenantID:    started.TenantID,
		},
		Message: "Never persist this test chat message.",
	})
	if err != nil {
		t.Fatalf("RunTestChat returned error: %v", err)
	}

	payload := mustActivationJSON(t, completed)
	payload += mustActivationJSON(t, metadata)
	payload += mustActivationJSON(t, auditSink.events)
	for _, forbidden := range []string{
		"Never persist this test chat message",
		"forbidden query",
		"forbidden reply",
		"forbidden transcript",
		"forbidden delta",
		"forbidden prompt",
		"forbidden raw payload",
		"Bearer forbidden",
		"accessToken",
		"refreshToken",
		"secret",
	} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("activation test chat retained forbidden evidence %q in %s", forbidden, payload)
		}
	}
	if completed.TestChat == nil || completed.TestChat.Usage["inputTokens"] != 1 {
		t.Fatalf("expected safe usage metadata to remain, got %#v", completed.TestChat)
	}
}

type recordingActivationChatRunner struct {
	result TestChatResult
	err    error
	last   TestChatInput
}

func (r *recordingActivationChatRunner) RunActivationTestChat(_ context.Context, input TestChatInput) (TestChatResult, error) {
	r.last = input
	return r.result, r.err
}

func mustActivationJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	return string(payload)
}
