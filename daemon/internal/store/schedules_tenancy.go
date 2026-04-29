package store

import (
	"context"
	"fmt"
	"strings"
)

// Roadmap 35 (T029 Pass A) — schedules-family tenant-aware reads. Writes
// stay on the existing tenantless helpers and are bound via
// BindRowTenant from the tenancy subpackage.

// ListSchedulesForTenantRaw mirrors ListSchedules but filtered by tenant.
func (s *SQLiteStore) ListSchedulesForTenantRaw(ctx context.Context, tenantID, environmentScope string) ([]ScheduleRecord, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
			SELECT schedule_id, environment_scope, tenant_id, kind, status, target_ref_id, timezone, next_due_at, last_attempt_at, last_outcome, created_at, updated_at, paused_at, cancelled_at, completed_at, document_json
		FROM schedules
		WHERE tenant_id = ? AND environment_scope = ?
		ORDER BY created_at ASC, schedule_id ASC
	`, tenantID, strings.TrimSpace(environmentScope))
	if err != nil {
		return nil, fmt.Errorf("list schedules for tenant: %w", err)
	}
	defer rows.Close()
	items := make([]ScheduleRecord, 0)
	for rows.Next() {
		record, scanErr := scanScheduleRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, record)
	}
	return items, rows.Err()
}
