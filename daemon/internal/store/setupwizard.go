package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
)

func (s *SQLiteStore) SaveSetupSession(ctx context.Context, session setupwizard.SetupSession) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal setup session: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO setup_sessions (
			setup_session_id, tenant_id, actor_principal_id, target_id, target_kind,
			setup_style, state, reason_code, diagnostic_result_id, redaction_status,
			created_at, updated_at, last_transition_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, target_id, setup_style) DO UPDATE SET
			setup_session_id = excluded.setup_session_id,
			actor_principal_id = excluded.actor_principal_id,
			state = excluded.state,
			reason_code = excluded.reason_code,
			diagnostic_result_id = excluded.diagnostic_result_id,
			redaction_status = excluded.redaction_status,
			updated_at = excluded.updated_at,
			last_transition_at = excluded.last_transition_at,
			document_json = excluded.document_json
	`, session.SetupSessionID, session.TenantID, nullString(session.ActorPrincipalID),
		session.TargetID, string(session.TargetKind), string(session.SetupStyle), string(session.State),
		nullString(session.ReasonCode), nullString(session.DiagnosticResultID), string(session.RedactionStatus),
		formatSetupTime(session.CreatedAt), formatSetupTime(session.UpdatedAt), formatSetupTime(session.LastTransitionAt), string(document))
	if err != nil {
		return fmt.Errorf("save setup session %s: %w", session.SetupSessionID, err)
	}
	return nil
}

func (s *SQLiteStore) GetSetupSession(ctx context.Context, tenantID, sessionID string) (setupwizard.SetupSession, bool, error) {
	if s == nil {
		return setupwizard.SetupSession{}, false, nil
	}
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT document_json
		FROM setup_sessions
		WHERE tenant_id = ? AND setup_session_id = ?
	`, tenantID, sessionID).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return setupwizard.SetupSession{}, false, nil
		}
		return setupwizard.SetupSession{}, false, fmt.Errorf("get setup session %s: %w", sessionID, err)
	}
	var session setupwizard.SetupSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return setupwizard.SetupSession{}, false, fmt.Errorf("decode setup session %s: %w", sessionID, err)
	}
	return session, true, nil
}

func (s *SQLiteStore) ListSetupSessions(ctx context.Context, tenantID string) ([]setupwizard.SetupSession, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM setup_sessions
		WHERE tenant_id = ?
		ORDER BY updated_at DESC, setup_session_id DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list setup sessions: %w", err)
	}
	defer rows.Close()
	items := make([]setupwizard.SetupSession, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var session setupwizard.SetupSession
		if err := json.Unmarshal([]byte(raw), &session); err != nil {
			return nil, fmt.Errorf("decode setup session: %w", err)
		}
		items = append(items, session)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) AppendSetupAttempt(ctx context.Context, attempt setupwizard.SetupAttempt) error {
	if s == nil {
		return nil
	}
	document, err := json.Marshal(attempt)
	if err != nil {
		return fmt.Errorf("marshal setup attempt: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO setup_attempts (
			attempt_id, setup_session_id, tenant_id, actor_principal_id, operation,
			from_state, to_state, reason_code, diagnostic_result_id, redaction_status,
			created_at, document_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, attempt.AttemptID, attempt.SetupSessionID, attempt.TenantID, nullString(attempt.ActorPrincipalID),
		string(attempt.Operation), nullString(string(attempt.FromState)), string(attempt.ToState),
		nullString(attempt.ReasonCode), nullString(attempt.DiagnosticResultID), string(attempt.RedactionStatus),
		formatSetupTime(attempt.CreatedAt), string(document))
	if err != nil {
		return fmt.Errorf("append setup attempt %s: %w", attempt.AttemptID, err)
	}
	return nil
}

func (s *SQLiteStore) ListSetupAttempts(ctx context.Context, tenantID, sessionID string) ([]setupwizard.SetupAttempt, error) {
	if s == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_json
		FROM setup_attempts
		WHERE tenant_id = ? AND setup_session_id = ?
		ORDER BY created_at ASC, attempt_id ASC
	`, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list setup attempts: %w", err)
	}
	defer rows.Close()
	items := make([]setupwizard.SetupAttempt, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var attempt setupwizard.SetupAttempt
		if err := json.Unmarshal([]byte(raw), &attempt); err != nil {
			return nil, fmt.Errorf("decode setup attempt: %w", err)
		}
		items = append(items, attempt)
	}
	return items, rows.Err()
}

func formatSetupTime(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return value.UTC().Format(time.RFC3339Nano)
}
