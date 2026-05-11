package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadLifecycleListDetailPaginationAndDenial(t *testing.T) {
	t.Parallel()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	for _, thread := range []threads.Thread{
		{ThreadID: "thr_active", TenantID: "ten_threads", LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_active", SourceKind: threads.SourceKindChannel, SourceSummary: "Slack Main / #support", LastActivityAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now.Add(time.Minute), RedactionStatus: threads.RedactionStatusRedacted},
		{ThreadID: "thr_archived", TenantID: "ten_threads", LifecycleState: threads.LifecycleStateArchived, CurrentSessionSegmentID: "seg_archived", SourceKind: threads.SourceKindWorkflow, SourceSummary: "Workflow", LastActivityAt: now.Add(2 * time.Minute), CreatedAt: now, UpdatedAt: now.Add(2 * time.Minute), RedactionStatus: threads.RedactionStatusRedacted},
		{ThreadID: "thr_other", TenantID: "ten_other", LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_other", SourceKind: threads.SourceKindChannel, SourceSummary: "Other", LastActivityAt: now.Add(3 * time.Minute), CreatedAt: now, UpdatedAt: now.Add(3 * time.Minute), RedactionStatus: threads.RedactionStatusRedacted},
	} {
		thread.RetentionExpiresAt = now.Add(90 * 24 * time.Hour)
		if err := sqliteStore.UpsertThread(context.Background(), thread); err != nil {
			t.Fatalf("UpsertThread: %v", err)
		}
	}
	if err := sqliteStore.SaveThreadSourceLinkage(context.Background(), threads.SourceLinkage{
		SourceLinkageID:      "src_active",
		ThreadID:             "thr_active",
		TenantID:             "ten_threads",
		SourceKind:           threads.SourceKindChannel,
		ConnectorID:          "slack-main",
		ConnectorKind:        "slack",
		SourceAccountID:      "workspace_redacted",
		SourceConversationID: "channel_redacted",
		SourceMessageID:      "msg_redacted",
		RoutingOutcome:       threads.RoutingOutcomeAccepted,
		Current:              true,
		LinkedAt:             now,
		RedactionStatus:      threads.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveThreadSourceLinkage: %v", err)
	}
	if err := sqliteStore.SaveThreadRuntimeProjection(context.Background(), threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{
		ProjectionID:     "rtp_run_active",
		ThreadID:         "thr_active",
		TenantID:         "ten_threads",
		SessionSegmentID: "seg_active",
		ResourceKind:     threads.RuntimeResourceRun,
		ResourceID:       "run_1",
		Status:           "completed",
		ReasonCode:       "accepted",
		OccurredAt:       now,
		Route:            "/v1/runs/run_1",
		SafeSummary:      "Assistant run completed",
	})); err != nil {
		t.Fatalf("SaveThreadRuntimeProjection: %v", err)
	}
	if _, err := sqliteStore.SaveConversationShapeEvidence(context.Background(), threads.ResolveConversationShape(threads.ConversationShapeResolutionInput{
		TenantID:                  "ten_threads",
		ThreadID:                  "thr_active",
		SessionSegmentID:          "seg_active",
		SourceKind:                threads.SourceKindChannel,
		ConnectorID:               "slack-main",
		ConnectorKind:             "slack",
		SourceAccountID:           "workspace_redacted",
		SourceConversationID:      "channel_redacted",
		SourceConversationSummary: "Slack Main / #support",
		ClaimedShape:              threads.ConversationShapeRoom,
		Now:                       now,
	})); err != nil {
		t.Fatalf("SaveConversationShapeEvidence: %v", err)
	}
	if _, err := sqliteStore.SaveParticipationDecision(context.Background(), threads.ParticipationDecision{
		TenantID:             "ten_threads",
		ThreadID:             "thr_active",
		SessionSegmentID:     "seg_active",
		ConnectorID:          "slack-main",
		SourceAccountID:      "workspace_redacted",
		SourceConversationID: "channel_redacted",
		SourceMessageID:      "msg_redacted",
		ConversationShape:    threads.ConversationShapeRoom,
		MentionStatus:        threads.MentionStatusMissing,
		AllowlistStatus:      threads.AllowlistStatusEligible,
		Decision:             threads.ParticipationDecisionIgnored,
		ReasonCode:           threads.GroupRoomReasonMissingQualifyingMention,
		CreatedAssistantWork: false,
		OccurredAt:           now,
		RedactionStatus:      threads.RedactionStatusRedacted,
		SafeSummary:          "Room message ignored by participation policy",
	}); err != nil {
		t.Fatalf("SaveParticipationDecision: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/threads?limit=1", nil)
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var list threads.ThreadListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if list.TenantID != "ten_threads" || len(list.Items) != 1 || list.Items[0].ThreadID != "thr_active" || list.Page.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v", list)
	}
	nextReq := httptest.NewRequest(http.MethodGet, "/v1/threads?limit=1&cursor="+list.Page.NextCursor, nil)
	nextReq = nextReq.WithContext(withTenantContext(nextReq.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	nextRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, nextRec, nextReq)
	if nextRec.Code != http.StatusOK {
		t.Fatalf("next page status=%d body=%s", nextRec.Code, nextRec.Body.String())
	}
	var nextList threads.ThreadListResponse
	if err := json.Unmarshal(nextRec.Body.Bytes(), &nextList); err != nil {
		t.Fatalf("decode next list: %v", err)
	}
	if len(nextList.Items) != 1 || nextList.Items[0].ThreadID != "thr_archived" {
		t.Fatalf("unexpected second page: %+v", nextList)
	}

	filterReq := httptest.NewRequest(http.MethodGet, "/v1/threads?state=archived&sourceKind=workflow", nil)
	filterReq = filterReq.WithContext(withTenantContext(filterReq.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	filterRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, filterRec, filterReq)
	var filtered threads.ThreadListResponse
	if err := json.Unmarshal(filterRec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("decode filtered list: %v", err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].ThreadID != "thr_archived" {
		t.Fatalf("unexpected filtered list: %+v", filtered)
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/v1/threads/thr_active", nil)
	detailReq = detailReq.WithContext(withTenantContext(detailReq.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	detailRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
	var detail threads.ThreadDetailResponse
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Thread.ThreadID != "thr_active" || detail.Thread.TenantID != "ten_threads" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if len(detail.SourceLinkages) != 1 || detail.SourceLinkages[0].RoutingOutcome != threads.RoutingOutcomeAccepted {
		t.Fatalf("detail missing source trace: %+v", detail.SourceLinkages)
	}
	if len(detail.RuntimeProjections) != 1 || detail.RuntimeProjections[0].ResourceKind != threads.RuntimeResourceRun {
		t.Fatalf("detail missing runtime trace: %+v", detail.RuntimeProjections)
	}
	if detail.ConversationShape == nil || detail.ConversationShape.Shape != threads.ConversationShapeRoom {
		t.Fatalf("detail missing conversation shape: %+v", detail.ConversationShape)
	}
	if len(detail.ParticipationDecisions) != 1 || detail.ParticipationDecisions[0].ReasonCode != threads.GroupRoomReasonMissingQualifyingMention {
		t.Fatalf("detail missing participation decision: %+v", detail.ParticipationDecisions)
	}
	for _, forbidden := range []string{"semanticSummary", "recalledMemory", "contextPacking", "autonomousPruning"} {
		if bytes.Contains(detailRec.Body.Bytes(), []byte(forbidden)) {
			t.Fatalf("detail response leaked memory behavior field %s: %s", forbidden, detailRec.Body.String())
		}
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/v1/threads", nil)
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), threadTenantContext()))
	deniedRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("denied status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	revokedReq := httptest.NewRequest(http.MethodGet, "/v1/threads/thr_active", nil)
	revokedReq = revokedReq.WithContext(withTenantContext(revokedReq.Context(), threadTenantContext()))
	revokedRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, revokedRec, revokedReq)
	if revokedRec.Code != http.StatusForbidden {
		t.Fatalf("revoked detail status=%d body=%s", revokedRec.Code, revokedRec.Body.String())
	}

	emptyReq := httptest.NewRequest(http.MethodGet, "/v1/threads", nil)
	emptyReq = emptyReq.WithContext(withTenantContext(emptyReq.Context(), identity.TenantContext{TenantID: "ten_empty", PrincipalID: "prn_empty", Permissions: []identity.Permission{identity.PermissionCredentialsInspect}}))
	emptyRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, emptyRec, emptyReq)
	var empty threads.ThreadListResponse
	if err := json.Unmarshal(emptyRec.Body.Bytes(), &empty); err != nil {
		t.Fatalf("decode empty list: %v", err)
	}
	if empty.TenantID != "ten_empty" || len(empty.Items) != 0 {
		t.Fatalf("unexpected empty state: %+v", empty)
	}
}

func TestThreadLifecycleMutationsRequireManagePermissionAndPersistAudit(t *testing.T) {
	t.Parallel()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{ThreadID: "thr_mutate", TenantID: "ten_threads", LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_1", SourceKind: threads.SourceKindChannel, SourceSummary: "Slack", LastActivityAt: now, CreatedAt: now, UpdatedAt: now, RedactionStatus: threads.RedactionStatusRedacted}
	if err := sqliteStore.UpsertThread(context.Background(), thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(context.Background(), threads.SessionSegment{SessionSegmentID: "seg_1", ThreadID: "thr_mutate", TenantID: "ten_threads", SessionID: "sess_1", Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment: %v", err)
	}
	if err := sqliteStore.SaveThreadSourceLinkage(context.Background(), threads.SourceLinkage{SourceLinkageID: "src_mutate_current", ThreadID: "thr_mutate", TenantID: "ten_threads", SourceKind: threads.SourceKindChannel, ConnectorID: "slack-main", ConnectorKind: "slack", SourceAccountID: "acct_redacted", SourceConversationID: "conv_redacted", RoutingOutcome: threads.RoutingOutcomeAccepted, Current: true, LinkedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted}); err != nil {
		t.Fatalf("SaveThreadSourceLinkage: %v", err)
	}
	if _, err := sqliteStore.SaveConversationShapeEvidence(context.Background(), threads.ResolveConversationShape(threads.ConversationShapeResolutionInput{
		TenantID:                  "ten_threads",
		ThreadID:                  "thr_mutate",
		SessionSegmentID:          "seg_1",
		SourceKind:                threads.SourceKindChannel,
		ConnectorID:               "slack-main",
		ConnectorKind:             "slack",
		SourceAccountID:           "acct_redacted",
		SourceConversationID:      "conv_redacted",
		SourceConversationSummary: "Slack / #support",
		ClaimedShape:              threads.ConversationShapeRoom,
		Now:                       now,
	})); err != nil {
		t.Fatalf("SaveConversationShapeEvidence: %v", err)
	}

	deniedReq := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_mutate/archive", bytes.NewBufferString(`{}`))
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	deniedRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("denied archive status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	archiveReq := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_mutate/archive", bytes.NewBufferString(`{"reasonCode":"operator_archive"}`))
	archiveReq = archiveReq.WithContext(withTenantContext(archiveReq.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	archiveRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", archiveRec.Code, archiveRec.Body.String())
	}
	var archived threadLifecycleActionResponse
	if err := json.Unmarshal(archiveRec.Body.Bytes(), &archived); err != nil {
		t.Fatalf("decode archive: %v", err)
	}
	if archived.LifecycleState != threads.LifecycleStateArchived || archived.AuditEventID == "" {
		t.Fatalf("unexpected archive response: %+v", archived)
	}

	reopenReq := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_mutate/reopen", bytes.NewBufferString(`{}`))
	reopenReq = reopenReq.WithContext(withTenantContext(reopenReq.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	reopenRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, reopenRec, reopenReq)
	if reopenRec.Code != http.StatusOK {
		t.Fatalf("reopen status=%d body=%s", reopenRec.Code, reopenRec.Body.String())
	}

	eventBus := events.NewBus()
	resetReq := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_mutate/reset", bytes.NewBufferString(`{}`))
	resetReq = resetReq.WithContext(withTenantContext(resetReq.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	resetRec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, eventBus, resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset status=%d body=%s", resetRec.Code, resetRec.Body.String())
	}
	detail, found, err := sqliteStore.GetThreadDetailForTenant(context.Background(), "ten_threads", "thr_mutate")
	if err != nil || !found {
		t.Fatalf("detail found=%v err=%v", found, err)
	}
	if detail.Thread.LifecycleState != threads.LifecycleStateReset || len(detail.SessionSegments) != 2 || len(detail.LifecycleActions) != 3 {
		t.Fatalf("mutation detail not persisted: %+v", detail)
	}
	if len(detail.ResetEvents) != 1 || detail.ResetEvents[0].ConversationShape != threads.ConversationShapeRoom || detail.ResetEvents[0].PermissionGate != "connectors.manage" {
		t.Fatalf("reset scope evidence not persisted: %+v", detail.ResetEvents)
	}
	foundScopedResetEvent := false
	for _, event := range eventBus.List(events.Filter{Category: "thread"}) {
		if event.Name == events.ThreadResetScopedName && event.Payload["conversationShape"] == "room" {
			foundScopedResetEvent = true
		}
	}
	if !foundScopedResetEvent {
		t.Fatalf("expected scoped reset event, got %+v", eventBus.List(events.Filter{}))
	}
}

func TestThreadLifecycleDetailShowsRecoveryProjectionAfterRestart(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	sqliteStore, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.UpsertThread(context.Background(), threads.Thread{
		ThreadID:                "thr_partial",
		TenantID:                "ten_threads",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_missing",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack",
		LastActivityAt:          now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
		CreatedAt:               now,
		UpdatedAt:               now,
	}); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	restored, err := store.NewSQLiteStore(dataDir)
	if err != nil {
		t.Fatalf("NewSQLiteStore restored: %v", err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	if stats, err := restored.RecoverThreadLifecycleAfterRestart(context.Background()); err != nil || stats.PartialThreadStates != 1 {
		t.Fatalf("RecoverThreadLifecycleAfterRestart stats=%+v err=%v", stats, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/threads/thr_partial", nil)
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(restored, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	var detail threads.ThreadDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(detail.RuntimeProjections) != 1 || detail.RuntimeProjections[0].ReasonCode != "restore_missing_session_segment" {
		t.Fatalf("expected recovery projection in API detail, got %+v", detail.RuntimeProjections)
	}
}

func threadTenantContext(permissions ...identity.Permission) identity.TenantContext {
	return identity.TenantContext{
		TenantID:    "ten_threads",
		PrincipalID: "prn_threads",
		Permissions: permissions,
	}
}
