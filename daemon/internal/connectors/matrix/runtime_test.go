package matrix

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/chat"
	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type matrixRuntimeTestProvider struct{}

func (p *matrixRuntimeTestProvider) Name() string { return "echo" }

func (p *matrixRuntimeTestProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       "reply:" + request.Messages[0].Content,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *matrixRuntimeTestProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	response, err := p.Complete(context.Background(), request)
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	if err := emit(llm.StreamChunk{Delta: response.Output}); err != nil {
		return llm.ProviderResponse{}, err
	}
	return response, nil
}

func TestNormalizeInboundEventTrimsIdentityAndAppliesDefaults(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	event := NormalizeInboundEvent(InboundEvent{
		TenantID:         " ten_matrix ",
		ConnectorID:      " matrix-main ",
		HomeserverID:     " example.org ",
		ConversationID:   " !room:example.org ",
		MatrixEventID:    " $event ",
		SenderID:         " @alice:example.org ",
		ConversationType: ConversationRoom,
		Text:             "  @bot:example.org hello  ",
		ReceivedAt:       now,
	})
	if event.TenantID != "ten_matrix" || event.ConnectorID != "matrix-main" || event.HomeserverID != "example.org" {
		t.Fatalf("expected trimmed event identity, got %+v", event)
	}
	if event.MessageKind != MessageUnencryptedText {
		t.Fatalf("expected default unencrypted text kind, got %s", event.MessageKind)
	}
	if event.Text != "@bot:example.org hello" {
		t.Fatalf("expected trimmed text, got %q", event.Text)
	}
}

func TestMatrixRollbackDisabledConnectorBlocksDeliveryEligibility(t *testing.T) {
	t.Parallel()

	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:           "ten_matrix",
		ConnectorID:        "matrix-main",
		DisplayName:        "Matrix Main",
		Cancelled:          true,
		ProviderAvailable:  true,
		NetworkAvailable:   true,
		ConformancePassed:  true,
		BotCredentialState: BotCredentialValid,
	})
	if setup.TerminalState != TerminalCancelled || setup.DeliveryEligible {
		t.Fatalf("disabled rollback should cancel and block delivery, got %+v", setup)
	}
}

func TestRuntimeRoutesAcceptedMatrixEventThroughMessageLoop(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&matrixRuntimeTestProvider{})
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
	messageLoop := im.NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	supervisor := baseconnectors.NewSupervisor()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveMatrixHostedSetup(context.Background(), store.MatrixHostedSetupRecord{
		TenantID:            "ten_matrix_runtime",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "healthy",
		TerminalState:       "ready",
		BotCredentialState:  "valid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "valid",
		DeliveryEligible:    true,
		HomeserverBindingID: "matrix_homeserver_matrix-main",
		ReasonCode:          "healthy",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now,
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveMatrixHostedSetup returned error: %v", err)
	}

	transport := NewFakeTransport(InboundEvent{
		HomeserverID:     "matrix.example.org",
		ConversationID:   "!room:example.org",
		MatrixEventID:    "$event1",
		SenderID:         "@alice:example.org",
		ConversationType: ConversationRoom,
		MessageKind:      MessageUnencryptedText,
		Text:             "@bot:example.org hello matrix",
		ReceivedAt:       now,
	})
	runtime, err := NewRuntime(Config{
		Enabled:            true,
		ConnectorID:        "matrix-main",
		DisplayName:        "Matrix Main",
		HomeserverID:       "matrix.example.org",
		BotUserID:          "@bot:example.org",
		SelectedRoomIDs:    []string{"!room:example.org"},
		ConfiguredCommands: []string{"!dope"},
	}, nil, supervisor, messageLoop, sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_matrix_runtime"})
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	replies := transport.SentReplies()
	if len(replies) != 1 || replies[0].Content != "reply:hello matrix" || replies[0].ReplyToExternalMessageID != "$event1" {
		t.Fatalf("unexpected Matrix runtime replies: %+v", replies)
	}
	runs, err := sqliteStore.ListRunsAllTenantsForTest(context.Background())
	if err != nil {
		t.Fatalf("ListRunsAllTenantsForTest returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Entrypoint != "matrix.message" {
		t.Fatalf("expected one Matrix run, got %+v", runs)
	}
	evidence, err := sqliteStore.ListMatrixEventEvidence(context.Background(), "ten_matrix_runtime", "matrix-main", now, 10)
	if err != nil {
		t.Fatalf("ListMatrixEventEvidence returned error: %v", err)
	}
	if len(evidence) != 1 || evidence[0].RouteOutcome != "accepted" || evidence[0].MatrixEventID != "$event1" {
		t.Fatalf("expected accepted Matrix event evidence, got %+v", evidence)
	}
}

func TestRuntimeClassifiesPersistedMatrixEventReplayAsDuplicateAfterRestart(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&matrixRuntimeTestProvider{})
	if err := dispatcher.SetDefaultProvider("echo"); err != nil {
		t.Fatalf("SetDefaultProvider returned error: %v", err)
	}
	dispatcher.SetDefaultModel("echo-v1")
	providerManager := providers.NewManager(config.Config{LLM: config.LLMConfig{DefaultProvider: "echo", DefaultModel: "echo-v1"}}, dispatcher, nil)
	chatService := chat.NewService(dispatcher, providerManager, nil, eventBus, sqliteStore)
	runtimeManager := runtime.NewManager()
	messageLoop := im.NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveMatrixHostedSetup(context.Background(), store.MatrixHostedSetupRecord{
		TenantID:            "ten_matrix_runtime",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "healthy",
		TerminalState:       "ready",
		BotCredentialState:  "valid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "valid",
		DeliveryEligible:    true,
		HomeserverBindingID: "matrix_homeserver_matrix-main",
		ReasonCode:          "healthy",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now,
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveMatrixHostedSetup returned error: %v", err)
	}
	event := InboundEvent{
		HomeserverID:     "matrix.example.org",
		ConversationID:   "!room:example.org",
		MatrixEventID:    "$event1",
		SenderID:         "@alice:example.org",
		ConversationType: ConversationRoom,
		MessageKind:      MessageUnencryptedText,
		Text:             "@bot:example.org hello matrix",
		ReceivedAt:       now,
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_matrix_runtime"})
	firstTransport := NewFakeTransport(event)
	firstRuntime, err := NewRuntime(Config{
		Enabled:         true,
		ConnectorID:     "matrix-main",
		DisplayName:     "Matrix Main",
		HomeserverID:    "matrix.example.org",
		BotUserID:       "@bot:example.org",
		SelectedRoomIDs: []string{"!room:example.org"},
	}, nil, baseconnectors.NewSupervisor(), messageLoop, sqliteStore, eventBus, firstTransport)
	if err != nil {
		t.Fatalf("NewRuntime first returned error: %v", err)
	}
	if err := firstRuntime.Start(ctx); err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}
	secondTransport := NewFakeTransport(event)
	secondRuntime, err := NewRuntime(Config{
		Enabled:         true,
		ConnectorID:     "matrix-main",
		DisplayName:     "Matrix Main",
		HomeserverID:    "matrix.example.org",
		BotUserID:       "@bot:example.org",
		SelectedRoomIDs: []string{"!room:example.org"},
	}, nil, baseconnectors.NewSupervisor(), messageLoop, sqliteStore, eventBus, secondTransport)
	if err != nil {
		t.Fatalf("NewRuntime second returned error: %v", err)
	}
	if err := secondRuntime.Start(ctx); err != nil {
		t.Fatalf("second Start returned error: %v", err)
	}
	if len(firstTransport.SentReplies()) != 1 || len(secondTransport.SentReplies()) != 0 {
		t.Fatalf("expected replay after restart to suppress reply, first=%+v second=%+v", firstTransport.SentReplies(), secondTransport.SentReplies())
	}
	evidence, err := sqliteStore.ListMatrixEventEvidence(context.Background(), "ten_matrix_runtime", "matrix-main", now, 10)
	if err != nil {
		t.Fatalf("ListMatrixEventEvidence returned error: %v", err)
	}
	if len(evidence) != 1 || evidence[0].RouteOutcome != "duplicate" {
		t.Fatalf("expected persisted replay evidence to be duplicate, got %+v", evidence)
	}
}
