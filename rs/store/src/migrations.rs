//! Schema migrations (port of the Go schemaMigrations list). Versions are added incrementally.

use crate::SchemaMigration;

#[must_use]
pub fn schema_migrations() -> Vec<SchemaMigration> {
    vec![
        SchemaMigration {
            version: 1,
            name: "baseline".to_string(),
            statements: vec![
                r#"CREATE TABLE IF NOT EXISTS schema_migrations (
                    version INTEGER PRIMARY KEY,
                    name TEXT NOT NULL,
                    applied_at TEXT NOT NULL
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS sessions (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS auth_pairings (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS auth_tokens (
                    token_id TEXT PRIMARY KEY,
                    label TEXT NOT NULL,
                    mode TEXT NOT NULL,
                    token_hash TEXT NOT NULL,
                    token_preview TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    last_used_at TEXT
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS approvals (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS decisions (
                    decision_id TEXT PRIMARY KEY,
                    action TEXT NOT NULL,
                    resource_kind TEXT,
                    resource_id TEXT,
                    outcome TEXT NOT NULL,
                    reason TEXT NOT NULL,
                    approval_id TEXT,
                    created_at TEXT NOT NULL,
                    FOREIGN KEY(approval_id) REFERENCES approvals(approval_id) ON DELETE SET NULL
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS runs (
                    run_id TEXT PRIMARY KEY,
                    session_id TEXT,
                    entrypoint TEXT NOT NULL,
                    status TEXT NOT NULL,
                    goal TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE SET NULL
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS connectors (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS capabilities (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS steps (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS tool_calls (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS llm_dispatches (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS events (
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
                );"#
                    .to_string(),
                r#"CREATE TABLE IF NOT EXISTS checkpoints (
                    checkpoint_id TEXT PRIMARY KEY,
                    run_id TEXT NOT NULL,
                    captured_at TEXT NOT NULL,
                    snapshot_json TEXT NOT NULL,
                    FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
                );"#
                    .to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_steps_run_id ON steps(run_id);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_connectors_kind_status ON connectors(kind, status);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_capabilities_kind_status ON capabilities(kind, status);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_tool_calls_run_step ON tool_calls(run_id, step_id, created_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_llm_dispatches_provider_status ON llm_dispatches(provider, status, created_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_approvals_status_created ON approvals(status, created_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_sessions_channel_peer ON sessions(channel, peer_id, thread_id);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id, occurred_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id, occurred_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_events_category ON events(category, occurred_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_checkpoints_run_id ON checkpoints(run_id, captured_at);"#.to_string(),
            ],
        },
    ]
}
