package app

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

// Roadmap 35 (US2 / T068+T069+T070+T071) — batch 2 backfill driver.
//
// Adds the approvals/decisions, schedules+children, workflows+children,
// and integrations+delivery domains. All steps reuse the engine in
// store/tenant_backfill.go. Order is enforced inside each domain so a
// child step never runs before its parent's tenant_id is bound.
//
// approvals + decisions: both top-level (decisions.approval_id is
// optional and the row stands alone — backfill independent of
// approvals).
//
// schedules → schedule_targets, schedule_dispatch_attempts (both
// derive from schedules.tenant_id).
//
// workflows → workflow_steps, workflow_dependencies, workflow_handoffs
// (all three derive from workflows.tenant_id).
//
// integrations + delivery: top-level for integrations, delivery_targets,
// delivery_preferences, delivery_outcomes, delivery_summary_windows;
// delivery_attempts derives from delivery_outcomes.tenant_id via
// delivery_attempts.delivery_id FK.

func runBatch2Backfills(ctx context.Context, sqliteStore *store.SQLiteStore, defaultTenantID string, bus *events.Bus, logger *telemetry.Logger) error {
	if sqliteStore == nil {
		return errors.New("runBatch2Backfills: nil store")
	}
	if defaultTenantID == "" {
		return errors.New("runBatch2Backfills: defaultTenantID required")
	}

	steps := []store.BackfillStep{
		// T068 — approvals/decisions
		{
			Name: store.ApprovalsBackfillStepName, Table: "approvals", PKColumn: "approval_id", ChunkSize: 500,
		},
		{
			Name: store.DecisionsBackfillStepName, Table: "decisions", PKColumn: "decision_id", ChunkSize: 500,
		},

		// T069 — schedules domain
		{
			Name: store.SchedulesBackfillStepName, Table: "schedules", PKColumn: "schedule_id", ChunkSize: 500,
		},
		{
			Name:           store.ScheduleTargetsBackfillStepName,
			Table:          "schedule_targets",
			PKColumn:       "target_ref_id",
			ParentTable:    "schedules",
			ParentFKColumn: "schedule_id",
			ParentTablePK:  "schedule_id",
			ChunkSize:      500,
		},
		{
			Name:           store.ScheduleDispatchAttemptsBackfillStepName,
			Table:          "schedule_dispatch_attempts",
			PKColumn:       "attempt_id",
			ParentTable:    "schedules",
			ParentFKColumn: "schedule_id",
			ParentTablePK:  "schedule_id",
			ChunkSize:      500,
		},

		// T070 — workflows domain
		{
			Name: store.WorkflowsBackfillStepName, Table: "workflows", PKColumn: "workflow_id", ChunkSize: 500,
		},
		{
			Name:           store.WorkflowStepsBackfillStepName,
			Table:          "workflow_steps",
			PKColumn:       "workflow_step_id",
			ParentTable:    "workflows",
			ParentFKColumn: "workflow_id",
			ParentTablePK:  "workflow_id",
			ChunkSize:      500,
		},
		{
			Name:           store.WorkflowDependenciesBackfillStepName,
			Table:          "workflow_dependencies",
			PKColumn:       "dependency_id",
			ParentTable:    "workflows",
			ParentFKColumn: "workflow_id",
			ParentTablePK:  "workflow_id",
			ChunkSize:      500,
		},
		{
			Name:           store.WorkflowHandoffsBackfillStepName,
			Table:          "workflow_handoffs",
			PKColumn:       "handoff_id",
			ParentTable:    "workflows",
			ParentFKColumn: "workflow_id",
			ParentTablePK:  "workflow_id",
			ChunkSize:      500,
		},

		// T071 — integrations + delivery
		{
			Name: store.IntegrationsBackfillStepName, Table: "integrations", PKColumn: "integration_id", ChunkSize: 500,
		},
		{
			Name: store.DeliveryTargetsBackfillStepName, Table: "delivery_targets", PKColumn: "target_id", ChunkSize: 500,
		},
		{
			Name: store.DeliveryPreferencesBackfillStepName, Table: "delivery_preferences", PKColumn: "preference_id", ChunkSize: 500,
		},
		{
			Name: store.DeliveryOutcomesBackfillStepName, Table: "delivery_outcomes", PKColumn: "delivery_id", ChunkSize: 500,
		},
		{
			Name:           store.DeliveryAttemptsBackfillStepName,
			Table:          "delivery_attempts",
			PKColumn:       "attempt_id",
			ParentTable:    "delivery_outcomes",
			ParentFKColumn: "delivery_id",
			ParentTablePK:  "delivery_id",
			ChunkSize:      500,
		},
		{
			Name: store.DeliverySummaryWindowsBackfillStepName, Table: "delivery_summary_windows", PKColumn: "summary_window_id", ChunkSize: 500,
		},
	}

	for _, step := range steps {
		if err := executeBackfillStep(ctx, sqliteStore, step, defaultTenantID, bus, logger); err != nil {
			return err
		}
	}
	return nil
}
