package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func hostedCredentialContractFixtures() map[string]string {
	return map[string]string{
		"schemas/api/tenant-secret-resource.schema.json":             `{"secretId":"sec_1","tenantId":"ten_1","secretRef":"provider/api-key","displayName":"Provider API key","status":"active","activeVersionId":"secver_1","createdAt":"2026-04-24T10:00:00Z","updatedAt":"2026-04-24T10:00:00Z","rotatedAt":"2026-04-24T10:00:00Z","document":{"owner":"ops"},"secretRefs":[{"secretRef":"provider/api-key","resolution":"unavailable","redactionRule":"secret_ref_only"}]}`,
		"schemas/api/tenant-secret-list.response.schema.json":        `{"items":[{"secretId":"sec_1","tenantId":"ten_1","secretRef":"provider/api-key","status":"active","createdAt":"2026-04-24T10:00:00Z","updatedAt":"2026-04-24T10:00:00Z"}]}`,
		"schemas/api/connector-resource.schema.json":                 `{"tenantId":"ten_1","connectorId":"discord","kind":"discord","displayName":"Discord","status":"healthy","secretRefs":["discord/token"],"secretSummary":[{"secretRef":"discord/token","resolution":"unavailable","redactionRule":"secret_ref_only"}],"failureCount":0,"restartCount":0,"backoffSeconds":0,"createdAt":"2026-04-24T10:00:00Z","updatedAt":"2026-04-24T10:00:00Z"}`,
		"schemas/api/mcp-server-resource.schema.json":                `{"tenantId":"ten_1","serverId":"srv_1","displayName":"GitHub","source":"api","enabled":true,"sandboxProfileId":"default","declarationId":"decl_1","declaration":{"executionMode":"subprocess","active":true},"transportKind":"stdio","command":"node","args":["server.js"],"secretRefs":["github/token"],"autoRestart":true,"createdAt":"2026-04-24T10:00:00Z","updatedAt":"2026-04-24T10:00:00Z","state":{"serverId":"srv_1","status":"healthy","failureCount":0,"restartCount":0,"updatedAt":"2026-04-24T10:00:00Z"},"transportConfigSummary":"stdio:node","secretSummary":[{"consumerId":"srv_1","secretRef":"github/token","environmentScope":"test","resolution":"unavailable","redactionRule":"secret_ref_only"}],"toolCount":0}`,
		"schemas/api/create-tenant-secret.request.schema.json":       `{"secretRef":"provider/api-key","displayName":"Provider API key","value":"fake-secret","document":{"owner":"ops"}}`,
		"schemas/api/rotate-tenant-secret.request.schema.json":       `{"value":"fake-new-secret"}`,
		"schemas/api/rotate-tenant-secret.response.schema.json":      `{"secret":{"secretId":"sec_1","tenantId":"ten_1","secretRef":"provider/api-key","status":"active","activeVersionId":"secver_2","createdAt":"2026-04-24T10:00:00Z","updatedAt":"2026-04-24T10:01:00Z","rotatedAt":"2026-04-24T10:01:00Z"}}`,
		"schemas/api/provider-auth-state.response.schema.json":       `{"auth":{"tenantId":"ten_1","providerId":"codex","family":"codex_cli","authMode":"local_cli_bridge","status":"authenticated","cliAvailable":true,"accountLabel":"operator","accountId":"acct_1","lastCheckedAt":"2026-04-24T10:00:00Z","secretRefs":[{"secretRef":"provider/codex","resolution":"unavailable","redactionRule":"secret_ref_only"}]}}`,
		"schemas/events/credential-audit-recorded.event.schema.json": `{"eventId":"evt_credential_1","sequence":1,"category":"tenant","name":"credential.audit_recorded","occurredAt":"2026-04-24T10:00:00Z","scope":{},"resource":{"kind":"tenant_secret","id":"sec_1"},"payload":{"tenantId":"ten_1","principalId":"prn_1","resourceKind":"tenant_secret","resourceId":"sec_1","action":"secret.rotate","outcome":"succeeded","reasonCode":"credential_rotated","secretRef":"provider/api-key","secretVersionId":"secver_2","secretRefCount":1,"secretRefs":[{"secretRef":"provider/api-key","resolution":"unavailable","redactionRule":"secret_ref_only"}]}}`,
		"schemas/api/tenant-audit-event-resource.schema.json":        `{"auditEventId":"audit_credential_1","eventKind":"credential.audit_recorded","tenantId":"ten_1","principalId":"prn_1","outcome":"succeeded","reasonCode":"credential_rotated","createdAt":"2026-04-24T10:00:00Z","document":{"resourceKind":"tenant_secret","resourceId":"sec_1","action":"secret.rotate","secretRef":"provider/api-key","secretVersionId":"secver_2","secretRefCount":1,"secretRefs":[{"secretRef":"provider/api-key","resolution":"unavailable","redactionRule":"secret_ref_only"}]}}`,
	}
}

func TestHostedCredentialSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	mustValidateFixtures(t, validator, hostedCredentialContractFixtures())
}
