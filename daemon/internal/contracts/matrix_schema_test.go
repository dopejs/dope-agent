package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestMatrixSchemaFixturesLoadIndividually(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	for name, fixture := range matrixConnectorContractFixtures() {
		if err := validator.ValidateRelative(name, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s) returned error: %v", name, err)
		}
	}
}
