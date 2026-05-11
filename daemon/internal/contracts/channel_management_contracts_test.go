package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestChannelManagementSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	root := schemaRootDir(t)
	listFixture := mustReadChannelManagementFixture(t, root, "list-detail.redacted.json")
	supportFixture := mustReadChannelManagementFixture(t, root, "support-evidence.retention-redacted.json")
	validator := contracts.NewValidator(root)
	mustValidateFixtures(t, validator, map[string]string{
		"schemas/api/channel-management-connector-list.response.schema.json": listFixture,
		"schemas/api/channel-management-support-evidence.schema.json":        supportFixture,
		"schemas/api/channel-management-action.schema.json":                  `{"actionKind":"reconnect","reasonCode":"permission_missing","sourceDiagnosticStateId":"diag_slack_permission"}`,
		"schemas/events/connector-management.event.schema.json":              `{"eventId":"evt_channel_management_1","sequence":1,"category":"connector","name":"connector.management_support_evidence_generated","occurredAt":"2026-05-10T10:00:00Z","scope":{"connectorId":"slack-main"},"resource":{"kind":"channel_support_evidence","id":"support_slack_main"},"payload":{"tenantId":"ten_channel_management","connectorId":"slack-main","action":"support_evidence","outcome":"succeeded","evidenceId":"support_slack_main","redactionStatus":"redacted"}}`,
	})
}

func TestChannelManagementFixturesRejectUnsafeChannelMaterial(t *testing.T) {
	t.Parallel()

	root := schemaRootDir(t)
	for _, name := range []string{
		"list-detail.redacted.json",
		"mutation-denied.audit-failed.json",
		"support-evidence.retention-redacted.json",
	} {
		body := strings.ToLower(mustReadChannelManagementFixture(t, root, name))
		for _, forbidden := range []string{
			"access_token",
			"refresh_token",
			"bearer ",
			"bot_token",
			"client_secret",
			"message body:",
			"raw payload:",
			"raw_provider_payload:",
			"authorization code",
			"oauth_verifier",
		} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s contains forbidden unsafe marker %q: %s", name, forbidden, body)
			}
		}
	}
}

func mustReadChannelManagementFixture(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "daemon/internal/contracts/testdata/channel-management", name))
	if err != nil {
		t.Fatalf("read channel management fixture %s: %v", name, err)
	}
	return string(data)
}
