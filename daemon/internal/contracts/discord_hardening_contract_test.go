package contracts_test

import (
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func discordHardeningContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/discord-hosted-setup-resource.schema.json":              `{"tenantId":"ten_discord","connectorId":"discord-main","connectorKind":"discord","displayName":"Discord Main","status":"degraded","readinessState":"degraded_needs_repair","hostedReady":false,"credentialState":"valid","respondInDM":true,"requireMention":true,"deliveryMode":"gateway","reasonCode":"destination_validation_failed","redactionStatus":"redacted","createdAt":"2026-05-07T10:00:00Z","updatedAt":"2026-05-07T10:01:00Z","validatedAt":"2026-05-07T10:01:00Z","retentionExpiresAt":"2026-08-05T10:01:00Z","destinations":[{"tenantId":"ten_discord","connectorId":"discord-main","destinationId":"channel_redacted","destinationType":"channel","selected":true,"validationState":"missing_permission","reasonCode":"permission_missing","validatedAt":"2026-05-07T10:01:00Z","redactionStatus":"redacted","safeEvidence":{"permission":"send_messages"}}]}`,
		"schemas/api/discord-destination-validation-resource.schema.json":    `{"tenantId":"ten_discord","connectorId":"discord-main","destinationId":"guild_redacted","destinationType":"guild","providerLabel":"Guild redacted","selected":true,"validationState":"valid","reasonCode":"healthy","validatedAt":"2026-05-07T10:01:00Z","redactionStatus":"redacted","safeEvidence":{"validation":"bot_member"}}`,
		"schemas/api/discord-smoke-evidence-resource.schema.json":            `{"smokeEvidenceId":"discord_smoke_1","tenantId":"ten_discord","connectorId":"discord-main","status":"skipped","credentialMode":"unavailable","owner":"operator","reason":"safe_credentials_unavailable","remainingRisk":"No live Discord hosted smoke was run in this release validation.","validatedAt":"2026-05-07T10:02:00Z","retentionExpiresAt":"2026-08-05T10:02:00Z","redactionStatus":"redacted","safeEvidence":{"policy":"structured_skip"}}`,
		"schemas/events/connector-discord-setup-validated.event.schema.json": `{"eventId":"evt_discord_setup_1","sequence":1,"category":"connector","name":"connector.discord_setup_validated","occurredAt":"2026-05-07T10:01:00Z","scope":{"connectorId":"discord-main"},"resource":{"kind":"discord_hosted_setup","id":"discord-main"},"payload":{"tenantId":"ten_discord","connectorId":"discord-main","readinessState":"degraded_needs_repair","hostedReady":false,"credentialState":"valid","reasonCode":"destination_validation_failed","redactionStatus":"redacted","validatedAt":"2026-05-07T10:01:00Z"}}`,
		"schemas/events/connector-reply-failed.event.schema.json":            `{"eventId":"evt_discord_reply_failed_1","sequence":2,"category":"connector","name":"connector.reply_failed","occurredAt":"2026-05-07T10:03:00Z","scope":{"connectorId":"discord-main","runId":"run_1"},"resource":{"kind":"connector","id":"discord-main"},"payload":{"tenantId":"ten_discord","connectorId":"discord-main","messageId":"discord_msg_1","replyMessageId":"discord_reply_1","assistantExecutionOutcome":"succeeded","discordDeliveryOutcome":"failed","errorClass":"network_failed","reasonCode":"reply_failed","redactionStatus":"redacted"}}`,
	}
}

func TestDiscordHardeningSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, discordHardeningContractFixtures())
}

func TestDiscordHardeningFixturesDoNotLeakCredentialMarkers(t *testing.T) {
	t.Parallel()

	for name, fixture := range discordHardeningContractFixtures() {
		lower := strings.ToLower(fixture)
		for _, marker := range []string{"bot token", "authorization", "rawproviderpayload", "discord-secret"} {
			if strings.Contains(lower, marker) {
				t.Fatalf("%s leaked sensitive marker %q: %s", name, marker, fixture)
			}
		}
	}
}
