package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadHandoffCreateChannelToWebPersistsSeparateDestinationAndReferences(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	source := threads.Thread{
		ThreadID:                "thr_handoff_source",
		TenantID:                "ten_threads",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_source",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack / #support",
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}
	if err := sqliteStore.UpsertThread(context.Background(), source); err != nil {
		t.Fatalf("UpsertThread source: %v", err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(context.Background(), threads.SessionSegment{SessionSegmentID: "seg_source", ThreadID: source.ThreadID, TenantID: source.TenantID, Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment source: %v", err)
	}
	if _, err := sqliteStore.SaveConversationShapeEvidence(context.Background(), threads.ResolveConversationShape(threads.ConversationShapeResolutionInput{
		TenantID:                  source.TenantID,
		ThreadID:                  source.ThreadID,
		SessionSegmentID:          source.CurrentSessionSegmentID,
		SourceKind:                threads.SourceKindChannel,
		ConnectorID:               "slack-main",
		ConnectorKind:             "slack",
		SourceAccountID:           "workspace_redacted",
		SourceConversationID:      "channel_redacted",
		SourceConversationSummary: source.SourceSummary,
		ClaimedShape:              threads.ConversationShapeRoom,
		Now:                       now,
	})); err != nil {
		t.Fatalf("SaveConversationShapeEvidence source: %v", err)
	}
	if _, err := sqliteStore.SaveContinuityTurn(context.Background(), threads.ContinuityTurn{
		ContinuityTurnID:       "turn_source_1",
		TenantID:               source.TenantID,
		ThreadID:               source.ThreadID,
		SessionSegmentID:       source.CurrentSessionSegmentID,
		AcceptanceSequence:     1,
		Role:                   threads.ContinuityRoleUser,
		SourceKind:             threads.SourceKindChannel,
		SafeContent:            "safe source context",
		ContentRedactionStatus: threads.RedactionStatusRedacted,
		RecordedAt:             now,
		RetentionExpiresAt:     now.Add(90 * 24 * time.Hour),
	}); err != nil {
		t.Fatalf("SaveContinuityTurn: %v", err)
	}
	seedHandoffConnectorAndPolicy(t, sqliteStore, source.TenantID, "slack-main", "channel_redacted")

	eventBus := events.NewBus()
	req := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_handoff_source/handoffs", strings.NewReader(`{"destination":{"surface":"web"},"reasonCode":"user_requested_handoff"}`))
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, eventBus, rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("handoff status=%d body=%s", rec.Code, rec.Body.String())
	}
	link := decodeStrictResponse[threads.HandoffLink](t, rec.Body.Bytes())
	if link.SourceThreadID != source.ThreadID || link.DestinationThreadID == "" || link.DestinationThreadID == source.ThreadID || link.DestinationConversationShape != threads.ConversationShapeWeb {
		t.Fatalf("unexpected handoff link: %+v", link)
	}
	refs, err := sqliteStore.ListHandoffSourceReferencesForLink(context.Background(), source.TenantID, link.HandoffLinkID)
	if err != nil {
		t.Fatalf("ListHandoffSourceReferencesForLink: %v", err)
	}
	if len(refs) != 1 || refs[0].Decision != threads.HandoffReferenceDecisionReferenced || refs[0].ContinuityTurnID != "turn_source_1" {
		t.Fatalf("handoff source references = %+v", refs)
	}
	detail, found, err := sqliteStore.GetThreadDetailForTenant(context.Background(), source.TenantID, link.DestinationThreadID)
	if err != nil || !found {
		t.Fatalf("destination detail found=%v err=%v", found, err)
	}
	if detail.ConversationShape == nil || detail.ConversationShape.Shape != threads.ConversationShapeWeb || len(detail.HandoffLinks) != 1 {
		t.Fatalf("destination detail missing web shape/handoff evidence: %+v", detail)
	}
	foundEvent := false
	for _, event := range eventBus.List(events.Filter{Category: "thread"}) {
		if event.Name == events.ThreadHandoffLinkedName && event.Payload["sourceThreadId"] == source.ThreadID {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Fatalf("expected handoff linked event, got %+v", eventBus.List(events.Filter{}))
	}
}

func TestThreadHandoffCreateWebToChannelRequiresEligibleDestination(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	source := threads.Thread{
		ThreadID:                "thr_web_source",
		TenantID:                "ten_threads",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_web_source",
		SourceKind:              threads.SourceKindShell,
		SourceSummary:           "Web source",
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}
	seedHandoffThread(t, sqliteStore, source, threads.ConversationShapeWeb, "")
	seedHandoffConnectorAndPolicy(t, sqliteStore, source.TenantID, "slack-main", "channel_redacted")

	req := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_web_source/handoffs", strings.NewReader(`{"destination":{"surface":"channel","connectorId":"slack-main","sourceAccountId":"workspace_redacted","sourceConversationId":"channel_redacted","conversationShape":"room"},"reasonCode":"user_requested_handoff"}`))
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("handoff status=%d body=%s", rec.Code, rec.Body.String())
	}
	link := decodeStrictResponse[threads.HandoffLink](t, rec.Body.Bytes())
	if link.SourceThreadID != source.ThreadID || link.DestinationThreadID == source.ThreadID || link.DestinationConversationShape != threads.ConversationShapeRoom {
		t.Fatalf("unexpected web-to-channel handoff link: %+v", link)
	}
}

func TestThreadHandoffDeniedDestinationDoesNotCreateThread(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	source := threads.Thread{
		ThreadID:                "thr_web_source_denied",
		TenantID:                "ten_threads",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_web_source_denied",
		SourceKind:              threads.SourceKindShell,
		SourceSummary:           "Web source",
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
	}
	seedHandoffThread(t, sqliteStore, source, threads.ConversationShapeWeb, "")
	before := listThreadCount(t, sqliteStore, source.TenantID)

	req := httptest.NewRequest(http.MethodPost, "/v1/threads/thr_web_source_denied/handoffs", strings.NewReader(`{"destination":{"surface":"channel","connectorId":"slack-main","sourceAccountId":"workspace_redacted","sourceConversationId":"channel_denied","conversationShape":"room"},"reasonCode":"user_requested_handoff"}`))
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionConnectorsManage)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, rec, req)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusConflict {
		t.Fatalf("expected denied handoff, got %d body=%s", rec.Code, rec.Body.String())
	}
	after := listThreadCount(t, sqliteStore, source.TenantID)
	if after != before {
		t.Fatalf("denied handoff created destination thread: before=%d after=%d", before, after)
	}
}

func TestThreadHandoffCreateRequiresManagePermission(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	req := httptest.NewRequest(http.MethodPost, "/v1/threads/missing/handoffs", strings.NewReader(`{"destination":{"surface":"web"}}`))
	req = req.WithContext(withTenantContext(req.Context(), threadTenantContext(identity.PermissionCredentialsInspect)))
	rec := httptest.NewRecorder()
	handleThreadLifecycleRoutes(sqliteStore, nil, rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected permission denial, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func seedHandoffThread(t *testing.T, sqliteStore *store.SQLiteStore, thread threads.Thread, shape threads.ConversationShape, connectorID string) {
	t.Helper()
	if err := sqliteStore.UpsertThread(context.Background(), thread); err != nil {
		t.Fatalf("UpsertThread %s: %v", thread.ThreadID, err)
	}
	if err := sqliteStore.UpsertThreadSessionSegment(context.Background(), threads.SessionSegment{SessionSegmentID: thread.CurrentSessionSegmentID, ThreadID: thread.ThreadID, TenantID: thread.TenantID, Generation: 1, State: "active", StartedAt: thread.CreatedAt, LastActiveAt: thread.UpdatedAt}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment %s: %v", thread.ThreadID, err)
	}
	if _, err := sqliteStore.SaveConversationShapeEvidence(context.Background(), threads.ResolveConversationShape(threads.ConversationShapeResolutionInput{
		TenantID:                  thread.TenantID,
		ThreadID:                  thread.ThreadID,
		SessionSegmentID:          thread.CurrentSessionSegmentID,
		SourceKind:                thread.SourceKind,
		ConnectorID:               connectorID,
		ConnectorKind:             "slack",
		SourceAccountID:           "workspace_redacted",
		SourceConversationID:      "channel_redacted",
		SourceConversationSummary: thread.SourceSummary,
		ClaimedShape:              shape,
		Now:                       thread.CreatedAt,
	})); err != nil {
		t.Fatalf("SaveConversationShapeEvidence %s: %v", thread.ThreadID, err)
	}
}

func seedHandoffConnectorAndPolicy(t *testing.T, sqliteStore *store.SQLiteStore, tenantID, connectorID, conversationID string) {
	t.Helper()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.UpsertConnector(context.Background(), connectors.Connector{
		TenantID:          tenantID,
		ConnectorID:       connectorID,
		Kind:              "slack",
		DisplayName:       "Slack Main",
		Status:            connectors.StatusHealthy,
		CapabilityProfile: map[string]any{connectors.HandoffSurfaceDestinationSupport: string(connectors.SurfaceSupported)},
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("UpsertConnector: %v", err)
	}
	if err := sqliteStore.SaveChannelRoutePolicy(context.Background(), connectors.RoutePolicy{
		RoutePolicyID:         "route_policy_" + connectorID,
		TenantID:              tenantID,
		ConnectorID:           connectorID,
		EligibleRooms:         []string{conversationID},
		EligibleChannels:      []string{conversationID},
		EligibleConversations: []string{conversationID},
		ValidationState:       "valid",
		ValidatedAt:           now,
		RedactionStatus:       connectors.RedactionStatusRedacted,
	}); err != nil {
		t.Fatalf("SaveChannelRoutePolicy: %v", err)
	}
}

func listThreadCount(t *testing.T, sqliteStore *store.SQLiteStore, tenantID string) int {
	t.Helper()
	list, err := sqliteStore.ListThreadsForTenant(context.Background(), store.ThreadListQuery{TenantID: tenantID, Limit: 100})
	if err != nil {
		t.Fatalf("ListThreadsForTenant: %v", err)
	}
	return len(list.Items)
}
