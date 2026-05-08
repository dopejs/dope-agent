package telegram

import (
	"testing"
	"time"
)

func TestBuildSmokeEvidenceStructuredSkipAndFakePass(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	skip := BuildSmokeEvidence(SmokeInput{
		TenantID:    "ten_telegram",
		ConnectorID: "telegram-main",
		ValidatedAt: now,
	})
	if skip.Status != SmokeSkipped || skip.CredentialMode != CredentialModeUnavailable || skip.RemainingRisk == "" {
		t.Fatalf("unexpected structured skip: %+v", skip)
	}

	fakePass := BuildSmokeEvidence(SmokeInput{
		TenantID:       "ten_telegram",
		ConnectorID:    "telegram-main",
		FakeSafePass:   true,
		Passed:         true,
		ValidatedAt:    now,
		SafeEvidence:   map[string]string{"transport": "fake"},
		RemainingRisk:  "live provider not exercised",
		SafeCredential: true,
	})
	if fakePass.Status != SmokePassed || fakePass.CredentialMode != CredentialModeFake || fakePass.Reason != "healthy" {
		t.Fatalf("unexpected fake pass: %+v", fakePass)
	}
}

func TestBuildSmokeEvidenceSuppressesUnsafeEvidence(t *testing.T) {
	t.Parallel()

	smoke := BuildSmokeEvidence(SmokeInput{
		TenantID:       "ten_telegram",
		ConnectorID:    "telegram-main",
		SafeCredential: true,
		Passed:         false,
		SafeEvidence: map[string]string{
			"token": "123:SECRET",
		},
	})
	if smoke.RedactionStatus != "suppressed" || len(smoke.SafeEvidence) != 0 {
		t.Fatalf("expected unsafe smoke evidence to be suppressed, got %+v", smoke)
	}
}
