package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type Config struct {
	Enabled     bool
	ConnectorID string
	DisplayName string
	BotToken    string
	BotUsername string
	Allowments  []AllowmentValidation
}

type Runtime struct {
	cfg        Config
	logger     *slog.Logger
	supervisor *baseconnectors.Supervisor
	loop       *im.MessageLoop
	store      *store.SQLiteStore
	eventBus   *events.Bus
	transport  Transport
	allowments AllowmentIndex

	mu      sync.Mutex
	started bool
}

func NewRuntime(cfg Config, logger *slog.Logger, supervisor *baseconnectors.Supervisor, loop *im.MessageLoop, sqliteStore *store.SQLiteStore, eventBus *events.Bus, transport Transport) (*Runtime, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.ConnectorID) == "" {
		return nil, fmt.Errorf("telegram connector id is required")
	}
	if strings.TrimSpace(cfg.DisplayName) == "" {
		return nil, fmt.Errorf("telegram display name is required")
	}
	if supervisor == nil {
		return nil, fmt.Errorf("telegram connector supervisor is not configured")
	}
	if transport == nil {
		transport = NewFakeTransport()
	}
	return &Runtime{
		cfg:        cfg,
		logger:     logger,
		supervisor: supervisor,
		loop:       loop,
		store:      sqliteStore,
		eventBus:   eventBus,
		transport:  transport,
		allowments: NewAllowmentIndex(cfg.Allowments),
	}, nil
}

func ConformanceProfile(cfg Config, declaredAt time.Time) baseconnectors.CapabilityProfile {
	if declaredAt.IsZero() {
		declaredAt = time.Now().UTC()
	}
	core := map[baseconnectors.ConformanceArea]baseconnectors.ConformanceResultStatus{}
	for _, area := range baseconnectors.CoreInvariantAreas() {
		core[area] = baseconnectors.ConformanceResultPass
	}
	return baseconnectors.CapabilityProfile{
		ProfileID:            "profile_telegram_" + cfg.ConnectorID,
		ConnectorID:          cfg.ConnectorID,
		ConnectorKind:        "telegram",
		CoreInvariantResults: core,
		ProviderSurfaceResults: map[string]baseconnectors.SurfaceSupport{
			"direct_message":               baseconnectors.SurfaceSupported,
			"group_message":                baseconnectors.SurfaceSupported,
			"mention_gating":               baseconnectors.SurfaceSupported,
			"command_gating":               baseconnectors.SurfaceSupported,
			"final_only_foreground_reply":  baseconnectors.SurfaceSupported,
			"connector_backed_delivery":    baseconnectors.SurfaceSupported,
			"attachments":                  baseconnectors.SurfaceUnsupported,
			"voice":                        baseconnectors.SurfaceUnsupported,
			"payments":                     baseconnectors.SurfaceUnsupported,
			"mini_apps":                    baseconnectors.SurfaceUnsupported,
			"media_transfer":               baseconnectors.SurfaceUnsupported,
			"thinking_visibility":          baseconnectors.SurfaceUnsupported,
			"incremental_visible_updates":  baseconnectors.SurfaceUnsupported,
			"standard_durable_identity":    baseconnectors.SurfaceSupported,
			"blocked_route_classification": baseconnectors.SurfaceSupported,
		},
		EquivalentDurableIdentityRuleID: "telegram_chat_message_id",
		EquivalentDurableIdentityRule:   "tenant_id + connector_account_id + telegram_chat_id + telegram_message_id",
		DeclaredAt:                      declaredAt,
	}
}

func (r *Runtime) Start(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.started = true
	r.mu.Unlock()
	connector, _, err := r.supervisor.Register(baseconnectors.RegisterInput{
		TenantID:    r.runtimeTenantID(ctx),
		ConnectorID: r.cfg.ConnectorID,
		Kind:        "telegram",
		DisplayName: r.cfg.DisplayName,
	})
	if err != nil {
		return err
	}
	if r.store != nil {
		if err := r.store.UpsertConnector(ctx, connector); err != nil {
			return err
		}
	}
	return r.transport.Start(ctx, r.handleUpdate)
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.transport.Close(ctx)
}

func (r *Runtime) RecordHostedSetupValidation(ctx context.Context, input HostedSetupInput) (HostedSetup, error) {
	if r == nil {
		return HostedSetup{}, nil
	}
	if strings.TrimSpace(input.TenantID) == "" {
		input.TenantID = r.runtimeTenantID(ctx)
	}
	if strings.TrimSpace(input.ConnectorID) == "" {
		input.ConnectorID = r.cfg.ConnectorID
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		input.DisplayName = r.cfg.DisplayName
	}
	setup := EvaluateHostedSetup(input)
	if r.store != nil {
		record := store.TelegramHostedSetupRecord{
			TenantID:           setup.TenantID,
			ConnectorID:        setup.ConnectorID,
			ConnectorKind:      setup.ConnectorKind,
			DisplayName:        setup.DisplayName,
			Status:             string(setup.Status),
			TerminalState:      string(setup.TerminalState),
			HostedReady:        setup.HostedReady,
			CredentialState:    string(setup.CredentialState),
			AllowmentState:     string(setup.AllowmentState),
			GroupBehavior:      string(setup.GroupBehavior),
			DeliveryEligible:   setup.DeliveryEligible,
			ReasonCode:         setup.ReasonCode,
			RedactionStatus:    string(setup.RedactionStatus),
			CreatedAt:          setup.CreatedAt,
			UpdatedAt:          setup.UpdatedAt,
			ValidatedAt:        setup.ValidatedAt,
			RetentionExpiresAt: setup.RetentionExpiresAt,
		}
		if strings.TrimSpace(setup.AccountBinding.ConnectorAccountID) != "" {
			record.AccountBinding = &store.ConnectorAccountBindingSummary{
				TenantID:            setup.AccountBinding.TenantID,
				ConnectorID:         setup.AccountBinding.ConnectorID,
				ConnectorAccountID:  setup.AccountBinding.ConnectorAccountID,
				DisplayName:         setup.AccountBinding.ProviderAccountLabel,
				ProviderAccountHint: setup.AccountBinding.ProviderAccountLabel,
				RedactionStatus:     string(setup.AccountBinding.RedactionStatus),
				UpdatedAt:           setup.AccountBinding.ValidatedAt,
			}
		}
		if err := r.store.SaveTelegramHostedSetup(ctx, record); err != nil {
			return setup, err
		}
		for _, allowment := range setup.Allowments {
			if err := r.store.SaveTelegramAllowment(ctx, store.TelegramAllowmentRecord{
				TenantID:        allowment.TenantID,
				ConnectorID:     allowment.ConnectorID,
				AllowmentID:     allowment.AllowmentID,
				ScopeType:       string(allowment.ScopeType),
				ScopeID:         allowment.ScopeID,
				ProviderLabel:   allowment.ProviderLabel,
				Enabled:         allowment.Enabled,
				GroupGate:       string(allowment.GroupGate),
				ValidationState: string(allowment.ValidationState),
				ReasonCode:      allowment.ReasonCode,
				ValidatedAt:     allowment.ValidatedAt,
				RedactionStatus: string(allowment.RedactionStatus),
				SafeEvidence:    allowment.SafeEvidence,
			}); err != nil {
				return setup, err
			}
		}
	}
	if r.eventBus != nil {
		r.eventBus.Publish(events.ConnectorTelegramSetupValidated(events.ConnectorTelegramSetupValidatedInput{
			TenantID:        setup.TenantID,
			ConnectorID:     setup.ConnectorID,
			TerminalState:   string(setup.TerminalState),
			HostedReady:     setup.HostedReady,
			CredentialState: string(setup.CredentialState),
			AllowmentState:  string(setup.AllowmentState),
			ReasonCode:      setup.ReasonCode,
			RedactionStatus: string(setup.RedactionStatus),
			ValidatedAt:     setup.ValidatedAt,
		}))
	}
	return setup, nil
}

func (r *Runtime) handleUpdate(ctx context.Context, update InboundUpdate) {
	inbound, ok := r.NormalizeInbound(ctx, update)
	if !ok || r.loop == nil {
		return
	}
	result, err := r.loop.ProcessSingleTurn(ctx, baseconnectors.Connector{
		ConnectorID: r.cfg.ConnectorID,
		Kind:        "telegram",
		DisplayName: r.cfg.DisplayName,
		Status:      baseconnectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, inbound, r.transport)
	if result.Duplicate {
		r.recordUpdateEvidence(ctx, update, RouteDecision{
			Outcome:    RouteDuplicate,
			ReasonCode: string(baseconnectors.DiagnosticDuplicateInbound),
			Surface:    string(update.ConversationType),
		})
		r.recordDiagnostic(ctx, baseconnectors.DiagnosticDuplicateInbound, map[string]string{
			"messageId": update.MessageID,
			"chatId":    update.ChatID,
			"surface":   string(update.ConversationType),
		})
	}
	if err != nil {
		r.recordDiagnostic(ctx, DiagnosticReasonForError(err), map[string]string{
			"messageId": update.MessageID,
			"chatId":    update.ChatID,
		})
		if r.logger != nil {
			r.logger.Error("telegram message loop failed", "connector_id", r.cfg.ConnectorID, "message_id", inbound.ExternalMessageID, "error", err.Error())
		}
	}
}

func (r *Runtime) NormalizeInbound(ctx context.Context, update InboundUpdate) (imtypes.InboundMessage, bool) {
	if update.ConversationType == "" {
		update.ConversationType = ConversationDirect
	}
	if update.Mentioned == false && r.cfg.BotUsername != "" {
		text, mentioned, command := NormalizeCommandText(update.Text, r.cfg.BotUsername)
		update.Text = text
		update.Mentioned = mentioned
		update.Command = update.Command || command
	}
	decision := DecideRoute(update, r.allowments)
	if decision.Outcome != RouteAccepted {
		r.recordUpdateEvidence(ctx, update, decision)
		r.recordRouteOutcome(decision)
		if reason := diagnosticReasonForRouteDecision(decision); reason != "" {
			r.recordDiagnostic(ctx, reason, map[string]string{
				"messageId": update.MessageID,
				"chatId":    update.ChatID,
				"surface":   decision.Surface,
			})
		}
		return imtypes.InboundMessage{}, false
	}
	r.recordUpdateEvidence(ctx, update, decision)
	now := update.ReceivedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	kind := router.SessionKindDirect
	peerID := update.SenderID
	if update.ConversationType == ConversationGroup {
		kind = router.SessionKindGroup
		peerID = update.ChatID
	}
	return imtypes.InboundMessage{
		ConnectorID:             r.cfg.ConnectorID,
		ConnectorKind:           "telegram",
		ExternalMessageID:       update.MessageID,
		TenantID:                r.runtimeTenantID(ctx),
		AccountID:               r.connectorAccountID(),
		ConnectorAccountID:      r.connectorAccountID(),
		ChannelOrConversationID: update.ChatID,
		ProviderMessageID:       update.MessageID,
		EquivalentRuleID:        "telegram_chat_message_id",
		ChannelID:               update.ChatID,
		PeerID:                  peerID,
		AuthorID:                update.SenderID,
		Content:                 strings.TrimSpace(update.Text),
		Kind:                    kind,
		Direct:                  update.ConversationType == ConversationDirect,
		Mentioned:               update.Mentioned || update.Command,
		ReceivedAt:              now,
	}, true
}

func (r *Runtime) recordRouteOutcome(decision RouteDecision) {
	if recorder, ok := r.transport.(interface{ RecordRouteOutcome(RouteDecision) }); ok {
		recorder.RecordRouteOutcome(decision)
	}
}

func (r *Runtime) recordUpdateEvidence(ctx context.Context, update InboundUpdate, decision RouteDecision) {
	if r == nil || r.store == nil {
		return
	}
	if strings.TrimSpace(update.ChatID) == "" || strings.TrimSpace(update.MessageID) == "" || strings.TrimSpace(update.UpdateID) == "" {
		return
	}
	receivedAt := update.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	err := r.store.SaveTelegramUpdateEvidence(ctx, store.TelegramUpdateEvidenceRecord{
		TenantID:           r.runtimeTenantID(ctx),
		ConnectorID:        r.cfg.ConnectorID,
		ChatID:             update.ChatID,
		MessageID:          update.MessageID,
		UpdateID:           update.UpdateID,
		RouteOutcome:       string(decision.Outcome),
		ReasonCode:         decision.ReasonCode,
		ReceivedAt:         receivedAt,
		RetentionExpiresAt: receivedAt.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence: map[string]string{
			"identityRule": "telegram_chat_message_id",
			"surface":      decision.Surface,
		},
	})
	if err != nil && r.logger != nil {
		r.logger.Warn("telegram update evidence not persisted", "connector_id", r.cfg.ConnectorID, "error", err.Error())
	}
}

func (r *Runtime) recordDiagnostic(ctx context.Context, reason baseconnectors.DiagnosticReasonCode, evidence map[string]string) {
	if r == nil || r.store == nil || reason == "" {
		return
	}
	state, err := BuildDiagnosticState(r.runtimeTenantID(ctx), r.cfg.ConnectorID, r.connectorAccountID(), reason, evidence, time.Now().UTC())
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("telegram diagnostic state not built", "connector_id", r.cfg.ConnectorID, "error", err.Error())
		}
		return
	}
	if err := r.store.SaveConnectorDiagnosticState(ctx, state); err != nil && r.logger != nil {
		r.logger.Warn("telegram diagnostic state not persisted", "connector_id", r.cfg.ConnectorID, "error", err.Error())
	}
}

func diagnosticReasonForRouteDecision(decision RouteDecision) baseconnectors.DiagnosticReasonCode {
	switch decision.Outcome {
	case RouteBlocked:
		return baseconnectors.DiagnosticBlockedRoute
	case RouteUnsupported:
		return baseconnectors.DiagnosticUnsupportedCapability
	case RouteFailed:
		return baseconnectors.DiagnosticUnknownConnectorFailure
	default:
		return ""
	}
}

func (r *Runtime) connectorAccountID() string {
	if strings.TrimSpace(r.cfg.BotUsername) != "" {
		return "bot_" + strings.TrimPrefix(strings.TrimSpace(r.cfg.BotUsername), "@")
	}
	return r.cfg.ConnectorID
}

func (r *Runtime) runtimeTenantID(ctx context.Context) string {
	if tenantContext, ok := tenantctx.FromContext(ctx); ok {
		return strings.TrimSpace(tenantContext.TenantID)
	}
	if r != nil && r.store != nil {
		tenantID, err := r.store.ResolveDefaultPersonalTenantID(ctx)
		if err == nil {
			return strings.TrimSpace(tenantID)
		}
	}
	return ""
}
