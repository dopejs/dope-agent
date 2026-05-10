package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestMatrixRouteAndDeliveryContractsAcceptRedactedEvidence(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	for name, fixture := range map[string]string{
		"schemas/api/matrix-route-policy-resource.schema.json":              matrixConnectorContractFixtures()["schemas/api/matrix-route-policy-resource.schema.json"],
		"schemas/api/matrix-smoke-evidence-resource.schema.json":            matrixConnectorContractFixtures()["schemas/api/matrix-smoke-evidence-resource.schema.json"],
		"schemas/events/connector-route-outcome-recorded.event.schema.json": `{"eventId":"evt_matrix_route_1","sequence":1,"category":"connector","name":"connector.route_outcome_recorded","occurredAt":"2026-05-10T10:03:00Z","scope":{"connectorId":"matrix-main"},"resource":{"kind":"connector_route_outcome","id":"$event"},"payload":{"tenantId":"ten_matrix","connectorId":"matrix-main","homeserverId":"example.org","conversationId":"!room:example.org","matrixEventId":"$event","outcome":"accepted","reasonCode":"accepted","surface":"room","redactionStatus":"redacted"}}`,
	} {
		if err := validator.ValidateRelative(name, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s) returned error: %v", name, err)
		}
	}
}
