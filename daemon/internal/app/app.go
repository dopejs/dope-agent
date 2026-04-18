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
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
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
	ConnectorSupervisor  *connectors.Supervisor
	CapabilitySupervisor *capabilities.Supervisor
	Server               *api.Server
	mu                   sync.Mutex
	closed               bool
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
	llmDispatcher, err := buildLLMDispatcher(cfg)
	if err != nil {
		return nil, err
	}
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()

	if err := recoverPersistedState(context.Background(), sqliteStore, sessionRouter, checkpointManager, eventBus, connectorSupervisor, capabilitySupervisor, policyEngine, authManager); err != nil {
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
		ConnectorSupervisor:  connectorSupervisor,
		CapabilitySupervisor: capabilitySupervisor,
		Server:               server,
	}, nil
}

func buildLLMDispatcher(cfg config.Config) (*llm.Dispatcher, error) {
	dispatcher := llm.NewDispatcher()
	dispatcher.SetDefaultTimeout(time.Duration(cfg.LLM.DefaultTimeoutMs) * time.Millisecond)
	dispatcher.SetDefaultRetries(cfg.LLM.DefaultMaxRetries)
	dispatcher.SetDefaultModel(cfg.LLM.DefaultModel)

	if openAIConfigured(cfg.LLM.OpenAICompatible) {
		provider, err := llm.NewOpenAICompatibleProvider(llm.OpenAICompatibleProviderConfig{
			BaseURL:      cfg.LLM.OpenAICompatible.BaseURL,
			APIKey:       cfg.LLM.OpenAICompatible.APIKey,
			DefaultModel: firstNonEmpty(cfg.LLM.OpenAICompatible.Model, cfg.LLM.DefaultModel),
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

func recoverPersistedState(ctx context.Context, sqliteStore *store.SQLiteStore, sessionRouter *router.SessionRouter, checkpointManager *checkpoints.Manager, eventBus *events.Bus, connectorSupervisor *connectors.Supervisor, capabilitySupervisor *capabilities.Supervisor, policyEngine *policy.Engine, authManager *auth.Manager) error {
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

	persistedEvents, err := sqliteStore.ListEvents(ctx, events.Filter{})
	if err != nil {
		return fmt.Errorf("load persisted events: %w", err)
	}

	for _, event := range persistedEvents {
		eventBus.Publish(event)
	}

	return nil
}
