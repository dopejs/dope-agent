package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/api"
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
	llmDispatcher := llm.NewDispatcher()
	connectorSupervisor := connectors.NewSupervisor()
	capabilitySupervisor := capabilities.NewSupervisor()

	if err := recoverPersistedState(context.Background(), sqliteStore, sessionRouter, checkpointManager, eventBus, connectorSupervisor, capabilitySupervisor); err != nil {
		return nil, err
	}

	server := api.NewServer(api.Dependencies{
		Config:       cfg,
		Logger:       logger.Slog(),
		EventBus:     eventBus,
		Policy:       policyEngine,
		Router:       sessionRouter,
		Runtime:      runtimeManager,
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
		LLM:                  llmDispatcher,
		ConnectorSupervisor:  connectorSupervisor,
		CapabilitySupervisor: capabilitySupervisor,
		Server:               server,
	}, nil
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

func recoverPersistedState(ctx context.Context, sqliteStore *store.SQLiteStore, sessionRouter *router.SessionRouter, checkpointManager *checkpoints.Manager, eventBus *events.Bus, connectorSupervisor *connectors.Supervisor, capabilitySupervisor *capabilities.Supervisor) error {
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

	persistedEvents, err := sqliteStore.ListEvents(ctx, events.Filter{})
	if err != nil {
		return fmt.Errorf("load persisted events: %w", err)
	}

	for _, event := range persistedEvents {
		eventBus.Publish(event)
	}

	return nil
}
