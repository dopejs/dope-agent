package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadGroupRoomDetailProjectsScopedResetEvidence(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{
		ThreadID:                "thr_group_room_reset",
		TenantID:                "ten_threads",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_before",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack / #support",
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}
	if err := sqliteStore.UpsertThread(context.Background(), thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(context.Background(), threads.SessionSegment{SessionSegmentID: "seg_before", ThreadID: thread.ThreadID, TenantID: thread.TenantID, Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment: %v", err)
	}
	if _, err := sqliteStore.SaveConversationShapeEvidence(context.Background(), threads.ResolveConversationShape(threads.ConversationShapeResolutionInput{
		TenantID:                  thread.TenantID,
		ThreadID:                  thread.ThreadID,
		SessionSegmentID:          "seg_before",
		SourceKind:                threads.SourceKindChannel,
		ConnectorID:               "slack-main",
		ConnectorKind:             "slack",
		SourceAccountID:           "workspace_redacted",
		SourceConversationID:      "channel_redacted",
		SourceConversationSummary: "Slack / #support",
		ClaimedShape:              threads.ConversationShapeRoom,
		Now:                       now,
	})); err != nil {
		t.Fatalf("SaveConversationShapeEvidence: %v", err)
	}
	if _, _, err := sqliteStore.ApplyThreadLifecycleAction(context.Background(), thread.TenantID, thread.ThreadID, threads.LifecycleActionReset, threads.LifecycleMutationInput{
		ActorPrincipalID: "prn_threads",
		ReasonCode:       "operator_reset",
		AuditEventID:     "audit_reset_group_room",
		Now:              now.Add(time.Minute),
		NewSegmentID:     "seg_after",
	}); err != nil {
		t.Fatalf("ApplyThreadLifecycleAction reset: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/threads/"+thread.ThreadID, nil)
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	detail := decodeStrictResponse[threads.ThreadDetailResponse](t, rec.Body.Bytes())
	if len(detail.ResetEvents) != 1 ||
		detail.ResetEvents[0].ConversationShape != threads.ConversationShapeRoom ||
		detail.ResetEvents[0].PriorSessionSegmentID != "seg_before" ||
		detail.ResetEvents[0].ResultingSessionSegmentID != "seg_after" {
		t.Fatalf("detail reset events = %+v", detail.ResetEvents)
	}
}
