package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestMatrixSetupContractAcceptsRedactedTerminalStates(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixture := `{"connectorId":"matrix-main","connectorKind":"matrix","displayName":"Matrix Main","status":"degraded","terminalState":"action-required","botCredentialState":"invalid","homeserverState":"reachable","routePolicyState":"blocked","deliveryEligible":false,"homeserverBindingId":"matrix_hs_1","reasonCode":"bot_auth_invalid","redactionStatus":"redacted","createdAt":"2026-05-10T10:00:00Z","updatedAt":"2026-05-10T10:01:00Z","retentionExpiresAt":"2026-08-08T10:01:00Z","diagnostic":{"reasonCode":"auth_missing","matrixCondition":"bot_auth_invalid","remediationOwner":"tenant_admin","freshnessState":"fresh"}}`
	if err := validator.ValidateRelative("schemas/api/matrix-hosted-setup-resource.schema.json", []byte(fixture)); err != nil {
		t.Fatalf("ValidateRelative(matrix setup) returned error: %v", err)
	}
}
