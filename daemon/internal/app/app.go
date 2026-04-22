package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/artifacts"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	discordconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/discord"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/managedproviders"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
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
	Sandboxes            *sandbox.Manager
	MCP                  *mcp.Manager
	Providers            *providers.Manager
	Integrations         *integrations.Manager
	Scheduler            *scheduler.Scheduler
	Delivery             *delivery.Manager
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
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxManager, policyEngine, nil)
	integrationManager := integrations.NewManager(string(cfg.Environment))
	managedRegistry := managedproviders.NewRegistry(cfg, sandboxManager)
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
	artifactService := artifacts.NewService(cfg.DataDir)
	computerUseManager := computeruse.NewManager(computeruse.Dependencies{
		EnvironmentScope: string(cfg.Environment),
		Runtime:          runtimeManager,
		Policy:           policyEngine,
		Store:            sqliteStore,
		Artifacts:        artifactService,
	})
	connectorAdapter := delivery.NewConnectorAdapter(sqliteStore)
	var discordTransport discordconnector.Transport
	if cfg.Connectors.Discord.Enabled {
		discordTransport, err = discordconnector.NewGatewayTransport(discordconnector.Config{
			Enabled:           cfg.Connectors.Discord.Enabled,
			ConnectorID:       cfg.Connectors.Discord.ConnectorID,
			DisplayName:       cfg.Connectors.Discord.DisplayName,
			DeliveryMode:      cfg.Connectors.Discord.DeliveryMode,
			BotToken:          cfg.Connectors.Discord.BotToken,
			RequireMention:    cfg.Connectors.Discord.RequireMention,
			RespondInDM:       cfg.Connectors.Discord.RespondInDM,
			AllowedGuildIDs:   append([]string(nil), cfg.Connectors.Discord.AllowedGuildIDs...),
			AllowedChannelIDs: append([]string(nil), cfg.Connectors.Discord.AllowedChannelIDs...),
		})
		if err != nil {
			return nil, err
		}
		connectorAdapter.Register(cfg.Connectors.Discord.ConnectorID, discordTransport)
	}
	deliveryManager := delivery.NewManager(string(cfg.Environment), eventBus, sqliteStore, delivery.NewTestSinkAdapter(), connectorAdapter)
	workflowLauncher := api.NewScheduleWorkflowLauncher(api.ScheduleWorkflowLauncherDependencies{
		Config:       cfg,
		Runtime:      runtimeManager,
		Policy:       policyEngine,
		Capabilities: capabilitySupervisor,
		Skills:       skillRegistry,
		MCP:          mcpManager,
		Sandboxes:    sandboxManager,
		ComputerUse:  computerUseManager,
		Delivery:     deliveryManager,
		EventBus:     eventBus,
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
	})
	scheduleManager := scheduler.New(scheduler.Dependencies{
		Config:           cfg,
		Runtime:          runtimeManager,
		EventBus:         eventBus,
		Store:            sqliteStore,
		Checkpoints:      checkpointManager,
		WorkflowLauncher: workflowLauncher,
	})
	envCtx := events.WithEnvironmentScope(context.Background(), string(cfg.Environment))

	if err := recoverPersistedState(envCtx, cfg.Environment, sqliteStore, sessionRouter, checkpointManager, eventBus, connectorSupervisor, capabilitySupervisor, policyEngine, authManager, providerManager, sandboxManager, mcpManager, integrationManager); err != nil {
		return nil, err
	}
	if err := syncManagedProviderState(envCtx, sqliteStore, providerManager); err != nil {
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
	}, logger.Slog(), connectorSupervisor, im.NewMessageLoop(sessionRouter, runtimeManager, checkpointManager, eventBus, sqliteStore, chatService), sqliteStore, eventBus, discordTransport)
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
		Sandboxes:    sandboxManager,
		MCP:          mcpManager,
		Integrations: integrationManager,
		Providers:    providerManager,
		Connectors:   connectorSupervisor,
		Capabilities: capabilitySupervisor,
		ComputerUse:  computerUseManager,
		Scheduler:    scheduleManager,
		Delivery:     deliveryManager,
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
		Sandboxes:            sandboxManager,
		MCP:                  mcpManager,
		Integrations:         integrationManager,
		Providers:            providerManager,
		Scheduler:            scheduleManager,
		Delivery:             deliveryManager,
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
	ctx = events.WithEnvironmentScope(ctx, string(a.Config.Environment))
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
	if a.Scheduler != nil {
		if err := a.Scheduler.Start(ctx); err != nil {
			return err
		}
	}
	if a.Delivery != nil {
		if err := a.Delivery.Restore(ctx); err != nil {
			return err
		}
	}

	if _, err := a.publishSystemEvent(ctx, "system.started", map[string]any{
		"service": "dope",
		"version": a.Config.Version,
	}); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	a.Server.Start(errCh)

	select {
	case <-ctx.Done():
		stopCtx := events.WithEnvironmentScope(context.Background(), string(a.Config.Environment))
		if _, err := a.publishSystemEvent(stopCtx, "system.stopped", map[string]any{
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
	if a.Sandboxes != nil {
		if err := a.Sandboxes.Close(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.Scheduler != nil {
		if err := a.Scheduler.Close(); err != nil && firstErr == nil {
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
		EnvironmentScope: events.EnvironmentScopeFromContext(ctx),
		Category:         "system",
		Name:             name,
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

func recoverPersistedState(ctx context.Context, environment config.Environment, sqliteStore *store.SQLiteStore, sessionRouter *router.SessionRouter, checkpointManager *checkpoints.Manager, eventBus *events.Bus, connectorSupervisor *connectors.Supervisor, capabilitySupervisor *capabilities.Supervisor, policyEngine *policy.Engine, authManager *auth.Manager, providerManager *providers.Manager, sandboxManager *sandbox.Manager, mcpManager *mcp.Manager, integrationManagers ...*integrations.Manager) error {
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
	if sandboxManager != nil {
		if err := sandboxManager.Restore(ctx); err != nil {
			return fmt.Errorf("restore sandbox executions: %w", err)
		}
		if err := reconcileRecoveredSandboxToolCalls(ctx, sqliteStore, checkpointManager, sandboxManager); err != nil {
			return fmt.Errorf("reconcile recovered sandbox tool calls: %w", err)
		}
	}
	if mcpManager != nil {
		if err := mcpManager.Restore(ctx); err != nil {
			return fmt.Errorf("restore mcp state: %w", err)
		}
	}
	var integrationManager *integrations.Manager
	if len(integrationManagers) > 0 {
		integrationManager = integrationManagers[0]
	}
	if integrationManager != nil {
		persistedIntegrations, err := sqliteStore.ListIntegrations(ctx, string(environment))
		if err != nil {
			return fmt.Errorf("load persisted integrations: %w", err)
		}
		integrationManager.Restore(persistedIntegrations)
	}
	if _, err := sqliteStore.MarkInFlightWorkflowsInterrupted(ctx, string(environment), time.Now().UTC()); err != nil {
		return fmt.Errorf("interrupt in-flight workflows: %w", err)
	}
	if err := interruptRecoveredComputerUse(ctx, string(environment), sqliteStore, checkpointManager); err != nil {
		return fmt.Errorf("interrupt in-flight computer-use state: %w", err)
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

func reconcileRecoveredSandboxToolCalls(ctx context.Context, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager, sandboxManager *sandbox.Manager) error {
	if checkpointManager == nil || sandboxManager == nil {
		return nil
	}
	runtimeManager := checkpointManager.Runtime()
	if runtimeManager == nil {
		return nil
	}
	changedRuns := map[string]struct{}{}
	for _, run := range runtimeManager.ListRuns() {
		steps, err := runtimeManager.ListSteps(run.RunID)
		if err != nil {
			return err
		}
		for _, step := range steps {
			toolCalls, err := runtimeManager.ListToolCalls(run.RunID, step.StepID)
			if err != nil {
				return err
			}
			for _, toolCall := range toolCalls {
				if strings.TrimSpace(toolCall.SandboxExecutionID) == "" {
					continue
				}
				switch toolCall.Status {
				case runtime.ToolCallStatusCompleted, runtime.ToolCallStatusFailed, runtime.ToolCallStatusCancelled, runtime.ToolCallStatusDenied:
					continue
				}
				execution, ok := sandboxManager.GetExecution(toolCall.SandboxExecutionID)
				if !ok {
					continue
				}
				var updated runtime.ToolCall
				switch execution.Status {
				case sandbox.ExecutionStatusCompleted:
					updated, err = runtimeManager.CompleteToolCall(run.RunID, step.StepID, toolCall.ToolCallID, runtime.CompleteToolCallInput{Output: recoveredSandboxToolCallOutput(execution)})
				case sandbox.ExecutionStatusFailed:
					updated, err = runtimeManager.FailToolCall(run.RunID, step.StepID, toolCall.ToolCallID, runtime.FailToolCallInput{
						Output:       recoveredSandboxToolCallOutput(execution),
						Error:        execution.Result.Error,
						FailureClass: string(execution.Result.ErrorClass),
					})
				case sandbox.ExecutionStatusCancelled:
					updated, err = runtimeManager.CancelToolCall(run.RunID, step.StepID, toolCall.ToolCallID, runtime.CancelToolCallInput{
						Output:       recoveredSandboxToolCallOutput(execution),
						Error:        execution.Result.Error,
						FailureClass: string(execution.Result.ErrorClass),
					})
				case sandbox.ExecutionStatusDenied:
					updated, err = runtimeManager.DenyToolCall(run.RunID, step.StepID, toolCall.ToolCallID, runtime.DenyToolCallInput{
						Output:       recoveredSandboxToolCallOutput(execution),
						Error:        execution.Result.Error,
						FailureClass: string(execution.Result.ErrorClass),
					})
				case sandbox.ExecutionStatusUnsupported:
					updated, err = runtimeManager.FailToolCall(run.RunID, step.StepID, toolCall.ToolCallID, runtime.FailToolCallInput{
						Output:       recoveredSandboxToolCallOutput(execution),
						Error:        execution.Result.Error,
						FailureClass: string(execution.Result.ErrorClass),
					})
				default:
					continue
				}
				if err != nil {
					return err
				}
				updated.SandboxExecutionID = execution.ExecutionID
				updated.Sandbox = recoveredConsumerViewMap(execution.Consumer)
				if sqliteStore != nil {
					if err := sqliteStore.UpsertToolCall(ctx, updated); err != nil {
						return err
					}
				}
				changedRuns[run.RunID] = struct{}{}
			}
		}
	}
	for runID := range changedRuns {
		if err := checkpointManager.SaveRunCheckpoint(ctx, runID); err != nil {
			return err
		}
	}
	return nil
}

func interruptRecoveredComputerUse(ctx context.Context, environmentScope string, sqliteStore *store.SQLiteStore, checkpointManager *checkpoints.Manager) error {
	if sqliteStore == nil || checkpointManager == nil {
		return nil
	}
	runtimeManager := checkpointManager.Runtime()
	if runtimeManager == nil {
		return nil
	}
	now := time.Now().UTC()
	_, actions, err := sqliteStore.MarkInFlightComputerUseInterrupted(ctx, environmentScope, now)
	if err != nil {
		return err
	}
	changedRuns := map[string]struct{}{}
	for _, action := range actions {
		if strings.TrimSpace(action.ToolCallID) != "" && strings.TrimSpace(action.StepID) != "" {
			updatedToolCall, err := runtimeManager.FailToolCall(action.RunID, action.StepID, action.ToolCallID, runtime.FailToolCallInput{
				Output: map[string]any{
					"computerUseSessionId": action.ComputerUseSessionID,
					"computerUseActionId":  action.ComputerUseActionID,
				},
				Error:        action.FailureReason,
				FailureClass: action.FailureClass,
			})
			if err == nil {
				if err := sqliteStore.UpsertToolCall(ctx, updatedToolCall); err != nil {
					return err
				}
				changedRuns[action.RunID] = struct{}{}
			}
		}
		if strings.TrimSpace(action.StepID) != "" {
			updatedStep, runUpdate, err := runtimeManager.UpdateStepStatusAndReconcileRun(action.RunID, action.StepID, runtime.UpdateStepStatusInput{
				Status: runtime.StepStatusFailed,
				Output: map[string]any{
					"computerUseActionId": action.ComputerUseActionID,
					"failureClass":        action.FailureClass,
				},
			})
			if err == nil {
				if err := sqliteStore.UpsertStep(ctx, updatedStep); err != nil {
					return err
				}
				if runUpdate != nil {
					if err := sqliteStore.UpsertRun(ctx, *runUpdate); err != nil {
						return err
					}
				}
				changedRuns[action.RunID] = struct{}{}
			}
		}
	}
	for runID := range changedRuns {
		if err := checkpointManager.SaveRunCheckpoint(ctx, runID); err != nil {
			return err
		}
	}
	return nil
}

func recoveredSandboxToolCallOutput(execution sandbox.Execution) map[string]any {
	output := map[string]any{
		"executionId": execution.ExecutionID,
		"status":      execution.Status,
		"stdout":      execution.Result.Stdout,
		"stderr":      execution.Result.Stderr,
	}
	if execution.Result.ExitCode != nil {
		output["exitCode"] = *execution.Result.ExitCode
	}
	if execution.Result.ErrorCode != "" {
		output["errorCode"] = execution.Result.ErrorCode
	}
	if execution.Result.Error != "" {
		output["error"] = execution.Result.Error
	}
	if execution.Consumer != nil {
		output["consumer"] = recoveredConsumerViewMap(execution.Consumer)
	}
	return output
}

func recoveredConsumerViewMap(view *sandbox.ConsumerContractView) map[string]any {
	if view == nil {
		return nil
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
