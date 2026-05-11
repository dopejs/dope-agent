package slack

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
	Enabled             bool
	ConnectorID         string
	DisplayName         string
	WorkspaceBindingID  string
	WorkspaceID         string
	BotUserID           string
	AllowedChannelIDs   []string
	AllowedDMUserIDs    []string
	AllowedDMUserGroups []string
}

type Runtime struct {
	cfg        Config
	logger     *slog.Logger
	supervisor *baseconnectors.Supervisor
	loop       *im.MessageLoop
	store      *store.SQLiteStore
	eventBus   *events.Bus
	transport  Transport
	policy     RoutePolicy
	mu         sync.Mutex
	seen       map[string]struct{}
	started    bool
}

func NewRuntime(cfg Config, logger *slog.Logger, supervisor *baseconnectors.Supervisor, loop *im.MessageLoop, sqliteStore *store.SQLiteStore, eventBus *events.Bus, transport Transport) (*Runtime, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.ConnectorID) == "" {
		return nil, fmt.Errorf("slack connector id is required")
	}
	if strings.TrimSpace(cfg.DisplayName) == "" {
		return nil, fmt.Errorf("slack display name is required")
	}
	if supervisor == nil || loop == nil {
		return nil, fmt.Errorf("slack connector dependencies are not configured")
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
		policy:     routePolicyFromConfig(cfg),
		seen:       make(map[string]struct{}),
	}, nil
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
		Kind:        "slack",
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
	return r.transport.Start(ctx, r.handleEvent)
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	r.started = false
	r.mu.Unlock()
	return r.transport.Close(ctx)
}

func ConformanceProfile(cfg Config, declaredAt time.Time) baseconnectors.CapabilityProfile {
	return conformanceProfileForSetup(cfg, HostedSetup{}, false, declaredAt)
}

func ConformanceProfileForSetup(cfg Config, setup HostedSetup, declaredAt time.Time) baseconnectors.CapabilityProfile {
	return conformanceProfileForSetup(cfg, setup, setup.TerminalState == TerminalReady, declaredAt)
}

func conformanceProfileForSetup(cfg Config, setup HostedSetup, ready bool, declaredAt time.Time) baseconnectors.CapabilityProfile {
	if declaredAt.IsZero() {
		declaredAt = time.Now().UTC()
	}
	coreStatus := baseconnectors.ConformanceResultFail
	if ready {
		coreStatus = baseconnectors.ConformanceResultPass
	}
	core := map[baseconnectors.ConformanceArea]baseconnectors.ConformanceResultStatus{}
	for _, area := range baseconnectors.CoreInvariantAreas() {
		core[area] = coreStatus
	}
	surfaces := map[string]baseconnectors.SurfaceSupport{
		"hosted_oauth_setup":             baseconnectors.SurfaceSupported,
		"submitted_token_setup":          baseconnectors.SurfaceUnsupported,
		"workspace_binding":              baseconnectors.SurfaceSupported,
		"multiple_connectors_per_tenant": baseconnectors.SurfaceSupported,
		"direct_message":                 supportFlag(len(cfg.AllowedDMUserIDs) > 0 || len(cfg.AllowedDMUserGroups) > 0),
		"selected_channel_mention":       supportFlag(len(cfg.AllowedChannelIDs) > 0),
		"channel_thread_reply":           baseconnectors.SurfaceSupported,
		"final_only_foreground_reply":    baseconnectors.SurfaceSupported,
		"connector_backed_delivery":      baseconnectors.SurfaceSupported,
		"marketplace_publication":        baseconnectors.SurfaceUnsupported,
		"enterprise_grid_administration": baseconnectors.SurfaceUnsupported,
		"memory_based_team_context":      baseconnectors.SurfaceUnsupported,
		"files":                          baseconnectors.SurfaceUnsupported,
		"voice_huddles":                  baseconnectors.SurfaceUnsupported,
		"canvases":                       baseconnectors.SurfaceUnsupported,
		"workflow_buttons":               baseconnectors.SurfaceUnsupported,
		"interactive_blocks":             baseconnectors.SurfaceUnsupported,
		"rich_media":                     baseconnectors.SurfaceUnsupported,
		"thinking_visibility":            baseconnectors.SurfaceUnsupported,
		"incremental_visible_updates":    baseconnectors.SurfaceUnsupported,
		"standard_durable_identity":      baseconnectors.SurfaceSupported,
		"blocked_route_classification":   baseconnectors.SurfaceSupported,
	}
	return baseconnectors.CapabilityProfile{
		ProfileID:              "profile_slack_" + cfg.ConnectorID,
		TenantID:               setup.TenantID,
		ConnectorID:            cfg.ConnectorID,
		ConnectorKind:          "slack",
		CoreInvariantResults:   core,
		ProviderSurfaceResults: surfaces,
		GroupRoomCapabilities: baseconnectors.GroupRoomCapabilities{
			MentionEvidence:           supportFlag(len(cfg.AllowedChannelIDs) > 0),
			AllowlistEvidence:         supportFlag(len(cfg.AllowedChannelIDs) > 0),
			UnsupportedSourceEvidence: baseconnectors.SurfaceLimited,
			DuplicateMessageEvidence:  baseconnectors.SurfaceSupported,
			EditedMessageEvidence:     baseconnectors.SurfaceUnsupported,
			DeletedMessageEvidence:    baseconnectors.SurfaceUnsupported,
		},
		HandoffCapabilities: baseconnectors.HandoffCapabilities{
			SourceSupport:                 supportFlag(len(cfg.AllowedChannelIDs) > 0),
			DestinationSupport:            supportFlag(len(cfg.AllowedChannelIDs) > 0),
			FirstResponseSourceReferences: baseconnectors.SurfaceSupported,
		},
		EquivalentDurableIdentityRuleID: "slack_workspace_conversation_message_id",
		EquivalentDurableIdentityRule:   "tenant_id + connector_id + workspace_id + conversation_id + slack_message_id",
		DeclaredAt:                      declaredAt,
	}
}

func supportFlag(enabled bool) baseconnectors.SurfaceSupport {
	if enabled {
		return baseconnectors.SurfaceSupported
	}
	return baseconnectors.SurfaceUnsupported
}

func routePolicyFromConfig(cfg Config) RoutePolicy {
	channels := make([]ConversationRoute, 0, len(cfg.AllowedChannelIDs))
	for _, id := range cfg.AllowedChannelIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		channels = append(channels, ConversationRoute{
			ConversationID:       strings.TrimSpace(id),
			ConversationType:     ConversationChannel,
			SelectedChannelState: SelectedChannelSelected,
			ValidationState:      RoutePolicyValid,
			RedactionStatus:      baseconnectors.RedactionStatusRedacted,
		})
	}
	return NormalizeRoutePolicy(RoutePolicy{
		ConnectorID:         cfg.ConnectorID,
		WorkspaceBindingID:  cfg.WorkspaceBindingID,
		SelectedChannels:    channels,
		AllowedDMUsers:      append([]string(nil), cfg.AllowedDMUserIDs...),
		AllowedDMUserGroups: append([]string(nil), cfg.AllowedDMUserGroups...),
		ValidationState:     RoutePolicyValid,
		RedactionStatus:     baseconnectors.RedactionStatusRedacted,
	}, time.Now().UTC())
}

func (r *Runtime) NormalizeInboundEvent(ctx context.Context, event InboundEvent) (imtypes.InboundMessage, bool) {
	if r == nil {
		return imtypes.InboundMessage{}, false
	}
	if strings.TrimSpace(event.TenantID) == "" {
		event.TenantID = r.runtimeTenantID(ctx)
	}
	if strings.TrimSpace(event.ConnectorID) == "" {
		event.ConnectorID = r.cfg.ConnectorID
	}
	if decision, ok := r.requireHostedSetupReady(ctx, event); !ok {
		r.recordEventEvidence(ctx, event, decision)
		r.recordRouteOutcome(event, decision)
		return imtypes.InboundMessage{}, false
	}
	decision := DecideRoute(event, r.policy, r.cfg.WorkspaceID, r.cfg.BotUserID)
	if decision.Outcome == RouteAccepted {
		key := slackMessageIdentityKey(event.TenantID, event.ConnectorID, event.WorkspaceID, event.ConversationID, event.MessageID)
		if r.markDuplicate(key) {
			decision = RouteDecision{Outcome: RouteDuplicate, ReasonCode: string(baseconnectors.DiagnosticDuplicateInbound), Surface: decision.Surface}
		}
	}
	r.recordEventEvidence(ctx, event, decision)
	r.recordRouteOutcome(event, decision)
	if decision.Outcome != RouteAccepted {
		return imtypes.InboundMessage{}, false
	}
	receivedAt := event.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	kind := router.SessionKindGroup
	peerID := event.ConversationID
	replyToMessageID := event.ThreadRootMessageID
	if strings.TrimSpace(replyToMessageID) == "" && event.ConversationType == ConversationChannel {
		replyToMessageID = event.MessageID
	}
	if event.ConversationType == ConversationDirectMessage {
		kind = router.SessionKindDirect
		peerID = event.SenderID
		replyToMessageID = event.MessageID
	}
	return imtypes.InboundMessage{
		ConnectorID:             event.ConnectorID,
		ConnectorKind:           "slack",
		ExternalMessageID:       event.MessageID,
		TenantID:                event.TenantID,
		AccountID:               event.WorkspaceID,
		ConnectorAccountID:      event.WorkspaceID,
		ChannelOrConversationID: event.ConversationID,
		ProviderMessageID:       event.MessageID,
		EquivalentRuleID:        "slack_workspace_conversation_message_id",
		ChannelID:               event.ConversationID,
		PeerID:                  peerID,
		ThreadID:                firstNonEmpty(event.ThreadRootMessageID, event.MessageID),
		AuthorID:                event.SenderID,
		Content:                 decision.NormalizedText,
		Kind:                    kind,
		ReplyToMessageID:        replyToMessageID,
		ReceivedAt:              receivedAt,
		Direct:                  event.ConversationType == ConversationDirectMessage,
		Mentioned:               event.ConversationType == ConversationChannel,
	}, true
}

func (r *Runtime) requireHostedSetupReady(ctx context.Context, event InboundEvent) (RouteDecision, bool) {
	if r == nil || r.store == nil {
		return RouteDecision{}, true
	}
	setup, ok, err := r.store.GetSlackHostedSetup(ctx, event.TenantID, event.ConnectorID)
	if err != nil {
		return RouteDecision{Outcome: RouteFailed, ReasonCode: string(baseconnectors.DiagnosticUnknownConnectorFailure), Surface: firstNonEmpty(event.Surface, string(event.ConversationType))}, false
	}
	if !ok {
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticAuthMissing), Surface: firstNonEmpty(event.Surface, string(event.ConversationType))}, false
	}
	if setup.TerminalState != string(TerminalReady) || !setup.DeliveryEligible {
		reason := strings.TrimSpace(setup.ReasonCode)
		if reason == "" || reason == "healthy" {
			reason = string(baseconnectors.DiagnosticAuthMissing)
		}
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: reason, Surface: firstNonEmpty(event.Surface, string(event.ConversationType))}, false
	}
	if strings.TrimSpace(setup.WorkspaceBindingID) != "" && strings.TrimSpace(r.cfg.WorkspaceBindingID) != "" && strings.TrimSpace(setup.WorkspaceBindingID) != strings.TrimSpace(r.cfg.WorkspaceBindingID) {
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: "workspace_mismatch", Surface: firstNonEmpty(event.Surface, string(event.ConversationType))}, false
	}
	return RouteDecision{}, true
}

func (r *Runtime) handleEvent(ctx context.Context, event InboundEvent) {
	inbound, ok := r.NormalizeInboundEvent(ctx, event)
	if !ok || r.loop == nil {
		return
	}
	result, err := r.loop.ProcessSingleTurn(ctx, baseconnectors.Connector{
		ConnectorID: r.cfg.ConnectorID,
		Kind:        "slack",
		DisplayName: r.cfg.DisplayName,
		Status:      baseconnectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, inbound, r.transport)
	if result.Duplicate {
		r.recordDiagnostic(ctx, baseconnectors.DiagnosticDuplicateInbound, map[string]string{
			"workspaceId":    event.WorkspaceID,
			"conversationId": event.ConversationID,
			"surface":        string(event.ConversationType),
			"stage":          "durable_dedupe",
		})
	}
	if err != nil {
		r.recordDiagnostic(ctx, DiagnosticReasonForError(err), map[string]string{
			"workspaceId":    event.WorkspaceID,
			"conversationId": event.ConversationID,
			"surface":        string(event.ConversationType),
			"stage":          "message_loop",
		})
		if r.logger != nil {
			r.logger.Error("slack message loop failed", "connector_id", r.cfg.ConnectorID, "message_id", event.MessageID, "error", err.Error())
		}
	}
}

func (r *Runtime) markDuplicate(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.seen[key]; ok {
		return true
	}
	r.seen[key] = struct{}{}
	return false
}

func slackMessageIdentityKey(values ...string) string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		cleaned = append(cleaned, strings.TrimSpace(value))
	}
	return strings.Join(cleaned, "\x00")
}

func (r *Runtime) recordEventEvidence(ctx context.Context, event InboundEvent, decision RouteDecision) {
	if r == nil || r.store == nil {
		return
	}
	receivedAt := event.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	_ = r.store.SaveSlackEventEvidence(ctx, store.SlackEventEvidenceRecord{
		TenantID:           event.TenantID,
		ConnectorID:        event.ConnectorID,
		WorkspaceID:        event.WorkspaceID,
		ConversationID:     event.ConversationID,
		MessageID:          event.MessageID,
		EventID:            event.EventID,
		RouteOutcome:       string(decision.Outcome),
		ReasonCode:         decision.ReasonCode,
		ReceivedAt:         receivedAt,
		RetentionExpiresAt: receivedAt.Add(90 * 24 * time.Hour),
		RedactionStatus:    string(baseconnectors.RedactionStatusRedacted),
		SafeEvidence: map[string]string{
			"identityRule": "slack_workspace_conversation_message_id",
			"surface":      decision.Surface,
		},
	})
}

func (r *Runtime) recordRouteOutcome(event InboundEvent, decision RouteDecision) {
	if r == nil || r.eventBus == nil {
		return
	}
	r.eventBus.Publish(events.ConnectorSlackRouteOutcomeRecorded(events.ConnectorSlackRouteOutcomeRecordedInput{
		TenantID:        event.TenantID,
		ConnectorID:     event.ConnectorID,
		WorkspaceID:     event.WorkspaceID,
		ConversationID:  event.ConversationID,
		MessageID:       event.MessageID,
		EventID:         event.EventID,
		Outcome:         string(decision.Outcome),
		ReasonCode:      decision.ReasonCode,
		Surface:         decision.Surface,
		RedactionStatus: "redacted",
	}))
}

func (r *Runtime) recordDiagnostic(ctx context.Context, reason baseconnectors.DiagnosticReasonCode, evidence map[string]string) {
	if r == nil || r.store == nil || reason == "" {
		return
	}
	state, err := BuildDiagnosticState(r.runtimeTenantID(ctx), r.cfg.ConnectorID, r.cfg.WorkspaceBindingID, reason, evidence, time.Now().UTC())
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("slack diagnostic state not built", "connector_id", r.cfg.ConnectorID, "error", err.Error())
		}
		return
	}
	if err := r.store.SaveConnectorDiagnosticState(ctx, state); err != nil && r.logger != nil {
		r.logger.Warn("slack diagnostic state not persisted", "connector_id", r.cfg.ConnectorID, "error", err.Error())
	}
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
		record := store.SlackHostedSetupRecord{
			TenantID:           setup.TenantID,
			ConnectorID:        setup.ConnectorID,
			ConnectorKind:      setup.ConnectorKind,
			DisplayName:        setup.DisplayName,
			Status:             string(setup.Status),
			TerminalState:      string(setup.TerminalState),
			OAuthState:         string(setup.OAuthState),
			RoutePolicyState:   string(setup.RoutePolicyState),
			DeliveryEligible:   setup.DeliveryEligible,
			WorkspaceBindingID: setup.WorkspaceBindingID,
			ReasonCode:         setup.ReasonCode,
			RedactionStatus:    string(setup.RedactionStatus),
			CreatedAt:          setup.CreatedAt,
			UpdatedAt:          setup.UpdatedAt,
			ValidatedAt:        setup.ValidatedAt,
			RetentionExpiresAt: setup.RetentionExpiresAt,
		}
		if strings.TrimSpace(setup.WorkspaceBinding.WorkspaceID) != "" || strings.TrimSpace(setup.WorkspaceBinding.InstallationID) != "" {
			record.WorkspaceBinding = &store.SlackWorkspaceBinding{
				TenantID:           setup.WorkspaceBinding.TenantID,
				ConnectorID:        setup.WorkspaceBinding.ConnectorID,
				WorkspaceBindingID: setup.WorkspaceBinding.WorkspaceBindingID,
				WorkspaceID:        setup.WorkspaceBinding.WorkspaceID,
				WorkspaceLabel:     setup.WorkspaceBinding.WorkspaceLabel,
				InstallationID:     setup.WorkspaceBinding.InstallationID,
				OAuthGrantState:    setup.WorkspaceBinding.OAuthGrantState,
				RequiredScopeState: setup.WorkspaceBinding.RequiredScopeState,
				ValidatedAt:        setup.WorkspaceBinding.ValidatedAt,
				RedactionStatus:    string(setup.WorkspaceBinding.RedactionStatus),
				SafeEvidence:       setup.WorkspaceBinding.SafeEvidence,
			}
		}
		if err := r.store.SaveSlackHostedSetup(ctx, record); err != nil {
			return setup, err
		}
		if setup.RoutePolicy.ValidationState != "" {
			if err := r.store.SaveSlackRoutePolicy(ctx, slackRoutePolicyRecord(setup)); err != nil {
				return setup, err
			}
		}
	}
	if r.eventBus != nil {
		r.eventBus.Publish(events.ConnectorSlackSetupValidated(events.ConnectorSlackSetupValidatedInput{
			TenantID:           setup.TenantID,
			ConnectorID:        setup.ConnectorID,
			WorkspaceBindingID: setup.WorkspaceBindingID,
			TerminalState:      string(setup.TerminalState),
			OAuthState:         string(setup.OAuthState),
			RoutePolicyState:   string(setup.RoutePolicyState),
			DeliveryEligible:   setup.DeliveryEligible,
			ReasonCode:         setup.ReasonCode,
			SlackCondition:     slackConditionForSetup(setup),
			RedactionStatus:    string(setup.RedactionStatus),
			ValidatedAt:        setup.ValidatedAt,
		}))
	}
	return setup, nil
}

func slackRoutePolicyRecord(setup HostedSetup) store.SlackRoutePolicyRecord {
	selected := make([]store.SlackConversationRouteRecord, 0, len(setup.RoutePolicy.SelectedChannels))
	for _, route := range setup.RoutePolicy.SelectedChannels {
		selected = append(selected, store.SlackConversationRouteRecord{
			ConversationID:       route.ConversationID,
			ConversationType:     string(route.ConversationType),
			SelectedChannelState: string(route.SelectedChannelState),
			ValidationState:      string(route.ValidationState),
			ReasonCode:           route.ReasonCode,
			RedactionStatus:      string(route.RedactionStatus),
			SafeEvidence:         route.SafeEvidence,
		})
	}
	return store.SlackRoutePolicyRecord{
		TenantID:            setup.TenantID,
		ConnectorID:         setup.ConnectorID,
		WorkspaceBindingID:  setup.WorkspaceBindingID,
		SelectedChannels:    selected,
		AllowedDMUsers:      append([]string(nil), setup.RoutePolicy.AllowedDMUsers...),
		AllowedDMUserGroups: append([]string(nil), setup.RoutePolicy.AllowedDMUserGroups...),
		MentionGate:         setup.RoutePolicy.MentionGate,
		ThreadReplyMode:     setup.RoutePolicy.ThreadReplyMode,
		ValidationState:     string(setup.RoutePolicy.ValidationState),
		ReasonCode:          setup.RoutePolicy.ReasonCode,
		ValidatedAt:         setup.RoutePolicy.ValidatedAt,
		RedactionStatus:     string(setup.RoutePolicy.RedactionStatus),
		SafeEvidence:        setup.RoutePolicy.SafeEvidence,
	}
}

func slackConditionForSetup(setup HostedSetup) string {
	if setup.ReasonCode == "" || setup.ReasonCode == "healthy" {
		return "healthy"
	}
	return setup.ReasonCode
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
