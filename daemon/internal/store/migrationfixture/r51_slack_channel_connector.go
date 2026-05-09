package migrationfixture

import (
	"context"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

var R51SlackChannelConnectorTableNames = []string{
	"slack_hosted_setups",
	"slack_route_policies",
	"slack_smoke_evidence",
	"slack_event_evidence",
}

type R51SlackChannelConnectorFixture struct {
	TenantIDs        []string
	ExpectedRowCount map[string]int
}

func BuildR51SlackChannelConnectorFixture() R51SlackChannelConnectorFixture {
	return R51SlackChannelConnectorFixture{
		TenantIDs: []string{"ten_slack_alpha", "ten_slack_beta"},
		ExpectedRowCount: map[string]int{
			"slack_hosted_setups":  2,
			"slack_route_policies": 2,
			"slack_smoke_evidence": 2,
			"slack_event_evidence": 2,
		},
	}
}

func SeedR51SlackChannelConnectorRows(ctx context.Context, s *store.SQLiteStore) (R51SlackChannelConnectorFixture, error) {
	fixture := BuildR51SlackChannelConnectorFixture()
	for i, tenantID := range fixture.TenantIDs {
		suffix := fmt.Sprintf("%d", i+1)
		connectorID := "slack-r51-" + suffix
		workspaceBindingID := "slack_workspace_binding_" + suffix
		if err := s.SaveSlackHostedSetup(ctx, store.SlackHostedSetupRecord{
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			ConnectorKind:      "slack",
			DisplayName:        "Slack R51",
			Status:             "degraded",
			TerminalState:      "action-required",
			OAuthState:         "grant_valid",
			RoutePolicyState:   "none",
			WorkspaceBindingID: workspaceBindingID,
			ReasonCode:         "blocked_route",
			RedactionStatus:    "redacted",
			CreatedAt:          mustR51FixtureTime(ts),
			UpdatedAt:          mustR51FixtureTime(ts),
			ValidatedAt:        mustR51FixtureTime(ts),
			RetentionExpiresAt: mustR51FixtureTime(ts),
		}); err != nil {
			return fixture, err
		}
		if err := s.SaveSlackRoutePolicy(ctx, store.SlackRoutePolicyRecord{
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			WorkspaceBindingID: workspaceBindingID,
			SelectedChannels: []store.SlackConversationRouteRecord{{
				ConversationID:       "channel_" + suffix,
				ConversationType:     "channel",
				SelectedChannelState: "selected",
				ValidationState:      "valid",
				RedactionStatus:      "redacted",
			}},
			AllowedDMUsers:      []string{"user_" + suffix},
			AllowedDMUserGroups: []string{"group_" + suffix},
			MentionGate:         "agent_mention_required",
			ThreadReplyMode:     "channel_mentions_thread_rooted",
			ValidationState:     "valid",
			ReasonCode:          "healthy",
			ValidatedAt:         mustR51FixtureTime(ts),
			RedactionStatus:     "redacted",
			SafeEvidence:        map[string]string{"scope": "selected_channel_and_dm"},
		}); err != nil {
			return fixture, err
		}
		if err := s.SaveSlackSmokeEvidence(ctx, store.SlackSmokeEvidenceRecord{
			SmokeEvidenceID:    "slack_smoke_" + suffix,
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			WorkspaceBindingID: workspaceBindingID,
			Status:             "skipped",
			AuthorizationMode:  "unavailable",
			Owner:              "operator",
			Reason:             "safe_slack_authorization_unavailable",
			RemainingRisk:      "live smoke skipped",
			ValidatedAt:        mustR51FixtureTime(ts),
			RetentionExpiresAt: mustR51FixtureTime(ts),
			RedactionStatus:    "redacted",
			SafeEvidence:       map[string]string{"policy": "structured_skip"},
		}); err != nil {
			return fixture, err
		}
		if err := s.SaveSlackEventEvidence(ctx, store.SlackEventEvidenceRecord{
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			WorkspaceID:        "workspace_" + suffix,
			ConversationID:     "channel_" + suffix,
			MessageID:          "message_" + suffix,
			EventID:            "event_" + suffix,
			RouteOutcome:       "accepted",
			ReasonCode:         "accepted",
			ReceivedAt:         mustR51FixtureTime(ts),
			RetentionExpiresAt: mustR51FixtureTime(ts),
			RedactionStatus:    "redacted",
			SafeEvidence:       map[string]string{"identityRule": "slack_workspace_conversation_message_id"},
		}); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

func CountR51SlackChannelConnectorRows(ctx context.Context, s *store.SQLiteStore) (map[string]int, error) {
	counts := map[string]int{}
	for _, table := range R51SlackChannelConnectorTableNames {
		var count int
		if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}

func mustR51FixtureTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return parsed
}
