package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
)

// Roadmap 35 (T028 Pass A — atomicity fix).
//
// The previous Pass A pattern composed `legacy Upsert` + `BindRowTenant`
// in two separate SQL statements. That pattern leaked: when tenant B
// attempted to write a row with the same primary key as a row owned by
// tenant A, the legacy UPSERT (`ON CONFLICT DO UPDATE`) would overwrite
// A's columns BEFORE the bind helper noticed the mismatch — A's data
// was clobbered and B got an error.
//
// The helpers in this file are atomic: they include `tenant_id` in the
// INSERT and gate the ON-CONFLICT branch with
// `WHERE table.tenant_id IS NULL OR table.tenant_id = excluded.tenant_id`.
// The whole statement is one SQL call; if the WHERE is false, zero rows
// are touched and the helper returns ErrCrossTenantRow without mutating
// any state. Pre-backfill rows whose `tenant_id` is still NULL are
// accepted (the WHERE matches `IS NULL`) and have their tenant_id
// claimed in the same statement.

// UpsertRunForTenantSafe persists a run row binding `tenant_id` in the
// same INSERT. Returns ErrCrossTenantRow if the row exists and is
// owned by a different tenant — the existing row is preserved.
func (s *SQLiteStore) UpsertRunForTenantSafe(ctx context.Context, run runtime.Run, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("UpsertRunForTenantSafe: empty tenantID")
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (
			run_id,
			session_id,
			schedule_id,
			schedule_attempt_id,
			reminder_id,
			reminder_occurrence_id,
			entrypoint,
			status,
			goal,
			created_at,
			updated_at,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			session_id = excluded.session_id,
			schedule_id = excluded.schedule_id,
			schedule_attempt_id = excluded.schedule_attempt_id,
			reminder_id = excluded.reminder_id,
			reminder_occurrence_id = excluded.reminder_occurrence_id,
			entrypoint = excluded.entrypoint,
			status = excluded.status,
			goal = excluded.goal,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			tenant_id = excluded.tenant_id
		WHERE runs.tenant_id IS NULL OR runs.tenant_id = excluded.tenant_id
	`,
		run.RunID,
		nullString(run.SessionID),
		nullString(run.ScheduleID),
		nullString(run.ScheduleAttemptID),
		nullString(run.ReminderID),
		nullString(run.ReminderOccurrenceID),
		run.Entrypoint,
		string(run.Status),
		run.Goal,
		run.CreatedAt.UTC().Format(time.RFC3339Nano),
		run.UpdatedAt.UTC().Format(time.RFC3339Nano),
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("upsert run for tenant: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		// Either the row exists in another tenant (ON CONFLICT WHERE
		// blocked the update), or — rare with INSERT — the engine made
		// no change. Distinguish with a follow-up lookup so we never
		// silently drop a write.
		owner, found, err := s.LookupRowTenant(ctx, "runs", "run_id", run.RunID)
		if err != nil {
			return err
		}
		if found && owner != "" && owner != tenantID {
			return ErrCrossTenantRow
		}
	}
	return nil
}

// UpsertSessionForTenantSafe persists a session row binding tenant_id in
// the same statement.
func (s *SQLiteStore) UpsertSessionForTenantSafe(ctx context.Context, session router.Session, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("UpsertSessionForTenantSafe: empty tenantID")
	}
	var lastResetAt sql.NullString
	if session.LastResetAt != nil {
		lastResetAt = sql.NullString{String: session.LastResetAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (
			session_id,
			kind,
			status,
			channel,
			account_id,
			peer_id,
			thread_id,
			routing_key,
			generation,
			created_at,
			updated_at,
			last_active_at,
			last_reset_at,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			kind = excluded.kind,
			status = excluded.status,
			channel = excluded.channel,
			account_id = excluded.account_id,
			peer_id = excluded.peer_id,
			thread_id = excluded.thread_id,
			routing_key = excluded.routing_key,
			generation = excluded.generation,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			last_active_at = excluded.last_active_at,
			last_reset_at = excluded.last_reset_at,
			tenant_id = excluded.tenant_id
		WHERE sessions.tenant_id IS NULL OR sessions.tenant_id = excluded.tenant_id
	`,
		session.SessionID,
		string(session.Kind),
		string(session.Status),
		session.Channel,
		nullString(session.AccountID),
		session.PeerID,
		nullString(session.ThreadID),
		session.RoutingKey,
		session.Generation,
		session.CreatedAt.UTC().Format(time.RFC3339Nano),
		session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		session.LastActiveAt.UTC().Format(time.RFC3339Nano),
		lastResetAt,
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("upsert session for tenant: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		owner, found, err := s.LookupRowTenant(ctx, "sessions", "session_id", session.SessionID)
		if err != nil {
			return err
		}
		if found && owner != "" && owner != tenantID {
			return ErrCrossTenantRow
		}
	}
	return nil
}

// UpsertLLMDispatchForTenantSafe persists an llm dispatch row binding
// tenant_id in the same statement.
func (s *SQLiteStore) UpsertLLMDispatchForTenantSafe(ctx context.Context, dispatch llm.Dispatch, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("UpsertLLMDispatchForTenantSafe: empty tenantID")
	}
	messagesJSON, err := marshalJSON(dispatch.Messages)
	if err != nil {
		return fmt.Errorf("marshal llm dispatch messages: %w", err)
	}
	usageJSON, err := marshalJSON(dispatch.Usage)
	if err != nil {
		return fmt.Errorf("marshal llm dispatch usage: %w", err)
	}
	var startedAt, completedAt sql.NullString
	if dispatch.StartedAt != nil {
		startedAt = sql.NullString{String: dispatch.StartedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	if dispatch.CompletedAt != nil {
		completedAt = sql.NullString{String: dispatch.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	stream := 0
	if dispatch.Stream {
		stream = 1
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO llm_dispatches (
			dispatch_id,
			provider,
			model,
			messages_json,
			stream,
			status,
			output_text,
			finish_reason,
			usage_json,
			error_code,
			error_text,
			timeout_ms,
			max_retries,
			attempt_count,
			created_at,
			updated_at,
			started_at,
			completed_at,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dispatch_id) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			messages_json = excluded.messages_json,
			stream = excluded.stream,
			status = excluded.status,
			output_text = excluded.output_text,
			finish_reason = excluded.finish_reason,
			usage_json = excluded.usage_json,
			error_code = excluded.error_code,
			error_text = excluded.error_text,
			timeout_ms = excluded.timeout_ms,
			max_retries = excluded.max_retries,
			attempt_count = excluded.attempt_count,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			tenant_id = excluded.tenant_id
		WHERE llm_dispatches.tenant_id IS NULL OR llm_dispatches.tenant_id = excluded.tenant_id
	`,
		dispatch.DispatchID,
		dispatch.Provider,
		dispatch.Model,
		messagesJSON,
		stream,
		string(dispatch.Status),
		dispatch.Output,
		nullString(dispatch.FinishReason),
		usageJSON,
		nullString(dispatch.ErrorCode),
		nullString(dispatch.Error),
		dispatch.TimeoutMs,
		dispatch.MaxRetries,
		dispatch.AttemptCount,
		dispatch.CreatedAt.UTC().Format(time.RFC3339Nano),
		dispatch.UpdatedAt.UTC().Format(time.RFC3339Nano),
		startedAt,
		completedAt,
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("upsert llm dispatch for tenant: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		owner, found, err := s.LookupRowTenant(ctx, "llm_dispatches", "dispatch_id", dispatch.DispatchID)
		if err != nil {
			return err
		}
		if found && owner != "" && owner != tenantID {
			return ErrCrossTenantRow
		}
	}
	return nil
}

// UpsertStepForTenantSafe persists a step row binding tenant_id in the
// same statement. Cross-tenant collisions are atomically refused —
// tenant A's row is never modified by tenant B's write attempt.
func (s *SQLiteStore) UpsertStepForTenantSafe(ctx context.Context, step runtime.Step, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("UpsertStepForTenantSafe: empty tenantID")
	}
	inputJSON, err := marshalJSON(step.Input)
	if err != nil {
		return fmt.Errorf("marshal step input: %w", err)
	}
	outputJSON, err := marshalJSON(step.Output)
	if err != nil {
		return fmt.Errorf("marshal step output: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO steps (
			step_id,
			run_id,
			workflow_id,
			workflow_step_id,
			attempt,
			title,
			kind,
			status,
			input_json,
			output_json,
			created_at,
			updated_at,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(step_id) DO UPDATE SET
			run_id = excluded.run_id,
			workflow_id = excluded.workflow_id,
			workflow_step_id = excluded.workflow_step_id,
			attempt = excluded.attempt,
			title = excluded.title,
			kind = excluded.kind,
			status = excluded.status,
			input_json = excluded.input_json,
			output_json = excluded.output_json,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			tenant_id = excluded.tenant_id
		WHERE steps.tenant_id IS NULL OR steps.tenant_id = excluded.tenant_id
	`,
		step.StepID,
		step.RunID,
		nullString(step.WorkflowID),
		nullString(step.WorkflowStepID),
		step.Attempt,
		step.Title,
		step.Kind,
		string(step.Status),
		inputJSON,
		outputJSON,
		step.CreatedAt.UTC().Format(time.RFC3339Nano),
		step.UpdatedAt.UTC().Format(time.RFC3339Nano),
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("upsert step for tenant: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		owner, found, err := s.LookupRowTenant(ctx, "steps", "step_id", step.StepID)
		if err != nil {
			return err
		}
		if found && owner != "" && owner != tenantID {
			return ErrCrossTenantRow
		}
	}
	return nil
}

// UpsertToolCallForTenantSafe persists a tool_call row binding tenant_id
// in the same statement.
func (s *SQLiteStore) UpsertToolCallForTenantSafe(ctx context.Context, toolCall runtime.ToolCall, tenantID string) error {
	if s == nil {
		return nil
	}
	if tenantID == "" {
		return errors.New("UpsertToolCallForTenantSafe: empty tenantID")
	}
	inputJSON, err := marshalJSON(toolCall.Input)
	if err != nil {
		return fmt.Errorf("marshal tool call input: %w", err)
	}
	outputJSON, err := marshalJSON(toolCall.Output)
	if err != nil {
		return fmt.Errorf("marshal tool call output: %w", err)
	}
	sandboxJSON, err := marshalJSON(toolCall.Sandbox)
	if err != nil {
		return fmt.Errorf("marshal tool call sandbox metadata: %w", err)
	}
	integrationBindingsJSON, err := marshalJSON(toolCall.IntegrationBindings)
	if err != nil {
		return fmt.Errorf("marshal tool call integration bindings: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO tool_calls (
			tool_call_id,
			run_id,
			step_id,
			workflow_id,
			workflow_step_id,
			attempt,
			computer_use_session_id,
			computer_use_action_id,
			invocation_kind,
			capability_id,
			skill_id,
			mcp_server_id,
			mcp_server_name,
			mcp_tool_name,
			mcp_transport_kind,
			mcp_session_id,
			authorization_result,
			tool_name,
			status,
			sandbox_execution_id,
			failure_class,
			input_json,
			output_json,
			sandbox_json,
			integration_bindings_json,
			error_text,
			created_at,
			updated_at,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tool_call_id) DO UPDATE SET
			run_id = excluded.run_id,
			step_id = excluded.step_id,
			workflow_id = excluded.workflow_id,
			workflow_step_id = excluded.workflow_step_id,
			attempt = excluded.attempt,
			computer_use_session_id = excluded.computer_use_session_id,
			computer_use_action_id = excluded.computer_use_action_id,
			invocation_kind = excluded.invocation_kind,
			capability_id = excluded.capability_id,
			skill_id = excluded.skill_id,
			mcp_server_id = excluded.mcp_server_id,
			mcp_server_name = excluded.mcp_server_name,
			mcp_tool_name = excluded.mcp_tool_name,
			mcp_transport_kind = excluded.mcp_transport_kind,
			mcp_session_id = excluded.mcp_session_id,
			authorization_result = excluded.authorization_result,
			tool_name = excluded.tool_name,
			status = excluded.status,
			sandbox_execution_id = excluded.sandbox_execution_id,
			failure_class = excluded.failure_class,
			input_json = excluded.input_json,
			output_json = excluded.output_json,
			sandbox_json = excluded.sandbox_json,
			integration_bindings_json = excluded.integration_bindings_json,
			error_text = excluded.error_text,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			tenant_id = excluded.tenant_id
		WHERE tool_calls.tenant_id IS NULL OR tool_calls.tenant_id = excluded.tenant_id
	`,
		toolCall.ToolCallID,
		toolCall.RunID,
		toolCall.StepID,
		nullString(toolCall.WorkflowID),
		nullString(toolCall.WorkflowStepID),
		toolCall.Attempt,
		nullString(toolCall.ComputerUseSessionID),
		nullString(toolCall.ComputerUseActionID),
		nullString(string(toolCall.InvocationKind)),
		toolCall.CapabilityID,
		nullString(toolCall.SkillID),
		nullString(toolCall.MCPServerID),
		nullString(toolCall.MCPServerName),
		nullString(toolCall.MCPToolName),
		nullString(toolCall.MCPTransportKind),
		nullString(toolCall.MCPSessionID),
		nullString(toolCall.AuthorizationResult),
		toolCall.ToolName,
		string(toolCall.Status),
		nullString(toolCall.SandboxExecutionID),
		nullString(toolCall.FailureClass),
		inputJSON,
		outputJSON,
		sandboxJSON,
		integrationBindingsJSON,
		nullString(toolCall.Error),
		toolCall.CreatedAt.UTC().Format(time.RFC3339Nano),
		toolCall.UpdatedAt.UTC().Format(time.RFC3339Nano),
		tenantID,
	)
	if err != nil {
		return fmt.Errorf("upsert tool_call for tenant: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		owner, found, err := s.LookupRowTenant(ctx, "tool_calls", "tool_call_id", toolCall.ToolCallID)
		if err != nil {
			return err
		}
		if found && owner != "" && owner != tenantID {
			return ErrCrossTenantRow
		}
	}
	return nil
}

// AppendEventForTenantSafe writes an event row with `tenant_id`
// pre-bound in a single INSERT. Unlike events table updates (which
// don't happen — events are append-only), this is a straightforward
// INSERT plus tenant_id assignment. Returns the persisted event with
// Sequence and TenantID populated.
//
// The events table is mixed (tenant-owned + global categories share it).
// Callers MUST refuse to call this for global categories — the tenancy
// subpackage's IsGlobalCategory gate enforces it.
func (s *SQLiteStore) AppendEventForTenantSafe(ctx context.Context, event sqliteAppendEventInput, tenantID string) (sqliteAppendEventResult, error) {
	if s == nil {
		return sqliteAppendEventResult{}, nil
	}
	if tenantID == "" {
		return sqliteAppendEventResult{}, errors.New("AppendEventForTenantSafe: empty tenantID")
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO events (
			event_id,
			environment_scope,
			category,
			name,
			occurred_at,
			session_id,
			run_id,
			workflow_id,
			workflow_step_id,
			schedule_id,
			schedule_attempt_id,
			step_id,
			connector_id,
			capability_id,
			resource_kind,
			resource_id,
			payload_json,
			tenant_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id) DO NOTHING
	`,
		event.EventID,
		event.EnvironmentScope,
		event.Category,
		event.Name,
		event.OccurredAt.UTC().Format(time.RFC3339Nano),
		nullString(event.SessionID),
		nullString(event.RunID),
		nullString(event.WorkflowID),
		nullString(event.WorkflowStepID),
		nullString(event.ScheduleID),
		nullString(event.ScheduleAttemptID),
		nullString(event.StepID),
		nullString(event.ConnectorID),
		nullString(event.CapabilityID),
		event.ResourceKind,
		event.ResourceID,
		event.PayloadJSON,
		tenantID,
	); err != nil {
		return sqliteAppendEventResult{}, fmt.Errorf("append event for tenant: %w", err)
	}
	var sequence int64
	if err := s.db.QueryRowContext(ctx, `SELECT rowid FROM events WHERE event_id = ?`, event.EventID).Scan(&sequence); err != nil {
		return sqliteAppendEventResult{}, fmt.Errorf("load event sequence for tenant: %w", err)
	}
	return sqliteAppendEventResult{Sequence: sequence, TenantID: tenantID}, nil
}

// sqliteAppendEventInput collects the columns the events INSERT needs,
// keeping AppendEventForTenantSafe independent of the higher-level
// events.Event struct for testability.
type sqliteAppendEventInput struct {
	EventID           string
	EnvironmentScope  string
	Category          string
	Name              string
	OccurredAt        time.Time
	SessionID         string
	RunID             string
	WorkflowID        string
	WorkflowStepID    string
	ScheduleID        string
	ScheduleAttemptID string
	StepID            string
	ConnectorID       string
	CapabilityID      string
	ResourceKind      string
	ResourceID        string
	PayloadJSON       string
}

type sqliteAppendEventResult struct {
	Sequence int64
	TenantID string
}
