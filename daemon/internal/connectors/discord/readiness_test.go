package discord

import (
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestEvaluateHostedSetupRequiresValidCredentialAndExplicitDestinations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:       "ten_discord",
		ConnectorID:    "discord-main",
		DisplayName:    "Discord Main",
		Credential:     CredentialValid,
		RespondInDM:    true,
		RequireMention: true,
		Destinations: []DestinationValidation{
			{DestinationID: "guild_redacted", DestinationType: DestinationGuild, Selected: true, ValidationState: DestinationValid},
			{DestinationID: "channel_redacted", DestinationType: DestinationChannel, Selected: true, ValidationState: DestinationValid},
		},
		ValidatedAt: now,
	})

	if setup.ReadinessState != ReadinessHostedReady {
		t.Fatalf("readiness=%s, want hosted_ready", setup.ReadinessState)
	}
	if setup.Status != baseconnectors.LifecycleStateHealthy {
		t.Fatalf("status=%s, want healthy", setup.Status)
	}
	if setup.HostedReady {
		// HostedReady is a derived bool so API consumers can gate without
		// reinterpreting state strings.
	} else {
		t.Fatalf("expected hosted ready setup, got %+v", setup)
	}
}

func TestEvaluateHostedSetupSavesDegradedForMissingOrPartiallyInvalidDestinations(t *testing.T) {
	t.Parallel()

	missing := EvaluateHostedSetup(HostedSetupInput{
		TenantID:    "ten_discord",
		ConnectorID: "discord-main",
		DisplayName: "Discord Main",
		Credential:  CredentialValid,
	})
	if missing.ReadinessState != ReadinessDegradedNeedsRepair || missing.HostedReady {
		t.Fatalf("missing destinations setup=%+v, want degraded/needs repair and not hosted-ready", missing)
	}
	if missing.ReasonCode != "missing_explicit_destination" {
		t.Fatalf("reason=%q, want missing_explicit_destination", missing.ReasonCode)
	}

	partial := EvaluateHostedSetup(HostedSetupInput{
		TenantID:    "ten_discord",
		ConnectorID: "discord-main",
		DisplayName: "Discord Main",
		Credential:  CredentialValid,
		Destinations: []DestinationValidation{
			{DestinationID: "guild_redacted", DestinationType: DestinationGuild, Selected: true, ValidationState: DestinationValid},
			{DestinationID: "channel_redacted", DestinationType: DestinationChannel, Selected: true, ValidationState: DestinationMissingPermission, ReasonCode: string(baseconnectors.DiagnosticPermissionMissing)},
		},
	})
	if partial.ReadinessState != ReadinessDegradedNeedsRepair || partial.HostedReady {
		t.Fatalf("partial setup=%+v, want degraded/needs repair and not hosted-ready", partial)
	}
	if partial.ReasonCode != "destination_validation_failed" {
		t.Fatalf("reason=%q, want destination_validation_failed", partial.ReasonCode)
	}
}

func TestDiscordConformanceProfileRequiresValidatedHostedEvidence(t *testing.T) {
	t.Parallel()

	cfg := Config{
		ConnectorID:       "discord-main",
		BotToken:          "fake-token",
		RequireMention:    true,
		RespondInDM:       true,
		AllowedGuildIDs:   []string{"guild_1"},
		AllowedChannelIDs: []string{"channel_1"},
	}
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)

	localOnly := ConformanceProfile(cfg, now)
	if err := baseconnectors.ValidateCapabilityProfile(localOnly); err == nil {
		t.Fatal("expected local config projection to fail core invariants without validated hosted evidence")
	}

	setup := EvaluateHostedSetup(HostedSetupInput{
		TenantID:       "ten_discord",
		ConnectorID:    cfg.ConnectorID,
		DisplayName:    "Discord Main",
		Credential:     CredentialValid,
		RespondInDM:    true,
		RequireMention: true,
		Destinations: []DestinationValidation{
			{DestinationID: "guild_redacted", DestinationType: DestinationGuild, Selected: true, ValidationState: DestinationValid},
			{DestinationID: "channel_redacted", DestinationType: DestinationChannel, Selected: true, ValidationState: DestinationValid},
		},
		ValidatedAt: now,
	})
	profile := ConformanceProfileForSetup(cfg, setup, now)

	if err := baseconnectors.ValidateCapabilityProfile(profile); err != nil {
		t.Fatalf("expected validated hosted Discord profile to pass core invariants: %v", err)
	}
	if profile.ProviderSurfaceResults["voice"] != baseconnectors.SurfaceUnsupported {
		t.Fatalf("expected voice unsupported declaration, got %s", profile.ProviderSurfaceResults["voice"])
	}
	if profile.ProviderSurfaceResults["connector_backed_delivery"] != baseconnectors.SurfaceSupported {
		t.Fatalf("expected connector-backed delivery support declaration")
	}
}
