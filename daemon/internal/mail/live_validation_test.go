package mail

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestMailLiveValidationMatrixRowsClassifyDraftSendReplyForward(t *testing.T) {
	rows := LiveValidationMatrixRows()
	if len(rows) != 5 {
		t.Fatalf("len(rows)=%d, want 5", len(rows))
	}
	byClass := map[livevalidation.ToolClass]livevalidation.MatrixRow{}
	for _, row := range rows {
		byClass[row.ToolClass] = row
	}
	if byClass[livevalidation.ToolClassMailDraftCreate].Approval != livevalidation.MatrixApprovalScopeLevel {
		t.Fatalf("draft create row=%+v, want scope approval", byClass[livevalidation.ToolClassMailDraftCreate])
	}
	for _, toolClass := range []livevalidation.ToolClass{livevalidation.ToolClassMailSend, livevalidation.ToolClassMailReply, livevalidation.ToolClassMailForward} {
		row := byClass[toolClass]
		if row.SafetyClass != livevalidation.SafetyClassNonIdempotentMutation || row.RetryPolicy != livevalidation.RetryPolicyNone || row.Approval != livevalidation.MatrixApprovalPerAction {
			t.Fatalf("%s row=%+v, want non-idempotent per-action no-retry", toolClass, row)
		}
	}
}
