package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestGroupRoomMigrationAndEvidencePersistence(t *testing.T) {
	ctx := context.Background()
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer sqliteStore.Close()

	for _, table := range []string{"thread_conversation_shapes", "thread_participation_decisions", "thread_reset_events", "thread_handoff_links", "thread_handoff_source_references"} {
		var name string
		if err := sqliteStore.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{
		ThreadID:                "thr_room",
		TenantID:                "ten_1",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_room",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack / #support",
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}
	if err := sqliteStore.UpsertThread(ctx, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	shape, err := sqliteStore.SaveConversationShapeEvidence(ctx, threads.ConversationShapeEvidence{
		TenantID:             "ten_1",
		ThreadID:             "thr_room",
		SessionSegmentID:     "seg_room",
		Shape:                threads.ConversationShapeRoom,
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          "slack-main",
		SourceAccountID:      "workspace_redacted",
		SourceConversationID: "channel_redacted",
		RecordedAt:           now,
	})
	if err != nil {
		t.Fatalf("SaveConversationShapeEvidence: %v", err)
	}
	if shape.ConversationShapeID == "" {
		t.Fatal("expected allocated conversation shape id")
	}
	decision, err := sqliteStore.SaveParticipationDecision(ctx, threads.ParticipationDecision{
		TenantID:             "ten_1",
		ThreadID:             "thr_room",
		SessionSegmentID:     "seg_room",
		ConnectorID:          "slack-main",
		SourceAccountID:      "workspace_redacted",
		SourceConversationID: "channel_redacted",
		SourceMessageID:      "msg_1",
		ConversationShape:    threads.ConversationShapeRoom,
		MentionStatus:        threads.MentionStatusQualified,
		AllowlistStatus:      threads.AllowlistStatusEligible,
		Decision:             threads.ParticipationDecisionAccepted,
		ReasonCode:           threads.GroupRoomReasonAcceptedQualifyingMention,
		CreatedAssistantWork: true,
		OccurredAt:           now,
	})
	if err != nil {
		t.Fatalf("SaveParticipationDecision: %v", err)
	}
	duplicate, err := sqliteStore.SaveParticipationDecision(ctx, threads.ParticipationDecision{
		TenantID:             "ten_1",
		ThreadID:             "thr_room",
		SessionSegmentID:     "seg_room",
		ConnectorID:          "slack-main",
		SourceAccountID:      "workspace_redacted",
		SourceConversationID: "channel_redacted",
		SourceMessageID:      "msg_1",
		ConversationShape:    threads.ConversationShapeRoom,
		Decision:             threads.ParticipationDecisionDuplicate,
		ReasonCode:           threads.GroupRoomReasonDuplicateSourceEvent,
		OccurredAt:           now,
	})
	if err != nil {
		t.Fatalf("SaveParticipationDecision duplicate: %v", err)
	}
	if duplicate.ParticipationDecisionID != decision.ParticipationDecisionID {
		t.Fatalf("duplicate source event created new decision: %s != %s", duplicate.ParticipationDecisionID, decision.ParticipationDecisionID)
	}
	detail, found, err := sqliteStore.GetThreadDetailForTenant(ctx, "ten_1", "thr_room")
	if err != nil || !found {
		t.Fatalf("GetThreadDetailForTenant found=%v err=%v", found, err)
	}
	if detail.ConversationShape == nil || detail.ConversationShape.Shape != threads.ConversationShapeRoom {
		t.Fatalf("detail conversation shape = %#v", detail.ConversationShape)
	}
	if len(detail.ParticipationDecisions) != 1 || detail.ParticipationDecisions[0].Decision != threads.ParticipationDecisionAccepted {
		t.Fatalf("detail participation decisions = %#v", detail.ParticipationDecisions)
	}
	resetEvent, err := sqliteStore.SaveResetEvent(ctx, threads.ResetEvent{
		TenantID:                  "ten_1",
		ThreadID:                  "thr_room",
		ConversationShape:         threads.ConversationShapeRoom,
		SourceConversationID:      "channel_redacted",
		ActorPrincipalID:          "prn_1",
		PermissionGate:            "connectors.manage",
		PriorSessionSegmentID:     "seg_room",
		ResultingSessionSegmentID: "seg_room_reset",
		Status:                    threads.ResetEventStatusSucceeded,
		ReasonCode:                threads.GroupRoomReasonScopedResetSucceeded,
		RequestedAt:               now,
		CompletedAt:               now,
		RedactionStatus:           threads.RedactionStatusRedacted,
	})
	if err != nil {
		t.Fatalf("SaveResetEvent: %v", err)
	}
	if resetEvent.ResetEventID == "" {
		t.Fatal("expected allocated reset event id")
	}
	detail, found, err = sqliteStore.GetThreadDetailForTenant(ctx, "ten_1", "thr_room")
	if err != nil || !found {
		t.Fatalf("GetThreadDetailForTenant reset found=%v err=%v", found, err)
	}
	if len(detail.ResetEvents) != 1 || detail.ResetEvents[0].ConversationShape != threads.ConversationShapeRoom || detail.ResetEvents[0].SourceConversationID != "channel_redacted" {
		t.Fatalf("detail reset events = %#v", detail.ResetEvents)
	}
}
