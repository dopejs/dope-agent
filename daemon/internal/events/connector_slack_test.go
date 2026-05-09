package events

import "testing"

func TestConnectorSlackRouteOutcomeRecordedUsesSharedConnectorContract(t *testing.T) {
	t.Parallel()

	event := ConnectorSlackRouteOutcomeRecorded(ConnectorSlackRouteOutcomeRecordedInput{
		TenantID:        "ten_slack",
		ConnectorID:     "slack-main",
		WorkspaceID:     "workspace_redacted",
		ConversationID:  "channel_redacted",
		MessageID:       "message_redacted",
		EventID:         "event_redacted",
		Outcome:         "blocked",
		ReasonCode:      "blocked_route",
		Surface:         "channel",
		RedactionStatus: "redacted",
	})
	if event.Name != "connector.route_outcome_recorded" || event.Resource.Kind != "connector_route_outcome" {
		t.Fatalf("unexpected Slack route outcome event: %+v", event)
	}
	if event.Payload["workspaceId"] != "workspace_redacted" || event.Payload["reasonCode"] != "blocked_route" {
		t.Fatalf("unexpected payload: %+v", event.Payload)
	}
}
