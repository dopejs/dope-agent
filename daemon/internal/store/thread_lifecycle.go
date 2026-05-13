package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type ThreadLifecycleStore struct {
	store *SQLiteStore
}

type ThreadListQuery struct {
	TenantID     string
	Limit        int
	Cursor       string
	StateFilter  string
	SourceFilter string
}

type ThreadLifecycleMutationResult struct {
	Thread  threads.Thread
	Action  threads.LifecycleAction
	Segment *threads.SessionSegment
}

type ThreadLifecycleRecoveryStats struct {
	Tenants                  []string
	CheckedThreads           int
	ProjectedLegacySessions  int
	PartialThreadStates      int
	RecoveryProjectionWrites int
}

type ThreadProjectionAnchor struct {
	ThreadID         string
	TenantID         string
	SessionSegmentID string
}

func NewThreadLifecycleStore(store *SQLiteStore) ThreadLifecycleStore {
	return ThreadLifecycleStore{store: store}
}

func (s ThreadLifecycleStore) Ping(ctx context.Context) error {
	_ = ctx
	_ = s.store
	return nil
}

func (s *SQLiteStore) UpsertThread(ctx context.Context, thread threads.Thread) error {
	if s == nil {
		return nil
	}
	return s.upsertThread(ctx, s.db, thread)
}

func (s *SQLiteStore) upsertThread(ctx context.Context, execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, thread threads.Thread) error {
	now := time.Now().UTC()
	if thread.CreatedAt.IsZero() {
		thread.CreatedAt = now
	}
	if thread.UpdatedAt.IsZero() {
		thread.UpdatedAt = now
	}
	if thread.LastActivityAt.IsZero() {
		thread.LastActivityAt = thread.UpdatedAt
	}
	if thread.RedactionStatus == "" {
		thread.RedactionStatus = threads.RedactionStatusRedacted
	}
	document, err := json.Marshal(thread)
	if err != nil {
		return fmt.Errorf("marshal thread %s: %w", thread.ThreadID, err)
	}
	_, err = execer.ExecContext(ctx, `
		INSERT INTO threads (
			thread_id, tenant_id, lifecycle_state, current_session_segment_id, source_kind,
			source_summary, last_activity_at, created_at, updated_at, retention_expires_at,
			redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(thread_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			lifecycle_state = excluded.lifecycle_state,
			current_session_segment_id = excluded.current_session_segment_id,
			source_kind = excluded.source_kind,
			source_summary = excluded.source_summary,
			last_activity_at = excluded.last_activity_at,
			updated_at = excluded.updated_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, thread.ThreadID, thread.TenantID, thread.LifecycleState, thread.CurrentSessionSegmentID, thread.SourceKind, thread.SourceSummary, formatTime(thread.LastActivityAt), formatTime(thread.CreatedAt), formatTime(thread.UpdatedAt), formatTime(thread.RetentionExpiresAt), thread.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("upsert thread %s: %w", thread.ThreadID, err)
	}
	return nil
}

func (s *SQLiteStore) upsertThreadTx(ctx context.Context, tx *sql.Tx, thread threads.Thread) error {
	return s.upsertThread(ctx, tx, thread)
}

func (s *SQLiteStore) updateThreadLifecycleTx(ctx context.Context, tx *sql.Tx, prior threads.Thread, updated threads.Thread) error {
	document, err := json.Marshal(updated)
	if err != nil {
		return fmt.Errorf("marshal thread %s: %w", updated.ThreadID, err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE threads
		SET lifecycle_state = ?,
			current_session_segment_id = ?,
			source_kind = ?,
			source_summary = ?,
			last_activity_at = ?,
			updated_at = ?,
			retention_expires_at = ?,
			redaction_status = ?,
			document_json = ?
		WHERE tenant_id = ?
		  AND thread_id = ?
		  AND lifecycle_state = ?
		  AND current_session_segment_id = ?
		  AND updated_at = ?
	`, updated.LifecycleState, updated.CurrentSessionSegmentID, updated.SourceKind, updated.SourceSummary, formatTime(updated.LastActivityAt), formatTime(updated.UpdatedAt), formatTime(updated.RetentionExpiresAt), updated.RedactionStatus, string(document), prior.TenantID, prior.ThreadID, prior.LifecycleState, prior.CurrentSessionSegmentID, formatTime(prior.UpdatedAt))
	if err != nil {
		return fmt.Errorf("update thread lifecycle %s: %w", updated.ThreadID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect thread lifecycle update %s: %w", updated.ThreadID, err)
	}
	if affected != 1 {
		return threads.ErrLifecycleMutationConflict
	}
	return nil
}

func (s *SQLiteStore) UpsertThreadSessionSegment(ctx context.Context, segment threads.SessionSegment) error {
	if s == nil {
		return nil
	}
	return s.upsertThreadSessionSegment(ctx, s.db, segment)
}

func (s *SQLiteStore) upsertThreadSessionSegment(ctx context.Context, execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, segment threads.SessionSegment) error {
	now := time.Now().UTC()
	if segment.StartedAt.IsZero() {
		segment.StartedAt = now
	}
	if segment.LastActiveAt.IsZero() {
		segment.LastActiveAt = segment.StartedAt
	}
	if segment.State == "" {
		segment.State = string(threads.LifecycleStateActive)
	}
	document, err := json.Marshal(segment)
	if err != nil {
		return fmt.Errorf("marshal thread session segment %s: %w", segment.SessionSegmentID, err)
	}
	var endedAt any
	if segment.EndedAt != nil && !segment.EndedAt.IsZero() {
		endedAt = formatTime(*segment.EndedAt)
	}
	_, err = execer.ExecContext(ctx, `
		INSERT INTO thread_session_segments (
			session_segment_id, thread_id, tenant_id, session_id, generation, state,
			started_at, ended_at, last_active_at, reset_from_session_segment_id,
			partial_evidence, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_segment_id) DO UPDATE SET
			thread_id = excluded.thread_id,
			tenant_id = excluded.tenant_id,
			session_id = excluded.session_id,
			generation = excluded.generation,
			state = excluded.state,
			started_at = excluded.started_at,
			ended_at = excluded.ended_at,
			last_active_at = excluded.last_active_at,
			reset_from_session_segment_id = excluded.reset_from_session_segment_id,
			partial_evidence = excluded.partial_evidence,
			document_json = excluded.document_json
	`, segment.SessionSegmentID, segment.ThreadID, segment.TenantID, segment.SessionID, segment.Generation, segment.State, formatTime(segment.StartedAt), endedAt, formatTime(segment.LastActiveAt), segment.ResetFromSessionSegment, boolToInt(segment.PartialEvidence), string(document))
	if err != nil {
		return fmt.Errorf("upsert thread session segment %s: %w", segment.SessionSegmentID, err)
	}
	return nil
}

func (s *SQLiteStore) upsertThreadSessionSegmentTx(ctx context.Context, tx *sql.Tx, segment threads.SessionSegment) error {
	return s.upsertThreadSessionSegment(ctx, tx, segment)
}

func (s *SQLiteStore) GetThreadForTenant(ctx context.Context, tenantID, threadID string) (threads.Thread, bool, error) {
	if s == nil {
		return threads.Thread{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM threads
		WHERE tenant_id = ? AND thread_id = ?
	`, tenantID, threadID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.Thread{}, false, nil
		}
		return threads.Thread{}, false, fmt.Errorf("get thread %s/%s: %w", tenantID, threadID, err)
	}
	var thread threads.Thread
	if err := json.Unmarshal([]byte(raw), &thread); err != nil {
		return threads.Thread{}, false, fmt.Errorf("decode thread %s/%s: %w", tenantID, threadID, err)
	}
	return thread, true, nil
}

func (s *SQLiteStore) GetCurrentThreadForSource(ctx context.Context, key threads.SourceContinuationKey) (threads.Thread, bool, error) {
	if s == nil {
		return threads.Thread{}, false, nil
	}
	normalized, err := threads.NormalizeSourceContinuationKey(key)
	if err != nil {
		return threads.Thread{}, false, err
	}
	var raw string
	err = s.db.QueryRowContext(ctx, `
		SELECT t.document_json
		FROM thread_source_links AS l
		JOIN threads AS t ON t.thread_id = l.thread_id AND t.tenant_id = l.tenant_id
		WHERE l.tenant_id = ? AND l.connector_id = ? AND l.source_account_id = ? AND l.source_conversation_id = ? AND l.current_flag = 1
	`, normalized.TenantID, normalized.ConnectorID, normalized.SourceAccountID, normalized.SourceConversationID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.Thread{}, false, nil
		}
		return threads.Thread{}, false, fmt.Errorf("get current thread for source %s: %w", normalized.String(), err)
	}
	var thread threads.Thread
	if err := json.Unmarshal([]byte(raw), &thread); err != nil {
		return threads.Thread{}, false, fmt.Errorf("decode current thread for source %s: %w", normalized.String(), err)
	}
	return thread, true, nil
}

func (s *SQLiteStore) GetThreadForSession(ctx context.Context, sessionID string) (threads.Thread, threads.SessionSegment, bool, error) {
	if s == nil || sessionID == "" {
		return threads.Thread{}, threads.SessionSegment{}, false, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.document_json, seg.document_json
		FROM thread_session_segments AS seg
		JOIN threads AS t ON t.thread_id = seg.thread_id AND t.tenant_id = seg.tenant_id
		WHERE seg.session_id = ?
		ORDER BY seg.generation DESC, seg.session_segment_id DESC
		LIMIT 1
	`, sessionID)
	if err != nil {
		return threads.Thread{}, threads.SessionSegment{}, false, fmt.Errorf("get thread for session %s: %w", sessionID, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return threads.Thread{}, threads.SessionSegment{}, false, rows.Err()
	}
	var rawThread, rawSegment string
	if err := rows.Scan(&rawThread, &rawSegment); err != nil {
		return threads.Thread{}, threads.SessionSegment{}, false, fmt.Errorf("scan thread for session %s: %w", sessionID, err)
	}
	var thread threads.Thread
	if err := json.Unmarshal([]byte(rawThread), &thread); err != nil {
		return threads.Thread{}, threads.SessionSegment{}, false, fmt.Errorf("decode thread for session %s: %w", sessionID, err)
	}
	var segment threads.SessionSegment
	if err := json.Unmarshal([]byte(rawSegment), &segment); err != nil {
		return threads.Thread{}, threads.SessionSegment{}, false, fmt.Errorf("decode segment for session %s: %w", sessionID, err)
	}
	return thread, segment, true, rows.Err()
}

func (s *SQLiteStore) ThreadProjectionAnchorForRun(ctx context.Context, runID string) (ThreadProjectionAnchor, bool, error) {
	if s == nil || runID == "" {
		return ThreadProjectionAnchor{}, false, nil
	}
	var projection threads.RuntimeProjection
	found, err := s.runtimeProjectionAnchor(ctx, threads.RuntimeResourceRun, runID, &projection)
	if err != nil || found {
		return projectionAnchorFromProjection(projection), found, err
	}
	var rawThread, rawSegment string
	err = s.db.QueryRowContext(ctx, `
		SELECT t.document_json, seg.document_json
		FROM runs AS r
		JOIN thread_session_segments AS seg ON seg.session_id = r.session_id
		JOIN threads AS t ON t.thread_id = seg.thread_id AND t.tenant_id = seg.tenant_id
		WHERE r.run_id = ?
		ORDER BY seg.generation DESC, seg.session_segment_id DESC
		LIMIT 1
	`, runID).Scan(&rawThread, &rawSegment)
	if err != nil {
		if err == sql.ErrNoRows {
			return ThreadProjectionAnchor{}, false, nil
		}
		return ThreadProjectionAnchor{}, false, fmt.Errorf("find thread projection anchor for run %s: %w", runID, err)
	}
	var thread threads.Thread
	if err := json.Unmarshal([]byte(rawThread), &thread); err != nil {
		return ThreadProjectionAnchor{}, false, fmt.Errorf("decode thread projection anchor for run %s: %w", runID, err)
	}
	var segment threads.SessionSegment
	if err := json.Unmarshal([]byte(rawSegment), &segment); err != nil {
		return ThreadProjectionAnchor{}, false, fmt.Errorf("decode segment projection anchor for run %s: %w", runID, err)
	}
	return ThreadProjectionAnchor{ThreadID: thread.ThreadID, TenantID: thread.TenantID, SessionSegmentID: segment.SessionSegmentID}, true, nil
}

func (s *SQLiteStore) ThreadProjectionAnchorForWorkflow(ctx context.Context, workflowID string) (ThreadProjectionAnchor, bool, error) {
	if s == nil || workflowID == "" {
		return ThreadProjectionAnchor{}, false, nil
	}
	var projection threads.RuntimeProjection
	found, err := s.runtimeProjectionAnchor(ctx, threads.RuntimeResourceWorkflow, workflowID, &projection)
	return projectionAnchorFromProjection(projection), found, err
}

func (s *SQLiteStore) runtimeProjectionAnchor(ctx context.Context, kind threads.RuntimeResourceKind, resourceID string, projection *threads.RuntimeProjection) (bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM thread_runtime_projections
		WHERE resource_kind = ? AND resource_id = ?
		ORDER BY occurred_at DESC, runtime_projection_id DESC
		LIMIT 1
	`, kind, resourceID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("find thread projection anchor %s/%s: %w", kind, resourceID, err)
	}
	if err := json.Unmarshal([]byte(raw), projection); err != nil {
		return false, fmt.Errorf("decode thread projection anchor %s/%s: %w", kind, resourceID, err)
	}
	return true, nil
}

func projectionAnchorFromProjection(projection threads.RuntimeProjection) ThreadProjectionAnchor {
	return ThreadProjectionAnchor{
		ThreadID:         projection.ThreadID,
		TenantID:         projection.TenantID,
		SessionSegmentID: projection.SessionSegmentID,
	}
}

func (s *SQLiteStore) SaveThreadRuntimeProjectionForRun(ctx context.Context, runID string, input threads.RuntimeProjectionInput) (threads.RuntimeProjection, bool, error) {
	anchor, found, err := s.ThreadProjectionAnchorForRun(ctx, runID)
	if err != nil || !found {
		return threads.RuntimeProjection{}, found, err
	}
	if input.ThreadID == "" {
		input.ThreadID = anchor.ThreadID
	}
	if input.TenantID == "" {
		input.TenantID = anchor.TenantID
	}
	if input.SessionSegmentID == "" {
		input.SessionSegmentID = anchor.SessionSegmentID
	}
	projection := threads.BuildRuntimeProjection(input)
	if err := s.SaveThreadRuntimeProjection(ctx, projection); err != nil {
		return threads.RuntimeProjection{}, true, err
	}
	return projection, true, nil
}

func (s *SQLiteStore) SaveThreadRuntimeProjectionForWorkflow(ctx context.Context, workflowID string, input threads.RuntimeProjectionInput) (threads.RuntimeProjection, bool, error) {
	anchor, found, err := s.ThreadProjectionAnchorForWorkflow(ctx, workflowID)
	if err != nil || !found {
		return threads.RuntimeProjection{}, found, err
	}
	if input.ThreadID == "" {
		input.ThreadID = anchor.ThreadID
	}
	if input.TenantID == "" {
		input.TenantID = anchor.TenantID
	}
	if input.SessionSegmentID == "" {
		input.SessionSegmentID = anchor.SessionSegmentID
	}
	projection := threads.BuildRuntimeProjection(input)
	if err := s.SaveThreadRuntimeProjection(ctx, projection); err != nil {
		return threads.RuntimeProjection{}, true, err
	}
	return projection, true, nil
}

func (s *SQLiteStore) SaveThreadSourceLinkage(ctx context.Context, linkage threads.SourceLinkage) error {
	if s == nil {
		return nil
	}
	if linkage.LinkedAt.IsZero() {
		linkage.LinkedAt = time.Now().UTC()
	}
	if linkage.RedactionStatus == "" {
		linkage.RedactionStatus = threads.RedactionStatusRedacted
	}
	if linkage.RetentionExpiresAt.IsZero() {
		linkage.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, linkage.TenantID, linkage.LinkedAt)
	}
	document, err := json.Marshal(linkage)
	if err != nil {
		return fmt.Errorf("marshal thread source linkage %s: %w", linkage.SourceLinkageID, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin thread source linkage: %w", err)
	}
	if linkage.Current {
		if _, err := tx.ExecContext(ctx, `
			UPDATE thread_source_links
			SET current_flag = 0
			WHERE tenant_id = ? AND connector_id = ? AND source_account_id = ? AND source_conversation_id = ? AND current_flag = 1
		`, linkage.TenantID, linkage.ConnectorID, linkage.SourceAccountID, linkage.SourceConversationID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("clear current source linkage: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO thread_source_links (
			source_linkage_id, thread_id, tenant_id, source_kind, connector_id,
			connector_kind, source_account_id, source_conversation_id, source_message_id,
			routing_outcome, current_flag, linked_at, retention_expires_at,
			redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_linkage_id) DO UPDATE SET
			thread_id = excluded.thread_id,
			tenant_id = excluded.tenant_id,
			source_kind = excluded.source_kind,
			connector_id = excluded.connector_id,
			connector_kind = excluded.connector_kind,
			source_account_id = excluded.source_account_id,
			source_conversation_id = excluded.source_conversation_id,
			source_message_id = excluded.source_message_id,
			routing_outcome = excluded.routing_outcome,
			current_flag = excluded.current_flag,
			linked_at = excluded.linked_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, linkage.SourceLinkageID, linkage.ThreadID, linkage.TenantID, linkage.SourceKind, nullString(linkage.ConnectorID), nullString(linkage.ConnectorKind), nullString(linkage.SourceAccountID), nullString(linkage.SourceConversationID), nullString(linkage.SourceMessageID), linkage.RoutingOutcome, boolToInt(linkage.Current), formatTime(linkage.LinkedAt), formatTime(linkage.RetentionExpiresAt), linkage.RedactionStatus, string(document))
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save thread source linkage %s: %w", linkage.SourceLinkageID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit thread source linkage: %w", err)
	}
	if linkage.RedactionStatus == threads.RedactionStatusRedactionFailed {
		if err := s.appendThreadLifecycleAuditEvent(ctx, events.ThreadRedactionFailedEvent(linkage.TenantID, linkage.ThreadID, "source_linkage_redaction_failed"), "redaction_source_"+linkage.SourceLinkageID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) SaveThreadRuntimeProjection(ctx context.Context, projection threads.RuntimeProjection) error {
	if s == nil {
		return nil
	}
	if projection.OccurredAt.IsZero() {
		projection.OccurredAt = time.Now().UTC()
	}
	if projection.RedactionStatus == "" {
		projection.RedactionStatus = threads.RedactionStatusRedacted
	}
	if projection.RetentionExpiresAt.IsZero() {
		projection.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, projection.TenantID, projection.OccurredAt)
	}
	document, err := json.Marshal(projection)
	if err != nil {
		return fmt.Errorf("marshal thread runtime projection %s: %w", projection.RuntimeProjectionID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO thread_runtime_projections (
			runtime_projection_id, thread_id, tenant_id, session_segment_id,
			resource_kind, resource_id, status, reason_code, occurred_at,
			route, safe_summary, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(runtime_projection_id) DO UPDATE SET
			thread_id = excluded.thread_id,
			tenant_id = excluded.tenant_id,
			session_segment_id = excluded.session_segment_id,
			resource_kind = excluded.resource_kind,
			resource_id = excluded.resource_id,
			status = excluded.status,
			reason_code = excluded.reason_code,
			occurred_at = excluded.occurred_at,
			route = excluded.route,
			safe_summary = excluded.safe_summary,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, projection.RuntimeProjectionID, projection.ThreadID, projection.TenantID, nullString(projection.SessionSegmentID), projection.ResourceKind, projection.ResourceID, projection.Status, nullString(projection.ReasonCode), formatTime(projection.OccurredAt), nullString(projection.Route), nullString(projection.SafeSummary), formatTime(projection.RetentionExpiresAt), projection.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("save thread runtime projection %s: %w", projection.RuntimeProjectionID, err)
	}
	if projection.RedactionStatus == threads.RedactionStatusRedactionFailed {
		if err := s.appendThreadLifecycleAuditEvent(ctx, events.ThreadRedactionFailedEvent(projection.TenantID, projection.ThreadID, "runtime_projection_redaction_failed"), "redaction_runtime_"+projection.RuntimeProjectionID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) appendThreadLifecycleAuditEvent(ctx context.Context, event events.Event, idSuffix string) error {
	if s == nil {
		return nil
	}
	if event.EventID == "" {
		event.EventID = "evt_thread_" + idSuffix
	}
	if event.TenantID != "" {
		_, err := s.AppendEventForTenantRaw(ctx, event, event.TenantID)
		return err
	}
	_, err := s.AppendEvent(ctx, event)
	return err
}

func EmptyThreadListResponse(tenantID string) threads.ThreadListResponse {
	return threads.ThreadListResponse{
		TenantID: tenantID,
		Page: threads.ThreadPage{
			Limit: 20,
			Order: "active_recent_archived_id",
		},
		Items: []threads.ThreadResource{},
	}
}

func (s *SQLiteStore) ListThreadsForTenant(ctx context.Context, query ThreadListQuery) (threads.ThreadListResponse, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := 0
	if query.Cursor != "" {
		parsed, err := strconv.Atoi(query.Cursor)
		if err == nil && parsed > 0 {
			offset = parsed
		}
	}
	args := []any{query.TenantID}
	where := "tenant_id = ?"
	if query.StateFilter != "" {
		where += " AND lifecycle_state = ?"
		args = append(args, query.StateFilter)
	}
	if query.SourceFilter != "" {
		where += " AND source_kind = ?"
		args = append(args, query.SourceFilter)
	}
	args = append(args, limit+1, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM threads
		WHERE `+where+`
		ORDER BY CASE lifecycle_state WHEN 'archived' THEN 1 ELSE 0 END ASC,
			last_activity_at DESC,
			thread_id ASC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return threads.ThreadListResponse{}, fmt.Errorf("list threads for tenant %s: %w", query.TenantID, err)
	}
	defer rows.Close()
	items := []threads.ThreadResource{}
	count := 0
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return threads.ThreadListResponse{}, fmt.Errorf("scan thread: %w", err)
		}
		count++
		if count > limit {
			continue
		}
		var thread threads.Thread
		if err := json.Unmarshal([]byte(raw), &thread); err != nil {
			return threads.ThreadListResponse{}, fmt.Errorf("decode thread: %w", err)
		}
		items = append(items, threads.BuildThreadResource(thread, ""))
	}
	if err := rows.Err(); err != nil {
		return threads.ThreadListResponse{}, err
	}
	nextCursor := ""
	if count > limit {
		nextCursor = strconv.Itoa(offset + limit)
	}
	return threads.ThreadListResponse{
		TenantID: query.TenantID,
		Page: threads.ThreadPage{
			Limit:      limit,
			NextCursor: nextCursor,
			Order:      "active_recent_archived_id",
		},
		Items: items,
	}, nil
}

func (s *SQLiteStore) GetThreadDetailForTenant(ctx context.Context, tenantID, threadID string) (threads.ThreadDetailResponse, bool, error) {
	thread, found, err := s.GetThreadForTenant(ctx, tenantID, threadID)
	if err != nil || !found {
		return threads.ThreadDetailResponse{}, found, err
	}
	segments, err := s.threadSegments(ctx, tenantID, threadID)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	actions, err := s.threadLifecycleActions(ctx, tenantID, threadID)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	sourceLinkages, err := s.threadSourceLinkages(ctx, tenantID, threadID)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	runtimeProjections, err := s.threadRuntimeProjections(ctx, tenantID, threadID)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	activeProfileProjections, err := s.ListRuntimeProfileProjections(ctx, tenantID, "", "", threadID, 1)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	continuityPreviews, err := s.ListContinuityPreviewSummaries(ctx, tenantID, threadID, 10)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	shape, hasShape, err := s.GetConversationShapeForThread(ctx, tenantID, threadID)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	participationDecisions, err := s.ListParticipationDecisionsForThread(ctx, tenantID, threadID, 20)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	resetEvents, err := s.ListResetEventsForThread(ctx, tenantID, threadID, 20)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	handoffLinks, err := s.ListHandoffLinksForThread(ctx, tenantID, threadID, 20)
	if err != nil {
		return threads.ThreadDetailResponse{}, false, err
	}
	currentSessionID := ""
	for _, segment := range segments {
		if segment.SessionSegmentID == thread.CurrentSessionSegmentID {
			currentSessionID = segment.SessionID
			break
		}
	}
	response := threads.ThreadDetailResponse{
		Thread:                 threads.BuildThreadResource(thread, currentSessionID),
		SessionSegments:        segments,
		SourceLinkages:         sourceLinkages,
		RuntimeProjections:     runtimeProjections,
		LifecycleActions:       actions,
		ContinuityPreviews:     continuityPreviews,
		ParticipationDecisions: participationDecisions,
		ResetEvents:            resetEvents,
		HandoffLinks:           handoffLinks,
	}
	if hasShape {
		response.ConversationShape = &shape
	}
	if len(activeProfileProjections) > 0 {
		response.ActiveProfileProjection = &activeProfileProjections[0]
	}
	return response, true, nil
}

func (s *SQLiteStore) ApplyThreadLifecycleAction(ctx context.Context, tenantID, threadID string, kind threads.LifecycleActionKind, input threads.LifecycleMutationInput) (ThreadLifecycleMutationResult, bool, error) {
	if s == nil {
		return ThreadLifecycleMutationResult{}, false, nil
	}
	if input.AuditEventID == "" {
		return ThreadLifecycleMutationResult{}, false, threads.ErrAuditEvidenceRequired
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	retentionExpiresAt := s.ThreadRetentionExpiry(ctx, tenantID, input.Now)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ThreadLifecycleMutationResult{}, true, fmt.Errorf("begin thread lifecycle action: %w", err)
	}
	thread, found, err := getThreadForTenantTx(ctx, tx, tenantID, threadID)
	if err != nil || !found {
		_ = tx.Rollback()
		return ThreadLifecycleMutationResult{}, found, err
	}
	var updated threads.Thread
	var action threads.LifecycleAction
	var segment *threads.SessionSegment
	switch kind {
	case threads.LifecycleActionReset:
		if input.NewSegmentID == "" {
			input.NewSegmentID = fmt.Sprintf("seg_%s_%d", threadID, input.Now.UnixNano())
		}
		next, lifecycleAction, newSegment, err := threads.ResetThread(thread, input)
		if err != nil {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, err
		}
		newSegment.Generation = nextThreadSegmentGenerationTx(ctx, tx, tenantID, threadID)
		updated, action, segment = next, lifecycleAction, &newSegment
	case threads.LifecycleActionArchive:
		next, lifecycleAction, err := threads.ArchiveThread(thread, input)
		if err != nil {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, err
		}
		updated, action = next, lifecycleAction
	case threads.LifecycleActionReopen:
		eligible, err := threadReopenEligibleTx(ctx, tx, thread)
		if err != nil {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, err
		}
		if !eligible {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, threads.ErrLifecycleReopenNotEligible
		}
		next, lifecycleAction, err := threads.ReopenThread(thread, input)
		if err != nil {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, err
		}
		updated, action = next, lifecycleAction
	default:
		_ = tx.Rollback()
		return ThreadLifecycleMutationResult{}, true, threads.ErrLifecycleTransitionNotAllowed
	}
	action.LifecycleActionID = input.AuditEventID
	action.RetentionExpiresAt = retentionExpiresAt
	if err := s.updateThreadLifecycleTx(ctx, tx, thread, updated); err != nil {
		_ = tx.Rollback()
		return ThreadLifecycleMutationResult{}, true, err
	}
	if segment != nil {
		if err := s.upsertThreadSessionSegmentTx(ctx, tx, *segment); err != nil {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, err
		}
	}
	if err := s.insertThreadLifecycleActionTx(ctx, tx, action); err != nil {
		_ = tx.Rollback()
		return ThreadLifecycleMutationResult{}, true, err
	}
	if kind == threads.LifecycleActionReset {
		shape, _, err := getConversationShapeForThreadTx(ctx, tx, tenantID, threadID)
		if err != nil {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, err
		}
		resetEvent := threads.BuildScopedResetEvent(action, shape)
		resetEvent.ResetEventID = "reset_" + action.AuditEventID
		resetEvent.RetentionExpiresAt = retentionExpiresAt
		if err := insertThreadResetEventTx(ctx, tx, resetEvent); err != nil {
			_ = tx.Rollback()
			return ThreadLifecycleMutationResult{}, true, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ThreadLifecycleMutationResult{}, true, fmt.Errorf("commit thread lifecycle action: %w", err)
	}
	return ThreadLifecycleMutationResult{Thread: updated, Action: action, Segment: segment}, true, nil
}

func (s *SQLiteStore) ProjectLegacySessionsForTenant(ctx context.Context, tenantID string) error {
	if s == nil {
		return nil
	}
	type legacySessionRow struct {
		sessionID      string
		kind           string
		status         string
		channel        string
		accountID      sql.NullString
		peerID         string
		legacyThreadID sql.NullString
		generation     int
		createdAt      string
		updatedAt      string
		lastActiveAt   string
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, kind, status, channel, account_id, peer_id, thread_id, generation, created_at, updated_at, last_active_at
		FROM sessions
		WHERE tenant_id = ?
		  AND session_id NOT IN (SELECT COALESCE(session_id, '') FROM thread_session_segments WHERE tenant_id = ?)
		ORDER BY last_active_at DESC, session_id ASC
	`, tenantID, tenantID)
	if err != nil {
		return fmt.Errorf("list legacy sessions for tenant %s: %w", tenantID, err)
	}
	legacyRows := []legacySessionRow{}
	for rows.Next() {
		var row legacySessionRow
		if err := rows.Scan(&row.sessionID, &row.kind, &row.status, &row.channel, &row.accountID, &row.peerID, &row.legacyThreadID, &row.generation, &row.createdAt, &row.updatedAt, &row.lastActiveAt); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan legacy session: %w", err)
		}
		legacyRows = append(legacyRows, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, row := range legacyRows {
		started := parseStoreTime(row.createdAt)
		updated := parseStoreTime(row.updatedAt)
		lastActive := parseStoreTime(row.lastActiveAt)
		threadID := row.legacyThreadID.String
		if threadID == "" {
			threadID = "thr_legacy_" + row.sessionID
		}
		segmentID := "seg_legacy_" + row.sessionID
		sourceSummary := "legacy " + row.kind + " session"
		if row.channel != "" || row.peerID != "" {
			sourceSummary = "legacy " + row.channel + " / " + row.peerID
		}
		expiry := s.ThreadRetentionExpiry(ctx, tenantID, lastActive)
		thread := threads.Thread{
			ThreadID:                threadID,
			TenantID:                tenantID,
			LifecycleState:          threads.LifecycleStateActive,
			CurrentSessionSegmentID: segmentID,
			SourceKind:              threads.SourceKindLegacy,
			SourceSummary:           sourceSummary,
			LastActivityAt:          lastActive,
			CreatedAt:               started,
			UpdatedAt:               updated,
			RetentionExpiresAt:      expiry,
			RedactionStatus:         threads.RedactionStatusRedacted,
		}
		if err := s.UpsertThread(ctx, thread); err != nil {
			return err
		}
		segment := threads.SessionSegment{
			SessionSegmentID: segmentID,
			ThreadID:         threadID,
			TenantID:         tenantID,
			SessionID:        row.sessionID,
			Generation:       row.generation,
			State:            row.status,
			StartedAt:        started,
			LastActiveAt:     lastActive,
			PartialEvidence:  true,
		}
		if err := s.UpsertThreadSessionSegment(ctx, segment); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) RecoverThreadLifecycleAfterRestart(ctx context.Context) (ThreadLifecycleRecoveryStats, error) {
	stats := ThreadLifecycleRecoveryStats{}
	if s == nil {
		return stats, nil
	}
	tenantIDs, err := s.threadLifecycleTenantIDs(ctx)
	if err != nil {
		return stats, err
	}
	stats.Tenants = tenantIDs
	for _, tenantID := range tenantIDs {
		before, err := s.countLegacySessionProjectionCandidates(ctx, tenantID)
		if err != nil {
			return stats, err
		}
		if err := s.ProjectLegacySessionsForTenant(ctx, tenantID); err != nil {
			return stats, err
		}
		stats.ProjectedLegacySessions += before
	}
	checked, partial, projectionWrites, err := s.recordPartialThreadRecoveryEvidence(ctx)
	if err != nil {
		return stats, err
	}
	stats.CheckedThreads = checked
	stats.PartialThreadStates = partial
	stats.RecoveryProjectionWrites = projectionWrites
	return stats, nil
}

func (s *SQLiteStore) threadLifecycleTenantIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tenant_id FROM threads WHERE tenant_id IS NOT NULL AND tenant_id != ''
		UNION
		SELECT tenant_id FROM sessions WHERE tenant_id IS NOT NULL AND tenant_id != ''
		ORDER BY tenant_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list thread lifecycle tenants: %w", err)
	}
	defer rows.Close()
	tenantIDs := []string{}
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return nil, fmt.Errorf("scan thread lifecycle tenant: %w", err)
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	return tenantIDs, rows.Err()
}

func (s *SQLiteStore) countLegacySessionProjectionCandidates(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sessions
		WHERE tenant_id = ?
		  AND session_id NOT IN (SELECT COALESCE(session_id, '') FROM thread_session_segments WHERE tenant_id = ?)
	`, tenantID, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count legacy session projection candidates %s: %w", tenantID, err)
	}
	return count, nil
}

func (s *SQLiteStore) recordPartialThreadRecoveryEvidence(ctx context.Context) (int, int, int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.document_json
		FROM threads AS t
		LEFT JOIN thread_session_segments AS seg
			ON seg.tenant_id = t.tenant_id
			AND seg.thread_id = t.thread_id
			AND seg.session_segment_id = t.current_session_segment_id
		WHERE t.current_session_segment_id IS NOT NULL
		  AND t.current_session_segment_id != ''
		  AND seg.session_segment_id IS NULL
		ORDER BY t.tenant_id ASC, t.thread_id ASC
	`)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("list partial thread recovery evidence: %w", err)
	}
	defer rows.Close()
	partialThreads := []threads.Thread{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return 0, 0, 0, fmt.Errorf("scan partial thread recovery evidence: %w", err)
		}
		var thread threads.Thread
		if err := json.Unmarshal([]byte(raw), &thread); err != nil {
			return 0, 0, 0, fmt.Errorf("decode partial thread recovery evidence: %w", err)
		}
		partialThreads = append(partialThreads, thread)
	}
	if err := rows.Err(); err != nil {
		return 0, 0, 0, err
	}

	var checked int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM threads`).Scan(&checked); err != nil {
		return 0, 0, 0, fmt.Errorf("count checked thread recovery rows: %w", err)
	}
	now := time.Now().UTC()
	writes := 0
	for _, thread := range partialThreads {
		projection := threads.BuildRuntimeProjection(threads.RuntimeProjectionInput{
			ProjectionID:     "rtp_restart_recovery_" + thread.ThreadID,
			ThreadID:         thread.ThreadID,
			TenantID:         thread.TenantID,
			SessionSegmentID: thread.CurrentSessionSegmentID,
			ResourceKind:     threads.RuntimeResourceSession,
			ResourceID:       thread.CurrentSessionSegmentID,
			Status:           "partial",
			ReasonCode:       "restore_missing_session_segment",
			OccurredAt:       now,
			SafeSummary:      "Restart recovery recorded partial thread lifecycle state",
		})
		if err := s.SaveThreadRuntimeProjection(ctx, projection); err != nil {
			return 0, 0, 0, err
		}
		writes++
	}
	return checked, len(partialThreads), writes, nil
}

func (s *SQLiteStore) SetThreadRetentionPolicy(ctx context.Context, tenantID string, expiresAt time.Time) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO thread_retention_policies (tenant_id, retention_expires_at)
		VALUES (?, ?)
		ON CONFLICT(tenant_id) DO UPDATE SET retention_expires_at = excluded.retention_expires_at
	`, tenantID, formatTime(expiresAt))
	if err != nil {
		return fmt.Errorf("set thread retention policy %s: %w", tenantID, err)
	}
	return nil
}

func (s *SQLiteStore) ThreadRetentionExpiry(ctx context.Context, tenantID string, now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	defaultExpiry := now.UTC().Add(90 * 24 * time.Hour)
	if s == nil {
		return defaultExpiry
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT retention_expires_at
		FROM thread_retention_policies
		WHERE tenant_id = ?
	`, tenantID).Scan(&raw)
	if err != nil {
		return defaultExpiry
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || parsed.Before(defaultExpiry) {
		return defaultExpiry
	}
	return parsed.UTC()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func (s *SQLiteStore) threadSegments(ctx context.Context, tenantID, threadID string) ([]threads.SessionSegment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_session_segments
		WHERE tenant_id = ? AND thread_id = ?
		ORDER BY generation ASC, session_segment_id ASC
	`, tenantID, threadID)
	if err != nil {
		return nil, fmt.Errorf("list thread session segments %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	segments := []threads.SessionSegment{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan thread session segment: %w", err)
		}
		var segment threads.SessionSegment
		if err := json.Unmarshal([]byte(raw), &segment); err != nil {
			return nil, fmt.Errorf("decode thread session segment: %w", err)
		}
		segments = append(segments, segment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return segments, nil
}

func (s *SQLiteStore) threadLifecycleActions(ctx context.Context, tenantID, threadID string) ([]threads.LifecycleAction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_lifecycle_events
		WHERE tenant_id = ? AND thread_id = ?
		ORDER BY occurred_at DESC, lifecycle_event_id DESC
	`, tenantID, threadID)
	if err != nil {
		return nil, fmt.Errorf("list thread lifecycle actions %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	actions := []threads.LifecycleAction{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan thread lifecycle action: %w", err)
		}
		var action threads.LifecycleAction
		if err := json.Unmarshal([]byte(raw), &action); err != nil {
			return nil, fmt.Errorf("decode thread lifecycle action: %w", err)
		}
		expiresAt := action.RetentionExpiresAt
		if expiresAt.IsZero() {
			expiresAt = s.ThreadRetentionExpiry(ctx, tenantID, action.CompletedAt)
		}
		if !expiresAt.After(time.Now().UTC()) {
			continue
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return actions, nil
}

func (s *SQLiteStore) threadSourceLinkages(ctx context.Context, tenantID, threadID string) ([]threads.SourceLinkage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_source_links
		WHERE tenant_id = ? AND thread_id = ?
		  AND (retention_expires_at IS NULL OR retention_expires_at = '' OR retention_expires_at > ?)
		ORDER BY linked_at DESC, source_linkage_id DESC
	`, tenantID, threadID, formatTime(time.Now().UTC()))
	if err != nil {
		return nil, fmt.Errorf("list thread source linkages %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	items := []threads.SourceLinkage{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan thread source linkage: %w", err)
		}
		var item threads.SourceLinkage
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode thread source linkage: %w", err)
		}
		expiresAt := item.RetentionExpiresAt
		if expiresAt.IsZero() {
			expiresAt = s.ThreadRetentionExpiry(ctx, tenantID, item.LinkedAt)
		}
		if !expiresAt.After(time.Now().UTC()) {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *SQLiteStore) threadRuntimeProjections(ctx context.Context, tenantID, threadID string) ([]threads.RuntimeProjection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_runtime_projections
		WHERE tenant_id = ? AND thread_id = ?
		  AND (retention_expires_at IS NULL OR retention_expires_at = '' OR retention_expires_at > ?)
		ORDER BY occurred_at DESC, runtime_projection_id DESC
	`, tenantID, threadID, formatTime(time.Now().UTC()))
	if err != nil {
		return nil, fmt.Errorf("list thread runtime projections %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	items := []threads.RuntimeProjection{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan thread runtime projection: %w", err)
		}
		var item threads.RuntimeProjection
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode thread runtime projection: %w", err)
		}
		expiresAt := item.RetentionExpiresAt
		if expiresAt.IsZero() {
			expiresAt = s.ThreadRetentionExpiry(ctx, tenantID, item.OccurredAt)
		}
		if !expiresAt.After(time.Now().UTC()) {
			continue
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *SQLiteStore) insertThreadLifecycleActionTx(ctx context.Context, tx *sql.Tx, action threads.LifecycleAction) error {
	document, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("marshal thread lifecycle action %s: %w", action.LifecycleActionID, err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO thread_lifecycle_events (
			lifecycle_event_id, thread_id, tenant_id, action, outcome, audit_event_id,
			occurred_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, action.LifecycleActionID, action.ThreadID, action.TenantID, action.ActionKind, action.Status, action.AuditEventID, formatTime(action.CompletedAt), action.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("insert thread lifecycle action %s: %w", action.LifecycleActionID, err)
	}
	return nil
}

func (s *SQLiteStore) nextThreadSegmentGeneration(ctx context.Context, tenantID, threadID string) int {
	var maxGeneration sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(generation)
		FROM thread_session_segments
		WHERE tenant_id = ? AND thread_id = ?
	`, tenantID, threadID).Scan(&maxGeneration)
	if err != nil || !maxGeneration.Valid {
		return 1
	}
	return int(maxGeneration.Int64) + 1
}

func nextThreadSegmentGenerationTx(ctx context.Context, tx *sql.Tx, tenantID, threadID string) int {
	var maxGeneration sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT MAX(generation)
		FROM thread_session_segments
		WHERE tenant_id = ? AND thread_id = ?
	`, tenantID, threadID).Scan(&maxGeneration)
	if err != nil || !maxGeneration.Valid {
		return 1
	}
	return int(maxGeneration.Int64) + 1
}

func getThreadForTenantTx(ctx context.Context, tx *sql.Tx, tenantID, threadID string) (threads.Thread, bool, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `
		SELECT document_json
		FROM threads
		WHERE tenant_id = ? AND thread_id = ?
	`, tenantID, threadID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.Thread{}, false, nil
		}
		return threads.Thread{}, false, fmt.Errorf("get thread %s/%s: %w", tenantID, threadID, err)
	}
	var thread threads.Thread
	if err := json.Unmarshal([]byte(raw), &thread); err != nil {
		return threads.Thread{}, false, fmt.Errorf("decode thread %s/%s: %w", tenantID, threadID, err)
	}
	return thread, true, nil
}

func getConversationShapeForThreadTx(ctx context.Context, tx *sql.Tx, tenantID, threadID string) (threads.ConversationShapeEvidence, bool, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `
		SELECT document_json
		FROM thread_conversation_shapes
		WHERE tenant_id = ? AND thread_id = ?
		ORDER BY updated_at DESC, conversation_shape_id DESC
		LIMIT 1
	`, tenantID, threadID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.ConversationShapeEvidence{}, false, nil
		}
		return threads.ConversationShapeEvidence{}, false, fmt.Errorf("get reset conversation shape %s/%s: %w", tenantID, threadID, err)
	}
	var evidence threads.ConversationShapeEvidence
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return threads.ConversationShapeEvidence{}, false, fmt.Errorf("decode reset conversation shape %s/%s: %w", tenantID, threadID, err)
	}
	return evidence, true, nil
}

func insertThreadResetEventTx(ctx context.Context, tx *sql.Tx, event threads.ResetEvent) error {
	if event.ResetEventID == "" {
		event.ResetEventID = newStoreID("reset")
	}
	if event.PermissionGate == "" {
		event.PermissionGate = "connectors.manage"
	}
	if event.Status == "" {
		event.Status = threads.ResetEventStatusSucceeded
	}
	if event.ReasonCode == "" {
		event.ReasonCode = threads.GroupRoomReasonScopedResetSucceeded
	}
	if event.RedactionStatus == "" {
		event.RedactionStatus = threads.RedactionStatusRedacted
	}
	document, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal reset event %s: %w", event.ResetEventID, err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO thread_reset_events (
			reset_event_id, tenant_id, thread_id, conversation_shape, source_conversation_id,
			actor_principal_id, permission_gate, prior_session_segment_id, resulting_session_segment_id,
			status, reason_code, requested_at, completed_at, audit_event_id,
			retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ResetEventID, event.TenantID, event.ThreadID, event.ConversationShape, event.SourceConversationID, event.ActorPrincipalID, event.PermissionGate, event.PriorSessionSegmentID, event.ResultingSessionSegmentID, event.Status, event.ReasonCode, formatTime(event.RequestedAt), formatTime(event.CompletedAt), event.AuditEventID, formatTime(event.RetentionExpiresAt), event.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("insert reset event %s: %w", event.ResetEventID, err)
	}
	return nil
}

func threadReopenEligibleTx(ctx context.Context, tx *sql.Tx, thread threads.Thread) (bool, error) {
	if thread.CurrentSessionSegmentID == "" {
		return false, nil
	}
	var segmentCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM thread_session_segments
		WHERE tenant_id = ? AND thread_id = ? AND session_segment_id = ?
	`, thread.TenantID, thread.ThreadID, thread.CurrentSessionSegmentID).Scan(&segmentCount); err != nil {
		return false, fmt.Errorf("check reopen session eligibility %s: %w", thread.ThreadID, err)
	}
	if segmentCount != 1 {
		return false, nil
	}
	if thread.SourceKind != threads.SourceKindChannel {
		return true, nil
	}
	var sourceCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM thread_source_links
		WHERE tenant_id = ?
		  AND thread_id = ?
		  AND current_flag = 1
		  AND COALESCE(connector_id, '') != ''
		  AND COALESCE(source_account_id, '') != ''
		  AND COALESCE(source_conversation_id, '') != ''
	`, thread.TenantID, thread.ThreadID).Scan(&sourceCount); err != nil {
		return false, fmt.Errorf("check reopen source eligibility %s: %w", thread.ThreadID, err)
	}
	return sourceCount == 1, nil
}

func parseStoreTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Now().UTC()
	}
	return parsed.UTC()
}
