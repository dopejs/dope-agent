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
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
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
	sandboxes       *sandbox.Manager
}

func newCodexBridge(homeDir string, cfg config.Config, runner Runner, sandboxes *sandbox.Manager) *codexBridge {
	return &codexBridge{
		homeDir:         homeDir,
		cliPath:         firstAvailablePath(cfg.LLM.Codex.CLIPath, "codex"),
		defaultModel:    strings.TrimSpace(cfg.LLM.Codex.DefaultModel),
		workDir:         resolvePath(homeDir, cfg.LLM.Codex.WorkDir),
		runner:          runner,
		authPath:        filepath.Join(homeDir, ".codex", "auth.json"),
		modelsCachePath: filepath.Join(homeDir, ".codex", "models_cache.json"),
		sandboxes:       sandboxes,
	}
}

func (b *codexBridge) ProviderID() string           { return CodexProviderID }
func (b *codexBridge) DisplayName() string          { return "Codex CLI" }
func (b *codexBridge) Family() providers.Family     { return providers.FamilyCodexCLI }
func (b *codexBridge) AuthMode() providers.AuthMode { return providers.AuthModeLocalCLIBridge }
func (b *codexBridge) Provider() llm.Provider       { return &codexCLIProvider{bridge: b} }

func (b *codexBridge) Detect(ctx context.Context) (providers.AuthState, []providers.Model, error) {
	state := b.baseState()
	now := time.Now().UTC()
	state.LastCheckedAt = now

	if strings.TrimSpace(b.cliPath) == "" {
		models := b.models(false)
		state.Status = providers.AuthStatusError
		state.LastError = "codex CLI is not installed"
		return state, models, nil
	}

	evaluation, err := b.authStatusEvaluation()
	if err != nil {
		state.Status = providers.AuthStatusError
		state.LastError = err.Error()
		state.Metadata = evaluation.Metadata
		return state, nil, nil
	}
	state.Metadata = evaluation.Metadata
	models := b.models(false)

	raw, err := os.ReadFile(b.authPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			state.Status = providers.AuthStatusLoginRequired
			state.Metadata = finalizeManagedProviderMetadata(state.Metadata, "missing_local_state")
			return state, models, nil
		}
		state.Status = providers.AuthStatusError
		state.LastError = err.Error()
		state.Metadata = finalizeManagedProviderMetadata(state.Metadata, "missing_local_state")
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
		state.Metadata = finalizeManagedProviderMetadata(state.Metadata, "provider_auth_failed")
		return state, models, nil
	}

	if strings.TrimSpace(authFile.Tokens.AccessToken) == "" {
		state.Status = providers.AuthStatusLoginRequired
		state.Metadata = finalizeManagedProviderMetadata(state.Metadata, "provider_auth_failed")
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
	evaluation, err := b.logoutEvaluation()
	if err != nil {
		state.Status = providers.AuthStatusError
		state.LastError = err.Error()
		state.LastCheckedAt = time.Now().UTC()
		state.Metadata = evaluation.Metadata
		return state, nil, nil
	}
	state.Metadata = evaluation.Metadata
	models := b.models(false)
	if strings.TrimSpace(b.cliPath) == "" {
		state.Status = providers.AuthStatusError
		state.LastError = "codex CLI is not installed"
		state.LastCheckedAt = time.Now().UTC()
		return state, models, nil
	}
	logoutOperation := b.cliOperationPlan(sandbox.ManagedProviderActionLogout, nil)
	state.Metadata = mergeStringMaps(state.Metadata, operationMetadataFromPlan(logoutOperation))
	result, err := b.runner.Run(withManagedProviderOperation(ctx, logoutOperation), b.cliPath, []string{"logout"}, b.workDir)
	if err != nil {
		state.Status = providers.AuthStatusError
		state.LastError = err.Error()
		state.Metadata = finalizeManagedProviderMetadata(state.Metadata, "process_failed")
		state.LastCheckedAt = time.Now().UTC()
		return state, models, nil
	}
	finalizeManagedProviderExecutionSuccess(b.sandboxes, result)
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
	evaluation, err := p.bridge.promptExecutionEvaluation(model == "")
	if err != nil {
		return llm.ProviderResponse{}, classifyCLIError(&RunError{Code: "sandbox_policy_denied", Message: err.Error(), Retryable: false}, "")
	}
	localState := cloneLocalStateSummaries(evaluation.Operation.LocalStateAccessSummaries)
	if model == "" {
		model = p.bridge.resolveDefaultModel(nil)
	}
	operation := p.bridge.cliOperationPlan(sandbox.ManagedProviderActionPromptExecution, localState)
	tmpFile, err := os.CreateTemp("", "dope-codex-output-*.txt")
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	_ = tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	operation.Access.WriteRoots = cloneRoots(append(operation.Access.WriteRoots, os.TempDir(), tmpFile.Name()))
	operation.LocalState = append(operation.LocalState, localStateSummary(p.bridge.ProviderID(), sandbox.ManagedProviderActionPromptExecution, "temp_output", sandbox.LocalStateAccessModeWrite, tmpFile.Name(), false))
	operation.SensitiveKinds = localStateClassList(operation.LocalState)

	args := []string{"exec", "--skip-git-repo-check", "--sandbox", "read-only", "--model", model, "-o", tmpFile.Name(), latestUserMessage(request.Messages)}
	result, runErr := p.bridge.runner.Run(withManagedProviderOperation(ctx, operation), p.bridge.cliPath, args, p.bridge.workDir)
	if runErr != nil {
		err := classifyCLIError(runErr, result.Stdout)
		finalizeManagedProviderExecutionFailure(p.bridge.sandboxes, result, err)
		return llm.ProviderResponse{}, err
	}

	raw, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		finalizeManagedProviderExecutionFailure(p.bridge.sandboxes, result, &llm.ProviderError{
			Code:      "provider_error",
			Message:   err.Error(),
			Retryable: false,
		})
		return llm.ProviderResponse{}, err
	}
	output := strings.TrimSpace(string(raw))
	if output == "" {
		err := &llm.ProviderError{
			Code:      "upstream_invalid_response",
			Message:   "codex CLI returned empty output",
			Retryable: false,
		}
		finalizeManagedProviderExecutionFailure(p.bridge.sandboxes, result, err)
		return llm.ProviderResponse{}, err
	}
	finalizeManagedProviderExecutionSuccess(p.bridge.sandboxes, result)
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

func (b *codexBridge) authStatusEvaluation() (managedProviderOperationEvaluation, error) {
	plan := managedProviderOperationPlan{
		ProviderID:  b.ProviderID(),
		Action:      sandbox.ManagedProviderActionAuthStatus,
		ProfileID:   sandbox.ProfileIDManagedProviderCodex,
		RequestedBy: managedProviderRequestedByPrefix + b.ProviderID(),
		Reason:      "managed provider local state inspection",
		DeclaredRead: []string{
			filepath.Join(b.homeDir, ".codex", "auth.json"),
			filepath.Join(b.homeDir, ".codex", "models_cache.json"),
			filepath.Join(b.homeDir, ".codex", "config.toml"),
		},
		Access: sandbox.AccessRequest{
			ReadRoots:     []string{b.authPath, b.modelsCachePath, filepath.Join(b.homeDir, ".codex", "config.toml")},
			WriteRoots:    []string{},
			NetworkMode:   sandbox.NetworkModeDeny,
			AllowedHosts:  []string{},
			AllowedPorts:  []int{},
			AllowLoopback: false,
		},
		LocalState: []sandbox.SensitiveLocalStateAccessSummary{
			localStateSummary(b.ProviderID(), sandbox.ManagedProviderActionAuthStatus, "auth_file", sandbox.LocalStateAccessModeRead, b.authPath, true),
			localStateSummary(b.ProviderID(), sandbox.ManagedProviderActionAuthStatus, "models_cache", sandbox.LocalStateAccessModeRead, b.modelsCachePath, false),
			localStateSummary(b.ProviderID(), sandbox.ManagedProviderActionAuthStatus, "config_file", sandbox.LocalStateAccessModeRead, filepath.Join(b.homeDir, ".codex", "config.toml"), false),
		},
		SensitiveKinds: []string{"auth_file", "models_cache", "config_file"},
	}
	evaluation, err := evaluateManagedProviderOperation(b.sandboxes, plan)
	if err != nil {
		return managedProviderOperationEvaluation{}, err
	}
	if evaluation.Operation.Decision != sandbox.DecisionResolutionAllow {
		evaluation.Metadata = finalizeManagedProviderMetadata(evaluation.Metadata, string(sandbox.ErrorClassPolicyDenied))
		return evaluation, errors.New("sandbox denied managed provider local state access")
	}
	return evaluation, nil
}

func (b *codexBridge) promptExecutionEvaluation(includeConfig bool) (managedProviderOperationEvaluation, error) {
	readRoots := []string{os.TempDir()}
	localState := []sandbox.SensitiveLocalStateAccessSummary{
		localStateSummary(b.ProviderID(), sandbox.ManagedProviderActionPromptExecution, "temp_output", sandbox.LocalStateAccessModeWrite, os.TempDir(), false),
	}
	sensitiveKinds := []string{"temp_output"}
	if includeConfig {
		readRoots = append(readRoots, filepath.Join(b.homeDir, ".codex", "config.toml"))
		localState = append(localState, localStateSummary(b.ProviderID(), sandbox.ManagedProviderActionPromptExecution, "config_file", sandbox.LocalStateAccessModeRead, filepath.Join(b.homeDir, ".codex", "config.toml"), false))
		sensitiveKinds = append(sensitiveKinds, "config_file")
	}
	plan := managedProviderOperationPlan{
		ProviderID:  b.ProviderID(),
		Action:      sandbox.ManagedProviderActionPromptExecution,
		ProfileID:   sandbox.ProfileIDManagedProviderCodex,
		RequestedBy: managedProviderRequestedByPrefix + b.ProviderID(),
		Reason:      "managed provider local state inspection",
		DeclaredRead: func() []string {
			items := []string{os.TempDir()}
			if includeConfig {
				items = append(items, filepath.Join(b.homeDir, ".codex", "config.toml"))
			}
			return items
		}(),
		DeclaredWrite: []string{os.TempDir()},
		Access: sandbox.AccessRequest{
			ReadRoots:     readRoots,
			WriteRoots:    []string{os.TempDir()},
			NetworkMode:   sandbox.NetworkModeDeny,
			AllowedHosts:  []string{},
			AllowedPorts:  []int{},
			AllowLoopback: false,
		},
		LocalState:     localState,
		SensitiveKinds: sensitiveKinds,
	}
	evaluation, err := evaluateManagedProviderOperation(b.sandboxes, plan)
	if err != nil {
		return managedProviderOperationEvaluation{}, err
	}
	if evaluation.Operation.Decision != sandbox.DecisionResolutionAllow {
		evaluation.Metadata = finalizeManagedProviderMetadata(evaluation.Metadata, string(sandbox.ErrorClassPolicyDenied))
		return evaluation, errors.New("sandbox denied managed provider local state access")
	}
	return evaluation, nil
}

func (b *codexBridge) logoutEvaluation() (managedProviderOperationEvaluation, error) {
	plan := managedProviderOperationPlan{
		ProviderID:  b.ProviderID(),
		Action:      sandbox.ManagedProviderActionLogout,
		ProfileID:   sandbox.ProfileIDManagedProviderCodex,
		RequestedBy: managedProviderRequestedByPrefix + b.ProviderID(),
		Reason:      "managed provider local state inspection",
		DeclaredRead: []string{
			filepath.Join(b.homeDir, ".codex", "models_cache.json"),
			filepath.Join(b.homeDir, ".codex", "config.toml"),
		},
		Access: sandbox.AccessRequest{
			ReadRoots:     []string{b.modelsCachePath, filepath.Join(b.homeDir, ".codex", "config.toml")},
			WriteRoots:    []string{},
			NetworkMode:   sandbox.NetworkModeDeny,
			AllowedHosts:  []string{},
			AllowedPorts:  []int{},
			AllowLoopback: false,
		},
		LocalState: []sandbox.SensitiveLocalStateAccessSummary{
			localStateSummary(b.ProviderID(), sandbox.ManagedProviderActionLogout, "models_cache", sandbox.LocalStateAccessModeRead, b.modelsCachePath, false),
			localStateSummary(b.ProviderID(), sandbox.ManagedProviderActionLogout, "config_file", sandbox.LocalStateAccessModeRead, filepath.Join(b.homeDir, ".codex", "config.toml"), false),
		},
		SensitiveKinds: []string{"models_cache", "config_file"},
	}
	evaluation, err := evaluateManagedProviderOperation(b.sandboxes, plan)
	if err != nil {
		return managedProviderOperationEvaluation{}, err
	}
	if evaluation.Operation.Decision != sandbox.DecisionResolutionAllow {
		evaluation.Metadata = finalizeManagedProviderMetadata(evaluation.Metadata, string(sandbox.ErrorClassPolicyDenied))
		return evaluation, errors.New("sandbox denied managed provider local state access")
	}
	return evaluation, nil
}

func (b *codexBridge) cliOperationPlan(action sandbox.ManagedProviderActionKind, localState []sandbox.SensitiveLocalStateAccessSummary) managedProviderOperationPlan {
	return managedProviderOperationPlan{
		OperationID: newManagedProviderOperationID(),
		ProviderID:  b.ProviderID(),
		Action:      action,
		ProfileID:   sandbox.ProfileIDManagedProviderCodex,
		RequestedBy: managedProviderRequestedByPrefix + b.ProviderID(),
		Reason:      "managed provider bridge execution",
		Access: sandbox.AccessRequest{
			ReadRoots:     cloneRoots([]string{b.workDir, filepath.Join(b.homeDir, ".codex"), os.TempDir()}),
			WriteRoots:    cloneRoots([]string{b.workDir, filepath.Join(b.homeDir, ".codex"), os.TempDir()}),
			NetworkMode:   sandbox.NetworkModeFull,
			AllowedHosts:  []string{},
			AllowedPorts:  []int{},
			AllowLoopback: true,
		},
		LocalState:     cloneLocalStateSummaries(localState),
		SensitiveKinds: localStateClassList(localState),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
