package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/router"
)

type GatewayTransport struct {
	cfg       Config
	session   *discordgo.Session
	botUserID string
	mu        sync.RWMutex
}

var (
	openDiscordSession = func(session *discordgo.Session) error {
		return session.Open()
	}
	sendDiscordTyping = func(session *discordgo.Session, channelID string, options ...discordgo.RequestOption) error {
		return session.ChannelTyping(channelID, options...)
	}
	sendDiscordMessage = func(session *discordgo.Session, channelID string, message *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error) {
		return session.ChannelMessageSendComplex(channelID, message, options...)
	}
	editDiscordMessage = func(session *discordgo.Session, edit *discordgo.MessageEdit, options ...discordgo.RequestOption) (*discordgo.Message, error) {
		return session.ChannelMessageEditComplex(edit, options...)
	}
)

type classifiedDiscordError struct {
	class string
	err   error
}

func (e *classifiedDiscordError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *classifiedDiscordError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *classifiedDiscordError) ErrorClass() string {
	if e == nil {
		return ""
	}
	return e.class
}

func NewGatewayTransport(cfg Config) (*GatewayTransport, error) {
	token := strings.TrimSpace(cfg.BotToken)
	if token == "" {
		return nil, fmt.Errorf("discord bot token is required")
	}
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentMessageContent

	return &GatewayTransport{
		cfg:     cfg,
		session: session,
	}, nil
}

func (t *GatewayTransport) Start(ctx context.Context, handle func(context.Context, imtypes.InboundMessage)) error {
	if t.session == nil {
		return fmt.Errorf("discord session is not configured")
	}

	t.session.AddHandler(func(s *discordgo.Session, ready *discordgo.Ready) {
		if ready == nil || ready.User == nil {
			return
		}
		t.mu.Lock()
		t.botUserID = ready.User.ID
		t.mu.Unlock()
	})
	t.session.AddHandler(func(s *discordgo.Session, message *discordgo.MessageCreate) {
		inbound, ok := t.normalizeMessage(message)
		if !ok {
			return
		}
		handle(ctx, inbound)
	})

	if err := openDiscordSession(t.session); err != nil {
		return wrapDiscordError("open discord session", err)
	}

	if user := t.session.State.User; user != nil && user.ID != "" {
		t.mu.Lock()
		t.botUserID = user.ID
		t.mu.Unlock()
	}
	return nil
}

func (t *GatewayTransport) SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	if t.session == nil {
		return imtypes.SentReply{}, fmt.Errorf("discord session is not configured")
	}

	message, err := sendDiscordMessage(t.session, reply.ChannelID, &discordgo.MessageSend{
		Content: reply.Content,
		Reference: &discordgo.MessageReference{
			MessageID: reply.ReplyToExternalMessageID,
			ChannelID: reply.ChannelID,
		},
	}, discordgo.WithContext(ctx))
	if err != nil {
		return imtypes.SentReply{}, wrapDiscordError("send discord reply", err)
	}
	return imtypes.SentReply{ExternalMessageID: message.ID}, nil
}

func (t *GatewayTransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{
		SupportsThinking:  true,
		SupportsStreaming: true,
	}
}

func (t *GatewayTransport) SendThinking(ctx context.Context, signal imtypes.ThinkingSignal) error {
	if t.session == nil {
		return fmt.Errorf("discord session is not configured")
	}
	return wrapDiscordError("send discord typing", sendDiscordTyping(t.session, signal.ChannelID, discordgo.WithContext(ctx)))
}

func (t *GatewayTransport) EditReply(ctx context.Context, edit imtypes.ReplyEdit) error {
	if t.session == nil {
		return fmt.Errorf("discord session is not configured")
	}
	_, err := editDiscordMessage(t.session, &discordgo.MessageEdit{
		ID:      edit.ExternalMessageID,
		Channel: edit.ChannelID,
		Content: &edit.Content,
	}, discordgo.WithContext(ctx))
	if err != nil {
		return wrapDiscordError("edit discord reply", err)
	}
	return nil
}

func (t *GatewayTransport) Close(_ context.Context) error {
	if t.session == nil {
		return nil
	}
	return t.session.Close()
}

func (t *GatewayTransport) normalizeMessage(message *discordgo.MessageCreate) (imtypes.InboundMessage, bool) {
	if message == nil || message.Message == nil || message.Author == nil {
		return imtypes.InboundMessage{}, false
	}
	if message.Author.Bot {
		return imtypes.InboundMessage{}, false
	}

	content := strings.TrimSpace(message.Content)
	if content == "" {
		return imtypes.InboundMessage{}, false
	}

	botUserID := t.currentBotUserID()
	mentioned := mentionedUser(message.Message, botUserID)
	direct := message.GuildID == ""
	if !direct && botUserID != "" {
		content = stripBotMention(content, botUserID)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return imtypes.InboundMessage{}, false
	}

	kind := router.SessionKindDirect
	threadID := ""
	peerID := message.Author.ID
	if !direct {
		kind = router.SessionKindGroup
		threadID = message.ChannelID
		peerID = message.ChannelID
	}

	return imtypes.InboundMessage{
		ConnectorID:       t.cfg.ConnectorID,
		ConnectorKind:     "discord",
		ExternalMessageID: message.ID,
		AccountID:         botUserID,
		ChannelID:         message.ChannelID,
		GuildID:           message.GuildID,
		PeerID:            peerID,
		ThreadID:          threadID,
		AuthorID:          message.Author.ID,
		Content:           content,
		Kind:              kind,
		Direct:            direct,
		Mentioned:         direct || mentioned,
		ReceivedAt:        time.Now().UTC(),
	}, true
}

func (t *GatewayTransport) currentBotUserID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.botUserID
}

func mentionedUser(message *discordgo.Message, userID string) bool {
	if message == nil || userID == "" {
		return false
	}
	for _, user := range message.Mentions {
		if user != nil && user.ID == userID {
			return true
		}
	}
	return false
}

func stripBotMention(content, userID string) string {
	content = strings.ReplaceAll(content, "<@"+userID+">", "")
	content = strings.ReplaceAll(content, "<@!"+userID+">", "")
	return strings.TrimSpace(content)
}

func wrapDiscordError(prefix string, err error) error {
	if err == nil {
		return nil
	}
	class := classifyDiscordError(err)
	return &classifiedDiscordError{
		class: class,
		err:   fmt.Errorf("%s: %w", prefix, err),
	}
}
