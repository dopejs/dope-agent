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

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	_ "modernc.org/sqlite"
)

const defaultDatabaseFile = "daemon.sqlite"

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
	statements := []string{
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
		`CREATE INDEX IF NOT EXISTS idx_sessions_channel_peer ON sessions(channel, peer_id, thread_id);`,
		`CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id, occurred_at);`,
		`CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id, occurred_at);`,
		`CREATE INDEX IF NOT EXISTS idx_events_category ON events(category, occurred_at);`,
		`CREATE INDEX IF NOT EXISTS idx_checkpoints_run_id ON checkpoints(run_id, captured_at);`,
	}

	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("run sqlite migration: %w", err)
		}
	}

	return nil
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
