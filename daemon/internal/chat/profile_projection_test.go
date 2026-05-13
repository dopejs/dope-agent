package chat

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type profileCaptureProvider struct {
	requests []llm.ProviderRequest
}

func (p *profileCaptureProvider) Name() string { return "profile-capture" }

func (p *profileCaptureProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	p.requests = append(p.requests, request)
	return llm.ProviderResponse{Output: "ok", FinishReason: "stop", Usage: llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2}}, nil
}

func (p *profileCaptureProvider) Stream(context.Context, llm.ProviderRequest, llm.StreamEmitter) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{}, nil
}

func TestChatResolvesActiveProfileDefaultsAndContextOnceAtWorkStart(t *testing.T) {
	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	if err := sqliteStore.ReplaceProviderModels(ctx, "profile-capture", []providers.Model{{ProviderID: "profile-capture", ModelID: "profile-model", Default: true, Available: true}}); err != nil {
		t.Fatalf("ReplaceProviderModels returned error: %v", err)
	}
	actor := identity.TenantContext{TenantID: "ten_chat_profile", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesManage, identity.PermissionProfilesInspect}}
	created, err := sqliteStore.CreateAgentProfile(ctx, actor, profiles.MutationInput{
		DisplayName: "Chat Profile",
		Persona:     profiles.Persona{Tone: "direct", SafeSummary: "direct profile"},
		DefaultProviderPreference: profiles.DefaultProviderPreference{
			ProviderID: "profile-capture",
			Model:      "profile-model",
		},
		SafetyDefaults: profiles.SafetyDefaults{ApprovalPosture: "ask"},
		Activate:       true,
	})
	if err != nil {
		t.Fatalf("CreateAgentProfile returned error: %v", err)
	}

	provider := &profileCaptureProvider{}
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(provider)
	service := NewService(dispatcher, nil, nil, events.NewBus(), sqliteStore)
	result, err := service.Query(ctx, QueryInput{TenantID: actor.TenantID, Query: "hello"})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if result.Dispatch.Provider != "profile-capture" || result.Dispatch.Model != "profile-model" {
		t.Fatalf("dispatch did not use active profile provider defaults: %+v", result.Dispatch)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected one provider request, got %d", len(provider.requests))
	}
	joined := profileTestMessagesText(provider.requests[0].Messages)
	if !strings.Contains(joined, "Agent profile persona: direct profile") || !strings.Contains(joined, "Agent profile safety posture: ask") {
		t.Fatalf("profile context was not assembled once at work start: %q", joined)
	}

	detail, found, err := sqliteStore.GetAgentProfileDetail(ctx, actor.TenantID, created.Profile.ProfileID)
	if err != nil {
		t.Fatalf("GetAgentProfileDetail returned error: %v", err)
	}
	if !found || len(detail.Versions) != 1 || detail.Profile.ActiveVersionID != created.Profile.ActiveVersionID {
		t.Fatalf("chat preferences must not create learned profile mutations, found=%v detail=%+v", found, detail)
	}
}

func profileTestMessagesText(messages []llm.Message) string {
	var builder strings.Builder
	for _, message := range messages {
		builder.WriteString(message.Content)
		builder.WriteByte('\n')
	}
	return builder.String()
}
