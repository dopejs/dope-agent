package activation_test

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	storepkg "github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestServiceActivationStateSurvivesRestartBeforeAndAfterTestChat(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	sqliteStore, err := storepkg.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	if err := sqliteStore.UpsertPrincipal(ctx, identity.Principal{
		PrincipalID:   "prn_restart",
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Restart User",
		Status:        identity.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	service := activation.NewService(activation.Dependencies{
		StateStore:       sqliteStore,
		Identity:         sqliteStore,
		Chat:             restartChatRunner{now: now},
		Audit:            sqliteStore,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})
	started, err := service.Activate(ctx, activation.ActivateInput{Token: identity.TokenAuthority{TokenID: "tok_restart", PrincipalID: "prn_restart", Status: identity.StatusActive}})
	if err != nil {
		t.Fatalf("Activate returned error: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close before restart returned error: %v", err)
	}

	restartedStore, err := storepkg.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore after restart returned error: %v", err)
	}
	service = activation.NewService(activation.Dependencies{
		StateStore:       restartedStore,
		Identity:         restartedStore,
		Chat:             restartChatRunner{now: now},
		Audit:            restartedStore,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})
	beforeAction, err := service.Get(ctx, activation.GetInput{TenantContext: identity.TenantContext{PrincipalID: "prn_restart", TenantID: started.TenantID, TokenID: "tok_restart"}})
	if err != nil {
		t.Fatalf("Get before action returned error: %v", err)
	}
	if beforeAction.Status != activation.StatusActive || beforeAction.ActivationID != started.ActivationID {
		t.Fatalf("expected active pre-action state after restart, got %#v", beforeAction)
	}
	completed, _, err := service.RunTestChat(ctx, activation.RunTestChatInput{
		Token:         identity.TokenAuthority{TokenID: "tok_restart", PrincipalID: "prn_restart", Status: identity.StatusActive},
		TenantContext: identity.TenantContext{PrincipalID: "prn_restart", TenantID: started.TenantID, TokenID: "tok_restart"},
		Message:       "Safe restart test.",
	})
	if err != nil {
		t.Fatalf("RunTestChat returned error: %v", err)
	}
	if completed.Status != activation.StatusFirstActionCompleted {
		t.Fatalf("expected completed test chat state, got %#v", completed)
	}
	if err := restartedStore.Close(); err != nil {
		t.Fatalf("Close after action returned error: %v", err)
	}

	finalStore, err := storepkg.NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore final restart returned error: %v", err)
	}
	t.Cleanup(func() { _ = finalStore.Close() })
	finalService := activation.NewService(activation.Dependencies{StateStore: finalStore, Identity: finalStore, Now: func() time.Time { return now }, EnvironmentScope: "test", Hosted: true})
	afterAction, err := finalService.Get(ctx, activation.GetInput{TenantContext: identity.TenantContext{PrincipalID: "prn_restart", TenantID: started.TenantID, TokenID: "tok_restart"}})
	if err != nil {
		t.Fatalf("Get after action returned error: %v", err)
	}
	if afterAction.Status != activation.StatusFirstActionCompleted || afterAction.TestChat == nil || afterAction.TestChat.DispatchID != "dispatch_restart" {
		t.Fatalf("expected completed activation after restart, got %#v", afterAction)
	}
}

type restartChatRunner struct {
	now time.Time
}

func (r restartChatRunner) RunActivationTestChat(context.Context, activation.TestChatInput) (activation.TestChatResult, error) {
	return activation.TestChatResult{
		DispatchID:   "dispatch_restart",
		Status:       activation.TestChatStatusCompleted,
		Provider:     "test",
		Model:        "test-chat",
		Usage:        map[string]any{"totalTokens": 2},
		FinishReason: "stop",
		CompletedAt:  r.now,
	}, nil
}
