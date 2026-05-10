package matrix

import (
	"testing"
	"time"
)

func TestDecideRouteAcceptsDirectAndRoomInvocationGate(t *testing.T) {
	t.Parallel()

	policy := NormalizeRoutePolicy(RoutePolicy{
		AllowedDirectUsers: []string{"@alice:example.org"},
		SelectedRooms: []ConversationRoute{{
			ConversationID:     "!room:example.org",
			ConversationType:   ConversationRoom,
			RoomSelectionState: RoomSelected,
			ValidationState:    RoutePolicyValid,
		}},
		ConfiguredCommands: []string{"!dope"},
		ValidationState:    RoutePolicyValid,
	}, time.Now())

	direct := DecideRoute(InboundEvent{
		HomeserverID:     "matrix.example.org",
		ConversationID:   "@alice:example.org",
		MatrixEventID:    "$event1",
		SenderID:         "@alice:example.org",
		ConversationType: ConversationDirectMessage,
		MessageKind:      MessageUnencryptedText,
		Text:             "hello",
	}, policy, "matrix.example.org", "@bot:example.org")
	if direct.Outcome != RouteAccepted {
		t.Fatalf("direct route outcome = %s, want accepted", direct.Outcome)
	}

	room := DecideRoute(InboundEvent{
		HomeserverID:     "matrix.example.org",
		ConversationID:   "!room:example.org",
		MatrixEventID:    "$event2",
		SenderID:         "@alice:example.org",
		ConversationType: ConversationRoom,
		MessageKind:      MessageUnencryptedText,
		Text:             "@bot:example.org hello",
		BotMentioned:     true,
	}, policy, "matrix.example.org", "@bot:example.org")
	if room.Outcome != RouteAccepted || room.NormalizedText != "hello" {
		t.Fatalf("room decision = %+v, want accepted normalized hello", room)
	}
}

func TestDecideRouteBlocksUnsupportedAndUngatedRooms(t *testing.T) {
	t.Parallel()

	policy := NormalizeRoutePolicy(RoutePolicy{
		SelectedRooms: []ConversationRoute{{
			ConversationID:     "!room:example.org",
			ConversationType:   ConversationRoom,
			RoomSelectionState: RoomSelected,
			ValidationState:    RoutePolicyValid,
		}},
		ValidationState: RoutePolicyValid,
	}, time.Now())

	encrypted := DecideRoute(InboundEvent{
		HomeserverID:     "matrix.example.org",
		ConversationID:   "!room:example.org",
		MatrixEventID:    "$event1",
		SenderID:         "@alice:example.org",
		ConversationType: ConversationRoom,
		MessageKind:      MessageEncryptedUnsupported,
	}, policy, "matrix.example.org", "@bot:example.org")
	if encrypted.Outcome != RouteUnsupported {
		t.Fatalf("encrypted route outcome = %s, want unsupported", encrypted.Outcome)
	}

	ungated := DecideRoute(InboundEvent{
		HomeserverID:     "matrix.example.org",
		ConversationID:   "!room:example.org",
		MatrixEventID:    "$event2",
		SenderID:         "@alice:example.org",
		ConversationType: ConversationRoom,
		MessageKind:      MessageUnencryptedText,
		Text:             "hello",
	}, policy, "matrix.example.org", "@bot:example.org")
	if ungated.Outcome != RouteIgnored || ungated.ReasonCode != "mention_required" {
		t.Fatalf("ungated route = %+v, want ignored mention_required", ungated)
	}
}
