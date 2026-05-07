package discord

import (
	"testing"
	"time"
)

func TestBuildSmokeEvidenceStructuresSkipWhenSafeCredentialsUnavailable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	evidence := BuildSmokeEvidence(SmokeInput{
		SmokeEvidenceID: "discord_smoke_1",
		TenantID:        "ten_discord",
		ConnectorID:     "discord-main",
		ValidatedAt:     now,
	})
	if evidence.Status != SmokeSkipped || evidence.CredentialMode != CredentialModeUnavailable {
		t.Fatalf("expected structured skip, got %+v", evidence)
	}
	if evidence.Owner == "" || evidence.Reason == "" || evidence.RemainingRisk == "" {
		t.Fatalf("skip evidence must include owner, reason, and remaining risk: %+v", evidence)
	}
	if evidence.RetentionExpiresAt.Sub(now) != 90*24*time.Hour {
		t.Fatalf("retention=%s, want 90 days", evidence.RetentionExpiresAt)
	}
}
