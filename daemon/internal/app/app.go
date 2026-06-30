package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/api"
	"github.com/dopejs/dope-agent/daemon/internal/artifacts"
	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	discordconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/discord"
	matrixconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/matrix"
	slackconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/slack"
	telegramconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/telegram"
	"github.com/dopejs/dope-agent/daemon/internal/delivery"
	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/managedproviders"
	"github.com/dopejs/dope-agent/daemon/internal/mcp"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/reminders"
	"github.com/dopejs/dope-agent/daemon/internal/routine"
	"github.com/dopejs/dope-agent/daemon/internal/triage"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/scheduler"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/skills"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
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
	Identity             *identity.Manager
	LLM                  *llm.Dispatcher
	Chat                 *chat.Service
	Skills               *skills.Registry
	Sandboxes            *sandbox.Manager
	Secrets              *secrets.Manager
	MCP                  *mcp.Manager
	Providers            *providers.Manager
	Integrations         *integrations.Manager
	Calendar             *calendar.Manager
	Mail                 *mail.Manager
	Reminders            *reminders.Manager
	Triage               *triage.Manager
	Routines             *routine.Manager
	Scheduler            *scheduler.Scheduler
	Delivery             *delivery.Manager
	Billing              *billing.Manager
	Activation           *activation.Service
	Evaluation           *evaluation.Manager
	LiveValidation       *livevalidation.Manager
	ConnectorSupervisor  *connectors.Supervisor
	CapabilitySupervisor *capabilities.Supervisor
	discordRuntime       managedConnectorRuntime
	telegramRuntime      managedConnectorRuntime
	slackRuntime         managedConnectorRuntime
	matrixRuntime        managedConnectorRuntime
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
	// Roadmap 35 (Pass B): seed the default-personal-tenant cache early so
	// any subsystem that issues legacy Upsert* calls before BootstrapLocal
	// runs (e.g. evaluation.Manager.LoadFixtures during NewManager wiring)
	// still binds tenant_id correctly on a daemon restart where the
	// personal tenant already exists. On the very first boot the personal
	// tenant does not exist yet — SeedDefaultTenantCache silently no-ops
	// and the cache is populated again after BootstrapLocal below. Both
	// orderings are required: NULL inserts pre-bootstrap are recovered by
	// the per-domain backfill, while post-bootstrap inserts MUST bind to
	// avoid the NOT NULL CHECK that step (c) enforcement installs.
	if err := sqliteStore.SeedDefaultTenantCache(context.Background()); err != nil {
		_ = sqliteStore.Close()
		return nil, fmt.Errorf("seed default tenant cache (early): %w", err)
	}
	if err := sqliteStore.EnsureBillingCatalog(context.Background()); err != nil {
		_ = sqliteStore.Close()
		return nil, fmt.Errorf("ensure billing catalog: %w", err)
	}
	// Roadmap 35 (T017): refuse to start if any tenant-migration step is in
	// the `failed` state. Resume of `running` steps is owned by the
	// individual backfill drivers (US2). At Phase 2 only the events
	// progress rows are registered; per-domain steps land in US1/US2.
	migrationGate, err := guardTenantMigrationStartup(context.Background(), sqliteStore, logger, eventBus)
	if err != nil {
		_ = sqliteStore.Close()
		return nil, err
	}
	_ = migrationGate // wired into api.Dependencies below so handlers can refuse tenant traffic during in-flight migration
	sessionRouter := router.NewSessionRouter()
	runtimeManager := runtime.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	policyEngine := policy.NewEngine()
	authManager := auth.NewManager()
	identityManager := identity.NewManager(sqliteStore)
	billingManager := billing.NewManager(sqliteStore)
	sandboxManager := sandbox.NewManager(cfg, sqliteStore, eventBus, policyEngine)
	secretBackend, err := secrets.NewLocalBackend(filepath.Join(cfg.DataDir, "tenant-secret-values"))
	if err != nil {
		_ = sqliteStore.Close()
		return nil, fmt.Errorf("create tenant secret backend: %w", err)
	}
	secretManager := secrets.NewManager(sqliteStore, secretBackend)
	sandboxManager.SetSecretManager(secretManager)
	mcpManager := mcp.NewManager(cfg, sqliteStore, eventBus, sandboxManager, policyEngine, nil)
	mcpManager.SetSecretManager(secretManager)
	integrationManager := integrations.NewManager(string(cfg.Environment))
	calendarManager := calendar.NewManager(string(cfg.Environment))
	mailManager := mail.NewManager(string(cfg.Environment))
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
	// Roadmap 59: optionally wire out-of-process integration adapters. Off by default — the
	// in-daemon fake backend remains the default backend in every environment. Enabled only
	// when DOPE_INTEGRATION_ADAPTER names an adapter binary.
	if adapterBin := strings.TrimSpace(os.Getenv("DOPE_INTEGRATION_ADAPTER")); adapterBin != "" {
		if err := wireIntegrationAdapters(context.Background(), adapterBin, capabilitySupervisor, calendarManager, mailManager, integrationManager, secretManager); err != nil {
			return nil, fmt.Errorf("integration adapter wiring: %w", err)
		}
	}
	chatService := chat.NewService(llmDispatcher, providerManager, skillRegistry, eventBus, sqliteStore)
	activationService := activation.NewService(activation.Dependencies{
		StateStore:       sqliteStore,
		Identity:         sqliteStore,
		Billing:          billingManager,
		Chat:             activationChatRunner{service: chatService},
		Audit:            sqliteStore,
		EnvironmentScope: string(cfg.Environment),
		Hosted:           cfg.Environment == config.EnvironmentProd,
	})
	artifactService := artifacts.NewService(cfg.DataDir)
	artifactService.ConfigureBilling(billingManager, cfg.Environment == config.EnvironmentProd)
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
		connectorAdapter.RegisterConnector(cfg.Connectors.Discord.ConnectorID, "discord", discordTransport)
	}
	var telegramTransport telegramconnector.Transport
	if cfg.Connectors.Telegram.Enabled {
		if strings.TrimSpace(cfg.Connectors.Telegram.BotToken) == "" {
			return nil, fmt.Errorf("telegram connector enabled but bot token is not configured")
		}
		telegramTransport, err = telegramconnector.NewBotAPITransport(telegramconnector.BotAPITransportConfig{
			ConnectorID: cfg.Connectors.Telegram.ConnectorID,
			BotToken:    cfg.Connectors.Telegram.BotToken,
			BotUsername: cfg.Connectors.Telegram.BotUsername,
			BaseURL:     cfg.Connectors.Telegram.BotAPIBaseURL,
		})
		if err != nil {
			return nil, err
		}
		connectorAdapter.RegisterConnector(cfg.Connectors.Telegram.ConnectorID, "telegram", telegramTransport)
	}
	var slackTransport slackconnector.Transport
	if cfg.Connectors.Slack.Enabled {
		slackTransport = slackconnector.NewWebAPITransport(slackconnector.WebAPITransportConfig{
			ConnectorID: cfg.Connectors.Slack.ConnectorID,
			BaseURL:     cfg.Connectors.Slack.APIBaseURL,
			TokenProvider: slackBotTokenProvider{
				secrets:   secretManager,
				secretRef: slackBotTokenSecretRef(cfg.Connectors.Slack),
			},
		})
		connectorAdapter.RegisterConnector(cfg.Connectors.Slack.ConnectorID, "slack", slackTransport)
	}
	var matrixTransport matrixconnector.Transport
	if cfg.Connectors.Matrix.Enabled {
		matrixTransport, err = matrixconnector.NewClientTransport(matrixconnector.ClientTransportConfig{
			ConnectorID:          cfg.Connectors.Matrix.ConnectorID,
			HomeserverURL:        cfg.Connectors.Matrix.HomeserverURL,
			BotAccessToken:       cfg.Connectors.Matrix.BotAccessToken,
			AccessTokenSource:    matrixBotAccessTokenProvider{secrets: secretManager},
			SelectedRoomIDs:      append([]string(nil), cfg.Connectors.Matrix.SelectedRoomIDs...),
			AllowedDirectUserIDs: append([]string(nil), cfg.Connectors.Matrix.AllowedDirectUserIDs...),
		})
		if err != nil {
			return nil, err
		}
		connectorAdapter.RegisterConnector(cfg.Connectors.Matrix.ConnectorID, "matrix", matrixTransport)
	}
	deliveryManager := delivery.NewManager(string(cfg.Environment), eventBus, sqliteStore, delivery.NewTestSinkAdapter(), connectorAdapter)
	envCtx := events.WithEnvironmentScope(context.Background(), string(cfg.Environment))
	replayRecorder := evaluation.NewRuntimeReplayRecorder(runtimeManager, sqliteStore)
	replayRecorder.ConfigureBilling(billingManager, cfg.Environment == config.EnvironmentProd)
	evaluationManager := evaluation.NewManager(evaluation.Dependencies{
		EnvironmentScope: string(cfg.Environment),
		Store:            sqliteStore,
		FixturesDir:      defaultEvaluationFixturesDir(),
		RuntimeRecorder:  replayRecorder,
		Billing:          billingManager,
		HostedBilling:    cfg.Environment == config.EnvironmentProd,
	})
	liveValidationManager := livevalidation.NewManager(livevalidation.Dependencies{
		EnvironmentScope: string(cfg.Environment),
		Store:            sqliteStore,
		Enabled:          true,
		Billing:          billingManager,
		HostedBilling:    cfg.Environment == config.EnvironmentProd,
		CandidateToolClassResolver: func(ctx context.Context, candidateID string) ([]livevalidation.ToolClass, error) {
			candidate, ok, err := sqliteStore.GetReplayCandidate(ctx, string(cfg.Environment), candidateID)
			if err != nil || !ok {
				return nil, err
			}
			return liveValidationToolClasses(candidate.ToolClasses), nil
		},
		LedgerEventSink: func(ctx context.Context, eventName string, entry livevalidation.SideEffectLedgerEntry) {
			event := events.LiveValidationLedgerEvent(eventName, entry)
			event.EnvironmentScope = string(cfg.Environment)
			published := eventBus.Publish(event)
			if published.TenantID != "" {
				_, _ = sqliteStore.AppendEventForTenantRaw(ctx, published, published.TenantID)
			}
		},
	})
	if err := evaluationManager.LoadFixtures(envCtx); err != nil {
		return nil, err
	}
	workflowLauncher := api.NewScheduleWorkflowLauncher(api.ScheduleWorkflowLauncherDependencies{
		Config:       cfg,
		Runtime:      runtimeManager,
		Policy:       policyEngine,
		Capabilities: capabilitySupervisor,
		Skills:       skillRegistry,
		MCP:          mcpManager,
		Sandboxes:    sandboxManager,
		Integrations: integrationManager,
		Calendar:     calendarManager,
		Mail:         mailManager,
		ComputerUse:  computerUseManager,
		Delivery:     deliveryManager,
		Billing:      billingManager,
		EventBus:     eventBus,
		Store:        sqliteStore,
		Checkpoints:  checkpointManager,
	})
	reminderManager := reminders.NewManager(reminders.Dependencies{
		EnvironmentScope: string(cfg.Environment),
		Store:            sqliteStore,
		EventBus:         eventBus,
		Delivery:         deliveryManager,
		WorkflowLauncher: workflowLauncher,
	})
	triageManager := triage.NewManager(string(cfg.Environment))
	scheduleManager := scheduler.New(scheduler.Dependencies{
		Config:           cfg,
		Runtime:          runtimeManager,
		EventBus:         eventBus,
		Store:            sqliteStore,
		Checkpoints:      checkpointManager,
		WorkflowLauncher: workflowLauncher,
		Billing:          billingManager,
	})
	routineManager := routine.NewManager(string(cfg.Environment), scheduleManager)
	if err := recoverPersistedStateWithSecrets(envCtx, cfg.DataDir, cfg.Environment, sqliteStore, sessionRouter, checkpointManager, eventBus, connectorSupervisor, capabilitySupervisor, policyEngine, authManager, identityManager, providerManager, sandboxManager, secretManager, mcpManager, integrationManager, calendarManager, mailManager, reminderManager); err != nil {
		return nil, err
	}
	if err := syncManagedProviderState(envCtx, sqliteStore, providerManager); err != nil {
		return nil, err
	}
	if _, err := billingManager.RecoverPendingReservations(envCtx, nil); err != nil {
		return nil, fmt.Errorf("recover billing reservations: %w", err)
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
	telegramRuntime, err := telegramconnector.NewRuntime(telegramconnector.Config{
		Enabled:     cfg.Connectors.Telegram.Enabled,
		ConnectorID: cfg.Connectors.Telegram.ConnectorID,
		DisplayName: cfg.Connectors.Telegram.DisplayName,
		BotToken:    cfg.Connectors.Telegram.BotToken,
		BotUsername: cfg.Connectors.Telegram.BotUsername,
		Allowments:  telegramAllowmentsFromConfig(cfg.Connectors.Telegram),
	}, logger.Slog(), connectorSupervisor, im.NewMessageLoop(sessionRouter, runtimeManager, checkpointManager, eventBus, sqliteStore, chatService), sqliteStore, eventBus, telegramTransport)
	if err != nil {
		return nil, err
	}
	slackRuntime, err := slackconnector.NewRuntime(slackconnector.Config{
		Enabled:             cfg.Connectors.Slack.Enabled,
		ConnectorID:         cfg.Connectors.Slack.ConnectorID,
		DisplayName:         cfg.Connectors.Slack.DisplayName,
		WorkspaceBindingID:  cfg.Connectors.Slack.WorkspaceBindingID,
		WorkspaceID:         cfg.Connectors.Slack.WorkspaceID,
		BotUserID:           cfg.Connectors.Slack.BotUserID,
		AllowedChannelIDs:   append([]string(nil), cfg.Connectors.Slack.AllowedChannelIDs...),
		AllowedDMUserIDs:    append([]string(nil), cfg.Connectors.Slack.AllowedDMUserIDs...),
		AllowedDMUserGroups: append([]string(nil), cfg.Connectors.Slack.AllowedDMUserGroups...),
	}, logger.Slog(), connectorSupervisor, im.NewMessageLoop(sessionRouter, runtimeManager, checkpointManager, eventBus, sqliteStore, chatService), sqliteStore, eventBus, slackTransport)
	if err != nil {
		return nil, err
	}
	matrixRuntime, err := matrixconnector.NewRuntime(matrixconnector.Config{
		Enabled:              cfg.Connectors.Matrix.Enabled,
		ConnectorID:          cfg.Connectors.Matrix.ConnectorID,
		DisplayName:          cfg.Connectors.Matrix.DisplayName,
		HomeserverURL:        cfg.Connectors.Matrix.HomeserverURL,
		HomeserverID:         cfg.Connectors.Matrix.HomeserverID,
		BotUserID:            cfg.Connectors.Matrix.BotUserID,
		SelectedRoomIDs:      append([]string(nil), cfg.Connectors.Matrix.SelectedRoomIDs...),
		AllowedDirectUserIDs: append([]string(nil), cfg.Connectors.Matrix.AllowedDirectUserIDs...),
		ConfiguredCommands:   append([]string(nil), cfg.Connectors.Matrix.ConfiguredCommands...),
	}, logger.Slog(), connectorSupervisor, im.NewMessageLoop(sessionRouter, runtimeManager, checkpointManager, eventBus, sqliteStore, chatService), sqliteStore, eventBus, matrixTransport)
	if err != nil {
		return nil, err
	}

	server := api.NewServer(api.Dependencies{
		Config:                cfg,
		Logger:                logger.Slog(),
		EventBus:              eventBus,
		Policy:                policyEngine,
		Auth:                  authManager,
		Identity:              identityManager,
		Router:                sessionRouter,
		Runtime:               runtimeManager,
		LLM:                   llmDispatcher,
		Chat:                  chatService,
		Skills:                skillRegistry,
		Sandboxes:             sandboxManager,
		Secrets:               secretManager,
		MCP:                   mcpManager,
		Integrations:          integrationManager,
		Calendar:              calendarManager,
		Mail:                  mailManager,
		Reminders:             reminderManager,
		Triage:                triageManager,
		Routines:              routineManager,
		Providers:             providerManager,
		Connectors:            connectorSupervisor,
		Capabilities:          capabilitySupervisor,
		ComputerUse:           computerUseManager,
		Scheduler:             scheduleManager,
		Delivery:              deliveryManager,
		Billing:               billingManager,
		Activation:            activationService,
		Store:                 sqliteStore,
		Checkpoints:           checkpointManager,
		Evaluation:            evaluationManager,
		LiveValidation:        liveValidationManager,
		AuditEmitter:          audit.NewEmitter(eventBus, logger.Slog()),
		TenantMigrationStatus: migrationGate,
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
		Identity:             identityManager,
		LLM:                  llmDispatcher,
		Chat:                 chatService,
		Skills:               skillRegistry,
		Sandboxes:            sandboxManager,
		Secrets:              secretManager,
		MCP:                  mcpManager,
		Integrations:         integrationManager,
		Calendar:             calendarManager,
		Mail:                 mailManager,
		Reminders:            reminderManager,
		Triage:               triageManager,
		Routines:             routineManager,
		Providers:            providerManager,
		Scheduler:            scheduleManager,
		Delivery:             deliveryManager,
		Billing:              billingManager,
		Activation:           activationService,
		Evaluation:           evaluationManager,
		LiveValidation:       liveValidationManager,
		ConnectorSupervisor:  connectorSupervisor,
		CapabilitySupervisor: capabilitySupervisor,
		discordRuntime:       discordRuntime,
		telegramRuntime:      telegramRuntime,
		slackRuntime:         slackRuntime,
		matrixRuntime:        matrixRuntime,
		Server:               server,
	}, nil
}

func telegramAllowmentsFromConfig(cfg config.TelegramConnectorConfig) []telegramconnector.AllowmentValidation {
	items := make([]telegramconnector.AllowmentValidation, 0, len(cfg.AllowedUserIDs)+len(cfg.AllowedDirectChatIDs)+len(cfg.AllowedGroupIDs))
	for _, id := range cfg.AllowedUserIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		items = append(items, telegramconnector.AllowmentValidation{
			ScopeType:       telegramconnector.ScopeUser,
			ScopeID:         strings.TrimSpace(id),
			Enabled:         true,
			GroupGate:       telegramconnector.GroupGateNotApplicable,
			ValidationState: telegramconnector.AllowmentValid,
		})
	}
	for _, id := range cfg.AllowedDirectChatIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		items = append(items, telegramconnector.AllowmentValidation{
			ScopeType:       telegramconnector.ScopeDirectChat,
			ScopeID:         strings.TrimSpace(id),
			Enabled:         true,
			GroupGate:       telegramconnector.GroupGateNotApplicable,
			ValidationState: telegramconnector.AllowmentValid,
		})
	}
	for _, id := range cfg.AllowedGroupIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		items = append(items, telegramconnector.AllowmentValidation{
			ScopeType:       telegramconnector.ScopeGroup,
			ScopeID:         strings.TrimSpace(id),
			Enabled:         true,
			GroupGate:       telegramconnector.GroupGateMentionOrCommandRequired,
			ValidationState: telegramconnector.AllowmentValid,
		})
	}
	return items
}

type slackBotTokenProvider struct {
	secrets   *secrets.Manager
	secretRef string
}

func (p slackBotTokenProvider) BotToken(ctx context.Context, connectorID string) (string, error) {
	if p.secrets == nil {
		return "", fmt.Errorf("slack bot token secret manager is not configured")
	}
	tenantID, err := tenantctx.Require(ctx)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(p.secretRef)
	if ref == "" {
		ref = "slack/" + strings.TrimSpace(connectorID) + "/bot_token"
	}
	resolved, err := p.secrets.Resolve(ctx, secrets.ResolveInput{TenantID: tenantID, SecretRef: ref})
	if err != nil {
		return "", err
	}
	return resolved.Value, nil
}

func slackBotTokenSecretRef(cfg config.SlackConnectorConfig) string {
	if strings.TrimSpace(cfg.BotTokenSecretRef) != "" {
		return strings.TrimSpace(cfg.BotTokenSecretRef)
	}
	return "slack/" + strings.TrimSpace(cfg.ConnectorID) + "/bot_token"
}

type matrixBotAccessTokenProvider struct {
	secrets *secrets.Manager
}

func (p matrixBotAccessTokenProvider) MatrixAccessToken(ctx context.Context, connectorID string) (string, error) {
	if p.secrets == nil {
		return "", fmt.Errorf("matrix bot access token secret manager is not configured")
	}
	tenantID, err := tenantctx.Require(ctx)
	if err != nil {
		return "", err
	}
	ref := "matrix/" + strings.TrimSpace(connectorID) + "/bot_access_token"
	resolved, err := p.secrets.Resolve(ctx, secrets.ResolveInput{TenantID: tenantID, SecretRef: ref})
	if err != nil {
		return "", err
	}
	return resolved.Value, nil
}

func defaultEvaluationFixturesDir() string {
	candidates := []string{
		filepath.Join("internal", "evaluation", "testdata", "fixtures"),
		filepath.Join("daemon", "internal", "evaluation", "testdata", "fixtures"),
	}
	if _, file, _, ok := stdruntime.Caller(0); ok {
		appDir := filepath.Dir(file)
		candidates = append(candidates,
			filepath.Clean(filepath.Join(appDir, "..", "evaluation", "testdata", "fixtures")),
			filepath.Clean(filepath.Join(appDir, "..", "..", "internal", "evaluation", "testdata", "fixtures")),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
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
	if a.telegramRuntime != nil {
		starter, ok := a.telegramRuntime.(interface{ Start(context.Context) error })
		if !ok {
			return fmt.Errorf("telegram runtime is not startable")
		}
		if err := starter.Start(ctx); err != nil {
			return err
		}
	}
	if a.slackRuntime != nil {
		starter, ok := a.slackRuntime.(interface{ Start(context.Context) error })
		if !ok {
			return fmt.Errorf("slack runtime is not startable")
		}
		if err := starter.Start(ctx); err != nil {
			return err
		}
	}
	if a.matrixRuntime != nil {
		starter, ok := a.matrixRuntime.(interface{ Start(context.Context) error })
		if !ok {
			return fmt.Errorf("matrix runtime is not startable")
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
	if a.Reminders != nil {
		if err := a.Reminders.Start(ctx); err != nil {
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
	if a.telegramRuntime != nil {
		if err := a.telegramRuntime.Close(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.slackRuntime != nil {
		if err := a.slackRuntime.Close(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.matrixRuntime != nil {
		if err := a.matrixRuntime.Close(context.Background()); err != nil && firstErr == nil {
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
	if a.Reminders != nil {
		if err := a.Reminders.Close(); err != nil && firstErr == nil {
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

// wireIntegrationAdapters spawns one out-of-process adapter per integration domain, registers
// the adapter-backed Backend on each manager, and starts a supervised runtime that gates
// readiness on the contract-version handshake (Roadmap 59). It is invoked only when the
// operator explicitly enables adapters; the in-daemon fake backend stays registered for
// fake_local-bound integrations. A failed readiness probe is logged and leaves the adapter
// circuit-breaking under supervision rather than aborting daemon startup.
func wireIntegrationAdapters(ctx context.Context, binary string, sup *capabilities.Supervisor, calMgr *calendar.Manager, mailMgr *mail.Manager, integrationMgr *integrations.Manager, secretMgr *secrets.Manager) error {
	// Credentials are resolved per call. When a real provider is selected (Roadmap 60/63), the
	// resolver is backed by the Roadmap 37 secret path so per-call scoped tokens reach the
	// adapter and missing credentials fail closed; the reference adapter needs no credentials.
	providerKind := strings.ToLower(strings.TrimSpace(os.Getenv("DOPE_ADAPTER_PROVIDER")))
	var fetcher adapterrpc.IntegrationCredentialFetcher
	calProviderKind := ""
	mailProviderKind := ""
	if providerKind == "feishu_lark" || providerKind == "feishu" || providerKind == "lark" {
		fetcher = integrationSecretFetcher(integrationMgr, secretMgr)
		calProviderKind = string(integrations.BackendKindFeishuLark)
		mailProviderKind = string(integrations.BackendKindFeishuLark)
	}
	creds := adapterrpc.ScopedResolver(fetcher)
	start := func(domain string) (*adapterrpc.Client, error) {
		client, err := adapterrpc.NewProcessClient(ctx, binary)
		if err != nil {
			return nil, fmt.Errorf("spawn %s adapter: %w", domain, err)
		}
		client.WithCredentials(creds)
		rt := capabilities.StartAdapterRuntime(sup, "integration-adapter-"+domain, domain, client)
		if perr := rt.Probe(ctx); perr != nil {
			slog.Warn("integration.adapter_probe_failed", "domain", domain, "error", perr)
		}
		return client, nil
	}

	calClient, err := start("calendar")
	if err != nil {
		return err
	}
	calMgr.RegisterBackend(integrations.BackendKindAdapterRPC, calendar.NewAdapterBackend(calClient, 0).WithProviderKind(calProviderKind))

	mailClient, err := start("mail")
	if err != nil {
		return err
	}
	mailMgr.RegisterBackend(integrations.BackendKindAdapterRPC, mail.NewAdapterBackend(mailClient, 0).WithProviderKind(mailProviderKind))
	return nil
}

// integrationSecretFetcher resolves an integration's scoped, short-lived credential material
// through the Roadmap 37 secret path. It fails closed (returns an error, never anonymous
// material) when the integration or its credential binding is absent, so the operation fails
// with a stable auth diagnostic rather than calling the provider unauthenticated (FR-012).
func integrationSecretFetcher(integrationMgr *integrations.Manager, secretMgr *secrets.Manager) adapterrpc.IntegrationCredentialFetcher {
	return func(ctx context.Context, integrationID string) (json.RawMessage, error) {
		if integrationMgr == nil || secretMgr == nil {
			return nil, fmt.Errorf("integration credential resolution is not configured")
		}
		resource, ok := integrationMgr.Get(integrationID)
		if !ok {
			return nil, fmt.Errorf("integration %q not found for credential resolution", integrationID)
		}
		ref := strings.TrimSpace(resource.BackendBinding.BackendRefID)
		if ref == "" {
			return nil, fmt.Errorf("integration %q has no credential binding", integrationID)
		}
		resolved, err := secretMgr.Resolve(ctx, secrets.ResolveInput{TenantID: resource.TenantID, SecretRef: ref})
		if err != nil {
			return nil, fmt.Errorf("resolve integration credential: %w", err)
		}
		// The stored value is the scoped access token; granted scopes ride on secret metadata
		// where present. No token material is logged here.
		return json.Marshal(map[string]any{"accessToken": resolved.Value})
	}
}

func recoverPersistedState(ctx context.Context, environment config.Environment, sqliteStore *store.SQLiteStore, sessionRouter *router.SessionRouter, checkpointManager *checkpoints.Manager, eventBus *events.Bus, connectorSupervisor *connectors.Supervisor, capabilitySupervisor *capabilities.Supervisor, policyEngine *policy.Engine, authManager *auth.Manager, identityManager *identity.Manager, providerManager *providers.Manager, sandboxManager *sandbox.Manager, mcpManager *mcp.Manager, integrationManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, reminderManager *reminders.Manager) error {
	return recoverPersistedStateWithSecrets(ctx, "", environment, sqliteStore, sessionRouter, checkpointManager, eventBus, connectorSupervisor, capabilitySupervisor, policyEngine, authManager, identityManager, providerManager, sandboxManager, nil, mcpManager, integrationManager, calendarManager, mailManager, reminderManager)
}

func recoverPersistedStateWithSecrets(ctx context.Context, dataDir string, environment config.Environment, sqliteStore *store.SQLiteStore, sessionRouter *router.SessionRouter, checkpointManager *checkpoints.Manager, eventBus *events.Bus, connectorSupervisor *connectors.Supervisor, capabilitySupervisor *capabilities.Supervisor, policyEngine *policy.Engine, authManager *auth.Manager, identityManager *identity.Manager, providerManager *providers.Manager, sandboxManager *sandbox.Manager, secretManager *secrets.Manager, mcpManager *mcp.Manager, integrationManager *integrations.Manager, calendarManager *calendar.Manager, mailManager *mail.Manager, reminderManager *reminders.Manager) error {
	_ = reminderManager
	if sqliteStore == nil {
		return nil
	}

	persistedSessions, err := sqliteStore.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("load persisted sessions: %w", err)
	}
	sessionRouter.RestoreSessions(persistedSessions)
	if stats, err := sqliteStore.RecoverThreadLifecycleAfterRestart(ctx); err != nil {
		return fmt.Errorf("recover thread lifecycle state: %w", err)
	} else if eventBus != nil && (stats.ProjectedLegacySessions > 0 || stats.PartialThreadStates > 0) {
		for _, tenantID := range stats.Tenants {
			eventBus.Publish(events.ThreadRestartRecoveryEvent(tenantID, stats.CheckedThreads, stats.ProjectedLegacySessions, stats.PartialThreadStates))
		}
	}

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
	if identityManager != nil {
		var localTokenIDs []string
		for _, token := range persistedTokens {
			if token.Mode == auth.PairingModeLocal && (token.Status == "" || token.Status == "active") {
				localTokenIDs = append(localTokenIDs, token.TokenID)
			}
		}
		principal, tenant, err := identityManager.BootstrapLocal(ctx, localTokenIDs)
		if err != nil {
			return fmt.Errorf("bootstrap local identity: %w", err)
		}
		// Roadmap 35 (Pass B): seed the in-memory default-personal-tenant
		// cache so legacy Upsert helpers can bind tenant_id without
		// issuing SQL on the hot path. Required because MaxOpenConns=1
		// would deadlock if the resolver ran inside an active transaction
		// (e.g. ReplaceWorkflowSteps).
		if err := sqliteStore.SeedDefaultTenantCache(ctx); err != nil {
			return fmt.Errorf("seed default tenant cache: %w", err)
		}
		if err := sqliteStore.EnsureDevelopmentBillingPlan(ctx, tenant.TenantID); err != nil {
			return fmt.Errorf("ensure development billing plan: %w", err)
		}
		if secretManager != nil {
			if _, err := secrets.BridgeLocalCredentialFiles(ctx, secrets.LocalCredentialBridgeInput{
				DataDir:       dataDir,
				TenantID:      tenant.TenantID,
				StepName:      store.HostedCredentialBridgeStepName,
				Manager:       secretManager,
				ProgressStore: sqliteStore,
				ResourceStore: sqliteStore,
			}); err != nil {
				return fmt.Errorf("bridge local credential files: %w", err)
			}
		}
		// Roadmap 35 (US2 / T067): drive the runtime spine backfill
		// (sessions/runs/steps/tool_calls/llm_dispatches/checkpoints).
		// Default-personal-tenant fallback uses the just-bootstrapped
		// personal tenant. Idempotent: any already-`completed` step is
		// skipped. On orphan or other failure the step is marked
		// `failed`, the daemon refuses to start on the next boot until
		// the operator resolves it.
		// nil logger is tolerated by the driver; recoverPersistedState
		// does not currently thread a telemetry.Logger and threading one
		// through every existing dependency belongs to a separate
		// refactor. The startup-level guard already publishes
		// daemon.migration.* events on the bus.
		if err := runRuntimeBackfills(ctx, sqliteStore, tenant.TenantID, eventBus, nil); err != nil {
			return fmt.Errorf("runtime backfill: %w", err)
		}
		// Roadmap 35 (US2 / T068+T069+T070+T071): approvals/decisions,
		// schedules, workflows, and integrations+delivery.
		if err := runBatch2Backfills(ctx, sqliteStore, tenant.TenantID, eventBus, nil); err != nil {
			return fmt.Errorf("batch2 backfill: %w", err)
		}
		// Roadmap 35 (US2 / T072–T076b): calendar/mail/reminders/
		// computer-use/evaluation/harness/connector_messages.
		if err := runBatch3Backfills(ctx, sqliteStore, tenant.TenantID, eventBus, nil); err != nil {
			return fmt.Errorf("batch3 backfill: %w", err)
		}
		// Roadmap 35 (US2 / T077): events backfill runs LAST. It
		// derives tenant_id from event parent FKs (priority cascade)
		// and reclassifies legacy connector/capability events into
		// the global pool. All earlier backfills MUST have completed
		// for parent.tenant_id to be non-NULL during the cascade.
		if err := runEventsBackfill(ctx, sqliteStore, tenant.TenantID, eventBus, nil); err != nil {
			return fmt.Errorf("events backfill: %w", err)
		}
		// Roadmap 35 (US2 / T077a + T077b): step (c) enforcement.
		// Recreates each runtime-spine table with NOT NULL + CHECK on
		// tenant_id; adds the events partial indexes. Gated on the
		// matching backfill step having `completed` status — refuses
		// otherwise with a clear operator error.
		if err := runEnforcementSteps(ctx, sqliteStore, eventBus, nil); err != nil {
			return fmt.Errorf("enforcement: %w", err)
		}
		for idx := range persistedTokens {
			if persistedTokens[idx].Mode != auth.PairingModeLocal {
				continue
			}
			changed := false
			if persistedTokens[idx].Status == "" {
				persistedTokens[idx].Status = "active"
				changed = true
			}
			if persistedTokens[idx].PrincipalID == "" {
				persistedTokens[idx].PrincipalID = principal.PrincipalID
				changed = true
			}
			if persistedTokens[idx].DefaultTenantID == "" {
				persistedTokens[idx].DefaultTenantID = tenant.TenantID
				changed = true
			}
			if changed {
				persistedTokens[idx].UpdatedAt = time.Now().UTC()
				if err := sqliteStore.UpsertAccessToken(ctx, persistedTokens[idx]); err != nil {
					return fmt.Errorf("persist bootstrap token identity %s: %w", persistedTokens[idx].TokenID, err)
				}
			}
		}
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
	if integrationManager != nil {
		persistedIntegrations, err := sqliteStore.ListIntegrations(ctx, string(environment))
		if err != nil {
			return fmt.Errorf("load persisted integrations: %w", err)
		}
		integrationManager.Restore(persistedIntegrations)
	}
	if calendarManager != nil {
		accounts, err := sqliteStore.ListCalendarAccounts(ctx, string(environment))
		if err != nil {
			return fmt.Errorf("load persisted calendar accounts: %w", err)
		}
		operations, err := sqliteStore.ListCalendarOperations(ctx, string(environment), store.CalendarOperationFilter{})
		if err != nil {
			return fmt.Errorf("load persisted calendar operations: %w", err)
		}
		artifacts, err := sqliteStore.ListCalendarArtifacts(ctx, string(environment), "")
		if err != nil {
			return fmt.Errorf("load persisted calendar artifacts: %w", err)
		}
		calendarManager.Restore(accounts, operations, artifacts)
	}
	if mailManager != nil {
		accounts, err := sqliteStore.ListMailAccounts(ctx, string(environment))
		if err != nil {
			return fmt.Errorf("load persisted mail accounts: %w", err)
		}
		operations, err := sqliteStore.ListMailOperations(ctx, string(environment), store.MailOperationFilter{})
		if err != nil {
			return fmt.Errorf("load persisted mail operations: %w", err)
		}
		artifacts, err := sqliteStore.ListMailArtifacts(ctx, string(environment), "")
		if err != nil {
			return fmt.Errorf("load persisted mail artifacts: %w", err)
		}
		mailManager.Restore(accounts, operations, artifacts)
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

func liveValidationToolClasses(items []string) []livevalidation.ToolClass {
	if len(items) == 0 {
		return nil
	}
	classes := make([]livevalidation.ToolClass, 0, len(items))
	seen := map[livevalidation.ToolClass]bool{}
	for _, item := range items {
		toolClass := livevalidation.ToolClass(strings.TrimSpace(item))
		if toolClass == "" || seen[toolClass] {
			continue
		}
		seen[toolClass] = true
		classes = append(classes, toolClass)
	}
	return classes
}
