package livevalidation

import (
	"testing"
	"time"
)

func TestMatrixConnectorSmokeEvidenceSupportsStructuredSkipAndSafeLivePass(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	skip := BuildMatrixConnectorSmokeEvidence(MatrixConnectorSmokeInput{
		TenantID:    "ten_matrix",
		ConnectorID: "matrix-main",
		Owner:       "operator",
		Reason:      "safe Matrix credentials unavailable",
		Now:         now,
	})
	if skip.Status != "skipped" || skip.AuthorizationMode != "unavailable" || skip.RemainingRisk == "" {
		t.Fatalf("unexpected structured skip smoke evidence: %+v", skip)
	}

	pass := BuildMatrixConnectorSmokeEvidence(MatrixConnectorSmokeInput{
		TenantID:          "ten_matrix",
		ConnectorID:       "matrix-main",
		Owner:             "operator",
		SafeLiveAvailable: true,
		Now:               now,
	})
	if pass.Status != "passed" || pass.AuthorizationMode != "safe_live" || pass.Reason != "healthy" {
		t.Fatalf("unexpected safe live smoke evidence: %+v", pass)
	}
}
