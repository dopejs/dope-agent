package slack

import (
	"context"
	"errors"
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
	daemonruntime "github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type slackRuntimeTestProvider struct{}

func (p *slackRuntimeTestProvider) Name() string { return "echo" }

func (p *slackRuntimeTestProvider) Complete(_ context.Context, request llm.ProviderRequest) (llm.ProviderResponse, error) {
	return llm.ProviderResponse{
		Output:       "reply:" + request.Messages[0].Content,
		FinishReason: "stop",
		Usage:        llm.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
	}, nil
}

func (p *slackRuntimeTestProvider) Stream(_ context.Context, request llm.ProviderRequest, emit llm.StreamEmitter) (llm.ProviderResponse, error) {
	response, err := p.Complete(context.Background(), request)
	if err != nil {
		return llm.ProviderResponse{}, err
	}
	if err := emit(llm.StreamChunk{Delta: response.Output}); err != nil {
		return llm.ProviderResponse{}, err
	}
	return response, nil
}

func newSlackRuntimeTestLoop(t *testing.T, sqliteStore *store.SQLiteStore, eventBus *events.Bus) *im.MessageLoop {
	t.Helper()

	dispatcher := llm.NewDispatcher()
	dispatcher.RegisterProvider(&slackRuntimeTestProvider{})
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
	runtimeManager := daemonruntime.NewManager()
	return im.NewMessageLoop(router.NewSessionRouter(), runtimeManager, checkpoints.NewManager(sqliteStore, runtimeManager), eventBus, sqliteStore, chatService)
}

func seedReadySlackSetup(t *testing.T, sqliteStore *store.SQLiteStore, tenantID string, cfg Config) {
	t.Helper()
	now := time.Date(2026, 5, 8, 11, 0, 0, 0, time.UTC)
	workspaceBindingID := firstNonEmpty(cfg.WorkspaceBindingID, "workspace_binding_redacted")
	policy := store.SlackRoutePolicyRecord{
		TenantID:            tenantID,
		ConnectorID:         cfg.ConnectorID,
		WorkspaceBindingID:  workspaceBindingID,
		AllowedDMUsers:      append([]string(nil), cfg.AllowedDMUserIDs...),
		AllowedDMUserGroups: append([]string(nil), cfg.AllowedDMUserGroups...),
		MentionGate:         "agent_mention_required",
		ThreadReplyMode:     "channel_mentions_thread_rooted",
		ValidationState:     string(RoutePolicyValid),
		ValidatedAt:         now,
		RedactionStatus:     string(baseconnectors.RedactionStatusRedacted),
	}
	for _, channelID := range cfg.AllowedChannelIDs {
		policy.SelectedChannels = append(policy.SelectedChannels, store.SlackConversationRouteRecord{
			ConversationID:       channelID,
			ConversationType:     string(ConversationChannel),
			SelectedChannelState: string(SelectedChannelSelected),
			ValidationState:      string(RoutePolicyValid),
			RedactionStatus:      string(baseconnectors.RedactionStatusRedacted),
		})
	}
	if err := sqliteStore.SaveSlackHostedSetup(context.Background(), store.SlackHostedSetupRecord{
		TenantID:           tenantID,
		ConnectorID:        cfg.ConnectorID,
		ConnectorKind:      "slack",
		DisplayName:        cfg.DisplayName,
		Status:             string(baseconnectors.LifecycleStateHealthy),
		TerminalState:      string(TerminalReady),
		OAuthState:         string(OAuthGrantValid),
		RoutePolicyState:   string(RoutePolicyStateValid),
		DeliveryEligible:   true,
		WorkspaceBindingID: workspaceBindingID,
		ReasonCode:         "healthy",
		RedactionStatus:    string(baseconnectors.RedactionStatusRedacted),
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		WorkspaceBinding: &store.SlackWorkspaceBinding{
			TenantID:           tenantID,
			ConnectorID:        cfg.ConnectorID,
			WorkspaceBindingID: workspaceBindingID,
			WorkspaceID:        cfg.WorkspaceID,
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
			ValidatedAt:        now,
			RedactionStatus:    string(baseconnectors.RedactionStatusRedacted),
		},
		RoutePolicy: &policy,
	}); err != nil {
		t.Fatalf("SaveSlackHostedSetup returned error: %v", err)
	}
}

func TestRuntimeRecordsSlackSetupValidationEventAndStoreProjection(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	runtime, err := NewRuntime(Config{
		Enabled:     true,
		ConnectorID: "slack-main",
		DisplayName: "Slack Main",
	}, nil, baseconnectors.NewSupervisor(), im.NewMessageLoop(nil, nil, nil, nil, nil, nil), sqliteStore, eventBus, NewFakeTransport())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	now := time.Date(2026, 5, 8, 10, 1, 0, 0, time.UTC)
	setup, err := runtime.RecordHostedSetupValidation(context.Background(), HostedSetupInput{
		TenantID:          "ten_slack",
		OAuthState:        OAuthGrantValid,
		ProviderAvailable: true,
		NetworkAvailable:  true,
		StartedAt:         now,
		ValidatedAt:       now,
		WorkspaceBinding: WorkspaceBinding{
			WorkspaceID:        "workspace_redacted",
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
		},
		RoutePolicy: RoutePolicy{
			ValidationState: RoutePolicyValid,
			SelectedChannels: []ConversationRoute{{
				ConversationID:       "channel_redacted",
				ConversationType:     ConversationChannel,
				SelectedChannelState: SelectedChannelSelected,
				ValidationState:      RoutePolicyValid,
			}},
		},
	})
	if err != nil {
		t.Fatalf("RecordHostedSetupValidation returned error: %v", err)
	}
	if setup.TerminalState != TerminalReady {
		t.Fatalf("expected ready setup, got %+v", setup)
	}
	stored, ok, err := sqliteStore.GetSlackHostedSetup(context.Background(), "ten_slack", "slack-main")
	if err != nil || !ok || stored.TerminalState != "ready" || stored.RoutePolicy == nil {
		t.Fatalf("stored setup ok=%v err=%v record=%+v", ok, err, stored)
	}
	if stored.WorkspaceBinding == nil || stored.WorkspaceBinding.WorkspaceID != "workspace_redacted" {
		t.Fatalf("expected workspace binding to be retained, got %+v", stored.WorkspaceBinding)
	}
	published := eventBus.List(events.Filter{Category: "connector"})
	if len(published) != 1 || published[0].Name != "connector.slack_setup_validated" {
		t.Fatalf("expected Slack setup validation event, got %+v", published)
	}
	if published[0].Payload["redactionStatus"] != "redacted" || published[0].Payload["routePolicyState"] != string(RoutePolicyStateValid) {
		t.Fatalf("unexpected event payload: %+v", published[0].Payload)
	}
}

func TestRuntimeNormalizesSlackInboundAndRecordsEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	cfg := Config{
		Enabled:             true,
		ConnectorID:         "slack-main",
		DisplayName:         "Slack Main",
		WorkspaceID:         "workspace_redacted",
		BotUserID:           "bot_redacted",
		AllowedChannelIDs:   []string{"channel_selected"},
		AllowedDMUserIDs:    []string{"user_allowed"},
		AllowedDMUserGroups: []string{"group_allowed"},
	}
	seedReadySlackSetup(t, sqliteStore, "ten_slack", cfg)
	runtime, err := NewRuntime(cfg, nil, baseconnectors.NewSupervisor(), im.NewMessageLoop(nil, nil, nil, nil, nil, nil), sqliteStore, eventBus, NewFakeTransport())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	inbound, ok := runtime.NormalizeInboundEvent(context.Background(), InboundEvent{
		TenantID:         "ten_slack",
		WorkspaceID:      "workspace_redacted",
		ConversationID:   "channel_selected",
		ConversationType: ConversationChannel,
		MessageID:        "message_1",
		EventID:          "event_1",
		SenderID:         "user_allowed",
		Text:             "<@bot_redacted> hello",
		ReceivedAt:       time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	})
	if !ok || inbound.Content != "hello" || inbound.EquivalentRuleID != "slack_workspace_conversation_message_id" {
		t.Fatalf("unexpected normalized inbound ok=%v inbound=%+v", ok, inbound)
	}
	duplicate, ok := runtime.NormalizeInboundEvent(context.Background(), InboundEvent{
		TenantID:         "ten_slack",
		WorkspaceID:      "workspace_redacted",
		ConversationID:   "channel_selected",
		ConversationType: ConversationChannel,
		MessageID:        "message_1",
		EventID:          "event_2",
		SenderID:         "user_allowed",
		Text:             "<@bot_redacted> hello again",
		Mentioned:        true,
		ReceivedAt:       time.Date(2026, 5, 8, 12, 1, 0, 0, time.UTC),
	})
	if ok || duplicate.ExternalMessageID != "" {
		t.Fatalf("duplicate should be suppressed, ok=%v inbound=%+v", ok, duplicate)
	}
	evidence, err := sqliteStore.ListSlackEventEvidence(context.Background(), "ten_slack", "slack-main", time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC), 10)
	if err != nil || len(evidence) != 2 {
		t.Fatalf("ListSlackEventEvidence len=%d err=%v", len(evidence), err)
	}
	if evidence[0].RouteOutcome != "duplicate" || evidence[1].RouteOutcome != "accepted" {
		t.Fatalf("unexpected Slack event evidence: %+v", evidence)
	}
	published := eventBus.List(events.Filter{Category: "connector"})
	if len(published) != 2 || published[1].Payload["outcome"] != "duplicate" {
		t.Fatalf("expected accepted and duplicate route events, got %+v", published)
	}
}

func TestRuntimeAppliesSlackRoutePolicyFailures(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(Config{
		Enabled:             true,
		ConnectorID:         "slack-main",
		DisplayName:         "Slack Main",
		WorkspaceID:         "workspace_redacted",
		BotUserID:           "bot_redacted",
		AllowedChannelIDs:   []string{"channel_selected"},
		AllowedDMUserIDs:    []string{"user_allowed"},
		AllowedDMUserGroups: []string{"group_allowed"},
	}, nil, baseconnectors.NewSupervisor(), im.NewMessageLoop(nil, nil, nil, nil, nil, nil), nil, nil, NewFakeTransport())
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	cases := []struct {
		name  string
		event InboundEvent
		want  bool
	}{
		{name: "allowed dm", event: InboundEvent{TenantID: "ten_slack", WorkspaceID: "workspace_redacted", ConversationID: "dm_1", ConversationType: ConversationDirectMessage, MessageID: "dm_1", SenderID: "user_allowed", Text: "hello"}, want: true},
		{name: "allowed dm user group", event: InboundEvent{TenantID: "ten_slack", WorkspaceID: "workspace_redacted", ConversationID: "dm_2", ConversationType: ConversationDirectMessage, MessageID: "dm_2", SenderID: "user_other", SenderUserGroupIDs: []string{"group_allowed"}, Text: "hello"}, want: true},
		{name: "blocked dm", event: InboundEvent{TenantID: "ten_slack", WorkspaceID: "workspace_redacted", ConversationID: "dm_3", ConversationType: ConversationDirectMessage, MessageID: "dm_3", SenderID: "user_other", Text: "hello"}, want: false},
		{name: "selected channel mention", event: InboundEvent{TenantID: "ten_slack", WorkspaceID: "workspace_redacted", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "chan_1", Text: "<@bot_redacted> hello"}, want: true},
		{name: "selected channel no mention", event: InboundEvent{TenantID: "ten_slack", WorkspaceID: "workspace_redacted", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "chan_2", Text: "hello"}, want: false},
		{name: "wrong workspace", event: InboundEvent{TenantID: "ten_slack", WorkspaceID: "workspace_other", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "chan_3", Mentioned: true, Text: "hello"}, want: false},
		{name: "unsupported file", event: InboundEvent{TenantID: "ten_slack", WorkspaceID: "workspace_redacted", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "chan_4", Surface: "file", Mentioned: true}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := runtime.NormalizeInboundEvent(context.Background(), tc.event)
			if ok != tc.want {
				t.Fatalf("NormalizeInboundEvent ok=%v, want %v for %+v", ok, tc.want, tc.event)
			}
		})
	}
}

func TestRuntimeSendsFinalOnlySlackDirectMessageReply(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	transport := NewFakeTransport(InboundEvent{
		TenantID:         "ten_slack",
		WorkspaceID:      "workspace_redacted",
		ConversationID:   "dm_redacted",
		ConversationType: ConversationDirectMessage,
		MessageID:        "message_dm_1",
		EventID:          "event_dm_1",
		SenderID:         "user_allowed",
		Text:             "hello",
		ReceivedAt:       time.Date(2026, 5, 8, 12, 2, 0, 0, time.UTC),
	})
	cfg := Config{
		Enabled:            true,
		ConnectorID:        "slack-main",
		DisplayName:        "Slack Main",
		WorkspaceID:        "workspace_redacted",
		WorkspaceBindingID: "workspace_binding_redacted",
		AllowedDMUserIDs:   []string{"user_allowed"},
	}
	seedReadySlackSetup(t, sqliteStore, "ten_slack", cfg)
	runtime, err := NewRuntime(cfg, nil, baseconnectors.NewSupervisor(), newSlackRuntimeTestLoop(t, sqliteStore, eventBus), sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_slack"})
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	replies := transport.SentReplies()
	if len(replies) != 1 {
		t.Fatalf("expected one final-only reply, got %d", len(replies))
	}
	if replies[0].ChannelID != "dm_redacted" || replies[0].Content != "reply:hello" || replies[0].ReplyToExternalMessageID != "message_dm_1" {
		t.Fatalf("unexpected direct message reply: %+v", replies[0])
	}
	if caps := transport.ReplyCapabilities(); caps.SupportsStreaming || caps.SupportsThinking {
		t.Fatalf("expected Slack fake transport to be final-only, got %+v", caps)
	}
}

func TestRuntimeBlocksInboundWhenHostedSetupIsNotReady(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	transport := NewFakeTransport()
	runtime, err := NewRuntime(Config{
		Enabled:            true,
		ConnectorID:        "slack-main",
		DisplayName:        "Slack Main",
		WorkspaceID:        "workspace_redacted",
		WorkspaceBindingID: "workspace_binding_redacted",
		AllowedDMUserIDs:   []string{"user_allowed"},
	}, nil, baseconnectors.NewSupervisor(), newSlackRuntimeTestLoop(t, sqliteStore, eventBus), sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}

	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_slack"})
	inbound, ok := runtime.NormalizeInboundEvent(ctx, InboundEvent{
		TenantID:         "ten_slack",
		WorkspaceID:      "workspace_redacted",
		ConversationID:   "dm_redacted",
		ConversationType: ConversationDirectMessage,
		MessageID:        "message_dm_unready",
		EventID:          "event_dm_unready",
		SenderID:         "user_allowed",
		Text:             "hello",
		ReceivedAt:       time.Date(2026, 5, 8, 12, 2, 0, 0, time.UTC),
	})
	if ok {
		t.Fatalf("expected unready hosted setup to block Slack inbound message, got %+v", inbound)
	}

	evidence, err := sqliteStore.ListSlackEventEvidence(ctx, "ten_slack", "slack-main", time.Date(2026, 5, 8, 12, 3, 0, 0, time.UTC), 10)
	if err != nil {
		t.Fatalf("ListSlackEventEvidence returned error: %v", err)
	}
	if len(evidence) != 1 || evidence[0].RouteOutcome != string(RouteBlocked) || evidence[0].ReasonCode != string(baseconnectors.DiagnosticAuthMissing) {
		t.Fatalf("expected auth-missing blocked route evidence, got %+v", evidence)
	}
}

func TestRuntimeSendsSlackChannelMentionReplyRootedAtTriggerThread(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	transport := NewFakeTransport(InboundEvent{
		TenantID:            "ten_slack",
		WorkspaceID:         "workspace_redacted",
		ConversationID:      "channel_selected",
		ConversationType:    ConversationChannel,
		MessageID:           "message_thread_reply",
		ThreadRootMessageID: "message_thread_root",
		EventID:             "event_channel_1",
		SenderID:            "user_allowed",
		Text:                "<@bot_redacted> summarize",
		ReceivedAt:          time.Date(2026, 5, 8, 12, 3, 0, 0, time.UTC),
	})
	cfg := Config{
		Enabled:            true,
		ConnectorID:        "slack-main",
		DisplayName:        "Slack Main",
		WorkspaceID:        "workspace_redacted",
		WorkspaceBindingID: "workspace_binding_redacted",
		BotUserID:          "bot_redacted",
		AllowedChannelIDs:  []string{"channel_selected"},
	}
	seedReadySlackSetup(t, sqliteStore, "ten_slack", cfg)
	runtime, err := NewRuntime(cfg, nil, baseconnectors.NewSupervisor(), newSlackRuntimeTestLoop(t, sqliteStore, eventBus), sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_slack"})
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	replies := transport.SentReplies()
	if len(replies) != 1 {
		t.Fatalf("expected one channel reply, got %d", len(replies))
	}
	if replies[0].ChannelID != "channel_selected" || replies[0].Content != "reply:summarize" || replies[0].ReplyToExternalMessageID != "message_thread_root" {
		t.Fatalf("expected thread-rooted Slack reply, got %+v", replies[0])
	}
}

func TestRuntimeRecordsSlackReplyFailureSeparatelyFromAssistantExecution(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	eventBus := events.NewBus()
	transport := NewFakeTransport(InboundEvent{
		TenantID:         "ten_slack",
		WorkspaceID:      "workspace_redacted",
		ConversationID:   "dm_redacted",
		ConversationType: ConversationDirectMessage,
		MessageID:        "message_dm_fail",
		EventID:          "event_dm_fail",
		SenderID:         "user_allowed",
		Text:             "hello",
		ReceivedAt:       time.Date(2026, 5, 8, 12, 4, 0, 0, time.UTC),
	})
	transport.SetReplyError(errors.New("slack 5xx transport failure"))
	cfg := Config{
		Enabled:            true,
		ConnectorID:        "slack-main",
		DisplayName:        "Slack Main",
		WorkspaceID:        "workspace_redacted",
		WorkspaceBindingID: "workspace_binding_redacted",
		AllowedDMUserIDs:   []string{"user_allowed"},
	}
	seedReadySlackSetup(t, sqliteStore, "ten_slack", cfg)
	runtime, err := NewRuntime(cfg, nil, baseconnectors.NewSupervisor(), newSlackRuntimeTestLoop(t, sqliteStore, eventBus), sqliteStore, eventBus, transport)
	if err != nil {
		t.Fatalf("NewRuntime returned error: %v", err)
	}
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_slack"})
	if err := runtime.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
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
	if failed.Payload["assistantExecutionOutcome"] != "succeeded" || failed.Payload["connectorDeliveryOutcome"] != "failed" || failed.Payload["connectorKind"] != "slack" {
		t.Fatalf("unexpected Slack reply failure separation payload: %+v", failed.Payload)
	}
	diagnostics, err := sqliteStore.ListConnectorDiagnosticStates(ctx, "ten_slack", "slack-main", time.Date(2026, 5, 8, 12, 5, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ListConnectorDiagnosticStates returned error: %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].ReasonCode != baseconnectors.DiagnosticProviderUnavailable {
		t.Fatalf("expected provider_unavailable diagnostic, got %+v", diagnostics)
	}
}

func TestConformanceProfileDeclaresSlackUnsupportedSurfaces(t *testing.T) {
	t.Parallel()

	profile := ConformanceProfile(Config{
		ConnectorID:         "slack-main",
		AllowedChannelIDs:   []string{"channel_selected"},
		AllowedDMUserGroups: []string{"group_allowed"},
	}, time.Date(2026, 5, 8, 13, 30, 0, 0, time.UTC))
	if profile.ConnectorKind != "slack" {
		t.Fatalf("connector kind=%s, want slack", profile.ConnectorKind)
	}
	for _, surface := range []string{"marketplace_publication", "enterprise_grid_administration", "memory_based_team_context", "files", "voice_huddles", "canvases", "workflow_buttons", "interactive_blocks", "rich_media", "thinking_visibility", "incremental_visible_updates"} {
		if profile.ProviderSurfaceResults[surface] != baseconnectors.SurfaceUnsupported {
			t.Fatalf("surface %s support=%s, want unsupported", surface, profile.ProviderSurfaceResults[surface])
		}
	}
	if profile.ProviderSurfaceResults["selected_channel_mention"] != baseconnectors.SurfaceSupported || profile.ProviderSurfaceResults["direct_message"] != baseconnectors.SurfaceSupported {
		t.Fatalf("expected selected channel and direct message support, got %+v", profile.ProviderSurfaceResults)
	}
}
