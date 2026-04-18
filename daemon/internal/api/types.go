package api

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

type SystemInfoResponse struct {
	Service  string `json:"service"`
	Version  string `json:"version"`
	BindAddr string `json:"bindAddr"`
	DataDir  string `json:"dataDir"`
	LogLevel string `json:"logLevel"`
}

type ConfigResponse struct {
	BindAddr       string            `json:"bindAddr"`
	DataDir        string            `json:"dataDir"`
	ConfigFilePath string            `json:"configFilePath"`
	LogLevel       string            `json:"logLevel"`
	Version        string            `json:"version"`
	LLM            ConfigLLMResponse `json:"llm"`
	RedactedFields []string          `json:"redactedFields"`
}

type ConfigLLMResponse struct {
	DefaultProvider   string                                 `json:"defaultProvider"`
	DefaultModel      string                                 `json:"defaultModel"`
	DefaultTimeoutMs  int                                    `json:"defaultTimeoutMs"`
	DefaultMaxRetries int                                    `json:"defaultMaxRetries"`
	OpenAICompatible  ConfigOpenAICompatibleProviderResponse `json:"openaiCompatible"`
}

type ConfigOpenAICompatibleProviderResponse struct {
	Configured       bool   `json:"configured"`
	BaseURL          string `json:"baseURL"`
	Model            string `json:"model"`
	TimeoutMs        int    `json:"timeoutMs"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
	APIKeyEnv        string `json:"apiKeyEnv,omitempty"`
}

type ChatQueryResponse struct {
	DispatchID   string    `json:"dispatchId"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Query        string    `json:"query"`
	Status       string    `json:"status"`
	Reply        string    `json:"reply"`
	FinishReason string    `json:"finishReason,omitempty"`
	Usage        llm.Usage `json:"usage"`
	ErrorCode    string    `json:"errorCode,omitempty"`
	Error        string    `json:"error,omitempty"`
}

type ChatQueryStreamStarted struct {
	DispatchID string `json:"dispatchId"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Query      string `json:"query"`
}

type ChatQueryStreamDelta struct {
	DispatchID string `json:"dispatchId"`
	Delta      string `json:"delta"`
	Reply      string `json:"reply"`
}

type SessionRouteRequest struct {
	Kind      router.SessionKind `json:"kind,omitempty"`
	Channel   string             `json:"channel,omitempty"`
	AccountID string             `json:"accountId,omitempty"`
	PeerID    string             `json:"peerId,omitempty"`
	ThreadID  string             `json:"threadId,omitempty"`
}

type CreateRunRequest struct {
	SessionID  string               `json:"sessionId,omitempty"`
	Route      *SessionRouteRequest `json:"route,omitempty"`
	Entrypoint string               `json:"entrypoint"`
	Goal       string               `json:"goal,omitempty"`
	Input      any                  `json:"input,omitempty"`
}

type ConnectorIngressMessage struct {
	MessageID string `json:"messageId"`
	Text      string `json:"text,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

type ConnectorIngressRunRequest struct {
	Entrypoint string `json:"entrypoint"`
	Goal       string `json:"goal,omitempty"`
}

type ConnectorIngressMessageRequest struct {
	Route   SessionRouteRequest         `json:"route"`
	Message ConnectorIngressMessage     `json:"message"`
	Run     *ConnectorIngressRunRequest `json:"run,omitempty"`
}

type ConnectorIngressMessageResponse struct {
	IngressID      string         `json:"ingressId"`
	ConnectorID    string         `json:"connectorId"`
	AcceptedAt     time.Time      `json:"acceptedAt"`
	Session        router.Session `json:"session"`
	SessionCreated bool           `json:"sessionCreated"`
	Run            *runtime.Run   `json:"run,omitempty"`
	RunCreated     bool           `json:"runCreated"`
}

type EventListResponse struct {
	Items      []events.Event `json:"items"`
	NextCursor int64          `json:"nextCursor,omitempty"`
}

type ListResponse[T any] struct {
	Items []T `json:"items"`
}

func buildSystemInfoResponse(cfg config.Config) SystemInfoResponse {
	return SystemInfoResponse{
		Service:  "dope",
		Version:  cfg.Version,
		BindAddr: cfg.BindAddr,
		DataDir:  cfg.DataDir,
		LogLevel: cfg.LogLevel,
	}
}

func buildConfigResponse(cfg config.Config) ConfigResponse {
	redactedFields := []string{}
	if cfg.LLM.OpenAICompatible.APIKey != "" {
		redactedFields = append(redactedFields, "llm.openaiCompatible.apiKey")
	}
	defaultTimeoutMs := cfg.LLM.DefaultTimeoutMs
	if defaultTimeoutMs <= 0 {
		defaultTimeoutMs = 30000
	}
	openAITimeoutMs := cfg.LLM.OpenAICompatible.TimeoutMs
	if openAITimeoutMs <= 0 {
		openAITimeoutMs = defaultTimeoutMs
	}

	return ConfigResponse{
		BindAddr:       cfg.BindAddr,
		DataDir:        cfg.DataDir,
		ConfigFilePath: config.ConfigFilePath(cfg.DataDir),
		LogLevel:       cfg.LogLevel,
		Version:        cfg.Version,
		LLM: ConfigLLMResponse{
			DefaultProvider:   cfg.LLM.DefaultProvider,
			DefaultModel:      cfg.LLM.DefaultModel,
			DefaultTimeoutMs:  defaultTimeoutMs,
			DefaultMaxRetries: cfg.LLM.DefaultMaxRetries,
			OpenAICompatible: ConfigOpenAICompatibleProviderResponse{
				Configured:       cfg.LLM.OpenAICompatible.BaseURL != "" || cfg.LLM.OpenAICompatible.APIKey != "" || cfg.LLM.OpenAICompatible.Model != "",
				BaseURL:          cfg.LLM.OpenAICompatible.BaseURL,
				Model:            cfg.LLM.OpenAICompatible.Model,
				TimeoutMs:        openAITimeoutMs,
				APIKeyConfigured: cfg.LLM.OpenAICompatible.APIKey != "",
				APIKeyEnv:        cfg.LLM.OpenAICompatible.APIKeyEnv,
			},
		},
		RedactedFields: redactedFields,
	}
}

func buildEventListResponse(items []events.Event) EventListResponse {
	response := EventListResponse{
		Items: items,
	}
	if len(items) > 0 {
		response.NextCursor = items[len(items)-1].Sequence
	}
	return response
}
