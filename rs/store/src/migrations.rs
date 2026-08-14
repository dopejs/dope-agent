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
        SchemaMigration {
            version: 2,
            name: "operational_indexes".to_string(),
            statements: vec![
                r#"CREATE INDEX IF NOT EXISTS idx_events_resource_scope ON events(resource_kind, resource_id, occurred_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_tool_calls_capability_created ON tool_calls(capability_id, created_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_auth_tokens_last_used_at ON auth_tokens(last_used_at);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_decisions_approval_id ON decisions(approval_id, created_at);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 3,
            name: "provider_checks".to_string(),
            statements: vec![
                r#"CREATE TABLE IF NOT EXISTS provider_checks (
                    check_id TEXT PRIMARY KEY,
                    provider_id TEXT NOT NULL,
                    family TEXT NOT NULL,
                    auth_mode TEXT NOT NULL,
                    status TEXT NOT NULL,
                    model TEXT NOT NULL,
                    endpoint TEXT,
                    error_class TEXT,
                    error_code TEXT,
                    error_message TEXT,
                    usage_json TEXT NOT NULL,
                    created_at TEXT NOT NULL,
                    completed_at TEXT NOT NULL
                );"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_provider_checks_provider_created ON provider_checks(provider_id, created_at DESC, check_id DESC);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 4,
            name: "managed_provider_state".to_string(),
            statements: vec![
                r#"CREATE TABLE IF NOT EXISTS provider_auth_states (
                    provider_id TEXT PRIMARY KEY,
                    family TEXT NOT NULL,
                    auth_mode TEXT NOT NULL,
                    status TEXT NOT NULL,
                    cli_path TEXT,
                    cli_available INTEGER NOT NULL,
                    account_label TEXT,
                    account_id TEXT,
                    plan TEXT,
                    auth_method TEXT,
                    login_command_json TEXT NOT NULL,
                    logout_command_json TEXT NOT NULL,
                    last_checked_at TEXT NOT NULL,
                    last_authenticated_at TEXT,
                    last_error TEXT,
                    metadata_json TEXT NOT NULL
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS provider_models (
                    provider_id TEXT NOT NULL,
                    model_id TEXT NOT NULL,
                    display_name TEXT NOT NULL,
                    description TEXT,
                    default_flag INTEGER NOT NULL,
                    available_flag INTEGER NOT NULL,
                    source TEXT NOT NULL,
                    chat INTEGER NOT NULL,
                    stream INTEGER NOT NULL,
                    coding INTEGER NOT NULL,
                    tool_use INTEGER NOT NULL,
                    reasoning_levels_json TEXT NOT NULL,
                    PRIMARY KEY (provider_id, model_id)
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS provider_preferences (
                    provider_id TEXT PRIMARY KEY,
                    default_model TEXT NOT NULL,
                    updated_at TEXT NOT NULL
                );"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_provider_models_provider ON provider_models(provider_id, model_id);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 5,
            name: "connector_messages".to_string(),
            statements: vec![
                r#"CREATE TABLE IF NOT EXISTS connector_messages (
                    delivery_id TEXT PRIMARY KEY,
                    connector_id TEXT NOT NULL,
                    direction TEXT NOT NULL,
                    external_message_id TEXT,
                    session_id TEXT,
                    run_id TEXT,
                    channel_id TEXT NOT NULL,
                    peer_id TEXT,
                    thread_id TEXT,
                    author_id TEXT,
                    content TEXT NOT NULL,
                    status TEXT NOT NULL,
                    error_text TEXT,
                    reply_to_external_message_id TEXT,
                    response_to_delivery_id TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE SET NULL,
                    FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE SET NULL
                );"#.to_string(),
                r#"CREATE UNIQUE INDEX IF NOT EXISTS idx_connector_messages_external ON connector_messages(connector_id, direction, external_message_id) WHERE external_message_id IS NOT NULL;"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_connector_messages_connector_created ON connector_messages(connector_id, created_at DESC, delivery_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_connector_messages_session_created ON connector_messages(session_id, created_at DESC, delivery_id DESC);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 6,
            name: "sandbox_execution_plane".to_string(),
            statements: vec![
                r#"CREATE TABLE IF NOT EXISTS sandbox_executions (
                    execution_id TEXT PRIMARY KEY,
                    profile_id TEXT NOT NULL,
                    backend_kind TEXT NOT NULL,
                    status TEXT NOT NULL,
                    approval_id TEXT,
                    requested_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    started_at TEXT,
                    completed_at TEXT,
                    document_json TEXT NOT NULL,
                    FOREIGN KEY(approval_id) REFERENCES approvals(approval_id) ON DELETE SET NULL
                );"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_sandbox_executions_status_requested ON sandbox_executions(status, requested_at DESC, execution_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_sandbox_executions_profile_requested ON sandbox_executions(profile_id, requested_at DESC, execution_id DESC);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 7,
            name: "sandbox_requirement_contract".to_string(),
            statements: vec![
                r#"ALTER TABLE provider_auth_states ADD COLUMN sandbox_json TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN sandbox_json TEXT;"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS consumer_policy_records (
                    policy_record_id TEXT PRIMARY KEY,
                    consumer_kind TEXT NOT NULL,
                    consumer_id TEXT NOT NULL,
                    operation_kind TEXT NOT NULL,
                    declaration_id TEXT,
                    status TEXT NOT NULL,
                    decision TEXT NOT NULL,
                    approval_status TEXT NOT NULL,
                    secret_resolution TEXT NOT NULL,
                    requested_by TEXT,
                    sandbox_execution_id TEXT,
                    tool_call_id TEXT,
                    provider_operation_id TEXT,
                    started_at TEXT NOT NULL,
                    completed_at TEXT,
                    document_json TEXT NOT NULL
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS secret_scope_bindings (
                    binding_id TEXT PRIMARY KEY,
                    consumer_kind TEXT NOT NULL,
                    consumer_id TEXT NOT NULL,
                    environment_scope TEXT NOT NULL,
                    secret_ref TEXT NOT NULL,
                    default_source TEXT NOT NULL,
                    delivery_kind TEXT NOT NULL,
                    active INTEGER NOT NULL,
                    document_json TEXT NOT NULL
                );"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_policy_records_consumer_started ON consumer_policy_records(consumer_kind, consumer_id, started_at DESC, policy_record_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_policy_records_status_started ON consumer_policy_records(status, started_at DESC, policy_record_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_secret_scope_bindings_consumer_secret ON secret_scope_bindings(consumer_kind, consumer_id, secret_ref);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 8,
            name: "mcp_execution_plane".to_string(),
            statements: vec![
                r#"CREATE TABLE IF NOT EXISTS mcp_servers (
                    server_id TEXT PRIMARY KEY,
                    enabled INTEGER NOT NULL,
                    updated_at TEXT NOT NULL,
                    document_json TEXT NOT NULL
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS mcp_server_states (
                    server_id TEXT PRIMARY KEY,
                    status TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    document_json TEXT NOT NULL,
                    FOREIGN KEY(server_id) REFERENCES mcp_servers(server_id) ON DELETE CASCADE
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS mcp_tools (
                    server_id TEXT NOT NULL,
                    tool_name TEXT NOT NULL,
                    discovery_status TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    last_discovered_at TEXT,
                    document_json TEXT NOT NULL,
                    PRIMARY KEY (server_id, tool_name),
                    FOREIGN KEY(server_id) REFERENCES mcp_servers(server_id) ON DELETE CASCADE
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS mcp_tool_exposure_rules (
                    server_id TEXT NOT NULL,
                    tool_name TEXT NOT NULL,
                    runtime_surface TEXT NOT NULL,
                    exposure_mode TEXT NOT NULL,
                    active INTEGER NOT NULL,
                    updated_at TEXT NOT NULL,
                    document_json TEXT NOT NULL,
                    PRIMARY KEY (server_id, tool_name, runtime_surface),
                    FOREIGN KEY(server_id, tool_name) REFERENCES mcp_tools(server_id, tool_name) ON DELETE CASCADE
                );"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_mcp_servers_enabled ON mcp_servers(enabled, updated_at DESC, server_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_mcp_server_states_status ON mcp_server_states(status, updated_at DESC, server_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_mcp_tools_server_status ON mcp_tools(server_id, discovery_status, tool_name);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_mcp_tool_exposure_surface ON mcp_tool_exposure_rules(runtime_surface, exposure_mode, server_id, tool_name);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 9,
            name: "skill_tool_sandbox_execution".to_string(),
            statements: vec![
                r#"ALTER TABLE tool_calls ADD COLUMN invocation_kind TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN skill_id TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN sandbox_execution_id TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN failure_class TEXT;"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_tool_calls_run_step_created ON tool_calls(run_id, step_id, created_at DESC, tool_call_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_tool_calls_skill_created ON tool_calls(skill_id, created_at DESC, tool_call_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_tool_calls_sandbox_execution ON tool_calls(sandbox_execution_id);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 10,
            name: "mcp_runtime_catalog".to_string(),
            statements: vec![
                r#"ALTER TABLE tool_calls ADD COLUMN mcp_server_id TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN mcp_server_name TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN mcp_tool_name TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN mcp_transport_kind TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN mcp_session_id TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN authorization_result TEXT;"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_tool_calls_mcp_server_created ON tool_calls(mcp_server_id, created_at DESC, tool_call_id DESC);"#.to_string(),
            ],
        },
        SchemaMigration {
            version: 11,
            name: "workflow_orchestration".to_string(),
            statements: vec![
                r#"CREATE TABLE IF NOT EXISTS workflows (
                    workflow_id TEXT PRIMARY KEY,
                    run_id TEXT NOT NULL,
                    environment_scope TEXT NOT NULL,
                    goal TEXT NOT NULL,
                    status TEXT NOT NULL,
                    plan_summary TEXT,
                    failure_summary TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    started_at TEXT,
                    completed_at TEXT,
                    interrupted_at TEXT,
                    document_json TEXT NOT NULL,
                    FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS workflow_steps (
                    workflow_step_id TEXT PRIMARY KEY,
                    workflow_id TEXT NOT NULL,
                    position INTEGER NOT NULL,
                    status TEXT NOT NULL,
                    runtime_step_id TEXT,
                    active_tool_call_id TEXT,
                    attempt_count INTEGER NOT NULL,
                    max_attempts INTEGER NOT NULL,
                    last_failure_class TEXT,
                    blocked_reason TEXT,
                    document_json TEXT NOT NULL,
                    FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS workflow_dependencies (
                    dependency_id TEXT PRIMARY KEY,
                    workflow_id TEXT NOT NULL,
                    document_json TEXT NOT NULL,
                    FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE
                );"#.to_string(),
                r#"CREATE TABLE IF NOT EXISTS workflow_handoffs (
                    handoff_id TEXT PRIMARY KEY,
                    workflow_id TEXT NOT NULL,
                    status TEXT NOT NULL,
                    document_json TEXT NOT NULL,
                    FOREIGN KEY(workflow_id) REFERENCES workflows(workflow_id) ON DELETE CASCADE
                );"#.to_string(),
                r#"ALTER TABLE steps ADD COLUMN workflow_id TEXT;"#.to_string(),
                r#"ALTER TABLE steps ADD COLUMN workflow_step_id TEXT;"#.to_string(),
                r#"ALTER TABLE steps ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN workflow_id TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN workflow_step_id TEXT;"#.to_string(),
                r#"ALTER TABLE tool_calls ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0;"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_workflows_run_env_created ON workflows(run_id, environment_scope, created_at DESC, workflow_id DESC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow_position ON workflow_steps(workflow_id, position ASC, workflow_step_id ASC);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_steps_workflow_linkage ON steps(workflow_id, workflow_step_id, attempt);"#.to_string(),
                r#"CREATE INDEX IF NOT EXISTS idx_tool_calls_workflow_linkage ON tool_calls(workflow_id, workflow_step_id, attempt, created_at DESC, tool_call_id DESC);"#.to_string(),
            ],
        },
    ]
}
