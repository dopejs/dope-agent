use std::path::Path;

use dope_store::{schema_migrations, SQLiteStore, CURRENT_SCHEMA_VERSION};

fn temp_dir(name: &str) -> String {
    let dir = std::env::temp_dir().join(format!("dope_store_{name}_{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    dir.to_string_lossy().to_string()
}

#[test]
fn opens_store_and_creates_schema_migrations_table() {
    let dir = temp_dir("open");
    let store = SQLiteStore::new(&dir).unwrap();
    assert_eq!(store.data_dir(), dir);
    assert!(Path::new(store.db_path()).exists());

    // All ported migrations are applied on open, up to the head of the ported list.
    let applied: i64 = store_conn_query(store.db_path(), "SELECT MAX(version) FROM schema_migrations");
    assert_eq!(applied, schema_migrations().last().unwrap().version);
}

#[test]
fn migrations_are_ordered_and_start_at_baseline() {
    let migrations = schema_migrations();
    assert!(migrations.len() >= 1);
    assert_eq!(migrations[0].version, 1);
    assert_eq!(migrations[0].name, "baseline");
    for pair in migrations.windows(2) {
        assert!(pair[0].version < pair[1].version);
    }
    assert_eq!(CURRENT_SCHEMA_VERSION, 55);
}

fn store_conn_query(db_path: &str, query: &str) -> i64 {
    let conn = rusqlite::Connection::open(db_path).unwrap();
    conn.query_row(query, [], |row| row.get(0)).unwrap()
}
use chrono::Utc;
use dope_capabilities::{Capability, Status as CapabilityStatus};
use dope_router::{Session, SessionKind, SessionStatus};
use dope_llm::{Dispatch, DispatchStatus, Message, MessageRole, Usage};
use dope_runtime::{Run, RunCheckpoint, RunStatus, Step, StepStatus, ToolCall, ToolCallStatus};

fn make_run() -> Run {
    let now = Utc::now();
    Run {
        run_id: "run_test".to_string(),
        session_id: String::new(),
        entrypoint: "test entrypoint".to_string(),
        status: RunStatus::Running,
        goal: "test goal".to_string(),
        created_at: now,
        updated_at: now,
        ..Run::default()
    }
}

fn make_step() -> Step {
    let now = Utc::now();
    Step {
        step_id: "step_1".to_string(),
        run_id: "run_test".to_string(),
        workflow_id: "wf_1".to_string(),
        workflow_step_id: "wfs_1".to_string(),
        attempt: 2,
        title: "Do the thing".to_string(),
        kind: "task".to_string(),
        status: StepStatus::Completed,
        created_at: now,
        updated_at: now,
        input: Some(serde_json::json!({"a": 1})),
        output: Some(serde_json::json!({"b": "done"})),
    }
}

fn make_tool_call() -> ToolCall {
    let now = Utc::now();
    let mut sandbox = serde_json::Map::new();
    sandbox.insert("session".to_string(), serde_json::json!("s-1"));
    ToolCall {
        tool_call_id: "tc_1".to_string(),
        run_id: "run_test".to_string(),
        step_id: "step_1".to_string(),
        invocation_kind: "mcp_tool".to_string(),
        capability_id: "cap_1".to_string(),
        mcp_server_id: "mcp_1".to_string(),
        mcp_tool_name: "search".to_string(),
        tool_name: "search".to_string(),
        status: ToolCallStatus::Completed,
        sandbox_execution_id: "sand_1".to_string(),
        failure_class: "timeout".to_string(),
        error: "boom".to_string(),
        input: Some(serde_json::json!({"q": "hi"})),
        output: Some(serde_json::json!({"r": 1})),
        sandbox,
        integration_bindings: vec![dope_integrations::BindingSummary {
            integration_id: "int_1".to_string(),
            domain_kind: "calendar".to_string(),
            display_name: "Calendar".to_string(),
            readiness_at_invocation: dope_integrations::ReadinessStatus::Healthy,
            backend_kind: dope_integrations::BackendKind::Native,
            ..dope_integrations::BindingSummary::default()
        }],
        created_at: now,
        updated_at: now,
        ..ToolCall::default()
    }
}

#[test]
fn upsert_run_and_read_tenant_id() {
    let dir = temp_dir("run");
    let store = SQLiteStore::new(&dir).unwrap();
    store.upsert_run(&make_run()).unwrap();
    // No tenant is bound through the legacy path, so the tenant id is absent.
    assert_eq!(store.run_tenant_id("run_test").unwrap(), None);
}

#[test]
fn step_round_trips_through_sqlite() {
    let dir = temp_dir("step");
    let store = SQLiteStore::new(&dir).unwrap();
    store.upsert_run(&make_run()).unwrap();
    let step = make_step();
    store.upsert_step(&step).unwrap();

    let listed = store.list_steps("run_test").unwrap();
    assert_eq!(listed.len(), 1);
    let got = &listed[0];
    assert_eq!(got.step_id, "step_1");
    assert_eq!(got.run_id, "run_test");
    assert_eq!(got.workflow_id, "wf_1");
    assert_eq!(got.workflow_step_id, "wfs_1");
    assert_eq!(got.attempt, 2);
    assert_eq!(got.title, "Do the thing");
    assert_eq!(got.kind, "task");
    assert_eq!(got.status, StepStatus::Completed);
    assert_eq!(got.input, Some(serde_json::json!({"a": 1})));
    assert_eq!(got.output, Some(serde_json::json!({"b": "done"})));
}

#[test]
fn tool_call_round_trips_through_sqlite() {
    let dir = temp_dir("toolcall");
    let store = SQLiteStore::new(&dir).unwrap();
    store.upsert_run(&make_run()).unwrap();
    store.upsert_step(&make_step()).unwrap();
    let tc = make_tool_call();
    store.upsert_tool_call(&tc).unwrap();

    let listed = store.list_tool_calls("run_test", "step_1").unwrap();
    assert_eq!(listed.len(), 1);
    let got = &listed[0];
    assert_eq!(got.tool_call_id, "tc_1");
    assert_eq!(got.invocation_kind, "mcp_tool");
    assert_eq!(got.capability_id, "cap_1");
    assert_eq!(got.mcp_server_id, "mcp_1");
    assert_eq!(got.mcp_tool_name, "search");
    assert_eq!(got.status, ToolCallStatus::Completed);
    assert_eq!(got.sandbox_execution_id, "sand_1");
    assert_eq!(got.failure_class, "timeout");
    assert_eq!(got.error, "boom");
    assert_eq!(got.input, Some(serde_json::json!({"q": "hi"})));
    assert_eq!(got.output, Some(serde_json::json!({"r": 1})));
    assert_eq!(got.sandbox.get("session"), Some(&serde_json::json!("s-1")));
    assert_eq!(got.integration_bindings.len(), 1);
    assert_eq!(got.integration_bindings[0].integration_id, "int_1");
    assert_eq!(got.integration_bindings[0].backend_kind, dope_integrations::BackendKind::Native);
}

#[test]
fn checkpoint_round_trips_through_sqlite() {
    let dir = temp_dir("checkpoint");
    let store = SQLiteStore::new(&dir).unwrap();
    let run = make_run();
    let step = make_step();
    let tool_call = make_tool_call();
    store.upsert_run(&run).unwrap();
    store.upsert_step(&step).unwrap();
    store.upsert_tool_call(&tool_call).unwrap();

    let checkpoint = RunCheckpoint {
        run: run.clone(),
        steps: vec![step.clone()],
        tool_calls: vec![tool_call.clone()],
        captured_at: Utc::now(),
    };
    store.save_checkpoint(&checkpoint).unwrap();

    let listed = store.list_latest_checkpoints().unwrap();
    assert_eq!(listed.len(), 1);
    let got = &listed[0];
    assert_eq!(got.run.run_id, "run_test");
    assert_eq!(got.run.goal, "test goal");
    assert_eq!(got.steps.len(), 1);
    assert_eq!(got.steps[0].step_id, "step_1");
    assert_eq!(got.tool_calls.len(), 1);
    assert_eq!(got.tool_calls[0].tool_call_id, "tc_1");
}
#[test]
fn session_round_trips_through_sqlite() {
    let dir = temp_dir("session");
    let store = SQLiteStore::new(&dir).unwrap();
    let now = Utc::now();
    let session = Session {
        session_id: "sess_1".to_string(),
        kind: SessionKind::Group,
        status: SessionStatus::Active,
        channel: "discord".to_string(),
        account_id: "acct_1".to_string(),
        peer_id: "peer_1".to_string(),
        thread_id: "thread_1".to_string(),
        routing_key: "discord:peer_1:thread_1".to_string(),
        generation: 3,
        created_at: now,
        updated_at: now,
        last_active_at: now,
        last_reset_at: Some(now),
        active_profile_projection: None,
    };
    store.upsert_session(&session).unwrap();

    let listed = store.list_sessions().unwrap();
    assert_eq!(listed.len(), 1);
    let got = &listed[0];
    assert_eq!(got.session_id, "sess_1");
    assert_eq!(got.kind, SessionKind::Group);
    assert_eq!(got.status, SessionStatus::Active);
    assert_eq!(got.channel, "discord");
    assert_eq!(got.account_id, "acct_1");
    assert_eq!(got.peer_id, "peer_1");
    assert_eq!(got.thread_id, "thread_1");
    assert_eq!(got.routing_key, "discord:peer_1:thread_1");
    assert_eq!(got.generation, 3);
    assert!(got.last_reset_at.is_some());
}

#[test]
fn capability_round_trips_through_sqlite() {
    let dir = temp_dir("capability");
    let store = SQLiteStore::new(&dir).unwrap();
    let now = Utc::now();
    let capability = Capability {
        capability_id: "cap_1".to_string(),
        kind: "browser".to_string(),
        display_name: "Browser".to_string(),
        status: CapabilityStatus::Healthy,
        failure_count: 2,
        restart_count: 1,
        backoff_seconds: 30,
        next_restart_at: Some(now),
        last_restart_at: Some(now),
        last_heartbeat_at: Some(now),
        last_failure_reason: "timeout".to_string(),
        created_at: now,
        updated_at: now,
    };
    store.upsert_capability(&capability).unwrap();

    let listed = store.list_capabilities().unwrap();
    assert_eq!(listed.len(), 1);
    let got = &listed[0];
    assert_eq!(got.capability_id, "cap_1");
    assert_eq!(got.kind, "browser");
    assert_eq!(got.display_name, "Browser");
    assert_eq!(got.status, CapabilityStatus::Healthy);
    assert_eq!(got.failure_count, 2);
    assert_eq!(got.restart_count, 1);
    assert_eq!(got.backoff_seconds, 30);
    assert_eq!(got.last_failure_reason, "timeout");
    assert!(got.next_restart_at.is_some());
}
#[test]
fn llm_dispatch_round_trips_through_sqlite() {
    let dir = temp_dir("llm");
    let store = SQLiteStore::new(&dir).unwrap();
    let now = Utc::now();
    let dispatch = Dispatch {
        dispatch_id: "disp_1".to_string(),
        provider: "openai".to_string(),
        model: "gpt-4o".to_string(),
        messages: vec![Message { role: MessageRole::User, content: "hi".to_string() }],
        stream: true,
        status: DispatchStatus::Completed,
        output: "hello".to_string(),
        finish_reason: "stop".to_string(),
        usage: Usage { input_tokens: 3, output_tokens: 1, total_tokens: 4 },
        error_code: String::new(),
        error: String::new(),
        timeout_ms: 30000,
        partial: false,
        max_retries: 2,
        attempt_count: 1,
        created_at: now,
        updated_at: now,
        started_at: Some(now),
        completed_at: Some(now),
    };
    store.upsert_llm_dispatch(&dispatch).unwrap();

    let listed = store.list_llm_dispatches().unwrap();
    assert_eq!(listed.len(), 1);
    let got = &listed[0];
    assert_eq!(got.dispatch_id, "disp_1");
    assert_eq!(got.provider, "openai");
    assert_eq!(got.model, "gpt-4o");
    assert_eq!(got.stream, true);
    assert_eq!(got.status, DispatchStatus::Completed);
    assert_eq!(got.output, "hello");
    assert_eq!(got.finish_reason, "stop");
    assert_eq!(got.messages.len(), 1);
    assert_eq!(got.messages[0].content, "hi");
    assert_eq!(got.usage.total_tokens, 4);
    assert_eq!(got.timeout_ms, 30000);
    assert!(got.started_at.is_some());
    assert!(got.completed_at.is_some());

    let fetched = store.get_llm_dispatch("disp_1").unwrap().expect("found");
    assert_eq!(fetched.dispatch_id, "disp_1");
    assert_eq!(store.get_llm_dispatch("missing").unwrap(), None);
}
