package matrix

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
		return nil, fmt.Errorf("matrix connector id is required")
	}
	if strings.TrimSpace(cfg.DisplayName) == "" {
		return nil, fmt.Errorf("matrix display name is required")
	}
	if supervisor == nil || loop == nil {
		return nil, fmt.Errorf("matrix connector dependencies are not configured")
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
		seen:       map[string]struct{}{},
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
		Kind:        ConnectorKind,
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
	if declaredAt.IsZero() {
		declaredAt = time.Now().UTC()
	}
	core := map[baseconnectors.ConformanceArea]baseconnectors.ConformanceResultStatus{}
	for _, area := range baseconnectors.CoreInvariantAreas() {
		core[area] = baseconnectors.ConformanceResultPass
	}
	surfaces := map[string]baseconnectors.SurfaceSupport{
		baseconnectors.MatrixSurfaceTenantProvidedBotSetup:   baseconnectors.SurfaceSupported,
		baseconnectors.MatrixSurfaceHostedHomeserver:         baseconnectors.SurfaceUnsupported,
		baseconnectors.MatrixSurfaceAccountProvisioning:      baseconnectors.SurfaceUnsupported,
		baseconnectors.MatrixSurfaceDirectMessage:            supportFlag(len(cfg.AllowedDirectUserIDs) > 0),
		baseconnectors.MatrixSurfaceAllowedRoomMention:       supportFlag(len(cfg.SelectedRoomIDs) > 0),
		baseconnectors.MatrixSurfaceAllowedRoomCommand:       supportFlag(len(cfg.SelectedRoomIDs) > 0 || len(cfg.ConfiguredCommands) > 0),
		baseconnectors.MatrixSurfaceUnencryptedText:          baseconnectors.SurfaceSupported,
		baseconnectors.MatrixSurfaceEncryptedRooms:           baseconnectors.SurfaceUnsupported,
		baseconnectors.MatrixSurfaceUndecryptableEvents:      baseconnectors.SurfaceUnsupported,
		"e2ee_key_session_management":                        baseconnectors.SurfaceUnsupported,
		baseconnectors.MatrixSurfaceFinalOnlyForegroundReply: baseconnectors.SurfaceSupported,
		baseconnectors.MatrixSurfaceConnectorBackedDelivery:  baseconnectors.SurfaceSupported,
		"whatsapp":                     baseconnectors.SurfaceUnsupported,
		"bridge_automation":            baseconnectors.SurfaceUnsupported,
		"media":                        baseconnectors.SurfaceUnsupported,
		"voice":                        baseconnectors.SurfaceUnsupported,
		"calls":                        baseconnectors.SurfaceUnsupported,
		"reactions":                    baseconnectors.SurfaceUnsupported,
		"thinking_visibility":          baseconnectors.SurfaceUnsupported,
		"incremental_visible_updates":  baseconnectors.SurfaceUnsupported,
		"blocked_route_classification": baseconnectors.SurfaceSupported,
		"standard_durable_identity":    baseconnectors.SurfaceSupported,
	}
	return baseconnectors.CapabilityProfile{
		ProfileID:              "profile_matrix_" + strings.TrimSpace(cfg.ConnectorID),
		ConnectorID:            strings.TrimSpace(cfg.ConnectorID),
		ConnectorKind:          baseconnectors.ConnectorKindMatrix,
		CoreInvariantResults:   core,
		ProviderSurfaceResults: surfaces,
		GroupRoomCapabilities: baseconnectors.GroupRoomCapabilities{
			MentionEvidence:           supportFlag(len(cfg.SelectedRoomIDs) > 0),
			AllowlistEvidence:         supportFlag(len(cfg.SelectedRoomIDs) > 0),
			UnsupportedSourceEvidence: baseconnectors.SurfaceLimited,
			DuplicateMessageEvidence:  baseconnectors.SurfaceSupported,
			EditedMessageEvidence:     baseconnectors.SurfaceUnsupported,
			DeletedMessageEvidence:    baseconnectors.SurfaceUnsupported,
		},
		HandoffCapabilities: baseconnectors.HandoffCapabilities{
			SourceSupport:                 supportFlag(len(cfg.SelectedRoomIDs) > 0),
			DestinationSupport:            supportFlag(len(cfg.SelectedRoomIDs) > 0),
			FirstResponseSourceReferences: baseconnectors.SurfaceSupported,
		},
		EquivalentDurableIdentityRuleID: baseconnectors.MatrixDurableIdentityRuleID,
		EquivalentDurableIdentityRule:   baseconnectors.MatrixDurableIdentityRule,
		DeclaredAt:                      declaredAt,
	}
}

func supportFlag(enabled bool) baseconnectors.SurfaceSupport {
	if enabled {
		return baseconnectors.SurfaceSupported
	}
	return baseconnectors.SurfaceUnsupported
}

func NormalizeInboundEvent(event InboundEvent) InboundEvent {
	event.TenantID = strings.TrimSpace(event.TenantID)
	event.ConnectorID = strings.TrimSpace(event.ConnectorID)
	event.HomeserverID = strings.TrimSpace(event.HomeserverID)
	event.ConversationID = strings.TrimSpace(event.ConversationID)
	event.MatrixEventID = strings.TrimSpace(event.MatrixEventID)
	event.SyncBatchID = strings.TrimSpace(event.SyncBatchID)
	event.TransactionID = strings.TrimSpace(event.TransactionID)
	event.SenderID = strings.TrimSpace(event.SenderID)
	event.Text = strings.TrimSpace(event.Text)
	if event.MessageKind == "" {
		event.MessageKind = MessageUnencryptedText
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = time.Now().UTC()
	}
	return event
}

func routePolicyFromConfig(cfg Config) RoutePolicy {
	now := time.Now().UTC()
	rooms := make([]ConversationRoute, 0, len(cfg.SelectedRoomIDs))
	for _, id := range cfg.SelectedRoomIDs {
		if strings.TrimSpace(id) == "" {
			continue
		}
		rooms = append(rooms, ConversationRoute{
			ConversationID:     strings.TrimSpace(id),
			ConversationType:   ConversationRoom,
			RoomSelectionState: RoomSelected,
			ValidationState:    RoutePolicyValid,
			RedactionStatus:    baseconnectors.RedactionStatusRedacted,
		})
	}
	state := RoutePolicyBlocked
	if len(rooms) > 0 || len(cfg.AllowedDirectUserIDs) > 0 {
		state = RoutePolicyValid
	}
	return NormalizeRoutePolicy(RoutePolicy{
		ConnectorID:         cfg.ConnectorID,
		HomeserverBindingID: "matrix_homeserver_" + strings.TrimSpace(cfg.ConnectorID),
		SelectedRooms:       rooms,
		AllowedDirectUsers:  append([]string(nil), cfg.AllowedDirectUserIDs...),
		RoomInvocationGate:  "bot_mention_or_command_required",
		ConfiguredCommands:  append([]string(nil), cfg.ConfiguredCommands...),
		EncryptedRoomPolicy: "unsupported",
		ValidationState:     state,
		ValidatedAt:         now,
		RedactionStatus:     baseconnectors.RedactionStatusRedacted,
	}, now)
}

func (r *Runtime) NormalizeInboundEvent(ctx context.Context, event InboundEvent) (imtypes.InboundMessage, bool) {
	if r == nil {
		return imtypes.InboundMessage{}, false
	}
	event = NormalizeInboundEvent(event)
	if event.TenantID == "" {
		event.TenantID = r.runtimeTenantID(ctx)
	}
	if event.ConnectorID == "" {
		event.ConnectorID = r.cfg.ConnectorID
	}
	if event.HomeserverID == "" {
		event.HomeserverID = r.cfg.HomeserverID
	}
	if decision, ok := r.requireHostedSetupReady(ctx, event); !ok {
		r.recordEventEvidence(ctx, event, decision)
		r.recordRouteOutcome(event, decision)
		return imtypes.InboundMessage{}, false
	}
	decision := DecideRoute(event, r.policy, r.cfg.HomeserverID, r.cfg.BotUserID)
	if decision.Outcome == RouteAccepted {
		if r.hasPersistedDuplicate(ctx, event) || r.markDuplicate(matrixEventIdentityKey(event.TenantID, event.ConnectorID, event.HomeserverID, event.ConversationID, event.MatrixEventID)) {
			decision = RouteDecision{Outcome: RouteDuplicate, ReasonCode: string(baseconnectors.DiagnosticDuplicateInbound), Surface: decision.Surface}
		}
	}
	r.recordEventEvidence(ctx, event, decision)
	r.recordRouteOutcome(event, decision)
	if decision.Outcome != RouteAccepted {
		return imtypes.InboundMessage{}, false
	}
	kind := router.SessionKindGroup
	peerID := event.ConversationID
	direct := false
	if event.ConversationType == ConversationDirectMessage {
		kind = router.SessionKindDirect
		peerID = event.SenderID
		direct = true
	}
	return imtypes.InboundMessage{
		ConnectorID:             event.ConnectorID,
		ConnectorKind:           ConnectorKind,
		ExternalMessageID:       event.MatrixEventID,
		TenantID:                event.TenantID,
		AccountID:               event.HomeserverID,
		ConnectorAccountID:      event.HomeserverID,
		ChannelOrConversationID: event.ConversationID,
		ProviderMessageID:       event.MatrixEventID,
		EquivalentRuleID:        baseconnectors.MatrixDurableIdentityRuleID,
		ChannelID:               event.ConversationID,
		PeerID:                  peerID,
		ThreadID:                event.ConversationID,
		AuthorID:                event.SenderID,
		Content:                 decision.NormalizedText,
		Kind:                    kind,
		ReplyToMessageID:        event.MatrixEventID,
		Direct:                  direct,
		Mentioned:               event.ConversationType == ConversationRoom,
		ReceivedAt:              event.ReceivedAt,
	}, true
}

func (r *Runtime) requireHostedSetupReady(ctx context.Context, event InboundEvent) (RouteDecision, bool) {
	if r == nil || r.store == nil {
		return RouteDecision{}, true
	}
	setup, ok, err := r.store.GetMatrixHostedSetup(ctx, event.TenantID, event.ConnectorID)
	if err != nil {
		return RouteDecision{Outcome: RouteFailed, ReasonCode: string(baseconnectors.DiagnosticUnknownConnectorFailure), Surface: string(event.ConversationType)}, false
	}
	if !ok {
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticAuthMissing), Surface: string(event.ConversationType)}, false
	}
	if setup.TerminalState != string(TerminalReady) || !setup.DeliveryEligible {
		reason := strings.TrimSpace(setup.ReasonCode)
		if reason == "" || reason == "healthy" {
			reason = string(baseconnectors.DiagnosticAuthMissing)
		}
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: reason, Surface: string(event.ConversationType)}, false
	}
	return RouteDecision{}, true
}

func (r *Runtime) handleEvent(ctx context.Context, event InboundEvent) {
	inbound, ok := r.NormalizeInboundEvent(ctx, event)
	if !ok || r.loop == nil {
		return
	}
	_, err := r.loop.ProcessSingleTurn(ctx, baseconnectors.Connector{
		ConnectorID: r.cfg.ConnectorID,
		Kind:        ConnectorKind,
		DisplayName: r.cfg.DisplayName,
		Status:      baseconnectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, inbound, r.transport)
	if err != nil && r.logger != nil {
		r.logger.Error("matrix message loop failed", "connector_id", r.cfg.ConnectorID, "event_id", event.MatrixEventID, "error", err.Error())
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

func (r *Runtime) hasPersistedDuplicate(ctx context.Context, event InboundEvent) bool {
	if r == nil || r.store == nil {
		return false
	}
	items, err := r.store.ListMatrixEventEvidence(ctx, event.TenantID, event.ConnectorID, time.Now().UTC(), 100)
	if err != nil {
		return false
	}
	for _, item := range items {
		if item.HomeserverID == event.HomeserverID &&
			item.ConversationID == event.ConversationID &&
			item.MatrixEventID == event.MatrixEventID &&
			(item.RouteOutcome == string(RouteAccepted) || item.RouteOutcome == string(RouteDuplicate)) {
			return true
		}
	}
	return false
}

func matrixEventIdentityKey(values ...string) string {
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
	_ = r.store.SaveMatrixEventEvidence(ctx, store.MatrixEventEvidenceRecord{
		TenantID:           event.TenantID,
		ConnectorID:        event.ConnectorID,
		HomeserverID:       event.HomeserverID,
		ConversationID:     event.ConversationID,
		MatrixEventID:      event.MatrixEventID,
		SyncBatchID:        event.SyncBatchID,
		TransactionID:      event.TransactionID,
		RouteOutcome:       string(decision.Outcome),
		ReasonCode:         decision.ReasonCode,
		ReceivedAt:         receivedAt,
		RetentionExpiresAt: receivedAt.Add(90 * 24 * time.Hour),
		RedactionStatus:    string(baseconnectors.RedactionStatusRedacted),
		SafeEvidence: map[string]string{
			"identityRule": baseconnectors.MatrixDurableIdentityRuleID,
			"surface":      decision.Surface,
		},
	})
}

func (r *Runtime) recordRouteOutcome(event InboundEvent, decision RouteDecision) {
	if r == nil || r.eventBus == nil {
		return
	}
	r.eventBus.Publish(events.ConnectorMatrixRouteOutcomeRecorded(events.ConnectorMatrixRouteOutcomeRecordedInput{
		TenantID:        event.TenantID,
		ConnectorID:     event.ConnectorID,
		HomeserverID:    event.HomeserverID,
		ConversationID:  event.ConversationID,
		MatrixEventID:   event.MatrixEventID,
		SyncBatchID:     event.SyncBatchID,
		TransactionID:   event.TransactionID,
		Outcome:         string(decision.Outcome),
		ReasonCode:      decision.ReasonCode,
		Surface:         decision.Surface,
		RedactionStatus: "redacted",
	}))
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
