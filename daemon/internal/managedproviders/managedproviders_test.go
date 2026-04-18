package managedproviders

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
)

func TestClaudeBridgeDetectLoginRequired(t *testing.T) {
	homeDir := t.TempDir()
	bridge := newClaudeBridge(homeDir, config.Config{LLM: config.LLMConfig{Claude: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/claude"}}}, runnerStub{
		run: func(_ context.Context, cmd string, args []string, workdir string) (RunResult, error) {
			return RunResult{Stdout: `{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty"}`}, nil
		},
	})

	state, models, err := bridge.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if state.Status != providers.AuthStatusLoginRequired {
		t.Fatalf("expected login_required, got %s", state.Status)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 built-in models, got %d", len(models))
	}
}

func TestClaudeProviderMapsAuthFailure(t *testing.T) {
	homeDir := t.TempDir()
	bridge := newClaudeBridge(homeDir, config.Config{LLM: config.LLMConfig{Claude: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/claude"}}}, runnerStub{
		run: func(_ context.Context, cmd string, args []string, workdir string) (RunResult, error) {
			return RunResult{Stdout: `{"is_error":true,"result":"Not logged in · Please run /login"}`}, nil
		},
	})

	_, err := bridge.Provider().Complete(context.Background(), llm.ProviderRequest{
		Model:    "claude-opus-4-6",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	var providerErr *llm.ProviderError
	if err == nil || !errors.As(err, &providerErr) {
		t.Fatalf("expected provider error, got %v", err)
	}
	if providerErr.Code != "upstream_auth_failed" {
		t.Fatalf("expected auth failure code, got %s", providerErr.Code)
	}
}

func TestCodexBridgeDetectAndModelCatalog(t *testing.T) {
	homeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeDir, ".codex"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	authPayload := map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"account_id":   "acct_1",
			"access_token": "header.payload.sig",
		},
		"last_refresh": time.Now().UTC().Format(time.RFC3339Nano),
	}
	modelsPayload := map[string]any{
		"models": []map[string]any{
			{
				"slug":         "gpt-5.4",
				"display_name": "GPT-5.4",
				"description":  "Primary coding model",
				"supported_reasoning_levels": []map[string]any{
					{"effort": "medium"},
					{"effort": "high"},
				},
			},
		},
	}
	writeJSONFile(t, filepath.Join(homeDir, ".codex", "auth.json"), authPayload)
	writeJSONFile(t, filepath.Join(homeDir, ".codex", "models_cache.json"), modelsPayload)
	if err := os.WriteFile(filepath.Join(homeDir, ".codex", "config.toml"), []byte(`model = "gpt-5.4"`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	bridge := newCodexBridge(homeDir, config.Config{LLM: config.LLMConfig{Codex: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/codex"}}}, runnerStub{})
	state, models, err := bridge.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if state.Status != providers.AuthStatusAuthenticated {
		t.Fatalf("expected authenticated, got %s", state.Status)
	}
	if len(models) != 1 || models[0].ModelID != "gpt-5.4" {
		t.Fatalf("unexpected models: %+v", models)
	}
	if !models[0].Default {
		t.Fatal("expected cached model to be default")
	}
	if len(models[0].ReasoningLevels) != 2 {
		t.Fatalf("expected reasoning levels, got %+v", models[0].ReasoningLevels)
	}
}

func TestCodexProviderReadsCLIOutputFile(t *testing.T) {
	homeDir := t.TempDir()
	bridge := newCodexBridge(homeDir, config.Config{LLM: config.LLMConfig{Codex: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/codex"}}}, runnerStub{
		run: func(_ context.Context, cmd string, args []string, workdir string) (RunResult, error) {
			outputPath := ""
			for index, arg := range args {
				if arg == "-o" && index+1 < len(args) {
					outputPath = args[index+1]
				}
			}
			if outputPath == "" {
				t.Fatal("expected output path arg")
			}
			if err := os.WriteFile(outputPath, []byte("codex reply"), 0o644); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			return RunResult{Stdout: "ok"}, nil
		},
	})

	response, err := bridge.Provider().Complete(context.Background(), llm.ProviderRequest{
		Model:    "gpt-5.4",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if response.Output != "codex reply" {
		t.Fatalf("expected codex reply, got %q", response.Output)
	}
}

type runnerStub struct {
	run func(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error)
}

func (r runnerStub) Run(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error) {
	if r.run == nil {
		return RunResult{}, nil
	}
	return r.run(ctx, cmd, args, workdir)
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
