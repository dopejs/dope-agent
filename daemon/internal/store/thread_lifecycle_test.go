package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func TestThreadLifecycleMigrationAndTenantSafePersistence(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	for _, table := range []string{"threads", "thread_session_segments", "thread_source_links", "thread_lifecycle_events", "thread_runtime_projections"} {
		var name string
		if err := store.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{
		ThreadID:                "thr_1",
		TenantID:                "ten_1",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_1",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack Main / #support",
		LastActivityAt:          now,
		RetentionExpiresAt:      now.Add(90 * 24 * time.Hour),
		RedactionStatus:         threads.RedactionStatusRedacted,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := store.UpsertThread(ctx, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	got, found, err := store.GetThreadForTenant(ctx, "ten_1", "thr_1")
	if err != nil || !found || got.ThreadID != "thr_1" {
		t.Fatalf("GetThreadForTenant same tenant = %#v %v %v", got, found, err)
	}
	if _, found, err := store.GetThreadForTenant(ctx, "ten_2", "thr_1"); err != nil || found {
		t.Fatalf("GetThreadForTenant cross tenant found=%v err=%v", found, err)
	}
}

func TestThreadLifecycleTenantRetentionPolicyOverride(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	if got := store.ThreadRetentionExpiry(ctx, "ten_default", now); !got.Equal(now.Add(90 * 24 * time.Hour)) {
		t.Fatalf("unexpected default retention: %s", got)
	}
	longer := now.Add(180 * 24 * time.Hour)
	if err := store.SetThreadRetentionPolicy(ctx, "ten_long", longer); err != nil && err != sql.ErrNoRows {
		t.Fatalf("SetThreadRetentionPolicy: %v", err)
	}
	if got := store.ThreadRetentionExpiry(ctx, "ten_long", now); !got.Equal(longer) {
		t.Fatalf("expected longer tenant retention %s, got %s", longer, got)
	}
}

func TestThreadLifecycleListOrderingDetailAndLegacyProjection(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	fixtures := []threads.Thread{
		{ThreadID: "thr_archived_newer", TenantID: "ten_1", LifecycleState: threads.LifecycleStateArchived, CurrentSessionSegmentID: "seg_archived", SourceKind: threads.SourceKindWorkflow, SourceSummary: "workflow", LastActivityAt: now.Add(3 * time.Minute), CreatedAt: now, UpdatedAt: now.Add(3 * time.Minute), RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted},
		{ThreadID: "thr_reopened", TenantID: "ten_1", LifecycleState: threads.LifecycleStateReopened, CurrentSessionSegmentID: "seg_reopened", SourceKind: threads.SourceKindChannel, SourceSummary: "Slack / #ops", LastActivityAt: now.Add(2 * time.Minute), CreatedAt: now, UpdatedAt: now.Add(2 * time.Minute), RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted},
		{ThreadID: "thr_reset", TenantID: "ten_1", LifecycleState: threads.LifecycleStateReset, CurrentSessionSegmentID: "seg_reset", SourceKind: threads.SourceKindChat, SourceSummary: "chat", LastActivityAt: now.Add(time.Minute), CreatedAt: now, UpdatedAt: now.Add(time.Minute), RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted},
	}
	for _, thread := range fixtures {
		if err := store.UpsertThread(ctx, thread); err != nil {
			t.Fatalf("UpsertThread: %v", err)
		}
		if err := store.UpsertThreadSessionSegment(ctx, threads.SessionSegment{
			SessionSegmentID: thread.CurrentSessionSegmentID,
			ThreadID:         thread.ThreadID,
			TenantID:         thread.TenantID,
			SessionID:        "sess_" + thread.ThreadID,
			Generation:       1,
			State:            "active",
			StartedAt:        now,
			LastActiveAt:     thread.LastActivityAt,
		}); err != nil {
			t.Fatalf("UpsertThreadSessionSegment: %v", err)
		}
	}

	list, err := store.ListThreadsForTenant(ctx, ThreadListQuery{TenantID: "ten_1", Limit: 10})
	if err != nil {
		t.Fatalf("ListThreadsForTenant: %v", err)
	}
	gotOrder := []string{}
	for _, item := range list.Items {
		gotOrder = append(gotOrder, item.ThreadID)
	}
	wantOrder := []string{"thr_reopened", "thr_reset", "thr_archived_newer"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("order=%v want=%v", gotOrder, wantOrder)
		}
	}

	detail, found, err := store.GetThreadDetailForTenant(ctx, "ten_1", "thr_reopened")
	if err != nil || !found {
		t.Fatalf("GetThreadDetailForTenant found=%v err=%v", found, err)
	}
	if detail.Thread.CurrentSessionID != "sess_thr_reopened" || len(detail.SessionSegments) != 1 {
		t.Fatalf("unexpected detail projection: %+v", detail)
	}

	if _, err := store.DB().ExecContext(ctx, `INSERT INTO sessions (session_id, kind, status, channel, account_id, peer_id, routing_key, generation, created_at, updated_at, last_active_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "sess_legacy", "chat", "active", "discord", nil, "peer_1", "rk_legacy", 7, formatTime(now), formatTime(now), formatTime(now), "ten_1"); err != nil {
		t.Fatalf("insert legacy session: %v", err)
	}
	if err := store.ProjectLegacySessionsForTenant(ctx, "ten_1"); err != nil {
		t.Fatalf("ProjectLegacySessionsForTenant: %v", err)
	}
	legacy, found, err := store.GetThreadDetailForTenant(ctx, "ten_1", "thr_legacy_sess_legacy")
	if err != nil || !found {
		t.Fatalf("legacy detail found=%v err=%v", found, err)
	}
	if legacy.Thread.SourceKind != threads.SourceKindLegacy || len(legacy.SessionSegments) != 1 || !legacy.SessionSegments[0].PartialEvidence {
		t.Fatalf("legacy projection missing partial evidence: %+v", legacy)
	}
}

func TestThreadLifecycleMutationSerializationAndAuditFailClosed(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()
	now := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	thread := threads.Thread{ThreadID: "thr_mutate", TenantID: "ten_1", LifecycleState: threads.LifecycleStateActive, CurrentSessionSegmentID: "seg_1", SourceKind: threads.SourceKindChannel, SourceSummary: "Slack", LastActivityAt: now, CreatedAt: now, UpdatedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted}
	if err := store.UpsertThread(ctx, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := store.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: "seg_1", ThreadID: "thr_mutate", TenantID: "ten_1", SessionID: "sess_1", Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment: %v", err)
	}
	if err := store.SaveThreadSourceLinkage(ctx, threads.SourceLinkage{SourceLinkageID: "src_mutate_current", ThreadID: "thr_mutate", TenantID: "ten_1", SourceKind: threads.SourceKindChannel, ConnectorID: "slack-main", ConnectorKind: "slack", SourceAccountID: "acct_redacted", SourceConversationID: "conv_redacted", RoutingOutcome: threads.RoutingOutcomeAccepted, Current: true, LinkedAt: now, RetentionExpiresAt: now.Add(90 * 24 * time.Hour), RedactionStatus: threads.RedactionStatusRedacted}); err != nil {
		t.Fatalf("SaveThreadSourceLinkage: %v", err)
	}
	if _, _, err := store.ApplyThreadLifecycleAction(ctx, "ten_1", "thr_mutate", threads.LifecycleActionArchive, threads.LifecycleMutationInput{ActorPrincipalID: "prn_1"}); err == nil {
		t.Fatal("expected mutation without audit evidence to fail closed")
	}
	unchanged, _, err := store.GetThreadDetailForTenant(ctx, "ten_1", "thr_mutate")
	if err != nil {
		t.Fatalf("GetThreadDetailForTenant: %v", err)
	}
	if unchanged.Thread.LifecycleState != threads.LifecycleStateActive || len(unchanged.LifecycleActions) != 0 {
		t.Fatalf("audit failure mutated thread: %+v", unchanged)
	}
	archived, found, err := store.ApplyThreadLifecycleAction(ctx, "ten_1", "thr_mutate", threads.LifecycleActionArchive, threads.LifecycleMutationInput{ActorPrincipalID: "prn_1", AuditEventID: "audit_archive", Now: now.Add(time.Minute)})
	if err != nil || !found {
		t.Fatalf("archive found=%v err=%v", found, err)
	}
	if archived.Thread.LifecycleState != threads.LifecycleStateArchived || archived.Action.AuditEventID != "audit_archive" {
		t.Fatalf("unexpected archive: %+v", archived)
	}
	reopened, found, err := store.ApplyThreadLifecycleAction(ctx, "ten_1", "thr_mutate", threads.LifecycleActionReopen, threads.LifecycleMutationInput{ActorPrincipalID: "prn_1", AuditEventID: "audit_reopen", Now: now.Add(2 * time.Minute)})
	if err != nil || !found || reopened.Thread.LifecycleState != threads.LifecycleStateReopened {
		t.Fatalf("reopen found=%v result=%+v err=%v", found, reopened, err)
	}
	reset, found, err := store.ApplyThreadLifecycleAction(ctx, "ten_1", "thr_mutate", threads.LifecycleActionReset, threads.LifecycleMutationInput{ActorPrincipalID: "prn_1", AuditEventID: "audit_reset", Now: now.Add(3 * time.Minute), NewSegmentID: "seg_2"})
	if err != nil || !found || reset.Thread.CurrentSessionSegmentID != "seg_2" || reset.Segment == nil || reset.Segment.Generation != 2 {
		t.Fatalf("reset found=%v result=%+v err=%v", found, reset, err)
	}
	detail, _, err := store.GetThreadDetailForTenant(ctx, "ten_1", "thr_mutate")
	if err != nil {
		t.Fatalf("GetThreadDetailForTenant after mutations: %v", err)
	}
	if len(detail.LifecycleActions) != 3 || len(detail.SessionSegments) != 2 {
		t.Fatalf("expected persisted actions and segments: %+v", detail)
	}
}

func TestThreadLifecycleRetentionFiltersExpiredEvidenceAndHonorsTenantOverride(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer store.Close()
	now := time.Now().UTC()
	expiredAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	futureExpiry := now.Add(180 * 24 * time.Hour)
	thread := threads.Thread{
		ThreadID:                "thr_retention",
		TenantID:                "ten_retention",
		LifecycleState:          threads.LifecycleStateActive,
		CurrentSessionSegmentID: "seg_retention",
		SourceKind:              threads.SourceKindChannel,
		SourceSummary:           "Slack",
		LastActivityAt:          now,
		CreatedAt:               now,
		UpdatedAt:               now,
		RetentionExpiresAt:      futureExpiry,
		RedactionStatus:         threads.RedactionStatusRedacted,
	}
	if err := store.UpsertThread(ctx, thread); err != nil {
		t.Fatalf("UpsertThread: %v", err)
	}
	if err := store.UpsertThreadSessionSegment(ctx, threads.SessionSegment{SessionSegmentID: "seg_retention", ThreadID: "thr_retention", TenantID: "ten_retention", SessionID: "sess_retention", Generation: 1, State: "active", StartedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("UpsertThreadSessionSegment: %v", err)
	}
	if err := store.SaveThreadSourceLinkage(ctx, threads.SourceLinkage{SourceLinkageID: "src_retention_current", ThreadID: "thr_retention", TenantID: "ten_retention", SourceKind: threads.SourceKindChannel, ConnectorID: "slack-main", ConnectorKind: "slack", SourceAccountID: "acct_redacted", SourceConversationID: "conv_redacted", RoutingOutcome: threads.RoutingOutcomeAccepted, Current: true, LinkedAt: now, RetentionExpiresAt: futureExpiry, RedactionStatus: threads.RedactionStatusRedacted}); err != nil {
		t.Fatalf("SaveThreadSourceLinkage current: %v", err)
	}
	if _, _, err := store.ApplyThreadLifecycleAction(ctx, "ten_retention", "thr_retention", threads.LifecycleActionArchive, threads.LifecycleMutationInput{ActorPrincipalID: "prn_1", AuditEventID: "audit_expired", Now: expiredAt}); err != nil {
		t.Fatalf("archive expired action: %v", err)
	}
	if err := store.SetThreadRetentionPolicy(ctx, "ten_retention", futureExpiry); err != nil {
		t.Fatalf("SetThreadRetentionPolicy: %v", err)
	}
	if _, _, err := store.ApplyThreadLifecycleAction(ctx, "ten_retention", "thr_retention", threads.LifecycleActionReopen, threads.LifecycleMutationInput{ActorPrincipalID: "prn_1", AuditEventID: "audit_retained", Now: now}); err != nil {
		t.Fatalf("reopen retained action: %v", err)
	}
	for _, linkage := range []threads.SourceLinkage{
		{SourceLinkageID: "src_expired", ThreadID: "thr_retention", TenantID: "ten_retention", SourceKind: threads.SourceKindChannel, RoutingOutcome: threads.RoutingOutcomeAccepted, Current: false, LinkedAt: expiredAt, RetentionExpiresAt: expiredAt, RedactionStatus: threads.RedactionStatusRedacted},
		{SourceLinkageID: "src_retained", ThreadID: "thr_retention", TenantID: "ten_retention", SourceKind: threads.SourceKindChannel, RoutingOutcome: threads.RoutingOutcomeDuplicate, Current: false, LinkedAt: now, RetentionExpiresAt: futureExpiry, RedactionStatus: threads.RedactionStatusRedacted},
	} {
		if err := store.SaveThreadSourceLinkage(ctx, linkage); err != nil {
			t.Fatalf("SaveThreadSourceLinkage %s: %v", linkage.SourceLinkageID, err)
		}
	}
	for _, projection := range []threads.RuntimeProjection{
		threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{ProjectionID: "rtp_expired", ThreadID: "thr_retention", TenantID: "ten_retention", ResourceKind: threads.RuntimeResourceRun, ResourceID: "run_expired", Status: "completed", OccurredAt: expiredAt, RetentionExpiresAt: expiredAt, SafeSummary: "expired"}),
		threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{ProjectionID: "rtp_retained", ThreadID: "thr_retention", TenantID: "ten_retention", ResourceKind: threads.RuntimeResourceRun, ResourceID: "run_retained", Status: "completed", OccurredAt: now, RetentionExpiresAt: futureExpiry, SafeSummary: "retained"}),
	} {
		if err := store.SaveThreadRuntimeProjection(ctx, projection); err != nil {
			t.Fatalf("SaveThreadRuntimeProjection %s: %v", projection.RuntimeProjectionID, err)
		}
	}

	detail, found, err := store.GetThreadDetailForTenant(ctx, "ten_retention", "thr_retention")
	if err != nil || !found {
		t.Fatalf("GetThreadDetailForTenant found=%v err=%v", found, err)
	}
	if len(detail.LifecycleActions) != 1 || detail.LifecycleActions[0].AuditEventID != "audit_retained" {
		t.Fatalf("expected only retained lifecycle action, got %+v", detail.LifecycleActions)
	}
	sourceIDs := map[string]bool{}
	for _, linkage := range detail.SourceLinkages {
		sourceIDs[linkage.SourceLinkageID] = true
	}
	if sourceIDs["src_expired"] || !sourceIDs["src_retained"] || !sourceIDs["src_retention_current"] {
		t.Fatalf("expected retained source linkage and current eligibility evidence only, got %+v", detail.SourceLinkages)
	}
	if len(detail.RuntimeProjections) != 1 || detail.RuntimeProjections[0].RuntimeProjectionID != "rtp_retained" {
		t.Fatalf("expected only retained runtime projection, got %+v", detail.RuntimeProjections)
	}
}
