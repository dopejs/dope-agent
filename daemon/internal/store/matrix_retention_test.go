package store

import (
	"context"
	"testing"
	"time"
)

func TestMatrixRetentionAccessorsHideExpiredEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveMatrixSmokeEvidence(ctx, MatrixSmokeEvidenceRecord{
		SmokeEvidenceID:     "matrix_smoke_expired",
		TenantID:            "ten_matrix_retention",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		Status:              "skipped",
		AuthorizationMode:   "unavailable",
		Owner:               "operator",
		Reason:              "old skip",
		ValidatedAt:         now.Add(-48 * time.Hour),
		RetentionExpiresAt:  now.Add(-time.Hour),
		RedactionStatus:     "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixSmokeEvidence expired returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixSmokeEvidence(ctx, MatrixSmokeEvidenceRecord{
		SmokeEvidenceID:     "matrix_smoke_current",
		TenantID:            "ten_matrix_retention",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		Status:              "skipped",
		AuthorizationMode:   "unavailable",
		Owner:               "operator",
		Reason:              "current skip",
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		RedactionStatus:     "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixSmokeEvidence current returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixEventEvidence(ctx, MatrixEventEvidenceRecord{
		TenantID:           "ten_matrix_retention",
		ConnectorID:        "matrix-main",
		HomeserverID:       "example.org",
		ConversationID:     "!room:example.org",
		MatrixEventID:      "$expired",
		RouteOutcome:       "accepted",
		ReceivedAt:         now.Add(-48 * time.Hour),
		RetentionExpiresAt: now.Add(-time.Hour),
		RedactionStatus:    "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixEventEvidence expired returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixEventEvidence(ctx, MatrixEventEvidenceRecord{
		TenantID:           "ten_matrix_retention",
		ConnectorID:        "matrix-main",
		HomeserverID:       "example.org",
		ConversationID:     "!room:example.org",
		MatrixEventID:      "$current",
		RouteOutcome:       "accepted",
		ReceivedAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixEventEvidence current returned error: %v", err)
	}

	smoke, ok, err := sqliteStore.LatestMatrixSmokeEvidence(ctx, "ten_matrix_retention", "matrix-main", now)
	if err != nil || !ok {
		t.Fatalf("LatestMatrixSmokeEvidence ok=%v err=%v", ok, err)
	}
	if smoke.SmokeEvidenceID != "matrix_smoke_current" {
		t.Fatalf("expected current smoke evidence, got %+v", smoke)
	}
	events, err := sqliteStore.ListMatrixEventEvidence(ctx, "ten_matrix_retention", "matrix-main", now, 10)
	if err != nil {
		t.Fatalf("ListMatrixEventEvidence returned error: %v", err)
	}
	if len(events) != 1 || events[0].MatrixEventID != "$current" {
		t.Fatalf("expected only retained current event evidence, got %+v", events)
	}
}

func TestMatrixRetentionAccessorsRetainRedactionFailureStatus(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveMatrixSmokeEvidence(ctx, MatrixSmokeEvidenceRecord{
		SmokeEvidenceID:     "matrix_smoke_redaction_failed",
		TenantID:            "ten_matrix_redaction",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		Status:              "failed",
		AuthorizationMode:   "unavailable",
		Owner:               "operator",
		Reason:              "redaction_failed",
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		RedactionStatus:     "redaction_failed",
	}); err != nil {
		t.Fatalf("SaveMatrixSmokeEvidence returned error: %v", err)
	}

	smoke, ok, err := sqliteStore.LatestMatrixSmokeEvidence(ctx, "ten_matrix_redaction", "matrix-main", now)
	if err != nil || !ok {
		t.Fatalf("LatestMatrixSmokeEvidence ok=%v err=%v", ok, err)
	}
	if smoke.RedactionStatus != "redaction_failed" {
		t.Fatalf("expected redaction failure status to be retained, got %+v", smoke)
	}
}
