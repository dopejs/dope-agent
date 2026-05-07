package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestSQLiteStoreConnectorDiagnosticsRetention(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	active, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		DiagnosticStateID:  "diag_active",
		TenantID:           "ten_033",
		ConnectorID:        "connector_discord",
		ConnectorAccountID: "acct_redacted",
		ReasonCode:         connectors.DiagnosticPermissionMissing,
		EvidenceTimestamp:  now,
		RedactionReliable:  true,
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic(active): %v", err)
	}
	expired := active
	expired.DiagnosticStateID = "diag_expired"
	expired.RetentionExpiresAt = now.Add(-time.Hour)

	if err := store.SaveConnectorDiagnosticState(ctx, active); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState(active): %v", err)
	}
	if err := store.SaveConnectorDiagnosticState(ctx, expired); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState(expired): %v", err)
	}

	items, err := store.ListConnectorDiagnosticStates(ctx, "ten_033", "connector_discord", now)
	if err != nil {
		t.Fatalf("ListConnectorDiagnosticStates: %v", err)
	}
	if len(items) != 1 || items[0].DiagnosticStateID != "diag_active" {
		t.Fatalf("expected only active diagnostic, got %#v", items)
	}
	if items[0].RemediationOwner != connectors.RemediationOwnerAdmin {
		t.Fatalf("remediation owner=%s, want tenant_admin", items[0].RemediationOwner)
	}
}

func TestSQLiteStoreConnectorDiagnosticsPersistsRedactionFailures(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	state, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		DiagnosticStateID: "diag_redaction_failed",
		TenantID:          "ten_033",
		ConnectorID:       "connector_discord",
		ReasonCode:        connectors.DiagnosticProviderUnavailable,
		EvidenceTimestamp: now,
		RedactionReliable: false,
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic: %v", err)
	}
	if err := store.SaveConnectorDiagnosticState(ctx, state); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState: %v", err)
	}

	var count int
	if err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM connector_diagnostic_redaction_failures
		WHERE redaction_failure_id = ? AND diagnostic_state_id = ? AND tenant_id = ? AND connector_id = ?
	`, state.RedactionFailureID, state.DiagnosticStateID, "ten_033", "connector_discord").Scan(&count); err != nil {
		t.Fatalf("query connector_diagnostic_redaction_failures: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected persisted redaction failure row, got %d", count)
	}
}
