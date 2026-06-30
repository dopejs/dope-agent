package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestWebhookSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/create-webhook.request.schema.json":    `{"name":"deploy hook","targetKind":"routine","targetRef":"routine_1"}`,
		"schemas/api/webhook-endpoint-resource.schema.json": `{"webhookId":"webhook_1","tenantId":"ten_a","environmentScope":"test","name":"deploy hook","targetKind":"routine","targetRef":"routine_1","status":"active","secretFingerprint":"sha256:abcdef012345","secretVersion":1,"createdAt":"2026-06-30T10:00:00Z","updatedAt":"2026-06-30T10:00:00Z"}`,
		"schemas/api/webhook-trigger-record.schema.json":    `{"triggerId":"webhook_trigger_1","webhookId":"webhook_1","tenantId":"ten_a","environmentScope":"test","idempotencyKey":"evt-1","status":"fired","payloadBytes":42,"executionRef":"run_1","createdAt":"2026-06-30T10:00:01Z"}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
