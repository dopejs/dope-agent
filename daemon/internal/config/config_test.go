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

	expectedDataDir := filepath.Join(homeDir, ".dope")
	if cfg.DataDir != expectedDataDir {
		t.Fatalf("expected data dir %s, got %s", expectedDataDir, cfg.DataDir)
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
	dataDir := filepath.Join(homeDir, ".dope")
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
	dataDir := filepath.Join(homeDir, ".dope")
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

func TestLoadRejectsInvalidConfigFile(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope")
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

func setBaseEnv(t *testing.T, homeDir string) {
	t.Helper()
	t.Setenv("HOME", homeDir)
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
	t.Setenv("OPENAI_TEST_KEY", "")
}
