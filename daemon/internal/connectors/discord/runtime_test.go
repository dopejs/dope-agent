package discord

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type testProvider struct{}

func (p *testProvider) Name() string { return "echo" }

func (p *testProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       "reply:" + request.Messages[0].Content,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *testProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	response, err := p.Complete(context.Background(), request)
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	if err := emit(llm.StreamChunk{Delta: response.Output}); err != nil {
		return llm.ProviderResponse{}, err
	}
	return response, nil
}

type fakeTransport struct {
	handler     func(context.Context, imtypes.InboundMessage)
	sent        []imtypes.OutboundReply
	edited      []imtypes.ReplyEdit
	thinking    []imtypes.ThinkingSignal
	closed      bool
	startErr    error
	caps        imtypes.ReplyCapabilities
	validations []DestinationValidation
	validateErr error
}

func (t *fakeTransport) Start(_ context.Context, handle func(context.Context, imtypes.InboundMessage)) error {
	if t.startErr != nil {
		return t.startErr
	}
	t.handler = handle
	return nil
}

func (t *fakeTransport) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	t.sent = append(t.sent, reply)
	return imtypes.SentReply{ExternalMessageID: "discord_reply_1"}, nil
}

func (t *fakeTransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return t.caps
}

func (t *fakeTransport) SendThinking(_ context.Context, signal imtypes.ThinkingSignal) error {
	t.thinking = append(t.thinking, signal)
	return nil
}

func (t *fakeTransport) EditReply(_ context.Context, edit imtypes.ReplyEdit) error {
	t.edited = append(t.edited, edit)
	return nil
}

func (t *fakeTransport) Close(context.Context) error {
	t.closed = true
	return nil
}

func (t *fakeTransport) ValidateDestinations(_ context.Context, destinations []DestinationValidation) ([]DestinationValidation, error) {
	if t.validateErr != nil {
		return nil, t.validateErr
	}
	if t.validations != nil {
		return t.validations, nil
	}
	return destinations, nil
}

func TestRuntimeProcessesDirectMessageEndToEnd(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testProvider{})
	if err := dispatcher.SetDefaultProvider("echo"); err != nil {
		t.Fatalf("SetDefaultProvider returned error: %v", err)
	}
	dispatcher.SetDefaultModel("echo-v1")

	chatService := chat.NewService(dispatcher, providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
			DefaultModel:    "echo-v1",
		},
	}, dispatcher, nil), nil, eventBus, sqliteStore)

	supervisor := baseconnectors.NewSupervisor()
	runtimeManager := runtime.NewManager()
	loop := im.NewMessageLoop(
		router.NewSessionRouter(),
		runtimeManager,
		checkpoints.NewManager(sqliteStore, runtimeManager),
		eventBus,
		sqliteStore,
		chatService,
	)
	transport := &fakeTransport{caps: imtypes.ReplyCapabilities{SupportsThinking: true, SupportsStreaming: true}}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_discord", PrincipalID: "operator"})

	connectorRuntime, err := NewRuntime(Config{
		Enabled:        true,
		ConnectorID:    "discord-main",
		DisplayName:    "Discord Main",
		BotToken:       "secret",
		RequireMention: true,
		RespondInDM:    true,
	}, slog.Default(), supervisor, loop, sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	if err := connectorRuntime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	transport.handler(ctx, imtypes.InboundMessage{
		TenantID:          "ten_discord",
		ConnectorID:       "discord-main",
		ConnectorKind:     "discord",
		ExternalMessageID: "discord_msg_1",
		AccountID:         "bot_1",
		ChannelID:         "dm_1",
		PeerID:            "user_1",
		AuthorID:          "user_1",
		Content:           "hello",
		Kind:              router.SessionKindDirect,
		Direct:            true,
		ReceivedAt:        time.Now().UTC(),
	})

	if len(transport.thinking) == 0 {
		t.Fatal("expected at least one thinking signal")
	}
	if len(transport.sent) != 1 {
		t.Fatalf("expected 1 initial reply to be sent, got %d", len(transport.sent))
	}
	if transport.sent[0].Content != "reply:hello" {
		t.Fatalf("expected reply content, got %q", transport.sent[0].Content)
	}
	if len(transport.edited) != 0 {
		t.Fatalf("expected no edit for single-chunk stream, got %d edits", len(transport.edited))
	}

	items, err := sqliteStore.ListConnectors(context.Background())
	if err != nil {
		t.Fatalf("ListConnectors returned error: %v", err)
	}
	if len(items) != 1 || items[0].Status != baseconnectors.StatusHealthy {
		t.Fatalf("expected persisted healthy discord connector, got %#v", items)
	}
}

func TestRuntimeIgnoresGuildMessageWithoutMentionWhenRequired(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&testProvider{})
	if err := dispatcher.SetDefaultProvider("echo"); err != nil {
		t.Fatalf("SetDefaultProvider returned error: %v", err)
	}
	dispatcher.SetDefaultModel("echo-v1")

	eventBus := events.NewBus()
	chatService := chat.NewService(dispatcher, providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
			DefaultModel:    "echo-v1",
		},
	}, dispatcher, nil), nil, eventBus, sqliteStore)
	supervisor := baseconnectors.NewSupervisor()
	runtimeManager := runtime.NewManager()
	loop := im.NewMessageLoop(
		router.NewSessionRouter(),
		runtimeManager,
		checkpoints.NewManager(sqliteStore, runtimeManager),
		eventBus,
		sqliteStore,
		chatService,
	)
	transport := &fakeTransport{caps: imtypes.ReplyCapabilities{SupportsThinking: true, SupportsStreaming: true}}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_discord", PrincipalID: "operator"})

	connectorRuntime, err := NewRuntime(Config{
		Enabled:        true,
		ConnectorID:    "discord-main",
		DisplayName:    "Discord Main",
		BotToken:       "secret",
		RequireMention: true,
		RespondInDM:    true,
	}, slog.Default(), supervisor, loop, sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	if err := connectorRuntime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	transport.handler(ctx, imtypes.InboundMessage{
		TenantID:          "ten_discord",
		ConnectorID:       "discord-main",
		ConnectorKind:     "discord",
		ExternalMessageID: "discord_msg_2",
		AccountID:         "bot_1",
		ChannelID:         "channel_1",
		GuildID:           "guild_1",
		PeerID:            "channel_1",
		ThreadID:          "channel_1",
		AuthorID:          "user_1",
		Content:           "hello",
		Kind:              router.SessionKindGroup,
		Direct:            false,
		Mentioned:         false,
		ReceivedAt:        time.Now().UTC(),
	})

	if len(transport.sent) != 0 {
		t.Fatalf("expected guild message without mention to be ignored, got %d replies", len(transport.sent))
	}
	connectorEvents := eventBus.List(events.Filter{Category: "connector"})
	if len(connectorEvents) == 0 {
		t.Fatal("expected route outcome event")
	}
	last := connectorEvents[len(connectorEvents)-1]
	if last.Name != "connector.route_outcome_recorded" {
		t.Fatalf("expected route outcome event, got %s", last.Name)
	}
	if got := last.Payload["outcome"]; got != "ignored" {
		t.Fatalf("expected ignored outcome, got %#v", got)
	}
	if got := last.Payload["reasonCode"]; got != "mention_required" {
		t.Fatalf("expected mention_required reason, got %#v", got)
	}
}

func TestNewRuntimeRejectsMissingBotToken(t *testing.T) {
	t.Parallel()

	_, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "discord-main",
		DisplayName: "Discord Main",
	}, slog.Default(), baseconnectors.NewSupervisor(), im.NewMessageLoop(nil, nil, nil, nil, nil, nil), nil, nil, &fakeTransport{})
	if err == nil {
		t.Fatal("expected missing bot token to be rejected")
	}
}

func TestRuntimePublishesClassifiedFailureWhenTransportStartFails(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	eventBus := events.NewBus()
	supervisor := baseconnectors.NewSupervisor()
	runtimeManager := runtime.NewManager()
	loop := im.NewMessageLoop(
		router.NewSessionRouter(),
		runtimeManager,
		checkpoints.NewManager(sqliteStore, runtimeManager),
		eventBus,
		sqliteStore,
		nil,
	)
	transport := &fakeTransport{startErr: errors.New("401 Unauthorized"), caps: imtypes.ReplyCapabilities{SupportsThinking: true, SupportsStreaming: true}}

	connectorRuntime, err := NewRuntime(Config{
		Enabled:        true,
		ConnectorID:    "discord-main",
		DisplayName:    "Discord Main",
		DeliveryMode:   "gateway",
		BotToken:       "secret",
		RequireMention: true,
		RespondInDM:    true,
	}, slog.Default(), supervisor, loop, sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	if err := connectorRuntime.Start(context.Background()); err == nil {
		t.Fatal("expected transport start failure")
	}

	connectorEvents := eventBus.List(events.Filter{Category: "connector"})
	if len(connectorEvents) == 0 {
		t.Fatal("expected connector failure event")
	}
	last := connectorEvents[len(connectorEvents)-1]
	if last.Name != "connector.failed" {
		t.Fatalf("expected connector.failed event, got %s", last.Name)
	}
	if got := last.Payload["errorClass"]; got != "auth_error" {
		t.Fatalf("expected auth_error classification, got %#v", got)
	}
}

func TestRuntimeDoesNotMarkHostedReadyWithoutDestinationValidationEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_discord", PrincipalID: "operator"})
	eventBus := events.NewBus()
	supervisor := baseconnectors.NewSupervisor()
	runtimeManager := runtime.NewManager()
	loop := im.NewMessageLoop(
		router.NewSessionRouter(),
		runtimeManager,
		checkpoints.NewManager(sqliteStore, runtimeManager),
		eventBus,
		sqliteStore,
		nil,
	)

	connectorRuntime, err := NewRuntime(Config{
		Enabled:           true,
		ConnectorID:       "discord-main",
		DisplayName:       "Discord Main",
		DeliveryMode:      "gateway",
		BotToken:          "secret",
		RequireMention:    true,
		RespondInDM:       true,
		AllowedGuildIDs:   []string{"guild_1"},
		AllowedChannelIDs: []string{"channel_1"},
	}, slog.Default(), supervisor, loop, sqliteStore, eventBus, &fakeTransport{})
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	if err := connectorRuntime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	setup, found, err := sqliteStore.GetDiscordHostedSetup(ctx, "ten_discord", "discord-main")
	if err != nil || !found {
		t.Fatalf("GetDiscordHostedSetup found=%v err=%v", found, err)
	}
	if setup.HostedReady || setup.ReadinessState != string(ReadinessDegradedNeedsRepair) {
		t.Fatalf("setup=%+v, want degraded until destination validation evidence exists", setup)
	}
	if setup.ReasonCode != "destination_validation_failed" {
		t.Fatalf("reason=%q, want destination_validation_failed", setup.ReasonCode)
	}
}

func TestRuntimeMarksHostedReadyWithValidatedDestinationEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_discord", PrincipalID: "operator"})
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	transport := &fakeTransport{validations: []DestinationValidation{
		{
			ConnectorID:     "discord-main",
			DestinationID:   "guild_1",
			DestinationType: DestinationGuild,
			Selected:        true,
			ValidationState: DestinationValid,
			ReasonCode:      "healthy",
			ValidatedAt:     now,
			RedactionStatus: baseconnectors.RedactionStatusRedacted,
			SafeEvidence:    map[string]string{"source": "gateway_state"},
		},
		{
			ConnectorID:     "discord-main",
			DestinationID:   "channel_1",
			DestinationType: DestinationChannel,
			Selected:        true,
			ValidationState: DestinationValid,
			ReasonCode:      "healthy",
			ValidatedAt:     now,
			RedactionStatus: baseconnectors.RedactionStatusRedacted,
			SafeEvidence:    map[string]string{"source": "gateway_state"},
		},
	}}
	eventBus := events.NewBus()
	supervisor := baseconnectors.NewSupervisor()
	runtimeManager := runtime.NewManager()
	loop := im.NewMessageLoop(
		router.NewSessionRouter(),
		runtimeManager,
		checkpoints.NewManager(sqliteStore, runtimeManager),
		eventBus,
		sqliteStore,
		nil,
	)

	connectorRuntime, err := NewRuntime(Config{
		Enabled:           true,
		ConnectorID:       "discord-main",
		DisplayName:       "Discord Main",
		DeliveryMode:      "gateway",
		BotToken:          "secret",
		RequireMention:    true,
		RespondInDM:       true,
		AllowedGuildIDs:   []string{"guild_1"},
		AllowedChannelIDs: []string{"channel_1"},
	}, slog.Default(), supervisor, loop, sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	if err := connectorRuntime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	setup, found, err := sqliteStore.GetDiscordHostedSetup(ctx, "ten_discord", "discord-main")
	if err != nil || !found {
		t.Fatalf("GetDiscordHostedSetup found=%v err=%v", found, err)
	}
	if !setup.HostedReady || setup.ReadinessState != string(ReadinessHostedReady) {
		t.Fatalf("setup=%+v, want hosted_ready with validated destination evidence", setup)
	}
	results, err := sqliteStore.ListConnectorConformanceResults(ctx, "ten_discord", "discord-main", time.Now().UTC())
	if err != nil {
		t.Fatalf("ListConnectorConformanceResults returned error: %v", err)
	}
	passedCore := 0
	for _, result := range results {
		if result.Result == baseconnectors.ConformanceResultPass {
			passedCore++
		}
	}
	if passedCore < len(baseconnectors.CoreInvariantAreas()) {
		t.Fatalf("expected persisted passing core conformance results, got %+v", results)
	}
}

func TestRuntimeBlocksInboundMissingDurableIdentity(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_discord", PrincipalID: "operator"})
	eventBus := events.NewBus()
	supervisor := baseconnectors.NewSupervisor()
	runtimeManager := runtime.NewManager()
	loop := im.NewMessageLoop(
		router.NewSessionRouter(),
		runtimeManager,
		checkpoints.NewManager(sqliteStore, runtimeManager),
		eventBus,
		sqliteStore,
		nil,
	)
	transport := &fakeTransport{caps: imtypes.ReplyCapabilities{SupportsThinking: true, SupportsStreaming: true}}
	connectorRuntime, err := NewRuntime(Config{
		Enabled:        true,
		ConnectorID:    "discord-main",
		DisplayName:    "Discord Main",
		DeliveryMode:   "gateway",
		BotToken:       "secret",
		RequireMention: false,
		RespondInDM:    true,
	}, slog.Default(), supervisor, loop, sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	if err := connectorRuntime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	transport.handler(ctx, imtypes.InboundMessage{
		TenantID:          "ten_discord",
		ConnectorID:       "discord-main",
		ConnectorKind:     "discord",
		ExternalMessageID: "discord_msg_missing_account",
		ChannelID:         "dm_1",
		PeerID:            "user_1",
		AuthorID:          "user_1",
		Content:           "hello",
		Kind:              router.SessionKindDirect,
		Direct:            true,
		ReceivedAt:        time.Now().UTC(),
	})

	if len(transport.sent) != 0 {
		t.Fatalf("expected no replies for missing durable identity, got %d", len(transport.sent))
	}
	connectorEvents := eventBus.List(events.Filter{Category: "connector"})
	if len(connectorEvents) == 0 {
		t.Fatal("expected blocked route event")
	}
	last := connectorEvents[len(connectorEvents)-1]
	if last.Name != "connector.route_outcome_recorded" {
		t.Fatalf("expected route outcome event, got %s", last.Name)
	}
	if got := last.Payload["outcome"]; got != "blocked" {
		t.Fatalf("outcome=%#v, want blocked", got)
	}
	if got := last.Payload["reasonCode"]; got != "missing_durable_identity" {
		t.Fatalf("reason=%#v, want missing_durable_identity", got)
	}

	diagnostics, err := sqliteStore.ListConnectorDiagnosticStates(ctx, "ten_discord", "discord-main", time.Now().UTC())
	if err != nil {
		t.Fatalf("ListConnectorDiagnosticStates returned error: %v", err)
	}
	if len(diagnostics) == 0 || diagnostics[len(diagnostics)-1].ReasonCode != baseconnectors.DiagnosticBlockedRoute {
		t.Fatalf("expected blocked route diagnostic, got %+v", diagnostics)
	}
}

func TestDiscordConformanceProfileDeclaresSurfacesWithoutSyntheticCorePasses(t *testing.T) {
	t.Parallel()

	profile := ConformanceProfile(Config{
		ConnectorID:    "discord-main",
		RequireMention: true,
		RespondInDM:    true,
	}, time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC))

	if err := baseconnectors.ValidateCapabilityProfile(profile); err == nil {
		t.Fatal("expected declared Discord profile without matrix evidence to fail core invariant validation")
	}
	if got := profile.ProviderSurfaceResults["direct_message"]; got != baseconnectors.SurfaceSupported {
		t.Fatalf("direct_message support=%s, want supported", got)
	}
	if got := profile.ProviderSurfaceResults["thread_reply"]; got != baseconnectors.SurfaceLimited {
		t.Fatalf("thread_reply support=%s, want limited", got)
	}
	if got := profile.ProviderSurfaceResults["incremental_visible_updates"]; got != baseconnectors.SurfaceUnsupported {
		t.Fatalf("incremental_visible_updates support=%s, want unsupported", got)
	}
}
