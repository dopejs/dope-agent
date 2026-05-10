package matrix

import (
	"testing"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
)

func TestMatrixDiagnosticsFreshnessAndRedactionSuppression(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	fresh := MapCondition(MatrixConditionRateLimited, DiagnosticInput{
		TenantID:          "ten_matrix",
		ConnectorID:       "matrix-main",
		EvidenceTimestamp: now.Add(-time.Minute),
		Now:               now,
		RedactionReliable: true,
		SafeEvidence:      map[string]string{"retryAfter": "60s"},
	})
	if fresh.FreshnessState != baseconnectors.FreshnessFresh || fresh.RedactionStatus != baseconnectors.RedactionStatusRedacted {
		t.Fatalf("expected fresh redacted diagnostic, got %+v", fresh)
	}
	if fresh.SafeEvidence["retryAfter"] != "60s" {
		t.Fatalf("expected retained safe evidence, got %+v", fresh.SafeEvidence)
	}

	suppressed := MapCondition(MatrixConditionReplyFailed, DiagnosticInput{
		TenantID:          "ten_matrix",
		ConnectorID:       "matrix-main",
		EvidenceTimestamp: now.Add(-30 * time.Minute),
		Now:               now,
		RedactionReliable: false,
		SafeEvidence:      map[string]string{"unsafe": "dropped"},
	})
	if suppressed.FreshnessState != baseconnectors.FreshnessStale || suppressed.RedactionStatus != baseconnectors.RedactionStatusSuppressed {
		t.Fatalf("expected stale suppressed diagnostic, got %+v", suppressed)
	}
	if len(suppressed.SafeEvidence) != 0 {
		t.Fatalf("suppressed diagnostic should drop evidence, got %+v", suppressed.SafeEvidence)
	}
}
