package store

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteStorePersistsMatrixSetupRoutePolicySmokeAndEventsTenantSafely(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	setup := MatrixHostedSetupRecord{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "degraded",
		TerminalState:       "action-required",
		BotCredentialState:  "valid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "valid",
		DeliveryEligible:    false,
		HomeserverBindingID: "matrix_hs_1",
		ReasonCode:          "blocked_route",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now,
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		HomeserverBinding: &MatrixHomeserverBindingRecord{
			TenantID:                  "ten_matrix",
			ConnectorID:               "matrix-main",
			HomeserverBindingID:       "matrix_hs_1",
			HomeserverURL:             "https://matrix.example.org",
			BotUserID:                 "@bot:example.org",
			AuthorizationState:        "valid",
			HomeserverCapabilityState: "valid",
			ValidatedAt:               now,
			RedactionStatus:           "redacted",
		},
	}
	if err := sqliteStore.SaveMatrixHostedSetup(ctx, setup); err != nil {
		t.Fatalf("SaveMatrixHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixRoutePolicy(ctx, MatrixRoutePolicyRecord{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		SelectedRooms:       []MatrixConversationRouteRecord{{ConversationID: "!room:example.org", ConversationType: "room", RoomSelectionState: "selected", ValidationState: "valid", RedactionStatus: "redacted"}},
		AllowedDirectUsers:  []string{"@alice:example.org"},
		RoomInvocationGate:  "bot_mention_or_command_required",
		ConfiguredCommands:  []string{"!dope"},
		EncryptedRoomPolicy: "unsupported",
		ValidationState:     "valid",
		ValidatedAt:         now,
		RedactionStatus:     "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixRoutePolicy returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixSmokeEvidence(ctx, MatrixSmokeEvidenceRecord{
		SmokeEvidenceID:     "matrix_smoke_1",
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		HomeserverBindingID: "matrix_hs_1",
		Status:              "skipped",
		AuthorizationMode:   "unavailable",
		Owner:               "operator",
		Reason:              "safe Matrix credentials unavailable",
		RemainingRisk:       "No live Matrix smoke was run.",
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		RedactionStatus:     "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixSmokeEvidence returned error: %v", err)
	}
	if err := sqliteStore.SaveMatrixEventEvidence(ctx, MatrixEventEvidenceRecord{
		TenantID:           "ten_matrix",
		ConnectorID:        "matrix-main",
		HomeserverID:       "matrix.example.org",
		ConversationID:     "!room:example.org",
		MatrixEventID:      "$event",
		SyncBatchID:        "sync-1",
		RouteOutcome:       "accepted",
		ReasonCode:         "accepted",
		ReceivedAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
	}); err != nil {
		t.Fatalf("SaveMatrixEventEvidence returned error: %v", err)
	}

	got, ok, err := sqliteStore.GetMatrixHostedSetup(ctx, "ten_matrix", "matrix-main")
	if err != nil || !ok {
		t.Fatalf("GetMatrixHostedSetup ok=%v err=%v", ok, err)
	}
	if got.RoutePolicy == nil || got.HomeserverBinding == nil || got.HomeserverBinding.BotUserID != "@bot:example.org" {
		t.Fatalf("expected setup with binding and route policy, got %+v", got)
	}
	if _, ok, err := sqliteStore.GetMatrixHostedSetup(ctx, "ten_other", "matrix-main"); err != nil || ok {
		t.Fatalf("cross-tenant setup lookup ok=%v err=%v, want ok=false nil", ok, err)
	}
}
