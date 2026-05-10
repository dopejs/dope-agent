package matrix

import (
	"context"
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

type FakeTransport struct {
	mu      sync.Mutex
	inbound []InboundEvent
	sent    []imtypes.OutboundReply
}

func NewFakeTransport(messages ...InboundEvent) *FakeTransport {
	return &FakeTransport{inbound: append([]InboundEvent(nil), messages...)}
}

func (t *FakeTransport) Start(ctx context.Context, handle func(context.Context, InboundEvent)) error {
	for _, event := range append([]InboundEvent(nil), t.inbound...) {
		handle(ctx, event)
	}
	return nil
}

func (t *FakeTransport) SendReply(_ context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sent = append(t.sent, reply)
	return imtypes.SentReply{ExternalMessageID: "matrix_reply_" + strings.TrimSpace(reply.ChannelID)}, nil
}

func (t *FakeTransport) SentReplies() []imtypes.OutboundReply {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]imtypes.OutboundReply(nil), t.sent...)
}

func (t *FakeTransport) ReplyCapabilities() imtypes.ReplyCapabilities {
	return imtypes.ReplyCapabilities{MaxMessageLength: 40000}
}

func (t *FakeTransport) Close(context.Context) error { return nil }
