//! Integration tests for the dope-mcp crate: round-trip persistence (in-memory and via
//! dope-store), manager behavior (registry, exposure, authorization, lifecycle with a
//! fake transport, catalog install/lifecycle), and pure-helper coverage (framing,
//! redaction, backoff, websocket endpoint validation).

use std::collections::HashMap;
use std::sync::mpsc;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use chrono::Utc;
use dope_mcp::catalog::{
    fingerprint_create_server_spec, requires_offline_verified_local_command,
};
use dope_mcp::manager::{
    redact_string, sanitize_websocket_endpoint_for_projection, validate_websocket_endpoint,
};
use dope_mcp::transport::read_framed_message;
use dope_mcp::types::*;
use dope_mcp::{
    McpError, Session, SessionPipes, Transport, is_terminal_status, live_validation_matrix_rows,
    restart_backoff_delay,
};
use serde_json::{Map, Value};

fn test_cfg(data_dir: &str) -> dope_config::Config {
    dope_config::Config {
        environment: dope_config::Environment::Test,
        bind_addr: "127.0.0.1:19192".to_string(),
        data_dir: data_dir.to_string(),
        log_level: "info".to_string(),
        version: "dev".to_string(),
        llm: Default::default(),
        connectors: Default::default(),
    }
}

fn streamable_server_input(server_id: &str) -> CreateServerInput {
    CreateServerInput {
        server_id: server_id.to_string(),
        display_name: "Test Server".to_string(),
        enabled: true,
        sandbox_profile_id: dope_sandbox::PROFILE_ID_SUBPROCESS_DEFAULT.to_string(),
        declaration_id: format!("mcp_server:{server_id}:lifecycle.start"),
        transport_kind: TransportKind::StreamableHTTP,
        endpoint: "https://example.test/mcp".to_string(),
        auto_restart: true,
        ..CreateServerInput::default()
    }
}

// ---------------------------------------------------------------------------
// Fake transport / session for lifecycle tests
// ---------------------------------------------------------------------------

struct FakeSession {
    id: String,
    tools: Vec<Tool>,
    done_tx: Mutex<mpsc::Sender<Result<(), String>>>,
    done_rx: Mutex<mpsc::Receiver<Result<(), String>>>,
}

impl FakeSession {
    fn new(id: &str, tools: Vec<Tool>) -> Arc<Self> {
        let (tx, rx) = mpsc::channel();
        Arc::new(FakeSession {
            id: id.to_string(),
            tools,
            done_tx: Mutex::new(tx),
            done_rx: Mutex::new(rx),
        })
    }
}

impl Session for FakeSession {
    fn id(&self) -> String {
        self.id.clone()
    }
    fn list_tools(&self, _timeout: Duration) -> Result<Vec<Tool>, String> {
        Ok(self.tools.clone())
    }
    fn call_tool(&self, _tool_name: &str, _input: Value) -> Result<Map<String, Value>, String> {
        Ok(Map::new())
    }
    fn close(&self) -> Result<(), String> {
        let _ = self.done_tx.lock().unwrap().send(Ok(()));
        Ok(())
    }
    fn wait_done(&self) -> Result<(), String> {
        self.done_rx.lock().unwrap().recv().unwrap_or(Ok(()))
    }
}

#[derive(Clone)]
struct FakeTransport {
    session: Arc<FakeSession>,
}

impl Transport for FakeTransport {
    fn open(
        &self,
        _server: &Server,
        _pipes: SessionPipes,
        _timeout: Duration,
    ) -> Result<Arc<dyn Session>, McpError> {
        Ok(self.session.clone())
    }
}

fn fake_tool(name: &str) -> Tool {
    Tool {
        tool_name: name.to_string(),
        discovery_status: DiscoveryStatus::Discovered,
        updated_at: Utc::now(),
        ..Tool::default()
    }
}

// ---------------------------------------------------------------------------
// Serde round-trip
// ---------------------------------------------------------------------------

#[test]
fn server_serde_round_trip_uses_camel_case_wire() {
    let now = Utc::now();
    let server = Server {
        tenant_id: "tenant-1".to_string(),
        server_id: "srv-1".to_string(),
        display_name: "Files".to_string(),
        source: Source::Config,
        origin_kind: OriginKind::Catalog,
        catalog_entry_id: "filesystem".to_string(),
        install_method: InstallMethod::Script,
        environment_scope: "test".to_string(),
        enabled: true,
        sandbox_profile_id: "subprocess_default".to_string(),
        declaration_id: "mcp_server:filesystem:lifecycle.start".to_string(),
        declaration: Declaration {
            execution_mode: dope_sandbox::ExecutionMode::Subprocess,
            allowed_backend_kinds: vec![dope_sandbox::BackendKind::Subprocess],
            read_roots: vec!["/tmp/root".to_string()],
            network_mode: dope_sandbox::NetworkMode::Deny,
            approval_mode: dope_sandbox::ApprovalMode::Allow,
            required_enforcement_strength: "declared_only".to_string(),
            active: true,
            ..Declaration::default()
        },
        transport_kind: TransportKind::StreamableHTTP,
        command: "npx".to_string(),
        args: vec!["-y".to_string(), "server".to_string()],
        endpoint: "https://example.test/mcp".to_string(),
        working_dir: "/tmp".to_string(),
        secret_refs: vec!["TOKEN".to_string()],
        auto_restart: true,
        operator_modified: false,
        created_at: now,
        updated_at: now,
        ..Server::default()
    };
    let value = serde_json::to_value(&server).unwrap();
    assert_eq!(value["serverId"], "srv-1");
    assert_eq!(value["displayName"], "Files");
    assert_eq!(value["source"], "config");
    assert_eq!(value["originKind"], "catalog");
    assert_eq!(value["installMethod"], "script");
    assert_eq!(value["transportKind"], "streamable-http");
    assert_eq!(value["declaration"]["executionMode"], "subprocess");
    assert_eq!(value["declaration"]["networkMode"], "deny");
    // resolvedWebsocketHeaders is json:"-" and must not serialize
    assert!(value.get("resolvedWebsocketHeaders").is_none());

    let decoded: Server = serde_json::from_value(value).unwrap();
    assert_eq!(decoded.server_id, "srv-1");
    assert_eq!(decoded.transport_kind, TransportKind::StreamableHTTP);
    assert_eq!(decoded.declaration.active, true);
    assert_eq!(decoded.source, Source::Config);
}

#[test]
fn exposure_rule_serde_round_trip() {
    let rule = ToolExposureRule {
        server_id: "s".to_string(),
        tool_name: "lookup".to_string(),
        runtime_surface: "chat".to_string(),
        exposure_mode: ExposureMode::ApprovalRequired,
        active: true,
        reason: "needs human gate".to_string(),
        updated_at: Utc::now(),
        ..ToolExposureRule::default()
    };
    let value = serde_json::to_value(&rule).unwrap();
    assert_eq!(value["exposureMode"], "approval_required");
    let decoded: ToolExposureRule = serde_json::from_value(value).unwrap();
    assert_eq!(decoded.exposure_mode, ExposureMode::ApprovalRequired);
}

// ---------------------------------------------------------------------------
// Registry behavior
// ---------------------------------------------------------------------------

#[test]
fn manager_registers_updates_and_lists_servers() {
    let manager = dope_mcp::Manager::new(test_cfg("~/.dope-test"), None, None, None, None, None);
    let (resource, created) = manager.create_server(streamable_server_input("srv-1")).unwrap();
    assert!(created);
    assert_eq!(resource.server.server_id, "srv-1");
    assert_eq!(resource.server.source, Source::Api);
    assert_eq!(resource.server.origin_kind, OriginKind::Manual);
    assert_eq!(resource.server.environment_scope, "test");
    // enabled => default state is Stopped
    assert_eq!(resource.state.status, LifecycleStatus::Stopped);
    assert_eq!(resource.availability_status, AvailabilityStatus::Ready);

    let servers = manager.list_servers();
    assert_eq!(servers.len(), 1);
    assert_eq!(servers[0].server.server_id, "srv-1");

    let server = manager.get_server("srv-1").unwrap();
    assert_eq!(server.display_name, "Test Server");

    let updated = manager
        .update_server(
            "srv-1",
            &UpdateServerInput {
                display_name: Some("Renamed".to_string()),
                enabled: Some(false),
                ..UpdateServerInput::default()
            },
        )
        .unwrap();
    assert_eq!(updated.server.display_name, "Renamed");
    assert!(updated.server.operator_modified);
    assert!(!updated.server.enabled);
    assert_eq!(updated.state.status, LifecycleStatus::Disabled);
}

#[test]
fn manager_rejects_invalid_servers() {
    let manager = dope_mcp::Manager::new(test_cfg("~/.dope-test"), None, None, None, None, None);
    // missing server id
    let err = manager
        .create_server(CreateServerInput::default())
        .unwrap_err();
    assert_eq!(err, McpError::ServerIDRequired);
    // missing declaration id
    let err = manager
        .create_server(CreateServerInput {
            server_id: "s".to_string(),
            sandbox_profile_id: "p".to_string(),
            transport_kind: TransportKind::StreamableHTTP,
            endpoint: "https://example.test/mcp".to_string(),
            ..CreateServerInput::default()
        })
        .unwrap_err();
    assert_eq!(err, McpError::DeclarationIDRequired);
    // stdio requires a command
    let err = manager
        .create_server(CreateServerInput {
            server_id: "s".to_string(),
            sandbox_profile_id: "p".to_string(),
            declaration_id: "d".to_string(),
            transport_kind: TransportKind::Stdio,
            ..CreateServerInput::default()
        })
        .unwrap_err();
    assert_eq!(err, McpError::CommandRequired);
    // streamable-http requires an endpoint
    let err = manager
        .create_server(CreateServerInput {
            server_id: "s".to_string(),
            sandbox_profile_id: "p".to_string(),
            declaration_id: "d".to_string(),
            transport_kind: TransportKind::StreamableHTTP,
            ..CreateServerInput::default()
        })
        .unwrap_err();
    assert_eq!(err, McpError::TransportUnavailable);
    // auto-restart requires enabled
    let err = manager
        .create_server(CreateServerInput {
            server_id: "s".to_string(),
            sandbox_profile_id: "p".to_string(),
            declaration_id: "d".to_string(),
            transport_kind: TransportKind::StreamableHTTP,
            endpoint: "https://example.test/mcp".to_string(),
            auto_restart: true,
            enabled: false,
            ..CreateServerInput::default()
        })
        .unwrap_err();
    assert_eq!(err, McpError::AutoRestartRequiresOn);
    // websocket endpoint validation
    let err = manager
        .create_server(CreateServerInput {
            server_id: "s".to_string(),
            sandbox_profile_id: "p".to_string(),
            declaration_id: "d".to_string(),
            transport_kind: TransportKind::Websocket,
            endpoint: "https://example.test/mcp".to_string(),
            ..CreateServerInput::default()
        })
        .unwrap_err();
    assert!(matches!(err, McpError::Other(_)));
}

// ---------------------------------------------------------------------------
// Lifecycle with a fake transport
// ---------------------------------------------------------------------------

#[test]
fn manager_start_discovers_tools_and_supports_stop() {
    let session = FakeSession::new("session-1", vec![fake_tool("lookup")]);
    let manager = dope_mcp::Manager::new(
        test_cfg("~/.dope-test"),
        None,
        None,
        None,
        None,
        Some(Arc::new(FakeTransport { session })),
    );
    manager.create_server(streamable_server_input("srv-1")).unwrap();

    let response = manager.start("srv-1", "operator").unwrap();
    assert_eq!(response.action, LifecycleAction::Start);
    assert!(!response.idempotent);
    assert_eq!(response.server.state.status, LifecycleStatus::Healthy);
    assert_eq!(response.server.state.last_session_id, "session-1");
    assert_eq!(response.server.tool_count, 1);

    // second start is idempotent
    let again = manager.start("srv-1", "operator").unwrap();
    assert!(again.idempotent);

    let tools = manager.list_tools("srv-1").unwrap();
    assert_eq!(tools.len(), 1);
    assert_eq!(tools[0].tool.tool_name, "lookup");
    assert_eq!(tools[0].tool.discovery_status, DiscoveryStatus::Discovered);
    // no exposure rule yet => blocked
    assert_eq!(tools[0].effective_availability, "blocked");

    let stop = manager.stop("srv-1").unwrap();
    assert_eq!(stop.action, LifecycleAction::Stop);
    assert_eq!(stop.server.state.status, LifecycleStatus::Stopped);

    // idempotent stop
    let stop = manager.stop("srv-1").unwrap();
    assert!(stop.idempotent);
}

#[test]
fn manager_start_fails_for_stdio_without_sandbox_manager() {
    let manager = dope_mcp::Manager::new(test_cfg("~/.dope-test"), None, None, None, None, None);
    manager
        .create_server(CreateServerInput {
            server_id: "srv-1".to_string(),
            enabled: true,
            sandbox_profile_id: "subprocess_default".to_string(),
            declaration_id: "d".to_string(),
            transport_kind: TransportKind::Stdio,
            command: "npx".to_string(),
            args: vec!["-y".to_string(), "server".to_string()],
            ..CreateServerInput::default()
        })
        .unwrap();
    let err = manager.start("srv-1", "operator").unwrap_err();
    assert_eq!(err, McpError::SandboxManagerMissing);
}

#[test]
fn manager_start_fails_when_transport_unavailable() {
    // Default manager transport mux has no concrete transports.
    let manager = dope_mcp::Manager::new(test_cfg("~/.dope-test"), None, None, None, None, None);
    manager.create_server(streamable_server_input("srv-1")).unwrap();
    let response = manager.start("srv-1", "operator").unwrap();
    assert_eq!(response.failure_class, "transport_runtime_failure");
    assert_eq!(response.server.state.status, LifecycleStatus::Failed);
}

// ---------------------------------------------------------------------------
// Exposure + authorization
// ---------------------------------------------------------------------------

#[test]
fn manager_update_exposure_and_authorize_tool() {
    let session = FakeSession::new("session-1", vec![fake_tool("lookup")]);
    let policy = dope_policy::Engine::new();
    let manager = dope_mcp::Manager::new(
        test_cfg("~/.dope-test"),
        None,
        None,
        None,
        Some(policy),
        Some(Arc::new(FakeTransport { session })),
    );
    manager.create_server(streamable_server_input("srv-1")).unwrap();
    manager.start("srv-1", "operator").unwrap();

    // no rule => blocked
    let blocked = manager
        .authorize_tool(
            "srv-1",
            "lookup",
            &AuthorizeToolInput {
                runtime_surface: "chat".to_string(),
                ..AuthorizeToolInput::default()
            },
        )
        .unwrap();
    assert_eq!(blocked.status, ToolAuthorizationStatus::Blocked);

    // allow rule => allowed
    let updated = manager
        .update_tool_exposure(
            "srv-1",
            "lookup",
            &UpdateExposureInput {
                runtime_surface: "chat".to_string(),
                exposure_mode: ExposureMode::Allow,
                active: true,
                ..UpdateExposureInput::default()
            },
        )
        .unwrap();
    assert_eq!(updated.effective_availability, "available");

    let allowed = manager
        .authorize_tool(
            "srv-1",
            "lookup",
            &AuthorizeToolInput {
                runtime_surface: "chat".to_string(),
                ..AuthorizeToolInput::default()
            },
        )
        .unwrap();
    assert_eq!(allowed.status, ToolAuthorizationStatus::Allowed);
    assert_eq!(allowed.session_id, "session-1");

    // invoke the tool
    let result = manager
        .call_tool("srv-1", "lookup", serde_json::json!({ "q": 1 }), &allowed)
        .unwrap();
    assert_eq!(result.failure_class, "");
    assert_eq!(result.session_id, "session-1");
    assert!(result.output.is_some());

    // un-authorized invocation is blocked
    let unauthed = ToolAuthorizationResponse {
        status: ToolAuthorizationStatus::Blocked,
        message: "denied".to_string(),
        ..ToolAuthorizationResponse::default()
    };
    let blocked = manager
        .call_tool("srv-1", "lookup", serde_json::json!({}), &unauthed)
        .unwrap();
    assert_eq!(blocked.failure_class, "blocked");

    // approval-required rule => pending via policy engine
    let _ = manager
        .update_tool_exposure(
            "srv-1",
            "lookup",
            &UpdateExposureInput {
                runtime_surface: "cli".to_string(),
                exposure_mode: ExposureMode::ApprovalRequired,
                active: true,
                ..UpdateExposureInput::default()
            },
        )
        .unwrap();
    let pending = manager
        .authorize_tool(
            "srv-1",
            "lookup",
            &AuthorizeToolInput {
                runtime_surface: "cli".to_string(),
                ..AuthorizeToolInput::default()
            },
        )
        .unwrap();
    assert_eq!(pending.status, ToolAuthorizationStatus::Pending);
    let approval = pending.approval.as_ref().unwrap();
    assert_eq!(approval.action, "tool_call.execute");
    assert!(manager.get_server("srv-1").is_some());

    manager.stop("srv-1").unwrap();
}

// ---------------------------------------------------------------------------
// Catalog install + lifecycle
// ---------------------------------------------------------------------------

#[test]
fn bundled_catalog_entries_are_sorted_and_context7_installs() {
    let manager = dope_mcp::Manager::new(test_cfg("~/.dope-test"), None, None, None, None, None);
    let entries = manager.list_catalog();
    assert_eq!(entries.len(), 5);
    let ids: Vec<&str> = entries.iter().map(|entry| entry.id.as_str()).collect();
    assert_eq!(ids, vec!["context7", "filesystem", "github", "postgres", "slack"]);
    assert_eq!(entries[0].transport_kind, TransportKind::StreamableHTTP);

    let result = manager
        .install_catalog_entry("context7", &CatalogInstallInput::default(), InstallMethod::Api)
        .unwrap();
    assert_eq!(result.status, "installed");
    assert_eq!(result.server.as_ref().unwrap().server_id, "context7");

    let server = manager.get_server("context7").unwrap();
    assert_eq!(server.origin_kind, OriginKind::Catalog);
    assert_eq!(server.catalog_entry_id, "context7");
    let management = server.catalog_management.as_ref().unwrap();
    assert_eq!(management.last_action, Some(CatalogAction::Install));
    assert!(!management.installed_revision.is_empty());

    // revalidate: healthy classification (streamable-http endpoint configured, no secrets)
    let revalidated = manager.revalidate_catalog_server("context7").unwrap();
    assert_eq!(revalidated.status, AvailabilityStatus::Ready);
    assert_eq!(revalidated.classification, RevalidationClassification::Healthy);

    // uninstall removes the server
    let uninstalled = manager.uninstall_catalog_server("context7").unwrap();
    assert_eq!(uninstalled.status, CatalogActionStatus::Completed);
    assert!(uninstalled.removed);
    assert!(manager.get_server("context7").is_none());

    // lifecycle action on a manual server is blocked
    manager.create_server(streamable_server_input("manual-1")).unwrap();
    let blocked = manager.refresh_catalog_server("manual-1").unwrap();
    assert_eq!(blocked.status, CatalogActionStatus::Blocked);
    assert_eq!(blocked.failure_class, "not_catalog_managed");
}

#[test]
fn catalog_install_blocks_manual_server_id_collision() {
    let manager = dope_mcp::Manager::new(test_cfg("~/.dope-test"), None, None, None, None, None);
    manager.create_server(streamable_server_input("context7")).unwrap();
    let result = manager
        .install_catalog_entry("context7", &CatalogInstallInput::default(), InstallMethod::Api)
        .unwrap();
    assert_eq!(result.status, "blocked");
    assert!(result.availability_reason.contains("already owned by a manual MCP server"));
}

#[test]
fn requires_offline_verified_local_command_matches_bundled_stdio_default() {
    let spec = CreateServerInput {
        transport_kind: TransportKind::Stdio,
        command: "npx".to_string(),
        args: vec!["-y".to_string(), "@modelcontextprotocol/server-filesystem".to_string()],
        declaration: Some(Declaration {
            network_mode: dope_sandbox::NetworkMode::Deny,
            ..Declaration::default()
        }),
        ..CreateServerInput::default()
    };
    assert!(requires_offline_verified_local_command(&spec));
    let spec = CreateServerInput {
        transport_kind: TransportKind::Stdio,
        command: "npx".to_string(),
        args: vec!["-y".to_string(), "some-other-package".to_string()],
        declaration: Some(Declaration {
            network_mode: dope_sandbox::NetworkMode::Deny,
            ..Declaration::default()
        }),
        ..CreateServerInput::default()
    };
    assert!(!requires_offline_verified_local_command(&spec));
}

#[test]
fn fingerprint_create_server_spec_is_stable() {
    let spec = CreateServerInput {
        command: "npx".to_string(),
        args: vec!["-y".to_string(), "@modelcontextprotocol/server-filesystem".to_string()],
        declaration: Some(Declaration {
            network_mode: dope_sandbox::NetworkMode::Deny,
            ..Declaration::default()
        }),
        ..CreateServerInput::default()
    };
    let first = fingerprint_create_server_spec(&spec);
    let second = fingerprint_create_server_spec(&spec);
    assert!(first.starts_with("sha256:"));
    assert_eq!(first, second);
    let changed = fingerprint_create_server_spec(&CreateServerInput {
        endpoint: "https://example.test/mcp".to_string(),
        ..spec
    });
    assert_ne!(first, changed);
}

// ---------------------------------------------------------------------------
// Store-backed restore
// ---------------------------------------------------------------------------

fn temp_data_dir(name: &str) -> std::path::PathBuf {
    let dir = std::env::temp_dir().join(format!("dope-mcp-{name}-{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    std::fs::create_dir_all(&dir).unwrap();
    dir
}

#[test]
fn manager_persists_and_restores_servers_from_store() {
    let dir = temp_data_dir("restore");
    let store = Arc::new(Mutex::new(dope_store::SQLiteStore::new(dir.to_str().unwrap()).unwrap()));
    let manager = dope_mcp::Manager::new(
        test_cfg(dir.to_str().unwrap()),
        Some(Arc::clone(&store)),
        None,
        None,
        None,
        None,
    );
    let (_, created) = manager
        .create_server(CreateServerInput {
            server_id: "srv-1".to_string(),
            display_name: "Persisted".to_string(),
            enabled: false,
            sandbox_profile_id: "subprocess_default".to_string(),
            declaration_id: "d".to_string(),
            transport_kind: TransportKind::StreamableHTTP,
            endpoint: "https://example.test/mcp".to_string(),
            ..CreateServerInput::default()
        })
        .unwrap();
    assert!(created);

    // a fresh manager over the same store restores the registry
    let manager2 = dope_mcp::Manager::new(
        test_cfg(dir.to_str().unwrap()),
        Some(Arc::clone(&store)),
        None,
        None,
        None,
        None,
    );
    manager2.restore().unwrap();
    let restored = manager2.get_server("srv-1").unwrap();
    assert_eq!(restored.display_name, "Persisted");
    assert!(!restored.enabled);
    assert_eq!(
        manager2.get_server_resource("srv-1").unwrap().state.status,
        LifecycleStatus::Disabled
    );
    let _ = std::fs::remove_dir_all(&dir);
}

#[test]
fn manager_restore_marks_tools_stale_and_reloads_exposure() {
    let dir = temp_data_dir("restore-tools");
    let store = Arc::new(Mutex::new(dope_store::SQLiteStore::new(dir.to_str().unwrap()).unwrap()));

    let session = FakeSession::new("session-1", vec![fake_tool("lookup")]);
    let manager = dope_mcp::Manager::new(
        test_cfg(dir.to_str().unwrap()),
        Some(Arc::clone(&store)),
        None,
        None,
        None,
        Some(Arc::new(FakeTransport { session })),
    );
    manager.create_server(streamable_server_input("srv-1")).unwrap();
    let response = manager.start("srv-1", "operator").unwrap();
    assert_eq!(response.server.state.status, LifecycleStatus::Healthy);
    let _ = manager
        .update_tool_exposure(
            "srv-1",
            "lookup",
            &UpdateExposureInput {
                runtime_surface: "chat".to_string(),
                exposure_mode: ExposureMode::Allow,
                active: true,
                ..UpdateExposureInput::default()
            },
        )
        .unwrap();
    manager.stop("srv-1").unwrap();

    // fresh manager without a transport: restore must reload tools and mark them stale
    // (state is not Healthy after daemon restart) and reload the exposure rule.
    let manager2 = dope_mcp::Manager::new(
        test_cfg(dir.to_str().unwrap()),
        Some(Arc::clone(&store)),
        None,
        None,
        None,
        None,
    );
    manager2.restore().unwrap();
    let tools = manager2.list_tools("srv-1").unwrap();
    assert_eq!(tools.len(), 1);
    assert_eq!(tools[0].tool.tool_name, "lookup");
    assert_eq!(tools[0].tool.discovery_status, DiscoveryStatus::Stale);
    let _ = std::fs::remove_dir_all(&dir);
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

#[test]
fn read_framed_message_decodes_content_length() {
    let payload = b"{\"hello\":\"world\"}";
    let framed = format!("Content-Length: {}\r\n\r\n{}", payload.len(), String::from_utf8_lossy(payload));
    let mut cursor = std::io::Cursor::new(framed.into_bytes());
    let decoded = read_framed_message(&mut cursor).unwrap();
    assert_eq!(decoded, payload);

    // lowercase header is accepted too
    let framed = format!("content-length: {}\r\n\r\n{}", payload.len(), String::from_utf8_lossy(payload));
    let mut cursor = std::io::Cursor::new(framed.into_bytes());
    let decoded = read_framed_message(&mut cursor).unwrap();
    assert_eq!(decoded, payload);

    // missing header is an error
    let mut cursor = std::io::Cursor::new(b"{}".to_vec());
    assert!(read_framed_message(&mut cursor).is_err());
}

#[test]
fn redact_string_redacts_common_derived_secret_forms() {
    let secrets = HashMap::from([("K".to_string(), "secret 123".to_string())]);
    // raw secret and its form-encoded candidate ("secret+123") are both redacted
    let redacted = redact_string("token=secret 123 and secret+123", &secrets);
    assert_eq!(redacted, "token=[REDACTED] and [REDACTED]");
    // unrelated text is untouched
    let redacted = redact_string("nothing to see", &secrets);
    assert_eq!(redacted, "nothing to see");
}

#[test]
fn websocket_endpoint_validation() {
    assert!(validate_websocket_endpoint("ws://example.com/mcp").is_ok());
    assert!(validate_websocket_endpoint("wss://example.com:8443/mcp").is_ok());
    assert!(validate_websocket_endpoint("").is_err());
    assert!(validate_websocket_endpoint("http://example.com").is_err());
    assert!(validate_websocket_endpoint("ws://user:pass@example.com").is_err());
    assert!(validate_websocket_endpoint("ws://example.com/mcp?token=abc").is_err());

    assert_eq!(
        sanitize_websocket_endpoint_for_projection("wss://user:pass@example.com/mcp?token=abc#frag"),
        "wss://example.com/mcp"
    );
}

#[test]
fn transport_capabilities_and_terminal_status() {
    let manager = dope_mcp::Manager::new(test_cfg("~/.dope-test"), None, None, None, None, None);
    let capabilities = manager.list_transport_capabilities();
    assert_eq!(capabilities.len(), 3);
    assert_eq!(capabilities[0].transport_kind, TransportKind::Stdio);
    assert!(capabilities[2].daemon_managed_reconnect);

    assert!(is_terminal_status(LifecycleStatus::Failed));
    assert!(is_terminal_status(LifecycleStatus::Disabled));
    assert!(is_terminal_status(LifecycleStatus::Stopped));
    assert!(!is_terminal_status(LifecycleStatus::Healthy));
    assert!(!is_terminal_status(LifecycleStatus::Starting));
}

#[test]
fn restart_backoff_delay_doubles_and_caps() {
    assert_eq!(restart_backoff_delay(0), Duration::from_secs(5));
    assert_eq!(restart_backoff_delay(1), Duration::from_secs(5));
    assert_eq!(restart_backoff_delay(2), Duration::from_secs(10));
    assert_eq!(restart_backoff_delay(3), Duration::from_secs(20));
    assert_eq!(restart_backoff_delay(4), Duration::from_secs(40));
    assert_eq!(restart_backoff_delay(100), Duration::from_secs(300));
}

#[test]
fn live_validation_matrix_marks_mcp_tool_calls_unsupported() {
    let rows = live_validation_matrix_rows();
    assert_eq!(rows.len(), 1);
    assert_eq!(rows[0].tool_class.as_str(), "mcp.tool_call");
    assert_eq!(rows[0].safety_class.as_str(), "unsupported");
}

#[test]
fn secret_resolution_falls_back_to_mcp_secrets_file() {
    let dir = temp_data_dir("secrets");
    std::fs::write(
        dir.join("mcp-secrets.json"),
        serde_json::json!({ "TOKEN": "  topsecret  " }).to_string(),
    )
    .unwrap();
    let manager = dope_mcp::Manager::new(test_cfg(dir.to_str().unwrap()), None, None, None, None, None);
    let (resolved, _) = manager
        .create_server(CreateServerInput {
            server_id: "srv-1".to_string(),
            enabled: true,
            sandbox_profile_id: "subprocess_default".to_string(),
            declaration_id: "d".to_string(),
            transport_kind: TransportKind::Websocket,
            endpoint: "wss://example.test/mcp".to_string(),
            secret_refs: vec!["TOKEN".to_string()],
            websocket_config: Some(WebsocketConfig {
                auth: Some(WebsocketAuthConfig {
                    mode: WebsocketAuthMode::BearerHeader,
                    secret_ref: "TOKEN".to_string(),
                    ..WebsocketAuthConfig::default()
                }),
                ..WebsocketConfig::default()
            }),
            ..CreateServerInput::default()
        })
        .unwrap();
    // availability is Blocked when the websocket auth secret cannot be resolved to a
    // value; here the file has TOKEN=topsecret so it resolves.
    assert_eq!(resolved.availability_status, AvailabilityStatus::Ready);
    let _ = std::fs::remove_dir_all(&dir);
}
