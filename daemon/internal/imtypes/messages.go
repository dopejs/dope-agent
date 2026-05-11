package imtypes

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/router"
)

type DeliveryDirection string

const (
	DeliveryDirectionInbound  DeliveryDirection = "inbound"
	DeliveryDirectionOutbound DeliveryDirection = "outbound"
)

type DeliveryStatus string

const (
	DeliveryStatusReceived   DeliveryStatus = "received"
	DeliveryStatusThinking   DeliveryStatus = "thinking"
	DeliveryStatusProcessing DeliveryStatus = "processing"
	DeliveryStatusStreaming  DeliveryStatus = "streaming"
	DeliveryStatusReplied    DeliveryStatus = "replied"
	DeliveryStatusPartial    DeliveryStatus = "partial"
	DeliveryStatusFailed     DeliveryStatus = "failed"
)

type ReplyCapabilities struct {
	SupportsThinking  bool `json:"supportsThinking"`
	SupportsStreaming bool `json:"supportsStreaming"`
	MaxMessageLength  int  `json:"maxMessageLength,omitempty"`
}

type MessageRecord struct {
	DeliveryID               string            `json:"deliveryId"`
	TenantID                 string            `json:"tenantId,omitempty"`
	ConnectorID              string            `json:"connectorId"`
	Direction                DeliveryDirection `json:"direction"`
	ExternalMessageID        string            `json:"externalMessageId,omitempty"`
	ConnectorAccountID       string            `json:"connectorAccountId,omitempty"`
	ChannelOrConversationID  string            `json:"channelOrConversationId,omitempty"`
	ProviderMessageID        string            `json:"providerMessageId,omitempty"`
	EquivalentRuleID         string            `json:"equivalentRuleId,omitempty"`
	SessionID                string            `json:"sessionId,omitempty"`
	ThreadSessionSegmentID   string            `json:"threadSessionSegmentId,omitempty"`
	RunID                    string            `json:"runId,omitempty"`
	ChannelID                string            `json:"channelId"`
	PeerID                   string            `json:"peerId,omitempty"`
	ThreadID                 string            `json:"threadId,omitempty"`
	AuthorID                 string            `json:"authorId,omitempty"`
	Content                  string            `json:"content"`
	Status                   DeliveryStatus    `json:"status"`
	Error                    string            `json:"error,omitempty"`
	ReplyToExternalMessageID string            `json:"replyToExternalMessageId,omitempty"`
	ResponseToDeliveryID     string            `json:"responseToDeliveryId,omitempty"`
	ForegroundOutcomeStatus  string            `json:"foregroundOutcomeStatus,omitempty"`
	BackgroundDeliveryID     string            `json:"backgroundDeliveryId,omitempty"`
	DeliveryBoundaryKind     string            `json:"deliveryBoundaryKind,omitempty"`
	CreatedAt                time.Time         `json:"createdAt"`
	UpdatedAt                time.Time         `json:"updatedAt"`
}

type InboundMessage struct {
	ConnectorID             string
	ConnectorKind           string
	ExternalMessageID       string
	TenantID                string
	AccountID               string
	ConnectorAccountID      string
	ChannelOrConversationID string
	ProviderMessageID       string
	EquivalentRuleID        string
	ChannelID               string
	GuildID                 string
	PeerID                  string
	ThreadID                string
	AuthorID                string
	Content                 string
	Kind                    router.SessionKind
	ReplyToMessageID        string
	Direct                  bool
	Mentioned               bool
	ReceivedAt              time.Time
}

type OutboundReply struct {
	ConnectorID              string
	ChannelID                string
	Content                  string
	ReplyToExternalMessageID string
}

type ReplyEdit struct {
	ConnectorID       string
	ChannelID         string
	ExternalMessageID string
	Content           string
}

type ThinkingSignal struct {
	ConnectorID string
	ChannelID   string
}

type SentReply struct {
	ExternalMessageID string
}
