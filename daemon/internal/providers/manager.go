package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
)

var (
	ErrModelNotSupported       = errors.New("model is not supported by provider")
	ErrManagedAuthUnsupported  = errors.New("managed auth is not supported by provider")
	ErrProviderAuthUnavailable = errors.New("tenant provider auth is unavailable")
)

type Family string

const (
	FamilyBuiltinEcho      Family = "builtin_echo"
	FamilyOpenAICompatible Family = "openai_compatible"
	FamilyClaudeCodeCLI    Family = "claude_code_cli"
	FamilyCodexCLI         Family = "codex_cli"
)

type AuthMode string

const (
	AuthModeNone           AuthMode = "none"
	AuthModeAPIKey         AuthMode = "api_key"
	AuthModeLocalCLIBridge AuthMode = "local_cli_bridge"
)

type Source string

const (
	SourceBuiltin Source = "builtin"
	SourceConfig  Source = "config"
	SourceManaged Source = "managed"
)

type ModelSelectionMode string

const (
	ModelSelectionFixed ModelSelectionMode = "fixed"
	ModelSelectionOpen  ModelSelectionMode = "open"
)

type CapabilityFlags struct {
	Chat    bool `json:"chat"`
	Stream  bool `json:"stream"`
	Coding  bool `json:"coding,omitempty"`
	ToolUse bool `json:"toolUse,omitempty"`
}

type Profile struct {
	ProviderID          string             `json:"providerId"`
	Title               string             `json:"title"`
	Family              Family             `json:"family"`
	AuthMode            AuthMode           `json:"authMode"`
	Source              Source             `json:"source"`
	ModelSelectionMode  ModelSelectionMode `json:"modelSelectionMode"`
	KnownModels         []string           `json:"knownModels,omitempty"`
	Registered          bool               `json:"registered"`
	Configured          bool               `json:"configured"`
	Ready               bool               `json:"ready"`
	Default             bool               `json:"default"`
	BaseURL             string             `json:"baseURL,omitempty"`
	RequestURL          string             `json:"requestURL,omitempty"`
	DefaultModel        string             `json:"defaultModel,omitempty"`
	EffectiveModel      string             `json:"effectiveModel,omitempty"`
	EffectiveTimeoutMs  int                `json:"effectiveTimeoutMs"`
	EffectiveMaxRetries int                `json:"effectiveMaxRetries"`
	SecretConfigured    bool               `json:"secretConfigured"`
	SecretRef           string             `json:"secretRef,omitempty"`
	Capabilities        CapabilityFlags    `json:"capabilities"`
	AuthStatus          AuthStatus         `json:"authStatus,omitempty"`
	CLIAvailable        bool               `json:"cliAvailable,omitempty"`
	AccountLabel        string             `json:"accountLabel,omitempty"`
	AccountID           string             `json:"accountId,omitempty"`
	Plan                string             `json:"plan,omitempty"`
	AuthMethod          string             `json:"authMethod,omitempty"`
	LoginCommand        []string           `json:"loginCommand,omitempty"`
	LogoutCommand       []string           `json:"logoutCommand,omitempty"`
	AvailableModelCount int                `json:"availableModelCount,omitempty"`
	Issues              []string           `json:"issues,omitempty"`
}

type CheckStatus string

const (
	CheckStatusPassed CheckStatus = "passed"
	CheckStatusFailed CheckStatus = "failed"
)

type CheckErrorClass string

const (
	CheckErrorClassConfig    CheckErrorClass = "config_error"
	CheckErrorClassAuth      CheckErrorClass = "auth_error"
	CheckErrorClassTransport CheckErrorClass = "transport_error"
	CheckErrorClassUpstream  CheckErrorClass = "upstream_error"
	CheckErrorClassTimeout   CheckErrorClass = "timeout"
)

type Check struct {
	CheckID      string          `json:"checkId"`
	ProviderID   string          `json:"providerId"`
	Family       Family          `json:"family"`
	AuthMode     AuthMode        `json:"authMode"`
	Status       CheckStatus     `json:"status"`
	Model        string          `json:"model"`
	Endpoint     string          `json:"endpoint,omitempty"`
	ErrorClass   CheckErrorClass `json:"errorClass,omitempty"`
	ErrorCode    string          `json:"errorCode,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Usage        llm.Usage       `json:"usage"`
	CreatedAt    time.Time       `json:"createdAt"`
	CompletedAt  time.Time       `json:"completedAt"`
}

type CheckInput struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ResolvedDispatch struct {
	ProviderID string
	Model      string
	TimeoutMs  int
	MaxRetries int
	Endpoint   string
	Profile    Profile
}

type ManagedBridge interface {
	ProviderID() string
	DisplayName() string
	Family() Family
	AuthMode() AuthMode
	Detect(ctx context.Context) (AuthState, []Model, error)
	Start(ctx context.Context) (AuthState, []Model, error)
	Complete(ctx context.Context) (AuthState, []Model, error)
	Refresh(ctx context.Context) (AuthState, []Model, error)
	Revoke(ctx context.Context) (AuthState, []Model, error)
	Provider() llm.Provider
}

type ManagedRegistry interface {
	List() []ManagedBridge
	Get(providerID string) (ManagedBridge, bool)
}

type SyncResult struct {
	State  AuthState
	Models []Model
}

type Manager struct {
	cfg         config.Config
	dispatcher  *llm.Dispatcher
	registry    ManagedRegistry
	profiles    map[string]Profile
	order       []string
	authStates  map[string]AuthState
	models      map[string][]Model
	preferences map[string]Preference
}

func NewManager(cfg config.Config, dispatcher *llm.Dispatcher, registries ...ManagedRegistry) *Manager {
	var registry ManagedRegistry
	if len(registries) > 0 {
		registry = registries[0]
	}

	manager := &Manager{
		cfg:         cfg,
		dispatcher:  dispatcher,
		registry:    registry,
		profiles:    make(map[string]Profile),
		authStates:  make(map[string]AuthState),
		models:      make(map[string][]Model),
		preferences: make(map[string]Preference),
	}
	manager.loadProfiles()
	return manager
}

func (m *Manager) ListProfiles() []Profile {
	items := make([]Profile, 0, len(m.order))
	for _, providerID := range m.order {
		items = append(items, cloneProfile(m.profiles[providerID]))
	}
	return items
}

func (m *Manager) GetProfile(providerID string) (Profile, bool) {
	profile, ok := m.profiles[strings.TrimSpace(providerID)]
	if !ok {
		return Profile{}, false
	}
	return cloneProfile(profile), true
}

func (m *Manager) GetProfileForTenant(providerID, tenantID string) (Profile, bool) {
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return Profile{}, false
	}
	if m.registry != nil {
		if bridge, ok := m.registry.Get(trimmed); ok {
			return cloneProfile(m.buildManagedProfileForTenant(bridge, tenantID)), true
		}
	}
	return m.GetProfile(trimmed)
}

func (m *Manager) GetAuthState(providerID string) (AuthState, bool) {
	state, ok := m.authStates[strings.TrimSpace(providerID)]
	if !ok {
		return AuthState{}, false
	}
	return cloneAuthState(state), true
}

func (m *Manager) GetAuthStateForTenant(providerID, tenantID string) (AuthState, bool) {
	state, ok := m.authStates[tenantAuthKey(tenantID, providerID)]
	if !ok {
		return AuthState{}, false
	}
	return cloneAuthState(state), true
}

func (m *Manager) RestoreManagedAuthStatesForTenant(tenantID string, states []AuthState) {
	for _, state := range states {
		if strings.TrimSpace(state.ProviderID) == "" {
			continue
		}
		state.TenantID = strings.TrimSpace(tenantID)
		m.authStates[tenantAuthKey(state.TenantID, state.ProviderID)] = cloneAuthState(state)
	}
	m.loadProfiles()
}

func (m *Manager) ListModels(providerID string) ([]Model, bool) {
	trimmed := strings.TrimSpace(providerID)
	if items, ok := m.models[trimmed]; ok {
		return cloneModels(items), true
	}
	profile, ok := m.profiles[trimmed]
	if !ok || len(profile.KnownModels) == 0 {
		return nil, false
	}
	items := make([]Model, 0, len(profile.KnownModels))
	for _, modelID := range profile.KnownModels {
		items = append(items, Model{
			ProviderID:  profile.ProviderID,
			ModelID:     modelID,
			DisplayName: modelID,
			Default:     modelID == profile.DefaultModel || modelID == profile.EffectiveModel,
			Available:   profile.Ready,
			Source:      string(profile.Source),
			Chat:        profile.Capabilities.Chat,
			Stream:      profile.Capabilities.Stream,
			Coding:      profile.Capabilities.Coding,
			ToolUse:     profile.Capabilities.ToolUse,
		})
	}
	return items, true
}

func (m *Manager) GetPreference(providerID string) (Preference, bool) {
	preference, ok := m.preferences[strings.TrimSpace(providerID)]
	if !ok {
		return Preference{}, false
	}
	return preference, true
}

func (m *Manager) RestoreManagedAuthStates(states []AuthState) {
	for _, state := range states {
		if strings.TrimSpace(state.ProviderID) == "" {
			continue
		}
		m.authStates[tenantAuthKey(state.TenantID, state.ProviderID)] = cloneAuthState(state)
	}
	m.loadProfiles()
}

func (m *Manager) RestoreProviderModels(items []Model) {
	grouped := make(map[string][]Model)
	for _, item := range items {
		providerID := strings.TrimSpace(item.ProviderID)
		if providerID == "" {
			continue
		}
		grouped[providerID] = append(grouped[providerID], item)
	}
	for providerID, models := range grouped {
		m.models[providerID] = cloneModels(models)
	}
	m.loadProfiles()
}

func (m *Manager) RestoreProviderPreferences(items []Preference) {
	for _, item := range items {
		if strings.TrimSpace(item.ProviderID) == "" {
			continue
		}
		m.preferences[item.ProviderID] = item
	}
	m.loadProfiles()
}

func (m *Manager) SyncManagedProviders(ctx context.Context) ([]SyncResult, error) {
	if m.registry == nil {
		return nil, nil
	}

	results := make([]SyncResult, 0, len(m.registry.List()))
	for _, bridge := range m.registry.List() {
		state, models, err := bridge.Detect(ctx)
		if err != nil {
			return nil, fmt.Errorf("detect provider %s: %w", bridge.ProviderID(), err)
		}
		m.applyManagedState(state, models)
		results = append(results, SyncResult{
			State:  cloneAuthState(state),
			Models: cloneModels(models),
		})
	}

	m.loadProfiles()
	return results, nil
}

func (m *Manager) StartManagedAuth(ctx context.Context, providerID string) (AuthState, []Model, error) {
	return m.runManagedAction(ctx, providerID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Start(ctx)
	})
}

func (m *Manager) StartManagedAuthForTenant(ctx context.Context, providerID, tenantID string) (AuthState, []Model, error) {
	return m.runManagedActionForTenant(ctx, providerID, tenantID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Start(ctx)
	})
}

func (m *Manager) CompleteManagedAuth(ctx context.Context, providerID string) (AuthState, []Model, error) {
	return m.runManagedAction(ctx, providerID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Complete(ctx)
	})
}

func (m *Manager) CompleteManagedAuthForTenant(ctx context.Context, providerID, tenantID string) (AuthState, []Model, error) {
	return m.runManagedActionForTenant(ctx, providerID, tenantID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Complete(ctx)
	})
}

func (m *Manager) RefreshManagedAuth(ctx context.Context, providerID string) (AuthState, []Model, error) {
	return m.runManagedAction(ctx, providerID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Refresh(ctx)
	})
}

func (m *Manager) RefreshManagedAuthForTenant(ctx context.Context, providerID, tenantID string) (AuthState, []Model, error) {
	return m.runManagedActionForTenant(ctx, providerID, tenantID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Refresh(ctx)
	})
}

func (m *Manager) RevokeManagedAuth(ctx context.Context, providerID string) (AuthState, []Model, error) {
	return m.runManagedAction(ctx, providerID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Revoke(ctx)
	})
}

func (m *Manager) RevokeManagedAuthForTenant(ctx context.Context, providerID, tenantID string) (AuthState, []Model, error) {
	return m.runManagedActionForTenant(ctx, providerID, tenantID, func(ctx context.Context, bridge ManagedBridge) (AuthState, []Model, error) {
		return bridge.Revoke(ctx)
	})
}

func (m *Manager) SetDefaultModel(providerID, model string) (Preference, error) {
	trimmedProviderID := strings.TrimSpace(providerID)
	trimmedModel := strings.TrimSpace(model)
	if trimmedProviderID == "" {
		return Preference{}, llm.ErrProviderRequired
	}
	if trimmedModel == "" {
		return Preference{}, llm.ErrModelRequired
	}

	profile, ok := m.GetProfile(trimmedProviderID)
	if !ok {
		return Preference{}, fmt.Errorf("%w: %s", llm.ErrProviderNotFound, trimmedProviderID)
	}
	if err := validateModel(profile, trimmedModel); err != nil {
		return Preference{}, err
	}

	preference := Preference{
		ProviderID:   trimmedProviderID,
		DefaultModel: trimmedModel,
		UpdatedAt:    time.Now().UTC(),
	}
	m.preferences[trimmedProviderID] = preference
	m.loadProfiles()
	return preference, nil
}

func (m *Manager) Resolve(providerID, model string, timeoutMs, maxRetries int) (ResolvedDispatch, error) {
	return m.resolveWithProfile(providerID, model, timeoutMs, maxRetries, m.GetProfile, false)
}

func (m *Manager) ResolveForTenant(providerID, model string, timeoutMs, maxRetries int, tenantID string) (ResolvedDispatch, error) {
	return m.resolveWithProfile(providerID, model, timeoutMs, maxRetries, func(providerID string) (Profile, bool) {
		return m.GetProfileForTenant(providerID, tenantID)
	}, true)
}

func (m *Manager) resolveWithProfile(providerID, model string, timeoutMs, maxRetries int, profileResolver func(string) (Profile, bool), requireManagedAuthReady bool) (ResolvedDispatch, error) {
	requestedProvider := strings.TrimSpace(providerID)
	effectiveProvider := requestedProvider
	if effectiveProvider == "" {
		effectiveProvider = m.defaultProviderID()
	}

	profile, ok := profileResolver(effectiveProvider)
	if !ok {
		return ResolvedDispatch{}, fmt.Errorf("%w: %s", llm.ErrProviderNotFound, effectiveProvider)
	}
	if !profile.Registered {
		return ResolvedDispatch{}, fmt.Errorf("%w: %s", llm.ErrProviderNotFound, effectiveProvider)
	}
	if requireManagedAuthReady && profile.Source == SourceManaged && !profile.Ready {
		return ResolvedDispatch{}, fmt.Errorf("%w: %s", ErrProviderAuthUnavailable, effectiveProvider)
	}

	effectiveModel := strings.TrimSpace(model)
	if effectiveModel == "" {
		configuredDefaultModel := strings.TrimSpace(m.cfg.LLM.DefaultModel)
		switch {
		case requestedProvider == "" && configuredDefaultModel != "":
			effectiveModel = configuredDefaultModel
		case strings.TrimSpace(m.cfg.LLM.DefaultProvider) == profile.ProviderID && configuredDefaultModel != "":
			effectiveModel = configuredDefaultModel
		}
	}
	if effectiveModel == "" {
		if preference, ok := m.GetPreference(profile.ProviderID); ok {
			effectiveModel = strings.TrimSpace(preference.DefaultModel)
		}
	}
	if effectiveModel == "" {
		effectiveModel = strings.TrimSpace(profile.DefaultModel)
	}
	if effectiveModel == "" {
		effectiveModel = strings.TrimSpace(profile.EffectiveModel)
	}
	if effectiveModel == "" {
		return ResolvedDispatch{}, llm.ErrModelRequired
	}

	if err := validateModel(profile, effectiveModel); err != nil {
		return ResolvedDispatch{}, err
	}

	effectiveTimeoutMs := timeoutMs
	if effectiveTimeoutMs <= 0 {
		effectiveTimeoutMs = defaultPositive(profile.EffectiveTimeoutMs, 30000)
	}

	effectiveMaxRetries := maxRetryValue(maxRetries, profile.EffectiveMaxRetries)
	return ResolvedDispatch{
		ProviderID: effectiveProvider,
		Model:      effectiveModel,
		TimeoutMs:  effectiveTimeoutMs,
		MaxRetries: effectiveMaxRetries,
		Endpoint:   profile.RequestURL,
		Profile:    profile,
	}, nil
}

func (m *Manager) ResolveDispatchInput(input llm.CreateDispatchInput) (ResolvedDispatch, llm.CreateDispatchInput, error) {
	resolved, err := m.Resolve(input.Provider, input.Model, input.TimeoutMs, input.MaxRetries)
	if err != nil {
		return ResolvedDispatch{}, llm.CreateDispatchInput{}, err
	}
	if len(input.Messages) == 0 {
		return ResolvedDispatch{}, llm.CreateDispatchInput{}, llm.ErrMessagesRequired
	}

	effective := llm.CreateDispatchInput{
		Provider:   resolved.ProviderID,
		Model:      resolved.Model,
		Messages:   cloneMessages(input.Messages),
		TimeoutMs:  resolved.TimeoutMs,
		MaxRetries: resolved.MaxRetries,
	}
	return resolved, effective, nil
}

func (m *Manager) RunCheck(ctx context.Context, providerID, checkID string, input CheckInput) (Check, error) {
	startedAt := time.Now().UTC()
	resolved, err := m.Resolve(providerID, input.Model, 0, 0)
	if err != nil {
		profile, _ := m.GetProfile(providerID)
		check := failedCheck(checkID, startedAt, profile, "", "", classifyPrepareError(err), err)
		return check, err
	}

	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		prompt = "Reply with the single word ok."
	}

	dispatchInput := llm.CreateDispatchInput{
		Provider:   resolved.ProviderID,
		Model:      resolved.Model,
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		TimeoutMs:  resolved.TimeoutMs,
		MaxRetries: resolved.MaxRetries,
	}

	dispatch, err := m.dispatcher.Prepare(dispatchInput, false)
	if err != nil {
		check := failedCheck(checkID, startedAt, resolved.Profile, resolved.Model, resolved.Endpoint, classifyPrepareError(err), err)
		return check, err
	}

	result, err := m.dispatcher.Dispatch(ctx, dispatch)
	if err != nil {
		check := failedCheck(checkID, startedAt, resolved.Profile, resolved.Model, resolved.Endpoint, classifyDispatchFailure(result.ErrorCode), err)
		return check, err
	}

	completedAt := time.Now().UTC()
	return Check{
		CheckID:     checkID,
		ProviderID:  resolved.Profile.ProviderID,
		Family:      resolved.Profile.Family,
		AuthMode:    resolved.Profile.AuthMode,
		Status:      CheckStatusPassed,
		Model:       resolved.Model,
		Endpoint:    resolved.Endpoint,
		Usage:       result.Usage,
		CreatedAt:   startedAt,
		CompletedAt: completedAt,
	}, nil
}

func NewCheckID() string {
	return "provider_check_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

func (m *Manager) runManagedAction(ctx context.Context, providerID string, action func(context.Context, ManagedBridge) (AuthState, []Model, error)) (AuthState, []Model, error) {
	return m.runManagedActionForTenant(ctx, providerID, "", action)
}

func (m *Manager) runManagedActionForTenant(ctx context.Context, providerID, tenantID string, action func(context.Context, ManagedBridge) (AuthState, []Model, error)) (AuthState, []Model, error) {
	bridge, ok := m.managedBridge(providerID)
	if !ok {
		if _, exists := m.GetProfile(providerID); exists {
			return AuthState{}, nil, ErrManagedAuthUnsupported
		}
		return AuthState{}, nil, fmt.Errorf("%w: %s", llm.ErrProviderNotFound, strings.TrimSpace(providerID))
	}
	state, models, err := action(ctx, bridge)
	if err != nil {
		return AuthState{}, nil, err
	}
	state.TenantID = strings.TrimSpace(tenantID)
	m.applyManagedState(state, models)
	m.loadProfiles()
	return cloneAuthState(state), cloneModels(models), nil
}

func (m *Manager) managedBridge(providerID string) (ManagedBridge, bool) {
	if m.registry == nil {
		return nil, false
	}
	return m.registry.Get(strings.TrimSpace(providerID))
}

func (m *Manager) isManagedProvider(providerID string) bool {
	_, ok := m.managedBridge(providerID)
	return ok
}

func (m *Manager) applyManagedState(state AuthState, models []Model) {
	providerID := strings.TrimSpace(state.ProviderID)
	if providerID == "" {
		return
	}
	m.authStates[tenantAuthKey(state.TenantID, providerID)] = cloneAuthState(state)
	m.models[providerID] = cloneModels(models)
}

func tenantAuthKey(tenantID, providerID string) string {
	providerID = strings.TrimSpace(providerID)
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return providerID
	}
	return tenantID + "\x00" + providerID
}

func (m *Manager) loadProfiles() {
	items := []Profile{
		m.buildEchoProfile(),
		m.buildOpenAICompatibleProfile(),
	}
	if m.registry != nil {
		for _, bridge := range m.registry.List() {
			items = append(items, m.buildManagedProfile(bridge))
		}
	}

	defaultProviderID := m.defaultProviderIDForItems(items)
	for index := range items {
		items[index].Default = items[index].ProviderID == defaultProviderID
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ProviderID < items[j].ProviderID
	})

	m.profiles = make(map[string]Profile, len(items))
	m.order = make([]string, 0, len(items))
	for _, profile := range items {
		m.profiles[profile.ProviderID] = profile
		m.order = append(m.order, profile.ProviderID)
	}
}

func (m *Manager) buildEchoProfile() Profile {
	providerID := "echo"
	effectiveModel := "echo-v1"
	issues := []string{}
	configuredDefaultModel := strings.TrimSpace(m.cfg.LLM.DefaultModel)
	if (strings.TrimSpace(m.cfg.LLM.DefaultProvider) == providerID || (strings.TrimSpace(m.cfg.LLM.DefaultProvider) == "" && configuredDefaultModel != "")) && configuredDefaultModel != "" {
		if strings.EqualFold(configuredDefaultModel, "echo-v1") {
			effectiveModel = "echo-v1"
		} else {
			effectiveModel = ""
			issues = append(issues, "configured default model is incompatible with provider echo")
		}
	}

	return Profile{
		ProviderID:          providerID,
		Title:               "Echo",
		Family:              FamilyBuiltinEcho,
		AuthMode:            AuthModeNone,
		Source:              SourceBuiltin,
		ModelSelectionMode:  ModelSelectionFixed,
		KnownModels:         []string{"echo-v1"},
		Registered:          hasProvider(m.dispatcher, providerID),
		Configured:          true,
		Ready:               effectiveModel != "",
		DefaultModel:        "echo-v1",
		EffectiveModel:      effectiveModel,
		EffectiveTimeoutMs:  defaultPositive(m.cfg.LLM.DefaultTimeoutMs, 30000),
		EffectiveMaxRetries: maxRetryValue(0, m.cfg.LLM.DefaultMaxRetries),
		Capabilities:        CapabilityFlags{Chat: true, Stream: true},
		Issues:              issues,
	}
}

func (m *Manager) buildOpenAICompatibleProfile() Profile {
	providerID := llm.OpenAICompatibleProviderName
	baseURL := strings.TrimSpace(m.cfg.LLM.OpenAICompatible.BaseURL)
	requestURL := ""
	issues := []string{}
	if baseURL == "" {
		issues = append(issues, "base URL is not configured")
	} else if normalized, err := llm.NormalizeOpenAICompatibleRequestURL(baseURL); err == nil {
		requestURL = normalized
	} else {
		issues = append(issues, err.Error())
	}

	secretConfigured := strings.TrimSpace(m.cfg.LLM.OpenAICompatible.APIKey) != ""
	if !secretConfigured {
		issues = append(issues, "API key is not configured")
	}

	profileDefaultModel := strings.TrimSpace(m.cfg.LLM.OpenAICompatible.Model)
	if preference, ok := m.GetPreference(providerID); ok && strings.TrimSpace(preference.DefaultModel) != "" {
		profileDefaultModel = strings.TrimSpace(preference.DefaultModel)
	}
	effectiveModel := profileDefaultModel
	configuredDefaultModel := strings.TrimSpace(m.cfg.LLM.DefaultModel)
	if effectiveModel == "" && (strings.TrimSpace(m.cfg.LLM.DefaultProvider) == providerID || (strings.TrimSpace(m.cfg.LLM.DefaultProvider) == "" && configuredDefaultModel != "")) {
		effectiveModel = configuredDefaultModel
	}
	if effectiveModel == "" {
		issues = append(issues, "default model is not configured")
	}

	timeoutMs := defaultPositive(m.cfg.LLM.OpenAICompatible.TimeoutMs, defaultPositive(m.cfg.LLM.DefaultTimeoutMs, 30000))
	maxRetries := maxRetryValue(0, m.cfg.LLM.DefaultMaxRetries)

	return Profile{
		ProviderID:          providerID,
		Title:               "OpenAI-Compatible",
		Family:              FamilyOpenAICompatible,
		AuthMode:            AuthModeAPIKey,
		Source:              SourceConfig,
		ModelSelectionMode:  ModelSelectionOpen,
		Registered:          hasProvider(m.dispatcher, providerID),
		Configured:          baseURL != "" || secretConfigured || profileDefaultModel != "" || strings.TrimSpace(m.cfg.LLM.DefaultProvider) == providerID,
		Ready:               requestURL != "" && secretConfigured && effectiveModel != "" && hasProvider(m.dispatcher, providerID),
		BaseURL:             baseURL,
		RequestURL:          requestURL,
		DefaultModel:        profileDefaultModel,
		EffectiveModel:      effectiveModel,
		EffectiveTimeoutMs:  timeoutMs,
		EffectiveMaxRetries: maxRetries,
		SecretConfigured:    secretConfigured,
		SecretRef:           strings.TrimSpace(m.cfg.LLM.OpenAICompatible.APIKeyEnv),
		Capabilities:        CapabilityFlags{Chat: true, Stream: true},
		Issues:              issues,
	}
}

func (m *Manager) buildManagedProfile(bridge ManagedBridge) Profile {
	return m.buildManagedProfileForTenant(bridge, "")
}

func (m *Manager) buildManagedProfileForTenant(bridge ManagedBridge, tenantID string) Profile {
	state, hasState := m.authStates[tenantAuthKey(tenantID, bridge.ProviderID())]
	models := cloneModels(m.models[bridge.ProviderID()])
	issues := []string{}
	knownModels := make([]string, 0, len(models))
	for _, model := range models {
		knownModels = append(knownModels, model.ModelID)
	}
	if len(knownModels) == 0 {
		issues = append(issues, "model catalog is not available")
	}

	defaultModel := ""
	if preference, ok := m.GetPreference(bridge.ProviderID()); ok && strings.TrimSpace(preference.DefaultModel) != "" {
		defaultModel = strings.TrimSpace(preference.DefaultModel)
	} else {
		defaultModel = defaultModelFromModels(models)
	}
	effectiveModel := defaultModel
	if strings.TrimSpace(m.cfg.LLM.DefaultProvider) == bridge.ProviderID() && strings.TrimSpace(m.cfg.LLM.DefaultModel) != "" {
		effectiveModel = strings.TrimSpace(m.cfg.LLM.DefaultModel)
	}

	if strings.TrimSpace(defaultModel) == "" {
		issues = append(issues, "default model is not configured")
	}

	if !hasState {
		state = AuthState{
			ProviderID: bridge.ProviderID(),
			Family:     bridge.Family(),
			AuthMode:   bridge.AuthMode(),
			Status:     AuthStatusUnknown,
		}
	}
	if !state.CLIAvailable {
		issues = append(issues, "provider CLI is not available")
	}
	switch state.Status {
	case AuthStatusLoginRequired, AuthStatusPendingLogin, AuthStatusRevoked:
		issues = append(issues, "provider login is required")
	case AuthStatusError:
		if strings.TrimSpace(state.LastError) != "" {
			issues = append(issues, state.LastError)
		} else {
			issues = append(issues, "provider auth state is in error")
		}
	}

	return Profile{
		ProviderID:          bridge.ProviderID(),
		Title:               bridge.DisplayName(),
		Family:              bridge.Family(),
		AuthMode:            bridge.AuthMode(),
		Source:              SourceManaged,
		ModelSelectionMode:  ModelSelectionFixed,
		KnownModels:         knownModels,
		Registered:          hasProvider(m.dispatcher, bridge.ProviderID()),
		Configured:          state.CLIAvailable || hasState,
		Ready:               hasProvider(m.dispatcher, bridge.ProviderID()) && state.Status == AuthStatusAuthenticated && strings.TrimSpace(effectiveModel) != "",
		DefaultModel:        defaultModel,
		EffectiveModel:      effectiveModel,
		EffectiveTimeoutMs:  defaultPositive(m.cfg.LLM.DefaultTimeoutMs, 30000),
		EffectiveMaxRetries: maxRetryValue(0, m.cfg.LLM.DefaultMaxRetries),
		Capabilities:        capabilitiesFromModels(models),
		AuthStatus:          state.Status,
		CLIAvailable:        state.CLIAvailable,
		AccountLabel:        state.AccountLabel,
		AccountID:           state.AccountID,
		Plan:                state.Plan,
		AuthMethod:          state.AuthMethod,
		LoginCommand:        append([]string(nil), state.LoginCommand...),
		LogoutCommand:       append([]string(nil), state.LogoutCommand...),
		AvailableModelCount: availableModelCount(models),
		Issues:              issues,
	}
}

func validateModel(profile Profile, model string) error {
	switch profile.ModelSelectionMode {
	case ModelSelectionFixed:
		for _, known := range profile.KnownModels {
			if strings.EqualFold(strings.TrimSpace(known), strings.TrimSpace(model)) {
				return nil
			}
		}
		return fmt.Errorf("%w: model %q is not supported by provider %s", ErrModelNotSupported, model, profile.ProviderID)
	default:
		return nil
	}
}

func classifyPrepareError(err error) CheckErrorClass {
	switch {
	case errors.Is(err, llm.ErrProviderRequired), errors.Is(err, llm.ErrProviderNotFound), errors.Is(err, llm.ErrModelRequired), errors.Is(err, llm.ErrMessagesRequired), errors.Is(err, ErrModelNotSupported):
		return CheckErrorClassConfig
	default:
		return CheckErrorClassConfig
	}
}

func classifyDispatchFailure(code string) CheckErrorClass {
	switch code {
	case "upstream_auth_failed":
		return CheckErrorClassAuth
	case "upstream_transport_error":
		return CheckErrorClassTransport
	case "timeout", "connect_timeout", "first_chunk_timeout", "idle_timeout", "max_duration_exceeded":
		return CheckErrorClassTimeout
	case "upstream_invalid_request":
		return CheckErrorClassConfig
	default:
		return CheckErrorClassUpstream
	}
}

func failedCheck(checkID string, createdAt time.Time, profile Profile, model string, endpoint string, class CheckErrorClass, err error) Check {
	completedAt := time.Now().UTC()
	errorCode := "provider_check_failed"
	var providerErr *llm.ProviderError
	if errors.As(err, &providerErr) && providerErr.Code != "" {
		errorCode = providerErr.Code
	}
	return Check{
		CheckID:      checkID,
		ProviderID:   profile.ProviderID,
		Family:       profile.Family,
		AuthMode:     profile.AuthMode,
		Status:       CheckStatusFailed,
		Model:        model,
		Endpoint:     endpoint,
		ErrorClass:   class,
		ErrorCode:    errorCode,
		ErrorMessage: err.Error(),
		CreatedAt:    createdAt,
		CompletedAt:  completedAt,
	}
}

func cloneProfile(profile Profile) Profile {
	profile.Issues = append([]string(nil), profile.Issues...)
	profile.KnownModels = append([]string(nil), profile.KnownModels...)
	profile.LoginCommand = append([]string(nil), profile.LoginCommand...)
	profile.LogoutCommand = append([]string(nil), profile.LogoutCommand...)
	return profile
}

func cloneAuthState(state AuthState) AuthState {
	state.LoginCommand = append([]string(nil), state.LoginCommand...)
	state.LogoutCommand = append([]string(nil), state.LogoutCommand...)
	if state.Metadata != nil {
		metadata := make(map[string]string, len(state.Metadata))
		for key, value := range state.Metadata {
			metadata[key] = value
		}
		state.Metadata = metadata
	}
	if state.Sandbox != nil {
		payload, err := json.Marshal(state.Sandbox)
		if err == nil {
			var cloned map[string]any
			if json.Unmarshal(payload, &cloned) == nil {
				state.Sandbox = cloned
			} else {
				state.Sandbox = state.Sandbox
			}
		} else {
			state.Sandbox = state.Sandbox
		}
	}
	return state
}

func cloneModels(items []Model) []Model {
	cloned := make([]Model, 0, len(items))
	for _, item := range items {
		model := item
		model.ReasoningLevels = append([]string(nil), item.ReasoningLevels...)
		cloned = append(cloned, model)
	}
	return cloned
}

func cloneMessages(messages []llm.Message) []llm.Message {
	cloned := make([]llm.Message, len(messages))
	copy(cloned, messages)
	return cloned
}

func hasProvider(dispatcher *llm.Dispatcher, providerID string) bool {
	if dispatcher == nil {
		return false
	}
	return dispatcher.HasProvider(providerID)
}

func (m *Manager) defaultProviderID() string {
	return m.defaultProviderIDForItems(m.ListProfiles())
}

func (m *Manager) defaultProviderIDForItems(items []Profile) string {
	if explicit := strings.TrimSpace(m.cfg.LLM.DefaultProvider); explicit != "" {
		return explicit
	}
	for _, item := range items {
		if item.ProviderID == llm.OpenAICompatibleProviderName && item.Ready {
			return item.ProviderID
		}
	}
	for _, item := range items {
		if item.Source == SourceManaged && item.Ready {
			return item.ProviderID
		}
	}
	return "echo"
}

func defaultPositive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func maxRetryValue(value, fallback int) int {
	if value < 0 {
		value = 0
	}
	if value > 0 {
		return value
	}
	if fallback < 0 {
		return 0
	}
	return fallback
}

func defaultModelFromModels(items []Model) string {
	for _, item := range items {
		if item.Default && strings.TrimSpace(item.ModelID) != "" {
			return item.ModelID
		}
	}
	if len(items) > 0 {
		return strings.TrimSpace(items[0].ModelID)
	}
	return ""
}

func availableModelCount(items []Model) int {
	count := 0
	for _, item := range items {
		if item.Available {
			count++
		}
	}
	return count
}

func capabilitiesFromModels(items []Model) CapabilityFlags {
	flags := CapabilityFlags{Chat: true, Stream: true}
	for _, item := range items {
		flags.Chat = flags.Chat || item.Chat
		flags.Stream = flags.Stream || item.Stream
		flags.Coding = flags.Coding || item.Coding
		flags.ToolUse = flags.ToolUse || item.ToolUse
	}
	return flags
}
