package delivery

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestDeliveryLiveValidationRowsClassifyDispatchAndConnectorSend(t *testing.T) {
	rows := LiveValidationMatrixRows()
	if len(rows) != 2 {
		t.Fatalf("len(rows)=%d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.SafetyClass != livevalidation.SafetyClassNonIdempotentMutation || row.Approval != livevalidation.MatrixApprovalPerAction {
			t.Fatalf("delivery row=%+v, want non-idempotent per-action", row)
		}
	}
}
