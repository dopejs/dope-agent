package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/capabilities"
	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	_ "modernc.org/sqlite"
)

const (
	defaultDatabaseFile  = "daemon.sqlite"
	CurrentSchemaVersion = 2
)

type schemaMigration struct {
	Version    int
	Name       string
	Statements []string
}

var schemaMigrations = []schemaMigration{
	{
		Version: 1,
		Name:    "baseline",
		Statements: []string{
			`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS sessions (
				session_id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				status TEXT NOT NULL,
				channel TEXT NOT NULL,
				account_id TEXT,
				peer_id TEXT NOT NULL,
				thread_id TEXT,
				routing_key TEXT NOT NULL UNIQUE,
				generation INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				last_active_at TEXT NOT NULL,
				last_reset_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS auth_pairings (
				pairing_id TEXT PRIMARY KEY,
				mode TEXT NOT NULL,
				label TEXT NOT NULL,
				status TEXT NOT NULL,
				code_hash TEXT NOT NULL,
				code_preview TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				expires_at TEXT NOT NULL,
				completed_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS auth_tokens (
				token_id TEXT PRIMARY KEY,
				label TEXT NOT NULL,
				mode TEXT NOT NULL,
				token_hash TEXT NOT NULL,
				token_preview TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				last_used_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS approvals (
				approval_id TEXT PRIMARY KEY,
				action TEXT NOT NULL,
				resource_kind TEXT,
				resource_id TEXT,
				reason TEXT NOT NULL,
				requested_by TEXT,
				status TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				resolved_at TEXT,
				resolution TEXT,
				comment TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS decisions (
				decision_id TEXT PRIMARY KEY,
				action TEXT NOT NULL,
				resource_kind TEXT,
				resource_id TEXT,
				outcome TEXT NOT NULL,
				reason TEXT NOT NULL,
				approval_id TEXT,
				created_at TEXT NOT NULL,
				FOREIGN KEY(approval_id) REFERENCES approvals(approval_id) ON DELETE SET NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS runs (
				run_id TEXT PRIMARY KEY,
				session_id TEXT,
				entrypoint TEXT NOT NULL,
				status TEXT NOT NULL,
				goal TEXT NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE SET NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS connectors (
				connector_id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				display_name TEXT NOT NULL,
				status TEXT NOT NULL,
				failure_count INTEGER NOT NULL,
				restart_count INTEGER NOT NULL,
				backoff_seconds INTEGER NOT NULL,
				next_restart_at TEXT,
				last_restart_at TEXT,
				last_heartbeat_at TEXT,
				last_failure_reason TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS capabilities (
				capability_id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				display_name TEXT NOT NULL,
				status TEXT NOT NULL,
				failure_count INTEGER NOT NULL,
				restart_count INTEGER NOT NULL,
				backoff_seconds INTEGER NOT NULL,
				next_restart_at TEXT,
				last_restart_at TEXT,
				last_heartbeat_at TEXT,
				last_failure_reason TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS steps (
				step_id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				title TEXT NOT NULL,
				kind TEXT NOT NULL,
				status TEXT NOT NULL,
				input_json TEXT,
				output_json TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS tool_calls (
				tool_call_id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				step_id TEXT NOT NULL,
				capability_id TEXT NOT NULL,
				tool_name TEXT NOT NULL,
				status TEXT NOT NULL,
				input_json TEXT,
				output_json TEXT,
				error_text TEXT,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE,
				FOREIGN KEY(step_id) REFERENCES steps(step_id) ON DELETE CASCADE
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS llm_dispatches (
				dispatch_id TEXT PRIMARY KEY,
				provider TEXT NOT NULL,
				model TEXT NOT NULL,
				messages_json TEXT NOT NULL,
				stream INTEGER NOT NULL,
				status TEXT NOT NULL,
				output_text TEXT NOT NULL,
				finish_reason TEXT,
				usage_json TEXT NOT NULL,
				error_code TEXT,
				error_text TEXT,
				timeout_ms INTEGER NOT NULL,
				max_retries INTEGER NOT NULL,
				attempt_count INTEGER NOT NULL,
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				started_at TEXT,
				completed_at TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS events (
				event_id TEXT PRIMARY KEY,
				category TEXT NOT NULL,
				name TEXT NOT NULL,
				occurred_at TEXT NOT NULL,
				session_id TEXT,
				run_id TEXT,
				step_id TEXT,
				connector_id TEXT,
				capability_id TEXT,
				resource_kind TEXT NOT NULL,
				resource_id TEXT NOT NULL,
				payload_json TEXT
			);
			`,
			`
			CREATE TABLE IF NOT EXISTS checkpoints (
				checkpoint_id TEXT PRIMARY KEY,
				run_id TEXT NOT NULL,
				captured_at TEXT NOT NULL,
				snapshot_json TEXT NOT NULL,
				FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
			);
			`,
			`CREATE INDEX IF NOT EXISTS idx_steps_run_id ON steps(run_id);`,
			`CREATE INDEX IF NOT EXISTS idx_connectors_kind_status ON connectors(kind, status);`,
			`CREATE INDEX IF NOT EXISTS idx_capabilities_kind_status ON capabilities(kind, status);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_run_step ON tool_calls(run_id, step_id, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_llm_dispatches_provider_status ON llm_dispatches(provider, status, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_approvals_status_created ON approvals(status, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_channel_peer ON sessions(channel, peer_id, thread_id);`,
			`CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_events_category ON events(category, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_checkpoints_run_id ON checkpoints(run_id, captured_at);`,
		},
	},
	{
		Version: 2,
		Name:    "operational_indexes",
		Statements: []string{
			`CREATE INDEX IF NOT EXISTS idx_events_resource_scope ON events(resource_kind, resource_id, occurred_at);`,
			`CREATE INDEX IF NOT EXISTS idx_tool_calls_capability_created ON tool_calls(capability_id, created_at);`,
			`CREATE INDEX IF NOT EXISTS idx_auth_tokens_last_used_at ON auth_tokens(last_used_at);`,
			`CREATE INDEX IF NOT EXISTS idx_decisions_approval_id ON decisions(approval_id, created_at);`,
		},
	},
}

type SQLiteStore struct {
	DataDir string
	DBPath  string
	db      *sql.DB
}

func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	resolvedDir, err := resolveDataDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(resolvedDir, defaultDatabaseFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	store := &SQLiteStore{
		DataDir: resolvedDir,
		DBPath:  dbPath,
		db:      db,
	}

	if err := store.configure(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) SchemaVersion(ctx context.Context) (int, error) {
	if s == nil {
		return 0, nil
	}

	version, err := currentSchemaVersion(ctx, s.db)
	if err != nil {
		return 0, fmt.Errorf("load schema version: %w", err)
	}
	return version, nil
}

func (s *SQLiteStore) UpsertRun(ctx context.Context, run runtime.Run) error {
	if s == nil {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (
			run_id,
			session_id,
			entrypoint,
			status,
			goal,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			session_id = excluded.session_id,
			entrypoint = excluded.entrypoint,
			status = excluded.status,
			goal = excluded.goal,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		run.RunID,
		nullString(run.SessionID),
		run.Entrypoint,
		string(run.Status),
		run.Goal,
		run.CreatedAt.UTC().Format(time.RFC3339Nano),
		run.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert run %s: %w", run.RunID, err)
	}

	return nil
}

func (s *SQLiteStore) UpsertSession(ctx context.Context, session router.Session) error {
	if s == nil {
		return nil
	}

	var lastResetAt sql.NullString
	if session.LastResetAt != nil {
		lastResetAt = sql.NullString{String: session.LastResetAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
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
			last_reset_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			last_reset_at = excluded.last_reset_at
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
	)
	if err != nil {
		return fmt.Errorf("upsert session %s: %w", session.SessionID, err)
	}

	return nil
}

func (s *SQLiteStore) UpsertPairing(ctx context.Context, pairing auth.Pairing) error {
	if s == nil {
		return nil
	}

	var completedAt sql.NullString
	if pairing.CompletedAt != nil {
		completedAt = sql.NullString{String: pairing.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_pairings (
			pairing_id,
			mode,
			label,
			status,
			code_hash,
			code_preview,
			created_at,
			updated_at,
			expires_at,
			completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pairing_id) DO UPDATE SET
			mode = excluded.mode,
			label = excluded.label,
			status = excluded.status,
			code_hash = excluded.code_hash,
			code_preview = excluded.code_preview,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			expires_at = excluded.expires_at,
			completed_at = excluded.completed_at
	`,
		pairing.PairingID,
		string(pairing.Mode),
		pairing.Label,
		string(pairing.Status),
		pairing.CodeHash,
		nullString(pairing.CodePreview),
		pairing.CreatedAt.UTC().Format(time.RFC3339Nano),
		pairing.UpdatedAt.UTC().Format(time.RFC3339Nano),
		pairing.ExpiresAt.UTC().Format(time.RFC3339Nano),
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert pairing %s: %w", pairing.PairingID, err)
	}

	return nil
}

func (s *SQLiteStore) ListPairings(ctx context.Context) ([]auth.Pairing, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT pairing_id, mode, label, status, code_hash, code_preview, created_at, updated_at, expires_at, completed_at
		FROM auth_pairings
		ORDER BY created_at ASC, pairing_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pairings: %w", err)
	}
	defer rows.Close()

	items := make([]auth.Pairing, 0)
	for rows.Next() {
		pairing, err := scanPairing(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, pairing)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertAccessToken(ctx context.Context, token auth.AccessToken) error {
	if s == nil {
		return nil
	}

	var lastUsedAt sql.NullString
	if token.LastUsedAt != nil {
		lastUsedAt = sql.NullString{String: token.LastUsedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_tokens (
			token_id,
			label,
			mode,
			token_hash,
			token_preview,
			created_at,
			updated_at,
			last_used_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(token_id) DO UPDATE SET
			label = excluded.label,
			mode = excluded.mode,
			token_hash = excluded.token_hash,
			token_preview = excluded.token_preview,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			last_used_at = excluded.last_used_at
	`,
		token.TokenID,
		token.Label,
		string(token.Mode),
		token.TokenHash,
		token.TokenPreview,
		token.CreatedAt.UTC().Format(time.RFC3339Nano),
		token.UpdatedAt.UTC().Format(time.RFC3339Nano),
		lastUsedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert access token %s: %w", token.TokenID, err)
	}

	return nil
}

func (s *SQLiteStore) ListAccessTokens(ctx context.Context) ([]auth.AccessToken, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT token_id, label, mode, token_hash, token_preview, created_at, updated_at, last_used_at
		FROM auth_tokens
		ORDER BY created_at ASC, token_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list access tokens: %w", err)
	}
	defer rows.Close()

	items := make([]auth.AccessToken, 0)
	for rows.Next() {
		token, err := scanAccessToken(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, token)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertApproval(ctx context.Context, approval policy.Approval) error {
	if s == nil {
		return nil
	}

	var resolvedAt sql.NullString
	if approval.ResolvedAt != nil {
		resolvedAt = sql.NullString{String: approval.ResolvedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO approvals (
			approval_id,
			action,
			resource_kind,
			resource_id,
			reason,
			requested_by,
			status,
			created_at,
			updated_at,
			resolved_at,
			resolution,
			comment
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(approval_id) DO UPDATE SET
			action = excluded.action,
			resource_kind = excluded.resource_kind,
			resource_id = excluded.resource_id,
			reason = excluded.reason,
			requested_by = excluded.requested_by,
			status = excluded.status,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			resolved_at = excluded.resolved_at,
			resolution = excluded.resolution,
			comment = excluded.comment
	`,
		approval.ApprovalID,
		approval.Action,
		nullString(approval.ResourceKind),
		nullString(approval.ResourceID),
		approval.Reason,
		nullString(approval.RequestedBy),
		string(approval.Status),
		approval.CreatedAt.UTC().Format(time.RFC3339Nano),
		approval.UpdatedAt.UTC().Format(time.RFC3339Nano),
		resolvedAt,
		nullString(approval.Resolution),
		nullString(approval.Comment),
	)
	if err != nil {
		return fmt.Errorf("upsert approval %s: %w", approval.ApprovalID, err)
	}

	return nil
}

func (s *SQLiteStore) ListApprovals(ctx context.Context) ([]policy.Approval, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT approval_id, action, resource_kind, resource_id, reason, requested_by, status, created_at, updated_at, resolved_at, resolution, comment
		FROM approvals
		ORDER BY created_at ASC, approval_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list approvals: %w", err)
	}
	defer rows.Close()

	items := make([]policy.Approval, 0)
	for rows.Next() {
		approval, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, approval)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertDecision(ctx context.Context, decision policy.Decision) error {
	if s == nil {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO decisions (
			decision_id,
			action,
			resource_kind,
			resource_id,
			outcome,
			reason,
			approval_id,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(decision_id) DO UPDATE SET
			action = excluded.action,
			resource_kind = excluded.resource_kind,
			resource_id = excluded.resource_id,
			outcome = excluded.outcome,
			reason = excluded.reason,
			approval_id = excluded.approval_id,
			created_at = excluded.created_at
	`,
		decision.DecisionID,
		decision.Action,
		nullString(decision.ResourceKind),
		nullString(decision.ResourceID),
		string(decision.Outcome),
		decision.Reason,
		nullString(decision.ApprovalID),
		decision.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert decision %s: %w", decision.DecisionID, err)
	}

	return nil
}

func (s *SQLiteStore) ListDecisions(ctx context.Context) ([]policy.Decision, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT decision_id, action, resource_kind, resource_id, outcome, reason, approval_id, created_at
		FROM decisions
		ORDER BY created_at ASC, decision_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	defer rows.Close()

	items := make([]policy.Decision, 0)
	for rows.Next() {
		decision, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, decision)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertConnector(ctx context.Context, connector connectors.Connector) error {
	if s == nil {
		return nil
	}

	var nextRestartAt sql.NullString
	if connector.NextRestartAt != nil {
		nextRestartAt = sql.NullString{String: connector.NextRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastRestartAt sql.NullString
	if connector.LastRestartAt != nil {
		lastRestartAt = sql.NullString{String: connector.LastRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastHeartbeatAt sql.NullString
	if connector.LastHeartbeatAt != nil {
		lastHeartbeatAt = sql.NullString{String: connector.LastHeartbeatAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO connectors (
			connector_id,
			kind,
			display_name,
			status,
			failure_count,
			restart_count,
			backoff_seconds,
			next_restart_at,
			last_restart_at,
			last_heartbeat_at,
			last_failure_reason,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(connector_id) DO UPDATE SET
			kind = excluded.kind,
			display_name = excluded.display_name,
			status = excluded.status,
			failure_count = excluded.failure_count,
			restart_count = excluded.restart_count,
			backoff_seconds = excluded.backoff_seconds,
			next_restart_at = excluded.next_restart_at,
			last_restart_at = excluded.last_restart_at,
			last_heartbeat_at = excluded.last_heartbeat_at,
			last_failure_reason = excluded.last_failure_reason,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		connector.ConnectorID,
		connector.Kind,
		connector.DisplayName,
		string(connector.Status),
		connector.FailureCount,
		connector.RestartCount,
		connector.BackoffSeconds,
		nextRestartAt,
		lastRestartAt,
		lastHeartbeatAt,
		nullString(connector.LastFailureReason),
		connector.CreatedAt.UTC().Format(time.RFC3339Nano),
		connector.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert connector %s: %w", connector.ConnectorID, err)
	}

	return nil
}

func (s *SQLiteStore) ListConnectors(ctx context.Context) ([]connectors.Connector, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT connector_id, kind, display_name, status, failure_count, restart_count, backoff_seconds, next_restart_at, last_restart_at, last_heartbeat_at, last_failure_reason, created_at, updated_at
		FROM connectors
		ORDER BY created_at ASC, connector_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}
	defer rows.Close()

	items := make([]connectors.Connector, 0)
	for rows.Next() {
		connector, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, connector)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) UpsertCapability(ctx context.Context, capability capabilities.Capability) error {
	if s == nil {
		return nil
	}

	var nextRestartAt sql.NullString
	if capability.NextRestartAt != nil {
		nextRestartAt = sql.NullString{String: capability.NextRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastRestartAt sql.NullString
	if capability.LastRestartAt != nil {
		lastRestartAt = sql.NullString{String: capability.LastRestartAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var lastHeartbeatAt sql.NullString
	if capability.LastHeartbeatAt != nil {
		lastHeartbeatAt = sql.NullString{String: capability.LastHeartbeatAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO capabilities (
			capability_id,
			kind,
			display_name,
			status,
			failure_count,
			restart_count,
			backoff_seconds,
			next_restart_at,
			last_restart_at,
			last_heartbeat_at,
			last_failure_reason,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(capability_id) DO UPDATE SET
			kind = excluded.kind,
			display_name = excluded.display_name,
			status = excluded.status,
			failure_count = excluded.failure_count,
			restart_count = excluded.restart_count,
			backoff_seconds = excluded.backoff_seconds,
			next_restart_at = excluded.next_restart_at,
			last_restart_at = excluded.last_restart_at,
			last_heartbeat_at = excluded.last_heartbeat_at,
			last_failure_reason = excluded.last_failure_reason,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		capability.CapabilityID,
		capability.Kind,
		capability.DisplayName,
		string(capability.Status),
		capability.FailureCount,
		capability.RestartCount,
		capability.BackoffSeconds,
		nextRestartAt,
		lastRestartAt,
		lastHeartbeatAt,
		nullString(capability.LastFailureReason),
		capability.CreatedAt.UTC().Format(time.RFC3339Nano),
		capability.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert capability %s: %w", capability.CapabilityID, err)
	}

	return nil
}

func (s *SQLiteStore) UpsertLLMDispatch(ctx context.Context, dispatch llm.Dispatch) error {
	if s == nil {
		return nil
	}

	messagesJSON, err := marshalJSON(dispatch.Messages)
	if err != nil {
		return fmt.Errorf("marshal llm dispatch messages: %w", err)
	}
	usageJSON, err := marshalJSON(dispatch.Usage)
	if err != nil {
		return fmt.Errorf("marshal llm dispatch usage: %w", err)
	}

	var startedAt sql.NullString
	if dispatch.StartedAt != nil {
		startedAt = sql.NullString{String: dispatch.StartedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}
	var completedAt sql.NullString
	if dispatch.CompletedAt != nil {
		completedAt = sql.NullString{String: dispatch.CompletedAt.UTC().Format(time.RFC3339Nano), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
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
			completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			completed_at = excluded.completed_at
	`,
		dispatch.DispatchID,
		dispatch.Provider,
		dispatch.Model,
		messagesJSON,
		dispatch.Stream,
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
	)
	if err != nil {
		return fmt.Errorf("upsert llm dispatch %s: %w", dispatch.DispatchID, err)
	}

	return nil
}

func (s *SQLiteStore) ListLLMDispatches(ctx context.Context) ([]llm.Dispatch, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT dispatch_id, provider, model, messages_json, stream, status, output_text, finish_reason, usage_json, error_code, error_text, timeout_ms, max_retries, attempt_count, created_at, updated_at, started_at, completed_at
		FROM llm_dispatches
		ORDER BY created_at ASC, dispatch_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list llm dispatches: %w", err)
	}
	defer rows.Close()

	items := make([]llm.Dispatch, 0)
	for rows.Next() {
		dispatch, err := scanLLMDispatch(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, dispatch)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) GetLLMDispatch(ctx context.Context, dispatchID string) (llm.Dispatch, bool, error) {
	if s == nil {
		return llm.Dispatch{}, false, nil
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT dispatch_id, provider, model, messages_json, stream, status, output_text, finish_reason, usage_json, error_code, error_text, timeout_ms, max_retries, attempt_count, created_at, updated_at, started_at, completed_at
		FROM llm_dispatches
		WHERE dispatch_id = ?
	`, dispatchID)

	dispatch, err := scanLLMDispatch(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return llm.Dispatch{}, false, nil
		}
		return llm.Dispatch{}, false, err
	}

	return dispatch, true, nil
}

func (s *SQLiteStore) ListCapabilities(ctx context.Context) ([]capabilities.Capability, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT capability_id, kind, display_name, status, failure_count, restart_count, backoff_seconds, next_restart_at, last_restart_at, last_heartbeat_at, last_failure_reason, created_at, updated_at
		FROM capabilities
		ORDER BY created_at ASC, capability_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list capabilities: %w", err)
	}
	defer rows.Close()

	items := make([]capabilities.Capability, 0)
	for rows.Next() {
		capability, err := scanCapability(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, capability)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) ListSessions(ctx context.Context) ([]router.Session, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, kind, status, channel, account_id, peer_id, thread_id, routing_key, generation, created_at, updated_at, last_active_at, last_reset_at
		FROM sessions
		ORDER BY created_at ASC, session_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]router.Session, 0)
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

func (s *SQLiteStore) ListRuns(ctx context.Context) ([]runtime.Run, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, session_id, entrypoint, status, goal, created_at, updated_at
		FROM runs
		ORDER BY created_at ASC, run_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	runs := make([]runtime.Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, rows.Err()
}

func (s *SQLiteStore) UpsertStep(ctx context.Context, step runtime.Step) error {
	if s == nil {
		return nil
	}

	inputJSON, err := marshalJSON(step.Input)
	if err != nil {
		return fmt.Errorf("marshal step input: %w", err)
	}
	outputJSON, err := marshalJSON(step.Output)
	if err != nil {
		return fmt.Errorf("marshal step output: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO steps (
			step_id,
			run_id,
			title,
			kind,
			status,
			input_json,
			output_json,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(step_id) DO UPDATE SET
			run_id = excluded.run_id,
			title = excluded.title,
			kind = excluded.kind,
			status = excluded.status,
			input_json = excluded.input_json,
			output_json = excluded.output_json,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		step.StepID,
		step.RunID,
		step.Title,
		step.Kind,
		string(step.Status),
		inputJSON,
		outputJSON,
		step.CreatedAt.UTC().Format(time.RFC3339Nano),
		step.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert step %s: %w", step.StepID, err)
	}

	return nil
}

func (s *SQLiteStore) UpsertToolCall(ctx context.Context, toolCall runtime.ToolCall) error {
	if s == nil {
		return nil
	}

	inputJSON, err := marshalJSON(toolCall.Input)
	if err != nil {
		return fmt.Errorf("marshal tool call input: %w", err)
	}
	outputJSON, err := marshalJSON(toolCall.Output)
	if err != nil {
		return fmt.Errorf("marshal tool call output: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tool_calls (
			tool_call_id,
			run_id,
			step_id,
			capability_id,
			tool_name,
			status,
			input_json,
			output_json,
			error_text,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tool_call_id) DO UPDATE SET
			run_id = excluded.run_id,
			step_id = excluded.step_id,
			capability_id = excluded.capability_id,
			tool_name = excluded.tool_name,
			status = excluded.status,
			input_json = excluded.input_json,
			output_json = excluded.output_json,
			error_text = excluded.error_text,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		toolCall.ToolCallID,
		toolCall.RunID,
		toolCall.StepID,
		toolCall.CapabilityID,
		toolCall.ToolName,
		string(toolCall.Status),
		inputJSON,
		outputJSON,
		nullString(toolCall.Error),
		toolCall.CreatedAt.UTC().Format(time.RFC3339Nano),
		toolCall.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert tool call %s: %w", toolCall.ToolCallID, err)
	}

	return nil
}

func (s *SQLiteStore) ListToolCalls(ctx context.Context, runID, stepID string) ([]runtime.ToolCall, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT tool_call_id, run_id, step_id, capability_id, tool_name, status, input_json, output_json, error_text, created_at, updated_at
		FROM tool_calls
		WHERE run_id = ? AND step_id = ?
		ORDER BY created_at ASC, tool_call_id ASC
	`, runID, stepID)
	if err != nil {
		return nil, fmt.Errorf("list tool calls for run %s step %s: %w", runID, stepID, err)
	}
	defer rows.Close()

	items := make([]runtime.ToolCall, 0)
	for rows.Next() {
		toolCall, err := scanToolCall(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, toolCall)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) ListSteps(ctx context.Context, runID string) ([]runtime.Step, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT step_id, run_id, title, kind, status, input_json, output_json, created_at, updated_at
		FROM steps
		WHERE run_id = ?
		ORDER BY created_at ASC, step_id ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("list steps for run %s: %w", runID, err)
	}
	defer rows.Close()

	steps := make([]runtime.Step, 0)
	for rows.Next() {
		step, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}

	return steps, rows.Err()
}

func (s *SQLiteStore) AppendEvent(ctx context.Context, event events.Event) (events.Event, error) {
	if s == nil {
		return event, nil
	}

	payloadJSON, err := marshalJSON(event.Payload)
	if err != nil {
		return events.Event{}, fmt.Errorf("marshal event payload: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO events (
			event_id,
			category,
			name,
			occurred_at,
			session_id,
			run_id,
			step_id,
			connector_id,
			capability_id,
			resource_kind,
			resource_id,
			payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id) DO NOTHING
	`,
		event.EventID,
		event.Category,
		event.Name,
		event.OccurredAt.UTC().Format(time.RFC3339Nano),
		nullString(event.Scope.SessionID),
		nullString(event.Scope.RunID),
		nullString(event.Scope.StepID),
		nullString(event.Scope.ConnectorID),
		nullString(event.Scope.CapabilityID),
		event.Resource.Kind,
		event.Resource.ID,
		payloadJSON,
	)
	if err != nil {
		return events.Event{}, fmt.Errorf("append event %s: %w", event.EventID, err)
	}

	if err := s.db.QueryRowContext(ctx, `SELECT rowid FROM events WHERE event_id = ?`, event.EventID).Scan(&event.Sequence); err != nil {
		return events.Event{}, fmt.Errorf("load event sequence %s: %w", event.EventID, err)
	}

	return event, nil
}

func (s *SQLiteStore) ListEvents(ctx context.Context, filter events.Filter) ([]events.Event, error) {
	if s == nil {
		return nil, nil
	}

	query := `
		SELECT rowid, event_id, category, name, occurred_at, session_id, run_id, step_id, connector_id, capability_id, resource_kind, resource_id, payload_json
		FROM events
		WHERE 1 = 1
	`
	args := make([]any, 0, 4)

	if filter.Category != "" {
		query += ` AND category = ?`
		args = append(args, filter.Category)
	}
	if filter.RunID != "" {
		query += ` AND run_id = ?`
		args = append(args, filter.RunID)
	}
	if filter.SessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, filter.SessionID)
	}
	if filter.ResourceKind != "" {
		query += ` AND resource_kind = ?`
		args = append(args, filter.ResourceKind)
	}
	if filter.Cursor > 0 {
		query += ` AND rowid > ?`
		args = append(args, filter.Cursor)
	}

	query += ` ORDER BY rowid ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	items := make([]events.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, event)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) SaveCheckpoint(ctx context.Context, checkpoint runtime.RunCheckpoint) error {
	if s == nil {
		return nil
	}

	snapshotJSON, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO checkpoints (
			checkpoint_id,
			run_id,
			captured_at,
			snapshot_json
		) VALUES (?, ?, ?, ?)
	`,
		newCheckpointID(),
		checkpoint.Run.RunID,
		checkpoint.CapturedAt.UTC().Format(time.RFC3339Nano),
		string(snapshotJSON),
	)
	if err != nil {
		return fmt.Errorf("save checkpoint for run %s: %w", checkpoint.Run.RunID, err)
	}

	return nil
}

func (s *SQLiteStore) ListLatestCheckpoints(ctx context.Context) ([]runtime.RunCheckpoint, error) {
	if s == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT checkpoint_id, run_id, captured_at, snapshot_json
		FROM checkpoints
		WHERE (run_id, captured_at) IN (
			SELECT run_id, MAX(captured_at)
			FROM checkpoints
			GROUP BY run_id
		)
		ORDER BY captured_at ASC, run_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list latest checkpoints: %w", err)
	}
	defer rows.Close()

	items := make([]runtime.RunCheckpoint, 0)
	for rows.Next() {
		checkpoint, err := scanCheckpoint(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, checkpoint)
	}

	return items, rows.Err()
}

func (s *SQLiteStore) configure(ctx context.Context) error {
	pragmas := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
	}
	for _, stmt := range pragmas {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply pragma %q: %w", stmt, err)
		}
	}
	return nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	currentVersion, err := currentSchemaVersion(ctx, tx)
	if err != nil {
		return fmt.Errorf("load current schema version: %w", err)
	}

	if currentVersion == 0 {
		legacy, err := hasLegacySchema(ctx, tx)
		if err != nil {
			return fmt.Errorf("detect legacy schema: %w", err)
		}
		if legacy {
			if err := recordSchemaMigration(ctx, tx, schemaMigrations[0].Version, schemaMigrations[0].Name+"_legacy_bootstrap"); err != nil {
				return fmt.Errorf("record legacy schema bootstrap: %w", err)
			}
			currentVersion = schemaMigrations[0].Version
		}
	}

	if currentVersion > CurrentSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", currentVersion, CurrentSchemaVersion)
	}

	for _, migration := range schemaMigrations {
		if migration.Version <= currentVersion {
			continue
		}
		for _, stmt := range migration.Statements {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("apply schema migration %d (%s): %w", migration.Version, migration.Name, err)
			}
		}
		if err := recordSchemaMigration(ctx, tx, migration.Version, migration.Name); err != nil {
			return fmt.Errorf("record schema migration %d (%s): %w", migration.Version, migration.Name, err)
		}
		currentVersion = migration.Version
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration transaction: %w", err)
	}
	return nil
}

func currentSchemaVersion(ctx context.Context, queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (int, error) {
	var version sql.NullInt64
	if err := queryer.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

func hasLegacySchema(ctx context.Context, queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}) (bool, error) {
	var count int
	if err := queryer.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM sqlite_master
		WHERE type = 'table'
		  AND name IN ('sessions', 'runs', 'steps', 'events', 'checkpoints', 'tool_calls', 'llm_dispatches', 'connectors', 'capabilities', 'auth_pairings', 'auth_tokens', 'approvals', 'decisions')
	`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func recordSchemaMigration(ctx context.Context, execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}, version int, name string) error {
	_, err := execer.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name, applied_at)
		VALUES (?, ?, ?)
		ON CONFLICT(version) DO UPDATE SET
			name = excluded.name,
			applied_at = excluded.applied_at
	`, version, name, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func resolveDataDir(dataDir string) (string, error) {
	if dataDir == "" {
		return "", errors.New("data dir is required")
	}
	if dataDir == "~" || strings.HasPrefix(dataDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		if dataDir == "~" {
			return homeDir, nil
		}
		return filepath.Join(homeDir, strings.TrimPrefix(dataDir, "~/")), nil
	}
	return dataDir, nil
}

func scanRun(scanner interface {
	Scan(dest ...any) error
}) (runtime.Run, error) {
	var (
		run       runtime.Run
		status    string
		sessionID sql.NullString
		createdAt string
		updatedAt string
	)

	if err := scanner.Scan(
		&run.RunID,
		&sessionID,
		&run.Entrypoint,
		&status,
		&run.Goal,
		&createdAt,
		&updatedAt,
	); err != nil {
		return runtime.Run{}, fmt.Errorf("scan run: %w", err)
	}

	run.SessionID = sessionID.String
	run.Status = runtime.RunStatus(status)

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return runtime.Run{}, fmt.Errorf("parse run created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return runtime.Run{}, fmt.Errorf("parse run updated_at: %w", err)
	}

	run.CreatedAt = parsedCreatedAt
	run.UpdatedAt = parsedUpdatedAt

	return run, nil
}

func scanSession(scanner interface {
	Scan(dest ...any) error
}) (router.Session, error) {
	var (
		session      router.Session
		kind         string
		status       string
		accountID    sql.NullString
		threadID     sql.NullString
		createdAt    string
		updatedAt    string
		lastActiveAt string
		lastResetAt  sql.NullString
	)

	if err := scanner.Scan(
		&session.SessionID,
		&kind,
		&status,
		&session.Channel,
		&accountID,
		&session.PeerID,
		&threadID,
		&session.RoutingKey,
		&session.Generation,
		&createdAt,
		&updatedAt,
		&lastActiveAt,
		&lastResetAt,
	); err != nil {
		return router.Session{}, fmt.Errorf("scan session: %w", err)
	}

	session.Kind = router.SessionKind(kind)
	session.Status = router.SessionStatus(status)
	session.AccountID = accountID.String
	session.ThreadID = threadID.String

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return router.Session{}, fmt.Errorf("parse session created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return router.Session{}, fmt.Errorf("parse session updated_at: %w", err)
	}
	parsedLastActiveAt, err := time.Parse(time.RFC3339Nano, lastActiveAt)
	if err != nil {
		return router.Session{}, fmt.Errorf("parse session last_active_at: %w", err)
	}

	session.CreatedAt = parsedCreatedAt
	session.UpdatedAt = parsedUpdatedAt
	session.LastActiveAt = parsedLastActiveAt

	if lastResetAt.Valid {
		parsedLastResetAt, err := time.Parse(time.RFC3339Nano, lastResetAt.String)
		if err != nil {
			return router.Session{}, fmt.Errorf("parse session last_reset_at: %w", err)
		}
		session.LastResetAt = &parsedLastResetAt
	}

	return session, nil
}

func scanPairing(scanner interface {
	Scan(dest ...any) error
}) (auth.Pairing, error) {
	var (
		pairing     auth.Pairing
		mode        string
		status      string
		codePreview sql.NullString
		createdAt   string
		updatedAt   string
		expiresAt   string
		completedAt sql.NullString
	)

	if err := scanner.Scan(
		&pairing.PairingID,
		&mode,
		&pairing.Label,
		&status,
		&pairing.CodeHash,
		&codePreview,
		&createdAt,
		&updatedAt,
		&expiresAt,
		&completedAt,
	); err != nil {
		return auth.Pairing{}, fmt.Errorf("scan pairing: %w", err)
	}

	pairing.Mode = auth.PairingMode(mode)
	pairing.Status = auth.PairingStatus(status)
	pairing.CodePreview = codePreview.String
	if err := assignRequiredTime(&pairing.CreatedAt, createdAt); err != nil {
		return auth.Pairing{}, err
	}
	if err := assignRequiredTime(&pairing.UpdatedAt, updatedAt); err != nil {
		return auth.Pairing{}, err
	}
	if err := assignRequiredTime(&pairing.ExpiresAt, expiresAt); err != nil {
		return auth.Pairing{}, err
	}
	if err := assignOptionalTime(&pairing.CompletedAt, completedAt); err != nil {
		return auth.Pairing{}, err
	}

	return pairing, nil
}

func scanAccessToken(scanner interface {
	Scan(dest ...any) error
}) (auth.AccessToken, error) {
	var (
		token      auth.AccessToken
		mode       string
		createdAt  string
		updatedAt  string
		lastUsedAt sql.NullString
	)

	if err := scanner.Scan(
		&token.TokenID,
		&token.Label,
		&mode,
		&token.TokenHash,
		&token.TokenPreview,
		&createdAt,
		&updatedAt,
		&lastUsedAt,
	); err != nil {
		return auth.AccessToken{}, fmt.Errorf("scan access token: %w", err)
	}

	token.Mode = auth.PairingMode(mode)
	if err := assignRequiredTime(&token.CreatedAt, createdAt); err != nil {
		return auth.AccessToken{}, err
	}
	if err := assignRequiredTime(&token.UpdatedAt, updatedAt); err != nil {
		return auth.AccessToken{}, err
	}
	if err := assignOptionalTime(&token.LastUsedAt, lastUsedAt); err != nil {
		return auth.AccessToken{}, err
	}

	return token, nil
}

func scanApproval(scanner interface {
	Scan(dest ...any) error
}) (policy.Approval, error) {
	var (
		approval     policy.Approval
		resourceKind sql.NullString
		resourceID   sql.NullString
		requestedBy  sql.NullString
		status       string
		createdAt    string
		updatedAt    string
		resolvedAt   sql.NullString
		resolution   sql.NullString
		comment      sql.NullString
	)

	if err := scanner.Scan(
		&approval.ApprovalID,
		&approval.Action,
		&resourceKind,
		&resourceID,
		&approval.Reason,
		&requestedBy,
		&status,
		&createdAt,
		&updatedAt,
		&resolvedAt,
		&resolution,
		&comment,
	); err != nil {
		return policy.Approval{}, fmt.Errorf("scan approval: %w", err)
	}

	approval.ResourceKind = resourceKind.String
	approval.ResourceID = resourceID.String
	approval.RequestedBy = requestedBy.String
	approval.Status = policy.ApprovalStatus(status)
	approval.Resolution = resolution.String
	approval.Comment = comment.String
	if err := assignRequiredTime(&approval.CreatedAt, createdAt); err != nil {
		return policy.Approval{}, err
	}
	if err := assignRequiredTime(&approval.UpdatedAt, updatedAt); err != nil {
		return policy.Approval{}, err
	}
	if err := assignOptionalTime(&approval.ResolvedAt, resolvedAt); err != nil {
		return policy.Approval{}, err
	}

	return approval, nil
}

func scanDecision(scanner interface {
	Scan(dest ...any) error
}) (policy.Decision, error) {
	var (
		decision     policy.Decision
		resourceKind sql.NullString
		resourceID   sql.NullString
		outcome      string
		approvalID   sql.NullString
		createdAt    string
	)

	if err := scanner.Scan(
		&decision.DecisionID,
		&decision.Action,
		&resourceKind,
		&resourceID,
		&outcome,
		&decision.Reason,
		&approvalID,
		&createdAt,
	); err != nil {
		return policy.Decision{}, fmt.Errorf("scan decision: %w", err)
	}

	decision.ResourceKind = resourceKind.String
	decision.ResourceID = resourceID.String
	decision.Outcome = policy.DecisionOutcome(outcome)
	decision.ApprovalID = approvalID.String
	if err := assignRequiredTime(&decision.CreatedAt, createdAt); err != nil {
		return policy.Decision{}, err
	}

	return decision, nil
}

func scanConnector(scanner interface {
	Scan(dest ...any) error
}) (connectors.Connector, error) {
	var (
		item            connectors.Connector
		status          string
		nextRestartAt   sql.NullString
		lastRestartAt   sql.NullString
		lastHeartbeatAt sql.NullString
		lastFailure     sql.NullString
		createdAt       string
		updatedAt       string
	)

	if err := scanner.Scan(
		&item.ConnectorID,
		&item.Kind,
		&item.DisplayName,
		&status,
		&item.FailureCount,
		&item.RestartCount,
		&item.BackoffSeconds,
		&nextRestartAt,
		&lastRestartAt,
		&lastHeartbeatAt,
		&lastFailure,
		&createdAt,
		&updatedAt,
	); err != nil {
		return connectors.Connector{}, fmt.Errorf("scan connector: %w", err)
	}

	item.Status = connectors.Status(status)
	item.LastFailureReason = lastFailure.String

	if err := assignOptionalTime(&item.NextRestartAt, nextRestartAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector next_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastRestartAt, lastRestartAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector last_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastHeartbeatAt, lastHeartbeatAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector last_heartbeat_at: %w", err)
	}
	if err := assignRequiredTime(&item.CreatedAt, createdAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector created_at: %w", err)
	}
	if err := assignRequiredTime(&item.UpdatedAt, updatedAt); err != nil {
		return connectors.Connector{}, fmt.Errorf("parse connector updated_at: %w", err)
	}

	return item, nil
}

func scanCapability(scanner interface {
	Scan(dest ...any) error
}) (capabilities.Capability, error) {
	var (
		item            capabilities.Capability
		status          string
		nextRestartAt   sql.NullString
		lastRestartAt   sql.NullString
		lastHeartbeatAt sql.NullString
		lastFailure     sql.NullString
		createdAt       string
		updatedAt       string
	)

	if err := scanner.Scan(
		&item.CapabilityID,
		&item.Kind,
		&item.DisplayName,
		&status,
		&item.FailureCount,
		&item.RestartCount,
		&item.BackoffSeconds,
		&nextRestartAt,
		&lastRestartAt,
		&lastHeartbeatAt,
		&lastFailure,
		&createdAt,
		&updatedAt,
	); err != nil {
		return capabilities.Capability{}, fmt.Errorf("scan capability: %w", err)
	}

	item.Status = capabilities.Status(status)
	item.LastFailureReason = lastFailure.String

	if err := assignOptionalTime(&item.NextRestartAt, nextRestartAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability next_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastRestartAt, lastRestartAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability last_restart_at: %w", err)
	}
	if err := assignOptionalTime(&item.LastHeartbeatAt, lastHeartbeatAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability last_heartbeat_at: %w", err)
	}
	if err := assignRequiredTime(&item.CreatedAt, createdAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability created_at: %w", err)
	}
	if err := assignRequiredTime(&item.UpdatedAt, updatedAt); err != nil {
		return capabilities.Capability{}, fmt.Errorf("parse capability updated_at: %w", err)
	}

	return item, nil
}

func scanLLMDispatch(scanner interface {
	Scan(dest ...any) error
}) (llm.Dispatch, error) {
	var (
		dispatch     llm.Dispatch
		messagesRaw  string
		stream       bool
		status       string
		finishReason sql.NullString
		usageRaw     string
		errorCode    sql.NullString
		errorText    sql.NullString
		createdAt    string
		updatedAt    string
		startedAt    sql.NullString
		completedAt  sql.NullString
	)

	if err := scanner.Scan(
		&dispatch.DispatchID,
		&dispatch.Provider,
		&dispatch.Model,
		&messagesRaw,
		&stream,
		&status,
		&dispatch.Output,
		&finishReason,
		&usageRaw,
		&errorCode,
		&errorText,
		&dispatch.TimeoutMs,
		&dispatch.MaxRetries,
		&dispatch.AttemptCount,
		&createdAt,
		&updatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return llm.Dispatch{}, err
	}

	dispatch.Stream = stream
	dispatch.Status = llm.DispatchStatus(status)
	dispatch.FinishReason = finishReason.String
	dispatch.ErrorCode = errorCode.String
	dispatch.Error = errorText.String

	if err := json.Unmarshal([]byte(messagesRaw), &dispatch.Messages); err != nil {
		return llm.Dispatch{}, fmt.Errorf("decode llm dispatch messages: %w", err)
	}
	if err := json.Unmarshal([]byte(usageRaw), &dispatch.Usage); err != nil {
		return llm.Dispatch{}, fmt.Errorf("decode llm dispatch usage: %w", err)
	}
	if err := assignRequiredTime(&dispatch.CreatedAt, createdAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch created_at: %w", err)
	}
	if err := assignRequiredTime(&dispatch.UpdatedAt, updatedAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch updated_at: %w", err)
	}
	if err := assignOptionalTime(&dispatch.StartedAt, startedAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch started_at: %w", err)
	}
	if err := assignOptionalTime(&dispatch.CompletedAt, completedAt); err != nil {
		return llm.Dispatch{}, fmt.Errorf("parse llm dispatch completed_at: %w", err)
	}

	return dispatch, nil
}

func scanStep(scanner interface {
	Scan(dest ...any) error
}) (runtime.Step, error) {
	var (
		step       runtime.Step
		status     string
		inputJSON  sql.NullString
		outputJSON sql.NullString
		createdAt  string
		updatedAt  string
	)

	if err := scanner.Scan(
		&step.StepID,
		&step.RunID,
		&step.Title,
		&step.Kind,
		&status,
		&inputJSON,
		&outputJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return runtime.Step{}, fmt.Errorf("scan step: %w", err)
	}

	step.Status = runtime.StepStatus(status)

	if err := unmarshalNullableJSON(inputJSON, &step.Input); err != nil {
		return runtime.Step{}, fmt.Errorf("decode step input: %w", err)
	}
	if err := unmarshalNullableJSON(outputJSON, &step.Output); err != nil {
		return runtime.Step{}, fmt.Errorf("decode step output: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return runtime.Step{}, fmt.Errorf("parse step created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return runtime.Step{}, fmt.Errorf("parse step updated_at: %w", err)
	}

	step.CreatedAt = parsedCreatedAt
	step.UpdatedAt = parsedUpdatedAt

	return step, nil
}

func scanToolCall(scanner interface {
	Scan(dest ...any) error
}) (runtime.ToolCall, error) {
	var (
		toolCall   runtime.ToolCall
		status     string
		inputJSON  sql.NullString
		outputJSON sql.NullString
		errorText  sql.NullString
		createdAt  string
		updatedAt  string
	)

	if err := scanner.Scan(
		&toolCall.ToolCallID,
		&toolCall.RunID,
		&toolCall.StepID,
		&toolCall.CapabilityID,
		&toolCall.ToolName,
		&status,
		&inputJSON,
		&outputJSON,
		&errorText,
		&createdAt,
		&updatedAt,
	); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("scan tool call: %w", err)
	}

	toolCall.Status = runtime.ToolCallStatus(status)
	toolCall.Error = errorText.String

	if err := unmarshalNullableJSON(inputJSON, &toolCall.Input); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("decode tool call input: %w", err)
	}
	if err := unmarshalNullableJSON(outputJSON, &toolCall.Output); err != nil {
		return runtime.ToolCall{}, fmt.Errorf("decode tool call output: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return runtime.ToolCall{}, fmt.Errorf("parse tool call created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return runtime.ToolCall{}, fmt.Errorf("parse tool call updated_at: %w", err)
	}

	toolCall.CreatedAt = parsedCreatedAt
	toolCall.UpdatedAt = parsedUpdatedAt

	return toolCall, nil
}

func assignRequiredTime(target *time.Time, value string) error {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return err
	}
	*target = parsed
	return nil
}

func assignOptionalTime(target **time.Time, value sql.NullString) error {
	if !value.Valid || value.String == "" {
		*target = nil
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return err
	}
	*target = &parsed
	return nil
}

func scanEvent(scanner interface {
	Scan(dest ...any) error
}) (events.Event, error) {
	var (
		event        events.Event
		sequence     int64
		occurredAt   string
		sessionID    sql.NullString
		runID        sql.NullString
		stepID       sql.NullString
		connectorID  sql.NullString
		capabilityID sql.NullString
		payloadJSON  sql.NullString
	)

	if err := scanner.Scan(
		&sequence,
		&event.EventID,
		&event.Category,
		&event.Name,
		&occurredAt,
		&sessionID,
		&runID,
		&stepID,
		&connectorID,
		&capabilityID,
		&event.Resource.Kind,
		&event.Resource.ID,
		&payloadJSON,
	); err != nil {
		return events.Event{}, fmt.Errorf("scan event: %w", err)
	}

	parsedOccurredAt, err := time.Parse(time.RFC3339Nano, occurredAt)
	if err != nil {
		return events.Event{}, fmt.Errorf("parse event occurred_at: %w", err)
	}

	event.Sequence = sequence
	event.OccurredAt = parsedOccurredAt
	event.Scope = events.Scope{
		SessionID:    sessionID.String,
		RunID:        runID.String,
		StepID:       stepID.String,
		ConnectorID:  connectorID.String,
		CapabilityID: capabilityID.String,
	}
	if err := unmarshalNullableJSON(payloadJSON, &event.Payload); err != nil {
		return events.Event{}, fmt.Errorf("decode event payload: %w", err)
	}
	if event.Payload == nil {
		event.Payload = map[string]any{}
	}

	return event, nil
}

func scanCheckpoint(scanner interface {
	Scan(dest ...any) error
}) (runtime.RunCheckpoint, error) {
	var (
		checkpointID string
		runID        string
		capturedAt   string
		snapshotJSON string
		checkpoint   runtime.RunCheckpoint
	)

	if err := scanner.Scan(&checkpointID, &runID, &capturedAt, &snapshotJSON); err != nil {
		return runtime.RunCheckpoint{}, fmt.Errorf("scan checkpoint: %w", err)
	}

	if err := json.Unmarshal([]byte(snapshotJSON), &checkpoint); err != nil {
		return runtime.RunCheckpoint{}, fmt.Errorf("decode checkpoint snapshot: %w", err)
	}
	if checkpoint.Run.RunID != runID {
		return runtime.RunCheckpoint{}, fmt.Errorf("checkpoint run mismatch: row=%s snapshot=%s", runID, checkpoint.Run.RunID)
	}

	parsedCapturedAt, err := time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		return runtime.RunCheckpoint{}, fmt.Errorf("parse checkpoint captured_at: %w", err)
	}
	checkpoint.CapturedAt = parsedCapturedAt

	return checkpoint, nil
}

func marshalJSON(value any) (sql.NullString, error) {
	if value == nil {
		return sql.NullString{}, nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return sql.NullString{}, err
	}

	return sql.NullString{String: string(encoded), Valid: true}, nil
}

func unmarshalNullableJSON(raw sql.NullString, target any) error {
	if !raw.Valid || raw.String == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw.String), target)
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func newCheckpointID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "ckpt_fallback"
	}
	return "ckpt_" + hex.EncodeToString(buf)
}
