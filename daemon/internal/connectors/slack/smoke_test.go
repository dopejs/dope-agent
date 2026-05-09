package slack

import (
	"testing"
	"time"
)

func TestBuildSmokeEvidenceStructuredSkipAndFakePass(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)
	skip := BuildSmokeEvidence(SmokeInput{
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		WorkspaceBindingID: "workspace_binding_redacted",
		ValidatedAt:        now,
	})
	if skip.Status != SmokeSkipped || skip.AuthorizationMode != AuthorizationModeUnavailable || skip.RemainingRisk == "" {
		t.Fatalf("unexpected structured skip: %+v", skip)
	}

	fakePass := BuildSmokeEvidence(SmokeInput{
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		WorkspaceBindingID: "workspace_binding_redacted",
		FakeOAuth:          true,
		Passed:             true,
		ValidatedAt:        now,
		SafeEvidence:       map[string]string{"transport": "fake"},
		RemainingRisk:      "live provider not exercised",
	})
	if fakePass.Status != SmokePassed || fakePass.AuthorizationMode != AuthorizationModeFakeOAuth || fakePass.Reason != "healthy" {
		t.Fatalf("unexpected fake pass: %+v", fakePass)
	}
}

func TestBuildSmokeEvidenceSuppressesUnsafeEvidence(t *testing.T) {
	t.Parallel()

	smoke := BuildSmokeEvidence(SmokeInput{
		TenantID:           "ten_slack",
		ConnectorID:        "slack-main",
		WorkspaceBindingID: "workspace_binding_redacted",
		SafeLiveApproved:   true,
		SafeEvidence: map[string]string{
			"authorization": "Bearer xoxb-secret",
		},
	})
	if smoke.RedactionStatus != "suppressed" || len(smoke.SafeEvidence) != 0 {
		t.Fatalf("expected unsafe smoke evidence to be suppressed, got %+v", smoke)
	}
}
