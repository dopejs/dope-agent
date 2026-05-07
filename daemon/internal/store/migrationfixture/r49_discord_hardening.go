package migrationfixture

import (
	"context"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

var R49DiscordHardeningTableNames = []string{
	"discord_hosted_setups",
	"discord_destination_validations",
	"discord_smoke_evidence",
}

type R49DiscordHardeningFixture struct {
	TenantIDs        []string
	ExpectedRowCount map[string]int
}

func BuildR49DiscordHardeningFixture() R49DiscordHardeningFixture {
	return R49DiscordHardeningFixture{
		TenantIDs: []string{"ten_discord_alpha", "ten_discord_beta"},
		ExpectedRowCount: map[string]int{
			"discord_hosted_setups":           2,
			"discord_destination_validations": 2,
			"discord_smoke_evidence":          2,
		},
	}
}

func SeedR49DiscordHardeningRows(ctx context.Context, s *store.SQLiteStore) (R49DiscordHardeningFixture, error) {
	fixture := BuildR49DiscordHardeningFixture()
	for i, tenantID := range fixture.TenantIDs {
		suffix := fmt.Sprintf("%d", i+1)
		connectorID := "discord-r49-" + suffix
		if err := s.SaveDiscordHostedSetup(ctx, store.DiscordHostedSetupRecord{
			TenantID:           tenantID,
			ConnectorID:        connectorID,
			ConnectorKind:      "discord",
			DisplayName:        "Discord R49",
			Status:             "degraded",
			ReadinessState:     "degraded_needs_repair",
			CredentialState:    "valid",
			RespondInDM:        true,
			RequireMention:     true,
			DeliveryMode:       "gateway",
			ReasonCode:         "destination_validation_failed",
			RedactionStatus:    "redacted",
			CreatedAt:          mustFixtureTime(ts),
			UpdatedAt:          mustFixtureTime(ts),
			ValidatedAt:        mustFixtureTime(ts),
			RetentionExpiresAt: mustFixtureTime(ts),
		}); err != nil {
			return fixture, err
		}
		if err := s.SaveDiscordDestinationValidation(ctx, store.DiscordDestinationValidationRecord{
			TenantID:        tenantID,
			ConnectorID:     connectorID,
			DestinationID:   "channel_" + suffix,
			DestinationType: "channel",
			Selected:        true,
			ValidationState: "missing_permission",
			ReasonCode:      "permission_missing",
			ValidatedAt:     mustFixtureTime(ts),
			RedactionStatus: "redacted",
			SafeEvidence:    map[string]string{"permission": "send_messages"},
		}); err != nil {
			return fixture, err
		}
		if err := exec(ctx, s, `INSERT INTO discord_smoke_evidence (smoke_evidence_id, tenant_id, connector_id, status, credential_mode, owner, reason, remaining_risk, validated_at, retention_expires_at, redaction_status, document_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			"discord_smoke_"+suffix, tenantID, connectorID, "skipped", "unavailable", "operator", "safe_credentials_unavailable", "live smoke skipped", ts, ts, "redacted", `{"status":"skipped"}`); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

func mustFixtureTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return parsed
}
