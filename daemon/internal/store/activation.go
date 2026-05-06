package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
)

func (s *SQLiteStore) UpsertActivationState(ctx context.Context, state activation.State) error {
	if s == nil {
		return nil
	}
	if err := validateActivationStateForStorage(state); err != nil {
		return err
	}
	completedStepsJSON, err := requiredJSON(state.CompletedStepIDs)
	if err != nil {
		return fmt.Errorf("encode activation %s completed steps: %w", state.ActivationID, err)
	}
	blockingReasonsJSON, err := requiredJSON(state.BlockingReasonCodes)
	if err != nil {
		return fmt.Errorf("encode activation %s blocking reasons: %w", state.ActivationID, err)
	}
	readinessItemsJSON, err := requiredJSON(state.ReadinessItems)
	if err != nil {
		return fmt.Errorf("encode activation %s readiness items: %w", state.ActivationID, err)
	}
	quotaBaselineJSON, err := marshalJSON(state.QuotaBaseline)
	if err != nil {
		return fmt.Errorf("encode activation %s quota baseline: %w", state.ActivationID, err)
	}
	firstActionJSON, err := requiredJSON(state.FirstAction)
	if err != nil {
		return fmt.Errorf("encode activation %s first action: %w", state.ActivationID, err)
	}
	testChatJSON, err := marshalJSON(state.TestChat)
	if err != nil {
		return fmt.Errorf("encode activation %s test chat metadata: %w", state.ActivationID, err)
	}
	failureReasonJSON, err := marshalJSON(state.FailureReason)
	if err != nil {
		return fmt.Errorf("encode activation %s failure reason: %w", state.ActivationID, err)
	}
	metadataJSON, err := marshalJSON(state.Metadata)
	if err != nil {
		return fmt.Errorf("encode activation %s metadata: %w", state.ActivationID, err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO activation_states (
			activation_id, principal_id, tenant_id, environment_scope, status, current_step_id,
			completed_step_ids_json, blocking_reason_codes_json, readiness_items_json,
			quota_baseline_json, first_action_json, test_chat_json, failure_reason_json,
			created_at, updated_at, first_action_completed_at, last_evaluated_at,
			last_transition_audit_event_id, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(principal_id, tenant_id) DO UPDATE SET
			activation_id = excluded.activation_id,
			environment_scope = excluded.environment_scope,
			status = excluded.status,
			current_step_id = excluded.current_step_id,
			completed_step_ids_json = excluded.completed_step_ids_json,
			blocking_reason_codes_json = excluded.blocking_reason_codes_json,
			readiness_items_json = excluded.readiness_items_json,
			quota_baseline_json = excluded.quota_baseline_json,
			first_action_json = excluded.first_action_json,
			test_chat_json = excluded.test_chat_json,
			failure_reason_json = excluded.failure_reason_json,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			first_action_completed_at = excluded.first_action_completed_at,
			last_evaluated_at = excluded.last_evaluated_at,
			last_transition_audit_event_id = excluded.last_transition_audit_event_id,
			metadata_json = excluded.metadata_json
	`, state.ActivationID, state.PrincipalID, state.TenantID, state.EnvironmentScope, state.Status, state.CurrentStepID,
		completedStepsJSON, blockingReasonsJSON, readinessItemsJSON, quotaBaselineJSON, firstActionJSON, testChatJSON, failureReasonJSON,
		state.CreatedAt.UTC().Format(time.RFC3339Nano), state.UpdatedAt.UTC().Format(time.RFC3339Nano),
		nullableTimeString(state.FirstActionCompletedAt), state.LastEvaluatedAt.UTC().Format(time.RFC3339Nano),
		nullString(state.LastTransitionAuditEvent), metadataJSON)
	if err != nil {
		return fmt.Errorf("upsert activation state %s for principal %s tenant %s: %w", state.ActivationID, state.PrincipalID, state.TenantID, err)
	}
	return nil
}

func (s *SQLiteStore) GetActivationState(ctx context.Context, activationID string) (activation.State, bool, error) {
	if s == nil {
		return activation.State{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, activationStateSelectSQL()+` WHERE activation_id = ?`, activationID)
	state, err := scanActivationState(row)
	if err == sql.ErrNoRows {
		return activation.State{}, false, nil
	}
	if err != nil {
		return activation.State{}, false, err
	}
	return state, true, nil
}

func (s *SQLiteStore) GetActivationStateForPrincipalTenant(ctx context.Context, principalID, tenantID string) (activation.State, bool, error) {
	if s == nil {
		return activation.State{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, activationStateSelectSQL()+` WHERE principal_id = ? AND tenant_id = ?`, principalID, tenantID)
	state, err := scanActivationState(row)
	if err == sql.ErrNoRows {
		return activation.State{}, false, nil
	}
	if err != nil {
		return activation.State{}, false, err
	}
	return state, true, nil
}

func (s *SQLiteStore) ListActivationStates(ctx context.Context, environmentScope string) ([]activation.State, error) {
	if s == nil {
		return nil, nil
	}
	query := activationStateSelectSQL()
	args := []any{}
	if environmentScope != "" {
		query += ` WHERE environment_scope = ?`
		args = append(args, environmentScope)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []activation.State{}
	for rows.Next() {
		state, err := scanActivationState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, state)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func activationStateSelectSQL() string {
	return `
		SELECT activation_id, principal_id, tenant_id, environment_scope, status, current_step_id,
		       completed_step_ids_json, blocking_reason_codes_json, readiness_items_json,
		       quota_baseline_json, first_action_json, test_chat_json, failure_reason_json,
		       created_at, updated_at, first_action_completed_at, last_evaluated_at,
		       last_transition_audit_event_id, metadata_json
		FROM activation_states
	`
}

func scanActivationState(scanner interface {
	Scan(dest ...any) error
}) (activation.State, error) {
	var (
		state                      activation.State
		status                     string
		completedStepsJSON         string
		blockingReasonsJSON        string
		readinessItemsJSON         string
		quotaBaselineJSON          sql.NullString
		firstActionJSON            string
		testChatJSON               sql.NullString
		failureReasonJSON          sql.NullString
		createdAt                  string
		updatedAt                  string
		firstActionCompletedAt     sql.NullString
		lastEvaluatedAt            string
		lastTransitionAuditEventID sql.NullString
		metadataJSON               sql.NullString
	)
	if err := scanner.Scan(
		&state.ActivationID,
		&state.PrincipalID,
		&state.TenantID,
		&state.EnvironmentScope,
		&status,
		&state.CurrentStepID,
		&completedStepsJSON,
		&blockingReasonsJSON,
		&readinessItemsJSON,
		&quotaBaselineJSON,
		&firstActionJSON,
		&testChatJSON,
		&failureReasonJSON,
		&createdAt,
		&updatedAt,
		&firstActionCompletedAt,
		&lastEvaluatedAt,
		&lastTransitionAuditEventID,
		&metadataJSON,
	); err != nil {
		return activation.State{}, err
	}
	state.Status = activation.Status(status)
	if err := json.Unmarshal([]byte(completedStepsJSON), &state.CompletedStepIDs); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s completed steps: %w", state.ActivationID, err)
	}
	if err := json.Unmarshal([]byte(blockingReasonsJSON), &state.BlockingReasonCodes); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s blocking reasons: %w", state.ActivationID, err)
	}
	if err := json.Unmarshal([]byte(readinessItemsJSON), &state.ReadinessItems); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s readiness items: %w", state.ActivationID, err)
	}
	if err := unmarshalNullableJSON(quotaBaselineJSON, &state.QuotaBaseline); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s quota baseline: %w", state.ActivationID, err)
	}
	if err := json.Unmarshal([]byte(firstActionJSON), &state.FirstAction); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s first action: %w", state.ActivationID, err)
	}
	if err := unmarshalNullableJSON(testChatJSON, &state.TestChat); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s test chat metadata: %w", state.ActivationID, err)
	}
	if err := unmarshalNullableJSON(failureReasonJSON, &state.FailureReason); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s failure reason: %w", state.ActivationID, err)
	}
	if err := assignRequiredTime(&state.CreatedAt, createdAt); err != nil {
		return activation.State{}, fmt.Errorf("parse activation %s created_at: %w", state.ActivationID, err)
	}
	if err := assignRequiredTime(&state.UpdatedAt, updatedAt); err != nil {
		return activation.State{}, fmt.Errorf("parse activation %s updated_at: %w", state.ActivationID, err)
	}
	if err := assignOptionalTime(&state.FirstActionCompletedAt, firstActionCompletedAt); err != nil {
		return activation.State{}, fmt.Errorf("parse activation %s first_action_completed_at: %w", state.ActivationID, err)
	}
	if err := assignRequiredTime(&state.LastEvaluatedAt, lastEvaluatedAt); err != nil {
		return activation.State{}, fmt.Errorf("parse activation %s last_evaluated_at: %w", state.ActivationID, err)
	}
	state.LastTransitionAuditEvent = lastTransitionAuditEventID.String
	if err := unmarshalNullableJSON(metadataJSON, &state.Metadata); err != nil {
		return activation.State{}, fmt.Errorf("decode activation %s metadata: %w", state.ActivationID, err)
	}
	return state, nil
}

func validateActivationStateForStorage(state activation.State) error {
	if state.ActivationID == "" {
		return fmt.Errorf("activation state activation id is required")
	}
	if state.PrincipalID == "" {
		return fmt.Errorf("activation state %s principal id is required", state.ActivationID)
	}
	if state.TenantID == "" {
		return fmt.Errorf("activation state %s tenant id is required", state.ActivationID)
	}
	if state.EnvironmentScope == "" {
		return fmt.Errorf("activation state %s environment scope is required", state.ActivationID)
	}
	if state.Status == "" {
		return fmt.Errorf("activation state %s status is required", state.ActivationID)
	}
	if state.CurrentStepID == "" {
		return fmt.Errorf("activation state %s current step id is required", state.ActivationID)
	}
	if state.FirstAction.ActionID == "" || state.FirstAction.ActionKind == "" {
		return fmt.Errorf("activation state %s first action is required", state.ActivationID)
	}
	if state.CreatedAt.IsZero() {
		return fmt.Errorf("activation state %s created at is required", state.ActivationID)
	}
	if state.UpdatedAt.IsZero() {
		return fmt.Errorf("activation state %s updated at is required", state.ActivationID)
	}
	if state.LastEvaluatedAt.IsZero() {
		return fmt.Errorf("activation state %s last evaluated at is required", state.ActivationID)
	}
	return nil
}

func requiredJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
