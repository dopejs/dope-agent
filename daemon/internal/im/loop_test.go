package im

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type loopTestProvider struct{}

func (p *loopTestProvider) Name() string { return "echo" }

func (p *loopTestProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       "reply:" + request.Messages[0].Content,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *loopTestProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	response, err := p.Complete(context.Background(), request)
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	if err := emit(llm.StreamChunk{Delta: response.Output}); err != nil {
		return llm.ProviderResponse{}, err
	}
	return response, nil
}

type loopChunkedProvider struct{}

func (p *loopChunkedProvider) Name() string { return "echo" }

func (p *loopChunkedProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       "reply:" + request.Messages[0].Content,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
	}, nil
}

func (p *loopChunkedProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	if err := emit(llm.StreamChunk{Delta: "reply:"}); err != nil {
		return llm.ProviderResponse{}, err
	}
	if err := emit(llm.StreamChunk{Delta: request.Messages[0].Content}); err != nil {
		return llm.ProviderResponse{}, err
	}
	return p.Complete(context.Background(), request)
}

type loopTestReplySender struct {
	last imtypes.OutboundReply
	err  error
}

func (s *loopTestReplySender) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	s.last = reply
	if s.err != nil {
		return imtypes.SentReply{}, s.err
	}
	return imtypes.SentReply{ExternalMessageID: "discord_reply_1"}, nil
}

type loopProgressReplySender struct {
	sent     []imtypes.OutboundReply
	edited   []imtypes.ReplyEdit
	thinking []imtypes.ThinkingSignal
}

func (s *loopProgressReplySender) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{SupportsThinking: true, SupportsStreaming: true}
}

func (s *loopProgressReplySender) SendThinking(_ context.Context, signal imtypes.ThinkingSignal) error {
	s.thinking = append(s.thinking, signal)
	return nil
}

func (s *loopProgressReplySender) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	s.sent = append(s.sent, reply)
	return imtypes.SentReply{ExternalMessageID: "discord_reply_1"}, nil
}

func (s *loopProgressReplySender) EditReply(_ context.Context, edit imtypes.ReplyEdit) error {
	s.edited = append(s.edited, edit)
	return nil
}

func TestMessageLoopProcessesSingleTurnAndDeduplicates(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&loopTestProvider{})
	if err := dispatcher.SetDefaultProvider("echo"); err != nil {
		t.Fatalf("SetDefaultProvider returned error: %v", err)
	}
	dispatcher.SetDefaultModel("echo-v1")

	providerManager := providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
			DefaultModel:    "echo-v1",
		},
	}, dispatcher, nil)
	chatService := chat.NewService(dispatcher, providerManager, eventBus, sqliteStore)
	sessionRouter := router.NewSessionRouter()
	runtimeManager := runtime.NewManager()
	checkpointManager := checkpoints.NewManager(sqliteStore, runtimeManager)
	loop := NewMessageLoop(sessionRouter, runtimeManager, checkpointManager, eventBus, sqliteStore, chatService)
	replySender := &loopTestReplySender{}

	inbound := imtypes.InboundMessage{
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
	}

	result, err := loop.ProcessSingleTurn(context.Background(), connectors.Connector{
		ConnectorID: "discord-main",
		Kind:        "discord",
		DisplayName: "Discord Main",
		Status:      connectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, inbound, replySender)
	if err != nil {
		t.Fatalf("ProcessSingleTurn returned error: %v", err)
	}
	if result.Duplicate {
		t.Fatal("expected first inbound message to be processed, not deduplicated")
	}
	if result.Run.Status != runtime.RunStatusCompleted {
		t.Fatalf("expected completed run, got %s", result.Run.Status)
	}
	if result.Step.Status != runtime.StepStatusCompleted {
		t.Fatalf("expected completed step, got %s", result.Step.Status)
	}
	if replySender.last.Content != "reply:hello" {
		t.Fatalf("expected outbound reply content, got %q", replySender.last.Content)
	}

	secondResult, err := loop.ProcessSingleTurn(context.Background(), connectors.Connector{
		ConnectorID: "discord-main",
		Kind:        "discord",
		DisplayName: "Discord Main",
		Status:      connectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, inbound, replySender)
	if err != nil {
		t.Fatalf("ProcessSingleTurn(duplicate) returned error: %v", err)
	}
	if !secondResult.Duplicate {
		t.Fatal("expected duplicate inbound message to be ignored")
	}
}

func TestMessageLoopMarksFailureWhenReplySendFails(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&loopTestProvider{})
	if err := dispatcher.SetDefaultProvider("echo"); err != nil {
		t.Fatalf("SetDefaultProvider returned error: %v", err)
	}
	dispatcher.SetDefaultModel("echo-v1")
	chatService := chat.NewService(dispatcher, providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
			DefaultModel:    "echo-v1",
		},
	}, dispatcher, nil), eventBus, sqliteStore)

	runtimeManager := runtime.NewManager()
	loop := NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	replySender := &loopTestReplySender{err: errors.New("discord send failed")}

	_, err = loop.ProcessSingleTurn(context.Background(), connectors.Connector{
		ConnectorID: "discord-main",
		Kind:        "discord",
		DisplayName: "Discord Main",
		Status:      connectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, imtypes.InboundMessage{
		ConnectorID:       "discord-main",
		ConnectorKind:     "discord",
		ExternalMessageID: "discord_msg_fail_1",
		AccountID:         "bot_1",
		ChannelID:         "dm_1",
		PeerID:            "user_1",
		AuthorID:          "user_1",
		Content:           "hello",
		Kind:              router.SessionKindDirect,
		Direct:            true,
		ReceivedAt:        time.Now().UTC(),
	}, replySender)
	if err == nil {
		t.Fatal("expected send failure to be returned")
	}
}

func TestMessageLoopStreamsReplyWhenProgressionIsSupported(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&loopChunkedProvider{})
	if err := dispatcher.SetDefaultProvider("echo"); err != nil {
		t.Fatalf("SetDefaultProvider returned error: %v", err)
	}
	dispatcher.SetDefaultModel("echo-v1")

	chatService := chat.NewService(dispatcher, providers.NewManager(config.Config{
		LLM: config.LLMConfig{
			DefaultProvider: "echo",
			DefaultModel:    "echo-v1",
		},
	}, dispatcher, nil), eventBus, sqliteStore)

	runtimeManager := runtime.NewManager()
	loop := NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	replySender := &loopProgressReplySender{}

	result, err := loop.ProcessSingleTurn(context.Background(), connectors.Connector{
		ConnectorID: "discord-main",
		Kind:        "discord",
		DisplayName: "Discord Main",
		Status:      connectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, imtypes.InboundMessage{
		ConnectorID:       "discord-main",
		ConnectorKind:     "discord",
		ExternalMessageID: "discord_msg_stream_1",
		AccountID:         "bot_1",
		ChannelID:         "dm_1",
		PeerID:            "user_1",
		AuthorID:          "user_1",
		Content:           "hello",
		Kind:              router.SessionKindDirect,
		Direct:            true,
		ReceivedAt:        time.Now().UTC(),
	}, replySender)
	if err != nil {
		t.Fatalf("ProcessSingleTurn returned error: %v", err)
	}
	if result.Reply != "reply:hello" {
		t.Fatalf("expected streamed reply, got %q", result.Reply)
	}
	if len(replySender.thinking) == 0 {
		t.Fatal("expected thinking signal to be sent")
	}
	if len(replySender.sent) != 1 {
		t.Fatalf("expected one initial streamed reply message, got %d", len(replySender.sent))
	}
	if len(replySender.edited) == 0 {
		t.Fatal("expected at least one streamed edit")
	}
	if replySender.edited[len(replySender.edited)-1].Content != "reply:hello" {
		t.Fatalf("expected final streamed content reply:hello, got %q", replySender.edited[len(replySender.edited)-1].Content)
	}
}
