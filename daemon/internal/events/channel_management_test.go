package events

import (
	"testing"
	"time"
)

func TestConnectorManagementEventsCarryTenantAndRedactionMetadata(t *testing.T) {
	t.Parallel()

	input := ConnectorManagementEventInput{
		TenantID:        "ten_channels",
		ConnectorID:     "matrix-main",
		EvidenceID:      "support_1",
		ReasonCode:      "support_evidence_generated",
		RedactionStatus: "redacted",
		OccurredAt:      time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC),
	}

	for _, event := range []Event{
		ConnectorManagementSupportEvidenceGenerated(input),
		ConnectorManagementRedactionFailed(input),
		ConnectorManagementRetentionApplied(input),
	} {
		if event.TenantID != "ten_channels" || event.Scope.ConnectorID != "matrix-main" {
			t.Fatalf("event missing tenant connector scope: %+v", event)
		}
		if event.Category != "connector" || event.Payload["redactionStatus"] != "redacted" {
			t.Fatalf("event missing connector redaction payload: %+v", event)
		}
	}
}
