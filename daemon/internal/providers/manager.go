package providers

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
)

var ErrModelNotSupported = errors.New("model is not supported by provider")

type Family string

const (
	FamilyBuiltinEcho      Family = "builtin_echo"
	FamilyOpenAICompatible Family = "openai_compatible"
)

type AuthMode string

const (
	AuthModeNone   AuthMode = "none"
	AuthModeAPIKey AuthMode = "api_key"
)

type Source string

const (
	SourceBuiltin Source = "builtin"
	SourceConfig  Source = "config"
)

type ModelSelectionMode string

const (
	ModelSelectionFixed ModelSelectionMode = "fixed"
	ModelSelectionOpen  ModelSelectionMode = "open"
)

type CapabilityFlags struct {
	Chat   bool `json:"chat"`
	Stream bool `json:"stream"`
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

type Manager struct {
	cfg        config.Config
	dispatcher *llm.Dispatcher
	profiles   map[string]Profile
	order      []string
}

func NewManager(cfg config.Config, dispatcher *llm.Dispatcher) *Manager {
	manager := &Manager{
		cfg:        cfg,
		dispatcher: dispatcher,
		profiles:   make(map[string]Profile),
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

func (m *Manager) Resolve(providerID, model string, timeoutMs, maxRetries int) (ResolvedDispatch, error) {
	requestedProvider := strings.TrimSpace(providerID)
	effectiveProvider := requestedProvider
	if effectiveProvider == "" {
		effectiveProvider = m.defaultProviderID()
	}

	profile, ok := m.GetProfile(effectiveProvider)
	if !ok {
		return ResolvedDispatch{}, fmt.Errorf("%w: %s", llm.ErrProviderNotFound, effectiveProvider)
	}
	if !profile.Registered {
		return ResolvedDispatch{}, fmt.Errorf("%w: %s", llm.ErrProviderNotFound, effectiveProvider)
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

	effectiveMaxRetries := maxRetries
	if effectiveMaxRetries < 0 {
		effectiveMaxRetries = 0
	}
	if effectiveMaxRetries == 0 {
		effectiveMaxRetries = max(profile.EffectiveMaxRetries, 0)
	}

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

func (m *Manager) loadProfiles() {
	items := []Profile{
		m.buildEchoProfile(),
		m.buildOpenAICompatibleProfile(),
	}

	defaultProviderID := m.defaultProviderIDForItems(items)
	for index := range items {
		items[index].Default = items[index].ProviderID == defaultProviderID
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ProviderID < items[j].ProviderID
	})

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
		EffectiveMaxRetries: max(m.cfg.LLM.DefaultMaxRetries, 0),
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
	effectiveModel := profileDefaultModel
	configuredDefaultModel := strings.TrimSpace(m.cfg.LLM.DefaultModel)
	if effectiveModel == "" && (strings.TrimSpace(m.cfg.LLM.DefaultProvider) == providerID || (strings.TrimSpace(m.cfg.LLM.DefaultProvider) == "" && configuredDefaultModel != "")) {
		effectiveModel = strings.TrimSpace(m.cfg.LLM.DefaultModel)
	}
	if effectiveModel == "" {
		issues = append(issues, "default model is not configured")
	}

	timeoutMs := defaultPositive(m.cfg.LLM.OpenAICompatible.TimeoutMs, defaultPositive(m.cfg.LLM.DefaultTimeoutMs, 30000))
	maxRetries := max(m.cfg.LLM.DefaultMaxRetries, 0)

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
	case errors.Is(err, llm.ErrProviderRequired), errors.Is(err, llm.ErrProviderNotFound), errors.Is(err, llm.ErrModelRequired), errors.Is(err, llm.ErrMessagesRequired):
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
	case "timeout":
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
	return profile
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
	return "echo"
}

func defaultPositive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func max(value, fallback int) int {
	if value >= fallback {
		return value
	}
	return fallback
}
