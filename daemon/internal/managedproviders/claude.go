package managedproviders

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
)

const ClaudeProviderID = "claude_managed"

type claudeBridge struct {
	homeDir      string
	cliPath      string
	defaultModel string
	workDir      string
	runner       Runner
	settingsPath string
}

func newClaudeBridge(homeDir string, cfg config.Config, runner Runner) *claudeBridge {
	return &claudeBridge{
		homeDir:      homeDir,
		cliPath:      firstAvailablePath(cfg.LLM.Claude.CLIPath, "claude"),
		defaultModel: strings.TrimSpace(cfg.LLM.Claude.DefaultModel),
		workDir:      resolvePath(homeDir, cfg.LLM.Claude.WorkDir),
		runner:       runner,
		settingsPath: filepathJoin(homeDir, ".claude", "settings.json"),
	}
}

func (b *claudeBridge) ProviderID() string           { return ClaudeProviderID }
func (b *claudeBridge) DisplayName() string          { return "Claude Code" }
func (b *claudeBridge) Family() providers.Family     { return providers.FamilyClaudeCodeCLI }
func (b *claudeBridge) AuthMode() providers.AuthMode { return providers.AuthModeLocalCLIBridge }
func (b *claudeBridge) Provider() llm.Provider       { return &claudeCLIProvider{bridge: b} }

func (b *claudeBridge) Detect(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	state := b.baseState()
	models := b.models(false)

	if strings.TrimSpace(b.cliPath) == "" {
		state.Status = providers.AuthStatusError
		state.LastError = "claude CLI is not installed"
		state.LastCheckedAt = time.Now().UTC()
		return state, models, nil
	}

	result, err := b.runner.Run(ctx, b.cliPath, []string{"auth", "status"}, b.workDir)
	now := time.Now().UTC()
	state.LastCheckedAt = now
	if err != nil {
		state.Status = providers.AuthStatusError
		state.LastError = result.Stdout
		return state, models, nil
	}

	var payload struct {
		LoggedIn    bool   `json:"loggedIn"`
		AuthMethod  string `json:"authMethod"`
		APIProvider string `json:"apiProvider"`
	}
	if parseErr := json.Unmarshal([]byte(result.Stdout), &payload); parseErr != nil {
		state.Status = providers.AuthStatusError
		state.LastError = parseErr.Error()
		return state, models, nil
	}
	state.AuthMethod = payload.AuthMethod
	state.Metadata = map[string]string{"apiProvider": payload.APIProvider}
	if payload.LoggedIn {
		state.Status = providers.AuthStatusAuthenticated
		state.AccountLabel = "Anthropic"
		state.LastAuthenticatedAt = nowPtr(now)
		for index := range models {
			models[index].Available = true
			models[index].Default = models[index].ModelID == b.resolveDefaultModel(models)
		}
		return state, models, nil
	}

	state.Status = providers.AuthStatusLoginRequired
	state.LastError = ""
	return state, models, nil
}

func (b *claudeBridge) Start(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	state, models, err := b.Detect(ctx)
	if err != nil {
		return state, models, err
	}
	state.Status = providers.AuthStatusPendingLogin
	state.LastCheckedAt = time.Now().UTC()
	return state, models, nil
}

func (b *claudeBridge) Complete(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	return b.Detect(ctx)
}

func (b *claudeBridge) Refresh(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	return b.Detect(ctx)
}

func (b *claudeBridge) Revoke(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	state := b.baseState()
	models := b.models(false)
	if strings.TrimSpace(b.cliPath) == "" {
		state.Status = providers.AuthStatusError
		state.LastError = "claude CLI is not installed"
		state.LastCheckedAt = time.Now().UTC()
		return state, models, nil
	}
	_, err := b.runner.Run(ctx, b.cliPath, []string{"auth", "logout"}, b.workDir)
	if err != nil {
		state.Status = providers.AuthStatusError
		state.LastError = err.Error()
		state.LastCheckedAt = time.Now().UTC()
		return state, models, nil
	}
	state.Status = providers.AuthStatusRevoked
	state.LastCheckedAt = time.Now().UTC()
	return state, models, nil
}

func (b *claudeBridge) baseState() providers.AuthState {
	return providers.AuthState{
		ProviderID:    b.ProviderID(),
		Family:        b.Family(),
		AuthMode:      b.AuthMode(),
		Status:        providers.AuthStatusUnknown,
		CLIPath:       b.cliPath,
		CLIAvailable:  strings.TrimSpace(b.cliPath) != "",
		LoginCommand:  []string{baseName(b.cliPath), "auth", "login", "--claudeai"},
		LogoutCommand: []string{baseName(b.cliPath), "auth", "logout"},
	}
}

func (b *claudeBridge) models(available bool) []providers.Model {
	items := []providers.Model{
		{
			ProviderID:  b.ProviderID(),
			ModelID:     "claude-opus-4-6",
			DisplayName: "claude-opus-4-6",
			Description: "Claude flagship coding model",
			Source:      "builtin",
			Available:   available,
			Chat:        true,
			Stream:      true,
			Coding:      true,
			ToolUse:     false,
		},
		{
			ProviderID:  b.ProviderID(),
			ModelID:     "claude-sonnet-4-6",
			DisplayName: "claude-sonnet-4-6",
			Description: "Claude balanced coding model",
			Source:      "builtin",
			Available:   available,
			Chat:        true,
			Stream:      true,
			Coding:      true,
			ToolUse:     false,
		},
	}
	defaultModel := b.resolveDefaultModel(items)
	for index := range items {
		items[index].Default = items[index].ModelID == defaultModel
	}
	return items
}

func (b *claudeBridge) resolveDefaultModel(items []providers.Model) string {
	if strings.TrimSpace(b.defaultModel) != "" {
		return strings.TrimSpace(b.defaultModel)
	}
	raw, err := os.ReadFile(b.settingsPath)
	if err == nil {
		var settings struct {
			Model string `json:"model"`
		}
		if json.Unmarshal(raw, &settings) == nil && strings.TrimSpace(settings.Model) != "" {
			return strings.TrimSpace(settings.Model)
		}
	}
	if len(items) > 0 {
		return items[0].ModelID
	}
	return ""
}

type claudeCLIProvider struct {
	bridge *claudeBridge
}

func (p *claudeCLIProvider) Name() string { return p.bridge.ProviderID() }

func (p *claudeCLIProvider) Complete(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = p.bridge.resolveDefaultModel(nil)
	}
	args := []string{"-p", "--output-format", "json", "--model", model, "--allowedTools", "", "--permission-mode", "dontAsk", latestUserMessage(request.Messages)}
	result, err := p.bridge.runner.Run(ctx, p.bridge.cliPath, args, p.bridge.workDir)
	return parseClaudeResult(result, err)
}

func (p *claudeCLIProvider) Stream(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	response, err := p.Complete(ctx, request)
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	if emit != nil && response.Output != "" {
		if emitErr := emit(llm.StreamChunk{Delta: response.Output, Output: response.Output}); emitErr != nil {
			return llm.ProviderResponse{}, emitErr
		}
	}
	return response, nil
}

func parseClaudeResult(result RunResult, runErr error) (llm.ProviderResponse, error) {
	var payload struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &payload); err == nil {
		if payload.IsError {
			code := "provider_error"
			if strings.Contains(strings.ToLower(payload.Result), "not logged in") {
				code = "upstream_auth_failed"
			}
			return llm.ProviderResponse{}, &llm.ProviderError{
				Code:      code,
				Message:   payload.Result,
				Retryable: false,
			}
		}
		return llm.ProviderResponse{
			Output:       payload.Result,
			FinishReason: "stop",
			Usage: llm.Usage{
				InputTokens:  payload.Usage.InputTokens,
				OutputTokens: payload.Usage.OutputTokens,
				TotalTokens:  payload.Usage.InputTokens + payload.Usage.OutputTokens,
			},
		}, nil
	}

	if runErr != nil {
		return llm.ProviderResponse{}, classifyCLIError(runErr, result.Stdout)
	}
	return llm.ProviderResponse{}, fmt.Errorf("decode claude CLI response: %s", result.Stdout)
}

func latestUserMessage(messages []llm.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.TrimSpace(messages[index].Content) != "" {
			return messages[index].Content
		}
	}
	return ""
}
