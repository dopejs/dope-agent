package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultConfigFileName = "config.json"

type Environment string

const (
	EnvironmentProd Environment = "prod"
	EnvironmentTest Environment = "test"
)

type Config struct {
	Environment Environment
	BindAddr    string
	DataDir     string
	LogLevel    string
	Version     string
	LLM         LLMConfig
	Connectors  ConnectorConfig
}

type LLMConfig struct {
	DefaultProvider   string
	DefaultModel      string
	DefaultTimeoutMs  int
	DefaultMaxRetries int
	OpenAICompatible  OpenAICompatibleProviderConfig
	Claude            ManagedCLIProviderConfig
	Codex             ManagedCLIProviderConfig
}

type OpenAICompatibleProviderConfig struct {
	BaseURL                   string
	APIKey                    string
	APIKeyEnv                 string
	Model                     string
	TimeoutMs                 int
	StreamFirstChunkTimeoutMs int
	StreamIdleTimeoutMs       int
	StreamMaxDurationMs       int
}

type ManagedCLIProviderConfig struct {
	CLIPath      string
	DefaultModel string
	WorkDir      string
}

type ConnectorConfig struct {
	Discord  DiscordConnectorConfig
	Telegram TelegramConnectorConfig
	Slack    SlackConnectorConfig
	Matrix   MatrixConnectorConfig
}

type DiscordConnectorConfig struct {
	Enabled           bool
	ConnectorID       string
	DisplayName       string
	DeliveryMode      string
	BotToken          string
	BotTokenEnv       string
	RequireMention    bool
	RespondInDM       bool
	AllowedGuildIDs   []string
	AllowedChannelIDs []string
}

type TelegramConnectorConfig struct {
	Enabled              bool
	ConnectorID          string
	DisplayName          string
	BotToken             string
	BotTokenEnv          string
	BotAPIBaseURL        string
	BotUsername          string
	AllowedUserIDs       []string
	AllowedDirectChatIDs []string
	AllowedGroupIDs      []string
}

type SlackConnectorConfig struct {
	Enabled              bool
	ConnectorID          string
	DisplayName          string
	APIBaseURL           string
	BotTokenSecretRef    string
	OAuthClientID        string
	OAuthClientSecret    string
	OAuthClientSecretEnv string
	OAuthAPIBaseURL      string
	WorkspaceBindingID   string
	WorkspaceID          string
	BotUserID            string
	AllowedChannelIDs    []string
	AllowedDMUserIDs     []string
	AllowedDMUserGroups  []string
}

type MatrixConnectorConfig struct {
	Enabled              bool
	ConnectorID          string
	DisplayName          string
	HomeserverURL        string
	HomeserverID         string
	BotUserID            string
	BotAccessToken       string
	BotAccessTokenEnv    string
	SelectedRoomIDs      []string
	AllowedDirectUserIDs []string
	ConfiguredCommands   []string
}

type SlackHostedReadinessProjection struct {
	TenantID            string   `json:"tenantId,omitempty"`
	ConnectorID         string   `json:"connectorId"`
	DisplayName         string   `json:"displayName"`
	TerminalState       string   `json:"terminalState"`
	HostedReady         bool     `json:"hostedReady"`
	LocalCompatible     bool     `json:"localCompatible"`
	ReasonCode          string   `json:"reasonCode,omitempty"`
	WorkspaceBindingID  string   `json:"workspaceBindingId,omitempty"`
	WorkspaceID         string   `json:"workspaceId,omitempty"`
	BotUserID           string   `json:"botUserId,omitempty"`
	AllowedChannelIDs   []string `json:"allowedChannelIds,omitempty"`
	AllowedDMUserIDs    []string `json:"allowedDMUserIds,omitempty"`
	AllowedDMUserGroups []string `json:"allowedDMUserGroups,omitempty"`
	RedactionStatus     string   `json:"redactionStatus"`
}

func (cfg SlackConnectorConfig) ProjectHostedReadiness(tenantID string) SlackHostedReadinessProjection {
	projection := SlackHostedReadinessProjection{
		TenantID:            strings.TrimSpace(tenantID),
		ConnectorID:         strings.TrimSpace(cfg.ConnectorID),
		DisplayName:         strings.TrimSpace(cfg.DisplayName),
		TerminalState:       "action-required",
		HostedReady:         false,
		LocalCompatible:     cfg.Enabled && strings.TrimSpace(cfg.WorkspaceID) != "",
		WorkspaceBindingID:  strings.TrimSpace(cfg.WorkspaceBindingID),
		WorkspaceID:         strings.TrimSpace(cfg.WorkspaceID),
		BotUserID:           strings.TrimSpace(cfg.BotUserID),
		AllowedChannelIDs:   append([]string(nil), cfg.AllowedChannelIDs...),
		AllowedDMUserIDs:    append([]string(nil), cfg.AllowedDMUserIDs...),
		AllowedDMUserGroups: append([]string(nil), cfg.AllowedDMUserGroups...),
		RedactionStatus:     "redacted",
	}
	if !projection.LocalCompatible {
		if !cfg.Enabled {
			projection.TerminalState = "cancelled"
			projection.ReasonCode = "disabled"
			return projection
		}
		projection.ReasonCode = "slack_workspace_binding_missing"
		return projection
	}
	if len(cfg.AllowedChannelIDs) == 0 && len(cfg.AllowedDMUserIDs) == 0 && len(cfg.AllowedDMUserGroups) == 0 {
		projection.ReasonCode = "slack_route_policy_missing"
		return projection
	}
	projection.ReasonCode = "slack_route_policy_validation_required"
	return projection
}

type MatrixHostedReadinessProjection struct {
	TenantID               string   `json:"tenantId,omitempty"`
	ConnectorID            string   `json:"connectorId"`
	DisplayName            string   `json:"displayName"`
	TerminalState          string   `json:"terminalState"`
	HostedReady            bool     `json:"hostedReady"`
	LocalCompatible        bool     `json:"localCompatible"`
	ReasonCode             string   `json:"reasonCode,omitempty"`
	HomeserverURL          string   `json:"homeserverUrl,omitempty"`
	HomeserverID           string   `json:"homeserverId,omitempty"`
	BotUserID              string   `json:"botUserId,omitempty"`
	BotAccessTokenSet      bool     `json:"botAccessTokenSet"`
	SelectedRoomIDs        []string `json:"selectedRoomIds,omitempty"`
	AllowedDirectUserIDs   []string `json:"allowedDirectUserIds,omitempty"`
	ConfiguredCommands     []string `json:"configuredCommands,omitempty"`
	HostedHomeserverPolicy string   `json:"hostedHomeserverPolicy"`
	RedactionStatus        string   `json:"redactionStatus"`
}

func (cfg MatrixConnectorConfig) ProjectHostedReadiness(tenantID string) MatrixHostedReadinessProjection {
	projection := MatrixHostedReadinessProjection{
		TenantID:               strings.TrimSpace(tenantID),
		ConnectorID:            strings.TrimSpace(cfg.ConnectorID),
		DisplayName:            strings.TrimSpace(cfg.DisplayName),
		TerminalState:          "action-required",
		HostedReady:            false,
		LocalCompatible:        cfg.Enabled && strings.TrimSpace(cfg.HomeserverURL) != "" && strings.TrimSpace(cfg.BotAccessToken) != "" && strings.TrimSpace(cfg.BotUserID) != "",
		HomeserverURL:          strings.TrimSpace(cfg.HomeserverURL),
		HomeserverID:           strings.TrimSpace(cfg.HomeserverID),
		BotUserID:              strings.TrimSpace(cfg.BotUserID),
		BotAccessTokenSet:      strings.TrimSpace(cfg.BotAccessToken) != "",
		SelectedRoomIDs:        append([]string(nil), cfg.SelectedRoomIDs...),
		AllowedDirectUserIDs:   append([]string(nil), cfg.AllowedDirectUserIDs...),
		ConfiguredCommands:     append([]string(nil), cfg.ConfiguredCommands...),
		HostedHomeserverPolicy: "unsupported",
		RedactionStatus:        "redacted",
	}
	if !cfg.Enabled {
		projection.TerminalState = "cancelled"
		projection.ReasonCode = "disabled"
		return projection
	}
	if strings.TrimSpace(cfg.HomeserverURL) == "" {
		projection.ReasonCode = "matrix_homeserver_missing"
		return projection
	}
	if strings.TrimSpace(cfg.BotAccessToken) == "" || strings.TrimSpace(cfg.BotUserID) == "" {
		projection.ReasonCode = "matrix_bot_credential_missing"
		return projection
	}
	if len(cfg.SelectedRoomIDs) == 0 && len(cfg.AllowedDirectUserIDs) == 0 {
		projection.ReasonCode = "matrix_route_policy_missing"
		return projection
	}
	projection.ReasonCode = "matrix_route_policy_validation_required"
	return projection
}

type TelegramHostedReadinessProjection struct {
	TenantID             string   `json:"tenantId,omitempty"`
	ConnectorID          string   `json:"connectorId"`
	DisplayName          string   `json:"displayName"`
	TerminalState        string   `json:"terminalState"`
	HostedReady          bool     `json:"hostedReady"`
	LocalCompatible      bool     `json:"localCompatible"`
	ReasonCode           string   `json:"reasonCode,omitempty"`
	BotTokenConfigured   bool     `json:"botTokenConfigured"`
	BotUsername          string   `json:"botUsername,omitempty"`
	AllowedUserIDs       []string `json:"allowedUserIds,omitempty"`
	AllowedDirectChatIDs []string `json:"allowedDirectChatIds,omitempty"`
	AllowedGroupIDs      []string `json:"allowedGroupIds,omitempty"`
	RedactionStatus      string   `json:"redactionStatus"`
}

type DiscordHostedReadinessProjection struct {
	TenantID           string   `json:"tenantId,omitempty"`
	ConnectorID        string   `json:"connectorId"`
	DisplayName        string   `json:"displayName"`
	DeliveryMode       string   `json:"deliveryMode"`
	ReadinessState     string   `json:"readinessState"`
	HostedReady        bool     `json:"hostedReady"`
	LocalCompatible    bool     `json:"localCompatible"`
	ReasonCode         string   `json:"reasonCode,omitempty"`
	RequireMention     bool     `json:"requireMention"`
	RespondInDM        bool     `json:"respondInDM"`
	AllowedGuildIDs    []string `json:"allowedGuildIds,omitempty"`
	AllowedChannelIDs  []string `json:"allowedChannelIds,omitempty"`
	BotTokenConfigured bool     `json:"botTokenConfigured"`
	BotToken           string   `json:"-"`
	RedactionStatus    string   `json:"redactionStatus"`
}

func (cfg DiscordConnectorConfig) ProjectHostedReadiness(tenantID string) DiscordHostedReadinessProjection {
	mode := strings.TrimSpace(cfg.DeliveryMode)
	if mode == "" {
		mode = "gateway"
	}
	projection := DiscordHostedReadinessProjection{
		TenantID:           strings.TrimSpace(tenantID),
		ConnectorID:        strings.TrimSpace(cfg.ConnectorID),
		DisplayName:        strings.TrimSpace(cfg.DisplayName),
		DeliveryMode:       mode,
		ReadinessState:     "degraded_needs_repair",
		HostedReady:        false,
		LocalCompatible:    cfg.Enabled && strings.TrimSpace(cfg.BotToken) != "",
		RequireMention:     cfg.RequireMention,
		RespondInDM:        cfg.RespondInDM,
		AllowedGuildIDs:    append([]string(nil), cfg.AllowedGuildIDs...),
		AllowedChannelIDs:  append([]string(nil), cfg.AllowedChannelIDs...),
		BotTokenConfigured: strings.TrimSpace(cfg.BotToken) != "",
		RedactionStatus:    "redacted",
	}
	if !cfg.Enabled {
		projection.ReadinessState = "disabled"
		projection.HostedReady = false
		projection.ReasonCode = "disabled"
		return projection
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		projection.ReadinessState = "failed"
		projection.HostedReady = false
		projection.ReasonCode = "auth_missing"
		return projection
	}
	if len(cfg.AllowedGuildIDs) == 0 && len(cfg.AllowedChannelIDs) == 0 {
		projection.ReadinessState = "degraded_needs_repair"
		projection.HostedReady = false
		projection.ReasonCode = "missing_explicit_destination"
		return projection
	}
	projection.ReasonCode = "destination_validation_required"
	return projection
}

func (cfg TelegramConnectorConfig) ProjectHostedReadiness(tenantID string) TelegramHostedReadinessProjection {
	projection := TelegramHostedReadinessProjection{
		TenantID:             strings.TrimSpace(tenantID),
		ConnectorID:          strings.TrimSpace(cfg.ConnectorID),
		DisplayName:          strings.TrimSpace(cfg.DisplayName),
		TerminalState:        "action-required",
		HostedReady:          false,
		LocalCompatible:      cfg.Enabled && strings.TrimSpace(cfg.BotToken) != "",
		BotTokenConfigured:   strings.TrimSpace(cfg.BotToken) != "",
		BotUsername:          strings.TrimSpace(cfg.BotUsername),
		AllowedUserIDs:       append([]string(nil), cfg.AllowedUserIDs...),
		AllowedDirectChatIDs: append([]string(nil), cfg.AllowedDirectChatIDs...),
		AllowedGroupIDs:      append([]string(nil), cfg.AllowedGroupIDs...),
		RedactionStatus:      "redacted",
	}
	if !cfg.Enabled {
		projection.TerminalState = "cancelled"
		projection.ReasonCode = "disabled"
		return projection
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		projection.ReasonCode = "auth_missing"
		return projection
	}
	if len(cfg.AllowedUserIDs) == 0 && len(cfg.AllowedDirectChatIDs) == 0 && len(cfg.AllowedGroupIDs) == 0 {
		projection.ReasonCode = "telegram_allowment_missing"
		return projection
	}
	projection.ReasonCode = "telegram_allowment_validation_required"
	return projection
}

type fileConfig struct {
	Environment string               `json:"environment"`
	BindAddr    string               `json:"bindAddr"`
	DataDir     string               `json:"dataDir"`
	LogLevel    string               `json:"logLevel"`
	LLM         *fileLLMConfig       `json:"llm"`
	Connectors  *fileConnectorConfig `json:"connectors"`
}

type fileLLMConfig struct {
	DefaultProvider   string                              `json:"defaultProvider"`
	DefaultModel      string                              `json:"defaultModel"`
	DefaultTimeoutMs  int                                 `json:"defaultTimeoutMs"`
	DefaultMaxRetries int                                 `json:"defaultMaxRetries"`
	OpenAICompatible  *fileOpenAICompatibleProviderConfig `json:"openaiCompatible"`
	Claude            *fileManagedCLIProviderConfig       `json:"claude"`
	Codex             *fileManagedCLIProviderConfig       `json:"codex"`
}

type fileOpenAICompatibleProviderConfig struct {
	BaseURL                   string `json:"baseURL"`
	APIKey                    string `json:"apiKey"`
	APIKeyEnv                 string `json:"apiKeyEnv"`
	Model                     string `json:"model"`
	TimeoutMs                 int    `json:"timeoutMs"`
	StreamFirstChunkTimeoutMs int    `json:"streamFirstChunkTimeoutMs"`
	StreamIdleTimeoutMs       int    `json:"streamIdleTimeoutMs"`
	StreamMaxDurationMs       int    `json:"streamMaxDurationMs"`
}

type fileManagedCLIProviderConfig struct {
	CLIPath      string `json:"cliPath"`
	DefaultModel string `json:"defaultModel"`
	WorkDir      string `json:"workDir"`
}

type fileConnectorConfig struct {
	Discord  *fileDiscordConnectorConfig  `json:"discord"`
	Telegram *fileTelegramConnectorConfig `json:"telegram"`
	Slack    *fileSlackConnectorConfig    `json:"slack"`
	Matrix   *fileMatrixConnectorConfig   `json:"matrix"`
}

type fileDiscordConnectorConfig struct {
	Enabled           *bool    `json:"enabled"`
	ConnectorID       string   `json:"connectorId"`
	DisplayName       string   `json:"displayName"`
	DeliveryMode      string   `json:"deliveryMode"`
	BotToken          string   `json:"botToken"`
	BotTokenEnv       string   `json:"botTokenEnv"`
	RequireMention    *bool    `json:"requireMention"`
	RespondInDM       *bool    `json:"respondInDM"`
	AllowedGuildIDs   []string `json:"allowedGuildIds"`
	AllowedChannelIDs []string `json:"allowedChannelIds"`
}

type fileTelegramConnectorConfig struct {
	Enabled              *bool    `json:"enabled"`
	ConnectorID          string   `json:"connectorId"`
	DisplayName          string   `json:"displayName"`
	BotToken             string   `json:"botToken"`
	BotTokenEnv          string   `json:"botTokenEnv"`
	BotAPIBaseURL        string   `json:"botApiBaseUrl"`
	BotUsername          string   `json:"botUsername"`
	AllowedUserIDs       []string `json:"allowedUserIds"`
	AllowedDirectChatIDs []string `json:"allowedDirectChatIds"`
	AllowedGroupIDs      []string `json:"allowedGroupIds"`
}

type fileSlackConnectorConfig struct {
	Enabled              *bool    `json:"enabled"`
	ConnectorID          string   `json:"connectorId"`
	DisplayName          string   `json:"displayName"`
	APIBaseURL           string   `json:"apiBaseUrl"`
	BotTokenSecretRef    string   `json:"botTokenSecretRef"`
	OAuthClientID        string   `json:"oauthClientId"`
	OAuthClientSecret    string   `json:"oauthClientSecret"`
	OAuthClientSecretEnv string   `json:"oauthClientSecretEnv"`
	OAuthAPIBaseURL      string   `json:"oauthApiBaseUrl"`
	WorkspaceBindingID   string   `json:"workspaceBindingId"`
	WorkspaceID          string   `json:"workspaceId"`
	BotUserID            string   `json:"botUserId"`
	AllowedChannelIDs    []string `json:"allowedChannelIds"`
	AllowedDMUserIDs     []string `json:"allowedDMUserIds"`
	AllowedDMUserGroups  []string `json:"allowedDMUserGroups"`
}

type fileMatrixConnectorConfig struct {
	Enabled              *bool    `json:"enabled"`
	ConnectorID          string   `json:"connectorId"`
	DisplayName          string   `json:"displayName"`
	HomeserverURL        string   `json:"homeserverUrl"`
	HomeserverID         string   `json:"homeserverId"`
	BotUserID            string   `json:"botUserId"`
	BotAccessToken       string   `json:"botAccessToken"`
	BotAccessTokenEnv    string   `json:"botAccessTokenEnv"`
	SelectedRoomIDs      []string `json:"selectedRoomIds"`
	AllowedDirectUserIDs []string `json:"allowedDirectUserIds"`
	ConfiguredCommands   []string `json:"configuredCommands"`
}

func Load() (Config, error) {
	version := getenv("DOPE_VERSION", "dev")
	envName := resolveEnvironment(getenv("DOPE_ENV", ""), version)

	bootstrapDir, err := ResolveDir(getenv("DOPE_DATA_DIR", defaultDataDir(envName)))
	if err != nil {
		return Config{}, fmt.Errorf("resolve bootstrap data dir: %w", err)
	}
	if err := ensureDir(bootstrapDir); err != nil {
		return Config{}, fmt.Errorf("initialize bootstrap data dir: %w", err)
	}

	cfg := Config{
		Environment: envName,
		BindAddr:    defaultBindAddr(envName),
		DataDir:     bootstrapDir,
		LogLevel:    "info",
		Version:     version,
		LLM: LLMConfig{
			DefaultTimeoutMs:  30000,
			DefaultMaxRetries: 0,
			OpenAICompatible: OpenAICompatibleProviderConfig{
				TimeoutMs:                 30000,
				StreamFirstChunkTimeoutMs: 30000,
				StreamIdleTimeoutMs:       30000,
			},
			Claude: ManagedCLIProviderConfig{
				WorkDir: "~",
			},
			Codex: ManagedCLIProviderConfig{
				WorkDir: "~",
			},
		},
		Connectors: ConnectorConfig{
			Discord: DiscordConnectorConfig{
				ConnectorID:    "discord-main",
				DisplayName:    "Discord Main",
				DeliveryMode:   "gateway",
				RequireMention: true,
				RespondInDM:    true,
			},
			Telegram: TelegramConnectorConfig{
				ConnectorID: "telegram-main",
				DisplayName: "Telegram Main",
			},
			Slack: SlackConnectorConfig{
				ConnectorID: "slack-main",
				DisplayName: "Slack Main",
			},
			Matrix: MatrixConnectorConfig{
				ConnectorID: "matrix-main",
				DisplayName: "Matrix Main",
			},
		},
	}

	loadedFileConfig, err := loadFileConfig(filepath.Join(bootstrapDir, defaultConfigFileName))
	if err != nil {
		return Config{}, err
	}
	applyFileConfig(&cfg, loadedFileConfig)
	applyEnvOverrides(&cfg)
	resolveSecretRefs(&cfg)

	cfg.DataDir, err = ResolveDir(cfg.DataDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve effective data dir: %w", err)
	}
	if err := ensureDir(cfg.DataDir); err != nil {
		return Config{}, fmt.Errorf("initialize effective data dir: %w", err)
	}

	return cfg, nil
}

func ResolveDir(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is required")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		if path == "~" {
			return homeDir, nil
		}
		return filepath.Join(homeDir, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func ManagedProviderHomeDir(cfg Config) string {
	if cfg.Environment == EnvironmentTest && strings.TrimSpace(cfg.DataDir) != "" {
		return filepath.Join(strings.TrimSpace(cfg.DataDir), "managed-provider-home")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(homeDir)
}

func ConfigFilePath(dataDir string) string {
	return filepath.Join(dataDir, defaultConfigFileName)
}

func applyFileConfig(cfg *Config, fileCfg fileConfig) {
	if envName := normalizeEnvironment(fileCfg.Environment); envName != "" {
		cfg.Environment = envName
	}
	if fileCfg.BindAddr != "" {
		cfg.BindAddr = fileCfg.BindAddr
	}
	if fileCfg.DataDir != "" {
		cfg.DataDir = fileCfg.DataDir
	}
	if fileCfg.LogLevel != "" {
		cfg.LogLevel = fileCfg.LogLevel
	}
	if fileCfg.LLM != nil {
		applyFileLLMConfig(&cfg.LLM, *fileCfg.LLM)
	}
	if fileCfg.Connectors != nil {
		applyFileConnectorConfig(&cfg.Connectors, *fileCfg.Connectors)
	}
}

func applyFileLLMConfig(cfg *LLMConfig, fileCfg fileLLMConfig) {
	if fileCfg.DefaultProvider != "" {
		cfg.DefaultProvider = fileCfg.DefaultProvider
	}
	if fileCfg.DefaultModel != "" {
		cfg.DefaultModel = fileCfg.DefaultModel
	}
	if fileCfg.DefaultTimeoutMs > 0 {
		cfg.DefaultTimeoutMs = fileCfg.DefaultTimeoutMs
	}
	if fileCfg.DefaultMaxRetries >= 0 {
		cfg.DefaultMaxRetries = fileCfg.DefaultMaxRetries
	}
	if fileCfg.OpenAICompatible != nil {
		if fileCfg.OpenAICompatible.BaseURL != "" {
			cfg.OpenAICompatible.BaseURL = fileCfg.OpenAICompatible.BaseURL
		}
		if fileCfg.OpenAICompatible.APIKey != "" {
			cfg.OpenAICompatible.APIKey = fileCfg.OpenAICompatible.APIKey
		}
		if fileCfg.OpenAICompatible.APIKeyEnv != "" {
			cfg.OpenAICompatible.APIKeyEnv = fileCfg.OpenAICompatible.APIKeyEnv
		}
		if fileCfg.OpenAICompatible.Model != "" {
			cfg.OpenAICompatible.Model = fileCfg.OpenAICompatible.Model
		}
		if fileCfg.OpenAICompatible.TimeoutMs > 0 {
			cfg.OpenAICompatible.TimeoutMs = fileCfg.OpenAICompatible.TimeoutMs
		}
		if fileCfg.OpenAICompatible.StreamFirstChunkTimeoutMs > 0 {
			cfg.OpenAICompatible.StreamFirstChunkTimeoutMs = fileCfg.OpenAICompatible.StreamFirstChunkTimeoutMs
		}
		if fileCfg.OpenAICompatible.StreamIdleTimeoutMs > 0 {
			cfg.OpenAICompatible.StreamIdleTimeoutMs = fileCfg.OpenAICompatible.StreamIdleTimeoutMs
		}
		if fileCfg.OpenAICompatible.StreamMaxDurationMs > 0 {
			cfg.OpenAICompatible.StreamMaxDurationMs = fileCfg.OpenAICompatible.StreamMaxDurationMs
		}
	}
	if fileCfg.Claude != nil {
		applyFileManagedCLIConfig(&cfg.Claude, *fileCfg.Claude)
	}
	if fileCfg.Codex != nil {
		applyFileManagedCLIConfig(&cfg.Codex, *fileCfg.Codex)
	}
}

func applyFileManagedCLIConfig(cfg *ManagedCLIProviderConfig, fileCfg fileManagedCLIProviderConfig) {
	if fileCfg.CLIPath != "" {
		cfg.CLIPath = fileCfg.CLIPath
	}
	if fileCfg.DefaultModel != "" {
		cfg.DefaultModel = fileCfg.DefaultModel
	}
	if fileCfg.WorkDir != "" {
		cfg.WorkDir = fileCfg.WorkDir
	}
}

func applyFileConnectorConfig(cfg *ConnectorConfig, fileCfg fileConnectorConfig) {
	if fileCfg.Discord != nil {
		applyFileDiscordConnectorConfig(&cfg.Discord, *fileCfg.Discord)
	}
	if fileCfg.Telegram != nil {
		applyFileTelegramConnectorConfig(&cfg.Telegram, *fileCfg.Telegram)
	}
	if fileCfg.Slack != nil {
		applyFileSlackConnectorConfig(&cfg.Slack, *fileCfg.Slack)
	}
	if fileCfg.Matrix != nil {
		applyFileMatrixConnectorConfig(&cfg.Matrix, *fileCfg.Matrix)
	}
}

func applyFileDiscordConnectorConfig(cfg *DiscordConnectorConfig, fileCfg fileDiscordConnectorConfig) {
	if fileCfg.Enabled != nil {
		cfg.Enabled = *fileCfg.Enabled
	}
	if fileCfg.ConnectorID != "" {
		cfg.ConnectorID = fileCfg.ConnectorID
	}
	if fileCfg.DisplayName != "" {
		cfg.DisplayName = fileCfg.DisplayName
	}
	if fileCfg.DeliveryMode != "" {
		cfg.DeliveryMode = fileCfg.DeliveryMode
	}
	if fileCfg.BotToken != "" {
		cfg.BotToken = fileCfg.BotToken
	}
	if fileCfg.BotTokenEnv != "" {
		cfg.BotTokenEnv = fileCfg.BotTokenEnv
	}
	if fileCfg.RequireMention != nil {
		cfg.RequireMention = *fileCfg.RequireMention
	}
	if fileCfg.RespondInDM != nil {
		cfg.RespondInDM = *fileCfg.RespondInDM
	}
	if fileCfg.AllowedGuildIDs != nil {
		cfg.AllowedGuildIDs = append([]string(nil), fileCfg.AllowedGuildIDs...)
	}
	if fileCfg.AllowedChannelIDs != nil {
		cfg.AllowedChannelIDs = append([]string(nil), fileCfg.AllowedChannelIDs...)
	}
}

func applyFileTelegramConnectorConfig(cfg *TelegramConnectorConfig, fileCfg fileTelegramConnectorConfig) {
	if fileCfg.Enabled != nil {
		cfg.Enabled = *fileCfg.Enabled
	}
	if fileCfg.ConnectorID != "" {
		cfg.ConnectorID = fileCfg.ConnectorID
	}
	if fileCfg.DisplayName != "" {
		cfg.DisplayName = fileCfg.DisplayName
	}
	if fileCfg.BotToken != "" {
		cfg.BotToken = fileCfg.BotToken
	}
	if fileCfg.BotTokenEnv != "" {
		cfg.BotTokenEnv = fileCfg.BotTokenEnv
	}
	if fileCfg.BotAPIBaseURL != "" {
		cfg.BotAPIBaseURL = fileCfg.BotAPIBaseURL
	}
	if fileCfg.BotUsername != "" {
		cfg.BotUsername = fileCfg.BotUsername
	}
	if fileCfg.AllowedUserIDs != nil {
		cfg.AllowedUserIDs = append([]string(nil), fileCfg.AllowedUserIDs...)
	}
	if fileCfg.AllowedDirectChatIDs != nil {
		cfg.AllowedDirectChatIDs = append([]string(nil), fileCfg.AllowedDirectChatIDs...)
	}
	if fileCfg.AllowedGroupIDs != nil {
		cfg.AllowedGroupIDs = append([]string(nil), fileCfg.AllowedGroupIDs...)
	}
}

func applyFileSlackConnectorConfig(cfg *SlackConnectorConfig, fileCfg fileSlackConnectorConfig) {
	if fileCfg.Enabled != nil {
		cfg.Enabled = *fileCfg.Enabled
	}
	if fileCfg.ConnectorID != "" {
		cfg.ConnectorID = fileCfg.ConnectorID
	}
	if fileCfg.DisplayName != "" {
		cfg.DisplayName = fileCfg.DisplayName
	}
	if fileCfg.APIBaseURL != "" {
		cfg.APIBaseURL = fileCfg.APIBaseURL
	}
	if fileCfg.BotTokenSecretRef != "" {
		cfg.BotTokenSecretRef = fileCfg.BotTokenSecretRef
	}
	if fileCfg.OAuthClientID != "" {
		cfg.OAuthClientID = fileCfg.OAuthClientID
	}
	if fileCfg.OAuthClientSecret != "" {
		cfg.OAuthClientSecret = fileCfg.OAuthClientSecret
	}
	if fileCfg.OAuthClientSecretEnv != "" {
		cfg.OAuthClientSecretEnv = fileCfg.OAuthClientSecretEnv
	}
	if fileCfg.OAuthAPIBaseURL != "" {
		cfg.OAuthAPIBaseURL = fileCfg.OAuthAPIBaseURL
	}
	if fileCfg.WorkspaceBindingID != "" {
		cfg.WorkspaceBindingID = fileCfg.WorkspaceBindingID
	}
	if fileCfg.WorkspaceID != "" {
		cfg.WorkspaceID = fileCfg.WorkspaceID
	}
	if fileCfg.BotUserID != "" {
		cfg.BotUserID = fileCfg.BotUserID
	}
	if fileCfg.AllowedChannelIDs != nil {
		cfg.AllowedChannelIDs = append([]string(nil), fileCfg.AllowedChannelIDs...)
	}
	if fileCfg.AllowedDMUserIDs != nil {
		cfg.AllowedDMUserIDs = append([]string(nil), fileCfg.AllowedDMUserIDs...)
	}
	if fileCfg.AllowedDMUserGroups != nil {
		cfg.AllowedDMUserGroups = append([]string(nil), fileCfg.AllowedDMUserGroups...)
	}
}

func applyFileMatrixConnectorConfig(cfg *MatrixConnectorConfig, fileCfg fileMatrixConnectorConfig) {
	if fileCfg.Enabled != nil {
		cfg.Enabled = *fileCfg.Enabled
	}
	if fileCfg.ConnectorID != "" {
		cfg.ConnectorID = fileCfg.ConnectorID
	}
	if fileCfg.DisplayName != "" {
		cfg.DisplayName = fileCfg.DisplayName
	}
	if fileCfg.HomeserverURL != "" {
		cfg.HomeserverURL = fileCfg.HomeserverURL
	}
	if fileCfg.HomeserverID != "" {
		cfg.HomeserverID = fileCfg.HomeserverID
	}
	if fileCfg.BotUserID != "" {
		cfg.BotUserID = fileCfg.BotUserID
	}
	if fileCfg.BotAccessToken != "" {
		cfg.BotAccessToken = fileCfg.BotAccessToken
	}
	if fileCfg.BotAccessTokenEnv != "" {
		cfg.BotAccessTokenEnv = fileCfg.BotAccessTokenEnv
	}
	if fileCfg.SelectedRoomIDs != nil {
		cfg.SelectedRoomIDs = append([]string(nil), fileCfg.SelectedRoomIDs...)
	}
	if fileCfg.AllowedDirectUserIDs != nil {
		cfg.AllowedDirectUserIDs = append([]string(nil), fileCfg.AllowedDirectUserIDs...)
	}
	if fileCfg.ConfiguredCommands != nil {
		cfg.ConfiguredCommands = append([]string(nil), fileCfg.ConfiguredCommands...)
	}
}

func applyEnvOverrides(cfg *Config) {
	if envName := resolveEnvironment(getenv("DOPE_ENV", ""), cfg.Version); envName != "" {
		cfg.Environment = envName
	}
	cfg.BindAddr = getenv("DOPE_BIND_ADDR", cfg.BindAddr)
	cfg.DataDir = getenv("DOPE_DATA_DIR", cfg.DataDir)
	cfg.LogLevel = getenv("DOPE_LOG_LEVEL", cfg.LogLevel)
	cfg.Version = getenv("DOPE_VERSION", cfg.Version)

	cfg.LLM.DefaultProvider = getenv("DOPE_LLM_DEFAULT_PROVIDER", cfg.LLM.DefaultProvider)
	cfg.LLM.DefaultModel = getenv("DOPE_LLM_DEFAULT_MODEL", cfg.LLM.DefaultModel)
	cfg.LLM.DefaultTimeoutMs = getenvInt("DOPE_LLM_DEFAULT_TIMEOUT_MS", cfg.LLM.DefaultTimeoutMs)
	cfg.LLM.DefaultMaxRetries = getenvInt("DOPE_LLM_DEFAULT_MAX_RETRIES", cfg.LLM.DefaultMaxRetries)

	cfg.LLM.OpenAICompatible.BaseURL = getenv("DOPE_LLM_OPENAI_COMPATIBLE_BASE_URL", cfg.LLM.OpenAICompatible.BaseURL)
	cfg.LLM.OpenAICompatible.APIKey = getenv("DOPE_LLM_OPENAI_COMPATIBLE_API_KEY", cfg.LLM.OpenAICompatible.APIKey)
	cfg.LLM.OpenAICompatible.APIKeyEnv = getenv("DOPE_LLM_OPENAI_COMPATIBLE_API_KEY_ENV", cfg.LLM.OpenAICompatible.APIKeyEnv)
	cfg.LLM.OpenAICompatible.Model = getenv("DOPE_LLM_OPENAI_COMPATIBLE_MODEL", cfg.LLM.OpenAICompatible.Model)
	cfg.LLM.OpenAICompatible.TimeoutMs = getenvInt("DOPE_LLM_OPENAI_COMPATIBLE_TIMEOUT_MS", cfg.LLM.OpenAICompatible.TimeoutMs)
	cfg.LLM.OpenAICompatible.StreamFirstChunkTimeoutMs = getenvInt("DOPE_LLM_OPENAI_COMPATIBLE_STREAM_FIRST_CHUNK_TIMEOUT_MS", cfg.LLM.OpenAICompatible.StreamFirstChunkTimeoutMs)
	cfg.LLM.OpenAICompatible.StreamIdleTimeoutMs = getenvInt("DOPE_LLM_OPENAI_COMPATIBLE_STREAM_IDLE_TIMEOUT_MS", cfg.LLM.OpenAICompatible.StreamIdleTimeoutMs)
	cfg.LLM.OpenAICompatible.StreamMaxDurationMs = getenvInt("DOPE_LLM_OPENAI_COMPATIBLE_STREAM_MAX_DURATION_MS", cfg.LLM.OpenAICompatible.StreamMaxDurationMs)
	cfg.LLM.Claude.CLIPath = getenv("DOPE_LLM_CLAUDE_CLI_PATH", cfg.LLM.Claude.CLIPath)
	cfg.LLM.Claude.DefaultModel = getenv("DOPE_LLM_CLAUDE_MODEL", cfg.LLM.Claude.DefaultModel)
	cfg.LLM.Claude.WorkDir = getenv("DOPE_LLM_CLAUDE_WORKDIR", cfg.LLM.Claude.WorkDir)
	cfg.LLM.Codex.CLIPath = getenv("DOPE_LLM_CODEX_CLI_PATH", cfg.LLM.Codex.CLIPath)
	cfg.LLM.Codex.DefaultModel = getenv("DOPE_LLM_CODEX_MODEL", cfg.LLM.Codex.DefaultModel)
	cfg.LLM.Codex.WorkDir = getenv("DOPE_LLM_CODEX_WORKDIR", cfg.LLM.Codex.WorkDir)

	cfg.Connectors.Discord.Enabled = getenvBool("DOPE_CONNECTORS_DISCORD_ENABLED", cfg.Connectors.Discord.Enabled)
	cfg.Connectors.Discord.ConnectorID = getenv("DOPE_CONNECTORS_DISCORD_CONNECTOR_ID", cfg.Connectors.Discord.ConnectorID)
	cfg.Connectors.Discord.DisplayName = getenv("DOPE_CONNECTORS_DISCORD_DISPLAY_NAME", cfg.Connectors.Discord.DisplayName)
	cfg.Connectors.Discord.DeliveryMode = getenv("DOPE_CONNECTORS_DISCORD_DELIVERY_MODE", cfg.Connectors.Discord.DeliveryMode)
	cfg.Connectors.Discord.BotToken = getenv("DOPE_CONNECTORS_DISCORD_BOT_TOKEN", cfg.Connectors.Discord.BotToken)
	cfg.Connectors.Discord.BotTokenEnv = getenv("DOPE_CONNECTORS_DISCORD_BOT_TOKEN_ENV", cfg.Connectors.Discord.BotTokenEnv)
	cfg.Connectors.Discord.RequireMention = getenvBool("DOPE_CONNECTORS_DISCORD_REQUIRE_MENTION", cfg.Connectors.Discord.RequireMention)
	cfg.Connectors.Discord.RespondInDM = getenvBool("DOPE_CONNECTORS_DISCORD_RESPOND_IN_DM", cfg.Connectors.Discord.RespondInDM)
	cfg.Connectors.Discord.AllowedGuildIDs = getenvCSV("DOPE_CONNECTORS_DISCORD_ALLOWED_GUILD_IDS", cfg.Connectors.Discord.AllowedGuildIDs)
	cfg.Connectors.Discord.AllowedChannelIDs = getenvCSV("DOPE_CONNECTORS_DISCORD_ALLOWED_CHANNEL_IDS", cfg.Connectors.Discord.AllowedChannelIDs)

	cfg.Connectors.Telegram.Enabled = getenvBool("DOPE_CONNECTORS_TELEGRAM_ENABLED", cfg.Connectors.Telegram.Enabled)
	cfg.Connectors.Telegram.ConnectorID = getenv("DOPE_CONNECTORS_TELEGRAM_CONNECTOR_ID", cfg.Connectors.Telegram.ConnectorID)
	cfg.Connectors.Telegram.DisplayName = getenv("DOPE_CONNECTORS_TELEGRAM_DISPLAY_NAME", cfg.Connectors.Telegram.DisplayName)
	cfg.Connectors.Telegram.BotToken = getenv("DOPE_CONNECTORS_TELEGRAM_BOT_TOKEN", cfg.Connectors.Telegram.BotToken)
	cfg.Connectors.Telegram.BotTokenEnv = getenv("DOPE_CONNECTORS_TELEGRAM_BOT_TOKEN_ENV", cfg.Connectors.Telegram.BotTokenEnv)
	cfg.Connectors.Telegram.BotAPIBaseURL = getenv("DOPE_CONNECTORS_TELEGRAM_BOT_API_BASE_URL", cfg.Connectors.Telegram.BotAPIBaseURL)
	cfg.Connectors.Telegram.BotUsername = getenv("DOPE_CONNECTORS_TELEGRAM_BOT_USERNAME", cfg.Connectors.Telegram.BotUsername)
	cfg.Connectors.Telegram.AllowedUserIDs = getenvCSV("DOPE_CONNECTORS_TELEGRAM_ALLOWED_USER_IDS", cfg.Connectors.Telegram.AllowedUserIDs)
	cfg.Connectors.Telegram.AllowedDirectChatIDs = getenvCSV("DOPE_CONNECTORS_TELEGRAM_ALLOWED_DIRECT_CHAT_IDS", cfg.Connectors.Telegram.AllowedDirectChatIDs)
	cfg.Connectors.Telegram.AllowedGroupIDs = getenvCSV("DOPE_CONNECTORS_TELEGRAM_ALLOWED_GROUP_IDS", cfg.Connectors.Telegram.AllowedGroupIDs)

	cfg.Connectors.Slack.Enabled = getenvBool("DOPE_CONNECTORS_SLACK_ENABLED", cfg.Connectors.Slack.Enabled)
	cfg.Connectors.Slack.ConnectorID = getenv("DOPE_CONNECTORS_SLACK_CONNECTOR_ID", cfg.Connectors.Slack.ConnectorID)
	cfg.Connectors.Slack.DisplayName = getenv("DOPE_CONNECTORS_SLACK_DISPLAY_NAME", cfg.Connectors.Slack.DisplayName)
	cfg.Connectors.Slack.APIBaseURL = getenv("DOPE_CONNECTORS_SLACK_API_BASE_URL", cfg.Connectors.Slack.APIBaseURL)
	cfg.Connectors.Slack.BotTokenSecretRef = getenv("DOPE_CONNECTORS_SLACK_BOT_TOKEN_SECRET_REF", cfg.Connectors.Slack.BotTokenSecretRef)
	cfg.Connectors.Slack.OAuthClientID = getenv("DOPE_CONNECTORS_SLACK_OAUTH_CLIENT_ID", cfg.Connectors.Slack.OAuthClientID)
	cfg.Connectors.Slack.OAuthClientSecret = getenv("DOPE_CONNECTORS_SLACK_OAUTH_CLIENT_SECRET", cfg.Connectors.Slack.OAuthClientSecret)
	cfg.Connectors.Slack.OAuthClientSecretEnv = getenv("DOPE_CONNECTORS_SLACK_OAUTH_CLIENT_SECRET_ENV", cfg.Connectors.Slack.OAuthClientSecretEnv)
	cfg.Connectors.Slack.OAuthAPIBaseURL = getenv("DOPE_CONNECTORS_SLACK_OAUTH_API_BASE_URL", cfg.Connectors.Slack.OAuthAPIBaseURL)
	cfg.Connectors.Slack.WorkspaceBindingID = getenv("DOPE_CONNECTORS_SLACK_WORKSPACE_BINDING_ID", cfg.Connectors.Slack.WorkspaceBindingID)
	cfg.Connectors.Slack.WorkspaceID = getenv("DOPE_CONNECTORS_SLACK_WORKSPACE_ID", cfg.Connectors.Slack.WorkspaceID)
	cfg.Connectors.Slack.BotUserID = getenv("DOPE_CONNECTORS_SLACK_BOT_USER_ID", cfg.Connectors.Slack.BotUserID)
	cfg.Connectors.Slack.AllowedChannelIDs = getenvCSV("DOPE_CONNECTORS_SLACK_ALLOWED_CHANNEL_IDS", cfg.Connectors.Slack.AllowedChannelIDs)
	cfg.Connectors.Slack.AllowedDMUserIDs = getenvCSV("DOPE_CONNECTORS_SLACK_ALLOWED_DM_USER_IDS", cfg.Connectors.Slack.AllowedDMUserIDs)
	cfg.Connectors.Slack.AllowedDMUserGroups = getenvCSV("DOPE_CONNECTORS_SLACK_ALLOWED_DM_USER_GROUPS", cfg.Connectors.Slack.AllowedDMUserGroups)

	cfg.Connectors.Matrix.Enabled = getenvBool("DOPE_CONNECTORS_MATRIX_ENABLED", cfg.Connectors.Matrix.Enabled)
	cfg.Connectors.Matrix.ConnectorID = getenv("DOPE_CONNECTORS_MATRIX_CONNECTOR_ID", cfg.Connectors.Matrix.ConnectorID)
	cfg.Connectors.Matrix.DisplayName = getenv("DOPE_CONNECTORS_MATRIX_DISPLAY_NAME", cfg.Connectors.Matrix.DisplayName)
	cfg.Connectors.Matrix.HomeserverURL = getenv("DOPE_CONNECTORS_MATRIX_HOMESERVER_URL", cfg.Connectors.Matrix.HomeserverURL)
	cfg.Connectors.Matrix.HomeserverID = getenv("DOPE_CONNECTORS_MATRIX_HOMESERVER_ID", cfg.Connectors.Matrix.HomeserverID)
	cfg.Connectors.Matrix.BotUserID = getenv("DOPE_CONNECTORS_MATRIX_BOT_USER_ID", cfg.Connectors.Matrix.BotUserID)
	cfg.Connectors.Matrix.BotAccessToken = getenv("DOPE_CONNECTORS_MATRIX_BOT_ACCESS_TOKEN", cfg.Connectors.Matrix.BotAccessToken)
	cfg.Connectors.Matrix.BotAccessTokenEnv = getenv("DOPE_CONNECTORS_MATRIX_BOT_ACCESS_TOKEN_ENV", cfg.Connectors.Matrix.BotAccessTokenEnv)
	cfg.Connectors.Matrix.SelectedRoomIDs = getenvCSV("DOPE_CONNECTORS_MATRIX_SELECTED_ROOM_IDS", cfg.Connectors.Matrix.SelectedRoomIDs)
	cfg.Connectors.Matrix.AllowedDirectUserIDs = getenvCSV("DOPE_CONNECTORS_MATRIX_ALLOWED_DIRECT_USER_IDS", cfg.Connectors.Matrix.AllowedDirectUserIDs)
	cfg.Connectors.Matrix.ConfiguredCommands = getenvCSV("DOPE_CONNECTORS_MATRIX_CONFIGURED_COMMANDS", cfg.Connectors.Matrix.ConfiguredCommands)
}

func resolveSecretRefs(cfg *Config) {
	if cfg.LLM.OpenAICompatible.APIKey == "" && cfg.LLM.OpenAICompatible.APIKeyEnv != "" {
		cfg.LLM.OpenAICompatible.APIKey = os.Getenv(cfg.LLM.OpenAICompatible.APIKeyEnv)
	}
	if cfg.Connectors.Discord.BotToken == "" && cfg.Connectors.Discord.BotTokenEnv != "" {
		cfg.Connectors.Discord.BotToken = os.Getenv(cfg.Connectors.Discord.BotTokenEnv)
	}
	if cfg.Connectors.Telegram.BotToken == "" && cfg.Connectors.Telegram.BotTokenEnv != "" {
		cfg.Connectors.Telegram.BotToken = os.Getenv(cfg.Connectors.Telegram.BotTokenEnv)
	}
	if cfg.Connectors.Slack.OAuthClientSecret == "" && cfg.Connectors.Slack.OAuthClientSecretEnv != "" {
		cfg.Connectors.Slack.OAuthClientSecret = os.Getenv(cfg.Connectors.Slack.OAuthClientSecretEnv)
	}
	if cfg.Connectors.Matrix.BotAccessToken == "" && cfg.Connectors.Matrix.BotAccessTokenEnv != "" {
		cfg.Connectors.Matrix.BotAccessToken = os.Getenv(cfg.Connectors.Matrix.BotAccessTokenEnv)
	}
}

func resolveEnvironment(raw, version string) Environment {
	if envName := normalizeEnvironment(raw); envName != "" {
		return envName
	}
	if strings.EqualFold(strings.TrimSpace(version), "dev") {
		return EnvironmentTest
	}
	return EnvironmentProd
}

func normalizeEnvironment(raw string) Environment {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "prod", "production":
		return EnvironmentProd
	case "test", "testing", "dev", "development":
		return EnvironmentTest
	default:
		return ""
	}
}

func defaultDataDir(env Environment) string {
	switch env {
	case EnvironmentTest:
		return "~/.dope-test"
	default:
		return "~/.dope"
	}
}

func defaultBindAddr(env Environment) string {
	switch env {
	case EnvironmentTest:
		return "127.0.0.1:19192"
	default:
		return "127.0.0.1:19191"
	}
}

func loadFileConfig(path string) (fileConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileConfig{}, nil
		}
		return fileConfig{}, fmt.Errorf("read config file %s: %w", path, err)
	}

	var cfg fileConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fileConfig{}, fmt.Errorf("decode config file %s: %w", path, err)
	}

	return cfg, nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
