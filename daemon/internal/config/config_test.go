package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInitializesDefaultDopeDir(t *testing.T) {
	homeDir := t.TempDir()
	setBaseEnv(t, homeDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	expectedDataDir := filepath.Join(homeDir, ".dope-test")
	if cfg.DataDir != expectedDataDir {
		t.Fatalf("expected data dir %s, got %s", expectedDataDir, cfg.DataDir)
	}
	if cfg.Environment != EnvironmentTest {
		t.Fatalf("expected test environment, got %s", cfg.Environment)
	}
	if _, err := os.Stat(expectedDataDir); err != nil {
		t.Fatalf("expected data dir to exist: %v", err)
	}
	if cfg.LLM.DefaultTimeoutMs != 30000 {
		t.Fatalf("expected default llm timeout 30000, got %d", cfg.LLM.DefaultTimeoutMs)
	}
}

func TestLoadReadsConfigFileFromDopeDir(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope-test")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"bindAddr": "127.0.0.1:19000",
		"logLevel": "debug",
		"llm": {
			"defaultProvider": "openai_compatible",
			"defaultModel": "gpt-test",
			"defaultTimeoutMs": 45000,
			"openaiCompatible": {
				"baseURL": "https://api.example.com/v1",
				"apiKeyEnv": "OPENAI_TEST_KEY",
				"model": "gpt-provider"
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	setBaseEnv(t, homeDir)
	t.Setenv("OPENAI_TEST_KEY", "secret-from-env")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.BindAddr != "127.0.0.1:19000" {
		t.Fatalf("expected bind addr from config file, got %s", cfg.BindAddr)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected log level from config file, got %s", cfg.LogLevel)
	}
	if cfg.LLM.DefaultProvider != "openai_compatible" {
		t.Fatalf("expected default provider openai_compatible, got %s", cfg.LLM.DefaultProvider)
	}
	if cfg.LLM.DefaultModel != "gpt-test" {
		t.Fatalf("expected default model gpt-test, got %s", cfg.LLM.DefaultModel)
	}
	if cfg.LLM.OpenAICompatible.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("expected base URL from config file, got %s", cfg.LLM.OpenAICompatible.BaseURL)
	}
	if cfg.LLM.OpenAICompatible.APIKey != "secret-from-env" {
		t.Fatalf("expected API key from apiKeyEnv, got %q", cfg.LLM.OpenAICompatible.APIKey)
	}
}

func TestLoadEnvironmentOverridesConfigFile(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope-test")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"bindAddr": "127.0.0.1:19000",
		"logLevel": "debug",
		"llm": {
			"defaultProvider": "openai_compatible",
			"defaultModel": "gpt-file",
			"openaiCompatible": {
				"baseURL": "https://api.file.example/v1",
				"apiKey": "file-secret",
				"model": "gpt-file-provider"
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	overrideDir := filepath.Join(homeDir, "custom-dope")
	setBaseEnv(t, homeDir)
	t.Setenv("DOPE_DATA_DIR", overrideDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:19999")
	t.Setenv("DOPE_LOG_LEVEL", "warn")
	t.Setenv("DOPE_VERSION", "test")
	t.Setenv("DOPE_LLM_DEFAULT_MODEL", "gpt-env")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_BASE_URL", "https://api.env.example/v1")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_API_KEY", "env-secret")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_MODEL", "gpt-env-provider")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.DataDir != overrideDir {
		t.Fatalf("expected overridden data dir %s, got %s", overrideDir, cfg.DataDir)
	}
	if cfg.BindAddr != "127.0.0.1:19999" {
		t.Fatalf("expected overridden bind addr, got %s", cfg.BindAddr)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("expected overridden log level, got %s", cfg.LogLevel)
	}
	if cfg.Version != "test" {
		t.Fatalf("expected overridden version, got %s", cfg.Version)
	}
	if cfg.LLM.DefaultModel != "gpt-env" {
		t.Fatalf("expected overridden default model, got %s", cfg.LLM.DefaultModel)
	}
	if cfg.LLM.OpenAICompatible.BaseURL != "https://api.env.example/v1" {
		t.Fatalf("expected overridden provider base URL, got %s", cfg.LLM.OpenAICompatible.BaseURL)
	}
	if cfg.LLM.OpenAICompatible.APIKey != "env-secret" {
		t.Fatalf("expected overridden provider api key, got %q", cfg.LLM.OpenAICompatible.APIKey)
	}
	if _, err := os.Stat(overrideDir); err != nil {
		t.Fatalf("expected overridden data dir to exist: %v", err)
	}
}

func TestLoadManagedCLIProviderConfig(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope-test")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"llm": {
			"claude": {
				"cliPath": "/usr/local/bin/claude",
				"defaultModel": "claude-opus-4-6",
				"workDir": "~/workspaces/claude"
			},
			"codex": {
				"cliPath": "/opt/homebrew/bin/codex",
				"defaultModel": "gpt-5.4",
				"workDir": "~/workspaces/codex"
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	setBaseEnv(t, homeDir)
	t.Setenv("DOPE_LLM_CLAUDE_MODEL", "claude-sonnet-4-6")
	t.Setenv("DOPE_LLM_CODEX_WORKDIR", "~/projects/codex")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLM.Claude.CLIPath != "/usr/local/bin/claude" {
		t.Fatalf("expected claude cli path, got %s", cfg.LLM.Claude.CLIPath)
	}
	if cfg.LLM.Claude.DefaultModel != "claude-sonnet-4-6" {
		t.Fatalf("expected env override for claude model, got %s", cfg.LLM.Claude.DefaultModel)
	}
	if cfg.LLM.Codex.CLIPath != "/opt/homebrew/bin/codex" {
		t.Fatalf("expected codex cli path, got %s", cfg.LLM.Codex.CLIPath)
	}
	if cfg.LLM.Codex.WorkDir != "~/projects/codex" {
		t.Fatalf("expected env override for codex workdir, got %s", cfg.LLM.Codex.WorkDir)
	}
}

func TestLoadDiscordConnectorConfig(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope-test")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"connectors": {
			"discord": {
				"enabled": true,
				"connectorId": "discord-bot",
				"displayName": "Discord Bot",
				"deliveryMode": "gateway",
				"botTokenEnv": "DISCORD_TEST_TOKEN",
				"requireMention": false,
				"respondInDM": true,
				"allowedGuildIds": ["guild_1"],
				"allowedChannelIds": ["channel_1", "channel_2"]
			}
		}
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	setBaseEnv(t, homeDir)
	t.Setenv("DISCORD_TEST_TOKEN", "discord-secret")
	t.Setenv("DOPE_CONNECTORS_DISCORD_ALLOWED_CHANNEL_IDS", "channel_3,channel_4")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Connectors.Discord.Enabled {
		t.Fatal("expected discord connector enabled")
	}
	if cfg.Connectors.Discord.ConnectorID != "discord-bot" {
		t.Fatalf("expected discord connector id discord-bot, got %s", cfg.Connectors.Discord.ConnectorID)
	}
	if cfg.Connectors.Discord.BotToken != "discord-secret" {
		t.Fatalf("expected discord bot token from env ref, got %q", cfg.Connectors.Discord.BotToken)
	}
	if cfg.Connectors.Discord.DeliveryMode != "gateway" {
		t.Fatalf("expected discord delivery mode gateway, got %q", cfg.Connectors.Discord.DeliveryMode)
	}
	if cfg.Connectors.Discord.RequireMention {
		t.Fatal("expected requireMention=false from file config")
	}
	if got := cfg.Connectors.Discord.AllowedGuildIDs; len(got) != 1 || got[0] != "guild_1" {
		t.Fatalf("expected allowed guild IDs from file config, got %#v", got)
	}
	if got := cfg.Connectors.Discord.AllowedChannelIDs; len(got) != 2 || got[0] != "channel_3" || got[1] != "channel_4" {
		t.Fatalf("expected allowed channel IDs from env override, got %#v", got)
	}
}

func TestLoadRejectsInvalidConfigFile(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope-test")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{"bindAddr":`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	setBaseEnv(t, homeDir)

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to fail for invalid config file")
	}
}

func TestLoadUsesProdDefaultsWhenEnvironmentIsProd(t *testing.T) {
	homeDir := t.TempDir()
	setBaseEnv(t, homeDir)
	t.Setenv("DOPE_ENV", "prod")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	expectedDataDir := filepath.Join(homeDir, ".dope")
	if cfg.Environment != EnvironmentProd {
		t.Fatalf("expected prod environment, got %s", cfg.Environment)
	}
	if cfg.DataDir != expectedDataDir {
		t.Fatalf("expected prod data dir %s, got %s", expectedDataDir, cfg.DataDir)
	}
	if cfg.BindAddr != "127.0.0.1:19191" {
		t.Fatalf("expected prod bind addr 127.0.0.1:19191, got %s", cfg.BindAddr)
	}
}

func setBaseEnv(t *testing.T, homeDir string) {
	t.Helper()
	t.Setenv("HOME", homeDir)
	t.Setenv("DOPE_ENV", "")
	t.Setenv("DOPE_DATA_DIR", "")
	t.Setenv("DOPE_BIND_ADDR", "")
	t.Setenv("DOPE_LOG_LEVEL", "")
	t.Setenv("DOPE_VERSION", "")
	t.Setenv("DOPE_LLM_DEFAULT_PROVIDER", "")
	t.Setenv("DOPE_LLM_DEFAULT_MODEL", "")
	t.Setenv("DOPE_LLM_DEFAULT_TIMEOUT_MS", "")
	t.Setenv("DOPE_LLM_DEFAULT_MAX_RETRIES", "")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_BASE_URL", "")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_API_KEY", "")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_API_KEY_ENV", "")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_MODEL", "")
	t.Setenv("DOPE_LLM_OPENAI_COMPATIBLE_TIMEOUT_MS", "")
	t.Setenv("DOPE_LLM_CLAUDE_CLI_PATH", "")
	t.Setenv("DOPE_LLM_CLAUDE_MODEL", "")
	t.Setenv("DOPE_LLM_CLAUDE_WORKDIR", "")
	t.Setenv("DOPE_LLM_CODEX_CLI_PATH", "")
	t.Setenv("DOPE_LLM_CODEX_MODEL", "")
	t.Setenv("DOPE_LLM_CODEX_WORKDIR", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_ENABLED", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_CONNECTOR_ID", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_DISPLAY_NAME", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_DELIVERY_MODE", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_BOT_TOKEN", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_BOT_TOKEN_ENV", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_REQUIRE_MENTION", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_RESPOND_IN_DM", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_ALLOWED_GUILD_IDS", "")
	t.Setenv("DOPE_CONNECTORS_DISCORD_ALLOWED_CHANNEL_IDS", "")
	t.Setenv("OPENAI_TEST_KEY", "")
	t.Setenv("DISCORD_TEST_TOKEN", "")
}
