package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestBackgroundDeliveryProjectionAppearsInThreadDetail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{
		ThreadID:                "thr_delivery",
		TenantID:                "ten_delivery",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_delivery",
		SourceKind:              threads.SourceKindWorkflow,
		SourceSummary:           "delivery workflow",
		LastActivityAt:          now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := sqliteStore.UpsertThread(ctx, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(ctx, threads.SessionSegment{
		SessionSegmentID: "seg_delivery",
		ThreadID:         "thr_delivery",
		TenantID:         "ten_delivery",
		SessionID:        "sess_delivery",
		Generation:       1,
		State:            "active",
		StartedAt:        now,
		LastActiveAt:     now,
	}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment: %v", err)
	}
	if err := sqliteStore.SaveThreadRuntimeProjection(ctx, threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{
		ProjectionID:     "rtp_delivery_1",
		ThreadID:         "thr_delivery",
		TenantID:         "ten_delivery",
		SessionSegmentID: "seg_delivery",
		ResourceKind:     threads.RuntimeResourceBackgroundDelivery,
		ResourceID:       "delivery_1",
		Status:           string(OutcomeStatusDelivered),
		ReasonCode:       "sent",
		OccurredAt:       now,
		SafeSummary:      "Background delivery delivered",
	})); err != nil {
		t.Fatalf("SaveThreadRuntimeProjection: %v", err)
	}

	detail, found, err := sqliteStore.GetThreadDetailForTenant(ctx, "ten_delivery", "thr_delivery")
	if err != nil || !found {
		t.Fatalf("GetThreadDetailForTenant found=%v err=%v", found, err)
	}
	if len(detail.RuntimeProjections) != 1 || detail.RuntimeProjections[0].ResourceKind != threads.RuntimeResourceBackgroundDelivery {
		t.Fatalf("expected background delivery projection in detail, got %+v", detail.RuntimeProjections)
	}
}
