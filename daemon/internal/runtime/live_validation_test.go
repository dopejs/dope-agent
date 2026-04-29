package runtime

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestRuntimeLocalToolCallLiveValidationClassification(t *testing.T) {
	rows := LiveValidationMatrixRows()
	if len(rows) != 1 {
		t.Fatalf("len(rows)=%d, want 1", len(rows))
	}
	row := rows[0]
	if row.ToolClass != livevalidation.ToolClassRuntimeLocalToolCall || row.SafetyClass != livevalidation.SafetyClassIdempotentMutation {
		t.Fatalf("unexpected runtime classification: %+v", row)
	}
	if row.RetryPolicy != livevalidation.RetryPolicyAutomatic {
		t.Fatalf("RetryPolicy=%s, want automatic", row.RetryPolicy)
	}
}
