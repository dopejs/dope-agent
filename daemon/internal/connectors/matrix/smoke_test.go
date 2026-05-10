package matrix

import (
	"testing"
	"time"
)

func TestSmokeEvidenceStructuredSkipIncludesRequiredRiskRecord(t *testing.T) {
	t.Parallel()

	smoke := StructuredSkipSmokeEvidence("ten", "matrix-main", "owner", "safe Matrix credentials unavailable", time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	if smoke.Status != SmokeSkipped || smoke.AuthorizationMode != SmokeAuthorizationUnavailable || smoke.Owner == "" || smoke.Reason == "" || smoke.RemainingRisk == "" {
		t.Fatalf("incomplete structured skip smoke evidence: %+v", smoke)
	}
	if smoke.RetentionExpiresAt.Sub(smoke.ValidatedAt) != 90*24*time.Hour {
		t.Fatalf("unexpected retention: %s", smoke.RetentionExpiresAt.Sub(smoke.ValidatedAt))
	}
}
