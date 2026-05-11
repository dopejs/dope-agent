package events

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestGroupRoomResetHandoffEventsUseSafeMetadata(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	participation := ThreadParticipationDecisionEvent(threads.ParticipationDecision{
		ParticipationDecisionID: "part_1",
		TenantID:                "ten_1",
		ThreadID:                "thr_1",
		SessionSegmentID:        "seg_1",
		ConnectorID:             "slack-main",
		ConversationShape:       threads.ConversationShapeRoom,
		Decision:                threads.ParticipationDecisionIgnored,
		ReasonCode:              threads.GroupRoomReasonMissingQualifyingMention,
		OccurredAt:              now,
		RedactionStatus:         threads.RedactionStatusRedacted,
	})
	if participation.Name != ThreadParticipationDecisionRecordedName || participation.Payload["decision"] != "ignored" || participation.Payload["redactionStatus"] != "redacted" {
		t.Fatalf("participation event = %+v", participation)
	}
	reset := ThreadScopedResetEvidenceEvent(threads.ResetEvent{
		ResetEventID:              "reset_1",
		TenantID:                  "ten_1",
		ThreadID:                  "thr_1",
		ConversationShape:         threads.ConversationShapeRoom,
		PermissionGate:            "connectors.manage",
		ResultingSessionSegmentID: "seg_2",
		Status:                    threads.ResetEventStatusSucceeded,
		ReasonCode:                threads.GroupRoomReasonScopedResetSucceeded,
		CompletedAt:               now,
		RedactionStatus:           threads.RedactionStatusRedacted,
	})
	if reset.Name != ThreadResetScopedName || reset.Payload["conversationShape"] != "room" || reset.Payload["permissionGate"] != "connectors.manage" {
		t.Fatalf("reset event = %+v", reset)
	}
	handoff := ThreadHandoffLinkedEvent(threads.HandoffLink{
		HandoffLinkID:                "handoff_1",
		TenantID:                     "ten_1",
		SourceThreadID:               "thr_source",
		DestinationThreadID:          "thr_destination",
		SourceConversationShape:      threads.ConversationShapeRoom,
		DestinationConversationShape: threads.ConversationShapeWeb,
		Status:                       threads.HandoffStatusSucceeded,
		ReasonCode:                   "user_requested_handoff",
		PermissionGate:               "connectors.manage",
		SourceReferenceStatus:        threads.HandoffSourceReferenceAvailable,
		CreatedAt:                    now,
		RedactionStatus:              threads.RedactionStatusRedacted,
	})
	if handoff.Name != ThreadHandoffLinkedName || handoff.Payload["sourceThreadId"] != "thr_source" || handoff.Payload["sourceReferenceStatus"] != "available" {
		t.Fatalf("handoff event = %+v", handoff)
	}
}
