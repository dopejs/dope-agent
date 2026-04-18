package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"

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
