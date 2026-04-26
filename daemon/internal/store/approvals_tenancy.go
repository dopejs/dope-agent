package store

import (
	"context"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/policy"
)

// Roadmap 35 (T029 Pass A) — approvals + decisions tenant-aware reads.

// ListApprovalsForTenantRaw mirrors ListApprovals but filtered by tenant.
func (s *SQLiteStore) ListApprovalsForTenantRaw(ctx context.Context, tenantID string) ([]policy.Approval, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT approval_id, action, resource_kind, resource_id, reason, requested_by, status, created_at, updated_at, resolved_at, resolution, comment, integration_bindings_json
		FROM approvals
		WHERE tenant_id = ?
		ORDER BY created_at ASC, approval_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list approvals for tenant: %w", err)
	}
	defer rows.Close()
	items := make([]policy.Approval, 0)
	for rows.Next() {
		approval, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, approval)
	}
	return items, rows.Err()
}

// ListDecisionsForTenantRaw mirrors ListDecisions but filtered by tenant.
func (s *SQLiteStore) ListDecisionsForTenantRaw(ctx context.Context, tenantID string) ([]policy.Decision, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT decision_id, action, resource_kind, resource_id, outcome, reason, approval_id, created_at
		FROM decisions
		WHERE tenant_id = ?
		ORDER BY created_at ASC, decision_id ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list decisions for tenant: %w", err)
	}
	defer rows.Close()
	items := make([]policy.Decision, 0)
	for rows.Next() {
		decision, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, decision)
	}
	return items, rows.Err()
}
