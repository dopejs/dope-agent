package contracts_test

import (
	"strings"
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestThreadContinuityPreviewAndEventContracts(t *testing.T) {
	t.Parallel()
	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/thread-continuity-preview.schema.json":                   `{"continuityPreviewId":"contprev_1","tenantId":"ten_1","threadId":"thr_1","sessionSegmentId":"seg_1","dispatchId":"dispatch_1","requestTurnId":"turn_user_1","responseTurnId":"turn_assistant_1","windowPolicyId":"default_recent_12_30d","maxPriorTurns":12,"activeWindowDays":30,"orderedBy":"daemon_acceptance_sequence","continuityApplied":true,"status":"applied","includedCount":1,"excludedCount":1,"assemblyStartedAt":"2026-05-11T10:00:00Z","assemblyCompletedAt":"2026-05-11T10:00:00.037Z","assemblyDurationMs":37,"retentionExpiresAt":"2026-08-09T10:00:00Z","redactionStatus":"redacted"}`,
		"schemas/api/thread-continuity-preview-item.schema.json":              `{"previewItemId":"contitem_1","continuityPreviewId":"contprev_1","tenantId":"ten_1","threadId":"thr_1","itemKind":"turn","continuityTurnId":"turn_1","role":"user","decision":"included","reasonCode":"included_recent","acceptanceSequence":8,"safeSummary":"User asked about option A","redactionStatus":"redacted","itemOrder":0}`,
		"schemas/api/thread-continuity-preview.response.schema.json":          `{"preview":{"continuityPreviewId":"contprev_1","tenantId":"ten_1","threadId":"thr_1","sessionSegmentId":"seg_1","dispatchId":"dispatch_1","requestTurnId":"turn_user_1","windowPolicyId":"default_recent_12_30d","maxPriorTurns":12,"activeWindowDays":30,"orderedBy":"daemon_acceptance_sequence","continuityApplied":true,"status":"applied","includedCount":1,"excludedCount":1,"assemblyDurationMs":37,"redactionStatus":"redacted"},"items":[{"previewItemId":"contitem_1","itemKind":"turn","decision":"included","reasonCode":"included_recent","continuityTurnId":"turn_1","role":"user","acceptanceSequence":8,"safeSummary":"User asked about option A","redactionStatus":"redacted","itemOrder":0}]}`,
		"schemas/api/chat-query.request.schema.json":                          `{"provider":"openai_compatible","model":"gpt-test","query":"follow up","threadId":"thr_1","continuity":{"mode":"auto"}}`,
		"schemas/api/chat-query.response.schema.json":                         `{"dispatchId":"dispatch_1","provider":"openai_compatible","model":"gpt-test","skills":[],"query":"follow up","status":"completed","partial":false,"reply":"answer","usage":{"inputTokens":2,"outputTokens":2,"totalTokens":4},"threadId":"thr_1","sessionSegmentId":"seg_1","requestTurnId":"turn_user_1","responseTurnId":"turn_assistant_1","continuityPreviewId":"contprev_1","continuityApplied":true,"continuityStatus":"applied","continuityIncludedCount":1,"continuityExcludedCount":0}`,
		"schemas/events/thread-continuity-turn-recorded.event.schema.json":    `{"tenantId":"ten_1","threadId":"thr_1","sessionSegmentId":"seg_1","continuityTurnId":"turn_1","dispatchId":"dispatch_1","action":"turn_recorded","outcome":"recorded","reasonCode":"included_recent","acceptanceSequence":8,"redactionStatus":"redacted"}`,
		"schemas/events/thread-continuity-preview-recorded.event.schema.json": `{"tenantId":"ten_1","threadId":"thr_1","sessionSegmentId":"seg_1","continuityPreviewId":"contprev_1","dispatchId":"dispatch_1","action":"preview_recorded","outcome":"applied","reasonCode":"included_recent","redactionStatus":"redacted"}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
		for _, forbidden := range []string{"rawPrompt", "memory", "semantic", "knowledgeGraph"} {
			if strings.Contains(fixture, forbidden) {
				t.Fatalf("%s fixture contains forbidden non-continuity field %q", schema, forbidden)
			}
		}
	}
}
