package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadContinuityMigrationPersistenceAndLookup(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	for _, table := range []string{"thread_continuity_turns", "thread_continuity_previews", "thread_continuity_preview_items"} {
		var name string
		if err := store.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	seedContinuityThread(t, ctx, store, "ten_1", "thr_1", "seg_1", now)
	first, err := store.SaveContinuityTurn(ctx, continuityStoreTestTurn("turn_1", "ten_1", "thr_1", "seg_1", 0, now))
	if err != nil {
		t.Fatalf("SaveContinuityTurn first: %v", err)
	}
	second, err := store.SaveContinuityTurn(ctx, continuityStoreTestTurn("turn_2", "ten_1", "thr_1", "seg_1", 0, now.Add(time.Minute)))
	if err != nil {
		t.Fatalf("SaveContinuityTurn second: %v", err)
	}
	if first.AcceptanceSequence != 1 || second.AcceptanceSequence != 2 {
		t.Fatalf("unexpected acceptance sequences: %d %d", first.AcceptanceSequence, second.AcceptanceSequence)
	}
	turns, err := store.ListContinuityTurns(ctx, ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: "seg_1", Now: now})
	if err != nil {
		t.Fatalf("ListContinuityTurns: %v", err)
	}
	if len(turns) != 2 || turns[0].ContinuityTurnID != "turn_2" || turns[1].ContinuityTurnID != "turn_1" {
		t.Fatalf("unexpected turns: %+v", turns)
	}

	preview, err := store.SaveContinuityPreview(ctx, threads.ContinuityPreview{
		ContinuityPreviewID: "contprev_1",
		TenantID:            "ten_1",
		ThreadID:            "thr_1",
		SessionSegmentID:    "seg_1",
		IncludedCount:       1,
		ExcludedCount:       1,
		ContinuityApplied:   true,
		Status:              threads.ContinuityStatusApplied,
		AssemblyStartedAt:   now,
		AssemblyCompletedAt: now.Add(10 * time.Millisecond),
		RedactionStatus:     threads.RedactionStatusRedacted,
	}, []threads.ContinuityPreviewItem{
		threads.PreviewItemForTurn(first, threads.ContinuityDecisionIncluded, threads.ContinuityReasonIncludedRecent, 0),
		threads.PreviewItemForTurn(second, threads.ContinuityDecisionExcluded, threads.ContinuityReasonOverLimit, 1),
	})
	if err != nil {
		t.Fatalf("SaveContinuityPreview: %v", err)
	}
	detail, found, err := store.GetContinuityPreviewDetail(ctx, "ten_1", "thr_1", preview.ContinuityPreviewID)
	if err != nil || !found {
		t.Fatalf("GetContinuityPreviewDetail found=%v err=%v", found, err)
	}
	if len(detail.Items) != 2 || detail.Preview.ContinuityPreviewID != "contprev_1" {
		t.Fatalf("unexpected preview detail: %+v", detail)
	}
	threadDetail, found, err := store.GetThreadDetailForTenant(ctx, "ten_1", "thr_1")
	if err != nil || !found {
		t.Fatalf("GetThreadDetailForTenant found=%v err=%v", found, err)
	}
	if len(threadDetail.ContinuityPreviews) != 1 {
		t.Fatalf("expected continuity preview summary, got %+v", threadDetail.ContinuityPreviews)
	}
}

func TestContinuityTurnResponseLinkageAndBoundedLookup(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	seedContinuityThread(t, ctx, store, "ten_1", "thr_1", "seg_1", now)
	request, err := store.SaveContinuityTurn(ctx, continuityStoreTestTurn("turn_user", "ten_1", "thr_1", "seg_1", 0, now))
	if err != nil {
		t.Fatalf("SaveContinuityTurn request: %v", err)
	}
	response := continuityStoreTestTurn("turn_assistant", "ten_1", "thr_1", "seg_1", 0, now.Add(time.Second))
	response.Role = threads.ContinuityRoleAssistant
	response.ResponseToTurnID = request.ContinuityTurnID
	response.SafeContent = "assistant response"
	response, err = store.SaveContinuityTurn(ctx, response)
	if err != nil {
		t.Fatalf("SaveContinuityTurn response: %v", err)
	}
	if response.ResponseToTurnID != request.ContinuityTurnID || response.AcceptanceSequence != request.AcceptanceSequence+1 {
		t.Fatalf("unexpected response linkage: request=%+v response=%+v", request, response)
	}

	for i := 0; i < 5; i++ {
		turn := continuityStoreTestTurn("turn_extra_"+string(rune('a'+i)), "ten_1", "thr_1", "seg_1", 0, now.Add(time.Duration(i+2)*time.Second))
		if _, err := store.SaveContinuityTurn(ctx, turn); err != nil {
			t.Fatalf("SaveContinuityTurn extra: %v", err)
		}
	}
	turns, err := store.ListContinuityTurns(ctx, ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: "seg_1", Limit: 3, Now: now})
	if err != nil {
		t.Fatalf("ListContinuityTurns: %v", err)
	}
	if len(turns) != 3 || turns[0].AcceptanceSequence != 7 || turns[2].AcceptanceSequence != 5 {
		t.Fatalf("expected three newest turns by daemon sequence, got %+v", turns)
	}
}

func TestContinuityTurnConcurrentSequenceAllocationConverges(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	seedContinuityThread(t, ctx, store, "ten_1", "thr_1", "seg_1", now)
	const count = 16
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			turn := continuityStoreTestTurn("turn_concurrent_"+string(rune('a'+index)), "ten_1", "thr_1", "seg_1", 0, now.Add(time.Duration(index)*time.Millisecond))
			_, err := store.SaveContinuityTurn(ctx, turn)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("SaveContinuityTurn concurrent returned error: %v", err)
		}
	}
	turns, err := store.ListContinuityTurns(ctx, ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: "seg_1", Limit: count, Now: now})
	if err != nil {
		t.Fatalf("ListContinuityTurns: %v", err)
	}
	if len(turns) != count {
		t.Fatalf("turns=%d want %d: %+v", len(turns), count, turns)
	}
	seen := map[int64]bool{}
	for _, turn := range turns {
		if turn.AcceptanceSequence < 1 || turn.AcceptanceSequence > count || seen[turn.AcceptanceSequence] {
			t.Fatalf("unexpected sequence allocation in %+v", turns)
		}
		seen[turn.AcceptanceSequence] = true
	}
}

func TestContinuityLookupAfterResetPreservesHistoricalPreviewEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	seedContinuityThread(t, ctx, store, "ten_1", "thr_1", "seg_old", now)
	thread, found, err := store.GetThreadForTenant(ctx, "ten_1", "thr_1")
	if err != nil || !found {
		t.Fatalf("GetThreadForTenant found=%v err=%v", found, err)
	}
	reset, _, _, err := threads.ResetThread(thread, threads.LifecycleMutationInput{
		ActorPrincipalID: "prn_1",
		ReasonCode:       "operator_reset",
		AuditEventID:     "audit_reset",
		Now:              now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ResetThread: %v", err)
	}
	if err := store.UpsertThread(ctx, reset); err != nil {
		t.Fatalf("UpsertThread reset: %v", err)
	}
	if err := store.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: reset.CurrentSessionSegmentID, ThreadID: "thr_1", TenantID: "ten_1", Generation: 2, State: "active", StartedAt: now.Add(time.Minute), LastActiveAt: now.Add(time.Minute), ResetFromSessionSegment: "seg_old"}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment reset: %v", err)
	}
	preReset, err := store.SaveContinuityTurn(ctx, continuityStoreTestTurn("turn_pre_reset", "ten_1", "thr_1", "seg_old", 0, now))
	if err != nil {
		t.Fatalf("SaveContinuityTurn pre-reset: %v", err)
	}
	postReset, err := store.SaveContinuityTurn(ctx, continuityStoreTestTurn("turn_post_reset", "ten_1", "thr_1", reset.CurrentSessionSegmentID, 0, now.Add(2*time.Minute)))
	if err != nil {
		t.Fatalf("SaveContinuityTurn post-reset: %v", err)
	}

	current, err := store.ListContinuityTurns(ctx, ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: reset.CurrentSessionSegmentID, Now: now.Add(3 * time.Minute)})
	if err != nil {
		t.Fatalf("ListContinuityTurns current: %v", err)
	}
	if len(current) != 1 || current[0].ContinuityTurnID != postReset.ContinuityTurnID {
		t.Fatalf("expected only post-reset current-segment turn, got %+v", current)
	}
	historical, err := store.ListContinuityTurnsOutsideSessionSegment(ctx, ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: reset.CurrentSessionSegmentID, Now: now.Add(3 * time.Minute)})
	if err != nil {
		t.Fatalf("ListContinuityTurnsOutsideSessionSegment: %v", err)
	}
	if len(historical) != 1 || historical[0].ContinuityTurnID != preReset.ContinuityTurnID {
		t.Fatalf("expected pre-reset historical turn, got %+v", historical)
	}

	preview, err := store.SaveContinuityPreview(ctx, threads.ContinuityPreview{
		ContinuityPreviewID: "contprev_reset",
		TenantID:            "ten_1",
		ThreadID:            "thr_1",
		SessionSegmentID:    reset.CurrentSessionSegmentID,
		ExcludedCount:       1,
		Status:              threads.ContinuityStatusEmpty,
		AssemblyStartedAt:   now.Add(3 * time.Minute),
		AssemblyCompletedAt: now.Add(3*time.Minute + time.Millisecond),
		RedactionStatus:     threads.RedactionStatusRedacted,
	}, threads.ResetBoundaryPreviewItems(historical, 0))
	if err != nil {
		t.Fatalf("SaveContinuityPreview reset: %v", err)
	}
	detail, found, err := store.GetContinuityPreviewDetail(ctx, "ten_1", "thr_1", preview.ContinuityPreviewID)
	if err != nil || !found {
		t.Fatalf("GetContinuityPreviewDetail found=%v err=%v", found, err)
	}
	if len(detail.Items) != 1 || detail.Items[0].ReasonCode != threads.ContinuityReasonResetBoundary {
		t.Fatalf("expected reset-boundary preview item, got %+v", detail)
	}
}

func TestContinuityPreviewPersistsArtifactExcerptRetentionAndRedaction(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	// Anchor to wall-clock: GetContinuityPreviewDetail filters by retention against
	// time.Now(), so a frozen past date makes the seeded preview/excerpt read as expired.
	now := time.Now().UTC()
	seedContinuityThread(t, ctx, store, "ten_1", "thr_1", "seg_1", now)
	preview, err := store.SaveContinuityPreview(ctx, threads.ContinuityPreview{
		ContinuityPreviewID: "contprev_artifact",
		TenantID:            "ten_1",
		ThreadID:            "thr_1",
		SessionSegmentID:    "seg_1",
		ExcludedCount:       1,
		Status:              threads.ContinuityStatusEmpty,
		AssemblyStartedAt:   now,
		AssemblyCompletedAt: now.Add(time.Millisecond),
		RetentionExpiresAt:  now.Add(30 * 24 * time.Hour),
		RedactionStatus:     threads.RedactionStatusRedacted,
	}, []threads.ContinuityPreviewItem{{
		TenantID:          "ten_1",
		ThreadID:          "thr_1",
		ItemKind:          threads.ContinuityItemArtifactExcerpt,
		ArtifactRef:       "run/run_1",
		ArtifactExcerptID: "artex_1",
		Decision:          threads.ContinuityDecisionExcluded,
		ReasonCode:        threads.ContinuityReasonArtifactReference,
		SafeSummary:       "artifact excerpt",
		RedactionStatus:   threads.RedactionStatusRedacted,
		ItemOrder:         0,
	}})
	if err != nil {
		t.Fatalf("SaveContinuityPreview artifact: %v", err)
	}
	detail, found, err := store.GetContinuityPreviewDetail(ctx, "ten_1", "thr_1", preview.ContinuityPreviewID)
	if err != nil || !found {
		t.Fatalf("GetContinuityPreviewDetail found=%v err=%v", found, err)
	}
	if detail.Preview.RetentionExpiresAt.IsZero() || len(detail.Items) != 1 {
		t.Fatalf("expected retained preview detail, got %+v", detail)
	}
	if detail.Items[0].ItemKind != threads.ContinuityItemArtifactExcerpt || detail.Items[0].ArtifactExcerptID != "artex_1" || detail.Items[0].RedactionStatus != threads.RedactionStatusRedacted {
		t.Fatalf("unexpected artifact item: %+v", detail.Items[0])
	}
}

func TestContinuityLookupExcludesPartialEvidenceSegments(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	seedContinuityThread(t, ctx, store, "ten_1", "thr_1", "seg_partial", now)
	if err := store.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: "seg_partial", ThreadID: "thr_1", TenantID: "ten_1", Generation: 1, State: "active", StartedAt: now, LastActiveAt: now, PartialEvidence: true}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment partial: %v", err)
	}
	if _, err := store.SaveContinuityTurn(ctx, continuityStoreTestTurn("turn_partial", "ten_1", "thr_1", "seg_partial", 0, now)); err != nil {
		t.Fatalf("SaveContinuityTurn partial: %v", err)
	}
	turns, err := store.ListContinuityTurns(ctx, ContinuityLookupQuery{TenantID: "ten_1", ThreadID: "thr_1", SessionSegmentID: "seg_partial", Now: now})
	if err != nil {
		t.Fatalf("ListContinuityTurns: %v", err)
	}
	if len(turns) != 0 {
		t.Fatalf("expected partial evidence segment excluded from continuity, got %+v", turns)
	}
}

func seedContinuityThread(t *testing.T, ctx context.Context, store *SQLiteStore, tenantID, threadID, segmentID string, now time.Time) {
	t.Helper()
	if err := store.UpsertThread(ctx, threads.Thread{
		ThreadID:                threadID,
		TenantID:                tenantID,
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: segmentID,
		SourceKind:              threads.SourceKindChat,
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := store.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: segmentID, ThreadID: threadID, TenantID: tenantID, Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment: %v", err)
	}
}

func continuityStoreTestTurn(id, tenantID, threadID, segmentID string, seq int64, now time.Time) threads.ContinuityTurn {
	return threads.ContinuityTurn{
		ContinuityTurnID:       id,
		TenantID:               tenantID,
		ThreadID:               threadID,
		SessionSegmentID:       segmentID,
		AcceptanceSequence:     seq,
		Role:                   threads.ContinuityRoleUser,
		SourceKind:             threads.SourceKindChat,
		SafeContent:            "safe " + id,
		ContentRedactionStatus: threads.RedactionStatusRedacted,
		RecordedAt:             now,
		RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
	}
}
