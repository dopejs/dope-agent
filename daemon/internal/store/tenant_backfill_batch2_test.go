package store

import (
	"context"
	"testing"
)

// Roadmap 35 (US2 / T068+T069+T070+T071) — batch 2 backfill regression
// tests. Covers one top-level (approvals) and one parent-derived
// (workflow_steps via workflows.tenant_id) per the engine contract,
// using each domain's real schema columns. The engine itself is
// covered by tenant_backfill_test.go; this file proves the per-domain
// step configurations are correct.

func TestRunBackfill_Approvals_TopLevel(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"apr_aaa", "apr_bbb"} {
		seedNullTenantRow(t, s, "approvals", "approval_id", id, map[string]any{
			"action":     "test.action",
			"reason":     "r",
			"status":     "pending",
			"created_at": "2025-01-01T00:00:00Z",
			"updated_at": "2025-01-01T00:00:00Z",
		})
	}
	if err := s.RegisterMigrationStep(ctx, ApprovalsBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	step := BackfillStep{
		Name:      ApprovalsBackfillStepName,
		Table:     "approvals",
		PKColumn:  "approval_id",
		ChunkSize: 50,
	}
	if err := s.RunBackfill(ctx, step, "ten_default"); err != nil {
		t.Fatalf("RunBackfill: %v", err)
	}
	for _, id := range []string{"apr_aaa", "apr_bbb"} {
		if got := tenantOf(t, s, "approvals", "approval_id", id); got != "ten_default" {
			t.Fatalf("approval %s tenant_id=%q, want ten_default", id, got)
		}
	}
}

func TestRunBackfill_WorkflowSteps_InheritsWorkflowTenant(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed a parent run (NULL tenant_id is fine; workflow_steps does
	// not derive from runs).
	seedNullTenantRow(t, s, "runs", "run_id", "run_w", map[string]any{
		"entrypoint": "t", "status": "queued", "goal": "g",
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
	})
	// Seed a parent workflow with tenant_id already bound.
	seedNullTenantRow(t, s, "workflows", "workflow_id", "wf_a", map[string]any{
		"run_id": "run_w", "environment_scope": "test", "goal": "g", "status": "queued",
		"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
		"document_json": "{}",
	})
	if _, err := s.db.ExecContext(ctx, `UPDATE workflows SET tenant_id = ? WHERE workflow_id = ?`,
		"ten_aaaa", "wf_a"); err != nil {
		t.Fatalf("bind parent: %v", err)
	}
	// Seed two NULL-tenant workflow_steps under wf_a.
	for _, id := range []string{"wfs_1", "wfs_2"} {
		seedNullTenantRow(t, s, "workflow_steps", "workflow_step_id", id, map[string]any{
			"workflow_id":   "wf_a",
			"position":      1,
			"status":        "queued",
			"attempt_count": 0,
			"max_attempts":  3,
			"document_json": "{}",
		})
	}
	if err := s.RegisterMigrationStep(ctx, WorkflowStepsBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	step := BackfillStep{
		Name:           WorkflowStepsBackfillStepName,
		Table:          "workflow_steps",
		PKColumn:       "workflow_step_id",
		ParentTable:    "workflows",
		ParentFKColumn: "workflow_id",
		ParentTablePK:  "workflow_id",
		ChunkSize:      50,
	}
	if err := s.RunBackfill(ctx, step, "ten_irrelevant"); err != nil {
		t.Fatalf("RunBackfill: %v", err)
	}
	for _, id := range []string{"wfs_1", "wfs_2"} {
		if got := tenantOf(t, s, "workflow_steps", "workflow_step_id", id); got != "ten_aaaa" {
			t.Fatalf("workflow_step %s tenant_id=%q, want ten_aaaa", id, got)
		}
	}
}

func TestRunBackfill_DeliveryAttempts_InheritsOutcomeTenant(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Seed a parent delivery_outcome with tenant_id bound.
	seedNullTenantRow(t, s, "delivery_outcomes", "delivery_id", "del_a", map[string]any{
		"environment_scope": "test",
		"source_kind":       "run",
		"source_id":         "run_x",
		"status":            "queued",
		"updated_at":        "2025-01-01T00:00:00Z",
		"document_json":     "{}",
	})
	if _, err := s.db.ExecContext(ctx, `UPDATE delivery_outcomes SET tenant_id = ? WHERE delivery_id = ?`,
		"ten_outcome_owner", "del_a"); err != nil {
		t.Fatalf("bind outcome: %v", err)
	}
	// Seed a NULL-tenant delivery_attempt under del_a.
	seedNullTenantRow(t, s, "delivery_attempts", "attempt_id", "atm_1", map[string]any{
		"delivery_id":    "del_a",
		"attempt_number": 1,
		"target_id":      "tgt_a",
		"status":         "queued",
		"document_json":  "{}",
	})
	if err := s.RegisterMigrationStep(ctx, DeliveryAttemptsBackfillStepName); err != nil {
		t.Fatalf("register: %v", err)
	}

	step := BackfillStep{
		Name:           DeliveryAttemptsBackfillStepName,
		Table:          "delivery_attempts",
		PKColumn:       "attempt_id",
		ParentTable:    "delivery_outcomes",
		ParentFKColumn: "delivery_id",
		ParentTablePK:  "delivery_id",
		ChunkSize:      50,
	}
	if err := s.RunBackfill(ctx, step, "ten_irrelevant"); err != nil {
		t.Fatalf("RunBackfill: %v", err)
	}
	if got := tenantOf(t, s, "delivery_attempts", "attempt_id", "atm_1"); got != "ten_outcome_owner" {
		t.Fatalf("delivery_attempt tenant_id=%q, want ten_outcome_owner", got)
	}
}
