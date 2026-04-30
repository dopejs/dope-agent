package evaluation

import (
	"testing"
	"time"
)

func TestValidateProductFixturePayloadMaterializesOnlyRedactedPayload(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	result, err := ValidateProductFixturePayload(CandidateEvidenceInput{
		TenantID:              "ten_eval",
		DiscoveredCandidateID: "candidate_1",
		Payload: map[string]any{
			"goal":          "safe",
			"access_token":  "secret",
			"custom_secret": "tenant secret",
		},
		RedactionPolicy: RedactionPolicy{SensitiveFieldRules: []string{"custom_secret"}},
		Now:             now,
	})
	if err != nil {
		t.Fatalf("ValidateProductFixturePayload: %v", err)
	}
	if result.Status != RedactionStatusRedacted {
		t.Fatalf("status=%s, want redacted", result.Status)
	}
	if _, ok := result.Payload["access_token"]; ok {
		t.Fatal("access token was materialized")
	}
	if _, ok := result.Payload["custom_secret"]; ok {
		t.Fatal("configured secret was materialized")
	}
	if !result.Evidence.MaterializationAllowed {
		t.Fatal("redacted evidence should remain materializable")
	}
}
