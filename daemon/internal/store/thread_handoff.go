package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/threads"
)

func (s *SQLiteStore) SaveHandoffLink(ctx context.Context, link threads.HandoffLink) (threads.HandoffLink, error) {
	if s == nil {
		return link, nil
	}
	if link.HandoffLinkID == "" {
		link.HandoffLinkID = newStoreID("handoff")
	}
	if link.PermissionGate == "" {
		link.PermissionGate = "connectors.manage"
	}
	if link.Status == "" {
		link.Status = threads.HandoffStatusSucceeded
	}
	if link.SourceReferenceStatus == "" {
		link.SourceReferenceStatus = threads.HandoffSourceReferenceNone
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}
	if link.RetentionExpiresAt.IsZero() {
		link.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, link.TenantID, link.CreatedAt)
	}
	if link.RedactionStatus == "" {
		link.RedactionStatus = threads.RedactionStatusRedacted
	}
	document, err := json.Marshal(link)
	if err != nil {
		return threads.HandoffLink{}, fmt.Errorf("marshal handoff link %s: %w", link.HandoffLinkID, err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO thread_handoff_links (
			handoff_link_id, tenant_id, source_thread_id, source_session_segment_id,
			destination_thread_id, destination_session_segment_id, source_conversation_shape,
			destination_conversation_shape, source_kind, destination_kind, source_connector_id,
			destination_connector_id, source_conversation_id, destination_conversation_id,
			actor_principal_id, permission_gate, status, reason_code,
			first_destination_response_id, source_reference_status, created_at, consumed_at,
			retention_expires_at, redaction_status, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(handoff_link_id) DO UPDATE SET
			status = excluded.status,
			reason_code = excluded.reason_code,
			first_destination_response_id = excluded.first_destination_response_id,
			source_reference_status = excluded.source_reference_status,
			consumed_at = excluded.consumed_at,
			redaction_status = excluded.redaction_status,
			document_json = excluded.document_json
	`, link.HandoffLinkID, link.TenantID, link.SourceThreadID, link.SourceSessionSegmentID, link.DestinationThreadID, link.DestinationSessionSegmentID, link.SourceConversationShape, link.DestinationConversationShape, link.SourceKind, link.DestinationKind, link.SourceConnectorID, link.DestinationConnectorID, link.SourceConversationID, link.DestinationConversationID, link.ActorPrincipalID, link.PermissionGate, link.Status, link.ReasonCode, link.FirstDestinationResponseID, link.SourceReferenceStatus, formatTime(link.CreatedAt), nullableTime(link.ConsumedAt), formatTime(link.RetentionExpiresAt), link.RedactionStatus, string(document))
	if err != nil {
		return threads.HandoffLink{}, fmt.Errorf("save handoff link %s: %w", link.HandoffLinkID, err)
	}
	return link, nil
}

func (s *SQLiteStore) ListHandoffLinksForThread(ctx context.Context, tenantID, threadID string, limit int) ([]threads.HandoffLink, error) {
	if s == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_handoff_links
		WHERE tenant_id = ? AND (source_thread_id = ? OR destination_thread_id = ?)
		ORDER BY created_at DESC, handoff_link_id DESC
		LIMIT ?
	`, tenantID, threadID, threadID, limit)
	if err != nil {
		return nil, fmt.Errorf("list handoff links %s/%s: %w", tenantID, threadID, err)
	}
	defer rows.Close()
	var links []threads.HandoffLink
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var link threads.HandoffLink
		if err := json.Unmarshal([]byte(raw), &link); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *SQLiteStore) GetHandoffLink(ctx context.Context, tenantID, handoffLinkID string) (threads.HandoffLink, bool, error) {
	if s == nil {
		return threads.HandoffLink{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM thread_handoff_links
		WHERE tenant_id = ? AND handoff_link_id = ?
	`, tenantID, handoffLinkID).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return threads.HandoffLink{}, false, nil
		}
		return threads.HandoffLink{}, false, fmt.Errorf("get handoff link %s/%s: %w", tenantID, handoffLinkID, err)
	}
	var link threads.HandoffLink
	if err := json.Unmarshal([]byte(raw), &link); err != nil {
		return threads.HandoffLink{}, false, fmt.Errorf("decode handoff link %s/%s: %w", tenantID, handoffLinkID, err)
	}
	return link, true, nil
}

func (s *SQLiteStore) SaveHandoffSourceReferences(ctx context.Context, refs []threads.HandoffSourceReference) ([]threads.HandoffSourceReference, error) {
	if s == nil {
		return refs, nil
	}
	for index := range refs {
		ref := refs[index]
		if ref.HandoffSourceReferenceID == "" {
			ref.HandoffSourceReferenceID = newStoreID("href")
		}
		if ref.CreatedAt.IsZero() {
			ref.CreatedAt = time.Now().UTC()
		}
		if ref.RetentionExpiresAt.IsZero() {
			ref.RetentionExpiresAt = s.ThreadRetentionExpiry(ctx, ref.TenantID, ref.CreatedAt)
		}
		if ref.RedactionStatus == "" {
			ref.RedactionStatus = threads.RedactionStatusRedacted
		}
		document, err := json.Marshal(ref)
		if err != nil {
			return nil, fmt.Errorf("marshal handoff source reference %s: %w", ref.HandoffSourceReferenceID, err)
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO thread_handoff_source_references (
				handoff_source_reference_id, handoff_link_id, tenant_id, source_thread_id,
				source_session_segment_id, destination_thread_id, destination_session_segment_id,
				continuity_turn_id, artifact_excerpt_ref, eligibility_status, decision,
				safe_summary, redaction_status, created_at, consumed_at, retention_expires_at,
				document_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, ref.HandoffSourceReferenceID, ref.HandoffLinkID, ref.TenantID, ref.SourceThreadID, ref.SourceSessionSegmentID, ref.DestinationThreadID, ref.DestinationSessionSegmentID, ref.ContinuityTurnID, ref.ArtifactExcerptRef, ref.EligibilityStatus, ref.Decision, ref.SafeSummary, ref.RedactionStatus, formatTime(ref.CreatedAt), nullableTime(ref.ConsumedAt), formatTime(ref.RetentionExpiresAt), string(document))
		if err != nil {
			return nil, fmt.Errorf("save handoff source reference %s: %w", ref.HandoffSourceReferenceID, err)
		}
		refs[index] = ref
	}
	return refs, nil
}

func (s *SQLiteStore) ListHandoffSourceReferencesForLink(ctx context.Context, tenantID, handoffLinkID string) ([]threads.HandoffSourceReference, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_handoff_source_references
		WHERE tenant_id = ? AND handoff_link_id = ?
		ORDER BY created_at ASC, handoff_source_reference_id ASC
	`, tenantID, handoffLinkID)
	if err != nil {
		return nil, fmt.Errorf("list handoff source references %s/%s: %w", tenantID, handoffLinkID, err)
	}
	defer rows.Close()
	refs := []threads.HandoffSourceReference{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var ref threads.HandoffSourceReference
		if err := json.Unmarshal([]byte(raw), &ref); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *SQLiteStore) MarkHandoffSourceReferencesConsumed(ctx context.Context, tenantID, handoffLinkID, firstDestinationResponseID string, consumedAt time.Time) error {
	if s == nil {
		return nil
	}
	if consumedAt.IsZero() {
		consumedAt = time.Now().UTC()
	}
	link, found, err := s.GetHandoffLink(ctx, tenantID, handoffLinkID)
	if err != nil || !found {
		return err
	}
	link.SourceReferenceStatus = threads.HandoffSourceReferenceConsumed
	link.FirstDestinationResponseID = firstDestinationResponseID
	link.ConsumedAt = consumedAt
	if _, err := s.SaveHandoffLink(ctx, link); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM thread_handoff_source_references
		WHERE tenant_id = ? AND handoff_link_id = ?
	`, tenantID, handoffLinkID)
	if err != nil {
		return fmt.Errorf("list handoff refs for consume %s/%s: %w", tenantID, handoffLinkID, err)
	}
	refs := []threads.HandoffSourceReference{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return err
		}
		var ref threads.HandoffSourceReference
		if err := json.Unmarshal([]byte(raw), &ref); err != nil {
			_ = rows.Close()
			return err
		}
		ref.ConsumedAt = consumedAt
		if ref.Decision == threads.HandoffReferenceDecisionReferenced {
			ref.Decision = threads.HandoffReferenceDecisionConsumed
		}
		refs = append(refs, ref)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, ref := range refs {
		document, err := json.Marshal(ref)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `
			UPDATE thread_handoff_source_references
			SET decision = ?, consumed_at = ?, document_json = ?
			WHERE tenant_id = ? AND handoff_source_reference_id = ?
		`, ref.Decision, nullableTime(ref.ConsumedAt), string(document), tenantID, ref.HandoffSourceReferenceID); err != nil {
			return fmt.Errorf("consume handoff source reference %s: %w", ref.HandoffSourceReferenceID, err)
		}
	}
	return nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return formatTime(value)
}
