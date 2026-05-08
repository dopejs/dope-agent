package contracts_test

import (
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func telegramConnectorContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/telegram-hosted-setup-resource.schema.json":              `{"tenantId":"ten_telegram","connectorId":"telegram-main","connectorKind":"telegram","displayName":"Telegram Main","status":"healthy","terminalState":"ready","hostedReady":true,"credentialState":"valid","allowmentState":"valid","groupBehavior":"mention_or_command_required","deliveryEligible":true,"reasonCode":"healthy","redactionStatus":"redacted","createdAt":"2026-05-08T10:00:00Z","updatedAt":"2026-05-08T10:01:00Z","validatedAt":"2026-05-08T10:01:00Z","retentionExpiresAt":"2026-08-06T10:01:00Z","allowments":[{"tenantId":"ten_telegram","connectorId":"telegram-main","allowmentId":"allow_dm","telegramScopeType":"direct_chat","telegramScopeId":"chat_redacted","enabled":true,"groupGate":"not_applicable","validationState":"valid","reasonCode":"healthy","validatedAt":"2026-05-08T10:01:00Z","redactionStatus":"redacted","safeEvidence":{"scope":"direct_chat"}}]}`,
		"schemas/api/telegram-allowment-resource.schema.json":                 `{"tenantId":"ten_telegram","connectorId":"telegram-main","allowmentId":"allow_group","telegramScopeType":"group","telegramScopeId":"group_redacted","providerLabel":"Telegram group","enabled":true,"groupGate":"mention_or_command_required","validationState":"valid","reasonCode":"healthy","validatedAt":"2026-05-08T10:01:00Z","redactionStatus":"redacted","safeEvidence":{"gate":"mention_or_command_required"}}`,
		"schemas/api/telegram-smoke-evidence-resource.schema.json":            `{"smokeEvidenceId":"telegram_smoke_1","tenantId":"ten_telegram","connectorId":"telegram-main","status":"passed","credentialMode":"fake","owner":"operator","reason":"healthy","remainingRisk":"live provider not exercised","validatedAt":"2026-05-08T10:02:00Z","retentionExpiresAt":"2026-08-06T10:02:00Z","redactionStatus":"redacted","safeEvidence":{"transport":"fake"}}`,
		"schemas/api/record-telegram-smoke.request.schema.json":               `{"connectorId":"telegram-main","status":"skipped","credentialMode":"unavailable","owner":"operator","reason":"safe_credentials_unavailable","remainingRisk":"No safe live Telegram credential was available for this release validation.","safeEvidence":{"policy":"structured_skip"}}`,
		"schemas/events/connector-telegram-setup-validated.event.schema.json": `{"eventId":"evt_telegram_setup_1","sequence":1,"category":"connector","name":"connector.telegram_setup_validated","occurredAt":"2026-05-08T10:01:00Z","scope":{"connectorId":"telegram-main"},"resource":{"kind":"telegram_hosted_setup","id":"telegram-main"},"payload":{"tenantId":"ten_telegram","connectorId":"telegram-main","terminalState":"ready","hostedReady":true,"credentialState":"valid","allowmentState":"valid","reasonCode":"healthy","redactionStatus":"redacted","validatedAt":"2026-05-08T10:01:00Z"}}`,
	}
}

func TestTelegramConnectorSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, telegramConnectorContractFixtures())
}

func TestTelegramConnectorFixturesDoNotLeakCredentialMarkers(t *testing.T) {
	t.Parallel()

	for name, fixture := range telegramConnectorContractFixtures() {
		lower := strings.ToLower(fixture)
		for _, marker := range []string{"bot token", "authorization", "rawproviderpayload", "123:secret", "telegram-secret"} {
			if strings.Contains(lower, marker) {
				t.Fatalf("%s leaked sensitive marker %q: %s", name, marker, fixture)
			}
		}
	}
}
