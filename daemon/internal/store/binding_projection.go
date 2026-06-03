package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
)

// RecordRuntimeBindingEvidence appends durable runtime binding evidence. Evidence is
// append-only; binding changes never rewrite prior projections (FR-012).
func (s *SQLiteStore) RecordRuntimeBindingEvidence(ctx context.Context, evidence bindings.RuntimeBindingEvidence) (bindings.RuntimeBindingEvidence, error) {
	if strings.TrimSpace(evidence.ProjectionID) == "" {
		evidence.ProjectionID = newStoreID("brp")
	}
	if evidence.OccurredAt.IsZero() {
		evidence.OccurredAt = time.Now().UTC()
	}
	if evidence.RedactionStatus == "" {
		evidence.RedactionStatus = bindings.RedactionRedacted
	}
	summary, err := json.Marshal(evidence.CapabilityVisibility)
	if err != nil {
		return bindings.RuntimeBindingEvidence{}, err
	}
	doc, err := json.Marshal(evidence)
	if err != nil {
		return bindings.RuntimeBindingEvidence{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO binding_runtime_projections (projection_id, tenant_id, resource_kind, resource_id, selected_profile_id, selected_profile_version_id, selected_workspace_id, binding_scope, binding_id, classification, selection_reason, capability_visibility_summary, occurred_at, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		evidence.ProjectionID, evidence.TenantID, evidence.ResourceKind, evidence.ResourceID, nullableString(evidence.SelectedProfileID), nullableString(evidence.SelectedProfileVersionID), nullableString(evidence.SelectedWorkspaceID), string(evidence.BindingScope), nullableString(evidence.BindingID), string(evidence.Classification), evidence.SelectionReason, string(summary), evidence.OccurredAt.Format(time.RFC3339Nano), string(evidence.RedactionStatus), doc)
	if err != nil {
		return bindings.RuntimeBindingEvidence{}, err
	}
	return evidence, nil
}

// ListRuntimeBindingEvidence returns runtime binding evidence for a resource, newest first.
func (s *SQLiteStore) ListRuntimeBindingEvidence(ctx context.Context, tenantID, resourceKind, resourceID string, limit int) ([]bindings.RuntimeBindingEvidence, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT document_json FROM binding_runtime_projections WHERE tenant_id = ?`
	args := []any{strings.TrimSpace(tenantID)}
	if strings.TrimSpace(resourceKind) != "" {
		query += ` AND resource_kind = ?`
		args = append(args, strings.TrimSpace(resourceKind))
	}
	if strings.TrimSpace(resourceID) != "" {
		query += ` AND resource_id = ?`
		args = append(args, strings.TrimSpace(resourceID))
	}
	query += ` ORDER BY occurred_at DESC, projection_id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]bindings.RuntimeBindingEvidence, 0)
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var item bindings.RuntimeBindingEvidence
		if err := json.Unmarshal(doc, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// LatestRuntimeBindingEvidence returns the most recent evidence for a resource, if any.
func (s *SQLiteStore) LatestRuntimeBindingEvidence(ctx context.Context, tenantID, resourceKind, resourceID string) (bindings.RuntimeBindingEvidence, bool, error) {
	items, err := s.ListRuntimeBindingEvidence(ctx, tenantID, resourceKind, resourceID, 1)
	if err != nil {
		return bindings.RuntimeBindingEvidence{}, false, err
	}
	if len(items) == 0 {
		return bindings.RuntimeBindingEvidence{}, false, nil
	}
	return items[0], true, nil
}
