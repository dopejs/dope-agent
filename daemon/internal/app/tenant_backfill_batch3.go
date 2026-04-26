package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

// Roadmap 35 (US2 / T072–T076b) — batch 3 backfill driver.
//
// Adds calendar (3) + mail (3) + reminders (3) + computer-use (3) +
// evaluation (4) + harness (5) + connector_messages = 22 step
// registrations. Most go through the generic engine; three use the
// specialized helpers in store/tenant_backfill_special.go:
//
//   - mcp_tool_exposure_rules → RunSimpleBulkBackfill (composite PK,
//     top-level → default personal tenant)
//   - sandbox_executions → RunOptionalParentBackfill (run_id optional;
//     if NULL, default personal tenant; if non-NULL, derive from runs
//     and apply orphan rule)
//   - connector_messages (T076b) → RunConnectorMessagesBackfill
//     (priority: session_id → run_id → default; orphan rule applies
//     whenever a non-NULL FK doesn't resolve to a tenant_owned row)

func runBatch3Backfills(ctx context.Context, sqliteStore *store.SQLiteStore, defaultTenantID string, bus *events.Bus, logger *telemetry.Logger) error {
	if sqliteStore == nil {
		return errors.New("runBatch3Backfills: nil store")
	}
	if defaultTenantID == "" {
		return errors.New("runBatch3Backfills: defaultTenantID required")
	}

	steps := []store.BackfillStep{
		// T072 — calendar
		{
			Name: store.CalendarAccountsBackfillStepName, Table: "calendar_accounts", PKColumn: "calendar_account_id", ChunkSize: 500,
		},
		{
			Name:           store.CalendarOperationsBackfillStepName,
			Table:          "calendar_operations",
			PKColumn:       "operation_id",
			ParentTable:    "calendar_accounts",
			ParentFKColumn: "calendar_account_id",
			ParentTablePK:  "calendar_account_id",
			ChunkSize:      500,
		},
		{
			Name:           store.CalendarArtifactsBackfillStepName,
			Table:          "calendar_artifacts",
			PKColumn:       "artifact_id",
			ParentTable:    "calendar_operations",
			ParentFKColumn: "operation_id",
			ParentTablePK:  "operation_id",
			ChunkSize:      500,
		},

		// T073 — mail
		{
			Name: store.MailAccountsBackfillStepName, Table: "mail_accounts", PKColumn: "mail_account_id", ChunkSize: 500,
		},
		{
			Name:           store.MailOperationsBackfillStepName,
			Table:          "mail_operations",
			PKColumn:       "operation_id",
			ParentTable:    "mail_accounts",
			ParentFKColumn: "mail_account_id",
			ParentTablePK:  "mail_account_id",
			ChunkSize:      500,
		},
		{
			Name:           store.MailArtifactsBackfillStepName,
			Table:          "mail_artifacts",
			PKColumn:       "artifact_id",
			ParentTable:    "mail_operations",
			ParentFKColumn: "operation_id",
			ParentTablePK:  "operation_id",
			ChunkSize:      500,
		},

		// T074 — reminders
		{
			Name: store.RemindersBackfillStepName, Table: "reminders", PKColumn: "reminder_id", ChunkSize: 500,
		},
		{
			Name:           store.ReminderOccurrencesBackfillStepName,
			Table:          "reminder_occurrences",
			PKColumn:       "occurrence_id",
			ParentTable:    "reminders",
			ParentFKColumn: "reminder_id",
			ParentTablePK:  "reminder_id",
			ChunkSize:      500,
		},
		{
			Name:           store.ReminderActionsBackfillStepName,
			Table:          "reminder_actions",
			PKColumn:       "action_id",
			ParentTable:    "reminders",
			ParentFKColumn: "reminder_id",
			ParentTablePK:  "reminder_id",
			ChunkSize:      500,
		},

		// T075 — computer-use
		{
			Name: store.ComputerUseSessionsBackfillStepName, Table: "computer_use_sessions", PKColumn: "computer_use_session_id", ChunkSize: 500,
		},
		{
			Name:           store.ComputerUseActionsBackfillStepName,
			Table:          "computer_use_actions",
			PKColumn:       "computer_use_action_id",
			ParentTable:    "computer_use_sessions",
			ParentFKColumn: "computer_use_session_id",
			ParentTablePK:  "computer_use_session_id",
			ChunkSize:      500,
		},
		{
			Name:           store.ComputerUseArtifactsBackfillStepName,
			Table:          "computer_use_artifacts",
			PKColumn:       "artifact_id",
			ParentTable:    "computer_use_sessions",
			ParentFKColumn: "computer_use_session_id",
			ParentTablePK:  "computer_use_session_id",
			ChunkSize:      500,
		},

		// T076 — evaluation
		{
			Name: store.EvaluationReplayCandidatesBackfillStepName, Table: "evaluation_replay_candidates", PKColumn: "candidate_id", ChunkSize: 500,
		},
		{
			Name:           store.EvaluationReplayAttemptsBackfillStepName,
			Table:          "evaluation_replay_attempts",
			PKColumn:       "attempt_id",
			ParentTable:    "evaluation_replay_candidates",
			ParentFKColumn: "candidate_id",
			ParentTablePK:  "candidate_id",
			ChunkSize:      500,
		},
		{
			Name: store.EvaluationComparisonsBackfillStepName, Table: "evaluation_comparisons", PKColumn: "comparison_id", ChunkSize: 500,
		},
		{
			Name: store.EvaluationRegressionFixturesBackfillStepName, Table: "evaluation_regression_fixtures", PKColumn: "fixture_id", ChunkSize: 500,
		},

		// T076a top-level harness (sandbox_executions handled below
		// via the optional-parent helper).
		{
			Name: store.ConsumerPolicyRecordsBackfillStepName, Table: "consumer_policy_records", PKColumn: "policy_record_id", ChunkSize: 500,
		},
		{
			Name: store.ProviderPreferencesBackfillStepName, Table: "provider_preferences", PKColumn: "provider_id", ChunkSize: 500,
		},
		{
			Name: store.SecretScopeBindingsBackfillStepName, Table: "secret_scope_bindings", PKColumn: "binding_id", ChunkSize: 500,
		},
		// T076a — sandbox_executions: spec described an optional run_id
		// parent, but the table has no run_id column today (see schema
		// in store.go). Treat as top-level → default personal tenant.
		// If a future schema migration adds run_id, switch this entry
		// back to the optional-parent helper.
		{
			Name: store.SandboxExecutionsBackfillStepName, Table: "sandbox_executions", PKColumn: "execution_id", ChunkSize: 500,
		},
	}
	for _, step := range steps {
		if err := executeBackfillStep(ctx, sqliteStore, step, defaultTenantID, bus, logger); err != nil {
			return err
		}
	}

	// T076a — mcp_tool_exposure_rules (composite PK, top-level).
	if err := executeSimpleBulk(ctx, sqliteStore, store.MCPToolExposureRulesBackfillStepName, "mcp_tool_exposure_rules", defaultTenantID, bus, logger); err != nil {
		return err
	}
	// T076b — connector_messages (priority: session_id → run_id → default).
	if err := executeConnectorMessages(ctx, sqliteStore, store.ConnectorMessagesBackfillStepName, defaultTenantID, bus, logger); err != nil {
		return err
	}
	return nil
}

func executeSimpleBulk(ctx context.Context, sqliteStore *store.SQLiteStore, stepName, table, defaultTenantID string, bus *events.Bus, logger *telemetry.Logger) error {
	err := sqliteStore.RunSimpleBulkBackfill(ctx, stepName, table, defaultTenantID)
	return finalizeStep(ctx, sqliteStore, stepName, err, bus, logger)
}

func executeConnectorMessages(ctx context.Context, sqliteStore *store.SQLiteStore, stepName, defaultTenantID string, bus *events.Bus, logger *telemetry.Logger) error {
	err := sqliteStore.RunConnectorMessagesBackfill(ctx, stepName, defaultTenantID)
	return finalizeStep(ctx, sqliteStore, stepName, err, bus, logger)
}

// finalizeStep mirrors the orphan-classification + step-failed
// publishing in executeBackfillStep, but for the specialized helpers
// that don't go through the generic engine.
func finalizeStep(ctx context.Context, sqliteStore *store.SQLiteStore, stepName string, err error, bus *events.Bus, logger *telemetry.Logger) error {
	if err == nil {
		if logger != nil {
			logger.Slog().Info("tenant migration step completed", "step", stepName)
		}
		return nil
	}
	var orphan *store.OrphanError
	if errors.As(err, &orphan) {
		if bus != nil {
			bus.Publish(events.Event{
				Category: "daemon.migration",
				Name:     "daemon.migration.orphan_detected",
				Resource: events.Resource{Kind: "tenant_migration", ID: stepName},
				Payload: map[string]any{
					"table":        orphan.Table,
					"parent_table": orphan.ParentTable,
					"parent_fk":    orphan.ParentFK,
					"parent_value": orphan.ParentValue,
					"step":         stepName,
				},
			})
		}
	}
	if failErr := sqliteStore.FailMigrationStep(ctx, stepName, err); failErr != nil {
		return fmt.Errorf("backfill %s: %w (also failed to mark step failed: %v)", stepName, err, failErr)
	}
	return fmt.Errorf("backfill %s: %w", stepName, err)
}
