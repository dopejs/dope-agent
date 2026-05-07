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
	lifecycle func(context.Context, TransportLifecycleEvent)
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
	t.session.AddHandler(func(s *discordgo.Session, _ *discordgo.Disconnect) {
		t.emitLifecycle(ctx, TransportLifecycleEvent{
			ReasonCode: "network_failed",
			Evidence:   map[string]string{"stage": "gateway_disconnect"},
			Degraded:   true,
		})
	})
	t.session.AddHandler(func(s *discordgo.Session, _ *discordgo.Resumed) {
		t.emitLifecycle(ctx, TransportLifecycleEvent{
			ReasonCode: "network_failed",
			Evidence:   map[string]string{"stage": "gateway_resumed"},
			Degraded:   false,
		})
	})
	t.session.AddHandler(func(s *discordgo.Session, rateLimit *discordgo.RateLimit) {
		evidence := map[string]string{"stage": "rate_limit"}
		if rateLimit != nil {
			evidence["url"] = redactDiscordRoute(rateLimit.URL)
			if rateLimit.TooManyRequests != nil {
				evidence["bucket"] = rateLimit.TooManyRequests.Bucket
				evidence["retryAfter"] = rateLimit.TooManyRequests.RetryAfter.String()
			}
		}
		t.emitLifecycle(ctx, TransportLifecycleEvent{
			ReasonCode: "rate_limited",
			Evidence:   evidence,
			Degraded:   true,
		})
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
		MaxMessageLength:  2000,
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

func (t *GatewayTransport) SetLifecycleObserver(observer func(context.Context, TransportLifecycleEvent)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lifecycle = observer
}

func (t *GatewayTransport) emitLifecycle(ctx context.Context, event TransportLifecycleEvent) {
	t.mu.RLock()
	observer := t.lifecycle
	t.mu.RUnlock()
	if observer != nil {
		observer(ctx, event)
	}
}

func (t *GatewayTransport) ValidateDestinations(ctx context.Context, destinations []DestinationValidation) ([]DestinationValidation, error) {
	if t.session == nil {
		return nil, fmt.Errorf("discord session is not configured")
	}
	now := time.Now().UTC()
	validated := make([]DestinationValidation, 0, len(destinations))
	for _, destination := range destinations {
		destination.ValidatedAt = now
		destination.RedactionStatus = "redacted"
		destination.SafeEvidence = map[string]string{"source": "gateway_state"}
		switch destination.DestinationType {
		case DestinationGuild:
			if guild, err := t.session.State.Guild(destination.DestinationID); err == nil && guild != nil {
				destination.ValidationState = DestinationValid
				destination.ReasonCode = "healthy"
				destination.ProviderLabel = redactedDiscordLabel(guild.ID)
			} else {
				destination.ValidationState = DestinationBotNotMember
				destination.ReasonCode = "bot_not_member"
			}
		case DestinationChannel:
			if channel, err := t.session.State.Channel(destination.DestinationID); err == nil && channel != nil {
				permissions, err := t.session.UserChannelPermissions(t.currentBotUserID(), destination.DestinationID, discordgo.WithContext(ctx))
				if err != nil {
					destination.ValidationState = DestinationMissingPermission
					destination.ReasonCode = "permission_missing"
					destination.ProviderLabel = redactedDiscordLabel(channel.ID)
					destination.SafeEvidence["permissionCheck"] = "failed"
					destination.SafeEvidence["errorClass"] = classifyDiscordError(err)
				} else if missing := missingDiscordChannelPermissions(permissions); missing != "" {
					destination.ValidationState = DestinationMissingPermission
					destination.ReasonCode = "permission_missing"
					destination.ProviderLabel = redactedDiscordLabel(channel.ID)
					destination.SafeEvidence["missingPermissions"] = missing
				} else {
					destination.ValidationState = DestinationValid
					destination.ReasonCode = "healthy"
					destination.ProviderLabel = redactedDiscordLabel(channel.ID)
					destination.SafeEvidence["permissionCheck"] = "send_read"
				}
			} else {
				destination.ValidationState = DestinationNotFound
				destination.ReasonCode = "not_found"
			}
		default:
			destination.ValidationState = DestinationInvalid
			destination.ReasonCode = "unsupported_destination"
		}
		validated = append(validated, destination)
	}
	return validated, nil
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
		ConnectorID:             t.cfg.ConnectorID,
		ConnectorKind:           "discord",
		ExternalMessageID:       message.ID,
		AccountID:               botUserID,
		ConnectorAccountID:      botUserID,
		ChannelOrConversationID: message.ChannelID,
		ProviderMessageID:       message.ID,
		EquivalentRuleID:        "discord_message_id",
		ChannelID:               message.ChannelID,
		GuildID:                 message.GuildID,
		PeerID:                  peerID,
		ThreadID:                threadID,
		AuthorID:                message.Author.ID,
		Content:                 content,
		Kind:                    kind,
		Direct:                  direct,
		Mentioned:               direct || mentioned,
		ReceivedAt:              time.Now().UTC(),
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

func redactDiscordRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return ""
	}
	parts := strings.Split(route, "/")
	for index, part := range parts {
		if looksLikeDiscordID(part) {
			parts[index] = "redacted_id"
		}
	}
	return strings.Join(parts, "/")
}

func redactedDiscordLabel(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	return "discord_resource_redacted"
}

func looksLikeDiscordID(value string) bool {
	if len(value) < 12 {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func missingDiscordChannelPermissions(permissions int64) string {
	required := []struct {
		name string
		bit  int64
	}{
		{name: "view_channel", bit: discordgo.PermissionViewChannel},
		{name: "send_messages", bit: discordgo.PermissionSendMessages},
		{name: "read_message_history", bit: discordgo.PermissionReadMessageHistory},
	}
	missing := make([]string, 0)
	for _, permission := range required {
		if permissions&permission.bit == 0 {
			missing = append(missing, permission.name)
		}
	}
	return strings.Join(missing, ",")
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
