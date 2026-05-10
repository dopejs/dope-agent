package im

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type loopLongProvider struct{}

func (p *loopLongProvider) Name() string { return "echo" }

func (p *loopLongProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	output := "reply:" + request.Messages[0].Content
	return llm.ProviderResponse{
		Output:       output,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: len([]rune(output)), TotalTokens: len([]rune(output)) + 1},
	}, nil
}

type loopPartialFailureProvider struct{}

func (p *loopPartialFailureProvider) Name() string { return "echo" }

func (p *loopPartialFailureProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{}, errors.New("not used")
}

func (p *loopPartialFailureProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	if err := emit(llm.StreamChunk{Delta: "reply:"}); err != nil {
		return llm.ProviderResponse{}, err
	}
	if err := emit(llm.StreamChunk{Delta: request.Messages[0].Content}); err != nil {
		return llm.ProviderResponse{}, err
	}
	return llm.ProviderResponse{
			Output: "reply:" + request.Messages[0].Content,
			Usage:  llm.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
		}, &llm.ProviderError{
			Code:      "idle_timeout",
			Message:   "stream stalled",
			Retryable: true,
		}
}

func (p *loopLongProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	response, err := p.Complete(context.Background(), request)
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	runes := []rune(response.Output)
	segments := []string{
		string(runes[:12]),
		string(runes[12:24]),
		string(runes[24:]),
	}
	for _, segment := range segments {
		if err := emit(llm.StreamChunk{Delta: segment}); err != nil {
			return llm.ProviderResponse{}, err
		}
	}
	return response, nil
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
	editErr  error
	maxLen   int
	nextID   int
}

func (s *loopProgressReplySender) ReplyCapabilities() imtypes.ReplyCapabilities {
	maxLen := s.maxLen
	if maxLen <= 0 {
		maxLen = 2000
	}
	return imtypes.ReplyCapabilities{SupportsThinking: true, SupportsStreaming: true, MaxMessageLength: maxLen}
}

func (s *loopProgressReplySender) SendThinking(_ context.Context, signal imtypes.ThinkingSignal) error {
	s.thinking = append(s.thinking, signal)
	return nil
}

func (s *loopProgressReplySender) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	s.sent = append(s.sent, reply)
	s.nextID++
	return imtypes.SentReply{ExternalMessageID: fmt.Sprintf("discord_reply_%d", s.nextID)}, nil
}

func (s *loopProgressReplySender) EditReply(_ context.Context, edit imtypes.ReplyEdit) error {
	if s.editErr != nil {
		return s.editErr
	}
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
	chatService := chat.NewService(dispatcher, providerManager, nil, eventBus, sqliteStore)
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

func TestMessageLoopProcessesMatrixInboundAndPublishesMatrixRouteEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

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
	chatService := chat.NewService(dispatcher, providerManager, nil, eventBus, sqliteStore)
	runtimeManager := runtime.NewManager()
	loop := NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	replySender := &loopTestReplySender{}

	connector := connectors.Connector{
		TenantID:    "ten_matrix",
		ConnectorID: "matrix-main",
		Kind:        "matrix",
		DisplayName: "Matrix Main",
		Status:      connectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	inbound := imtypes.InboundMessage{
		TenantID:                "ten_matrix",
		ConnectorID:             "matrix-main",
		ConnectorKind:           "matrix",
		ExternalMessageID:       "$event_redacted",
		AccountID:               "@bot:example.org",
		ConnectorAccountID:      "matrix.example.org",
		ChannelID:               "!room:example.org",
		PeerID:                  "!room:example.org",
		ThreadID:                "!room:example.org",
		ChannelOrConversationID: "!room:example.org",
		ProviderMessageID:       "$event_redacted",
		EquivalentRuleID:        "matrix_homeserver_conversation_event_id",
		AuthorID:                "@alice:example.org",
		Content:                 "hello matrix",
		Kind:                    router.SessionKindGroup,
		ReplyToMessageID:        "$event_redacted",
		ReceivedAt:              time.Now().UTC(),
	}

	result, err := loop.ProcessSingleTurn(context.Background(), connector, inbound, replySender)
	if err != nil {
		t.Fatalf("ProcessSingleTurn returned error: %v", err)
	}
	if result.Run.Entrypoint != "matrix.message" || result.Session.Channel != "matrix" {
		t.Fatalf("unexpected Matrix run/session: run=%+v session=%+v", result.Run, result.Session)
	}
	if replySender.last.ConnectorID != "matrix-main" || replySender.last.ReplyToExternalMessageID != "$event_redacted" {
		t.Fatalf("unexpected Matrix reply target: %+v", replySender.last)
	}

	persisted, ok, err := sqliteStore.GetConnectorMessageByExternalIDForTenant(context.Background(), "ten_matrix", "matrix-main", imtypes.DeliveryDirectionInbound, "$event_redacted")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID ok=%v err=%v", ok, err)
	}
	if persisted.ConnectorAccountID != "matrix.example.org" || persisted.ChannelOrConversationID != "!room:example.org" || persisted.ProviderMessageID != "$event_redacted" {
		t.Fatalf("Matrix inbound identity was not retained: %+v", persisted)
	}

	var routeEvent events.Event
	var runCreated events.Event
	for _, event := range eventBus.List(events.Filter{}) {
		if event.Name == events.ConnectorEventRouteOutcomeRecorded && event.Scope.ConnectorID == "matrix-main" {
			routeEvent = event
		}
		if event.Name == "run.created" && event.Scope.RunID == result.Run.RunID {
			runCreated = event
		}
	}
	if routeEvent.Name == "" {
		t.Fatalf("expected Matrix route outcome event, got %+v", eventBus.List(events.Filter{}))
	}
	if routeEvent.Payload["homeserverId"] != "matrix.example.org" || routeEvent.Payload["matrixEventId"] != "$event_redacted" || routeEvent.Payload["redactionStatus"] != "redacted" {
		t.Fatalf("unexpected Matrix route outcome payload: %+v", routeEvent.Payload)
	}
	if runCreated.Payload["source"] != "connector.matrix" {
		t.Fatalf("expected Matrix run source, got %+v", runCreated.Payload)
	}

	duplicate, err := loop.ProcessSingleTurn(context.Background(), connector, inbound, replySender)
	if err != nil {
		t.Fatalf("ProcessSingleTurn duplicate returned error: %v", err)
	}
	if !duplicate.Duplicate {
		t.Fatal("expected Matrix duplicate to be suppressed")
	}
	var duplicateRoute events.Event
	for _, event := range eventBus.List(events.Filter{Category: "connector"}) {
		if event.Name == events.ConnectorEventRouteOutcomeRecorded && event.Payload["outcome"] == "duplicate" {
			duplicateRoute = event
		}
	}
	if duplicateRoute.Payload["reasonCode"] != "duplicate_inbound" {
		t.Fatalf("expected duplicate Matrix route evidence, got %+v", duplicateRoute)
	}
}

func TestMessageLoopSlackWorkspaceConversationMessageIdentityDedupesReplay(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	now := time.Date(2026, 5, 8, 12, 30, 0, 0, time.UTC)
	first, created, err := sqliteStore.CreateConnectorMessageIfAbsent(context.Background(), imtypes.MessageRecord{
		DeliveryID:              "slack_delivery_1",
		TenantID:                "ten_slack",
		ConnectorID:             "slack-main",
		Direction:               imtypes.DeliveryDirectionInbound,
		ExternalMessageID:       "message_redacted",
		ConnectorAccountID:      "workspace_redacted",
		ChannelOrConversationID: "channel_redacted",
		ProviderMessageID:       "message_redacted",
		EquivalentRuleID:        "slack_workspace_conversation_message_id",
		Content:                 "hello",
		Status:                  imtypes.DeliveryStatusReceived,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	if err != nil || !created {
		t.Fatalf("first CreateConnectorMessageIfAbsent created=%v err=%v", created, err)
	}
	replay, created, err := sqliteStore.CreateConnectorMessageIfAbsent(context.Background(), imtypes.MessageRecord{
		DeliveryID:              "slack_delivery_2",
		TenantID:                "ten_slack",
		ConnectorID:             "slack-main",
		Direction:               imtypes.DeliveryDirectionInbound,
		ExternalMessageID:       "message_redacted",
		ConnectorAccountID:      "workspace_redacted",
		ChannelOrConversationID: "channel_redacted",
		ProviderMessageID:       "message_redacted",
		EquivalentRuleID:        "slack_workspace_conversation_message_id",
		Content:                 "hello again",
		Status:                  imtypes.DeliveryStatusReceived,
		CreatedAt:               now.Add(time.Second),
		UpdatedAt:               now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("replay CreateConnectorMessageIfAbsent returned error: %v", err)
	}
	if created || replay.DeliveryID != first.DeliveryID {
		t.Fatalf("expected Slack replay to resolve existing delivery, created=%v first=%+v replay=%+v", created, first, replay)
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
	}, dispatcher, nil), nil, eventBus, sqliteStore)

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
	connectorEvents := eventBus.List(events.Filter{Category: "connector"})
	if len(connectorEvents) == 0 {
		t.Fatal("expected connector reply failure event")
	}
	last := connectorEvents[len(connectorEvents)-1]
	if last.Name != "connector.reply_failed" {
		t.Fatalf("expected connector.reply_failed, got %s", last.Name)
	}
	if _, ok := last.Payload["error"]; ok {
		t.Fatalf("reply failure event must not expose raw provider error: %+v", last.Payload)
	}
	if got := last.Payload["reasonCode"]; got != "reply_failed" {
		t.Fatalf("reasonCode=%#v, want reply_failed", got)
	}
	if got := last.Payload["redactionStatus"]; got != "redacted" {
		t.Fatalf("redactionStatus=%#v, want redacted", got)
	}
	inbound, ok, err := sqliteStore.GetConnectorMessageByExternalID(context.Background(), "discord-main", imtypes.DeliveryDirectionInbound, "discord_msg_fail_1")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID inbound ok=%v err=%v", ok, err)
	}
	if strings.Contains(inbound.Error, "discord send failed") {
		t.Fatalf("persisted inbound error exposed raw provider error: %q", inbound.Error)
	}
}

func TestMessageLoopSlackReplyFailureUsesConnectorDeliveryOutcome(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

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
	}, dispatcher, nil), nil, eventBus, sqliteStore)

	runtimeManager := runtime.NewManager()
	loop := NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	replySender := &loopTestReplySender{err: errors.New("slack send failed")}

	_, err = loop.ProcessSingleTurn(context.Background(), connectors.Connector{
		ConnectorID: "slack-main",
		Kind:        "slack",
		DisplayName: "Slack Main",
		Status:      connectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, imtypes.InboundMessage{
		ConnectorID:       "slack-main",
		ConnectorKind:     "slack",
		ExternalMessageID: "slack_msg_fail_1",
		TenantID:          "ten_slack",
		AccountID:         "workspace_redacted",
		ChannelID:         "channel_selected",
		PeerID:            "channel_selected",
		ThreadID:          "slack_thread_root_1",
		AuthorID:          "user_1",
		Content:           "hello",
		Kind:              router.SessionKindGroup,
		ReplyToMessageID:  "slack_thread_root_1",
		ReceivedAt:        time.Now().UTC(),
	}, replySender)
	if err == nil {
		t.Fatal("expected send failure to be returned")
	}
	if replySender.last.ConnectorID != "slack-main" {
		t.Fatalf("expected Slack reply sender to be reached, err=%v got %+v", err, replySender.last)
	}
	connectorEvents := eventBus.List(events.Filter{Category: "connector"})
	var failed events.Event
	for _, event := range connectorEvents {
		if event.Name == "connector.reply_failed" {
			failed = event
		}
	}
	if failed.Name == "" {
		t.Fatalf("expected connector.reply_failed event, got %+v", connectorEvents)
	}
	if _, ok := failed.Payload["discordDeliveryOutcome"]; ok {
		t.Fatalf("Slack reply failure must not use Discord-specific outcome key: %+v", failed.Payload)
	}
	if failed.Payload["connectorDeliveryOutcome"] != "failed" || failed.Payload["connectorKind"] != "slack" {
		t.Fatalf("unexpected Slack reply failure payload: %+v", failed.Payload)
	}
	if replySender.last.ReplyToExternalMessageID != "slack_thread_root_1" {
		t.Fatalf("expected Slack reply to target thread root, got %+v", replySender.last)
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
	}, dispatcher, nil), nil, eventBus, sqliteStore)

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

	connectorEvents := eventBus.List(events.Filter{RunID: result.Run.RunID, Category: "connector"})
	replySentCount := 0
	for _, event := range connectorEvents {
		if event.Name == "connector.reply_sent" {
			replySentCount++
		}
	}
	if replySentCount != 1 {
		t.Fatalf("expected exactly one connector.reply_sent event, got %d", replySentCount)
	}
}

func TestMessageLoopSplitsLongStreamingReplyWithinChannelLimit(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&loopLongProvider{})
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

	runtimeManager := runtime.NewManager()
	loop := NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	replySender := &loopProgressReplySender{maxLen: 10}

	longPrompt := strings.Repeat("abcdefghij", 4)
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
		ExternalMessageID: "discord_msg_stream_long_1",
		AccountID:         "bot_1",
		ChannelID:         "dm_1",
		PeerID:            "user_1",
		AuthorID:          "user_1",
		Content:           longPrompt,
		Kind:              router.SessionKindDirect,
		Direct:            true,
		ReceivedAt:        time.Now().UTC(),
	}, replySender)
	if err != nil {
		t.Fatalf("ProcessSingleTurn returned error: %v", err)
	}
	if result.Run.Status != runtime.RunStatusCompleted {
		t.Fatalf("expected completed run, got %s", result.Run.Status)
	}
	if len(replySender.sent) < 2 {
		t.Fatalf("expected multipart send for long reply, got %d sends", len(replySender.sent))
	}
	for index, sent := range replySender.sent {
		if len([]rune(sent.Content)) > 10 {
			t.Fatalf("expected chunk %d to respect max length, got %d runes", index, len([]rune(sent.Content)))
		}
	}
}

func TestMessageLoopDoesNotMarkLLMDispatchFailedWhenConnectorStreamingFails(t *testing.T) {
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
	}, dispatcher, nil), nil, eventBus, sqliteStore)

	runtimeManager := runtime.NewManager()
	loop := NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	replySender := &loopProgressReplySender{editErr: errors.New("transport edit failed")}

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
		ExternalMessageID: "discord_msg_stream_fail_1",
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
		t.Fatal("expected connector streaming failure to be returned")
	}

	llmEvents := eventBus.List(events.Filter{RunID: result.Run.RunID, Category: "llm"})
	if len(llmEvents) == 0 {
		t.Fatal("expected llm events to be recorded")
	}
	lastLLMEvent := llmEvents[len(llmEvents)-1]
	if lastLLMEvent.Name != "llm.dispatch.completed" {
		t.Fatalf("expected llm dispatch to complete despite connector failure, got %s", lastLLMEvent.Name)
	}
}

func TestMessageLoopPreservesPartialReplyWhenProviderStreamFailsAfterVisibleOutput(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&loopPartialFailureProvider{})
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
		ExternalMessageID: "discord_msg_stream_partial_1",
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
		t.Fatal("expected provider partial failure to be returned")
	}
	if result.Run.Status != runtime.RunStatusFailed {
		t.Fatalf("expected failed run after partial provider failure, got %s", result.Run.Status)
	}
	if len(replySender.edited) == 0 {
		t.Fatal("expected visible streamed edits before failure")
	}
	lastReply := replySender.edited[len(replySender.edited)-1].Content
	if !strings.Contains(lastReply, "[response interrupted]") {
		t.Fatalf("expected partial reply marker, got %q", lastReply)
	}

	connectorEvents := eventBus.List(events.Filter{RunID: result.Run.RunID, Category: "connector"})
	partialCount := 0
	failedCount := 0
	for _, event := range connectorEvents {
		switch event.Name {
		case "connector.reply_partial":
			partialCount++
		case "connector.reply_failed":
			failedCount++
		}
	}
	if partialCount != 1 {
		t.Fatalf("expected exactly one connector.reply_partial event, got %d", partialCount)
	}
	if failedCount != 0 {
		t.Fatalf("expected no connector.reply_failed event for partial provider failure, got %d", failedCount)
	}

	dispatches, err := sqliteStore.ListLLMDispatches(context.Background())
	if err != nil {
		t.Fatalf("ListLLMDispatches returned error: %v", err)
	}
	if len(dispatches) == 0 {
		t.Fatal("expected persisted llm dispatch")
	}
	dispatch := dispatches[len(dispatches)-1]
	if dispatch.Status != llm.DispatchStatusPartialFailed {
		t.Fatalf("expected partial_failed dispatch, got %s", dispatch.Status)
	}
	if !dispatch.Partial {
		t.Fatal("expected persisted dispatch partial flag to be true")
	}

	record, ok, err := sqliteStore.GetConnectorMessageByExternalID(context.Background(), "discord-main", imtypes.DeliveryDirectionOutbound, "discord_reply_1")
	if err != nil {
		t.Fatalf("GetConnectorMessageByExternalID returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected outbound connector record to exist")
	}
	if record.Status != imtypes.DeliveryStatusPartial {
		t.Fatalf("expected outbound connector record to be partial, got %s", record.Status)
	}
}
