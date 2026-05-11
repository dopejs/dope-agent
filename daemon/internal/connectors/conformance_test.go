package connectors_test

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestConnectorSourceLinkageConformanceSurvivesRestartReplay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dataDir := t.TempDir()
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{
		ThreadID:                "thr_replay",
		TenantID:                "ten_replay",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_replay",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack / channel_redacted",
		LastActivityAt:          now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := sqliteStore.UpsertThread(ctx, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	key := threads.SourceContinuationKey{
		TenantID:             "ten_replay",
		ConnectorID:          "slack-main",
		SourceAccountID:      "workspace_redacted",
		SourceConversationID: "channel_redacted",
	}
	if err := sqliteStore.SaveThreadSourceLinkage(ctx, threads.SourceLinkage{
		SourceLinkageID:      "src_replay_current",
		ThreadID:             "thr_replay",
		TenantID:             "ten_replay",
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          "slack-main",
		ConnectorKind:        "slack",
		SourceAccountID:      key.SourceAccountID,
		SourceConversationID: key.SourceConversationID,
		SourceMessageID:      "msg_1",
		RoutingOutcome:       threads.RoutingOutcomeAccepted,
		Current:              true,
		LinkedAt:             now,
		RedactionStatus:      threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveThreadSourceLinkage: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	restored, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore restored: %v", err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	current, found, err := restored.GetCurrentThreadForSource(ctx, key)
	if err != nil || !found {
		t.Fatalf("GetCurrentThreadForSource after restart found=%v err=%v", found, err)
	}
	if current.ThreadID != "thr_replay" || current.CurrentSessionSegmentID != "seg_replay" {
		t.Fatalf("connector replay source did not resolve to daemon-owned thread: %+v", current)
	}
}
