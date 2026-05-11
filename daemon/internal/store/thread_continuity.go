package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

type ContinuityLookupQuery struct {
	TenantID         string
	ThreadID         string
	SessionSegmentID string
	Limit            int
	Now              time.Time
}

func (s *SQLiteStore) SaveContinuityTurn(ctx context.Context, turn threads.ContinuityTurn) (threads.ContinuityTurn, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		saved, err := s.saveContinuityTurnOnce(ctx, turn)
		if err == nil {
			return saved, nil
		}
		if !(isUniqueConstraintError(err) || isSQLiteBusyError(err)) {
			return threads.ContinuityTurn{}, err
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return threads.ContinuityTurn{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 5 * time.Millisecond):
		}
	}
	if lastErr != nil {
		return threads.ContinuityTurn{}, lastErr
	}
	return s.saveContinuityTurnOnce(ctx, turn)
}

func (s *SQLiteStore) saveContinuityTurnOnce(ctx context.Context, turn threads.ContinuityTurn) (threads.ContinuityTurn, error) {
	if s == nil {
		return turn, nil
	}
	if turn.RecordedAt.IsZero() {
		turn.RecordedAt = time.Now().UTC()
	}
	if turn.RetentionExpiresAt.IsZero() {
		turn.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, turn.TenantID, turn.RecordedAt)
	}
	if turn.ContentRedactionStatus == "" {
		turn.ContentRedactionStatus = threads.RedactionStatusRedacted
	}
	if turn.ContinuityTurnID == "" {
		turn.ContinuityTurnID = newStoreID("turn")
	}
	if err := threads.ValidateContinuityTurn(turn); err != nil {
		return threads.ContinuityTurn{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return threads.ContinuityTurn{}, fmt.Errorf("begin continuity turn: %w", err)
	}
	if turn.AcceptanceSequence == 0 {
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(acceptance_sequence), 0) + 1
			FROM thread_continuity_turns
			WHERE tenant_id = ? AND thread_id = ?
		`, turn.TenantID, turn.ThreadID).Scan(&turn.AcceptanceSequence); err != nil {
			_ = tx.Rollback()
			return threads.ContinuityTurn{}, fmt.Errorf("allocate continuity sequence: %w", err)
		}
	}
	if err := insertContinuityTurnTx(ctx, tx, turn); err != nil {
		if isUniqueConstraintError(err) && strings.TrimSpace(turn.SourceEventKey) != "" {
			_ = tx.Rollback()
			existing, found, getErr := s.getContinuityTurnBySourceEventKey(ctx, turn.TenantID, turn.SourceEventKey)
			if getErr != nil || !found {
				return threads.ContinuityTurn{}, err
			}
			return existing, nil
		}
		_ = tx.Rollback()
		return threads.ContinuityTurn{}, err
	}
	if err := tx.Commit(); err != nil {
		return threads.ContinuityTurn{}, fmt.Errorf("commit continuity turn: %w", err)
	}
	return turn, nil
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") || strings.Contains(message, "busy")
}

func insertContinuityTurnTx(ctx context.Context, tx *sql.Tx, turn threads.ContinuityTurn) error {
	document, err := json.Marshal(turn)
	if err != nil {
		return fmt.Errorf("marshal continuity turn %s: %w", turn.ContinuityTurnID, err)
	}
	var sourceTimestamp any
	if turn.SourceTimestamp != nil && !turn.SourceTimestamp.IsZero() {
		sourceTimestamp = formatTime(*turn.SourceTimestamp)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO thread_continuity_turns (
			continuity_turn_id, tenant_id, thread_id, session_segment_id, acceptance_sequence,
			role, source_kind, source_linkage_id, source_message_id, source_timestamp,
			dispatch_id, response_to_turn_id, safe_content, content_redaction_status,
			recorded_at, retention_expires_at, source_event_key, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, turn.ContinuityTurnID, turn.TenantID, turn.ThreadID, turn.SessionSegmentID, turn.AcceptanceSequence, turn.Role, turn.SourceKind, turn.SourceLinkageID, turn.SourceMessageID, sourceTimestamp, turn.DispatchID, turn.ResponseToTurnID, turn.SafeContent, turn.ContentRedactionStatus, formatTime(turn.RecordedAt), formatTime(turn.RetentionExpiresAt), nullString(turn.SourceEventKey), string(document))
	if err != nil {
		return fmt.Errorf("insert continuity turn %s: %w", turn.ContinuityTurnID, err)
	}
	return nil
}

func (s *SQLiteStore) ListContinuityTurns(ctx context.Context, query ContinuityLookupQuery) ([]threads.ContinuityTurn, error) {
	if s == nil {
		return nil, nil
	}
	now := query.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := query.Limit
	if limit <= 0 {
		limit = threads.DefaultContinuityMaxPriorTurns + 64
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.document_json
		FROM thread_continuity_turns AS c
		JOIN thread_session_segments AS s
		  ON s.tenant_id = c.tenant_id
		 AND s.thread_id = c.thread_id
		 AND s.session_segment_id = c.session_segment_id
		WHERE c.tenant_id = ?
		  AND c.thread_id = ?
		  AND c.session_segment_id = ?
		  AND c.retention_expires_at >= ?
		  AND s.partial_evidence = 0
		ORDER BY c.acceptance_sequence DESC
		LIMIT ?
	`, query.TenantID, query.ThreadID, query.SessionSegmentID, formatTime(now), limit)
	if err != nil {
		return nil, fmt.Errorf("list continuity turns %s/%s: %w", query.TenantID, query.ThreadID, err)
	}
	defer rows.Close()
	turns := []threads.ContinuityTurn{}
	for rows.Next() {
		turn, err := scanContinuityTurn(rows)
		if err != nil {
			return nil, err
		}
		turns = append(turns, turn)
	}
	return turns, rows.Err()
}

func (s *SQLiteStore) ListContinuityTurnsOutsideSessionSegment(ctx context.Context, query ContinuityLookupQuery) ([]threads.ContinuityTurn, error) {
	if s == nil {
		return nil, nil
	}
	now := query.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := query.Limit
	if limit <= 0 {
		limit = threads.DefaultContinuityMaxPriorTurns + 64
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_continuity_turns
		WHERE tenant_id = ?
		  AND thread_id = ?
		  AND session_segment_id <> ?
		  AND retention_expires_at >= ?
		ORDER BY acceptance_sequence DESC
		LIMIT ?
	`, query.TenantID, query.ThreadID, query.SessionSegmentID, formatTime(now), limit)
	if err != nil {
		return nil, fmt.Errorf("list reset-boundary continuity turns %s/%s: %w", query.TenantID, query.ThreadID, err)
	}
	defer rows.Close()
	turns := []threads.ContinuityTurn{}
	for rows.Next() {
		turn, err := scanContinuityTurn(rows)
		if err != nil {
			return nil, err
		}
		turns = append(turns, turn)
	}
	return turns, rows.Err()
}

func (s *SQLiteStore) SaveContinuityPreview(ctx context.Context, preview threads.ContinuityPreview, items []threads.ContinuityPreviewItem) (threads.ContinuityPreview, error) {
	if s == nil {
		return preview, nil
	}
	now := time.Now().UTC()
	if preview.ContinuityPreviewID == "" {
		preview.ContinuityPreviewID = newStoreID("contprev")
	}
	if preview.WindowPolicyID == "" {
		policy := threads.DefaultContinuityPolicy()
		preview.WindowPolicyID = policy.WindowPolicyID
		preview.MaxPriorTurns = policy.MaxPriorTurns
		preview.ActiveWindowDays = policy.ActiveWindowDays
	}
	if preview.MaxPriorTurns == 0 {
		preview.MaxPriorTurns = threads.DefaultContinuityMaxPriorTurns
	}
	if preview.ActiveWindowDays == 0 {
		preview.ActiveWindowDays = threads.DefaultContinuityActiveDays
	}
	if preview.AssemblyStartedAt.IsZero() {
		preview.AssemblyStartedAt = now
	}
	if preview.AssemblyCompletedAt.IsZero() {
		preview.AssemblyCompletedAt = now
	}
	if preview.AssemblyDurationMs == 0 {
		preview.AssemblyDurationMs = preview.AssemblyCompletedAt.Sub(preview.AssemblyStartedAt).Milliseconds()
	}
	if preview.RetentionExpiresAt.IsZero() {
		preview.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, preview.TenantID, preview.AssemblyCompletedAt)
	}
	if preview.RedactionStatus == "" {
		preview.RedactionStatus = threads.RedactionStatusRedacted
	}
	if preview.Status == "" {
		if preview.ContinuityApplied {
			preview.Status = threads.ContinuityStatusApplied
		} else {
			preview.Status = threads.ContinuityStatusEmpty
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return threads.ContinuityPreview{}, fmt.Errorf("begin continuity preview: %w", err)
	}
	if err := insertContinuityPreviewTx(ctx, tx, preview); err != nil {
		_ = tx.Rollback()
		return threads.ContinuityPreview{}, err
	}
	for index, item := range items {
		if item.PreviewItemID == "" {
			item.PreviewItemID = newStoreID("contitem")
		}
		item.ContinuityPreviewID = preview.ContinuityPreviewID
		item.TenantID = preview.TenantID
		item.ThreadID = preview.ThreadID
		if item.RedactionStatus == "" {
			item.RedactionStatus = threads.RedactionStatusRedacted
		}
		if item.ItemOrder == 0 {
			item.ItemOrder = index
		}
		if err := insertContinuityPreviewItemTx(ctx, tx, item); err != nil {
			_ = tx.Rollback()
			return threads.ContinuityPreview{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return threads.ContinuityPreview{}, fmt.Errorf("commit continuity preview: %w", err)
	}
	return preview, nil
}

func insertContinuityPreviewTx(ctx context.Context, tx *sql.Tx, preview threads.ContinuityPreview) error {
	document, err := json.Marshal(preview)
	if err != nil {
		return fmt.Errorf("marshal continuity preview %s: %w", preview.ContinuityPreviewID, err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO thread_continuity_previews (
			continuity_preview_id, tenant_id, thread_id, session_segment_id, dispatch_id,
			request_turn_id, response_turn_id, window_policy_id, max_prior_turns,
			active_window_days, included_count, excluded_count, continuity_applied,
			status, failure_class, assembly_started_at, assembly_completed_at,
			assembly_duration_ms, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, preview.ContinuityPreviewID, preview.TenantID, preview.ThreadID, preview.SessionSegmentID, preview.DispatchID, preview.RequestTurnID, preview.ResponseTurnID, preview.WindowPolicyID, preview.MaxPriorTurns, preview.ActiveWindowDays, preview.IncludedCount, preview.ExcludedCount, boolToInt(preview.ContinuityApplied), preview.Status, nullString(preview.FailureClass), formatTime(preview.AssemblyStartedAt), formatTime(preview.AssemblyCompletedAt), preview.AssemblyDurationMs, formatTime(preview.RetentionExpiresAt), preview.RedactionStatus, string(document))
	if err != nil {
		return fmt.Errorf("insert continuity preview %s: %w", preview.ContinuityPreviewID, err)
	}
	return nil
}

func insertContinuityPreviewItemTx(ctx context.Context, tx *sql.Tx, item threads.ContinuityPreviewItem) error {
	document, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal continuity preview item %s: %w", item.PreviewItemID, err)
	}
	var sourceTimestamp any
	if item.SourceTimestamp != nil && !item.SourceTimestamp.IsZero() {
		sourceTimestamp = formatTime(*item.SourceTimestamp)
	}
	var sequence any
	if item.AcceptanceSequence > 0 {
		sequence = item.AcceptanceSequence
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO thread_continuity_preview_items (
			preview_item_id, continuity_preview_id, tenant_id, thread_id, item_kind,
			continuity_turn_id, artifact_ref, artifact_excerpt_id, decision, reason_code,
			acceptance_sequence, source_timestamp, safe_summary, redaction_status,
			item_order, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.PreviewItemID, item.ContinuityPreviewID, item.TenantID, item.ThreadID, item.ItemKind, item.ContinuityTurnID, item.ArtifactRef, item.ArtifactExcerptID, item.Decision, item.ReasonCode, sequence, sourceTimestamp, item.SafeSummary, item.RedactionStatus, item.ItemOrder, string(document))
	if err != nil {
		return fmt.Errorf("insert continuity preview item %s: %w", item.PreviewItemID, err)
	}
	return nil
}

func (s *SQLiteStore) ListContinuityPreviewSummaries(ctx context.Context, tenantID, threadID string, limit int) ([]threads.ContinuityPreview, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_continuity_previews
		WHERE tenant_id = ? AND thread_id = ? AND retention_expires_at >= ?
		ORDER BY assembly_completed_at DESC, continuity_preview_id DESC
		LIMIT ?
	`, tenantID, threadID, formatTime(time.Now().UTC()), limit)
	if err != nil {
		return nil, fmt.Errorf("list continuity previews %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	previews := []threads.ContinuityPreview{}
	for rows.Next() {
		preview, err := scanContinuityPreview(rows)
		if err != nil {
			return nil, err
		}
		previews = append(previews, preview)
	}
	return previews, rows.Err()
}

func (s *SQLiteStore) GetContinuityPreviewDetail(ctx context.Context, tenantID, threadID, previewID string) (threads.ContinuityPreviewDetail, bool, error) {
	if s == nil {
		return threads.ContinuityPreviewDetail{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM thread_continuity_previews
		WHERE tenant_id = ? AND thread_id = ? AND continuity_preview_id = ? AND retention_expires_at >= ?
	`, tenantID, threadID, previewID, formatTime(time.Now().UTC())).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.ContinuityPreviewDetail{}, false, nil
		}
		return threads.ContinuityPreviewDetail{}, false, fmt.Errorf("get continuity preview %s/%s/%s: %w", tenantID, threadID, previewID, err)
	}
	var preview threads.ContinuityPreview
	if err := json.Unmarshal([]byte(raw), &preview); err != nil {
		return threads.ContinuityPreviewDetail{}, false, fmt.Errorf("decode continuity preview %s: %w", previewID, err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_continuity_preview_items
		WHERE continuity_preview_id = ? AND tenant_id = ? AND thread_id = ?
		ORDER BY item_order ASC, preview_item_id ASC
	`, previewID, tenantID, threadID)
	if err != nil {
		return threads.ContinuityPreviewDetail{}, false, fmt.Errorf("list continuity preview items %s: %w", previewID, err)
	}
	defer rows.Close()
	items := []threads.ContinuityPreviewItem{}
	for rows.Next() {
		var item threads.ContinuityPreviewItem
		var itemRaw string
		if err := rows.Scan(&itemRaw); err != nil {
			return threads.ContinuityPreviewDetail{}, false, fmt.Errorf("scan continuity preview item: %w", err)
		}
		if err := json.Unmarshal([]byte(itemRaw), &item); err != nil {
			return threads.ContinuityPreviewDetail{}, false, fmt.Errorf("decode continuity preview item: %w", err)
		}
		items = append(items, item)
	}
	return threads.ContinuityPreviewDetail{Preview: preview, Items: items}, true, rows.Err()
}

func (s *SQLiteStore) getContinuityTurnBySourceEventKey(ctx context.Context, tenantID, sourceEventKey string) (threads.ContinuityTurn, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM thread_continuity_turns
		WHERE tenant_id = ? AND source_event_key = ?
	`, tenantID, sourceEventKey).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.ContinuityTurn{}, false, nil
		}
		return threads.ContinuityTurn{}, false, fmt.Errorf("get continuity turn by source event key: %w", err)
	}
	var turn threads.ContinuityTurn
	if err := json.Unmarshal([]byte(raw), &turn); err != nil {
		return threads.ContinuityTurn{}, false, fmt.Errorf("decode continuity turn by source event key: %w", err)
	}
	return turn, true, nil
}

func scanContinuityTurn(scanner interface{ Scan(dest ...any) error }) (threads.ContinuityTurn, error) {
	var raw string
	if err := scanner.Scan(&raw); err != nil {
		return threads.ContinuityTurn{}, fmt.Errorf("scan continuity turn: %w", err)
	}
	var turn threads.ContinuityTurn
	if err := json.Unmarshal([]byte(raw), &turn); err != nil {
		return threads.ContinuityTurn{}, fmt.Errorf("decode continuity turn: %w", err)
	}
	return turn, nil
}

func scanContinuityPreview(scanner interface{ Scan(dest ...any) error }) (threads.ContinuityPreview, error) {
	var raw string
	if err := scanner.Scan(&raw); err != nil {
		return threads.ContinuityPreview{}, fmt.Errorf("scan continuity preview: %w", err)
	}
	var preview threads.ContinuityPreview
	if err := json.Unmarshal([]byte(raw), &preview); err != nil {
		return threads.ContinuityPreview{}, fmt.Errorf("decode continuity preview: %w", err)
	}
	return preview, nil
}
