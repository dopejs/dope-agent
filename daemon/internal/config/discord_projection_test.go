package config

import "testing"

func TestDiscordLocalConfigProjectsIntoHostedReadinessWithoutBreakingLegacyUse(t *testing.T) {
	t.Parallel()

	cfg := DiscordConnectorConfig{
		Enabled:        true,
		ConnectorID:    "discord-main",
		DisplayName:    "Discord Main",
		DeliveryMode:   "gateway",
		BotToken:       "local-dev-token",
		RequireMention: true,
		RespondInDM:    true,
		AllowedGuildIDs: []string{
			"guild_local",
		},
		AllowedChannelIDs: []string{
			"channel_local",
		},
	}

	projection := cfg.ProjectHostedReadiness("ten_local")
	if !projection.LocalCompatible {
		t.Fatalf("expected local config to remain compatible, got %+v", projection)
	}
	if projection.ReadinessState != "degraded_needs_repair" {
		t.Fatalf("readiness=%s, want degraded_needs_repair until explicit hosted destinations validate", projection.ReadinessState)
	}
	if projection.HostedReady {
		t.Fatalf("legacy local config must not become hosted-ready without validated destination evidence: %+v", projection)
	}
	if projection.ReasonCode != "destination_validation_required" {
		t.Fatalf("reason=%q, want destination_validation_required", projection.ReasonCode)
	}
	if projection.BotTokenConfigured != true || projection.BotToken != "" {
		t.Fatalf("projection must expose configured flag without token material: %+v", projection)
	}
}
