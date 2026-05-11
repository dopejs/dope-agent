package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func (s *SQLiteStore) SaveConversationShapeEvidence(ctx context.Context, evidence threads.ConversationShapeEvidence) (threads.ConversationShapeEvidence, error) {
	if s == nil {
		return evidence, nil
	}
	now := time.Now().UTC()
	if evidence.ConversationShapeID == "" {
		evidence.ConversationShapeID = newStoreID("shape")
	}
	if evidence.RecordedAt.IsZero() {
		evidence.RecordedAt = now
	}
	if evidence.UpdatedAt.IsZero() {
		evidence.UpdatedAt = evidence.RecordedAt
	}
	if evidence.RetentionExpiresAt.IsZero() {
		evidence.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, evidence.TenantID, evidence.UpdatedAt)
	}
	if evidence.ShapeEvidenceStatus == "" {
		evidence.ShapeEvidenceStatus = threads.ShapeEvidenceStatusProven
	}
	if evidence.RedactionStatus == "" {
		evidence.RedactionStatus = threads.RedactionStatusRedacted
	}
	document, err := json.Marshal(evidence)
	if err != nil {
		return threads.ConversationShapeEvidence{}, fmt.Errorf("marshal conversation shape evidence %s: %w", evidence.ConversationShapeID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO thread_conversation_shapes (
			conversation_shape_id, tenant_id, thread_id, session_segment_id, shape,
			source_kind, connector_id, connector_kind, source_account_id, source_conversation_id,
			shape_evidence_status, recorded_at, updated_at, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conversation_shape_id) DO UPDATE SET
			thread_id = excluded.thread_id,
			session_segment_id = excluded.session_segment_id,
			shape = excluded.shape,
			source_kind = excluded.source_kind,
			connector_id = excluded.connector_id,
			connector_kind = excluded.connector_kind,
			source_account_id = excluded.source_account_id,
			source_conversation_id = excluded.source_conversation_id,
			shape_evidence_status = excluded.shape_evidence_status,
			updated_at = excluded.updated_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, evidence.ConversationShapeID, evidence.TenantID, evidence.ThreadID, evidence.SessionSegmentID, evidence.Shape, evidence.SourceKind, evidence.ConnectorID, evidence.ConnectorKind, evidence.SourceAccountID, evidence.SourceConversationID, evidence.ShapeEvidenceStatus, formatTime(evidence.RecordedAt), formatTime(evidence.UpdatedAt), formatTime(evidence.RetentionExpiresAt), evidence.RedactionStatus, string(document))
	if err != nil {
		return threads.ConversationShapeEvidence{}, fmt.Errorf("save conversation shape evidence %s: %w", evidence.ConversationShapeID, err)
	}
	return evidence, nil
}

func (s *SQLiteStore) GetConversationShapeForThread(ctx context.Context, tenantID, threadID string) (threads.ConversationShapeEvidence, bool, error) {
	if s == nil {
		return threads.ConversationShapeEvidence{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
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
		return threads.ConversationShapeEvidence{}, false, fmt.Errorf("get conversation shape %s/%s: %w", tenantID, threadID, err)
	}
	var evidence threads.ConversationShapeEvidence
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return threads.ConversationShapeEvidence{}, false, fmt.Errorf("decode conversation shape %s/%s: %w", tenantID, threadID, err)
	}
	return evidence, true, nil
}

func (s *SQLiteStore) SaveParticipationDecision(ctx context.Context, decision threads.ParticipationDecision) (threads.ParticipationDecision, error) {
	if s == nil {
		return decision, nil
	}
	if decision.ParticipationDecisionID == "" {
		decision.ParticipationDecisionID = newStoreID("part")
	}
	if decision.OccurredAt.IsZero() {
		decision.OccurredAt = time.Now().UTC()
	}
	if decision.RetentionExpiresAt.IsZero() {
		decision.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, decision.TenantID, decision.OccurredAt)
	}
	if decision.RedactionStatus == "" {
		decision.RedactionStatus = threads.RedactionStatusRedacted
	}
	document, err := json.Marshal(decision)
	if err != nil {
		return threads.ParticipationDecision{}, fmt.Errorf("marshal participation decision %s: %w", decision.ParticipationDecisionID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO thread_participation_decisions (
			participation_decision_id, tenant_id, thread_id, session_segment_id,
			connector_id, source_account_id, source_conversation_id, source_message_id,
			conversation_shape, decision, reason_code, created_assistant_work,
			occurred_at, retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, decision.ParticipationDecisionID, decision.TenantID, decision.ThreadID, decision.SessionSegmentID, decision.ConnectorID, decision.SourceAccountID, decision.SourceConversationID, decision.SourceMessageID, decision.ConversationShape, decision.Decision, decision.ReasonCode, boolToInt(decision.CreatedAssistantWork), formatTime(decision.OccurredAt), formatTime(decision.RetentionExpiresAt), decision.RedactionStatus, string(document))
	if err != nil {
		if isUniqueConstraintError(err) && decision.SourceMessageID != "" {
			existing, found, getErr := s.GetParticipationDecisionBySourceMessage(ctx, decision.TenantID, decision.ConnectorID, decision.SourceAccountID, decision.SourceConversationID, decision.SourceMessageID)
			if getErr == nil && found {
				return existing, nil
			}
		}
		return threads.ParticipationDecision{}, fmt.Errorf("save participation decision %s: %w", decision.ParticipationDecisionID, err)
	}
	return decision, nil
}

func (s *SQLiteStore) GetParticipationDecisionBySourceMessage(ctx context.Context, tenantID, connectorID, accountID, conversationID, messageID string) (threads.ParticipationDecision, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM thread_participation_decisions
		WHERE tenant_id = ? AND connector_id = ? AND source_account_id = ? AND source_conversation_id = ? AND source_message_id = ?
		LIMIT 1
	`, tenantID, connectorID, accountID, conversationID, messageID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.ParticipationDecision{}, false, nil
		}
		return threads.ParticipationDecision{}, false, err
	}
	var decision threads.ParticipationDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return threads.ParticipationDecision{}, false, err
	}
	return decision, true, nil
}

func (s *SQLiteStore) ListParticipationDecisionsForThread(ctx context.Context, tenantID, threadID string, limit int) ([]threads.ParticipationDecision, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_participation_decisions
		WHERE tenant_id = ? AND thread_id = ?
		ORDER BY occurred_at DESC, participation_decision_id DESC
		LIMIT ?
	`, tenantID, threadID, limit)
	if err != nil {
		return nil, fmt.Errorf("list participation decisions %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	var decisions []threads.ParticipationDecision
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var decision threads.ParticipationDecision
		if err := json.Unmarshal([]byte(raw), &decision); err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}
	return decisions, rows.Err()
}

func (s *SQLiteStore) SaveResetEvent(ctx context.Context, event threads.ResetEvent) (threads.ResetEvent, error) {
	if s == nil {
		return event, nil
	}
	if event.ResetEventID == "" {
		event.ResetEventID = newStoreID("reset")
	}
	if event.RequestedAt.IsZero() {
		event.RequestedAt = time.Now().UTC()
	}
	if event.CompletedAt.IsZero() {
		event.CompletedAt = event.RequestedAt
	}
	if event.RetentionExpiresAt.IsZero() {
		event.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, event.TenantID, event.CompletedAt)
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
		return threads.ResetEvent{}, fmt.Errorf("marshal reset event %s: %w", event.ResetEventID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO thread_reset_events (
			reset_event_id, tenant_id, thread_id, conversation_shape, source_conversation_id,
			actor_principal_id, permission_gate, prior_session_segment_id, resulting_session_segment_id,
			status, reason_code, requested_at, completed_at, audit_event_id,
			retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(reset_event_id) DO UPDATE SET
			conversation_shape = excluded.conversation_shape,
			source_conversation_id = excluded.source_conversation_id,
			status = excluded.status,
			reason_code = excluded.reason_code,
			completed_at = excluded.completed_at,
			retention_expires_at = excluded.retention_expires_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, event.ResetEventID, event.TenantID, event.ThreadID, event.ConversationShape, event.SourceConversationID, event.ActorPrincipalID, event.PermissionGate, event.PriorSessionSegmentID, event.ResultingSessionSegmentID, event.Status, event.ReasonCode, formatTime(event.RequestedAt), formatTime(event.CompletedAt), event.AuditEventID, formatTime(event.RetentionExpiresAt), event.RedactionStatus, string(document))
	if err != nil {
		return threads.ResetEvent{}, fmt.Errorf("save reset event %s: %w", event.ResetEventID, err)
	}
	return event, nil
}

func (s *SQLiteStore) ListResetEventsForThread(ctx context.Context, tenantID, threadID string, limit int) ([]threads.ResetEvent, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_reset_events
		WHERE tenant_id = ? AND thread_id = ?
		ORDER BY completed_at DESC, reset_event_id DESC
		LIMIT ?
	`, tenantID, threadID, limit)
	if err != nil {
		return nil, fmt.Errorf("list reset events %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	events := []threads.ResetEvent{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var event threads.ResetEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
