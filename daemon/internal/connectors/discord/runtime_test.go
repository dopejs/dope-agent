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
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
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
	handler  func(context.Context, imtypes.InboundMessage)
	sent     []imtypes.OutboundReply
	edited   []imtypes.ReplyEdit
	thinking []imtypes.ThinkingSignal
	closed   bool
	startErr error
	caps     imtypes.ReplyCapabilities
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
	if err := connectorRuntime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	transport.handler(context.Background(), imtypes.InboundMessage{
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
	if err := connectorRuntime.Start(context.Background()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	transport.handler(context.Background(), imtypes.InboundMessage{
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
