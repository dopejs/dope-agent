package api

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
)

type SystemInfoResponse struct {
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
	BindAddr    string `json:"bindAddr"`
	DataDir     string `json:"dataDir"`
	LogLevel    string `json:"logLevel"`
}

type ConfigResponse struct {
	Environment    string                   `json:"environment"`
	BindAddr       string                   `json:"bindAddr"`
	DataDir        string                   `json:"dataDir"`
	ConfigFilePath string                   `json:"configFilePath"`
	LogLevel       string                   `json:"logLevel"`
	Version        string                   `json:"version"`
	LLM            ConfigLLMResponse        `json:"llm"`
	Connectors     ConfigConnectorsResponse `json:"connectors"`
	MCP            ConfigMCPResponse        `json:"mcp"`
	Sandbox        ConfigSandboxResponse    `json:"sandbox"`
	RedactedFields []string                 `json:"redactedFields"`
}

type ConfigLLMResponse struct {
	DefaultProvider   string                                 `json:"defaultProvider"`
	DefaultModel      string                                 `json:"defaultModel"`
	DefaultTimeoutMs  int                                    `json:"defaultTimeoutMs"`
	DefaultMaxRetries int                                    `json:"defaultMaxRetries"`
	OpenAICompatible  ConfigOpenAICompatibleProviderResponse `json:"openaiCompatible"`
	Claude            ConfigManagedCLIProviderResponse       `json:"claude"`
	Codex             ConfigManagedCLIProviderResponse       `json:"codex"`
}

type ConfigOpenAICompatibleProviderResponse struct {
	Configured                bool   `json:"configured"`
	BaseURL                   string `json:"baseURL"`
	Model                     string `json:"model"`
	TimeoutMs                 int    `json:"timeoutMs"`
	StreamFirstChunkTimeoutMs int    `json:"streamFirstChunkTimeoutMs"`
	StreamIdleTimeoutMs       int    `json:"streamIdleTimeoutMs"`
	StreamMaxDurationMs       int    `json:"streamMaxDurationMs"`
	APIKeyConfigured          bool   `json:"apiKeyConfigured"`
	APIKeyEnv                 string `json:"apiKeyEnv,omitempty"`
}

type ConfigManagedCLIProviderResponse struct {
	Configured   bool           `json:"configured"`
	CLIPath      string         `json:"cliPath,omitempty"`
	DefaultModel string         `json:"defaultModel,omitempty"`
	WorkDir      string         `json:"workDir,omitempty"`
	Sandbox      map[string]any `json:"sandbox,omitempty"`
}

type ConfigConnectorsResponse struct {
	Discord ConfigDiscordConnectorResponse `json:"discord"`
}

type ConfigMCPResponse struct {
	Servers    []mcp.ServerResource      `json:"servers"`
	Catalog    []mcp.CatalogEntry        `json:"catalog,omitempty"`
	Transports []mcp.TransportCapability `json:"transports"`
}

type ConfigSandboxResponse struct {
	Backends []sandbox.BackendCapabilityProfile `json:"backends"`
}

type ConfigDiscordConnectorResponse struct {
	Enabled            bool     `json:"enabled"`
	Configured         bool     `json:"configured"`
	ConnectorID        string   `json:"connectorId"`
	DisplayName        string   `json:"displayName"`
	DeliveryMode       string   `json:"deliveryMode"`
	RequireMention     bool     `json:"requireMention"`
	RespondInDM        bool     `json:"respondInDM"`
	AllowedGuildIDs    []string `json:"allowedGuildIds"`
	AllowedChannelIDs  []string `json:"allowedChannelIds"`
	BotTokenConfigured bool     `json:"botTokenConfigured"`
	BotTokenEnv        string   `json:"botTokenEnv,omitempty"`
}

type ChatQueryResponse struct {
	DispatchID     string           `json:"dispatchId"`
	Provider       string           `json:"provider"`
	Model          string           `json:"model"`
	Skills         []string         `json:"skills"`
	SkillContracts []map[string]any `json:"skillContracts,omitempty"`
	Query          string           `json:"query"`
	Status         string           `json:"status"`
	Partial        bool             `json:"partial"`
	Reply          string           `json:"reply"`
	FinishReason   string           `json:"finishReason,omitempty"`
	Usage          llm.Usage        `json:"usage"`
	ErrorCode      string           `json:"errorCode,omitempty"`
	Error          string           `json:"error,omitempty"`
}

type ChatQueryStreamStarted struct {
	DispatchID     string           `json:"dispatchId"`
	Provider       string           `json:"provider"`
	Model          string           `json:"model"`
	Skills         []string         `json:"skills"`
	SkillContracts []map[string]any `json:"skillContracts,omitempty"`
	Query          string           `json:"query"`
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

type ProviderListResponse struct {
	Items []providers.Profile `json:"items"`
}

type ProviderCheckListResponse struct {
	Items []providers.Check `json:"items"`
}

type ProviderAuthStateResponse struct {
	Auth providers.AuthState `json:"auth"`
}

type ProviderModelListResponse struct {
	Items []providers.Model `json:"items"`
}

type ProviderDefaultModelRequest struct {
	Model string `json:"model"`
}

type ProviderDefaultModelResponse struct {
	ProviderID   string    `json:"providerId"`
	DefaultModel string    `json:"defaultModel"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SkillFileResponse struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

type SkillSummaryResponse struct {
	SkillID            string                     `json:"skillId"`
	Name               string                     `json:"name"`
	Description        string                     `json:"description"`
	Source             string                     `json:"source"`
	RootPath           string                     `json:"rootPath"`
	SkillPath          string                     `json:"skillPath"`
	InstructionPath    string                     `json:"instructionPath"`
	Files              []SkillFileResponse        `json:"files"`
	Frontmatter        map[string]string          `json:"frontmatter"`
	ExecutionManifest  *skills.ExecutableManifest `json:"executionManifest,omitempty"`
	AvailabilityStatus string                     `json:"availabilityStatus,omitempty"`
	AvailabilityReason string                     `json:"availabilityReason,omitempty"`
	Sandbox            map[string]any             `json:"sandbox,omitempty"`
}

type SkillDetailResponse struct {
	SkillSummaryResponse
	FrontmatterRaw string `json:"frontmatterRaw,omitempty"`
	Body           string `json:"body"`
}

type SkillOverlayResponse struct {
	OverlayID  string    `json:"overlayId"`
	Source     string    `json:"source"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"sizeBytes"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

type SkillRegistryResponse struct {
	LoadedAt time.Time              `json:"loadedAt"`
	Items    []SkillSummaryResponse `json:"items"`
	Overlays []SkillOverlayResponse `json:"overlays"`
}

type SandboxExplainResponse struct {
	Decision sandbox.Decision `json:"decision"`
}

type ListResponse[T any] struct {
	Items []T `json:"items"`
}

func buildSystemInfoResponse(cfg config.Config) SystemInfoResponse {
	return SystemInfoResponse{
		Service:     "dope",
		Environment: effectiveEnvironment(cfg),
		Version:     cfg.Version,
		BindAddr:    cfg.BindAddr,
		DataDir:     cfg.DataDir,
		LogLevel:    cfg.LogLevel,
	}
}

func buildConfigResponse(cfg config.Config, mcpManager *mcp.Manager, sandboxManager *sandbox.Manager) ConfigResponse {
	redactedFields := []string{}
	if cfg.LLM.OpenAICompatible.APIKey != "" {
		redactedFields = append(redactedFields, "llm.openaiCompatible.apiKey")
	}
	if cfg.Connectors.Discord.BotToken != "" {
		redactedFields = append(redactedFields, "connectors.discord.botToken")
	}
	defaultTimeoutMs := cfg.LLM.DefaultTimeoutMs
	if defaultTimeoutMs <= 0 {
		defaultTimeoutMs = 30000
	}
	openAITimeoutMs := cfg.LLM.OpenAICompatible.TimeoutMs
	if openAITimeoutMs <= 0 {
		openAITimeoutMs = defaultTimeoutMs
	}
	firstChunkTimeoutMs := cfg.LLM.OpenAICompatible.StreamFirstChunkTimeoutMs
	if firstChunkTimeoutMs <= 0 {
		firstChunkTimeoutMs = openAITimeoutMs
	}
	idleTimeoutMs := cfg.LLM.OpenAICompatible.StreamIdleTimeoutMs
	if idleTimeoutMs <= 0 {
		idleTimeoutMs = firstChunkTimeoutMs
	}
	discordDeliveryMode := cfg.Connectors.Discord.DeliveryMode
	if discordDeliveryMode == "" {
		discordDeliveryMode = "gateway"
	}

	return ConfigResponse{
		Environment:    effectiveEnvironment(cfg),
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
				Configured:                cfg.LLM.OpenAICompatible.BaseURL != "" || cfg.LLM.OpenAICompatible.APIKey != "" || cfg.LLM.OpenAICompatible.Model != "",
				BaseURL:                   cfg.LLM.OpenAICompatible.BaseURL,
				Model:                     cfg.LLM.OpenAICompatible.Model,
				TimeoutMs:                 openAITimeoutMs,
				StreamFirstChunkTimeoutMs: firstChunkTimeoutMs,
				StreamIdleTimeoutMs:       idleTimeoutMs,
				StreamMaxDurationMs:       cfg.LLM.OpenAICompatible.StreamMaxDurationMs,
				APIKeyConfigured:          cfg.LLM.OpenAICompatible.APIKey != "",
				APIKeyEnv:                 cfg.LLM.OpenAICompatible.APIKeyEnv,
			},
			Claude: ConfigManagedCLIProviderResponse{
				Configured:   cfg.LLM.Claude.CLIPath != "" || cfg.LLM.Claude.DefaultModel != "" || (cfg.LLM.Claude.WorkDir != "" && cfg.LLM.Claude.WorkDir != "~"),
				CLIPath:      cfg.LLM.Claude.CLIPath,
				DefaultModel: cfg.LLM.Claude.DefaultModel,
				WorkDir:      cfg.LLM.Claude.WorkDir,
				Sandbox:      buildManagedProviderConfigSandbox("claude_managed", cfg.LLM.Claude.WorkDir),
			},
			Codex: ConfigManagedCLIProviderResponse{
				Configured:   cfg.LLM.Codex.CLIPath != "" || cfg.LLM.Codex.DefaultModel != "" || (cfg.LLM.Codex.WorkDir != "" && cfg.LLM.Codex.WorkDir != "~"),
				CLIPath:      cfg.LLM.Codex.CLIPath,
				DefaultModel: cfg.LLM.Codex.DefaultModel,
				WorkDir:      cfg.LLM.Codex.WorkDir,
				Sandbox:      buildManagedProviderConfigSandbox("codex_managed", cfg.LLM.Codex.WorkDir),
			},
		},
		Connectors: ConfigConnectorsResponse{
			Discord: ConfigDiscordConnectorResponse{
				Enabled:            cfg.Connectors.Discord.Enabled,
				Configured:         cfg.Connectors.Discord.BotToken != "",
				ConnectorID:        cfg.Connectors.Discord.ConnectorID,
				DisplayName:        cfg.Connectors.Discord.DisplayName,
				DeliveryMode:       discordDeliveryMode,
				RequireMention:     cfg.Connectors.Discord.RequireMention,
				RespondInDM:        cfg.Connectors.Discord.RespondInDM,
				AllowedGuildIDs:    cloneStringSlice(cfg.Connectors.Discord.AllowedGuildIDs),
				AllowedChannelIDs:  cloneStringSlice(cfg.Connectors.Discord.AllowedChannelIDs),
				BotTokenConfigured: cfg.Connectors.Discord.BotToken != "",
				BotTokenEnv:        cfg.Connectors.Discord.BotTokenEnv,
			},
		},
		MCP: ConfigMCPResponse{
			Servers:    listMCPServersForConfig(mcpManager),
			Catalog:    listMCPCatalogForConfig(mcpManager),
			Transports: listMCPTransportsForConfig(mcpManager),
		},
		Sandbox: ConfigSandboxResponse{
			Backends: listSandboxBackendsForConfig(sandboxManager),
		},
		RedactedFields: redactedFields,
	}
}

func listMCPServersForConfig(manager *mcp.Manager) []mcp.ServerResource {
	if manager == nil {
		return []mcp.ServerResource{}
	}
	return manager.ListServers()
}

func listMCPCatalogForConfig(manager *mcp.Manager) []mcp.CatalogEntry {
	if manager == nil {
		return []mcp.CatalogEntry{}
	}
	return manager.ListCatalog()
}

func listMCPTransportsForConfig(manager *mcp.Manager) []mcp.TransportCapability {
	if manager == nil {
		return []mcp.TransportCapability{}
	}
	return manager.ListTransportCapabilities()
}

func listSandboxBackendsForConfig(manager *sandbox.Manager) []sandbox.BackendCapabilityProfile {
	if manager == nil {
		return []sandbox.BackendCapabilityProfile{}
	}
	return manager.BackendCapabilities()
}

func buildSkillRegistryResponse(snapshot skills.Snapshot) SkillRegistryResponse {
	items := make([]SkillSummaryResponse, 0, len(snapshot.Skills))
	for _, skill := range snapshot.Skills {
		items = append(items, buildSkillSummaryResponse(skill))
	}
	overlays := make([]SkillOverlayResponse, 0, len(snapshot.Overlays))
	for _, overlay := range snapshot.Overlays {
		overlays = append(overlays, SkillOverlayResponse{
			OverlayID:  overlay.OverlayID,
			Source:     string(overlay.Source),
			Path:       overlay.Path,
			SizeBytes:  overlay.SizeBytes,
			ModifiedAt: overlay.ModifiedAt,
		})
	}
	return SkillRegistryResponse{
		LoadedAt: snapshot.LoadedAt,
		Items:    items,
		Overlays: overlays,
	}
}

func buildSkillSummaryResponse(skill skills.Skill) SkillSummaryResponse {
	files := make([]SkillFileResponse, 0, len(skill.Files))
	for _, file := range skill.Files {
		files = append(files, SkillFileResponse{
			Path:      file.Path,
			SizeBytes: file.SizeBytes,
		})
	}
	return SkillSummaryResponse{
		SkillID:            skill.SkillID,
		Name:               skill.Name,
		Description:        skill.Description,
		Source:             string(skill.Source),
		RootPath:           skill.RootPath,
		SkillPath:          skill.SkillPath,
		InstructionPath:    skill.InstructionPath,
		Files:              files,
		Frontmatter:        skill.Frontmatter,
		ExecutionManifest:  cloneExecutableManifest(skill.ExecutionManifest),
		AvailabilityStatus: string(skill.AvailabilityStatus),
		AvailabilityReason: skill.AvailabilityReason,
		Sandbox:            cloneSandboxConsumerView(skill.Sandbox),
	}
}

func buildSkillDetailResponse(skill skills.Skill) SkillDetailResponse {
	return SkillDetailResponse{
		SkillSummaryResponse: buildSkillSummaryResponse(skill),
		FrontmatterRaw:       skill.FrontmatterRaw,
		Body:                 skill.Body,
	}
}

func buildManagedProviderConfigSandbox(providerID, workDir string) map[string]any {
	readRoots := []string{}
	if strings.TrimSpace(workDir) != "" {
		readRoots = []string{strings.TrimSpace(workDir)}
	}
	view := &sandbox.ConsumerContractView{
		Declaration: &sandbox.ConsumerRequirementDeclaration{
			DeclarationID:               "managed_provider:" + strings.TrimSpace(providerID) + ":config",
			ConsumerKind:                sandbox.ConsumerKindManagedProvider,
			ConsumerID:                  strings.TrimSpace(providerID),
			OperationKind:               "config_inspect",
			ProfileID:                   sandbox.ProfileIDSubprocessDefault,
			ExecutionMode:               sandbox.ExecutionModeDeclarationOnly,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			ReadRoots:                   readRoots,
			WriteRoots:                  []string{},
			NetworkMode:                 sandbox.NetworkModeDeny,
			SecretRefs:                  []string{},
			ApprovalMode:                sandbox.ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
			Source:                      sandbox.SourceBuiltin,
		},
	}
	payload, err := json.Marshal(view)
	if err != nil {
		return nil
	}
	var item map[string]any
	if err := json.Unmarshal(payload, &item); err != nil {
		return nil
	}
	return item
}

func cloneSandboxConsumerView(view map[string]any) map[string]any {
	if view == nil {
		return nil
	}
	payload, err := json.Marshal(view)
	if err != nil {
		return view
	}
	var cloned map[string]any
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return view
	}
	return cloned
}

func cloneSandboxConsumerViews(items []map[string]any) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, cloneSandboxConsumerView(item))
	}
	return cloned
}

func cloneExecutableManifest(manifest *skills.ExecutableManifest) *skills.ExecutableManifest {
	if manifest == nil {
		return nil
	}
	cloned := *manifest
	cloned.Args = append([]string(nil), manifest.Args...)
	cloned.ReadRoots = append([]string(nil), manifest.ReadRoots...)
	cloned.WriteRoots = append([]string(nil), manifest.WriteRoots...)
	cloned.AllowedHosts = append([]string(nil), manifest.AllowedHosts...)
	cloned.AllowedPorts = append([]int(nil), manifest.AllowedPorts...)
	cloned.SecretRefs = append([]string(nil), manifest.SecretRefs...)
	return &cloned
}

func effectiveEnvironment(cfg config.Config) string {
	switch cfg.Environment {
	case config.EnvironmentProd, config.EnvironmentTest:
		return string(cfg.Environment)
	default:
		return string(config.EnvironmentTest)
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

func cloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string(nil), items...)
}
