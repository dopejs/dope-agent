package store

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteStorePersistsDiscordHostedSetupAndDestinationsTenantSafely(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	setup := DiscordHostedSetupRecord{
		TenantID:           "ten_discord",
		ConnectorID:        "discord-main",
		ConnectorKind:      "discord",
		DisplayName:        "Discord Main",
		Status:             "degraded",
		ReadinessState:     "degraded_needs_repair",
		CredentialState:    "valid",
		RespondInDM:        true,
		RequireMention:     true,
		DeliveryMode:       "gateway",
		ReasonCode:         "destination_validation_failed",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	if err := sqliteStore.SaveDiscordHostedSetup(ctx, setup); err != nil {
		t.Fatalf("SaveDiscordHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.SaveDiscordDestinationValidation(ctx, DiscordDestinationValidationRecord{
		TenantID:        "ten_discord",
		ConnectorID:     "discord-main",
		DestinationID:   "channel_redacted",
		DestinationType: "channel",
		Selected:        true,
		ValidationState: "missing_permission",
		ReasonCode:      "permission_missing",
		ValidatedAt:     now,
		RedactionStatus: "redacted",
		SafeEvidence:    map[string]string{"permission": "send_messages"},
	}); err != nil {
		t.Fatalf("SaveDiscordDestinationValidation returned error: %v", err)
	}

	got, ok, err := sqliteStore.GetDiscordHostedSetup(ctx, "ten_discord", "discord-main")
	if err != nil || !ok {
		t.Fatalf("GetDiscordHostedSetup ok=%v err=%v", ok, err)
	}
	if got.ReadinessState != "degraded_needs_repair" || got.CredentialState != "valid" {
		t.Fatalf("unexpected setup record: %+v", got)
	}
	if _, ok, err := sqliteStore.GetDiscordHostedSetup(ctx, "ten_other", "discord-main"); err != nil || ok {
		t.Fatalf("cross-tenant setup lookup ok=%v err=%v, want not found", ok, err)
	}
	destinations, err := sqliteStore.ListDiscordDestinationValidations(ctx, "ten_discord", "discord-main")
	if err != nil {
		t.Fatalf("ListDiscordDestinationValidations returned error: %v", err)
	}
	if len(destinations) != 1 || destinations[0].SafeEvidence["permission"] != "send_messages" {
		t.Fatalf("unexpected destination evidence: %+v", destinations)
	}
}

func TestSQLiteStorePersistsDiscordSmokeEvidenceWithRetention(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveDiscordSmokeEvidence(ctx, DiscordSmokeEvidenceRecord{
		SmokeEvidenceID:    "discord_smoke_1",
		TenantID:           "ten_discord",
		ConnectorID:        "discord-main",
		Status:             "skipped",
		CredentialMode:     "unavailable",
		Owner:              "operator",
		Reason:             "safe_credentials_unavailable",
		RemainingRisk:      "live smoke skipped",
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence:       map[string]string{"policy": "structured_skip"},
	}); err != nil {
		t.Fatalf("SaveDiscordSmokeEvidence returned error: %v", err)
	}
	got, ok, err := sqliteStore.LatestDiscordSmokeEvidence(ctx, "ten_discord", "discord-main", now)
	if err != nil || !ok {
		t.Fatalf("LatestDiscordSmokeEvidence ok=%v err=%v", ok, err)
	}
	if got.Status != "skipped" || got.SafeEvidence["policy"] != "structured_skip" {
		t.Fatalf("unexpected smoke evidence: %+v", got)
	}
	if _, ok, err := sqliteStore.LatestDiscordSmokeEvidence(ctx, "ten_discord", "discord-main", now.Add(91*24*time.Hour)); err != nil || ok {
		t.Fatalf("expired smoke evidence ok=%v err=%v, want not found", ok, err)
	}
}
