package telegram

import "testing"

func TestRouteDecisionEnforcesExplicitDirectAllowment(t *testing.T) {
	t.Parallel()

	allowed := NewAllowmentIndex([]AllowmentValidation{{
		ScopeType:       ScopeDirectChat,
		ScopeID:         "chat_1",
		Enabled:         true,
		ValidationState: AllowmentValid,
	}})
	if got := DecideRoute(InboundUpdate{ConversationType: ConversationDirect, ChatID: "chat_1", SenderID: "user_1", Text: "hello"}, allowed); got.Outcome != RouteAccepted {
		t.Fatalf("allowed direct chat outcome=%s reason=%s, want accepted", got.Outcome, got.ReasonCode)
	}
	if got := DecideRoute(InboundUpdate{ConversationType: ConversationDirect, ChatID: "chat_2", SenderID: "user_2", Text: "hello"}, allowed); got.Outcome != RouteBlocked || got.ReasonCode != "blocked_route" {
		t.Fatalf("unknown direct chat outcome=%s reason=%s, want blocked route", got.Outcome, got.ReasonCode)
	}
}

func TestRouteDecisionRequiresGroupAllowmentAndMentionOrCommand(t *testing.T) {
	t.Parallel()

	allowed := NewAllowmentIndex([]AllowmentValidation{{
		ScopeType:       ScopeGroup,
		ScopeID:         "group_1",
		Enabled:         true,
		GroupGate:       GroupGateMentionOrCommandRequired,
		ValidationState: AllowmentValid,
	}})
	if got := DecideRoute(InboundUpdate{ConversationType: ConversationGroup, ChatID: "group_1", Text: "ordinary chatter"}, allowed); got.Outcome != RouteIgnored || got.ReasonCode != "mention_required" {
		t.Fatalf("group without mention outcome=%s reason=%s, want ignored mention_required", got.Outcome, got.ReasonCode)
	}
	if got := DecideRoute(InboundUpdate{ConversationType: ConversationGroup, ChatID: "group_1", Text: "/ask status", Command: true}, allowed); got.Outcome != RouteAccepted {
		t.Fatalf("group command outcome=%s reason=%s, want accepted", got.Outcome, got.ReasonCode)
	}
	if got := DecideRoute(InboundUpdate{ConversationType: ConversationGroup, ChatID: "group_2", Text: "@bot status", Mentioned: true}, allowed); got.Outcome != RouteBlocked {
		t.Fatalf("unallowed group outcome=%s reason=%s, want blocked", got.Outcome, got.ReasonCode)
	}
}

func TestRouteDecisionRejectsUnsupportedAndMissingIdentity(t *testing.T) {
	t.Parallel()

	allowed := NewAllowmentIndex([]AllowmentValidation{{ScopeType: ScopeDirectChat, ScopeID: "chat_1", Enabled: true, ValidationState: AllowmentValid}})
	if got := DecideRoute(InboundUpdate{ConversationType: ConversationDirect, ChatID: "chat_1", UnsupportedSurface: "voice", Text: "voice"}, allowed); got.Outcome != RouteUnsupported {
		t.Fatalf("unsupported outcome=%s reason=%s, want unsupported", got.Outcome, got.ReasonCode)
	}
	if got := DecideRoute(InboundUpdate{ConversationType: ConversationDirect, Text: "missing chat"}, allowed); got.Outcome != RouteFailed || got.ReasonCode != "missing_durable_identity" {
		t.Fatalf("missing identity outcome=%s reason=%s, want failed", got.Outcome, got.ReasonCode)
	}
}
