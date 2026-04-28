package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MigrationStepStatus is the lifecycle state of a single tenant-migration step.
type MigrationStepStatus string

const (
	MigrationStepPending   MigrationStepStatus = "pending"
	MigrationStepRunning   MigrationStepStatus = "running"
	MigrationStepCompleted MigrationStepStatus = "completed"
	MigrationStepFailed    MigrationStepStatus = "failed"
)

// MigrationStep is the durable record of a per-step migration's progress,
// matching the `tenant_migration_progress` table.
type MigrationStep struct {
	Name             string
	Status           MigrationStepStatus
	StartedAt        *time.Time
	CompletedAt      *time.Time
	LastProcessedKey string
	Error            string
}

// ErrMigrationStepFailed is returned by LoadMigrationSteps callers when any
// row carries the `failed` status. The daemon refuses to serve traffic in
// this state per Q1's resume-safety contract.
var ErrMigrationStepFailed = errors.New("tenant migration step is in failed state")

// RegisterMigrationStep inserts a `pending` row for a step name if it does
// not already exist. Idempotent: re-registering an existing step is a no-op
// regardless of the existing status. Used by the schema framework at startup
// to declare every step before any of them runs.
func (s *SQLiteStore) RegisterMigrationStep(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("migration step name required")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tenant_migration_progress (step_name, status) VALUES (?, ?)
		 ON CONFLICT(step_name) DO NOTHING`,
		name, string(MigrationStepPending))
	if err != nil {
		return fmt.Errorf("register migration step %q: %w", name, err)
	}
	return nil
}

// BeginMigrationStep transitions a step from `pending` to `running`. If the
// step is already `completed` it returns false (caller should skip). If the
// step is `failed` it returns ErrMigrationStepFailed (operator action
// required). If the step is `running` it returns true so the caller can
// resume from `last_processed_key`.
func (s *SQLiteStore) BeginMigrationStep(ctx context.Context, name string) (resume bool, err error) {
	step, err := s.GetMigrationStep(ctx, name)
	if err != nil {
		return false, err
	}
	switch step.Status {
	case MigrationStepCompleted:
		return false, nil
	case MigrationStepFailed:
		return false, fmt.Errorf("%w: %s: %s", ErrMigrationStepFailed, name, step.Error)
	case MigrationStepRunning:
		return true, nil
	}

	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx,
		`UPDATE tenant_migration_progress SET status = ?, started_at = COALESCE(started_at, ?), error = NULL WHERE step_name = ?`,
		string(MigrationStepRunning), now, name)
	if err != nil {
		return false, fmt.Errorf("begin migration step %q: %w", name, err)
	}
	return false, nil
}

// IsMigrationStepCompleted reports whether a registered migration step has
// already completed and should be skipped by callers that cannot distinguish
// BeginMigrationStep's completed and first-run false return values.
func (s *SQLiteStore) IsMigrationStepCompleted(ctx context.Context, name string) (bool, error) {
	step, err := s.GetMigrationStep(ctx, name)
	if err != nil {
		return false, err
	}
	return step.Status == MigrationStepCompleted, nil
}

// RecordMigrationChunk persists the resume cursor after a successful chunk
// commit. Callers MUST invoke this inside the same transaction (or
// immediately after) the chunk write, so the cursor never leads the data.
func (s *SQLiteStore) RecordMigrationChunk(ctx context.Context, name, lastProcessedKey string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tenant_migration_progress SET last_processed_key = ? WHERE step_name = ?`,
		lastProcessedKey, name)
	if err != nil {
		return fmt.Errorf("record migration chunk %q: %w", name, err)
	}
	return nil
}

// CompleteMigrationStep marks a step as completed. Idempotent.
func (s *SQLiteStore) CompleteMigrationStep(ctx context.Context, name string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx,
		`UPDATE tenant_migration_progress
		 SET status = ?, completed_at = ?, error = NULL
		 WHERE step_name = ?`,
		string(MigrationStepCompleted), now, name)
	if err != nil {
		return fmt.Errorf("complete migration step %q: %w", name, err)
	}
	return nil
}

// FailMigrationStep marks a step as failed and records the operator-actionable
// error message. The daemon refuses to start while any step is failed.
func (s *SQLiteStore) FailMigrationStep(ctx context.Context, name string, cause error) error {
	if cause == nil {
		cause = errors.New("unspecified migration failure")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE tenant_migration_progress SET status = ?, error = ? WHERE step_name = ?`,
		string(MigrationStepFailed), cause.Error(), name)
	if err != nil {
		return fmt.Errorf("fail migration step %q: %w", name, err)
	}
	return nil
}

// ResetMigrationStep moves a `failed` step back to `pending` for operator
// retry after the underlying issue has been resolved.
func (s *SQLiteStore) ResetMigrationStep(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tenant_migration_progress
		 SET status = ?, error = NULL, started_at = NULL
		 WHERE step_name = ? AND status = ?`,
		string(MigrationStepPending), name, string(MigrationStepFailed))
	if err != nil {
		return fmt.Errorf("reset migration step %q: %w", name, err)
	}
	return nil
}

// GetMigrationStep returns the current state of a single step. If the step
// has never been registered, the returned step has Status="pending" and the
// caller should treat that as a no-op cue (still register it before use).
func (s *SQLiteStore) GetMigrationStep(ctx context.Context, name string) (MigrationStep, error) {
	var step MigrationStep
	step.Name = name
	var startedAt, completedAt sql.NullInt64
	var lastKey, errText sql.NullString
	var status sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT status, started_at, completed_at, last_processed_key, error
		 FROM tenant_migration_progress WHERE step_name = ?`, name).
		Scan(&status, &startedAt, &completedAt, &lastKey, &errText)
	if errors.Is(err, sql.ErrNoRows) {
		step.Status = MigrationStepPending
		return step, nil
	}
	if err != nil {
		return step, fmt.Errorf("get migration step %q: %w", name, err)
	}
	step.Status = MigrationStepStatus(status.String)
	if startedAt.Valid {
		t := time.Unix(startedAt.Int64, 0).UTC()
		step.StartedAt = &t
	}
	if completedAt.Valid {
		t := time.Unix(completedAt.Int64, 0).UTC()
		step.CompletedAt = &t
	}
	step.LastProcessedKey = lastKey.String
	step.Error = errText.String
	return step, nil
}

// EventsBackfillStepName is the canonical step name for T077 (events
// backfill). Held here so callers don't need to duplicate the literal.
const EventsBackfillStepName = "tenant_migration:backfill:events"

// EventsEnforceCheckStepName is the canonical step name for T077b's CHECK
// constraint enforcement on the mixed events table. The name uses
// `enforce_check` (not `enforce_not_null`) because events allows NULL
// tenant_id for global categories.
const EventsEnforceCheckStepName = "tenant_migration:enforce_check:events"

const HostedCredentialBridgeStepName = "tenant_migration:bridge:hosted_credentials"

// RegisterEventsMigrationSteps inserts the two events-table progress rows in
// `pending` state if they don't already exist. The daemon startup path
// invokes this after schema migrations apply so that operators can observe
// the migration plan via `tenant_migration_progress` even before any
// per-domain backfills run. Idempotent.
func (s *SQLiteStore) RegisterEventsMigrationSteps(ctx context.Context) error {
	for _, name := range []string{EventsBackfillStepName, EventsEnforceCheckStepName} {
		if err := s.RegisterMigrationStep(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

// Runtime backfill step names (T067). One per tenant_owned runtime spine
// table. Backfill order is enforced by US2 — sessions before runs, runs
// before steps/checkpoints, steps before tool_calls. llm_dispatches has no
// run/step FK, so it is independent.
const (
	RuntimeBackfillSessionsStepName      = "tenant_migration:backfill:sessions"
	RuntimeBackfillRunsStepName          = "tenant_migration:backfill:runs"
	RuntimeBackfillStepsStepName         = "tenant_migration:backfill:steps"
	RuntimeBackfillToolCallsStepName     = "tenant_migration:backfill:tool_calls"
	RuntimeBackfillLLMDispatchesStepName = "tenant_migration:backfill:llm_dispatches"
	RuntimeBackfillCheckpointsStepName   = "tenant_migration:backfill:checkpoints"
)

// RegisterRuntimeMigrationSteps inserts the six runtime spine progress rows
// in `pending` state. Idempotent. Called from NewSQLiteStore so the rows
// appear as soon as the v24 schema migration applies.
func (s *SQLiteStore) RegisterRuntimeMigrationSteps(ctx context.Context) error {
	for _, name := range []string{
		RuntimeBackfillSessionsStepName,
		RuntimeBackfillRunsStepName,
		RuntimeBackfillStepsStepName,
		RuntimeBackfillToolCallsStepName,
		RuntimeBackfillLLMDispatchesStepName,
		RuntimeBackfillCheckpointsStepName,
	} {
		if err := s.RegisterMigrationStep(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

// Approvals/decisions backfill (T068) — both top-level for backfill
// purposes (decisions.approval_id is optional and the row stands alone).
const (
	ApprovalsBackfillStepName = "tenant_migration:backfill:approvals"
	DecisionsBackfillStepName = "tenant_migration:backfill:decisions"
)

// Schedules-domain backfill (T069). Order: schedules → schedule_targets,
// schedule_dispatch_attempts (both derive from schedules.tenant_id).
const (
	SchedulesBackfillStepName                = "tenant_migration:backfill:schedules"
	ScheduleTargetsBackfillStepName          = "tenant_migration:backfill:schedule_targets"
	ScheduleDispatchAttemptsBackfillStepName = "tenant_migration:backfill:schedule_dispatch_attempts"
)

// Workflows-domain backfill (T070). Order: workflows → workflow_steps,
// workflow_dependencies, workflow_handoffs (all derive from workflows).
const (
	WorkflowsBackfillStepName            = "tenant_migration:backfill:workflows"
	WorkflowStepsBackfillStepName        = "tenant_migration:backfill:workflow_steps"
	WorkflowDependenciesBackfillStepName = "tenant_migration:backfill:workflow_dependencies"
	WorkflowHandoffsBackfillStepName     = "tenant_migration:backfill:workflow_handoffs"
)

// Integrations + delivery backfill (T071). delivery_attempts derives
// from delivery_outcomes; everything else is top-level.
const (
	IntegrationsBackfillStepName           = "tenant_migration:backfill:integrations"
	DeliveryTargetsBackfillStepName        = "tenant_migration:backfill:delivery_targets"
	DeliveryPreferencesBackfillStepName    = "tenant_migration:backfill:delivery_preferences"
	DeliveryOutcomesBackfillStepName       = "tenant_migration:backfill:delivery_outcomes"
	DeliveryAttemptsBackfillStepName       = "tenant_migration:backfill:delivery_attempts"
	DeliverySummaryWindowsBackfillStepName = "tenant_migration:backfill:delivery_summary_windows"
)

// Calendar/mail/reminders/computer-use/evaluation backfill (T072–T076).
const (
	CalendarAccountsBackfillStepName   = "tenant_migration:backfill:calendar_accounts"
	CalendarOperationsBackfillStepName = "tenant_migration:backfill:calendar_operations"
	CalendarArtifactsBackfillStepName  = "tenant_migration:backfill:calendar_artifacts"

	MailAccountsBackfillStepName   = "tenant_migration:backfill:mail_accounts"
	MailOperationsBackfillStepName = "tenant_migration:backfill:mail_operations"
	MailArtifactsBackfillStepName  = "tenant_migration:backfill:mail_artifacts"

	RemindersBackfillStepName           = "tenant_migration:backfill:reminders"
	ReminderOccurrencesBackfillStepName = "tenant_migration:backfill:reminder_occurrences"
	ReminderActionsBackfillStepName     = "tenant_migration:backfill:reminder_actions"

	ComputerUseSessionsBackfillStepName  = "tenant_migration:backfill:computer_use_sessions"
	ComputerUseActionsBackfillStepName   = "tenant_migration:backfill:computer_use_actions"
	ComputerUseArtifactsBackfillStepName = "tenant_migration:backfill:computer_use_artifacts"

	EvaluationReplayCandidatesBackfillStepName   = "tenant_migration:backfill:evaluation_replay_candidates"
	EvaluationReplayAttemptsBackfillStepName     = "tenant_migration:backfill:evaluation_replay_attempts"
	EvaluationComparisonsBackfillStepName        = "tenant_migration:backfill:evaluation_comparisons"
	EvaluationRegressionFixturesBackfillStepName = "tenant_migration:backfill:evaluation_regression_fixtures"

	// Harness-domain (T076a). All top-level → default personal tenant
	// except sandbox_executions which has an optional run_id parent.
	ConsumerPolicyRecordsBackfillStepName = "tenant_migration:backfill:consumer_policy_records"
	ProviderPreferencesBackfillStepName   = "tenant_migration:backfill:provider_preferences"
	MCPToolExposureRulesBackfillStepName  = "tenant_migration:backfill:mcp_tool_exposure_rules"
	SecretScopeBindingsBackfillStepName   = "tenant_migration:backfill:secret_scope_bindings"
	SandboxExecutionsBackfillStepName     = "tenant_migration:backfill:sandbox_executions"

	// T076b parent-dependent connector_messages.
	ConnectorMessagesBackfillStepName = "tenant_migration:backfill:connector_messages"
)

// RegisterBatch3MigrationSteps inserts the T072–T076b progress rows
// in `pending` state. Idempotent.
func (s *SQLiteStore) RegisterBatch3MigrationSteps(ctx context.Context) error {
	for _, name := range []string{
		CalendarAccountsBackfillStepName,
		CalendarOperationsBackfillStepName,
		CalendarArtifactsBackfillStepName,
		MailAccountsBackfillStepName,
		MailOperationsBackfillStepName,
		MailArtifactsBackfillStepName,
		RemindersBackfillStepName,
		ReminderOccurrencesBackfillStepName,
		ReminderActionsBackfillStepName,
		ComputerUseSessionsBackfillStepName,
		ComputerUseActionsBackfillStepName,
		ComputerUseArtifactsBackfillStepName,
		EvaluationReplayCandidatesBackfillStepName,
		EvaluationReplayAttemptsBackfillStepName,
		EvaluationComparisonsBackfillStepName,
		EvaluationRegressionFixturesBackfillStepName,
		ConsumerPolicyRecordsBackfillStepName,
		ProviderPreferencesBackfillStepName,
		MCPToolExposureRulesBackfillStepName,
		SecretScopeBindingsBackfillStepName,
		SandboxExecutionsBackfillStepName,
		ConnectorMessagesBackfillStepName,
	} {
		if err := s.RegisterMigrationStep(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

// RegisterBatch2MigrationSteps inserts the T068+T069+T070+T071 progress
// rows in `pending` state. Called from NewSQLiteStore so the rows exist
// as soon as the matching ALTER TABLE migrations apply.
func (s *SQLiteStore) RegisterBatch2MigrationSteps(ctx context.Context) error {
	for _, name := range []string{
		ApprovalsBackfillStepName,
		DecisionsBackfillStepName,
		SchedulesBackfillStepName,
		ScheduleTargetsBackfillStepName,
		ScheduleDispatchAttemptsBackfillStepName,
		WorkflowsBackfillStepName,
		WorkflowStepsBackfillStepName,
		WorkflowDependenciesBackfillStepName,
		WorkflowHandoffsBackfillStepName,
		IntegrationsBackfillStepName,
		DeliveryTargetsBackfillStepName,
		DeliveryPreferencesBackfillStepName,
		DeliveryOutcomesBackfillStepName,
		DeliveryAttemptsBackfillStepName,
		DeliverySummaryWindowsBackfillStepName,
	} {
		if err := s.RegisterMigrationStep(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

// LoadMigrationSteps returns every registered step ordered by name. The
// daemon startup path uses this to gate API serving until every tenant_owned
// step reaches `completed`, and to refuse startup when any step is `failed`.
func (s *SQLiteStore) LoadMigrationSteps(ctx context.Context) ([]MigrationStep, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT step_name, status, started_at, completed_at, last_processed_key, error
		 FROM tenant_migration_progress ORDER BY step_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("load migration steps: %w", err)
	}
	defer rows.Close()

	var steps []MigrationStep
	for rows.Next() {
		var step MigrationStep
		var startedAt, completedAt sql.NullInt64
		var lastKey, errText sql.NullString
		if err := rows.Scan(&step.Name, &step.Status, &startedAt, &completedAt, &lastKey, &errText); err != nil {
			return nil, fmt.Errorf("scan migration step: %w", err)
		}
		if startedAt.Valid {
			t := time.Unix(startedAt.Int64, 0).UTC()
			step.StartedAt = &t
		}
		if completedAt.Valid {
			t := time.Unix(completedAt.Int64, 0).UTC()
			step.CompletedAt = &t
		}
		step.LastProcessedKey = lastKey.String
		step.Error = errText.String
		steps = append(steps, step)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return steps, nil
}
