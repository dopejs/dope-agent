package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInitializesDefaultDopeDir(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("DOPE_DATA_DIR", "")
	t.Setenv("DOPE_BIND_ADDR", "")
	t.Setenv("DOPE_LOG_LEVEL", "")
	t.Setenv("DOPE_VERSION", "")

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
}

func TestLoadReadsConfigFileFromDopeDir(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"bindAddr": "127.0.0.1:19000",
		"logLevel": "debug"
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("DOPE_DATA_DIR", "")
	t.Setenv("DOPE_BIND_ADDR", "")
	t.Setenv("DOPE_LOG_LEVEL", "")
	t.Setenv("DOPE_VERSION", "")

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
}

func TestLoadEnvironmentOverridesConfigFile(t *testing.T) {
	homeDir := t.TempDir()
	dataDir := filepath.Join(homeDir, ".dope")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{
		"bindAddr": "127.0.0.1:19000",
		"logLevel": "debug"
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	overrideDir := filepath.Join(homeDir, "custom-dope")
	t.Setenv("HOME", homeDir)
	t.Setenv("DOPE_DATA_DIR", overrideDir)
	t.Setenv("DOPE_BIND_ADDR", "127.0.0.1:19999")
	t.Setenv("DOPE_LOG_LEVEL", "warn")
	t.Setenv("DOPE_VERSION", "test")

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

	t.Setenv("HOME", homeDir)
	t.Setenv("DOPE_DATA_DIR", "")
	t.Setenv("DOPE_BIND_ADDR", "")
	t.Setenv("DOPE_LOG_LEVEL", "")
	t.Setenv("DOPE_VERSION", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected Load to fail for invalid config file")
	}
}
