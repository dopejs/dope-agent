package discord

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
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type Config struct {
	Enabled           bool
	ConnectorID       string
	DisplayName       string
	DeliveryMode      string
	BotToken          string
	RequireMention    bool
	RespondInDM       bool
	AllowedGuildIDs   []string
	AllowedChannelIDs []string
}

type Transport interface {
	Start(ctx context.Context, handle func(context.Context, imtypes.InboundMessage)) error
	SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error)
	ReplyCapabilities() imtypes.ReplyCapabilities
	SendThinking(ctx context.Context, signal imtypes.ThinkingSignal) error
	EditReply(ctx context.Context, edit imtypes.ReplyEdit) error
	Close(ctx context.Context) error
}

type DestinationValidator interface {
	ValidateDestinations(ctx context.Context, destinations []DestinationValidation) ([]DestinationValidation, error)
}

type TransportLifecycleEvent struct {
	ReasonCode baseconnectors.DiagnosticReasonCode
	Evidence   map[string]string
	Degraded   bool
}

type LifecycleObservableTransport interface {
	SetLifecycleObserver(func(context.Context, TransportLifecycleEvent))
}

var newGatewayTransport = func(cfg Config) (Transport, error) {
	return NewGatewayTransport(cfg)
}

type Runtime struct {
	cfg        Config
	logger     *slog.Logger
	supervisor *baseconnectors.Supervisor
	loop       *im.MessageLoop
	store      *store.SQLiteStore
	eventBus   *events.Bus
	transport  Transport

	mu      sync.Mutex
	started bool
}

func NewRuntime(cfg Config, logger *slog.Logger, supervisor *baseconnectors.Supervisor, loop *im.MessageLoop, sqliteStore *store.SQLiteStore, eventBus *events.Bus, transport Transport) (*Runtime, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.ConnectorID) == "" {
		return nil, fmt.Errorf("discord connector id is required")
	}
	if strings.TrimSpace(cfg.DisplayName) == "" {
		return nil, fmt.Errorf("discord display name is required")
	}
	if mode := strings.TrimSpace(cfg.DeliveryMode); mode == "" {
		cfg.DeliveryMode = "gateway"
	} else if mode != "gateway" {
		return nil, fmt.Errorf("unsupported discord delivery mode: %s", mode)
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, fmt.Errorf("discord bot token is required")
	}
	if supervisor == nil || loop == nil {
		return nil, fmt.Errorf("discord connector dependencies are not configured")
	}
	if transport == nil {
		var err error
		transport, err = newGatewayTransport(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Runtime{
		cfg:        cfg,
		logger:     logger,
		supervisor: supervisor,
		loop:       loop,
		store:      sqliteStore,
		eventBus:   eventBus,
		transport:  transport,
	}, nil
}

func ConformanceProfile(cfg Config, declaredAt time.Time) baseconnectors.CapabilityProfile {
	return conformanceProfileForEvidence(cfg, HostedSetup{}, false, declaredAt)
}

func ConformanceProfileForSetup(cfg Config, setup HostedSetup, declaredAt time.Time) baseconnectors.CapabilityProfile {
	validated := setup.HostedReady && selectedDestinationsValid(setup.Destinations)
	return conformanceProfileForEvidence(cfg, setup, validated, declaredAt)
}

func conformanceProfileForEvidence(cfg Config, setup HostedSetup, validated bool, declaredAt time.Time) baseconnectors.CapabilityProfile {
	if declaredAt.IsZero() {
		declaredAt = time.Now().UTC()
	}
	core := map[baseconnectors.ConformanceArea]baseconnectors.ConformanceResultStatus{}
	coreStatus := baseconnectors.ConformanceResultFail
	if validated {
		coreStatus = baseconnectors.ConformanceResultPass
	}
	for _, area := range baseconnectors.CoreInvariantAreas() {
		core[area] = coreStatus
	}
	incrementalSupport := baseconnectors.SurfaceUnsupported
	if coreStatus == baseconnectors.ConformanceResultPass {
		incrementalSupport = baseconnectors.SurfaceLimited
	}
	surfaces := map[string]baseconnectors.SurfaceSupport{
		"direct_message":               supportFlag(cfg.RespondInDM),
		"group_channel":                supportFlag(len(cfg.AllowedGuildIDs) > 0 || len(cfg.AllowedChannelIDs) > 0),
		"mention_gating":               supportFlag(cfg.RequireMention),
		"room":                         baseconnectors.SurfaceUnsupported,
		"voice":                        baseconnectors.SurfaceUnsupported,
		"thread_reply":                 baseconnectors.SurfaceLimited,
		"thinking_visibility":          baseconnectors.SurfaceLimited,
		"incremental_visible_updates":  incrementalSupport,
		"rich_media":                   baseconnectors.SurfaceUnsupported,
		"placeholder_card_update":      baseconnectors.SurfaceUnsupported,
		"provider_specific_stop":       baseconnectors.SurfaceUnsupported,
		"connector_backed_delivery":    baseconnectors.SurfaceSupported,
		"final_only_foreground_reply":  baseconnectors.SurfaceSupported,
		"thinking_plus_final_reply":    baseconnectors.SurfaceSupported,
		"thinking_plus_incremental":    baseconnectors.SurfaceUnsupported,
		"equivalent_durable_identity":  baseconnectors.SurfaceUnsupported,
		"standard_durable_identity":    baseconnectors.SurfaceSupported,
		"blocked_route_classification": baseconnectors.SurfaceSupported,
	}
	return baseconnectors.CapabilityProfile{
		ProfileID:                       "profile_discord_" + cfg.ConnectorID,
		TenantID:                        setup.TenantID,
		ConnectorID:                     cfg.ConnectorID,
		ConnectorKind:                   "discord",
		CoreInvariantResults:            core,
		ProviderSurfaceResults:          surfaces,
		EquivalentDurableIdentityRuleID: "discord_message_id",
		EquivalentDurableIdentityRule:   "tenant_id + connector_account_id + channel_or_conversation_id + provider_message_id",
		DeclaredAt:                      declaredAt,
	}
}

func supportFlag(enabled bool) baseconnectors.SurfaceSupport {
	if enabled {
		return baseconnectors.SurfaceSupported
	}
	return baseconnectors.SurfaceUnsupported
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
		Kind:        "discord",
		DisplayName: r.cfg.DisplayName,
	})
	if err != nil {
		return err
	}
	if err := r.persistConnector(ctx, connector); err != nil {
		return err
	}

	if observable, ok := r.transport.(LifecycleObservableTransport); ok {
		observable.SetLifecycleObserver(func(eventCtx context.Context, event TransportLifecycleEvent) {
			if event.ReasonCode != "" {
				_ = r.persistDiagnostic(eventCtx, event.ReasonCode, event.Evidence)
			}
			if event.Degraded {
				if degraded, reportErr := r.supervisor.ReportHealth(r.cfg.ConnectorID, baseconnectors.ReportHealthInput{Status: baseconnectors.StatusDegraded}); reportErr == nil {
					_ = r.persistConnector(eventCtx, degraded)
				}
			}
		})
	}

	if err := r.transport.Start(ctx, r.handleInbound); err != nil {
		r.started = false
		_ = r.persistDiagnostic(ctx, DiagnosticReasonForError(err), map[string]string{"stage": "transport_start"})
		_ = r.persistHostedSetupProjection(ctx)
		if failed, reportErr := r.supervisor.ReportFailure(r.cfg.ConnectorID, baseconnectors.ReportFailureInput{Reason: err.Error()}); reportErr == nil {
			_ = r.persistConnector(ctx, failed)
			_, _ = r.publishEvent(ctx, "connector.failed", map[string]any{
				"kind":         failed.Kind,
				"status":       failed.Status,
				"deliveryMode": r.cfg.DeliveryMode,
				"error":        err.Error(),
				"errorClass":   classifyDiscordError(err),
			})
		}
		return err
	}
	if err := r.persistHostedSetupProjection(ctx); err != nil {
		return err
	}

	connector, err = r.supervisor.ReportHealth(r.cfg.ConnectorID, baseconnectors.ReportHealthInput{
		Status: baseconnectors.StatusHealthy,
	})
	if err != nil {
		return err
	}
	if err := r.persistConnector(ctx, connector); err != nil {
		return err
	}
	_, _ = r.publishEvent(ctx, "connector.healthy", map[string]any{
		"kind":         connector.Kind,
		"status":       connector.Status,
		"deliveryMode": r.cfg.DeliveryMode,
	})
	return nil
}

func (r *Runtime) persistHostedSetupProjection(ctx context.Context) error {
	if r.store == nil {
		return nil
	}
	now := time.Now().UTC()
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:       r.runtimeTenantID(ctx),
		ConnectorID:    r.cfg.ConnectorID,
		DisplayName:    r.cfg.DisplayName,
		Credential:     credentialStateForConfig(r.cfg),
		RespondInDM:    r.cfg.RespondInDM,
		RequireMention: r.cfg.RequireMention,
		DeliveryMode:   r.cfg.DeliveryMode,
		Destinations:   r.destinationValidationEvidence(ctx, now),
		ValidatedAt:    now,
	})
	record := store.DiscordHostedSetupRecord{
		TenantID:           setup.TenantID,
		ConnectorID:        setup.ConnectorID,
		ConnectorKind:      setup.ConnectorKind,
		DisplayName:        setup.DisplayName,
		Status:             string(setup.Status),
		ReadinessState:     string(setup.ReadinessState),
		HostedReady:        setup.HostedReady,
		CredentialState:    string(setup.CredentialState),
		RespondInDM:        setup.RespondInDM,
		RequireMention:     setup.RequireMention,
		DeliveryMode:       setup.DeliveryMode,
		ReasonCode:         setup.ReasonCode,
		RedactionStatus:    string(setup.RedactionStatus),
		CreatedAt:          setup.CreatedAt,
		UpdatedAt:          setup.UpdatedAt,
		ValidatedAt:        setup.ValidatedAt,
		RetentionExpiresAt: setup.RetentionExpiresAt,
	}
	if err := r.store.SaveDiscordHostedSetup(ctx, record); err != nil {
		return err
	}
	for _, destination := range setup.Destinations {
		if err := r.store.SaveDiscordDestinationValidation(ctx, store.DiscordDestinationValidationRecord{
			TenantID:        destination.TenantID,
			ConnectorID:     destination.ConnectorID,
			DestinationID:   destination.DestinationID,
			DestinationType: string(destination.DestinationType),
			ProviderLabel:   destination.ProviderLabel,
			Selected:        destination.Selected,
			ValidationState: string(destination.ValidationState),
			ReasonCode:      destination.ReasonCode,
			ValidatedAt:     destination.ValidatedAt,
			RedactionStatus: string(destination.RedactionStatus),
			SafeEvidence:    destination.SafeEvidence,
		}); err != nil {
			return err
		}
	}
	if err := r.persistConformanceEvidence(ctx, setup, now); err != nil {
		return err
	}
	event := events.ConnectorDiscordSetupValidated(events.ConnectorDiscordSetupValidatedInput{
		TenantID:        record.TenantID,
		ConnectorID:     record.ConnectorID,
		ReadinessState:  record.ReadinessState,
		HostedReady:     record.HostedReady,
		CredentialState: record.CredentialState,
		ReasonCode:      record.ReasonCode,
		RedactionStatus: record.RedactionStatus,
		ValidatedAt:     record.ValidatedAt,
	})
	persisted, err := r.store.AppendEvent(ctx, event)
	if err != nil {
		return err
	}
	if r.eventBus != nil {
		r.eventBus.Publish(persisted)
	}
	return nil
}

func (r *Runtime) persistConformanceEvidence(ctx context.Context, setup HostedSetup, now time.Time) error {
	if r.store == nil {
		return nil
	}
	profile := ConformanceProfileForSetup(r.cfg, setup, now)
	results, _, err := baseconnectors.RunMatrixCase(baseconnectors.MatrixCase{
		ScenarioID:                      "discord_hosted_setup_" + r.cfg.ConnectorID,
		TenantID:                        setup.TenantID,
		ConnectorKind:                   "discord",
		ConnectorID:                     r.cfg.ConnectorID,
		CoreInvariantResults:            profile.CoreInvariantResults,
		ProviderSurfaceResults:          profile.ProviderSurfaceResults,
		EquivalentDurableIdentityRuleID: profile.EquivalentDurableIdentityRuleID,
		EquivalentDurableIdentityRule:   profile.EquivalentDurableIdentityRule,
		RedactionStatus:                 baseconnectors.RedactionStatusRedacted,
		Now:                             now,
	})
	if err != nil {
		return err
	}
	for _, result := range results {
		if err := r.store.SaveConnectorConformanceResult(ctx, result); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) persistDiagnostic(ctx context.Context, reason baseconnectors.DiagnosticReasonCode, evidence map[string]string) error {
	return r.persistDiagnosticForInbound(ctx, imtypes.InboundMessage{}, reason, evidence)
}

func (r *Runtime) persistDiagnosticForInbound(ctx context.Context, inbound imtypes.InboundMessage, reason baseconnectors.DiagnosticReasonCode, evidence map[string]string) error {
	if r.store == nil || reason == "" {
		return nil
	}
	state, err := BuildDiagnosticState(firstNonEmpty(inbound.TenantID, r.runtimeTenantID(ctx)), r.cfg.ConnectorID, inboundConnectorAccountID(inbound), reason, evidence, time.Now().UTC())
	if err != nil {
		return err
	}
	return r.store.SaveConnectorDiagnosticState(ctx, state)
}

func credentialStateForConfig(cfg Config) CredentialState {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return CredentialMissing
	}
	return CredentialValid
}

func destinationEvidenceForConfig(cfg Config, now time.Time) []DestinationValidation {
	destinations := make([]DestinationValidation, 0, len(cfg.AllowedGuildIDs)+len(cfg.AllowedChannelIDs))
	for _, guildID := range cfg.AllowedGuildIDs {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			continue
		}
		destinations = append(destinations, DestinationValidation{
			ConnectorID:     cfg.ConnectorID,
			DestinationID:   guildID,
			DestinationType: DestinationGuild,
			Selected:        true,
			ValidationState: DestinationStale,
			ReasonCode:      "destination_validation_required",
			ValidatedAt:     now,
			RedactionStatus: baseconnectors.RedactionStatusRedacted,
			SafeEvidence:    map[string]string{"source": "local_config_projection", "validation": "required"},
		})
	}
	for _, channelID := range cfg.AllowedChannelIDs {
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		destinations = append(destinations, DestinationValidation{
			ConnectorID:     cfg.ConnectorID,
			DestinationID:   channelID,
			DestinationType: DestinationChannel,
			Selected:        true,
			ValidationState: DestinationStale,
			ReasonCode:      "destination_validation_required",
			ValidatedAt:     now,
			RedactionStatus: baseconnectors.RedactionStatusRedacted,
			SafeEvidence:    map[string]string{"source": "local_config_projection", "validation": "required"},
		})
	}
	return destinations
}

func (r *Runtime) destinationValidationEvidence(ctx context.Context, now time.Time) []DestinationValidation {
	destinations := destinationEvidenceForConfig(r.cfg, now)
	validator, ok := r.transport.(DestinationValidator)
	if !ok || len(destinations) == 0 {
		return destinations
	}
	validated, err := validator.ValidateDestinations(ctx, destinations)
	if err != nil {
		_ = r.persistDiagnostic(ctx, DiagnosticReasonForError(err), map[string]string{"stage": "destination_validation"})
		for idx := range destinations {
			destinations[idx].ValidationState = DestinationInvalid
			destinations[idx].ReasonCode = string(baseconnectors.DiagnosticUnknownConnectorFailure)
			destinations[idx].SafeEvidence = map[string]string{"source": "transport_validation", "errorClass": classifyDiscordError(err)}
		}
		return destinations
	}
	if len(validated) == 0 {
		return destinations
	}
	return validated
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}
	r.started = false
	r.mu.Unlock()

	return r.transport.Close(ctx)
}

func (r *Runtime) handleInbound(ctx context.Context, inbound imtypes.InboundMessage) {
	var reason string
	inbound, reason = r.normalizeInboundIdentity(ctx, inbound)
	if reason != "" {
		r.publishRouteOutcome(ctx, inbound, "blocked", reason)
		_ = r.persistDiagnosticForInbound(ctx, inbound, baseconnectors.DiagnosticBlockedRoute, map[string]string{"reason": reason, "stage": "identity_binding"})
		return
	}
	if !r.shouldHandle(inbound) {
		reason := discordRouteReason(r.cfg, inbound)
		r.publishRouteOutcome(ctx, inbound, discordRouteOutcome(r.cfg, inbound), reason)
		if discordRouteOutcome(r.cfg, inbound) == "blocked" {
			_ = r.persistDiagnosticForInbound(ctx, inbound, baseconnectors.DiagnosticBlockedRoute, map[string]string{"reason": reason, "stage": "route_gating"})
		}
		return
	}
	if connector, err := r.supervisor.ReportHealth(r.cfg.ConnectorID, baseconnectors.ReportHealthInput{Status: baseconnectors.StatusHealthy}); err == nil {
		_ = r.persistConnector(ctx, connector)
	}

	if r.logger != nil {
		r.logger.Info("discord inbound message accepted", "connector_id", r.cfg.ConnectorID, "message_id", inbound.ExternalMessageID)
	}

	result, err := r.loop.ProcessSingleTurn(ctx, baseconnectors.Connector{
		ConnectorID: r.cfg.ConnectorID,
		Kind:        "discord",
		DisplayName: r.cfg.DisplayName,
		Status:      baseconnectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, inbound, r.transport)
	if err != nil {
		if r.logger != nil {
			r.logger.Error("discord message loop failed", "connector_id", r.cfg.ConnectorID, "message_id", inbound.ExternalMessageID, "error", err.Error())
		}
		_ = r.persistDiagnosticForInbound(ctx, inbound, baseconnectors.DiagnosticReplyFailed, map[string]string{"errorClass": classifyDiscordError(err), "stage": "message_loop"})
		return
	}
	if result.Duplicate {
		_ = r.persistDiagnosticForInbound(ctx, inbound, baseconnectors.DiagnosticDuplicateInbound, map[string]string{"stage": "durable_dedupe"})
		if r.logger != nil {
			r.logger.Info("discord duplicate message ignored", "connector_id", r.cfg.ConnectorID, "message_id", inbound.ExternalMessageID)
		}
	}
}

func (r *Runtime) normalizeInboundIdentity(ctx context.Context, inbound imtypes.InboundMessage) (imtypes.InboundMessage, string) {
	inbound.TenantID = firstNonEmpty(inbound.TenantID, r.runtimeTenantID(ctx))
	inbound.ConnectorID = firstNonEmpty(inbound.ConnectorID, r.cfg.ConnectorID)
	inbound.ConnectorKind = firstNonEmpty(inbound.ConnectorKind, "discord")
	inbound.ConnectorAccountID = inboundConnectorAccountID(inbound)
	inbound.AccountID = firstNonEmpty(inbound.AccountID, inbound.ConnectorAccountID)
	inbound.ChannelOrConversationID = inboundChannelOrConversationID(inbound)
	inbound.ProviderMessageID = inboundProviderMessageID(inbound)
	if inbound.EquivalentRuleID == "" {
		inbound.EquivalentRuleID = "discord_message_id"
	}
	switch {
	case strings.TrimSpace(inbound.TenantID) == "":
		return inbound, "tenant_binding_missing"
	case strings.TrimSpace(inbound.ConnectorAccountID) == "",
		strings.TrimSpace(inbound.ChannelOrConversationID) == "",
		strings.TrimSpace(inbound.ProviderMessageID) == "":
		return inbound, "missing_durable_identity"
	default:
		return inbound, ""
	}
}

func (r *Runtime) publishRouteOutcome(ctx context.Context, inbound imtypes.InboundMessage, outcome, reason string) {
	_, _ = r.publishEvent(ctx, "connector.route_outcome_recorded", map[string]any{
		"tenantId":                inbound.TenantID,
		"connectorId":             r.cfg.ConnectorID,
		"outcome":                 outcome,
		"reasonCode":              reason,
		"surface":                 discordRouteSurface(inbound),
		"connectorAccountId":      inboundConnectorAccountID(inbound),
		"channelOrConversationId": inboundChannelOrConversationID(inbound),
		"providerMessageId":       inboundProviderMessageID(inbound),
		"equivalentRuleId":        inbound.EquivalentRuleID,
		"redactionStatus":         "redacted",
	})
}

func discordRouteOutcome(cfg Config, inbound imtypes.InboundMessage) string {
	if !inbound.Direct && cfg.RequireMention && !inbound.Mentioned {
		return "ignored"
	}
	return "blocked"
}

func discordRouteReason(cfg Config, inbound imtypes.InboundMessage) string {
	switch {
	case inbound.Direct && !cfg.RespondInDM:
		return "direct_message_disabled"
	case len(cfg.AllowedGuildIDs) > 0 && !contains(cfg.AllowedGuildIDs, inbound.GuildID):
		return "blocked_guild"
	case len(cfg.AllowedChannelIDs) > 0 && !contains(cfg.AllowedChannelIDs, inbound.ChannelID):
		return "blocked_channel"
	case !inbound.Direct && cfg.RequireMention && !inbound.Mentioned:
		return "mention_required"
	default:
		return "blocked_route"
	}
}

func discordRouteSurface(inbound imtypes.InboundMessage) string {
	if inbound.Direct {
		return "direct_message"
	}
	if strings.TrimSpace(inbound.ThreadID) != "" && inbound.ThreadID != inbound.ChannelID {
		return "thread_reply"
	}
	return "group_channel"
}

func inboundConnectorAccountID(inbound imtypes.InboundMessage) string {
	return firstNonEmpty(inbound.ConnectorAccountID, inbound.AccountID)
}

func inboundChannelOrConversationID(inbound imtypes.InboundMessage) string {
	return firstNonEmpty(inbound.ChannelOrConversationID, inbound.ChannelID, inbound.PeerID)
}

func inboundProviderMessageID(inbound imtypes.InboundMessage) string {
	return firstNonEmpty(inbound.ProviderMessageID, inbound.ExternalMessageID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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

func (r *Runtime) shouldHandle(inbound imtypes.InboundMessage) bool {
	if inbound.Direct {
		return r.cfg.RespondInDM
	}
	if len(r.cfg.AllowedGuildIDs) > 0 && !contains(r.cfg.AllowedGuildIDs, inbound.GuildID) {
		return false
	}
	if len(r.cfg.AllowedChannelIDs) > 0 && !contains(r.cfg.AllowedChannelIDs, inbound.ChannelID) {
		return false
	}
	if r.cfg.RequireMention && !inbound.Mentioned {
		return false
	}
	return true
}

func (r *Runtime) persistConnector(ctx context.Context, connector baseconnectors.Connector) error {
	if r.store == nil {
		return nil
	}
	return r.store.UpsertConnector(ctx, connector)
}

func (r *Runtime) publishEvent(ctx context.Context, name string, payload map[string]any) (events.Event, error) {
	if r.eventBus == nil {
		return events.Event{}, nil
	}
	event := events.Event{
		Category: "connector",
		Name:     name,
		Scope: events.Scope{
			ConnectorID: r.cfg.ConnectorID,
		},
		Resource: events.Resource{
			Kind: "connector",
			ID:   r.cfg.ConnectorID,
		},
		Payload: payload,
	}
	if r.store != nil {
		persisted, err := r.store.AppendEvent(ctx, event)
		if err != nil {
			return events.Event{}, err
		}
		event = persisted
	}
	return r.eventBus.Publish(event), nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func classifyDiscordError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "401"),
		strings.Contains(message, "unauthorized"),
		strings.Contains(message, "token"):
		return "auth_error"
	case strings.Contains(message, "403"),
		strings.Contains(message, "forbidden"),
		strings.Contains(message, "permission"),
		strings.Contains(message, "message content"):
		return "permission_missing"
	case strings.Contains(message, "429"),
		strings.Contains(message, "rate limit"):
		return "rate_limited"
	case strings.Contains(message, "unavailable"),
		strings.Contains(message, "5xx"):
		return "provider_unavailable"
	case strings.Contains(message, "gateway"),
		strings.Contains(message, "network"),
		strings.Contains(message, "connection"):
		return "network_failed"
	default:
		return "transport_error"
	}
}
