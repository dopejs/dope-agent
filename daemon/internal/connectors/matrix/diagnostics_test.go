package matrix

import (
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestMatrixDiagnosticMappingAndFreshness(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	diag := MapCondition(MatrixConditionHomeserverUnsupported, DiagnosticInput{
		TenantID:          "ten",
		ConnectorID:       "matrix-main",
		EvidenceTimestamp: now.Add(-16 * time.Minute),
		Now:               now,
		RedactionReliable: true,
	})
	if diag.ReasonCode != baseconnectors.DiagnosticUnsupportedCapability {
		t.Fatalf("ReasonCode = %s, want unsupported_capability", diag.ReasonCode)
	}
	if diag.FreshnessState != baseconnectors.FreshnessStale {
		t.Fatalf("FreshnessState = %s, want stale", diag.FreshnessState)
	}
}
