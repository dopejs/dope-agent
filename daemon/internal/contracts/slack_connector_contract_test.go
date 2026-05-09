package contracts_test

import (
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func slackConnectorContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/slack-hosted-setup-resource.schema.json":              `{"tenantId":"ten_slack","connectorId":"slack-main","connectorKind":"slack","displayName":"Slack Main","status":"degraded","terminalState":"action-required","oauthState":"scope_missing","routePolicyState":"valid","deliveryEligible":false,"workspaceBindingId":"slack_workspace_binding_1","redactionStatus":"redacted","createdAt":"2026-05-08T10:00:00Z","updatedAt":"2026-05-08T10:01:00Z","validatedAt":"2026-05-08T10:01:00Z","retentionExpiresAt":"2026-08-06T10:01:00Z","workspaceBinding":{"workspaceId":"workspace_redacted","workspaceLabel":"Workspace redacted","installationId":"installation_redacted","oauthGrantState":"scope_missing","requiredScopeState":"missing","validatedAt":"2026-05-08T10:01:00Z","redactionStatus":"redacted"},"routePolicy":{"tenantId":"ten_slack","connectorId":"slack-main","workspaceBindingId":"slack_workspace_binding_1","selectedChannels":[{"conversationId":"channel_redacted","conversationType":"channel","selectedChannelState":"selected","validationState":"valid","redactionStatus":"redacted"}],"allowedDMUsers":["user_hash_1"],"allowedDMUserGroups":["group_hash_1"],"mentionGate":"agent_mention_required","threadReplyMode":"channel_mentions_thread_rooted","validationState":"valid","validatedAt":"2026-05-08T10:01:00Z","redactionStatus":"redacted"},"diagnostic":{"reasonCode":"permission_missing","slackCondition":"missing_scope","remediationOwner":"tenant_admin","freshnessState":"fresh"}}`,
		"schemas/api/slack-route-policy-resource.schema.json":              `{"tenantId":"ten_slack","connectorId":"slack-main","workspaceBindingId":"slack_workspace_binding_1","selectedChannels":[{"conversationId":"channel_redacted","conversationType":"channel","selectedChannelState":"selected","validationState":"valid","redactionStatus":"redacted"}],"allowedDMUsers":["user_hash_1"],"allowedDMUserGroups":["group_hash_1"],"mentionGate":"agent_mention_required","threadReplyMode":"channel_mentions_thread_rooted","validationState":"valid","validatedAt":"2026-05-08T10:01:00Z","redactionStatus":"redacted","safeEvidence":{"route":"selected_channel_and_dm_allowment"}}`,
		"schemas/api/slack-smoke-evidence-resource.schema.json":            `{"smokeEvidenceId":"slack_smoke_1","tenantId":"ten_slack","connectorId":"slack-main","workspaceBindingId":"slack_workspace_binding_1","status":"skipped","authorizationMode":"unavailable","owner":"operator","reason":"safe_slack_authorization_unavailable","remainingRisk":"No live Slack hosted smoke was run in this release validation.","validatedAt":"2026-05-08T10:02:00Z","retentionExpiresAt":"2026-08-06T10:02:00Z","redactionStatus":"redacted","safeEvidence":{"policy":"structured_skip"}}`,
		"schemas/events/connector-slack-setup-validated.event.schema.json": `{"eventId":"evt_slack_setup_1","sequence":1,"category":"connector","name":"connector.slack_setup_validated","occurredAt":"2026-05-08T10:01:00Z","scope":{"connectorId":"slack-main"},"resource":{"kind":"slack_hosted_setup","id":"slack-main"},"payload":{"tenantId":"ten_slack","connectorId":"slack-main","workspaceBindingId":"slack_workspace_binding_1","terminalState":"action-required","oauthState":"scope_missing","routePolicyState":"valid","deliveryEligible":false,"reasonCode":"permission_missing","slackCondition":"missing_scope","redactionStatus":"redacted","validatedAt":"2026-05-08T10:01:00Z"}}`,
	}
}

func TestSlackConnectorSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, slackConnectorContractFixtures())
}

func TestSlackConnectorFixturesDoNotLeakCredentialMarkers(t *testing.T) {
	t.Parallel()

	for name, fixture := range slackConnectorContractFixtures() {
		lower := strings.ToLower(fixture)
		for _, marker := range []string{"xoxb-", "bot token", "signing secret", "authorization header", "rawproviderpayload", "slack-secret"} {
			if strings.Contains(lower, marker) {
				t.Fatalf("%s leaked sensitive marker %q: %s", name, marker, fixture)
			}
		}
	}
}
