package telegram

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
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type telegramTestProvider struct{}

func (p *telegramTestProvider) Name() string { return "echo" }

func (p *telegramTestProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       "reply:" + request.Messages[0].Content,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *telegramTestProvider) Stream(ctx context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	response, err := p.Complete(ctx, request)
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	if err := emit(llm.StreamChunk{Delta: response.Output}); err != nil {
		return llm.ProviderResponse{}, err
	}
	return response, nil
}

func TestRuntimeNormalizesTelegramIdentityAndBlocksUnallowedRoutes(t *testing.T) {
	t.Parallel()

	transport := NewFakeTransport()
	runtime, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		Allowments:  []AllowmentValidation{{ScopeType: ScopeDirectChat, ScopeID: "chat_1", Enabled: true, ValidationState: AllowmentValid}},
	}, nil, baseconnectors.NewSupervisor(), nil, nil, nil, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	inbound, ok := runtime.NormalizeInbound(context.Background(), InboundUpdate{
		UpdateID:         "update_1",
		MessageID:        "message_1",
		ChatID:           "chat_1",
		SenderID:         "user_1",
		Text:             "hello",
		ConversationType: ConversationDirect,
		ReceivedAt:       time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
	})
	if !ok {
		t.Fatalf("expected inbound to normalize")
	}
	if inbound.ConnectorKind != "telegram" || inbound.EquivalentRuleID != "telegram_chat_message_id" || inbound.ProviderMessageID != "message_1" {
		t.Fatalf("unexpected identity: %+v", inbound)
	}

	_, ok = runtime.NormalizeInbound(context.Background(), InboundUpdate{
		UpdateID:         "update_2",
		MessageID:        "message_2",
		ChatID:           "chat_2",
		SenderID:         "user_2",
		Text:             "hello",
		ConversationType: ConversationDirect,
	})
	if ok {
		t.Fatalf("unallowed sender/chat should not normalize into accepted inbound")
	}
	if transport.LastRouteOutcome().Outcome != RouteBlocked {
		t.Fatalf("expected blocked route evidence, got %+v", transport.LastRouteOutcome())
	}
}

func TestFakeTransportSendsFinalOnlyReplies(t *testing.T) {
	t.Parallel()

	transport := NewFakeTransport()
	sent, err := transport.SendReply(context.Background(), imtypes.OutboundReply{ConnectorID: "telegram-main", ChannelID: "chat_1", Content: "final"})
	if err != nil {
		t.Fatalf("SendReply returned error: %v", err)
	}
	if sent.ExternalMessageID == "" || transport.ReplyCapabilities().SupportsStreaming {
		t.Fatalf("expected final-only reply capability and external id, got sent=%+v caps=%+v", sent, transport.ReplyCapabilities())
	}
}

func TestRuntimeEnforcesTelegramGroupMentionAndCommandGate(t *testing.T) {
	t.Parallel()

	transport := NewFakeTransport()
	runtime, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		BotUsername: "dope_test_bot",
		Allowments: []AllowmentValidation{{
			ScopeType:       ScopeGroup,
			ScopeID:         "group_1",
			Enabled:         true,
			GroupGate:       GroupGateMentionOrCommandRequired,
			ValidationState: AllowmentValid,
		}},
	}, nil, baseconnectors.NewSupervisor(), nil, nil, nil, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	if _, ok := runtime.NormalizeInbound(context.Background(), InboundUpdate{
		UpdateID:         "update_group_ignored",
		MessageID:        "message_group_ignored",
		ChatID:           "group_1",
		SenderID:         "user_1",
		Text:             "hello group",
		ConversationType: ConversationGroup,
	}); ok {
		t.Fatalf("allowed group without mention or command should be ignored")
	}
	if got := transport.LastRouteOutcome(); got.Outcome != RouteIgnored || got.ReasonCode != "mention_required" {
		t.Fatalf("expected mention-required route outcome, got %+v", got)
	}

	mentioned, ok := runtime.NormalizeInbound(context.Background(), InboundUpdate{
		UpdateID:         "update_group_mentioned",
		MessageID:        "message_group_mentioned",
		ChatID:           "group_1",
		SenderID:         "user_1",
		Text:             "@dope_test_bot summarize this",
		ConversationType: ConversationGroup,
	})
	if !ok || !mentioned.Mentioned || mentioned.Content != "summarize this" {
		t.Fatalf("expected mentioned group message to normalize, got ok=%v inbound=%+v", ok, mentioned)
	}

	command, ok := runtime.NormalizeInbound(context.Background(), InboundUpdate{
		UpdateID:         "update_group_command",
		MessageID:        "message_group_command",
		ChatID:           "group_1",
		SenderID:         "user_1",
		Text:             "/dope summarize this",
		ConversationType: ConversationGroup,
	})
	if !ok || !command.Mentioned || command.Content != "/dope summarize this" {
		t.Fatalf("expected command group message to normalize, got ok=%v inbound=%+v", ok, command)
	}
}

func TestRuntimeRejectsUnsupportedTelegramSurfaces(t *testing.T) {
	t.Parallel()

	transport := NewFakeTransport()
	runtime, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		Allowments:  []AllowmentValidation{{ScopeType: ScopeDirectChat, ScopeID: "chat_1", Enabled: true, ValidationState: AllowmentValid}},
	}, nil, baseconnectors.NewSupervisor(), nil, nil, nil, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	for _, surface := range []string{"attachment", "media_transfer", "voice", "payment", "mini_app"} {
		_, ok := runtime.NormalizeInbound(context.Background(), InboundUpdate{
			UpdateID:           "update_" + surface,
			MessageID:          "message_" + surface,
			ChatID:             "chat_1",
			SenderID:           "user_1",
			Text:               "unsupported",
			ConversationType:   ConversationDirect,
			UnsupportedSurface: surface,
		})
		if ok {
			t.Fatalf("%s should not normalize into accepted inbound", surface)
		}
		if got := transport.LastRouteOutcome(); got.Outcome != RouteUnsupported || got.Surface != surface {
			t.Fatalf("expected unsupported outcome for %s, got %+v", surface, got)
		}
	}
}

func TestRuntimePersistsRedactedTelegramUpdateEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	runtime, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		Allowments:  []AllowmentValidation{{ScopeType: ScopeDirectChat, ScopeID: "chat_1", Enabled: true, ValidationState: AllowmentValid}},
	}, nil, baseconnectors.NewSupervisor(), nil, sqliteStore, nil, NewFakeTransport())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_telegram"})
	_, ok := runtime.NormalizeInbound(ctx, InboundUpdate{
		UpdateID:         "update_1",
		MessageID:        "message_1",
		ChatID:           "chat_1",
		SenderID:         "user_1",
		Text:             "hello",
		ConversationType: ConversationDirect,
		ReceivedAt:       time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
	})
	if !ok {
		t.Fatalf("expected inbound to normalize")
	}
	evidence, err := sqliteStore.ListTelegramUpdateEvidence(ctx, "ten_telegram", "telegram-main", time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC), 10)
	if err != nil || len(evidence) != 1 {
		t.Fatalf("ListTelegramUpdateEvidence len=%d err=%v", len(evidence), err)
	}
	if evidence[0].RouteOutcome != "accepted" || evidence[0].SafeEvidence["identityRule"] != "telegram_chat_message_id" {
		t.Fatalf("unexpected Telegram update evidence: %+v", evidence[0])
	}
}

func TestRuntimeRecordsTelegramSetupValidationEventAndStoreProjection(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	runtime, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
	}, nil, baseconnectors.NewSupervisor(), nil, sqliteStore, eventBus, NewFakeTransport())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	now := time.Date(2026, 5, 8, 10, 1, 0, 0, time.UTC)
	setup, err := runtime.RecordHostedSetupValidation(context.Background(), HostedSetupInput{
		TenantID:       "ten_telegram",
		Credential:     CredentialValid,
		AccountBinding: AccountBinding{ConnectorAccountID: "bot_redacted", ProviderAccountLabel: "telegram:bot_redacted", PermissionState: PermissionValid},
		Allowments:     []AllowmentValidation{{AllowmentID: "allow_dm", ScopeType: ScopeDirectChat, ScopeID: "chat_redacted", Enabled: true, ValidationState: AllowmentValid}},
		StartedAt:      now,
		ValidatedAt:    now,
	})
	if err != nil {
		t.Fatalf("RecordHostedSetupValidation returned error: %v", err)
	}
	if setup.TerminalState != TerminalReady {
		t.Fatalf("expected ready setup, got %+v", setup)
	}
	stored, ok, err := sqliteStore.GetTelegramHostedSetup(context.Background(), "ten_telegram", "telegram-main")
	if err != nil || !ok || stored.TerminalState != "ready" || len(stored.Allowments) != 1 {
		t.Fatalf("stored setup ok=%v err=%v record=%+v", ok, err, stored)
	}
	if stored.AccountBinding == nil || stored.AccountBinding.ConnectorAccountID != "bot_redacted" || stored.AccountBinding.ProviderAccountHint != "telegram:bot_redacted" {
		t.Fatalf("expected account binding to be retained, got %+v", stored.AccountBinding)
	}
	published := eventBus.List(events.Filter{Category: "connector"})
	if len(published) != 1 || published[0].Name != "connector.telegram_setup_validated" {
		t.Fatalf("expected Telegram setup validation event, got %+v", published)
	}
	if published[0].Payload["redactionStatus"] != "redacted" || published[0].Payload["credentialState"] != "valid" {
		t.Fatalf("unexpected event payload: %+v", published[0].Payload)
	}
}

func TestRuntimeUpdatesTelegramEvidenceWhenMessageLoopDetectsDuplicate(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&telegramTestProvider{})
	if err := dispatcher.SetDefaultProvider("echo"); err != nil {
		t.Fatalf("SetDefaultProvider returned error: %v", err)
	}
	dispatcher.SetDefaultModel("echo-v1")
	providerManager := providers.NewManager(config.Config{
		LLM: config.LLMConfig{DefaultProvider: "echo", DefaultModel: "echo-v1"},
	}, dispatcher, nil)
	chatService := chat.NewService(dispatcher, providerManager, nil, eventBus, sqliteStore)
	runtimeManager := runtime.NewManager()
	loop := im.NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
	transport := NewFakeTransport()
	connectorRuntime, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "telegram-main",
		DisplayName: "Telegram Main",
		Allowments:  []AllowmentValidation{{ScopeType: ScopeDirectChat, ScopeID: "chat_1", Enabled: true, ValidationState: AllowmentValid}},
	}, nil, baseconnectors.NewSupervisor(), loop, sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_telegram"})
	update := InboundUpdate{
		UpdateID:         "update_1",
		MessageID:        "message_1",
		ChatID:           "chat_1",
		SenderID:         "user_1",
		Text:             "hello",
		ConversationType: ConversationDirect,
		ReceivedAt:       time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
	}

	connectorRuntime.handleUpdate(ctx, update)
	connectorRuntime.handleUpdate(ctx, update)

	evidence, err := sqliteStore.ListTelegramUpdateEvidence(ctx, "ten_telegram", "telegram-main", time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC), 10)
	if err != nil || len(evidence) != 1 {
		t.Fatalf("ListTelegramUpdateEvidence len=%d err=%v", len(evidence), err)
	}
	if evidence[0].RouteOutcome != "duplicate" || evidence[0].ReasonCode != "duplicate_inbound" {
		t.Fatalf("expected duplicate evidence, got %+v", evidence[0])
	}
}

func TestTelegramConformanceProfileDeclaresExplicitSurfaces(t *testing.T) {
	t.Parallel()

	profile := ConformanceProfile(Config{ConnectorID: "telegram-main"}, time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC))
	if profile.ConnectorKind != "telegram" || profile.EquivalentDurableIdentityRuleID != "telegram_chat_message_id" {
		t.Fatalf("unexpected Telegram conformance profile identity: %+v", profile)
	}
	for _, surface := range []string{"direct_message", "group_message", "mention_gating", "command_gating", "final_only_foreground_reply", "connector_backed_delivery"} {
		if profile.ProviderSurfaceResults[surface] != baseconnectors.SurfaceSupported {
			t.Fatalf("expected %s to be supported, got %+v", surface, profile.ProviderSurfaceResults[surface])
		}
	}
	for _, surface := range []string{"attachments", "voice", "payments", "mini_apps", "media_transfer", "thinking_visibility", "incremental_visible_updates"} {
		if profile.ProviderSurfaceResults[surface] != baseconnectors.SurfaceUnsupported {
			t.Fatalf("expected %s to be unsupported, got %+v", surface, profile.ProviderSurfaceResults[surface])
		}
	}
}
