package migrationfixture

import (
	"context"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

var R50TelegramChannelConnectorTableNames = []string{
	"telegram_hosted_setups",
	"telegram_allowments",
	"telegram_smoke_evidence",
	"telegram_update_evidence",
}

type R50TelegramChannelConnectorFixture struct {
	TenantIDs        []string
	ExpectedRowCount map[string]int
}

func BuildR50TelegramChannelConnectorFixture() R50TelegramChannelConnectorFixture {
	return R50TelegramChannelConnectorFixture{
		TenantIDs: []string{"ten_telegram_alpha", "ten_telegram_beta"},
		ExpectedRowCount: map[string]int{
			"telegram_hosted_setups":   2,
			"telegram_allowments":      2,
			"telegram_smoke_evidence":  2,
			"telegram_update_evidence": 2,
		},
	}
}

func SeedR50TelegramChannelConnectorRows(ctx context.Context, s *store.SQLiteStore) (R50TelegramChannelConnectorFixture, error) {
	fixture := BuildR50TelegramChannelConnectorFixture()
	for i, tenantID := range fixture.TenantIDs {
		suffix := fmt.Sprintf("%d", i+1)
		connectorID := "telegram-r50-" + suffix
		if err := s.SaveTelegramHostedSetup(ctx, store.TelegramHostedSetupRecord{
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			ConnectorKind:      "telegram",
			DisplayName:        "Telegram R50",
			Status:             "degraded",
			TerminalState:      "action-required",
			CredentialState:    "valid",
			AllowmentState:     "none",
			GroupBehavior:      "disabled",
			ReasonCode:         "telegram_allowment_missing",
			RedactionStatus:    "redacted",
			CreatedAt:          mustR50FixtureTime(ts),
			UpdatedAt:          mustR50FixtureTime(ts),
			ValidatedAt:        mustR50FixtureTime(ts),
			RetentionExpiresAt: mustR50FixtureTime(ts),
		}); err != nil {
			return fixture, err
		}
		if err := s.SaveTelegramAllowment(ctx, store.TelegramAllowmentRecord{
			TenantID:        tenantID,
			ConnectorID:     connectorID,
			AllowmentID:     "allow_" + suffix,
			ScopeType:       "direct_chat",
			ScopeID:         "chat_" + suffix,
			Enabled:         true,
			GroupGate:       "not_applicable",
			ValidationState: "valid",
			ReasonCode:      "healthy",
			ValidatedAt:     mustR50FixtureTime(ts),
			RedactionStatus: "redacted",
			SafeEvidence:    map[string]string{"scope": "direct_chat"},
		}); err != nil {
			return fixture, err
		}
		if err := s.SaveTelegramSmokeEvidence(ctx, store.TelegramSmokeEvidenceRecord{
			SmokeEvidenceID:    "telegram_smoke_" + suffix,
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			Status:             "skipped",
			CredentialMode:     "unavailable",
			Owner:              "operator",
			Reason:             "safe_credentials_unavailable",
			RemainingRisk:      "live smoke skipped",
			ValidatedAt:        mustR50FixtureTime(ts),
			RetentionExpiresAt: mustR50FixtureTime(ts),
			RedactionStatus:    "redacted",
			SafeEvidence:       map[string]string{"policy": "structured_skip"},
		}); err != nil {
			return fixture, err
		}
		if err := s.SaveTelegramUpdateEvidence(ctx, store.TelegramUpdateEvidenceRecord{
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			ChatID:             "chat_" + suffix,
			MessageID:          "message_" + suffix,
			UpdateID:           "update_" + suffix,
			RouteOutcome:       "accepted",
			ReasonCode:         "accepted",
			ReceivedAt:         mustR50FixtureTime(ts),
			RetentionExpiresAt: mustR50FixtureTime(ts),
			RedactionStatus:    "redacted",
			SafeEvidence:       map[string]string{"identityRule": "telegram_chat_message_id"},
		}); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

func CountR50TelegramChannelConnectorRows(ctx context.Context, s *store.SQLiteStore) (map[string]int, error) {
	counts := map[string]int{}
	for _, table := range R50TelegramChannelConnectorTableNames {
		var count int
		if err := s.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}

func mustR50FixtureTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return parsed
}
