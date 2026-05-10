package events

import (
	"testing"
	"time"
)

func TestMatrixConnectorEventNamesCoverPhase52Evidence(t *testing.T) {
	t.Parallel()

	want := []string{
		ConnectorEventMatrixSetupValidated,
		ConnectorEventRouteOutcomeRecorded,
		ConnectorEventInboundDuplicateDetected,
		ConnectorEventReplySent,
		ConnectorEventReplyFailed,
		ConnectorEventForegroundReplyFailed,
		ConnectorEventDeliverySeparationRecorded,
		ConnectorEventDiagnosticStateChanged,
		ConnectorEventDiagnosticRedactionFailed,
		ConnectorEventMatrixSmokeEvidenceRecorded,
	}
	seen := map[string]bool{}
	for _, name := range MatrixConnectorEventNames {
		seen[name] = true
	}
	for _, name := range want {
		if !seen[name] {
			t.Fatalf("MatrixConnectorEventNames missing %s: %+v", name, MatrixConnectorEventNames)
		}
	}
}

func TestMatrixConnectorEventConstructorsAreRedacted(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	route := ConnectorMatrixRouteOutcomeRecorded(ConnectorMatrixRouteOutcomeRecordedInput{
		TenantID:        "ten_matrix",
		ConnectorID:     "matrix-main",
		HomeserverID:    "matrix.example.org",
		ConversationID:  "!room:example.org",
		MatrixEventID:   "$event_redacted",
		SyncBatchID:     "batch_redacted",
		TransactionID:   "txn_redacted",
		Outcome:         "accepted",
		ReasonCode:      "accepted",
		Surface:         "room",
		RedactionStatus: "redacted",
	})
	if route.Name != ConnectorEventRouteOutcomeRecorded || route.Payload["redactionStatus"] != "redacted" {
		t.Fatalf("unexpected Matrix route event: %+v", route)
	}

	smoke := ConnectorMatrixSmokeEvidenceRecorded(ConnectorMatrixSmokeEvidenceRecordedInput{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		SmokeEvidenceID:     "matrix_smoke_1",
		HomeserverBindingID: "matrix_hs_1",
		Status:              "skipped",
		AuthorizationMode:   "unavailable",
		Owner:               "operator",
		Reason:              "safe_credentials_unavailable",
		RedactionStatus:     "redacted",
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	})
	if smoke.Name != ConnectorEventMatrixSmokeEvidenceRecorded || smoke.Resource.Kind != "matrix_smoke_evidence" {
		t.Fatalf("unexpected Matrix smoke event: %+v", smoke)
	}
	if smoke.Payload["redactionStatus"] != "redacted" || smoke.Payload["reason"] != "safe_credentials_unavailable" {
		t.Fatalf("unexpected Matrix smoke payload: %+v", smoke.Payload)
	}
}
