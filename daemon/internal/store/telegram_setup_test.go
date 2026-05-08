package store

import (
	"context"
	"testing"
	"time"
)

func TestSQLiteStorePersistsTelegramSetupAllowmentsAndSmokeTenantSafely(t *testing.T) {
	t.Parallel()

	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	ctx := context.Background()
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	setup := TelegramHostedSetupRecord{
		TenantID:           "ten_telegram",
		ConnectorID:        "telegram-main",
		ConnectorKind:      "telegram",
		DisplayName:        "Telegram Main",
		Status:             "healthy",
		TerminalState:      "ready",
		HostedReady:        true,
		CredentialState:    "valid",
		AllowmentState:     "valid",
		GroupBehavior:      "mention_or_command_required",
		DeliveryEligible:   true,
		ReasonCode:         "healthy",
		RedactionStatus:    "redacted",
		CreatedAt:          now,
		UpdatedAt:          now,
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		AccountBinding: &ConnectorAccountBindingSummary{
			TenantID:            "ten_telegram",
			ConnectorID:         "telegram-main",
			ConnectorAccountID:  "telegram_bot_42",
			DisplayName:         "dope_test_bot",
			ProviderAccountHint: "dope_test_bot",
			RedactionStatus:     "redacted",
			UpdatedAt:           now,
		},
	}
	if err := sqliteStore.SaveTelegramHostedSetup(ctx, setup); err != nil {
		t.Fatalf("SaveTelegramHostedSetup returned error: %v", err)
	}
	if err := sqliteStore.SaveTelegramAllowment(ctx, TelegramAllowmentRecord{
		TenantID:        "ten_telegram",
		ConnectorID:     "telegram-main",
		AllowmentID:     "allow_dm",
		ScopeType:       "direct_chat",
		ScopeID:         "chat_redacted",
		Enabled:         true,
		GroupGate:       "not_applicable",
		ValidationState: "valid",
		ReasonCode:      "healthy",
		ValidatedAt:     now,
		RedactionStatus: "redacted",
		SafeEvidence:    map[string]string{"scope": "direct_chat"},
	}); err != nil {
		t.Fatalf("SaveTelegramAllowment returned error: %v", err)
	}
	if err := sqliteStore.SaveTelegramSmokeEvidence(ctx, TelegramSmokeEvidenceRecord{
		SmokeEvidenceID:    "telegram_smoke_1",
		TenantID:           "ten_telegram",
		ConnectorID:        "telegram-main",
		Status:             "passed",
		CredentialMode:     "fake",
		Owner:              "operator",
		Reason:             "healthy",
		ValidatedAt:        now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence:       map[string]string{"transport": "fake"},
	}); err != nil {
		t.Fatalf("SaveTelegramSmokeEvidence returned error: %v", err)
	}
	if err := sqliteStore.SaveTelegramUpdateEvidence(ctx, TelegramUpdateEvidenceRecord{
		TenantID:           "ten_telegram",
		ConnectorID:        "telegram-main",
		ChatID:             "chat_redacted",
		MessageID:          "message_redacted",
		UpdateID:           "update_redacted",
		RouteOutcome:       "accepted",
		ReasonCode:         "accepted",
		ReceivedAt:         now,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
		RedactionStatus:    "redacted",
		SafeEvidence:       map[string]string{"identityRule": "telegram_chat_message_id"},
	}); err != nil {
		t.Fatalf("SaveTelegramUpdateEvidence returned error: %v", err)
	}

	got, ok, err := sqliteStore.GetTelegramHostedSetup(ctx, "ten_telegram", "telegram-main")
	if err != nil || !ok {
		t.Fatalf("GetTelegramHostedSetup ok=%v err=%v", ok, err)
	}
	if got.TerminalState != "ready" || len(got.Allowments) != 1 {
		t.Fatalf("unexpected setup record: %+v", got)
	}
	if got.AccountBinding == nil || got.AccountBinding.ConnectorAccountID != "telegram_bot_42" || got.AccountBinding.ProviderAccountHint != "dope_test_bot" {
		t.Fatalf("expected account binding to round-trip, got %+v", got.AccountBinding)
	}
	if _, ok, err := sqliteStore.GetTelegramHostedSetup(ctx, "ten_other", "telegram-main"); err != nil || ok {
		t.Fatalf("cross-tenant setup lookup ok=%v err=%v, want not found", ok, err)
	}
	smoke, ok, err := sqliteStore.LatestTelegramSmokeEvidence(ctx, "ten_telegram", "telegram-main", now)
	if err != nil || !ok || smoke.CredentialMode != "fake" {
		t.Fatalf("LatestTelegramSmokeEvidence ok=%v err=%v smoke=%+v", ok, err, smoke)
	}
	evidence, err := sqliteStore.ListTelegramUpdateEvidence(ctx, "ten_telegram", "telegram-main", now, 10)
	if err != nil || len(evidence) != 1 {
		t.Fatalf("ListTelegramUpdateEvidence len=%d err=%v", len(evidence), err)
	}
	if evidence[0].ChatID != "chat_redacted" || evidence[0].SafeEvidence["identityRule"] != "telegram_chat_message_id" {
		t.Fatalf("unexpected retained update evidence: %+v", evidence[0])
	}
	otherEvidence, err := sqliteStore.ListTelegramUpdateEvidence(ctx, "ten_other", "telegram-main", now, 10)
	if err != nil || len(otherEvidence) != 0 {
		t.Fatalf("cross-tenant update evidence len=%d err=%v, want none", len(otherEvidence), err)
	}
}
