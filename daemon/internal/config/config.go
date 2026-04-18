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
	Discord DiscordConnectorConfig
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
	Discord *fileDiscordConnectorConfig `json:"discord"`
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
	if fileCfg.Discord == nil {
		return
	}
	applyFileDiscordConnectorConfig(&cfg.Discord, *fileCfg.Discord)
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
}

func resolveSecretRefs(cfg *Config) {
	if cfg.LLM.OpenAICompatible.APIKey == "" && cfg.LLM.OpenAICompatible.APIKeyEnv != "" {
		cfg.LLM.OpenAICompatible.APIKey = os.Getenv(cfg.LLM.OpenAICompatible.APIKeyEnv)
	}
	if cfg.Connectors.Discord.BotToken == "" && cfg.Connectors.Discord.BotTokenEnv != "" {
		cfg.Connectors.Discord.BotToken = os.Getenv(cfg.Connectors.Discord.BotTokenEnv)
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
