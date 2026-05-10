package matrix

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

func TestFinalReplyOutcomeSeparatesAssistantAndMatrixReplyTruth(t *testing.T) {
	t.Parallel()

	transport := NewFakeTransport()
	outcome := SendFinalReply(context.Background(), transport, InboundEvent{
		TenantID:         "ten",
		ConnectorID:      "matrix-main",
		ConversationID:   "!room:example.org",
		MatrixEventID:    "$event",
		ConversationType: ConversationRoom,
	}, imtypes.OutboundReply{Content: "done"})
	if outcome.AssistantExecutionOutcome != "succeeded" || outcome.MatrixReplyOutcome != "sent" || outcome.ReplyProgressionLevel != "final_only" {
		t.Fatalf("unexpected reply outcome: %+v", outcome)
	}
}
