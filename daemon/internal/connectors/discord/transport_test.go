package discord

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/router"
)

func TestGatewayTransportNormalizeDirectMessage(t *testing.T) {
	t.Parallel()

	transport := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{},
		botUserID: "bot_1",
	}

	inbound, ok := transport.normalizeMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "msg_1",
			ChannelID: "dm_1",
			Content:   "hello from dm",
			Author:    &discordgo.User{ID: "user_1"},
		},
	})
	if !ok {
		t.Fatal("expected direct message to be normalized")
	}
	if inbound.Kind != router.SessionKindDirect {
		t.Fatalf("expected direct session kind, got %s", inbound.Kind)
	}
	if !inbound.Direct || !inbound.Mentioned {
		t.Fatalf("expected direct message to be treated as mentioned; got direct=%v mentioned=%v", inbound.Direct, inbound.Mentioned)
	}
	if inbound.Content != "hello from dm" {
		t.Fatalf("expected direct content preserved, got %q", inbound.Content)
	}
	if inbound.ConnectorAccountID != "bot_1" || inbound.ChannelOrConversationID != "dm_1" || inbound.ProviderMessageID != "msg_1" {
		t.Fatalf("expected standard identity fields, got account=%q channel=%q provider=%q", inbound.ConnectorAccountID, inbound.ChannelOrConversationID, inbound.ProviderMessageID)
	}
}

func TestGatewayTransportNormalizeGuildMentionStripsBotMention(t *testing.T) {
	t.Parallel()

	transport := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{},
		botUserID: "bot_1",
	}

	inbound, ok := transport.normalizeMessage(&discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "msg_2",
			ChannelID: "channel_1",
			GuildID:   "guild_1",
			Content:   "<@bot_1> hello guild",
			Author:    &discordgo.User{ID: "user_1"},
			Mentions: []*discordgo.User{
				{ID: "bot_1"},
			},
		},
	})
	if !ok {
		t.Fatal("expected guild message to be normalized")
	}
	if inbound.Kind != router.SessionKindGroup {
		t.Fatalf("expected group session kind, got %s", inbound.Kind)
	}
	if inbound.Direct {
		t.Fatal("expected guild message not to be direct")
	}
	if !inbound.Mentioned {
		t.Fatal("expected guild message to be marked mentioned")
	}
	if inbound.Content != "hello guild" {
		t.Fatalf("expected stripped guild content, got %q", inbound.Content)
	}
	if inbound.PeerID != "channel_1" || inbound.ThreadID != "channel_1" {
		t.Fatalf("expected channel scoped peer/thread ids, got peer=%q thread=%q", inbound.PeerID, inbound.ThreadID)
	}
	if inbound.ConnectorAccountID != "bot_1" || inbound.ChannelOrConversationID != "channel_1" || inbound.ProviderMessageID != "msg_2" || inbound.EquivalentRuleID != "discord_message_id" {
		t.Fatalf("expected standard identity fields, got account=%q channel=%q provider=%q rule=%q", inbound.ConnectorAccountID, inbound.ChannelOrConversationID, inbound.ProviderMessageID, inbound.EquivalentRuleID)
	}
}

func TestGatewayTransportSendReplyShapesDiscordRequest(t *testing.T) {
	originalSend := sendDiscordMessage
	defer func() {
		sendDiscordMessage = originalSend
	}()

	var capturedChannelID string
	var capturedMessage *discordgo.MessageSend
	sendDiscordMessage = func(_ *discordgo.Session, channelID string, message *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
		capturedChannelID = channelID
		capturedMessage = message
		return &discordgo.Message{ID: "reply_1"}, nil
	}

	transport := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{},
		botUserID: "bot_1",
	}

	reply, err := transport.SendReply(context.Background(), imtypes.OutboundReply{
		ConnectorID:              "discord-main",
		ChannelID:                "channel_1",
		Content:                  "assistant reply",
		ReplyToExternalMessageID: "msg_1",
	})
	if err != nil {
		t.Fatalf("SendReply returned error: %v", err)
	}
	if reply.ExternalMessageID != "reply_1" {
		t.Fatalf("expected reply message id reply_1, got %q", reply.ExternalMessageID)
	}
	if capturedChannelID != "channel_1" {
		t.Fatalf("expected channel_1, got %q", capturedChannelID)
	}
	if capturedMessage == nil || capturedMessage.Content != "assistant reply" {
		t.Fatalf("expected outbound content to be shaped, got %#v", capturedMessage)
	}
	if capturedMessage.Reference == nil || capturedMessage.Reference.MessageID != "msg_1" {
		t.Fatalf("expected outbound reply reference to msg_1, got %#v", capturedMessage.Reference)
	}
}

func TestGatewayTransportWrapsAuthFailure(t *testing.T) {
	originalSend := sendDiscordMessage
	defer func() {
		sendDiscordMessage = originalSend
	}()

	sendDiscordMessage = func(_ *discordgo.Session, _ string, _ *discordgo.MessageSend, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
		return nil, errors.New("401 Unauthorized")
	}

	transport := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{},
		botUserID: "bot_1",
	}

	_, err := transport.SendReply(context.Background(), imtypes.OutboundReply{
		ConnectorID:              "discord-main",
		ChannelID:                "channel_1",
		Content:                  "assistant reply",
		ReplyToExternalMessageID: "msg_1",
	})
	if err == nil {
		t.Fatal("expected auth failure from send reply")
	}

	classified, ok := err.(interface{ ErrorClass() string })
	if !ok {
		t.Fatalf("expected classified discord error, got %T", err)
	}
	if classified.ErrorClass() != "auth_error" {
		t.Fatalf("expected auth_error classification, got %q", classified.ErrorClass())
	}
}

func TestGatewayTransportSendThinkingUsesChannelTyping(t *testing.T) {
	originalTyping := sendDiscordTyping
	defer func() {
		sendDiscordTyping = originalTyping
	}()

	var capturedChannelID string
	sendDiscordTyping = func(_ *discordgo.Session, channelID string, _ ...discordgo.RequestOption) error {
		capturedChannelID = channelID
		return nil
	}

	transport := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{},
		botUserID: "bot_1",
	}

	if err := transport.SendThinking(context.Background(), imtypes.ThinkingSignal{
		ConnectorID: "discord-main",
		ChannelID:   "channel_1",
	}); err != nil {
		t.Fatalf("SendThinking returned error: %v", err)
	}
	if capturedChannelID != "channel_1" {
		t.Fatalf("expected typing signal for channel_1, got %q", capturedChannelID)
	}
}

func TestGatewayTransportEditReplyShapesDiscordRequest(t *testing.T) {
	originalEdit := editDiscordMessage
	defer func() {
		editDiscordMessage = originalEdit
	}()

	var capturedEdit *discordgo.MessageEdit
	editDiscordMessage = func(_ *discordgo.Session, edit *discordgo.MessageEdit, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
		capturedEdit = edit
		return &discordgo.Message{ID: edit.ID}, nil
	}

	transport := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{},
		botUserID: "bot_1",
	}

	if err := transport.EditReply(context.Background(), imtypes.ReplyEdit{
		ConnectorID:       "discord-main",
		ChannelID:         "channel_1",
		ExternalMessageID: "reply_1",
		Content:           "updated reply",
	}); err != nil {
		t.Fatalf("EditReply returned error: %v", err)
	}
	if capturedEdit == nil {
		t.Fatal("expected edit request to be issued")
	}
	if capturedEdit.ID != "reply_1" || capturedEdit.Channel != "channel_1" {
		t.Fatalf("expected reply edit target reply_1/channel_1, got %#v", capturedEdit)
	}
	if capturedEdit.Content == nil || *capturedEdit.Content != "updated reply" {
		t.Fatalf("expected updated content, got %#v", capturedEdit.Content)
	}
}

func TestGatewayTransportValidateDestinationsRequiresChannelPermissions(t *testing.T) {
	t.Parallel()

	valid := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{State: discordgo.NewState(), StateEnabled: true},
		botUserID: "bot_1",
	}
	addGuildWithBotChannelPermissions(t, valid.session.State, "guild_1", "channel_1", discordgo.PermissionViewChannel|discordgo.PermissionSendMessages|discordgo.PermissionReadMessageHistory)

	validated, err := valid.ValidateDestinations(context.Background(), []DestinationValidation{
		{ConnectorID: "discord-main", DestinationID: "channel_1", DestinationType: DestinationChannel, Selected: true},
	})
	if err != nil {
		t.Fatalf("ValidateDestinations valid returned error: %v", err)
	}
	if len(validated) != 1 || validated[0].ValidationState != DestinationValid {
		t.Fatalf("validated=%+v, want channel valid with send/read permissions", validated)
	}

	blocked := &GatewayTransport{
		cfg:       Config{ConnectorID: "discord-main"},
		session:   &discordgo.Session{State: discordgo.NewState(), StateEnabled: true},
		botUserID: "bot_1",
	}
	addGuildWithBotChannelPermissions(t, blocked.session.State, "guild_1", "channel_1", discordgo.PermissionViewChannel)

	degraded, err := blocked.ValidateDestinations(context.Background(), []DestinationValidation{
		{ConnectorID: "discord-main", DestinationID: "channel_1", DestinationType: DestinationChannel, Selected: true},
	})
	if err != nil {
		t.Fatalf("ValidateDestinations blocked returned error: %v", err)
	}
	if len(degraded) != 1 || degraded[0].ValidationState != DestinationMissingPermission || degraded[0].ReasonCode != "permission_missing" {
		t.Fatalf("validated=%+v, want missing_permission when send/read permissions are absent", degraded)
	}
	if degraded[0].SafeEvidence["missingPermissions"] == "" {
		t.Fatalf("expected missing permission evidence, got %+v", degraded[0].SafeEvidence)
	}
}

func TestGatewayTransportLifecycleObserverRecordsGatewayAndRateLimitEvidence(t *testing.T) {
	t.Parallel()

	transport := &GatewayTransport{cfg: Config{ConnectorID: "discord-main"}}
	events := make([]TransportLifecycleEvent, 0)
	transport.SetLifecycleObserver(func(_ context.Context, event TransportLifecycleEvent) {
		events = append(events, event)
	})

	transport.emitLifecycle(context.Background(), TransportLifecycleEvent{
		ReasonCode: baseconnectors.DiagnosticNetworkFailed,
		Evidence:   map[string]string{"stage": "gateway_disconnect"},
		Degraded:   true,
	})
	transport.emitLifecycle(context.Background(), TransportLifecycleEvent{
		ReasonCode: baseconnectors.DiagnosticNetworkFailed,
		Evidence:   map[string]string{"stage": "gateway_resumed"},
	})
	transport.emitLifecycle(context.Background(), TransportLifecycleEvent{
		ReasonCode: baseconnectors.DiagnosticRateLimited,
		Evidence: map[string]string{
			"stage":      "rate_limit",
			"bucket":     "messages",
			"retryAfter": (5 * time.Second).String(),
		},
		Degraded: true,
	})

	if len(events) != 3 {
		t.Fatalf("expected 3 lifecycle events, got %+v", events)
	}
	if events[0].ReasonCode != baseconnectors.DiagnosticNetworkFailed || events[0].Evidence["stage"] != "gateway_disconnect" || !events[0].Degraded {
		t.Fatalf("unexpected disconnect evidence: %+v", events[0])
	}
	if events[1].ReasonCode != baseconnectors.DiagnosticNetworkFailed || events[1].Evidence["stage"] != "gateway_resumed" {
		t.Fatalf("unexpected resume evidence: %+v", events[1])
	}
	if events[2].ReasonCode != baseconnectors.DiagnosticRateLimited || events[2].Evidence["retryAfter"] == "" || !events[2].Degraded {
		t.Fatalf("unexpected rate limit evidence: %+v", events[2])
	}
}

func addGuildWithBotChannelPermissions(t *testing.T, state *discordgo.State, guildID, channelID string, permissions int64) {
	t.Helper()
	if err := state.GuildAdd(&discordgo.Guild{
		ID: guildID,
		Roles: []*discordgo.Role{
			{ID: guildID, Permissions: 0},
			{ID: "role_bot", Permissions: permissions},
		},
		Members: []*discordgo.Member{
			{User: &discordgo.User{ID: "bot_1"}, Roles: []string{"role_bot"}},
		},
		Channels: []*discordgo.Channel{
			{ID: channelID, GuildID: guildID, Type: discordgo.ChannelTypeGuildText},
		},
	}); err != nil {
		t.Fatalf("GuildAdd returned error: %v", err)
	}
}
