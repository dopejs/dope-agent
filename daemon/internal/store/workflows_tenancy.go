package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/orchestration"
)

// Roadmap 35 (T031 Pass A) — workflows-family tenant-aware reads.

// ListWorkflowsForTenantRaw mirrors ListWorkflows but filtered by tenant.
func (s *SQLiteStore) ListWorkflowsForTenantRaw(ctx context.Context, tenantID, environmentScope, runID string) ([]orchestration.Workflow, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT workflow_id, run_id, schedule_id, schedule_attempt_id, environment_scope, goal, status, plan_summary, failure_summary, created_at, updated_at, started_at, completed_at, interrupted_at, document_json
		FROM workflows
		WHERE tenant_id = ? AND environment_scope = ? AND run_id = ?
		ORDER BY created_at ASC, workflow_id ASC
	`, tenantID, strings.TrimSpace(environmentScope), strings.TrimSpace(runID))
	if err != nil {
		return nil, fmt.Errorf("list workflows for tenant: %w", err)
	}
	defer rows.Close()
	records := make([]WorkflowRecord, 0)
	for rows.Next() {
		record, scanErr := scanWorkflowRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items := make([]orchestration.Workflow, 0, len(records))
	for _, record := range records {
		workflow, decodeErr := s.decodeWorkflowRecord(ctx, record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		items = append(items, workflow)
	}
	return items, nil
}
