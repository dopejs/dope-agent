package slack

import (
	"testing"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestDecideRouteValidatesSlackPolicyAndSurfaces(t *testing.T) {
	t.Parallel()

	policy := RoutePolicy{
		ValidationState:     RoutePolicyValid,
		AllowedDMUsers:      []string{"user_allowed"},
		AllowedDMUserGroups: []string{"group_allowed"},
		SelectedChannels: []ConversationRoute{{
			ConversationID:       "channel_selected",
			ConversationType:     ConversationChannel,
			SelectedChannelState: SelectedChannelSelected,
			ValidationState:      RoutePolicyValid,
		}},
	}
	cases := []struct {
		name   string
		event  InboundEvent
		want   RouteOutcome
		reason string
	}{
		{name: "allowed dm user", event: InboundEvent{WorkspaceID: "workspace", ConversationID: "dm_1", ConversationType: ConversationDirectMessage, MessageID: "m1", SenderID: "user_allowed"}, want: RouteAccepted, reason: "accepted"},
		{name: "allowed dm group", event: InboundEvent{WorkspaceID: "workspace", ConversationID: "dm_2", ConversationType: ConversationDirectMessage, MessageID: "m2", SenderID: "user_other", SenderUserGroupIDs: []string{"group_allowed"}}, want: RouteAccepted, reason: "accepted"},
		{name: "blocked dm", event: InboundEvent{WorkspaceID: "workspace", ConversationID: "dm_3", ConversationType: ConversationDirectMessage, MessageID: "m3", SenderID: "user_other"}, want: RouteBlocked, reason: string(baseconnectors.DiagnosticBlockedRoute)},
		{name: "selected channel mention", event: InboundEvent{WorkspaceID: "workspace", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "m4", Mentioned: true}, want: RouteAccepted, reason: "accepted"},
		{name: "selected channel no mention", event: InboundEvent{WorkspaceID: "workspace", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "m5"}, want: RouteIgnored, reason: "mention_required"},
		{name: "unselected channel", event: InboundEvent{WorkspaceID: "workspace", ConversationID: "channel_other", ConversationType: ConversationChannel, MessageID: "m6", Mentioned: true}, want: RouteBlocked, reason: string(baseconnectors.DiagnosticBlockedRoute)},
		{name: "wrong workspace", event: InboundEvent{WorkspaceID: "workspace_other", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "m7", Mentioned: true}, want: RouteBlocked, reason: string(baseconnectors.DiagnosticBlockedRoute)},
		{name: "unsupported", event: InboundEvent{WorkspaceID: "workspace", ConversationID: "channel_selected", ConversationType: ConversationChannel, MessageID: "m8", Surface: "huddle"}, want: RouteUnsupported, reason: string(baseconnectors.DiagnosticUnsupportedCapability)},
		{name: "missing identity", event: InboundEvent{WorkspaceID: "workspace", ConversationType: ConversationChannel, MessageID: "m9"}, want: RouteFailed, reason: string(baseconnectors.DiagnosticUnknownConnectorFailure)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideRoute(tc.event, policy, "workspace", "bot_1")
			if got.Outcome != tc.want || got.ReasonCode != tc.reason {
				t.Fatalf("decision=%+v, want %s/%s", got, tc.want, tc.reason)
			}
		})
	}
}
