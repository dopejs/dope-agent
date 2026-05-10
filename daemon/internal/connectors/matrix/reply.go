package matrix

import (
	"context"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

func SendFinalReply(ctx context.Context, transport Transport, event InboundEvent, reply imtypes.OutboundReply) ReplyOutcome {
	if reply.ConnectorID == "" {
		reply.ConnectorID = event.ConnectorID
	}
	if reply.ChannelID == "" {
		reply.ChannelID = event.ConversationID
	}
	if reply.ReplyToExternalMessageID == "" {
		reply.ReplyToExternalMessageID = event.MatrixEventID
	}
	outcome := ReplyOutcome{
		TenantID:                  event.TenantID,
		ConnectorID:               event.ConnectorID,
		InboundEventIdentity:      DedupeKey(event),
		AssistantExecutionOutcome: "succeeded",
		MatrixReplyOutcome:        "sent",
		ReplyProgressionLevel:     "final_only",
		ReplyContext:              event.ConversationType,
		RedactionStatus:           baseconnectors.RedactionStatusRedacted,
	}
	if transport == nil {
		outcome.MatrixReplyOutcome = "not_attempted"
		outcome.FailureReasonCode = string(baseconnectors.DiagnosticReplyFailed)
		return outcome
	}
	if _, err := transport.SendReply(ctx, reply); err != nil {
		outcome.MatrixReplyOutcome = "failed"
		outcome.FailureReasonCode = string(baseconnectors.DiagnosticReplyFailed)
	}
	return outcome
}
