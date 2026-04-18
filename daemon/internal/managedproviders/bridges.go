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

type Registry struct {
	bridges map[string]providers.ManagedBridge
	order   []string
}

func NewRegistry(cfg config.Config) *Registry {
	homeDir, _ := os.UserHomeDir()
	registry := &Registry{
		bridges: make(map[string]providers.ManagedBridge),
	}

	items := []providers.ManagedBridge{
		newClaudeBridge(homeDir, cfg, execRunner{}),
		newCodexBridge(homeDir, cfg, execRunner{}),
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
