package threads

import (
	"errors"
	"strings"
	"time"
)

type SourceKind string

const (
	SourceKindChat     SourceKind = "chat"
	SourceKindChannel  SourceKind = "channel"
	SourceKindWorkflow SourceKind = "workflow"
	SourceKindSchedule SourceKind = "schedule"
	SourceKindShell    SourceKind = "shell"
	SourceKindLegacy   SourceKind = "legacy"
)

type RoutingOutcome string

const (
	RoutingOutcomeAccepted                  RoutingOutcome = "accepted"
	RoutingOutcomeIgnored                   RoutingOutcome = "ignored"
	RoutingOutcomeBlocked                   RoutingOutcome = "blocked"
	RoutingOutcomeDuplicate                 RoutingOutcome = "duplicate"
	RoutingOutcomeDisabled                  RoutingOutcome = "disabled"
	RoutingOutcomeUnsupported               RoutingOutcome = "unsupported"
	RoutingOutcomeFailed                    RoutingOutcome = "failed"
	RoutingOutcomeUnknownSource             RoutingOutcome = "unknown_source"
	RoutingOutcomeStaleSource               RoutingOutcome = "stale_source"
	RoutingOutcomeInaccessibleTenantBinding RoutingOutcome = "inaccessible_tenant_binding"
)

type SourceLinkage struct {
	SourceLinkageID      string          `json:"sourceLinkageId"`
	ThreadID             string          `json:"threadId,omitempty"`
	TenantID             string          `json:"tenantId,omitempty"`
	SourceKind           SourceKind      `json:"sourceKind"`
	ConnectorID          string          `json:"connectorId,omitempty"`
	ConnectorKind        string          `json:"connectorKind,omitempty"`
	SourceAccountID      string          `json:"sourceAccountId,omitempty"`
	SourceConversationID string          `json:"sourceConversationId,omitempty"`
	SourceMessageID      string          `json:"sourceMessageId,omitempty"`
	RoutingOutcome       RoutingOutcome  `json:"routingOutcome"`
	Current              bool            `json:"current"`
	LinkedAt             time.Time       `json:"linkedAt,omitempty"`
	RetentionExpiresAt   time.Time       `json:"retentionExpiresAt,omitempty"`
	RedactionStatus      RedactionStatus `json:"redactionStatus"`
}

type SourceContinuationKey struct {
	TenantID             string
	ConnectorID          string
	SourceAccountID      string
	SourceConversationID string
}

var ErrInvalidSourceContinuationKey = errors.New("source continuation key requires tenant, connector, source account, and source conversation")

func NormalizeSourceContinuationKey(key SourceContinuationKey) (SourceContinuationKey, error) {
	normalized := SourceContinuationKey{
		TenantID:             normalizeKeyPart(key.TenantID),
		ConnectorID:          normalizeKeyPart(key.ConnectorID),
		SourceAccountID:      normalizeKeyPart(key.SourceAccountID),
		SourceConversationID: normalizeKeyPart(key.SourceConversationID),
	}
	if normalized.TenantID == "" || normalized.ConnectorID == "" || normalized.SourceAccountID == "" || normalized.SourceConversationID == "" {
		return SourceContinuationKey{}, ErrInvalidSourceContinuationKey
	}
	return normalized, nil
}

func (key SourceContinuationKey) String() string {
	return key.TenantID + "\x00" + key.ConnectorID + "\x00" + key.SourceAccountID + "\x00" + key.SourceConversationID
}

func normalizeKeyPart(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
