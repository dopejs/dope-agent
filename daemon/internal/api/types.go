package api

import (
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
)

type SystemInfoResponse struct {
	Service  string `json:"service"`
	Version  string `json:"version"`
	BindAddr string `json:"bindAddr"`
	DataDir  string `json:"dataDir"`
	LogLevel string `json:"logLevel"`
}

type ConfigResponse struct {
	BindAddr       string   `json:"bindAddr"`
	DataDir        string   `json:"dataDir"`
	ConfigFilePath string   `json:"configFilePath"`
	LogLevel       string   `json:"logLevel"`
	Version        string   `json:"version"`
	RedactedFields []string `json:"redactedFields"`
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
	return ConfigResponse{
		BindAddr:       cfg.BindAddr,
		DataDir:        cfg.DataDir,
		ConfigFilePath: config.ConfigFilePath(cfg.DataDir),
		LogLevel:       cfg.LogLevel,
		Version:        cfg.Version,
		RedactedFields: []string{},
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
