package reminders

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestReminderLifecycleLiveValidationClassification(t *testing.T) {
	rows := LiveValidationMatrixRows()
	if len(rows) != 1 {
		t.Fatalf("len(rows)=%d, want 1", len(rows))
	}
	row := rows[0]
	if row.ToolClass != livevalidation.ToolClassReminderLifecycleMutation || row.SafetyClass != livevalidation.SafetyClassIdempotentMutation {
		t.Fatalf("unexpected reminder classification: %+v", row)
	}
}
