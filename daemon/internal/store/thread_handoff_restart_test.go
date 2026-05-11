package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestHandoffEvidenceSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	sqliteStore, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
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
		DestinationThreadID:          "thr_dest",
		SourceConversationShape:      threads.ConversationShapeRoom,
		DestinationConversationShape: threads.ConversationShapeWeb,
		SourceReferenceStatus:        threads.HandoffSourceReferenceConsumed,
		CreatedAt:                    now,
	})
	if err != nil {
		t.Fatalf("SaveHandoffLink: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := NewSQLiteStore(dir)
	if err != nil {
		t.Fatalf("reopen NewSQLiteStore: %v", err)
	}
	defer reopened.Close()
	got, found, err := reopened.GetHandoffLink(ctx, "ten_1", link.HandoffLinkID)
	if err != nil || !found {
		t.Fatalf("GetHandoffLink found=%v err=%v", found, err)
	}
	if got.SourceReferenceStatus != threads.HandoffSourceReferenceConsumed {
		t.Fatalf("source reference status after restart = %s", got.SourceReferenceStatus)
	}
}
