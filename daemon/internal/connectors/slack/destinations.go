package slack

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type ConversationType string

const (
	ConversationDirectMessage ConversationType = "direct_message"
	ConversationChannel       ConversationType = "channel"
)

type SelectedChannelState string

const (
	SelectedChannelSelected          SelectedChannelState = "selected"
	SelectedChannelNotSelected       SelectedChannelState = "not_selected"
	SelectedChannelStale             SelectedChannelState = "stale"
	SelectedChannelArchived          SelectedChannelState = "archived"
	SelectedChannelMissingMembership SelectedChannelState = "missing_membership"
	SelectedChannelNotApplicable     SelectedChannelState = "not_applicable"
)

type RouteValidationState string

const (
	RoutePolicyValid             RouteValidationState = "valid"
	RoutePolicyPartial           RouteValidationState = "partial"
	RoutePolicyStale             RouteValidationState = "stale"
	RoutePolicyBlocked           RouteValidationState = "blocked"
	RoutePolicyMissingPermission RouteValidationState = "missing_permission"
)

type ConversationRoute struct {
	ConversationID       string                         `json:"conversationId"`
	ConversationType     ConversationType               `json:"conversationType"`
	SelectedChannelState SelectedChannelState           `json:"selectedChannelState"`
	ValidationState      RouteValidationState           `json:"validationState"`
	ReasonCode           string                         `json:"reasonCode,omitempty"`
	RedactionStatus      baseconnectors.RedactionStatus `json:"redactionStatus,omitempty"`
	SafeEvidence         map[string]string              `json:"safeEvidence,omitempty"`
}

type RoutePolicy struct {
	TenantID            string                         `json:"tenantId,omitempty"`
	ConnectorID         string                         `json:"connectorId"`
	WorkspaceBindingID  string                         `json:"workspaceBindingId"`
	SelectedChannels    []ConversationRoute            `json:"selectedChannels"`
	AllowedDMUsers      []string                       `json:"allowedDMUsers"`
	AllowedDMUserGroups []string                       `json:"allowedDMUserGroups"`
	MentionGate         string                         `json:"mentionGate"`
	ThreadReplyMode     string                         `json:"threadReplyMode"`
	ValidationState     RouteValidationState           `json:"validationState"`
	ReasonCode          string                         `json:"reasonCode,omitempty"`
	CreatedAt           time.Time                      `json:"createdAt,omitempty"`
	UpdatedAt           time.Time                      `json:"updatedAt,omitempty"`
	ValidatedAt         time.Time                      `json:"validatedAt,omitempty"`
	RedactionStatus     baseconnectors.RedactionStatus `json:"redactionStatus"`
	SafeEvidence        map[string]string              `json:"safeEvidence,omitempty"`
}

func NormalizeRoutePolicy(policy RoutePolicy, now time.Time) RoutePolicy {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	policy.TenantID = strings.TrimSpace(policy.TenantID)
	policy.ConnectorID = strings.TrimSpace(policy.ConnectorID)
	policy.WorkspaceBindingID = strings.TrimSpace(policy.WorkspaceBindingID)
	policy.MentionGate = firstNonEmpty(strings.TrimSpace(policy.MentionGate), "agent_mention_required")
	policy.ThreadReplyMode = firstNonEmpty(strings.TrimSpace(policy.ThreadReplyMode), "channel_mentions_thread_rooted")
	if policy.ValidationState == "" {
		policy.ValidationState = RoutePolicyBlocked
	}
	if policy.RedactionStatus == "" {
		policy.RedactionStatus = baseconnectors.RedactionStatusRedacted
	}
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = now
	}
	if policy.UpdatedAt.IsZero() {
		policy.UpdatedAt = now
	}
	if policy.ValidatedAt.IsZero() {
		policy.ValidatedAt = now
	}
	for i := range policy.SelectedChannels {
		policy.SelectedChannels[i].ConversationID = strings.TrimSpace(policy.SelectedChannels[i].ConversationID)
		if policy.SelectedChannels[i].ConversationType == "" {
			policy.SelectedChannels[i].ConversationType = ConversationChannel
		}
		if policy.SelectedChannels[i].SelectedChannelState == "" {
			policy.SelectedChannels[i].SelectedChannelState = SelectedChannelNotSelected
		}
		if policy.SelectedChannels[i].ValidationState == "" {
			policy.SelectedChannels[i].ValidationState = RoutePolicyBlocked
		}
		if policy.SelectedChannels[i].RedactionStatus == "" {
			policy.SelectedChannels[i].RedactionStatus = baseconnectors.RedactionStatusRedacted
		}
	}
	return policy
}

func HasReadyRoutePolicy(policy RoutePolicy) bool {
	policy = NormalizeRoutePolicy(policy, time.Time{})
	if policy.ValidationState != RoutePolicyValid {
		return false
	}
	if hasAllowedDM(policy) {
		return true
	}
	for _, channel := range policy.SelectedChannels {
		if channel.ConversationType == ConversationChannel &&
			channel.ConversationID != "" &&
			channel.SelectedChannelState == SelectedChannelSelected &&
			channel.ValidationState == RoutePolicyValid {
			return true
		}
	}
	return false
}

func hasAllowedDM(policy RoutePolicy) bool {
	for _, id := range policy.AllowedDMUsers {
		if strings.TrimSpace(id) != "" {
			return true
		}
	}
	for _, id := range policy.AllowedDMUserGroups {
		if strings.TrimSpace(id) != "" {
			return true
		}
	}
	return false
}
