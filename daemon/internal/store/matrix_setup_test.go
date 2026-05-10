package store

import (
	"context"
	"testing"
	"time"
)

func TestMatrixSetupPersistenceLifecycleUpdatesTerminalState(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveMatrixHostedSetup(ctx, MatrixHostedSetupRecord{
		TenantID:            "ten_matrix_setup",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "degraded",
		TerminalState:       "action-required",
		BotCredentialState:  "submitted",
		HomeserverState:     "reachable",
		RoutePolicyState:    "none",
		DeliveryEligible:    false,
		HomeserverBindingID: "matrix_hs_setup",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveMatrixHostedSetup initial returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixHostedSetup(ctx, MatrixHostedSetupRecord{
		TenantID:            "ten_matrix_setup",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "healthy",
		TerminalState:       "ready",
		BotCredentialState:  "valid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "valid",
		DeliveryEligible:    true,
		HomeserverBindingID: "matrix_hs_setup",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now.Add(time.Minute),
		ValidatedAt:         now.Add(time.Minute),
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveMatrixHostedSetup update returned error: %v", err)
	}

	got, ok, err := sqliteStore.GetMatrixHostedSetup(ctx, "ten_matrix_setup", "matrix-main")
	if err != nil || !ok {
		t.Fatalf("GetMatrixHostedSetup ok=%v err=%v", ok, err)
	}
	if got.TerminalState != "ready" || !got.DeliveryEligible || got.BotCredentialState != "valid" {
		t.Fatalf("expected ready lifecycle update, got %+v", got)
	}
}
