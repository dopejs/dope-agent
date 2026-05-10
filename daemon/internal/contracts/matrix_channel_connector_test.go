package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func matrixConnectorContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/matrix-hosted-setup-resource.schema.json":              `{"tenantId":"ten_matrix","connectorId":"matrix-main","connectorKind":"matrix","displayName":"Matrix Main","status":"degraded","terminalState":"action-required","botCredentialState":"valid","homeserverState":"reachable","routePolicyState":"valid","deliveryEligible":false,"homeserverBindingId":"matrix_hs_1","redactionStatus":"redacted","createdAt":"2026-05-10T10:00:00Z","updatedAt":"2026-05-10T10:01:00Z","validatedAt":"2026-05-10T10:01:00Z","retentionExpiresAt":"2026-08-08T10:01:00Z","homeserverBinding":{"homeserverUrl":"https://matrix.example.org","botUserId":"@bot:example.org","authorizationState":"valid","homeserverCapabilityState":"valid","validatedAt":"2026-05-10T10:01:00Z","redactionStatus":"redacted"},"routePolicy":{"tenantId":"ten_matrix","connectorId":"matrix-main","homeserverBindingId":"matrix_hs_1","selectedRooms":[{"conversationId":"!room:example.org","conversationType":"room","roomSelectionState":"selected","validationState":"valid","redactionStatus":"redacted"}],"allowedDirectUsers":["@alice:example.org"],"roomInvocationGate":"bot_mention_or_command_required","configuredCommands":["!dope"],"encryptedRoomPolicy":"unsupported","validationState":"valid","validatedAt":"2026-05-10T10:01:00Z","redactionStatus":"redacted"},"diagnostic":{"reasonCode":"blocked_route","matrixCondition":"blocked_route","remediationOwner":"tenant_admin","freshnessState":"fresh"}}`,
		"schemas/api/matrix-route-policy-resource.schema.json":              `{"tenantId":"ten_matrix","connectorId":"matrix-main","homeserverBindingId":"matrix_hs_1","selectedRooms":[{"conversationId":"!room:example.org","conversationType":"room","roomSelectionState":"selected","validationState":"valid","redactionStatus":"redacted"}],"allowedDirectUsers":["@alice:example.org"],"roomInvocationGate":"bot_mention_or_command_required","configuredCommands":["!dope"],"encryptedRoomPolicy":"unsupported","validationState":"valid","validatedAt":"2026-05-10T10:01:00Z","redactionStatus":"redacted","safeEvidence":{"route":"selected_room_and_direct_allowment"}}`,
		"schemas/api/matrix-smoke-evidence-resource.schema.json":            `{"smokeEvidenceId":"matrix_smoke_1","tenantId":"ten_matrix","connectorId":"matrix-main","homeserverBindingId":"matrix_hs_1","status":"skipped","authorizationMode":"unavailable","owner":"operator","reason":"safe Matrix credentials unavailable","remainingRisk":"No live Matrix smoke was run.","validatedAt":"2026-05-10T10:02:00Z","retentionExpiresAt":"2026-08-08T10:02:00Z","redactionStatus":"redacted","safeEvidence":{"policy":"structured_skip"}}`,
		"schemas/events/connector-matrix-setup-validated.event.schema.json": `{"eventId":"evt_matrix_setup_1","sequence":1,"category":"connector","name":"connector.matrix_setup_validated","occurredAt":"2026-05-10T10:01:00Z","scope":{"connectorId":"matrix-main"},"resource":{"kind":"matrix_hosted_setup","id":"matrix-main"},"payload":{"tenantId":"ten_matrix","connectorId":"matrix-main","homeserverBindingId":"matrix_hs_1","terminalState":"action-required","botCredentialState":"valid","routePolicyState":"valid","deliveryEligible":false,"reasonCode":"blocked_route","matrixCondition":"blocked_route","redactionStatus":"redacted","validatedAt":"2026-05-10T10:01:00Z"}}`,
	}
}

func TestMatrixConnectorSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, matrixConnectorContractFixtures())
}

func TestMatrixConnectorFixturesDoNotLeakCredentialMarkers(t *testing.T) {
	t.Parallel()

	for name, fixture := range matrixConnectorContractFixtures() {
		lower := strings.ToLower(fixture)
		for _, marker := range []string{"access_token", "bearer ", "authorization header", "rawproviderpayload", "matrix-secret", "mxc://secret"} {
			if strings.Contains(lower, marker) {
				t.Fatalf("%s leaked sensitive marker %q: %s", name, marker, fixture)
			}
		}
	}
}

func TestMatrixPhase52HandoffDocumentStaysAligned(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join(schemaRootDir(t), "docs/channels/channel-connector-conformance.md"))
	if err != nil {
		t.Fatalf("read channel conformance doc: %v", err)
	}
	doc := string(body)
	required := []string{
		"## Matrix Phase 52 Handoff",
		"phase 52 chooses Matrix and explicitly rejects WhatsApp fallback",
		"tenant-provided Matrix bot credential",
		"bot mention or configured command",
		"encrypted rooms",
		"undecryptable events",
		"thinking visibility",
		"incremental visible updates",
		"safe-live Matrix smoke evidence",
	}
	for _, needle := range required {
		if !strings.Contains(doc, needle) {
			t.Fatalf("Matrix phase 52 handoff doc missing %q", needle)
		}
	}
}

func TestMatrixPlanningContractKeepsProviderDecisionAndUnsupportedRows(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join(schemaRootDir(t), "specs/037-whatsapp-matrix-channel/contracts/matrix-channel-connector.md"))
	if err != nil {
		t.Fatalf("read Matrix planning contract: %v", err)
	}
	contract := string(body)
	required := []string{
		"| Provider selection | Matrix is the chosen provider and WhatsApp is rejected for phase 52",
		"| Unsupported setup modes | DopeAgent-hosted homeserver provisioning, Matrix account provisioning, local-only sessions, bridge automation, and unsupported unofficial automation are rejected",
		"WhatsApp, bridge automation, broad media, voice, calls, reactions, memory-based",
		"Encrypted rooms and undecryptable events: unsupported.",
		"E2EE key/session management: unsupported.",
		"Thinking visibility: unsupported unless explicitly recut.",
		"Incremental visible updates: unsupported unless explicitly recut.",
		"GET /v1/connectors/{connectorId}/matrix-setup",
		"GET /v1/connectors/{connectorId}/matrix-smoke",
		"getMatrixSetup",
		"getMatrixSmokeEvidence",
	}
	for _, needle := range required {
		if !strings.Contains(contract, needle) {
			t.Fatalf("Matrix planning contract missing %q", needle)
		}
	}
}
