package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestContinuityTurnsAndPreviewsSurviveRestart(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "dope-data")
	sqliteStore, err := NewSQLiteStore(root)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	for _, item := range []struct {
		threadID string
		segment  string
		state    threads.LifecycleState
	}{
		{"thr_active", "seg_active", threads.LifecycleStateActive},
		{"thr_reset", "seg_reset", threads.LifecycleStateReset},
		{"thr_archived", "seg_archived", threads.LifecycleStateArchived},
		{"thr_reopened", "seg_reopened", threads.LifecycleStateReopened},
	} {
		if err := sqliteStore.UpsertThread(ctx, threads.Thread{
			ThreadID:                item.threadID,
			TenantID:                "ten_restart",
			LifecycleState:          item.state,
			CurrentSessionSegmentID: item.segment,
			SourceKind:              threads.SourceKindChat,
			LastActivityAt:          now,
			CreatedAt:               now,
			UpdatedAt:               now,
			RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
			RedactionStatus:         threads.RedactionStatusRedacted,
		}); err != nil {
			t.Fatalf("UpsertThread %s: %v", item.threadID, err)
		}
		if err := sqliteStore.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: item.segment, ThreadID: item.threadID, TenantID: "ten_restart", Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
			t.Fatalf("UpsertThreadSessionSegment %s: %v", item.threadID, err)
		}
		turn, err := sqliteStore.SaveContinuityTurn(ctx, continuityStoreTestTurn("turn_"+item.threadID, "ten_restart", item.threadID, item.segment, 0, now))
		if err != nil {
			t.Fatalf("SaveContinuityTurn %s: %v", item.threadID, err)
		}
		if _, err := sqliteStore.SaveContinuityPreview(ctx, threads.ContinuityPreview{
			ContinuityPreviewID: "contprev_" + item.threadID,
			TenantID:            "ten_restart",
			ThreadID:            item.threadID,
			SessionSegmentID:    item.segment,
			IncludedCount:       1,
			ContinuityApplied:   true,
			Status:              threads.ContinuityStatusApplied,
			AssemblyStartedAt:   now,
			AssemblyCompletedAt: now.Add(time.Millisecond),
			RedactionStatus:     threads.RedactionStatusRedacted,
		}, []threads.ContinuityPreviewItem{threads.PreviewItemForTurn(turn, threads.ContinuityDecisionIncluded, threads.ContinuityReasonIncludedRecent, 0)}); err != nil {
			t.Fatalf("SaveContinuityPreview %s: %v", item.threadID, err)
		}
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteStore(root)
	if err != nil {
		t.Fatalf("NewSQLiteStore reopen: %v", err)
	}
	defer reopened.Close()
	for _, threadID := range []string{"thr_active", "thr_reset", "thr_archived", "thr_reopened"} {
		detail, found, err := reopened.GetThreadDetailForTenant(ctx, "ten_restart", threadID)
		if err != nil || !found {
			t.Fatalf("GetThreadDetailForTenant %s found=%v err=%v", threadID, found, err)
		}
		if len(detail.ContinuityPreviews) != 1 || detail.ContinuityPreviews[0].ContinuityPreviewID != "contprev_"+threadID {
			t.Fatalf("expected persisted preview for %s, got %+v", threadID, detail.ContinuityPreviews)
		}
		turns, err := reopened.ListContinuityTurns(ctx, ContinuityLookupQuery{TenantID: "ten_restart", ThreadID: threadID, SessionSegmentID: detail.Thread.CurrentSessionSegmentID, Now: now})
		if err != nil {
			t.Fatalf("ListContinuityTurns %s: %v", threadID, err)
		}
		if len(turns) != 1 || turns[0].AcceptanceSequence != 1 {
			t.Fatalf("expected persisted sequence for %s, got %+v", threadID, turns)
		}
	}
}
