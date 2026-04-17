package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultConfigFileName = "config.json"

type Config struct {
	BindAddr string
	DataDir  string
	LogLevel string
	Version  string
}

type fileConfig struct {
	BindAddr string `json:"bindAddr"`
	DataDir  string `json:"dataDir"`
	LogLevel string `json:"logLevel"`
}

func Load() (Config, error) {
	bootstrapDir, err := ResolveDir(getenv("DOPE_DATA_DIR", "~/.dope"))
	if err != nil {
		return Config{}, fmt.Errorf("resolve bootstrap data dir: %w", err)
	}
	if err := ensureDir(bootstrapDir); err != nil {
		return Config{}, fmt.Errorf("initialize bootstrap data dir: %w", err)
	}

	cfg := Config{
		BindAddr: "127.0.0.1:18789",
		DataDir:  bootstrapDir,
		LogLevel: "info",
		Version:  getenv("DOPE_VERSION", "dev"),
	}

	loadedFileConfig, err := loadFileConfig(filepath.Join(bootstrapDir, defaultConfigFileName))
	if err != nil {
		return Config{}, err
	}
	applyFileConfig(&cfg, loadedFileConfig)
	applyEnvOverrides(&cfg)

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
	if fileCfg.BindAddr != "" {
		cfg.BindAddr = fileCfg.BindAddr
	}
	if fileCfg.DataDir != "" {
		cfg.DataDir = fileCfg.DataDir
	}
	if fileCfg.LogLevel != "" {
		cfg.LogLevel = fileCfg.LogLevel
	}
}

func applyEnvOverrides(cfg *Config) {
	cfg.BindAddr = getenv("DOPE_BIND_ADDR", cfg.BindAddr)
	cfg.DataDir = getenv("DOPE_DATA_DIR", cfg.DataDir)
	cfg.LogLevel = getenv("DOPE_LOG_LEVEL", cfg.LogLevel)
	cfg.Version = getenv("DOPE_VERSION", cfg.Version)
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
