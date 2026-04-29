package calendar

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestCalendarLiveValidationMatrixRowsClassifyCreateUpdateCancel(t *testing.T) {
	rows := LiveValidationMatrixRows()
	if len(rows) != 3 {
		t.Fatalf("len(rows)=%d, want 3", len(rows))
	}
	byClass := map[livevalidation.ToolClass]livevalidation.MatrixRow{}
	for _, row := range rows {
		byClass[row.ToolClass] = row
	}
	if byClass[livevalidation.ToolClassCalendarEventCreate].Approval != livevalidation.MatrixApprovalPerAction {
		t.Fatalf("create row=%+v, want per-action approval", byClass[livevalidation.ToolClassCalendarEventCreate])
	}
	if byClass[livevalidation.ToolClassCalendarEventUpdate].SafetyClass != livevalidation.SafetyClassIdempotentMutation {
		t.Fatalf("update row=%+v, want idempotent mutation", byClass[livevalidation.ToolClassCalendarEventUpdate])
	}
	if byClass[livevalidation.ToolClassCalendarEventCancel].RetryPolicy != livevalidation.RetryPolicyNone {
		t.Fatalf("cancel row=%+v, want no retry", byClass[livevalidation.ToolClassCalendarEventCancel])
	}
}
