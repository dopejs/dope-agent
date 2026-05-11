package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestHandoffLinkAndSourceReferencePersistence(t *testing.T) {
	ctx := context.Background()
	sqliteStore, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer sqliteStore.Close()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	for _, threadID := range []string{"thr_source", "thr_dest"} {
		if err := sqliteStore.UpsertThread(ctx, threads.Thread{
			ThreadID:                threadID,
			TenantID:                "ten_1",
			LifecycleState:          threads.LifecycleStateActive,
			CurrentSessionSegmentID: "seg_" + threadID,
			SourceKind:              threads.SourceKindChannel,
			LastActivityAt:          now,
			CreatedAt:               now,
			UpdatedAt:               now,
			RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
			RedactionStatus:         threads.RedactionStatusRedacted,
		}); err != nil {
			t.Fatalf("UpsertThread %s: %v", threadID, err)
		}
	}
	link, err := sqliteStore.SaveHandoffLink(ctx, threads.HandoffLink{
		TenantID:                     "ten_1",
		SourceThreadID:               "thr_source",
		SourceSessionSegmentID:       "seg_thr_source",
		DestinationThreadID:          "thr_dest",
		DestinationSessionSegmentID:  "seg_thr_dest",
		SourceConversationShape:      threads.ConversationShapeRoom,
		DestinationConversationShape: threads.ConversationShapeWeb,
		SourceReferenceStatus:        threads.HandoffSourceReferenceAvailable,
		ReasonCode:                   "user_requested_handoff",
		CreatedAt:                    now,
	})
	if err != nil {
		t.Fatalf("SaveHandoffLink: %v", err)
	}
	refs, err := sqliteStore.SaveHandoffSourceReferences(ctx, []threads.HandoffSourceReference{{
		HandoffLinkID:               link.HandoffLinkID,
		TenantID:                    "ten_1",
		SourceThreadID:              "thr_source",
		SourceSessionSegmentID:      "seg_thr_source",
		DestinationThreadID:         "thr_dest",
		DestinationSessionSegmentID: "seg_thr_dest",
		ContinuityTurnID:            "turn_1",
		EligibilityStatus:           threads.HandoffReferenceEligible,
		Decision:                    threads.HandoffReferenceDecisionReferenced,
		SafeSummary:                 "safe source summary",
		CreatedAt:                   now,
	}})
	if err != nil {
		t.Fatalf("SaveHandoffSourceReferences: %v", err)
	}
	if refs[0].HandoffSourceReferenceID == "" {
		t.Fatal("expected source reference id")
	}
	links, err := sqliteStore.ListHandoffLinksForThread(ctx, "ten_1", "thr_dest", 10)
	if err != nil {
		t.Fatalf("ListHandoffLinksForThread: %v", err)
	}
	if len(links) != 1 || links[0].HandoffLinkID != link.HandoffLinkID {
		t.Fatalf("links = %#v", links)
	}
	detail, found, err := sqliteStore.GetThreadDetailForTenant(ctx, "ten_1", "thr_dest")
	if err != nil || !found {
		t.Fatalf("GetThreadDetailForTenant found=%v err=%v", found, err)
	}
	if len(detail.HandoffLinks) != 1 || detail.HandoffLinks[0].SourceThreadID != "thr_source" {
		t.Fatalf("detail handoff links = %#v", detail.HandoffLinks)
	}
}
