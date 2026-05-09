package slack

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

type Transport interface {
	Start(ctx context.Context, handle func(context.Context, InboundEvent)) error
	SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error)
	ReplyCapabilities() imtypes.ReplyCapabilities
	Close(ctx context.Context) error
}

type InstallationValidator interface {
	ValidateInstallation(ctx context.Context, binding WorkspaceBinding) (WorkspaceBinding, error)
}

type RouteValidator interface {
	ValidateRoutePolicy(ctx context.Context, policy RoutePolicy) (RoutePolicy, error)
}

type FakeTransport struct {
	mu       sync.Mutex
	started  bool
	inbound  []InboundEvent
	sent     []imtypes.OutboundReply
	startErr error
	replyErr error
}

func NewFakeTransport(messages ...InboundEvent) *FakeTransport {
	return &FakeTransport{inbound: append([]InboundEvent(nil), messages...)}
}

func (t *FakeTransport) Start(ctx context.Context, handle func(context.Context, InboundEvent)) error {
	t.mu.Lock()
	t.started = true
	messages := append([]InboundEvent(nil), t.inbound...)
	err := t.startErr
	t.mu.Unlock()
	if err != nil {
		return err
	}
	for _, message := range messages {
		handle(ctx, message)
	}
	return nil
}

func (t *FakeTransport) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.replyErr != nil {
		return imtypes.SentReply{}, t.replyErr
	}
	t.sent = append(t.sent, reply)
	return imtypes.SentReply{ExternalMessageID: "slack_reply_" + strings.TrimSpace(reply.ChannelID)}, nil
}

func (t *FakeTransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{MaxMessageLength: 40000}
}

func (t *FakeTransport) Close(context.Context) error { return nil }

func (t *FakeTransport) SetReplyError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.replyErr = err
}

func (t *FakeTransport) SentReplies() []imtypes.OutboundReply {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]imtypes.OutboundReply(nil), t.sent...)
}

func NormalizeMentionText(text, botUserID string) string {
	mention := "<@" + strings.TrimSpace(botUserID) + ">"
	return strings.TrimSpace(strings.ReplaceAll(text, mention, ""))
}

func UnsupportedSurfaceError(surface string) error {
	return fmt.Errorf("unsupported slack surface: %s", strings.TrimSpace(surface))
}
