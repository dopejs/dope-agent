package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
)

func TestManagerListsProfiles(t *testing.T) {
	dispatcher := llm.NewDispatcher()
	manager := NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "openai_compatible",
			DefaultModel:    "gpt-5.4",
			OpenAICompatible: config.OpenAICompatibleProviderConfig{
				BaseURL:   "https://example.com",
				APIKey:    "secret",
				Model:     "gpt-4.1-mini",
				TimeoutMs: 45000,
			},
		},
	}, dispatcher)

	items := manager.ListProfiles()
	if len(items) != 2 {
		t.Fatalf("expected 2 provider profiles, got %d", len(items))
	}

	openAIProfile, ok := manager.GetProfile(llm.OpenAICompatibleProviderName)
	if !ok {
		t.Fatal("expected openai-compatible profile")
	}
	if openAIProfile.Family != FamilyOpenAICompatible {
		t.Fatalf("expected openai-compatible family, got %s", openAIProfile.Family)
	}
	if openAIProfile.AuthMode != AuthModeAPIKey {
		t.Fatalf("expected api_key auth mode, got %s", openAIProfile.AuthMode)
	}
	if !openAIProfile.SecretConfigured {
		t.Fatal("expected openai-compatible secret to be configured")
	}
	if openAIProfile.RequestURL != "https://example.com/v1/chat/completions" {
		t.Fatalf("unexpected request URL %q", openAIProfile.RequestURL)
	}
}

func TestManagerResolveAppliesExplicitPolicy(t *testing.T) {
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&providerTestLLM{name: llm.OpenAICompatibleProviderName})

	manager := NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "openai_compatible",
			DefaultModel:    "gpt-5.4",
			OpenAICompatible: config.OpenAICompatibleProviderConfig{
				BaseURL: "https://example.com",
				APIKey:  "secret",
				Model:   "gpt-4.1-mini",
			},
		},
	}, dispatcher)

	resolved, err := manager.Resolve("", "", 0, 0)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if resolved.ProviderID != llm.OpenAICompatibleProviderName {
		t.Fatalf("expected configured default provider, got %s", resolved.ProviderID)
	}
	if resolved.Model != "gpt-5.4" {
		t.Fatalf("expected configured default model, got %s", resolved.Model)
	}

	resolved, err = manager.Resolve("echo", "", 0, 0)
	if err != nil {
		t.Fatalf("Resolve echo returned error: %v", err)
	}
	if resolved.Model != "echo-v1" {
		t.Fatalf("expected echo-v1 model, got %s", resolved.Model)
	}

	if _, err := manager.Resolve("echo", "not-echo", 0, 0); err == nil {
		t.Fatal("expected unsupported echo model to fail")
	}
}

func TestManagerRunCheckClassifiesConfigAndSuccess(t *testing.T) {
	dispatcher := llm.NewDispatcher()
	openAIProvider := &providerTestLLM{
		name: llm.OpenAICompatibleProviderName,
		complete: func(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
			return llm.ProviderResponse{
				Output:       "ok",
				FinishReason: "stop",
				Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			}, nil
		},
	}
	dispatcher.RegisterProvider(openAIProvider)

	manager := NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "openai_compatible",
			OpenAICompatible: config.OpenAICompatibleProviderConfig{
				BaseURL: "https://example.com",
				APIKey:  "secret",
				Model:   "gpt-5.4",
			},
		},
	}, dispatcher)

	passed, err := manager.RunCheck(context.Background(), llm.OpenAICompatibleProviderName, "provider_check_1", CheckInput{})
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if passed.Status != CheckStatusPassed {
		t.Fatalf("expected passed status, got %s", passed.Status)
	}
	if passed.Model != "gpt-5.4" {
		t.Fatalf("expected resolved model gpt-5.4, got %s", passed.Model)
	}

	missingConfig := NewManager(config.Config{}, dispatcher)
	failed, err := missingConfig.RunCheck(context.Background(), llm.OpenAICompatibleProviderName, "provider_check_2", CheckInput{})
	if err == nil {
		t.Fatal("expected missing config check to fail")
	}
	if failed.Status != CheckStatusFailed {
		t.Fatalf("expected failed status, got %s", failed.Status)
	}
	if failed.ErrorClass != CheckErrorClassConfig {
		t.Fatalf("expected config_error, got %s", failed.ErrorClass)
	}
}

func TestManagerManagedProfilesAndDefaultModelPreference(t *testing.T) {
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&providerTestLLM{name: "claude_managed"})

	registry := managedRegistryStub{
		bridges: []ManagedBridge{
			managedBridgeStub{
				providerID:  "claude_managed",
				displayName: "Claude Code",
				family:      FamilyClaudeCodeCLI,
				authMode:    AuthModeLocalCLIBridge,
				state: AuthState{
					ProviderID:    "claude_managed",
					Family:        FamilyClaudeCodeCLI,
					AuthMode:      AuthModeLocalCLIBridge,
					Status:        AuthStatusAuthenticated,
					CLIAvailable:  true,
					AccountLabel:  "Anthropic",
					LastCheckedAt: time.Now().UTC(),
				},
				models: []Model{
					{ProviderID: "claude_managed", ModelID: "claude-opus-4-6", DisplayName: "claude-opus-4-6", Default: true, Available: true, Source: "builtin", Chat: true, Stream: true, Coding: true},
					{ProviderID: "claude_managed", ModelID: "claude-sonnet-4-6", DisplayName: "claude-sonnet-4-6", Available: true, Source: "builtin", Chat: true, Stream: true, Coding: true},
				},
			},
		},
	}
	manager := NewManager(config.Config{}, dispatcher, registry)
	manager.RestoreManagedAuthStates([]AuthState{registry.bridges[0].(managedBridgeStub).state})
	manager.RestoreProviderModels(registry.bridges[0].(managedBridgeStub).models)

	profile, ok := manager.GetProfile("claude_managed")
	if !ok {
		t.Fatal("expected managed provider profile")
	}
	if profile.Family != FamilyClaudeCodeCLI {
		t.Fatalf("expected claude managed family, got %s", profile.Family)
	}
	if !profile.Ready {
		t.Fatal("expected managed provider to be ready")
	}
	if profile.DefaultModel != "claude-opus-4-6" {
		t.Fatalf("expected default model from model catalog, got %s", profile.DefaultModel)
	}

	preference, err := manager.SetDefaultModel("claude_managed", "claude-sonnet-4-6")
	if err != nil {
		t.Fatalf("SetDefaultModel returned error: %v", err)
	}
	if preference.DefaultModel != "claude-sonnet-4-6" {
		t.Fatalf("expected updated default model, got %s", preference.DefaultModel)
	}
	resolved, err := manager.Resolve("claude_managed", "", 0, 0)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if resolved.Model != "claude-sonnet-4-6" {
		t.Fatalf("expected preferred model to resolve, got %s", resolved.Model)
	}
}

func TestManagerManagedAuthLifecycle(t *testing.T) {
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&providerTestLLM{name: "codex_managed"})
	registry := managedRegistryStub{
		bridges: []ManagedBridge{
			managedBridgeStub{
				providerID:  "codex_managed",
				displayName: "Codex CLI",
				family:      FamilyCodexCLI,
				authMode:    AuthModeLocalCLIBridge,
				startState: AuthState{
					ProviderID:    "codex_managed",
					Family:        FamilyCodexCLI,
					AuthMode:      AuthModeLocalCLIBridge,
					Status:        AuthStatusPendingLogin,
					CLIAvailable:  true,
					LastCheckedAt: time.Now().UTC(),
				},
				completeState: AuthState{
					ProviderID:    "codex_managed",
					Family:        FamilyCodexCLI,
					AuthMode:      AuthModeLocalCLIBridge,
					Status:        AuthStatusAuthenticated,
					CLIAvailable:  true,
					AccountLabel:  "user@example.com",
					LastCheckedAt: time.Now().UTC(),
				},
				models: []Model{
					{ProviderID: "codex_managed", ModelID: "gpt-5.4", DisplayName: "gpt-5.4", Default: true, Available: true, Source: "cache", Chat: true, Stream: true, Coding: true},
				},
			},
		},
	}
	manager := NewManager(config.Config{}, dispatcher, registry)

	state, models, err := manager.StartManagedAuth(context.Background(), "codex_managed")
	if err != nil {
		t.Fatalf("StartManagedAuth returned error: %v", err)
	}
	if state.Status != AuthStatusPendingLogin {
		t.Fatalf("expected pending_login, got %s", state.Status)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}

	state, _, err = manager.CompleteManagedAuth(context.Background(), "codex_managed")
	if err != nil {
		t.Fatalf("CompleteManagedAuth returned error: %v", err)
	}
	if state.Status != AuthStatusAuthenticated {
		t.Fatalf("expected authenticated status, got %s", state.Status)
	}

	if _, _, err := manager.StartManagedAuth(context.Background(), "openai_compatible"); !errors.Is(err, ErrManagedAuthUnsupported) {
		t.Fatalf("expected managed auth unsupported error, got %v", err)
	}
}

type providerTestLLM struct {
	name     string
	complete func(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error)
}

func (p *providerTestLLM) Name() string { return p.name }

func (p *providerTestLLM) Complete(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	if p.complete == nil {
		return llm.ProviderResponse{}, errors.New("complete not implemented")
	}
	return p.complete(ctx, request)
}

func (p *providerTestLLM) Stream(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	if p.complete == nil {
		return llm.ProviderResponse{}, errors.New("stream not implemented")
	}
	return p.complete(ctx, request)
}

type managedRegistryStub struct {
	bridges []ManagedBridge
}

func (r managedRegistryStub) List() []ManagedBridge {
	return append([]ManagedBridge(nil), r.bridges...)
}

func (r managedRegistryStub) Get(providerID string) (ManagedBridge, bool) {
	for _, bridge := range r.bridges {
		if bridge.ProviderID() == providerID {
			return bridge, true
		}
	}
	return nil, false
}

type managedBridgeStub struct {
	providerID    string
	displayName   string
	family        Family
	authMode      AuthMode
	state         AuthState
	startState    AuthState
	completeState AuthState
	refreshState  AuthState
	revokeState   AuthState
	models        []Model
}

func (b managedBridgeStub) ProviderID() string      { return b.providerID }
func (b managedBridgeStub) DisplayName() string     { return b.displayName }
func (b managedBridgeStub) Family() Family          { return b.family }
func (b managedBridgeStub) AuthMode() AuthMode      { return b.authMode }
func (b managedBridgeStub) Provider() llm.Provider  { return &providerTestLLM{name: b.providerID, complete: func(context.Context, llm.ProviderRequest) (llm.ProviderResponse, error) { return llm.ProviderResponse{Output: "ok"}, nil }} }
func (b managedBridgeStub) Detect(context.Context) (AuthState, []Model, error) {
	return b.state, cloneModels(b.models), nil
}
func (b managedBridgeStub) Start(context.Context) (AuthState, []Model, error) {
	return b.startState, cloneModels(b.models), nil
}
func (b managedBridgeStub) Complete(context.Context) (AuthState, []Model, error) {
	return b.completeState, cloneModels(b.models), nil
}
func (b managedBridgeStub) Refresh(context.Context) (AuthState, []Model, error) {
	if b.refreshState.ProviderID != "" {
		return b.refreshState, cloneModels(b.models), nil
	}
	return b.completeState, cloneModels(b.models), nil
}
func (b managedBridgeStub) Revoke(context.Context) (AuthState, []Model, error) {
	if b.revokeState.ProviderID != "" {
		return b.revokeState, cloneModels(b.models), nil
	}
	return AuthState{ProviderID: b.providerID, Family: b.family, AuthMode: b.authMode, Status: AuthStatusRevoked, CLIAvailable: true, LastCheckedAt: time.Now().UTC()}, cloneModels(b.models), nil
}
