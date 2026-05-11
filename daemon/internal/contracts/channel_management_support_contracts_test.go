package contracts_test

import (
	"strings"
	"testing"
)

func TestChannelManagementSupportFixtureRejectsRawChannelMaterial(t *testing.T) {
	t.Parallel()

	body := strings.ToLower(mustReadChannelManagementFixture(t, schemaRootDir(t), "support-evidence.retention-redacted.json"))
	for _, forbidden := range []string{"access_token", "refresh_token", "bearer ", "message body:", "raw payload:", "client_secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("support fixture contains forbidden marker %q: %s", forbidden, body)
		}
	}
}
