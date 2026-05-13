package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

func (s *SQLiteStore) ActiveAgentProfileSelection(ctx context.Context, tenantID string) (profiles.AgentProfile, profiles.ActiveSelection, bool, error) {
	var profileDoc []byte
	var selectionDoc []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT p.document_json, s.document_json
		FROM agent_profile_active_selections s
		JOIN agent_profiles p ON p.tenant_id = s.tenant_id AND p.profile_id = s.profile_id
		WHERE s.tenant_id = ? AND s.selection_scope = 'tenant_default'`, tenantID).Scan(&profileDoc, &selectionDoc)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return profiles.AgentProfile{}, profiles.ActiveSelection{}, false, err
		}
		defaultProfile, ensureErr := s.EnsureDefaultAgentProfile(ctx, tenantID)
		if ensureErr != nil {
			return profiles.AgentProfile{}, profiles.ActiveSelection{}, false, ensureErr
		}
		return s.ActiveAgentProfileSelection(ctx, defaultProfile.TenantID)
	}
	var profile profiles.AgentProfile
	var selection profiles.ActiveSelection
	if err := json.Unmarshal(profileDoc, &profile); err != nil {
		return profiles.AgentProfile{}, profiles.ActiveSelection{}, false, err
	}
	if err := json.Unmarshal(selectionDoc, &selection); err != nil {
		return profiles.AgentProfile{}, profiles.ActiveSelection{}, false, err
	}
	return profile, selection, true, nil
}

func (s *SQLiteStore) RecordRuntimeProfileProjection(ctx context.Context, projection profiles.RuntimeProjection) (profiles.RuntimeProjection, error) {
	if projection.RuntimeProfileProjectionID == "" {
		projection.RuntimeProfileProjectionID = newStoreID("rpp")
	}
	if projection.OccurredAt.IsZero() {
		projection.OccurredAt = time.Now().UTC()
	}
	doc, err := json.Marshal(projection)
	if err != nil {
		return profiles.RuntimeProjection{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO agent_profile_runtime_projections (runtime_profile_projection_id, tenant_id, profile_id, profile_version_id, selection_id, resource_kind, resource_id, thread_id, session_segment_id, run_id, workflow_id, handoff_link_id, selection_scope, selection_reason, safe_display_name, safe_summary, occurred_at, retention_expires_at, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		projection.RuntimeProfileProjectionID, projection.TenantID, projection.ProfileID, projection.ProfileVersionID, projection.SelectionID, projection.ResourceKind, projection.ResourceID, projection.ThreadID, projection.SessionSegmentID, projection.RunID, projection.WorkflowID, projection.HandoffLinkID, projection.SelectionScope, projection.SelectionReason, projection.SafeDisplayName, projection.SafeSummary, projection.OccurredAt.Format(time.RFC3339Nano), nullableProfileTime(projection.RetentionExpiresAt), projection.RedactionStatus, doc)
	if err != nil {
		return profiles.RuntimeProjection{}, err
	}
	return projection, nil
}

func (s *SQLiteStore) ListRuntimeProfileProjections(ctx context.Context, tenantID, resourceKind, resourceID, threadID string, limit int) ([]profiles.RuntimeProjection, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT document_json FROM agent_profile_runtime_projections WHERE tenant_id = ?`
	args := []any{tenantID}
	if resourceKind != "" {
		query += ` AND resource_kind = ?`
		args = append(args, resourceKind)
	}
	if resourceID != "" {
		query += ` AND resource_id = ?`
		args = append(args, resourceID)
	}
	if threadID != "" {
		query += ` AND thread_id = ?`
		args = append(args, threadID)
	}
	query += ` ORDER BY occurred_at DESC, runtime_profile_projection_id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []profiles.RuntimeProjection{}
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var item profiles.RuntimeProjection
		if err := json.Unmarshal(doc, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
