package providers

import (
	"context"
	"errors"
	"testing"

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
