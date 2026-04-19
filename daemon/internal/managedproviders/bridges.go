package managedproviders

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
)

type Bridge interface {
	ProviderID() string
	DisplayName() string
	Family() providers.Family
	AuthMode() providers.AuthMode
	Detect(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Start(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Complete(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Refresh(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Revoke(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Provider() llm.Provider
}

type Runner interface {
	Run(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error)
}

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type RunError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *RunError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return e.Code
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error) {
	command := exec.CommandContext(ctx, cmd, args...)
	if workdir != "" {
		command.Dir = workdir
	}
	output, err := command.CombinedOutput()
	result := RunResult{
		Stdout: strings.TrimSpace(string(output)),
		Stderr: strings.TrimSpace(string(output)),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, err
	}
	return result, err
}

type sandboxRunner struct {
	manager    *sandbox.Manager
	profileID  string
	providerID string
	roots      []string
}

func (r sandboxRunner) Run(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error) {
	if r.manager == nil {
		return execRunner{}.Run(ctx, cmd, args, workdir)
	}
	request := sandbox.ExecutionRequest{
		ProfileID:    r.profileID,
		Command:      cmd,
		Args:         append([]string(nil), args...),
		Cwd:          workdir,
		RequestedBy:  "managed_provider:" + r.providerID,
		ResourceKind: "provider",
		ResourceID:   r.providerID,
		Scope:        "managed_provider",
		Reason:       "managed provider bridge execution",
		Access: sandbox.AccessRequest{
			ReadRoots:     cloneRoots(r.roots),
			WriteRoots:    cloneRoots(r.roots),
			NetworkMode:   sandbox.NetworkModeFull,
			AllowedHosts:  []string{},
			AllowedPorts:  []int{},
			AllowLoopback: true,
		},
	}
	execution, err := r.manager.StartExecution(ctx, request)
	if err != nil {
		return RunResult{}, err
	}
	execution, err = r.manager.WaitExecution(ctx, execution.ExecutionID)
	if err != nil {
		return RunResult{}, err
	}
	result := RunResult{
		Stdout: strings.TrimSpace(execution.Result.Stdout),
		Stderr: strings.TrimSpace(execution.Result.Stderr),
	}
	if execution.Result.ExitCode != nil {
		result.ExitCode = *execution.Result.ExitCode
	}
	switch execution.Status {
	case sandbox.ExecutionStatusCompleted:
		return result, nil
	case sandbox.ExecutionStatusFailed:
		if execution.Result.ErrorClass == sandbox.ErrorClassProcessFailed {
			return result, errors.New(firstNonEmpty(execution.Result.Stderr, execution.Result.Stdout, execution.Result.Error, "sandbox process failed"))
		}
		return result, &RunError{
			Code:      firstNonEmpty(execution.Result.ErrorCode, "sandbox_execution_failed"),
			Message:   firstNonEmpty(execution.Result.Error, execution.Result.Stderr, execution.Result.Stdout, "sandbox execution failed"),
			Retryable: execution.Result.ErrorClass == sandbox.ErrorClassTimeout,
		}
	case sandbox.ExecutionStatusCancelled:
		return result, &RunError{
			Code:      firstNonEmpty(execution.Result.ErrorCode, "sandbox_cancelled"),
			Message:   firstNonEmpty(execution.Result.Error, "sandbox execution was cancelled"),
			Retryable: false,
		}
	case sandbox.ExecutionStatusDenied:
		return result, &RunError{
			Code:      firstNonEmpty(execution.Result.ErrorCode, "sandbox_policy_denied"),
			Message:   firstNonEmpty(execution.Result.Error, "sandbox execution was denied"),
			Retryable: false,
		}
	default:
		return result, &RunError{
			Code:      "sandbox_unknown_status",
			Message:   "sandbox execution returned unexpected status",
			Retryable: false,
		}
	}
}

type Registry struct {
	bridges map[string]providers.ManagedBridge
	order   []string
}

func NewRegistry(cfg config.Config, sandboxes *sandbox.Manager) *Registry {
	homeDir, _ := os.UserHomeDir()
	registry := &Registry{
		bridges: make(map[string]providers.ManagedBridge),
	}

	claudeWorkDir := firstNonEmpty(resolvePath(homeDir, cfg.LLM.Claude.WorkDir), homeFallbackWorkdir(homeDir))
	codexWorkDir := firstNonEmpty(resolvePath(homeDir, cfg.LLM.Codex.WorkDir), homeFallbackWorkdir(homeDir))

	claudeRunner := Runner(execRunner{})
	codexRunner := Runner(execRunner{})
	if sandboxes != nil {
		claudeRunner = sandboxRunner{
			manager:    sandboxes,
			profileID:  sandbox.ProfileIDManagedProviderClaude,
			providerID: ClaudeProviderID,
			roots:      []string{claudeWorkDir, filepath.Join(homeDir, ".claude"), os.TempDir()},
		}
		codexRunner = sandboxRunner{
			manager:    sandboxes,
			profileID:  sandbox.ProfileIDManagedProviderCodex,
			providerID: CodexProviderID,
			roots:      []string{codexWorkDir, filepath.Join(homeDir, ".codex"), os.TempDir()},
		}
	}

	items := []providers.ManagedBridge{
		newClaudeBridge(homeDir, cfg, claudeRunner),
		newCodexBridge(homeDir, cfg, codexRunner),
	}
	for _, bridge := range items {
		registry.bridges[bridge.ProviderID()] = bridge
		registry.order = append(registry.order, bridge.ProviderID())
	}
	return registry
}

func (r *Registry) List() []providers.ManagedBridge {
	items := make([]providers.ManagedBridge, 0, len(r.order))
	for _, id := range r.order {
		items = append(items, r.bridges[id])
	}
	return items
}

func (r *Registry) Get(providerID string) (providers.ManagedBridge, bool) {
	item, ok := r.bridges[strings.TrimSpace(providerID)]
	return item, ok
}

func firstAvailablePath(explicit string, candidates ...string) string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func decodeJWTPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	raw := parts[1]
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

func homeFallbackWorkdir(homeDir string) string {
	if strings.TrimSpace(homeDir) == "" {
		return "."
	}
	return homeDir
}

func nowPtr(t time.Time) *time.Time {
	return &t
}

func resolvePath(homeDir, value string) string {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return ""
	case trimmed == "~":
		return homeDir
	case strings.HasPrefix(trimmed, "~/"):
		return filepath.Join(homeDir, strings.TrimPrefix(trimmed, "~/"))
	default:
		return trimmed
	}
}

func cloneRoots(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}
