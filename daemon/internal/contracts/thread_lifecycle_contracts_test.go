package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestThreadLifecycleAPIAndEventContracts(t *testing.T) {
	t.Parallel()
	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/thread-list.response.schema.json":              `{"tenantId":"ten_1","page":{"limit":20,"order":"active_recent_archived_id"},"items":[{"threadId":"thr_1","tenantId":"ten_1","lifecycleState":"active","sourceKind":"channel","sourceSummary":"Slack Main / #support","currentSessionSegmentId":"seg_1","currentSessionId":"sess_1","lastActivityAt":"2026-05-11T10:00:00Z","availableActions":["reset","archive"],"redactionStatus":"redacted","retentionExpiresAt":"2026-08-09T10:00:00Z","updatedAt":"2026-05-11T10:00:00Z"}]}`,
		"schemas/api/thread-detail.response.schema.json":            `{"thread":{"threadId":"thr_1","tenantId":"ten_1","lifecycleState":"active","sourceKind":"channel","sourceSummary":"Slack Main / #support","currentSessionSegmentId":"seg_1","currentSessionId":"sess_1","lastActivityAt":"2026-05-11T10:00:00Z","availableActions":["reset"],"redactionStatus":"redacted","retentionExpiresAt":"2026-08-09T10:00:00Z","updatedAt":"2026-05-11T10:00:00Z"},"sessionSegments":[{"sessionSegmentId":"seg_1","partialEvidence":false}],"sourceLinkages":[{"sourceLinkageId":"src_1","sourceKind":"channel","routingOutcome":"accepted","current":true,"linkedAt":"2026-05-11T10:00:00Z","retentionExpiresAt":"2026-08-09T10:00:00Z","redactionStatus":"redacted"}],"runtimeProjections":[{"runtimeProjectionId":"rtp_1","resourceKind":"run","resourceId":"run_1","status":"completed","occurredAt":"2026-05-11T10:00:00Z","retentionExpiresAt":"2026-08-09T10:00:00Z","redactionStatus":"redacted"}],"lifecycleActions":[{"lifecycleActionId":"act_1"}]}`,
		"schemas/api/thread-lifecycle-action.response.schema.json":  `{"threadId":"thr_1","lifecycleState":"reset","previousSessionSegmentId":"seg_old","currentSessionSegmentId":"seg_new","auditEventId":"audit_1","changedAt":"2026-05-11T10:00:00Z","action":"reset","availableActions":["reset","archive"]}`,
		"schemas/events/thread-lifecycle.event.schema.json":         `{"tenantId":"ten_1","threadId":"thr_1","sessionSegmentId":"seg_1","action":"reset","outcome":"succeeded","auditEventId":"audit_1","reasonCode":"user_requested_reset","redactionStatus":"redacted"}`,
		"schemas/events/thread-source-linked.event.schema.json":     `{"tenantId":"ten_1","threadId":"thr_1","sessionSegmentId":"seg_1","sourceLinkageId":"src_1","routingOutcome":"accepted","redactionStatus":"redacted"}`,
		"schemas/events/thread-retention-applied.event.schema.json": `{"tenantId":"ten_1","threadId":"thr_1","retentionExpiresAt":"2026-08-09T10:00:00Z","policySource":"default","redactionStatus":"redacted"}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
