package managedproviders

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestClaudeBridgeDetectLoginRequired(t *testing.T) {
	homeDir := t.TempDir()
	bridge := newClaudeBridge(homeDir, config.Config{LLM: config.LLMConfig{Claude: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/claude"}}}, runnerStub{
		run: func(_ context.Context, cmd string, args []string, workdir string) (RunResult, error) {
			return RunResult{Stdout: `{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty"}`}, nil
		},
	}, nil)

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
	}, nil)

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

	bridge := newCodexBridge(homeDir, config.Config{LLM: config.LLMConfig{Codex: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/codex"}}}, runnerStub{}, nil)
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
	}, nil)

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

func TestNewRegistryRoutesClaudeDetectThroughSandbox(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sandbox-backed managed provider script fixture is Unix-only")
	}
	homeDir := t.TempDir()
	cliPath := writeManagedProviderScript(t, homeDir, "claude", managedProviderScript(`
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  printf '{"loggedIn":false,"authMethod":"none","apiProvider":"firstParty"}'
  exit 0
fi
printf 'unexpected args' >&2
exit 1
`))
	sandboxes, cleanup := newSandboxManagerForManagedProviderTest(t, homeDir)
	defer cleanup()

	registry := NewRegistry(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(homeDir, "dope-data"),
		LLM: config.LLMConfig{
			Claude: config.ManagedCLIProviderConfig{
				CLIPath: cliPath,
			},
		},
	}, sandboxes)

	bridge, ok := registry.Get(ClaudeProviderID)
	if !ok {
		t.Fatal("expected claude bridge")
	}
	state, _, err := bridge.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if state.Status != providers.AuthStatusLoginRequired {
		t.Fatalf("expected login_required, got %s", state.Status)
	}

	executions := sandboxes.ListExecutions()
	if len(executions) != 1 {
		t.Fatalf("expected 1 sandbox execution, got %d", len(executions))
	}
	if executions[0].ProfileID != sandbox.ProfileIDManagedProviderClaude {
		t.Fatalf("expected claude sandbox profile, got %s", executions[0].ProfileID)
	}
	if executions[0].Metadata[managedProviderMetadataAction] != string(sandbox.ManagedProviderActionAuthStatus) {
		t.Fatalf("expected auth-status metadata, got %+v", executions[0].Metadata)
	}
}

func TestNewRegistryRoutesCodexCompleteThroughSandbox(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sandbox-backed managed provider script fixture is Unix-only")
	}
	homeDir := t.TempDir()
	cliPath := writeManagedProviderScript(t, homeDir, "codex", managedProviderScript(`
output=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then
    output="$arg"
    break
  fi
  prev="$arg"
done
if [ -z "$output" ]; then
  printf 'missing output path' >&2
  exit 1
fi
printf 'sandbox codex reply' > "$output"
`))
	sandboxes, cleanup := newSandboxManagerForManagedProviderTest(t, homeDir)
	defer cleanup()

	registry := NewRegistry(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(homeDir, "dope-data"),
		LLM: config.LLMConfig{
			Codex: config.ManagedCLIProviderConfig{
				CLIPath: cliPath,
			},
		},
	}, sandboxes)

	bridge, ok := registry.Get(CodexProviderID)
	if !ok {
		t.Fatal("expected codex bridge")
	}
	response, err := bridge.Provider().Complete(context.Background(), llm.ProviderRequest{
		Model:    "gpt-5.4",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if response.Output != "sandbox codex reply" {
		t.Fatalf("expected sandbox codex reply, got %q", response.Output)
	}

	executions := sandboxes.ListExecutions()
	if len(executions) != 1 {
		t.Fatalf("expected 1 sandbox execution, got %d", len(executions))
	}
	if executions[0].ProfileID != sandbox.ProfileIDManagedProviderCodex {
		t.Fatalf("expected codex sandbox profile, got %s", executions[0].ProfileID)
	}
	if executions[0].Metadata[managedProviderMetadataAction] != string(sandbox.ManagedProviderActionPromptExecution) {
		t.Fatalf("expected prompt-execution metadata, got %+v", executions[0].Metadata)
	}
	if got := executions[0].Result.BackendMetadata["managedProviderAction"]; got != string(sandbox.ManagedProviderActionPromptExecution) {
		t.Fatalf("expected backend metadata action, got %+v", executions[0].Result.BackendMetadata)
	}
}

func TestClaudeProviderPromptExecutionRoutesThroughSandboxWithLocalStateSummary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sandbox-backed managed provider script fixture is Unix-only")
	}
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, "dope-data")
	writeManagedProviderTextFile(t, filepath.Join(config.ManagedProviderHomeDir(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     dataDir,
	}), ".claude", "settings.json"), `{"model":"claude-sonnet-4-6"}`)
	cliPath := writeManagedProviderScript(t, homeDir, "claude", managedProviderScript(`
printf '{"is_error":false,"result":"sandbox claude reply","usage":{"input_tokens":1,"output_tokens":1}}'
	`))
	sandboxes, cleanup := newSandboxManagerForManagedProviderTest(t, homeDir)
	defer cleanup()

	registry := NewRegistry(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     dataDir,
		LLM: config.LLMConfig{
			Claude: config.ManagedCLIProviderConfig{
				CLIPath: cliPath,
			},
		},
	}, sandboxes)

	bridge, ok := registry.Get(ClaudeProviderID)
	if !ok {
		t.Fatal("expected claude bridge")
	}
	response, err := bridge.Provider().Complete(context.Background(), llm.ProviderRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if response.Output != "sandbox claude reply" {
		t.Fatalf("expected sandbox claude reply, got %q", response.Output)
	}

	executions := sandboxes.ListExecutions()
	if len(executions) != 1 {
		t.Fatalf("expected 1 sandbox execution, got %d", len(executions))
	}
	if executions[0].Metadata[managedProviderMetadataAction] != string(sandbox.ManagedProviderActionPromptExecution) {
		t.Fatalf("expected prompt execution metadata, got %+v", executions[0].Metadata)
	}
	if executions[0].Metadata[managedProviderMetadataSensitiveStates] != "settings_file" {
		t.Fatalf("expected settings summary, got %+v", executions[0].Metadata)
	}
}

func TestNewRegistryUsesManagedProviderHomeUnderDataDirInTestEnvironment(t *testing.T) {
	homeDir := t.TempDir()
	cfg := config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(homeDir, "dope-data"),
	}

	registry := NewRegistry(cfg, nil)
	bridge, ok := registry.Get(ClaudeProviderID)
	if !ok {
		t.Fatal("expected claude bridge")
	}
	claude, ok := bridge.(*claudeBridge)
	if !ok {
		t.Fatalf("expected claude bridge type, got %T", bridge)
	}
	if got, want := claude.homeDir, config.ManagedProviderHomeDir(cfg); got != want {
		t.Fatalf("expected managed provider home %s, got %s", want, got)
	}
}

func TestClaudeProviderAuthFailureProjectsToFailedSandboxExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sandbox-backed managed provider script fixture is Unix-only")
	}
	homeDir := t.TempDir()
	cliPath := writeManagedProviderScript(t, homeDir, "claude", managedProviderScript(`
printf '{"is_error":true,"result":"Not logged in · Please run /login"}'
	`))
	sandboxes, cleanup := newSandboxManagerForManagedProviderTest(t, homeDir)
	defer cleanup()

	registry := NewRegistry(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     filepath.Join(homeDir, "dope-data"),
		LLM: config.LLMConfig{
			Claude: config.ManagedCLIProviderConfig{
				CLIPath: cliPath,
			},
		},
	}, sandboxes)

	bridge, ok := registry.Get(ClaudeProviderID)
	if !ok {
		t.Fatal("expected claude bridge")
	}
	_, err := bridge.Provider().Complete(context.Background(), llm.ProviderRequest{
		Model:    "claude-opus-4-6",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	var providerErr *llm.ProviderError
	if err == nil || !errors.As(err, &providerErr) {
		t.Fatalf("expected provider error, got %v", err)
	}
	if providerErr.Code != "upstream_auth_failed" {
		t.Fatalf("expected upstream auth failure, got %s", providerErr.Code)
	}

	executions := sandboxes.ListExecutions()
	if len(executions) != 1 {
		t.Fatalf("expected 1 sandbox execution, got %d", len(executions))
	}
	if executions[0].Status != sandbox.ExecutionStatusFailed {
		t.Fatalf("expected failed sandbox execution, got %s", executions[0].Status)
	}
	if executions[0].Result.ErrorClass != sandbox.ErrorClassProviderAuth {
		t.Fatalf("expected provider auth failure class, got %+v", executions[0].Result)
	}
}

func TestCodexDetectFailsClosedWhenLocalStateEscapesDeclaration(t *testing.T) {
	homeDir := t.TempDir()
	writeJSONFile(t, filepath.Join(homeDir, ".codex", "models_cache.json"), map[string]any{
		"models": []map[string]any{{"slug": "gpt-5.4"}},
	})
	if err := os.WriteFile(filepath.Join(homeDir, ".codex", "config.toml"), []byte(`model = "gpt-5.4"`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	outsideFile, err := os.CreateTemp("", "dope-outside-auth-*.json")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	outsideAuthPath := outsideFile.Name()
	_ = outsideFile.Close()
	t.Cleanup(func() { _ = os.Remove(outsideAuthPath) })
	writeJSONFile(t, outsideAuthPath, map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"account_id":   "acct_1",
			"access_token": "secret-token",
		},
	})
	sandboxes, cleanup := newSandboxManagerForManagedProviderTest(t, homeDir)
	defer cleanup()

	bridge := newCodexBridge(homeDir, config.Config{LLM: config.LLMConfig{Codex: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/codex"}}}, runnerStub{}, sandboxes)
	bridge.authPath = outsideAuthPath

	state, _, err := bridge.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if state.Status != providers.AuthStatusError {
		t.Fatalf("expected error status, got %s", state.Status)
	}
	if state.Metadata[managedProviderMetadataFailureClass] != string(sandbox.ErrorClassPolicyDenied) {
		t.Fatalf("expected policy denial metadata, got %+v", state.Metadata)
	}
	if strings.Contains(state.Metadata[managedProviderMetadataAccessSummary], "secret-token") {
		t.Fatalf("expected redacted metadata, got %s", state.Metadata[managedProviderMetadataAccessSummary])
	}
}

func TestManagedProviderPreflightEvaluationStaysUnderHundredMilliseconds(t *testing.T) {
	homeDir := t.TempDir()
	writeManagedProviderTextFile(t, filepath.Join(homeDir, ".claude", "settings.json"), `{"model":"claude-sonnet-4-6"}`)
	writeJSONFile(t, filepath.Join(homeDir, ".codex", "models_cache.json"), map[string]any{
		"models": []map[string]any{{"slug": "gpt-5.4"}},
	})
	writeJSONFile(t, filepath.Join(homeDir, ".codex", "auth.json"), map[string]any{
		"auth_mode": "chatgpt",
		"tokens":    map[string]any{"access_token": "header.payload.sig"},
	})
	writeManagedProviderTextFile(t, filepath.Join(homeDir, ".codex", "config.toml"), `model = "gpt-5.4"`)
	sandboxes, cleanup := newSandboxManagerForManagedProviderTest(t, homeDir)
	defer cleanup()

	claude := newClaudeBridge(homeDir, config.Config{LLM: config.LLMConfig{Claude: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/claude"}}}, runnerStub{}, sandboxes)
	codex := newCodexBridge(homeDir, config.Config{LLM: config.LLMConfig{Codex: config.ManagedCLIProviderConfig{CLIPath: "/usr/bin/codex"}}}, runnerStub{}, sandboxes)

	started := time.Now()
	if _, _, err := claude.settingsEvaluation(sandbox.ManagedProviderActionPromptExecution); err != nil {
		t.Fatalf("settingsEvaluation returned error: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("expected claude preflight <=100ms, got %s", elapsed)
	}

	started = time.Now()
	if _, err := codex.authStatusEvaluation(); err != nil {
		t.Fatalf("authStatusEvaluation returned error: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("expected codex auth-status preflight <=100ms, got %s", elapsed)
	}

	started = time.Now()
	if _, err := codex.promptExecutionEvaluation(true); err != nil {
		t.Fatalf("promptExecutionEvaluation returned error: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("expected codex prompt preflight <=100ms, got %s", elapsed)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func writeManagedProviderTextFile(t *testing.T, path string, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func newSandboxManagerForManagedProviderTest(t *testing.T, homeDir string) (*sandbox.Manager, func()) {
	t.Helper()
	dataDir := filepath.Join(homeDir, "dope-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	previousHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("Setenv returned error: %v", err)
	}
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	manager := sandbox.NewManager(config.Config{
		Environment: config.EnvironmentTest,
		DataDir:     dataDir,
		LLM:         config.LLMConfig{},
	}, sqliteStore, events.NewBus(), policy.NewEngine())
	cleanup := func() {
		_ = os.Setenv("HOME", previousHome)
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}
	return manager, cleanup
}

func writeManagedProviderScript(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		path += ".cmd"
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func managedProviderScript(body string) string {
	if runtime.GOOS == "windows" {
		return "@echo off\r\n" + body + "\r\n"
	}
	return "#!/bin/sh\nset -eu\n" + body + "\n"
}
