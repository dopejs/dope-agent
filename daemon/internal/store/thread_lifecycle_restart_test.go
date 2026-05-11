package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadLifecycleRestartPersistsStatesAndRecoveryEvidence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataDir := t.TempDir()
	sqliteStore, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	for _, state := range []threads.LifecycleState{
		threads.LifecycleStateActive,
		threads.LifecycleStateReset,
		threads.LifecycleStateArchived,
		threads.LifecycleStateReopened,
	} {
		threadID := "thr_restart_" + string(state)
		if err := sqliteStore.UpsertThread(ctx, threads.Thread{
			ThreadID:                threadID,
			TenantID:                "ten_restart",
			LifecycleState:          state,
			CurrentSessionSegmentID: "seg_" + string(state),
			SourceKind:              threads.SourceKindChannel,
			SourceSummary:           "restart " + string(state),
			LastActivityAt:          now,
			RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
			RedactionStatus:         threads.RedactionStatusRedacted,
			CreatedAt:               now,
			UpdatedAt:               now,
		}); err != nil {
			t.Fatalf("UpsertThread(%s): %v", state, err)
		}
		if err := sqliteStore.UpsertThreadSessionSegment(ctx, threads.SessionSegment{
			SessionSegmentID: "seg_" + string(state),
			ThreadID:         threadID,
			TenantID:         "ten_restart",
			SessionID:        "sess_" + string(state),
			Generation:       1,
			State:            "active",
			StartedAt:        now,
			LastActiveAt:     now,
		}); err != nil {
			t.Fatalf("UpsertThreadSessionSegment(%s): %v", state, err)
		}
	}
	if err := sqliteStore.UpsertSessionForTenantSafe(ctx, router.Session{
		SessionID:    "sess_legacy_restart",
		Kind:         router.SessionKindDirect,
		Status:       router.SessionStatusActive,
		Channel:      "discord",
		PeerID:       "user_restart",
		RoutingKey:   "direct:discord::user_restart",
		Generation:   1,
		CreatedAt:    now,
		UpdatedAt:    now,
		LastActiveAt: now,
	}, "ten_restart"); err != nil {
		t.Fatalf("UpsertSession legacy: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	restored, err := NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore restored: %v", err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	stats, err := restored.RecoverThreadLifecycleAfterRestart(ctx)
	if err != nil {
		t.Fatalf("RecoverThreadLifecycleAfterRestart: %v", err)
	}
	if stats.ProjectedLegacySessions != 1 || stats.PartialThreadStates != 0 {
		t.Fatalf("unexpected recovery stats: %+v", stats)
	}
	for _, state := range []threads.LifecycleState{
		threads.LifecycleStateActive,
		threads.LifecycleStateReset,
		threads.LifecycleStateArchived,
		threads.LifecycleStateReopened,
	} {
		threadID := "thr_restart_" + string(state)
		detail, found, err := restored.GetThreadDetailForTenant(ctx, "ten_restart", threadID)
		if err != nil || !found {
			t.Fatalf("GetThreadDetailForTenant(%s) found=%v err=%v", state, found, err)
		}
		if detail.Thread.LifecycleState != state || len(detail.SessionSegments) != 1 {
			t.Fatalf("state %s was not restart-persistent: %+v", state, detail)
		}
	}
	legacy, found, err := restored.GetThreadDetailForTenant(ctx, "ten_restart", "thr_legacy_sess_legacy_restart")
	if err != nil || !found || !legacy.SessionSegments[0].PartialEvidence {
		t.Fatalf("legacy session was not projected after restart found=%v detail=%+v err=%v", found, legacy, err)
	}
}
