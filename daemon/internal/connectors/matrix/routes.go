package matrix

import (
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func NormalizeRoutePolicy(policy RoutePolicy, now time.Time) RoutePolicy {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if policy.RoomInvocationGate == "" {
		policy.RoomInvocationGate = "bot_mention_or_command_required"
	}
	if policy.EncryptedRoomPolicy == "" {
		policy.EncryptedRoomPolicy = "unsupported"
	}
	if policy.ValidationState == "" {
		policy.ValidationState = RoutePolicyNone
	}
	if policy.ValidatedAt.IsZero() {
		policy.ValidatedAt = now
	}
	if policy.RedactionStatus == "" {
		policy.RedactionStatus = baseconnectors.RedactionStatusRedacted
	}
	for i := range policy.SelectedRooms {
		if policy.SelectedRooms[i].ConversationType == "" {
			policy.SelectedRooms[i].ConversationType = ConversationRoom
		}
		if policy.SelectedRooms[i].RoomSelectionState == "" {
			policy.SelectedRooms[i].RoomSelectionState = RoomSelected
		}
		if policy.SelectedRooms[i].ValidationState == "" {
			policy.SelectedRooms[i].ValidationState = RoutePolicyValid
		}
		if policy.SelectedRooms[i].RedactionStatus == "" {
			policy.SelectedRooms[i].RedactionStatus = baseconnectors.RedactionStatusRedacted
		}
	}
	return policy
}

func HasReadyRoutePolicy(policy RoutePolicy) bool {
	policy = NormalizeRoutePolicy(policy, time.Now().UTC())
	if policy.ValidationState != RoutePolicyValid {
		return false
	}
	if len(policy.AllowedDirectUsers) > 0 {
		return true
	}
	for _, room := range policy.SelectedRooms {
		if room.ConversationType == ConversationRoom && room.RoomSelectionState == RoomSelected && room.ValidationState == RoutePolicyValid {
			return true
		}
	}
	return false
}

func DecideRoute(event InboundEvent, policy RoutePolicy, homeserverID, botUserID string) RouteDecision {
	surface := string(event.ConversationType)
	if missingMatrixIdentity(event) {
		return RouteDecision{Outcome: RouteFailed, ReasonCode: string(baseconnectors.DiagnosticUnknownConnectorFailure), Surface: surface}
	}
	if event.MessageKind != MessageUnencryptedText {
		return RouteDecision{Outcome: RouteUnsupported, ReasonCode: string(baseconnectors.DiagnosticUnsupportedCapability), Surface: surface}
	}
	if strings.TrimSpace(homeserverID) != "" && strings.TrimSpace(event.HomeserverID) != strings.TrimSpace(homeserverID) {
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: surface}
	}
	policy = NormalizeRoutePolicy(policy, event.ReceivedAt)
	switch event.ConversationType {
	case ConversationDirectMessage:
		if allowedDirectSender(event.SenderID, policy) {
			return RouteDecision{Outcome: RouteAccepted, ReasonCode: "accepted", Surface: surface, NormalizedText: strings.TrimSpace(event.Text)}
		}
		return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: surface}
	case ConversationRoom:
		if !selectedRoom(event.ConversationID, policy) {
			return RouteDecision{Outcome: RouteBlocked, ReasonCode: string(baseconnectors.DiagnosticBlockedRoute), Surface: surface}
		}
		text := strings.TrimSpace(event.Text)
		if strings.TrimSpace(botUserID) != "" {
			mention := strings.TrimSpace(botUserID)
			if strings.Contains(text, mention) {
				event.BotMentioned = true
				text = strings.TrimSpace(strings.ReplaceAll(text, mention, ""))
			}
		}
		if !event.BotMentioned && !hasConfiguredCommand(text, policy.ConfiguredCommands) {
			return RouteDecision{Outcome: RouteIgnored, ReasonCode: "mention_required", Surface: surface}
		}
		text = trimConfiguredCommand(text, policy.ConfiguredCommands)
		return RouteDecision{Outcome: RouteAccepted, ReasonCode: "accepted", Surface: surface, NormalizedText: strings.TrimSpace(text)}
	default:
		return RouteDecision{Outcome: RouteUnsupported, ReasonCode: string(baseconnectors.DiagnosticUnsupportedCapability), Surface: surface}
	}
}

func missingMatrixIdentity(event InboundEvent) bool {
	return strings.TrimSpace(event.HomeserverID) == "" || strings.TrimSpace(event.ConversationID) == "" || strings.TrimSpace(event.MatrixEventID) == ""
}

func allowedDirectSender(senderID string, policy RoutePolicy) bool {
	for _, allowed := range policy.AllowedDirectUsers {
		if strings.TrimSpace(allowed) != "" && strings.TrimSpace(allowed) == strings.TrimSpace(senderID) {
			return true
		}
	}
	return false
}

func selectedRoom(conversationID string, policy RoutePolicy) bool {
	for _, room := range policy.SelectedRooms {
		if room.ConversationID == strings.TrimSpace(conversationID) && room.RoomSelectionState == RoomSelected && room.ValidationState == RoutePolicyValid {
			return true
		}
	}
	return false
}

func hasConfiguredCommand(text string, commands []string) bool {
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" && strings.HasPrefix(strings.TrimSpace(text), command) {
			return true
		}
	}
	return false
}

func trimConfiguredCommand(text string, commands []string) string {
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" && strings.HasPrefix(strings.TrimSpace(text), command) {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), command))
		}
	}
	return text
}
