package store

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteStorePersistsSlackSetupRoutePolicySmokeAndEventsTenantSafely(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	setup := SlackHostedSetupRecord{
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		ConnectorKind:      "slack",
		DisplayName:        "Slack Main",
		Status:             "degraded",
		TerminalState:      "action-required",
		OAuthState:         "grant_valid",
		RoutePolicyState:   "valid",
		WorkspaceBindingID: "slack_workspace_binding_1",
		ReasonCode:         "slack_route_policy_missing",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		WorkspaceBinding: &SlackWorkspaceBinding{
			TenantID:           "ten_slack",
			ConnectorID:        "slack-main",
			WorkspaceBindingID: "slack_workspace_binding_1",
			WorkspaceID:        "workspace_redacted",
			WorkspaceLabel:     "Workspace Redacted",
			InstallationID:     "installation_redacted",
			OAuthGrantState:    "valid",
			RequiredScopeState: "valid",
			ValidatedAt:        now,
			RedactionStatus:    "redacted",
			SafeEvidence:       map[string]string{"workspace": "redacted"},
		},
	}
	if err := sqliteStore.SaveSlackHostedSetup(ctx, setup); err != nil {
		t.Fatalf("SaveSlackHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.SaveSlackRoutePolicy(ctx, SlackRoutePolicyRecord{
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		WorkspaceBindingID: "slack_workspace_binding_1",
		SelectedChannels: []SlackConversationRouteRecord{{
			ConversationID:       "channel_redacted",
			ConversationType:     "channel",
			SelectedChannelState: "selected",
			ValidationState:      "valid",
			RedactionStatus:      "redacted",
			SafeEvidence:         map[string]string{"membership": "present"},
		}},
		AllowedDMUsers:      []string{"user_hash_1"},
		AllowedDMUserGroups: []string{"group_hash_1"},
		MentionGate:         "agent_mention_required",
		ThreadReplyMode:     "channel_mentions_thread_rooted",
		ValidationState:     "valid",
		ReasonCode:          "healthy",
		ValidatedAt:         now,
		RedactionStatus:     "redacted",
		SafeEvidence:        map[string]string{"route": "selected_channel_and_dm_allowment"},
	}); err != nil {
		t.Fatalf("SaveSlackRoutePolicy returned error: %v", err)
	}
	if err := sqliteStore.SaveSlackSmokeEvidence(ctx, SlackSmokeEvidenceRecord{
		SmokeEvidenceID:    "slack_smoke_1",
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		WorkspaceBindingID: "slack_workspace_binding_1",
		Status:             "skipped",
		AuthorizationMode:  "unavailable",
		Owner:              "operator",
		Reason:             "safe_slack_authorization_unavailable",
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence:       map[string]string{"policy": "structured_skip"},
	}); err != nil {
		t.Fatalf("SaveSlackSmokeEvidence returned error: %v", err)
	}
	if err := sqliteStore.SaveSlackEventEvidence(ctx, SlackEventEvidenceRecord{
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		WorkspaceID:        "workspace_redacted",
		ConversationID:     "channel_redacted",
		MessageID:          "message_redacted",
		EventID:            "event_redacted",
		RouteOutcome:       "accepted",
		ReasonCode:         "accepted",
		ReceivedAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence:       map[string]string{"identityRule": "slack_workspace_conversation_message_id"},
	}); err != nil {
		t.Fatalf("SaveSlackEventEvidence returned error: %v", err)
	}

	got, ok, err := sqliteStore.GetSlackHostedSetup(ctx, "ten_slack", "slack-main")
	if err != nil || !ok {
		t.Fatalf("GetSlackHostedSetup ok=%v err=%v", ok, err)
	}
	if got.TerminalState != "action-required" || got.RoutePolicy == nil || len(got.RoutePolicy.SelectedChannels) != 1 {
		t.Fatalf("unexpected setup record: %+v", got)
	}
	if got.WorkspaceBinding == nil || got.WorkspaceBinding.WorkspaceID != "workspace_redacted" {
		t.Fatalf("expected workspace binding to round-trip, got %+v", got.WorkspaceBinding)
	}
	if _, ok, err := sqliteStore.GetSlackHostedSetup(ctx, "ten_other", "slack-main"); err != nil || ok {
		t.Fatalf("cross-tenant setup lookup ok=%v err=%v, want not found", ok, err)
	}
	smoke, ok, err := sqliteStore.LatestSlackSmokeEvidence(ctx, "ten_slack", "slack-main", now)
	if err != nil || !ok || smoke.AuthorizationMode != "unavailable" {
		t.Fatalf("LatestSlackSmokeEvidence ok=%v err=%v smoke=%+v", ok, err, smoke)
	}
	events, err := sqliteStore.ListSlackEventEvidence(ctx, "ten_slack", "slack-main", now, 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("ListSlackEventEvidence len=%d err=%v", len(events), err)
	}
	if events[0].SafeEvidence["identityRule"] != "slack_workspace_conversation_message_id" {
		t.Fatalf("unexpected retained event evidence: %+v", events[0])
	}
	otherEvents, err := sqliteStore.ListSlackEventEvidence(ctx, "ten_other", "slack-main", now, 10)
	if err != nil || len(otherEvents) != 0 {
		t.Fatalf("cross-tenant event evidence len=%d err=%v, want none", len(otherEvents), err)
	}
}

func TestSQLiteStorePersistsSlackSetupLifecycleRepairStates(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 8, 11, 0, 0, 0, time.UTC)
	base := SlackHostedSetupRecord{
		TenantID:           "ten_slack_lifecycle",
		ConnectorID:        "slack-main",
		ConnectorKind:      "slack",
		DisplayName:        "Slack Main",
		Status:             "degraded",
		TerminalState:      "action-required",
		OAuthState:         "grant_missing",
		RoutePolicyState:   "none",
		WorkspaceBindingID: "slack_workspace_binding_lifecycle",
		ReasonCode:         "auth_missing",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	for _, state := range []struct {
		name          string
		terminalState string
		reason        string
	}{
		{name: "retry", terminalState: "action-required", reason: "auth_missing"},
		{name: "replacement", terminalState: "action-required", reason: "workspace_mismatch"},
		{name: "cancellation", terminalState: "cancelled", reason: "user_cancelled"},
		{name: "disablement", terminalState: "cancelled", reason: "disabled_by_user"},
	} {
		record := base
		record.TerminalState = state.terminalState
		record.ReasonCode = state.reason
		record.UpdatedAt = now.Add(time.Duration(len(state.name)) * time.Minute)
		if err := sqliteStore.SaveSlackHostedSetup(ctx, record); err != nil {
			t.Fatalf("SaveSlackHostedSetup %s returned error: %v", state.name, err)
		}
		got, ok, err := sqliteStore.GetSlackHostedSetup(ctx, "ten_slack_lifecycle", "slack-main")
		if err != nil || !ok {
			t.Fatalf("GetSlackHostedSetup %s ok=%v err=%v", state.name, ok, err)
		}
		if got.TerminalState != state.terminalState || got.ReasonCode != state.reason || got.RetentionExpiresAt.IsZero() {
			t.Fatalf("unexpected lifecycle state after %s: %+v", state.name, got)
		}
	}
}
