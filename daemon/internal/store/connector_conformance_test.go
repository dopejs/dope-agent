package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestSQLiteStoreConnectorConformanceResultsRedactionAndRetention(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	active := connectors.ConformanceResult{
		ConformanceResultID: "conf_active",
		TenantID:            "ten_033",
		ConnectorKind:       "discord",
		ConnectorID:         "connector_discord",
		ScenarioID:          "discord.direct.pass",
		Area:                string(connectors.ConformanceAreaRedaction),
		Result:              connectors.ConformanceResultPass,
		RedactionStatus:     connectors.RedactionStatusRedacted,
		EvidenceTimestamp:   now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}
	suppressed := connectors.ConformanceResult{
		ConformanceResultID: "conf_suppressed",
		TenantID:            "ten_033",
		ConnectorKind:       "discord",
		ConnectorID:         "connector_discord",
		ScenarioID:          "discord.redaction.suppressed",
		Area:                string(connectors.ConformanceAreaRedaction),
		Result:              connectors.ConformanceResultFail,
		ReasonCode:          "redaction_failed_closed",
		RedactionStatus:     connectors.RedactionStatusSuppressed,
		EvidenceTimestamp:   now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}
	expired := active
	expired.ConformanceResultID = "conf_expired"
	expired.ScenarioID = "discord.direct.expired"
	expired.RetentionExpiresAt = now.Add(-time.Hour)

	for _, result := range []connectors.ConformanceResult{active, suppressed, expired} {
		if err := store.SaveConnectorConformanceResult(ctx, result); err != nil {
			t.Fatalf("SaveConnectorConformanceResult(%s): %v", result.ConformanceResultID, err)
		}
	}

	results, err := store.ListConnectorConformanceResults(ctx, "ten_033", "connector_discord", now)
	if err != nil {
		t.Fatalf("ListConnectorConformanceResults: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("active result count=%d, want 2: %#v", len(results), results)
	}
	for _, result := range results {
		if result.ConformanceResultID == "conf_expired" {
			t.Fatalf("expired result should not be listed: %#v", results)
		}
		if result.RedactionStatus == "" {
			t.Fatalf("missing redaction status: %#v", result)
		}
	}
}
