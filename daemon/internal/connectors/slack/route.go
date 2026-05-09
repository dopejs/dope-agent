package slack

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

type RouteOutcome string

const (
	RouteAccepted    RouteOutcome = "accepted"
	RouteIgnored     RouteOutcome = "ignored"
	RouteBlocked     RouteOutcome = "blocked"
	RouteDuplicate   RouteOutcome = "duplicate"
	RouteUnsupported RouteOutcome = "unsupported"
	RouteFailed      RouteOutcome = "failed"
)

type InboundEvent struct {
	TenantID            string
	ConnectorID         string
	WorkspaceID         string
	ConversationID      string
	ConversationType    ConversationType
	MessageID           string
	ThreadRootMessageID string
	EventID             string
	SenderID            string
	SenderUserGroupIDs  []string
	Text                string
	Mentioned           bool
	Surface             string
	ReceivedAt          time.Time
}

type RouteDecision struct {
	Outcome        RouteOutcome
	ReasonCode     string
	Surface        string
	NormalizedText string
}

func DecideRoute(event InboundEvent, policy RoutePolicy, workspaceID, botUserID string) RouteDecision {
	surface := firstNonEmpty(event.Surface, string(event.ConversationType))
	if missingSlackIdentity(event) {
		return RouteDecision{Outcome: RouteFailed, ReasonCode: string(baseconnectors.DiagnosticUnknownConnectorFailure), Surface: surface}
	}
	if isUnsupportedSurface(surface) {
		return RouteDecision{Outcome: RouteUnsupported, ReasonCode: string(baseconnectors.DiagnosticUnsupportedCapability), Surface: surface}
	}
	if strings.TrimSpace(workspaceID) != "" && strings.TrimSpace(event.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: surface}
	}
	policy = NormalizeRoutePolicy(policy, event.ReceivedAt)
	switch event.ConversationType {
	case ConversationDirectMessage:
		if allowedDMSender(event, policy) {
			return RouteDecision{Outcome: RouteAccepted, ReasonCode: "accepted", Surface: surface, NormalizedText: strings.TrimSpace(event.Text)}
		}
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: surface}
	case ConversationChannel:
		if !selectedChannel(event.ConversationID, policy) {
			return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: surface}
		}
		text := event.Text
		mentioned := event.Mentioned
		if botUserID != "" {
			normalized := NormalizeMentionText(text, botUserID)
			mentioned = mentioned || normalized != strings.TrimSpace(text)
			text = normalized
		}
		if !mentioned {
			return RouteDecision{Outcome: RouteIgnored, ReasonCode: "mention_required", Surface: surface}
		}
		return RouteDecision{Outcome: RouteAccepted, ReasonCode: "accepted", Surface: surface, NormalizedText: strings.TrimSpace(text)}
	default:
		return RouteDecision{Outcome: RouteUnsupported, ReasonCode: string(baseconnectors.DiagnosticUnsupportedCapability), Surface: surface}
	}
}

func missingSlackIdentity(event InboundEvent) bool {
	return strings.TrimSpace(event.WorkspaceID) == "" ||
		strings.TrimSpace(event.ConversationID) == "" ||
		strings.TrimSpace(event.MessageID) == ""
}

func allowedDMSender(event InboundEvent, policy RoutePolicy) bool {
	for _, id := range policy.AllowedDMUsers {
		if strings.TrimSpace(id) != "" && strings.TrimSpace(id) == strings.TrimSpace(event.SenderID) {
			return true
		}
	}
	allowedGroups := map[string]struct{}{}
	for _, id := range policy.AllowedDMUserGroups {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			allowedGroups[trimmed] = struct{}{}
		}
	}
	for _, id := range event.SenderUserGroupIDs {
		if _, ok := allowedGroups[strings.TrimSpace(id)]; ok {
			return true
		}
	}
	return false
}

func selectedChannel(conversationID string, policy RoutePolicy) bool {
	for _, channel := range policy.SelectedChannels {
		if channel.ConversationType == ConversationChannel &&
			channel.ConversationID == strings.TrimSpace(conversationID) &&
			channel.SelectedChannelState == SelectedChannelSelected &&
			channel.ValidationState == RoutePolicyValid {
			return true
		}
	}
	return false
}

func isUnsupportedSurface(surface string) bool {
	switch strings.TrimSpace(surface) {
	case "file", "voice_clip", "huddle", "canvas", "workflow_button", "interactive_block", "rich_media", "thinking", "incremental_update":
		return true
	default:
		return false
	}
}
