package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	discordconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/discord"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/managedproviders"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

type App struct {
	Config               config.Config
	Logger               *telemetry.Logger
	Store                *store.SQLiteStore
	Checkpoints          *checkpoints.Manager
	EventBus             *events.Bus
	Router               *router.SessionRouter
	Runtime              *runtime.Manager
	Policy               *policy.Engine
	Auth                 *auth.Manager
	LLM                  *llm.Dispatcher
	Chat                 *chat.Service
	Skills               *skills.Registry
	Providers            *providers.Manager
	ConnectorSupervisor  *connectors.Supervisor
	CapabilitySupervisor *capabilities.Supervisor
	discordRuntime       managedConnectorRuntime
	Server               *api.Server
	mu                   sync.Mutex
	closed               bool
}

type managedConnectorRuntime interface {
	Start(context.Context) error
	Close(context.Context) error
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	logger := telemetry.New(cfg.LogLevel)
	eventBus := events.NewBus()
	sqliteStore, err := store.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	sessionRouter := router.NewSessionRouter()
	runtimeManager := runtime.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	managedRegistry := managedproviders.NewRegistry(cfg)
	llmDispatcher, err := buildLLMDispatcher(cfg, managedRegistry)
	if err != nil {
		return nil, err
	}
	skillRegistry, err := skills.NewRegistry(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	providerManager := providers.NewManager(cfg, llmDispatcher, managedRegistry)
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()
	chatService := chat.NewService(llmDispatcher, providerManager, skillRegistry, eventBus, sqliteStore)

	if err := recoverPersistedState(context.Background(), sqliteStore, sessionRouter, checkpointManager, eventBus, connectorSupervisor, capabilitySupervisor, policyEngine, authManager, providerManager); err != nil {
		return nil, err
	}
	if err := syncManagedProviderState(context.Background(), sqliteStore, providerManager); err != nil {
		return nil, err
	}

	discordRuntime, err := discordconnector.NewRuntime(discordconnector.Config{
		Enabled:           cfg.Connectors.Discord.Enabled,
		ConnectorID:       cfg.Connectors.Discord.ConnectorID,
		DisplayName:       cfg.Connectors.Discord.DisplayName,
		DeliveryMode:      cfg.Connectors.Discord.DeliveryMode,
		BotToken:          cfg.Connectors.Discord.BotToken,
		RequireMention:    cfg.Connectors.Discord.RequireMention,
		RespondInDM:       cfg.Connectors.Discord.RespondInDM,
		AllowedGuildIDs:   append([]string(nil), cfg.Connectors.Discord.AllowedGuildIDs...),
		AllowedChannelIDs: append([]string(nil), cfg.Connectors.Discord.AllowedChannelIDs...),
	}, logger.Slog(), connectorSupervisor, im.NewMessageLoop(sessionRouter, runtimeManager, checkpointManager, eventBus, sqliteStore, chatService), sqliteStore, eventBus, nil)
	if err != nil {
		return nil, err
	}

	server := api.NewServer(api.Dependencies{
		Config:       cfg,
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Policy:       policyEngine,
		Auth:         authManager,
		Router:       sessionRouter,
		Runtime:      runtimeManager,
		LLM:          llmDispatcher,
		Chat:         chatService,
		Skills:       skillRegistry,
		Providers:    providerManager,
		Connectors:   connectorSupervisor,
		Capabilities: capabilitySupervisor,
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
	})

	return &App{
		Config:               cfg,
		Logger:               logger,
		Store:                sqliteStore,
		Checkpoints:          checkpointManager,
		EventBus:             eventBus,
		Router:               sessionRouter,
		Runtime:              runtimeManager,
		Policy:               policyEngine,
		Auth:                 authManager,
		LLM:                  llmDispatcher,
		Chat:                 chatService,
		Skills:               skillRegistry,
		Providers:            providerManager,
		ConnectorSupervisor:  connectorSupervisor,
		CapabilitySupervisor: capabilitySupervisor,
		discordRuntime:       discordRuntime,
		Server:               server,
	}, nil
}

func buildLLMDispatcher(cfg config.Config, registry providers.ManagedRegistry) (*llm.Dispatcher, error) {
	dispatcher := llm.NewDispatcher()
	dispatcher.SetDefaultTimeout(time.Duration(cfg.LLM.DefaultTimeoutMs) * time.Millisecond)
	dispatcher.SetDefaultRetries(cfg.LLM.DefaultMaxRetries)
	dispatcher.SetDefaultModel(cfg.LLM.DefaultModel)
	registerManagedProviders(dispatcher, registry)

	if openAIConfigured(cfg.LLM.OpenAICompatible) {
		provider, err := llm.NewOpenAICompatibleProvider(llm.OpenAICompatibleProviderConfig{
			BaseURL:                   cfg.LLM.OpenAICompatible.BaseURL,
			APIKey:                    cfg.LLM.OpenAICompatible.APIKey,
			DefaultModel:              firstNonEmpty(cfg.LLM.OpenAICompatible.Model, cfg.LLM.DefaultModel),
			RequestTimeoutMs:          cfg.LLM.OpenAICompatible.TimeoutMs,
			StreamFirstChunkTimeoutMs: cfg.LLM.OpenAICompatible.StreamFirstChunkTimeoutMs,
			StreamIdleTimeoutMs:       cfg.LLM.OpenAICompatible.StreamIdleTimeoutMs,
			StreamMaxDurationMs:       cfg.LLM.OpenAICompatible.StreamMaxDurationMs,
		})
		if err != nil {
			return nil, fmt.Errorf("configure openai-compatible provider: %w", err)
		}
		dispatcher.RegisterProvider(provider)
		if cfg.LLM.DefaultProvider == "" {
			if err := dispatcher.SetDefaultProvider(llm.OpenAICompatibleProviderName); err != nil {
				return nil, fmt.Errorf("set default provider: %w", err)
			}
		}
		if cfg.LLM.DefaultModel == "" && cfg.LLM.OpenAICompatible.Model != "" {
			dispatcher.SetDefaultModel(cfg.LLM.OpenAICompatible.Model)
		}
		if cfg.LLM.DefaultTimeoutMs <= 0 && cfg.LLM.OpenAICompatible.TimeoutMs > 0 {
			dispatcher.SetDefaultTimeout(time.Duration(cfg.LLM.OpenAICompatible.TimeoutMs) * time.Millisecond)
		}
	}

	if cfg.LLM.DefaultProvider != "" {
		if err := dispatcher.SetDefaultProvider(cfg.LLM.DefaultProvider); err != nil {
			return nil, fmt.Errorf("set default provider: %w", err)
		}
	}

	return dispatcher, nil
}

func registerManagedProviders(dispatcher *llm.Dispatcher, registry providers.ManagedRegistry) {
	if dispatcher == nil || registry == nil {
		return
	}
	for _, bridge := range registry.List() {
		dispatcher.RegisterProvider(bridge.Provider())
	}
}

func openAIConfigured(cfg config.OpenAICompatibleProviderConfig) bool {
	return strings.TrimSpace(cfg.BaseURL) != "" ||
		strings.TrimSpace(cfg.APIKey) != "" ||
		strings.TrimSpace(cfg.APIKeyEnv) != "" ||
		strings.TrimSpace(cfg.Model) != ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (a *App) Run(ctx context.Context) error {
	a.Logger.Info("starting daemon", "bind_addr", a.Config.BindAddr)

	if a.discordRuntime != nil {
		starter, ok := a.discordRuntime.(interface{ Start(context.Context) error })
		if !ok {
			return fmt.Errorf("discord runtime is not startable")
		}
		if err := starter.Start(ctx); err != nil {
			return err
		}
	}

	if _, err := a.publishSystemEvent(context.Background(), "system.started", map[string]any{
		"service": "dope",
		"version": a.Config.Version,
	}); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	a.Server.Start(errCh)

	select {
	case <-ctx.Done():
		if _, err := a.publishSystemEvent(context.Background(), "system.stopped", map[string]any{
			"service": "dope",
			"reason":  "context_cancelled",
		}); err != nil {
			_ = a.Close(context.Background())
			return err
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.Server.Shutdown(shutdownCtx); err != nil {
			_ = a.Close(context.Background())
			return err
		}
		if err := a.Close(context.Background()); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		_ = a.Close(context.Background())
		return err
	}
}

func (a *App) Close(_ context.Context) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	a.mu.Unlock()

	var firstErr error

	if a.discordRuntime != nil {
		if err := a.discordRuntime.Close(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.Checkpoints != nil {
		if err := a.Checkpoints.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.EventBus != nil {
		a.EventBus.Close()
	}
	if a.Store != nil {
		if err := a.Store.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (a *App) publishSystemEvent(ctx context.Context, name string, payload map[string]any) (events.Event, error) {
	event := events.Event{
		Category: "system",
		Name:     name,
		Resource: events.Resource{
			Kind: "system",
			ID:   "dope",
		},
		Payload: payload,
	}
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	if a.Store != nil {
		persisted, err := a.Store.AppendEvent(ctx, event)
		if err != nil {
			return events.Event{}, fmt.Errorf("persist system event %s: %w", name, err)
		}
		event = persisted
	}
	if a.EventBus != nil {
		event = a.EventBus.Publish(event)
	}

	return event, nil
}

func newEventID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "evt_fallback"
	}

	return "evt_" + hex.EncodeToString(buf)
}

func recoverPersistedState(ctx context.Context, sqliteStore *store.SQLiteStore, sessionRouter *router.SessionRouter, checkpointManager *checkpoints.Manager, eventBus *events.Bus, connectorSupervisor *connectors.Supervisor, capabilitySupervisor *capabilities.Supervisor, policyEngine *policy.Engine, authManager *auth.Manager, providerManager *providers.Manager) error {
	if sqliteStore == nil {
		return nil
	}

	persistedSessions, err := sqliteStore.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("load persisted sessions: %w", err)
	}
	sessionRouter.RestoreSessions(persistedSessions)

	if _, err := checkpointManager.Restore(ctx); err != nil {
		return fmt.Errorf("restore runtime checkpoints: %w", err)
	}

	persistedConnectors, err := sqliteStore.ListConnectors(ctx)
	if err != nil {
		return fmt.Errorf("load persisted connectors: %w", err)
	}
	if connectorSupervisor != nil {
		connectorSupervisor.Restore(persistedConnectors)
	}

	persistedCapabilities, err := sqliteStore.ListCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("load persisted capabilities: %w", err)
	}
	if capabilitySupervisor != nil {
		capabilitySupervisor.Restore(persistedCapabilities)
	}

	persistedApprovals, err := sqliteStore.ListApprovals(ctx)
	if err != nil {
		return fmt.Errorf("load persisted approvals: %w", err)
	}
	persistedDecisions, err := sqliteStore.ListDecisions(ctx)
	if err != nil {
		return fmt.Errorf("load persisted decisions: %w", err)
	}
	if policyEngine != nil {
		policyEngine.Restore(persistedApprovals, persistedDecisions)
	}

	persistedPairings, err := sqliteStore.ListPairings(ctx)
	if err != nil {
		return fmt.Errorf("load persisted pairings: %w", err)
	}
	persistedTokens, err := sqliteStore.ListAccessTokens(ctx)
	if err != nil {
		return fmt.Errorf("load persisted access tokens: %w", err)
	}
	if authManager != nil {
		authManager.Restore(persistedPairings, persistedTokens)
	}

	persistedProviderAuthStates, err := sqliteStore.ListProviderAuthStates(ctx)
	if err != nil {
		return fmt.Errorf("load persisted provider auth states: %w", err)
	}
	persistedProviderModels, err := sqliteStore.ListProviderModels(ctx)
	if err != nil {
		return fmt.Errorf("load persisted provider models: %w", err)
	}
	persistedProviderPreferences, err := sqliteStore.ListProviderPreferences(ctx)
	if err != nil {
		return fmt.Errorf("load persisted provider preferences: %w", err)
	}
	if providerManager != nil {
		providerManager.RestoreManagedAuthStates(persistedProviderAuthStates)
		providerManager.RestoreProviderModels(persistedProviderModels)
		providerManager.RestoreProviderPreferences(persistedProviderPreferences)
	}

	persistedEvents, err := sqliteStore.ListEvents(ctx, events.Filter{})
	if err != nil {
		return fmt.Errorf("load persisted events: %w", err)
	}

	for _, event := range persistedEvents {
		eventBus.Publish(event)
	}

	return nil
}

func syncManagedProviderState(ctx context.Context, sqliteStore *store.SQLiteStore, providerManager *providers.Manager) error {
	if sqliteStore == nil || providerManager == nil {
		return nil
	}

	results, err := providerManager.SyncManagedProviders(ctx)
	if err != nil {
		return fmt.Errorf("sync managed provider state: %w", err)
	}
	for _, result := range results {
		if err := sqliteStore.UpsertProviderAuthState(ctx, result.State); err != nil {
			return fmt.Errorf("persist provider auth state %s: %w", result.State.ProviderID, err)
		}
		if err := sqliteStore.ReplaceProviderModels(ctx, result.State.ProviderID, result.Models); err != nil {
			return fmt.Errorf("persist provider models %s: %w", result.State.ProviderID, err)
		}
	}
	return nil
}
