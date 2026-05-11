package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestChannelManagementListSchemaAcceptsRedactedFixture(t *testing.T) {
	t.Parallel()

	root := schemaRootDir(t)
	validator := contracts.NewValidator(root)
	mustValidateFixtures(t, validator, map[string]string{
		"schemas/api/channel-management-connector-list.response.schema.json": mustReadChannelManagementFixture(t, root, "list-detail.redacted.json"),
	})
}
