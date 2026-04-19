package managedproviders

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
)

const CodexProviderID = "codex_managed"

type codexBridge struct {
	homeDir         string
	cliPath         string
	defaultModel    string
	workDir         string
	runner          Runner
	authPath        string
	modelsCachePath string
}

func newCodexBridge(homeDir string, cfg config.Config, runner Runner) *codexBridge {
	return &codexBridge{
		homeDir:         homeDir,
		cliPath:         firstAvailablePath(cfg.LLM.Codex.CLIPath, "codex"),
		defaultModel:    strings.TrimSpace(cfg.LLM.Codex.DefaultModel),
		workDir:         resolvePath(homeDir, cfg.LLM.Codex.WorkDir),
		runner:          runner,
		authPath:        filepath.Join(homeDir, ".codex", "auth.json"),
		modelsCachePath: filepath.Join(homeDir, ".codex", "models_cache.json"),
	}
}

func (b *codexBridge) ProviderID() string           { return CodexProviderID }
func (b *codexBridge) DisplayName() string          { return "Codex CLI" }
func (b *codexBridge) Family() providers.Family     { return providers.FamilyCodexCLI }
func (b *codexBridge) AuthMode() providers.AuthMode { return providers.AuthModeLocalCLIBridge }
func (b *codexBridge) Provider() llm.Provider       { return &codexCLIProvider{bridge: b} }

func (b *codexBridge) Detect(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	state := b.baseState()
	models := b.models(false)
	now := time.Now().UTC()
	state.LastCheckedAt = now

	if strings.TrimSpace(b.cliPath) == "" {
		state.Status = providers.AuthStatusError
		state.LastError = "codex CLI is not installed"
		return state, models, nil
	}

	raw, err := os.ReadFile(b.authPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			state.Status = providers.AuthStatusLoginRequired
			return state, models, nil
		}
		state.Status = providers.AuthStatusError
		state.LastError = err.Error()
		return state, models, nil
	}

	var authFile struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccountID    string `json:"account_id"`
			AccessToken  string `json:"access_token"`
			IDToken      string `json:"id_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
		LastRefresh string `json:"last_refresh"`
	}
	if err := json.Unmarshal(raw, &authFile); err != nil {
		state.Status = providers.AuthStatusError
		state.LastError = err.Error()
		return state, models, nil
	}

	if strings.TrimSpace(authFile.Tokens.AccessToken) == "" {
		state.Status = providers.AuthStatusLoginRequired
		return state, models, nil
	}

	state.Status = providers.AuthStatusAuthenticated
	state.AuthMethod = authFile.AuthMode
	state.AccountID = authFile.Tokens.AccountID
	state.AccountLabel = "ChatGPT"
	if claims := decodeJWTPayload(firstNonEmpty(authFile.Tokens.IDToken, authFile.Tokens.AccessToken)); claims != nil {
		if email, ok := claims["email"].(string); ok {
			state.AccountLabel = email
		}
		if authMeta, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
			if plan, ok := authMeta["chatgpt_plan_type"].(string); ok {
				state.Plan = plan
			}
		}
	}
	if strings.TrimSpace(authFile.LastRefresh) != "" {
		if parsed, parseErr := time.Parse(time.RFC3339Nano, authFile.LastRefresh); parseErr == nil {
			state.LastAuthenticatedAt = nowPtr(parsed.UTC())
		}
	}

	models = b.models(true)
	defaultModel := b.resolveDefaultModel(models)
	for index := range models {
		models[index].Default = models[index].ModelID == defaultModel
	}
	return state, models, nil
}

func (b *codexBridge) Start(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	state, models, err := b.Detect(ctx)
	if err != nil {
		return state, models, err
	}
	state.Status = providers.AuthStatusPendingLogin
	state.LastCheckedAt = time.Now().UTC()
	return state, models, nil
}

func (b *codexBridge) Complete(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	return b.Detect(ctx)
}

func (b *codexBridge) Refresh(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	return b.Detect(ctx)
}

func (b *codexBridge) Revoke(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	state := b.baseState()
	models := b.models(false)
	if strings.TrimSpace(b.cliPath) == "" {
		state.Status = providers.AuthStatusError
		state.LastError = "codex CLI is not installed"
		state.LastCheckedAt = time.Now().UTC()
		return state, models, nil
	}
	_, err := b.runner.Run(ctx, b.cliPath, []string{"logout"}, b.workDir)
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

func (b *codexBridge) baseState() providers.AuthState {
	return providers.AuthState{
		ProviderID:    b.ProviderID(),
		Family:        b.Family(),
		AuthMode:      b.AuthMode(),
		Status:        providers.AuthStatusUnknown,
		CLIPath:       b.cliPath,
		CLIAvailable:  strings.TrimSpace(b.cliPath) != "",
		LoginCommand:  []string{baseName(b.cliPath), "login"},
		LogoutCommand: []string{baseName(b.cliPath), "logout"},
	}
}

func (b *codexBridge) models(available bool) []providers.Model {
	items := []providers.Model{}
	raw, err := os.ReadFile(b.modelsCachePath)
	if err == nil {
		var payload struct {
			Models []struct {
				Slug                     string `json:"slug"`
				DisplayName              string `json:"display_name"`
				Description              string `json:"description"`
				SupportedReasoningLevels []struct {
					Effort string `json:"effort"`
				} `json:"supported_reasoning_levels"`
			} `json:"models"`
		}
		if json.Unmarshal(raw, &payload) == nil {
			for _, item := range payload.Models {
				if strings.TrimSpace(item.Slug) == "" {
					continue
				}
				model := providers.Model{
					ProviderID:  b.ProviderID(),
					ModelID:     item.Slug,
					DisplayName: firstNonEmpty(item.DisplayName, item.Slug),
					Description: item.Description,
					Source:      "cache",
					Available:   available,
					Chat:        true,
					Stream:      true,
					Coding:      true,
					ToolUse:     false,
				}
				for _, level := range item.SupportedReasoningLevels {
					if strings.TrimSpace(level.Effort) != "" {
						model.ReasoningLevels = append(model.ReasoningLevels, level.Effort)
					}
				}
				items = append(items, model)
			}
		}
	}
	if len(items) == 0 {
		items = append(items, providers.Model{
			ProviderID:  b.ProviderID(),
			ModelID:     firstNonEmpty(b.defaultModel, "gpt-5.4"),
			DisplayName: firstNonEmpty(b.defaultModel, "gpt-5.4"),
			Source:      "fallback",
			Available:   available,
			Chat:        true,
			Stream:      true,
			Coding:      true,
			ToolUse:     false,
		})
	}
	defaultModel := b.resolveDefaultModel(items)
	for index := range items {
		items[index].Default = items[index].ModelID == defaultModel
	}
	return items
}

func (b *codexBridge) resolveDefaultModel(items []providers.Model) string {
	if strings.TrimSpace(b.defaultModel) != "" {
		return strings.TrimSpace(b.defaultModel)
	}
	if raw, err := os.ReadFile(filepath.Join(b.homeDir, ".codex", "config.toml")); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "model = ") {
				model := strings.Trim(strings.TrimPrefix(line, "model = "), "\"")
				if strings.TrimSpace(model) != "" {
					return strings.TrimSpace(model)
				}
			}
		}
	}
	if len(items) > 0 {
		return items[0].ModelID
	}
	return ""
}

type codexCLIProvider struct {
	bridge *codexBridge
}

func (p *codexCLIProvider) Name() string { return p.bridge.ProviderID() }

func (p *codexCLIProvider) Complete(ctx context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = p.bridge.resolveDefaultModel(nil)
	}
	tmpFile, err := os.CreateTemp("", "dope-codex-output-*.txt")
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	args := []string{"exec", "--skip-git-repo-check", "--sandbox", "read-only", "--model", model, "-o", tmpFile.Name(), latestUserMessage(request.Messages)}
	result, runErr := p.bridge.runner.Run(ctx, p.bridge.cliPath, args, p.bridge.workDir)
	if runErr != nil {
		return llm.ProviderResponse{}, classifyCLIError(runErr, result.Stdout)
	}

	raw, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	output := strings.TrimSpace(string(raw))
	if output == "" {
		return llm.ProviderResponse{}, &llm.ProviderError{
			Code:      "upstream_invalid_response",
			Message:   "codex CLI returned empty output",
			Retryable: false,
		}
	}
	return llm.ProviderResponse{
		Output:       output,
		FinishReason: "stop",
		Usage:        llm.Usage{},
	}, nil
}

func (p *codexCLIProvider) Stream(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
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

func classifyCLIError(runErr error, output string) error {
	var runCLIError *RunError
	if errors.As(runErr, &runCLIError) {
		return &llm.ProviderError{
			Code:      firstNonEmpty(runCLIError.Code, "provider_error"),
			Message:   firstNonEmpty(runCLIError.Message, runErr.Error()),
			Retryable: runCLIError.Retryable,
		}
	}
	message := strings.TrimSpace(output)
	if message == "" {
		message = runErr.Error()
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "not logged in"), strings.Contains(lower, "please run /login"), strings.Contains(lower, "logged in using"):
		if strings.Contains(lower, "not logged in") || strings.Contains(lower, "please run /login") {
			return &llm.ProviderError{Code: "upstream_auth_failed", Message: message, Retryable: false}
		}
	case strings.Contains(lower, "permission denied"):
		return &llm.ProviderError{Code: "upstream_transport_error", Message: message, Retryable: true}
	}
	return &llm.ProviderError{Code: "provider_error", Message: message, Retryable: false}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
