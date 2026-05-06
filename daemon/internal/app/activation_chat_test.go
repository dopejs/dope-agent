package app

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
)

func TestActivationChatRunnerUsesBuiltinEchoWithoutExternalProviderCredentials(t *testing.T) {
	dispatcher := llm.NewDispatcher()
	providerManager := providers.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		LLM: config.LLMConfig{
			DefaultProvider: llm.OpenAICompatibleProviderName,
			DefaultModel:    "gpt-5.4",
		},
	}, dispatcher)
	runner := activationChatRunner{service: chat.NewService(dispatcher, providerManager, nil, nil, nil)}

	result, err := runner.RunActivationTestChat(context.Background(), activation.TestChatInput{
		ActivationID:     "act_echo",
		PrincipalID:      "prn_echo",
		TenantID:         "ten_echo",
		EnvironmentScope: "test",
		Message:          "Run a safe hosted activation test.",
	})
	if err != nil {
		t.Fatalf("RunActivationTestChat returned error without external credentials: %v", err)
	}
	if result.Status != activation.TestChatStatusCompleted || result.Provider != "echo" || result.Model != "echo-v1" {
		t.Fatalf("expected builtin echo completion, got %#v", result)
	}
}
