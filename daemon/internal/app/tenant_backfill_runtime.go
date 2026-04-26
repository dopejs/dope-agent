package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

// Roadmap 35 (US2 / T067) — runtime spine backfill driver.
//
// Wires the six runtime backfill steps into the generic engine in
// store/tenant_backfill.go. Order is enforced (sessions → runs →
// steps/tool_calls/checkpoints with their parent FK), and the function
// is restart-safe: any step already `completed` is skipped.
//
// llm_dispatches has no parent FK; pre-roadmap-35 rows have no
// recoverable per-tenant attribution and are assigned to the default
// personal tenant. New dispatches after migration receive their
// tenant_id at write time via the tenancy.Runtime helper.
//
// On orphan detection (a child row whose parent FK does not resolve to
// a tenant_owned row), the engine returns store.OrphanError; this
// driver emits `daemon.migration.orphan_detected` carrying the table +
// parent FK value (no row data) and marks the step `failed`. The
// daemon refuses to start until the operator resolves it.

// runRuntimeBackfills executes the six T067 steps in dependency order.
// Returns nil when every step is `completed`. Failures (orphan or
// otherwise) are recorded against the step name and propagate as a
// non-nil error so the caller can surface a clear startup failure.
func runRuntimeBackfills(ctx context.Context, sqliteStore *store.SQLiteStore, defaultTenantID string, bus *events.Bus, logger *telemetry.Logger) error {
	if sqliteStore == nil {
		return errors.New("runRuntimeBackfills: nil store")
	}
	if defaultTenantID == "" {
		return errors.New("runRuntimeBackfills: defaultTenantID required")
	}

	steps := []store.BackfillStep{
		// (1) sessions — top-level
		{
			Name:      store.RuntimeBackfillSessionsStepName,
			Table:     "sessions",
			PKColumn:  "session_id",
			ChunkSize: 500,
		},
		// (2) runs — top-level
		{
			Name:      store.RuntimeBackfillRunsStepName,
			Table:     "runs",
			PKColumn:  "run_id",
			ChunkSize: 500,
		},
		// (3) steps — parent runs.tenant_id via steps.run_id FK
		{
			Name:           store.RuntimeBackfillStepsStepName,
			Table:          "steps",
			PKColumn:       "step_id",
			ParentTable:    "runs",
			ParentFKColumn: "run_id",
			ParentTablePK:  "run_id",
			ChunkSize:      500,
		},
		// (4) tool_calls — parent steps.tenant_id via tool_calls.step_id FK
		{
			Name:           store.RuntimeBackfillToolCallsStepName,
			Table:          "tool_calls",
			PKColumn:       "tool_call_id",
			ParentTable:    "steps",
			ParentFKColumn: "step_id",
			ParentTablePK:  "step_id",
			ChunkSize:      500,
		},
		// (5) llm_dispatches — top-level (no parent FK)
		{
			Name:      store.RuntimeBackfillLLMDispatchesStepName,
			Table:     "llm_dispatches",
			PKColumn:  "dispatch_id",
			ChunkSize: 500,
		},
		// (6) checkpoints — parent runs.tenant_id via checkpoints.run_id FK
		{
			Name:           store.RuntimeBackfillCheckpointsStepName,
			Table:          "checkpoints",
			PKColumn:       "checkpoint_id",
			ParentTable:    "runs",
			ParentFKColumn: "run_id",
			ParentTablePK:  "run_id",
			ChunkSize:      500,
		},
	}

	for _, step := range steps {
		if err := executeBackfillStep(ctx, sqliteStore, step, defaultTenantID, bus, logger); err != nil {
			return err
		}
	}
	return nil
}

// executeBackfillStep runs a single step, classifies orphan errors,
// emits the matching `daemon.migration.*` event, and persists the
// step's failed/completed status.
func executeBackfillStep(ctx context.Context, sqliteStore *store.SQLiteStore, step store.BackfillStep, defaultTenantID string, bus *events.Bus, logger *telemetry.Logger) error {
	err := sqliteStore.RunBackfill(ctx, step, defaultTenantID)
	if err == nil {
		if logger != nil {
			logger.Slog().Info("tenant migration step completed", "step", step.Name)
		}
		return nil
	}

	var orphan *store.OrphanError
	if errors.As(err, &orphan) {
		if bus != nil {
			bus.Publish(events.Event{
				Category: "daemon.migration",
				Name:     "daemon.migration.orphan_detected",
				Resource: events.Resource{Kind: "tenant_migration", ID: step.Name},
				Payload: map[string]any{
					"table":          orphan.Table,
					"parent_table":   orphan.ParentTable,
					"parent_fk":      orphan.ParentFK,
					"parent_value":   orphan.ParentValue,
					"step":           step.Name,
				},
			})
		}
		if logger != nil {
			logger.Slog().Error("tenant migration step orphan",
				"step", step.Name,
				"table", orphan.Table,
				"parent_table", orphan.ParentTable,
				"parent_fk", orphan.ParentFK,
				"parent_value", orphan.ParentValue,
			)
		}
	}
	if failErr := sqliteStore.FailMigrationStep(ctx, step.Name, err); failErr != nil {
		return fmt.Errorf("backfill %s: %w (also failed to mark step failed: %v)", step.Name, err, failErr)
	}
	return fmt.Errorf("backfill %s: %w", step.Name, err)
}
