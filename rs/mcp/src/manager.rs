//! Port of `daemon/internal/mcp/manager.go`: the MCP server registry manager.
//!
//! The manager keeps servers, per-server runtime states, discovered tools, tool
//! exposure rules, and live sessions in memory behind a `parking_lot::RwLock` with
//! insertion-ordered server ids (the `dope-runtime` pattern). Every mutating method
//! persists through `dope-store`'s MCP CRUD when a store is installed and publishes
//! events through `dope-events` (plus the store event ledger).
//!
//! Conventions vs the Go original:
//! - `context.Context` is dropped (synchronous port). Background persistence in Go's
//!   detached goroutines is just direct calls.
//! - Tenant context (`tenantctx`) is not ported: `activeTenantID` is always "".
//! - The store `HasActiveMCPToolCalls` guard and approval/decision SQLite persistence
//!   are deferred (dope-store has no such CRUD yet); the corresponding checks are
//!   skipped and persist_approval/persist_decision are no-ops.
//! - watch_session / schedule_restart / schedule_websocket_reconnect run on detached
//!   threads holding an Arc clone of the manager, mirroring Go goroutines.

use std::collections::HashMap;
use std::sync::Arc;
use std::sync::Mutex;
use std::time::Instant;

use chrono::{DateTime, Utc};
use dope_events::Resource;
use serde_json::{Map, Value};

use crate::catalog::{
    bundled_catalog_entries, catalog_install_input_from_snapshot,
    evaluate_catalog_install_spec_availability, fingerprint_create_server_spec,
    install_snapshot_from_create_spec, look_path, requires_offline_verified_local_command,
    resolve_mcp_secrets, secret_refs_from_requirements,
};
use crate::transport::{Session, SessionPipes, Transport, TransportMux};
use crate::types::*;
use crate::{
    McpError, RESOURCE_KIND_SERVER, RESOURCE_KIND_TOOL, clean_strings, clone_backend_kinds,
    clone_catalog_management, clone_catalog_install_snapshot, clone_declaration_ptr,
    clone_strings, clone_tool_map, clone_websocket_config, default_declaration,
    environment_scope, first_non_empty, mcp_backoff_delay, normalize_declaration,
    rfc3339_nano, session_start_timeout,
};

/// Go `websocketReconnectMaxAttempts`.
const WEBSOCKET_RECONNECT_MAX_ATTEMPTS: i64 = 3;

/// Go `isRestoreLifecycleRequester`.
#[must_use]
pub fn is_restore_lifecycle_requester(requested_by: &str) -> bool {
    requested_by.trim() == "system.restore"
}

/// Go `isWebsocketReconnectRequester`.
#[must_use]
pub fn is_websocket_reconnect_requester(requested_by: &str) -> bool {
    requested_by.trim() == "mcp.websocket_reconnect"
}

/// Go `sandbox.AttachedExecution` (not yet in dope-sandbox): the process pipes handed
/// to a stdio MCP transport. Ported here as part of the deferred sandbox integration.
pub struct AttachedExecution {
    pub execution: dope_sandbox::Execution,
    pub stdin: Option<Box<dyn std::io::Write + Send>>,
    pub stdout: Option<Box<dyn std::io::Read + Send>>,
    pub stderr: Option<Box<dyn std::io::Read + Send>>,
}

/// Go `attachedExecutionStarter` interface: the sandbox execution-plane collaborator.
/// No workspace implementation exists yet; the manager behaves like the Go manager with
/// a nil sandbox manager when `None`.
pub trait AttachedExecutionStarter: Send + Sync {
    fn start_attached_execution(
        &self,
        request: &dope_sandbox::ExecutionRequest,
    ) -> Result<(dope_sandbox::Execution, Option<AttachedExecution>), String>;
    fn cancel_execution(&self, execution_id: &str) -> Result<(dope_sandbox::Execution, bool), String>;
    fn get_execution(&self, execution_id: &str) -> Option<dope_sandbox::Execution>;
    fn persist_consumer_view(&self, view: &dope_sandbox::ConsumerContractView) -> Result<(), String>;
    fn get_profile(&self, profile_id: &str) -> Option<dope_sandbox::Profile>;
}

/// Sync secret resolver replacing the (async, tenant-scoped) dope-secrets manager. The
/// Go fallback path (no secret manager) reads `mcp-secrets.json` from the data dir.
pub trait SecretResolver: Send + Sync {
    fn resolve(&self, secret_ref: &str) -> Result<Option<String>, String>;
}

/// In-memory session registration (Go `sessionState`).
#[derive(Clone, Default)]
pub struct SessionState {
    pub session_id: String,
    pub execution_id: String,
    pub session: Option<Arc<dyn Session>>,
    pub transport_kind: TransportKind,
    pub stop_requested: bool,
    pub cancel_requested: bool,
}

#[derive(Default)]
struct ManagerState {
    servers: HashMap<String, Server>,
    server_ids: Vec<String>,
    states: HashMap<String, ServerState>,
    tools: HashMap<String, HashMap<String, Tool>>,
    exposure: HashMap<String, HashMap<String, HashMap<String, ToolExposureRule>>>,
    sessions: HashMap<String, SessionState>,
}

struct ManagerInner {
    cfg: dope_config::Config,
    store: Option<Arc<Mutex<dope_store::SQLiteStore>>>,
    event_bus: Option<dope_events::Bus>,
    policy: Option<dope_policy::Engine>,
    sandboxes: Option<Arc<dyn AttachedExecutionStarter>>,
    transport: Option<Arc<dyn Transport>>,
    secrets: parking_lot::RwLock<Option<Arc<dyn SecretResolver>>>,
    state: parking_lot::RwLock<ManagerState>,
}

/// Cloneable handle over the shared MCP manager state (port of `*Manager`). Methods
/// are synchronous; detached watcher/restart threads hold `Arc` clones.
#[derive(Clone)]
pub struct Manager {
    inner: Arc<ManagerInner>,
}

impl Default for Manager {
    fn default() -> Self {
        Self::new(
            dope_config::Config {
                environment: dope_config::Environment::Test,
                bind_addr: "127.0.0.1:19192".to_string(),
                data_dir: "~/.dope-test".to_string(),
                log_level: "info".to_string(),
                version: "dev".to_string(),
                llm: Default::default(),
                connectors: Default::default(),
            },
            None,
            None,
            None,
            None,
            None,
        )
    }
}

impl Manager {
    /// Go `NewManager`. A `None` transport defaults to an all-unavailable
    /// `TransportMux` (the concrete transports are deferred).
    #[must_use]
    pub fn new(
        cfg: dope_config::Config,
        store: Option<Arc<Mutex<dope_store::SQLiteStore>>>,
        event_bus: Option<dope_events::Bus>,
        sandboxes: Option<Arc<dyn AttachedExecutionStarter>>,
        policy: Option<dope_policy::Engine>,
        transport: Option<Arc<dyn Transport>>,
    ) -> Self {
        let transport = match transport {
            Some(transport) => transport,
            None => Arc::new(TransportMux::default()),
        };
        Manager {
            inner: Arc::new(ManagerInner {
                cfg,
                store,
                event_bus,
                policy,
                sandboxes,
                transport: Some(transport),
                secrets: parking_lot::RwLock::new(None),
                state: parking_lot::RwLock::new(ManagerState::default()),
            }),
        }
    }

    /// Go `SetSecretManager`.
    pub fn set_secret_manager(&self, resolver: Arc<dyn SecretResolver>) {
        *self.inner.secrets.write() = Some(resolver);
    }

    /// Go `ListCatalog`.
    #[must_use]
    pub fn list_catalog(&self) -> Vec<CatalogEntry> {
        bundled_catalog_entries(&self.inner.cfg)
    }

    /// Go `GetCatalogEntry`.
    #[must_use]
    pub fn get_catalog_entry(&self, entry_id: &str) -> Option<CatalogEntry> {
        bundled_catalog_entries(&self.inner.cfg)
            .into_iter()
            .find(|entry| entry.id == entry_id.trim())
    }

    /// Go `ListTransportCapabilities`.
    #[must_use]
    pub fn list_transport_capabilities(&self) -> Vec<TransportCapability> {
        let environment = environment_scope(self.inner.cfg.environment);
        let mut items = vec![
            TransportCapability {
                transport_kind: TransportKind::Stdio,
                availability_status: AvailabilityStatus::Ready,
                health_status: TransportHealthStatus::Healthy,
                prerequisites: vec![
                    "stdio command must be configured per server".to_string(),
                    "sandbox profile must remain available for subprocess execution".to_string(),
                ],
                environment_scope: environment.clone(),
                daemon_managed_reconnect: false,
                recovery_summary: "stdio sessions restart through the existing daemon-owned lifecycle path".to_string(),
                ..TransportCapability::default()
            },
            TransportCapability {
                transport_kind: TransportKind::StreamableHTTP,
                availability_status: AvailabilityStatus::Ready,
                health_status: TransportHealthStatus::Healthy,
                prerequisites: vec![
                    "streamable-http endpoint must be configured per server".to_string(),
                    "remote endpoint reachability is evaluated per server".to_string(),
                ],
                environment_scope: environment.clone(),
                daemon_managed_reconnect: false,
                recovery_summary: "streamable-http sessions restart through the normal lifecycle path".to_string(),
                ..TransportCapability::default()
            },
            TransportCapability {
                transport_kind: TransportKind::Websocket,
                availability_status: AvailabilityStatus::Ready,
                health_status: TransportHealthStatus::Healthy,
                prerequisites: vec![
                    "websocket endpoint must be configured per server".to_string(),
                    "authenticated endpoints require secret-ref-backed header auth".to_string(),
                ],
                environment_scope: environment.clone(),
                supported_auth_kinds: vec![
                    WebsocketAuthMode::BearerHeader.as_str().to_string(),
                    WebsocketAuthMode::Header.as_str().to_string(),
                ],
                daemon_managed_reconnect: true,
                recovery_summary: "daemon manages bounded websocket reconnect and restore history".to_string(),
                ..TransportCapability::default()
            },
        ];
        let guard = self.inner.state.read();
        for server_id in &guard.server_ids {
            let Some(server) = guard.servers.get(server_id) else {
                continue;
            };
            if !server.environment_scope.is_empty() && server.environment_scope != environment {
                continue;
            }
            let state = guard.states.get(server_id).cloned().unwrap_or_default();
            for item in &mut items {
                if item.transport_kind != server.transport_kind {
                    continue;
                }
                match state.status {
                    LifecycleStatus::Degraded | LifecycleStatus::BackingOff => {
                        item.health_status = TransportHealthStatus::Degraded;
                        item.reason = first_non_empty(&[
                            item.reason.as_str(),
                            state.health_reason.as_str(),
                            "one or more servers are recovering",
                        ]);
                    }
                    LifecycleStatus::Unsupported => {
                        item.availability_status = AvailabilityStatus::Unsupported;
                        item.reason = first_non_empty(&[
                            item.reason.as_str(),
                            state.health_reason.as_str(),
                            "transport is unsupported for at least one configured server",
                        ]);
                    }
                    _ => {}
                }
            }
        }
        items
    }

    /// Go `Restore`: reloads servers/states/tools/exposure from the store, normalizes
    /// states and tool staleness, then auto-starts enabled servers.
    pub fn restore(&self) -> Result<(), McpError> {
        let Some(store) = self.inner.store.clone() else {
            return Ok(());
        };
        let lock = || {
            store
                .lock()
                .map_err(|_| McpError::Store("store lock poisoned".to_string()))
        };
        let server_records = lock()?.list_mcp_servers().map_err(McpError::Store)?;
        let state_records = lock()?.list_mcp_server_states().map_err(McpError::Store)?;
        let tool_records = lock()?.list_mcp_tools("").map_err(McpError::Store)?;
        let exposure_records = lock()?.list_mcp_tool_exposure_rules("").map_err(McpError::Store)?;

        let mut servers: HashMap<String, Server> = HashMap::new();
        let mut server_ids: Vec<String> = Vec::new();
        let mut states: HashMap<String, ServerState> = HashMap::new();
        let mut tools: HashMap<String, HashMap<String, Tool>> = HashMap::new();
        let mut exposure: HashMap<String, HashMap<String, HashMap<String, ToolExposureRule>>> = HashMap::new();

        for record in server_records {
            let server: Server = serde_json::from_str(&record.document)
                .map_err(|e| McpError::Store(format!("decode mcp server {}: {e}", record.server_id)))?;
            servers.insert(server.server_id.clone(), server.clone());
            server_ids.push(server.server_id.clone());
        }
        for record in state_records {
            let state: ServerState = serde_json::from_str(&record.document)
                .map_err(|e| McpError::Store(format!("decode mcp server state {}: {e}", record.server_id)))?;
            states.insert(state.server_id.clone(), state);
        }
        for record in tool_records {
            let tool: Tool = serde_json::from_str(&record.document)
                .map_err(|e| McpError::Store(format!("decode mcp tool {}/{}: {e}", record.server_id, record.tool_name)))?;
            tools.entry(tool.server_id.clone()).or_default().insert(tool.tool_name.clone(), tool);
        }
        for record in exposure_records {
            let rule: ToolExposureRule = serde_json::from_str(&record.document).map_err(|e| {
                McpError::Store(format!(
                    "decode mcp tool exposure rule {}/{}/{}: {e}",
                    record.server_id, record.tool_name, record.runtime_surface
                ))
            })?;
            exposure
                .entry(rule.server_id.clone())
                .or_default()
                .entry(rule.tool_name.clone())
                .or_default()
                .insert(rule.runtime_surface.clone(), rule);
        }

        for server_id in &server_ids {
            if !states.contains_key(server_id) {
                states.insert(server_id.clone(), default_state_for_server(&servers[server_id]));
                continue;
            }
            let server = &servers[server_id];
            let mut state = states[server_id].clone();
            if !server.enabled {
                state.status = LifecycleStatus::Disabled;
            } else if state.status != LifecycleStatus::Stopped && state.status != LifecycleStatus::Disabled {
                state.status = LifecycleStatus::Stopped;
                state.health_reason = "daemon restart cleared in-memory MCP session state".to_string();
                state.last_execution_id = String::new();
            }
            state.updated_at = Utc::now();
            states.insert(server_id.clone(), state);
        }

        {
            let mut guard = self.inner.state.write();
            guard.servers = servers;
            guard.server_ids = server_ids.clone();
            guard.states = states.clone();
            guard.tools = tools.clone();
            guard.exposure = exposure.clone();
            guard.sessions = HashMap::new();

            for server_id in &server_ids {
                if let Some(state) = guard.states.get(server_id) {
                    let _ = self.persist_state(state);
                }
            }
            for server_id in &server_ids {
                let Some(map) = guard.tools.get(server_id) else { continue };
                if map.is_empty() {
                    continue;
                }
                let server = &guard.servers[server_id];
                let healthy = server.enabled
                    && guard
                        .states
                        .get(server_id)
                        .is_some_and(|s| s.status == LifecycleStatus::Healthy);
                if !healthy {
                    if let Some(tool_map) = guard.tools.get_mut(server_id) {
                        for tool in tool_map.values_mut() {
                            if tool.discovery_status == DiscoveryStatus::Discovered {
                                tool.discovery_status = DiscoveryStatus::Stale;
                                tool.updated_at = Utc::now();
                            }
                        }
                    }
                }
            }
        }

        for server_id in &server_ids {
            let tools = {
                let guard = self.inner.state.read();
                guard.tools.get(server_id).cloned().unwrap_or_default()
            };
            let tool_list: Vec<Tool> = tools.into_values().collect();
            if let Err(err) = self.persist_tool_map(server_id, &tool_list) {
                return Err(err);
            }
        }

        for server_id in &server_ids {
            let Some(server) = self.get_server(server_id) else {
                continue;
            };
            if !server.enabled {
                continue;
            }
            let _ = self.start(server_id, "system.restore");
        }
        Ok(())
    }

    /// Go `ListServers` (insertion order).
    #[must_use]
    pub fn list_servers(&self) -> Vec<ServerResource> {
        let guard = self.inner.state.read();
        guard
            .server_ids
            .iter()
            .filter_map(|server_id| {
                let server = guard.servers.get(server_id)?;
                Some(self.build_server_resource_locked(&guard, server))
            })
            .collect()
    }

    /// Go `ListServersForTenant` (an empty tenant id lists everything).
    #[must_use]
    pub fn list_servers_for_tenant(&self, tenant_id: &str) -> Vec<ServerResource> {
        let tenant_id = tenant_id.trim().to_string();
        let guard = self.inner.state.read();
        guard
            .server_ids
            .iter()
            .filter_map(|server_id| {
                let server = guard.servers.get(server_id)?;
                if !tenant_id.is_empty() && server.tenant_id != tenant_id {
                    return None;
                }
                Some(self.build_server_resource_locked(&guard, server))
            })
            .collect()
    }

    /// Go `GetServer`.
    #[must_use]
    pub fn get_server(&self, server_id: &str) -> Option<Server> {
        self.inner.state.read().servers.get(server_id.trim()).cloned()
    }

    /// Go `GetServerForTenant`.
    #[must_use]
    pub fn get_server_for_tenant(&self, server_id: &str, tenant_id: &str) -> Option<Server> {
        let server = self.get_server(server_id)?;
        let tenant_id = tenant_id.trim().to_string();
        if !tenant_id.is_empty() && server.tenant_id != tenant_id {
            return None;
        }
        Some(server)
    }

    /// Go `GetServerResource`.
    #[must_use]
    pub fn get_server_resource(&self, server_id: &str) -> Option<ServerResource> {
        let guard = self.inner.state.read();
        let server = guard.servers.get(server_id.trim())?;
        Some(self.build_server_resource_locked(&guard, server))
    }

    /// Go `GetServerResourceForTenant`.
    #[must_use]
    pub fn get_server_resource_for_tenant(&self, server_id: &str, tenant_id: &str) -> Option<ServerResource> {
        let guard = self.inner.state.read();
        let server = guard.servers.get(server_id.trim())?;
        let tenant_id = tenant_id.trim().to_string();
        if !tenant_id.is_empty() && server.tenant_id != tenant_id {
            return None;
        }
        Some(self.build_server_resource_locked(&guard, server))
    }

    /// Go `ListTools`.
    pub fn list_tools(&self, server_id: &str) -> Result<Vec<ToolResource>, McpError> {
        let guard = self.inner.state.read();
        let server = guard
            .servers
            .get(server_id.trim())
            .cloned()
            .ok_or(McpError::ServerNotFound)?;
        let mut items = Vec::new();
        if let Some(map) = guard.tools.get(&server.server_id) {
            for tool in map.values() {
                items.push(self.build_tool_resource_locked(&guard, &server, tool));
            }
        }
        Ok(items)
    }

    /// Go `ListToolsForTenant`.
    pub fn list_tools_for_tenant(&self, server_id: &str, tenant_id: &str) -> Result<Vec<ToolResource>, McpError> {
        let guard = self.inner.state.read();
        let server = guard
            .servers
            .get(server_id.trim())
            .cloned()
            .ok_or(McpError::ServerNotFound)?;
        let tenant_id = tenant_id.trim().to_string();
        if !tenant_id.is_empty() && server.tenant_id != tenant_id {
            return Err(McpError::ServerNotFound);
        }
        let mut items = Vec::new();
        if let Some(map) = guard.tools.get(&server.server_id) {
            for tool in map.values() {
                items.push(self.build_tool_resource_locked(&guard, &server, tool));
            }
        }
        Ok(items)
    }

    /// Go `CreateServer`.
    pub fn create_server(&self, input: CreateServerInput) -> Result<(ServerResource, bool), McpError> {
        self.upsert_server(input, None)
    }

    /// Go `UpdateServer`.
    pub fn update_server(&self, server_id: &str, input: &UpdateServerInput) -> Result<ServerResource, McpError> {
        let (resource, _) = self.upsert_server(
            CreateServerInput::default(),
            Some(UpdateOperation {
                server_id: server_id.trim().to_string(),
                input: input.clone(),
            }),
        )?;
        Ok(resource)
    }

    /// Go `UpdateToolExposure`.
    pub fn update_tool_exposure(
        &self,
        server_id: &str,
        tool_name: &str,
        input: &UpdateExposureInput,
    ) -> Result<ToolResource, McpError> {
        let server_id = server_id.trim().to_string();
        let tool_name = tool_name.trim().to_string();
        if server_id.is_empty() {
            return Err(McpError::ServerIDRequired);
        }
        if tool_name.is_empty() {
            return Err(McpError::ToolNameRequired);
        }
        if input.runtime_surface.trim().is_empty() {
            return Err(McpError::RuntimeSurfaceRequired);
        }

        let now = Utc::now();
        let rule = ToolExposureRule {
            server_id: server_id.clone(),
            tool_name: tool_name.clone(),
            runtime_surface: input.runtime_surface.trim().to_string(),
            exposure_mode: input.exposure_mode,
            active: input.active,
            reason: input.reason.trim().to_string(),
            updated_at: now,
            ..ToolExposureRule::default()
        };

        let resource = {
            let mut guard = self.inner.state.write();
            let server = guard
                .servers
                .get(&server_id)
                .cloned()
                .ok_or(McpError::ServerNotFound)?;
            let tool = guard
                .tools
                .get(&server_id)
                .and_then(|map| map.get(&tool_name))
                .cloned()
                .ok_or(McpError::ToolNameRequired)?;
            guard
                .exposure
                .entry(server_id.clone())
                .or_default()
                .entry(tool_name.clone())
                .or_default()
                .insert(rule.runtime_surface.clone(), rule.clone());
            self.build_tool_resource_locked(&guard, &server, &tool)
        };

        self.persist_exposure_rule(&rule)?;
        let mut payload = Map::new();
        payload.insert("serverId".to_string(), Value::String(server_id.clone()));
        payload.insert("toolName".to_string(), Value::String(tool_name.clone()));
        payload.insert("runtimeSurface".to_string(), Value::String(rule.runtime_surface.clone()));
        payload.insert("exposureMode".to_string(), Value::String(rule.exposure_mode.as_str().to_string()));
        payload.insert("active".to_string(), Value::Bool(rule.active));
        payload.insert("reason".to_string(), Value::String(rule.reason.clone()));
        self.publish_event(
            "mcp",
            "mcp.tool_exposure_updated",
            Resource {
                kind: RESOURCE_KIND_TOOL.to_string(),
                id: format!("{server_id}:{tool_name}"),
            },
            payload,
        )?;
        Ok(resource)
    }

    /// Go `AuthorizeTool`: checks the exposure rule, then either allow-lists, requests
    /// approval through the policy engine, or resolves a previously issued approval.
    pub fn authorize_tool(
        &self,
        server_id: &str,
        tool_name: &str,
        input: &AuthorizeToolInput,
    ) -> Result<ToolAuthorizationResponse, McpError> {
        let server_id = server_id.trim().to_string();
        let tool_name = tool_name.trim().to_string();
        let runtime_surface = input.runtime_surface.trim().to_string();
        if server_id.is_empty() {
            return Err(McpError::ServerIDRequired);
        }
        if tool_name.is_empty() {
            return Err(McpError::ToolNameRequired);
        }
        if runtime_surface.is_empty() {
            return Err(McpError::RuntimeSurfaceRequired);
        }

        let (server, tool, active, rule, resource) = {
            let guard = self.inner.state.read();
            let server = guard
                .servers
                .get(&server_id)
                .cloned()
                .ok_or(McpError::ServerNotFound)?;
            let tool = guard
                .tools
                .get(&server_id)
                .and_then(|map| map.get(&tool_name))
                .cloned()
                .ok_or(McpError::ToolNameRequired)?;
            let active = guard.sessions.get(&server_id).cloned();
            let rule = guard
                .exposure
                .get(&server_id)
                .and_then(|map| map.get(&tool_name))
                .and_then(|map| map.get(&runtime_surface))
                .cloned();
            let resource = self.build_tool_resource_locked(&guard, &server, &tool);
            (server, tool, active, rule, resource)
        };

        let rule_blocked = match &rule {
            None => true,
            Some(rule) => !rule.active || rule.exposure_mode == ExposureMode::Blocked,
        };
        if rule_blocked {
            let message = first_non_empty(&[
                resource.unavailable_reason.as_str(),
                "tool is not allowlisted for this runtime surface",
            ]);
            return Ok(ToolAuthorizationResponse {
                status: ToolAuthorizationStatus::Blocked,
                tool: resource,
                message,
                ..ToolAuthorizationResponse::default()
            });
        }
        let rule = rule.expect("rule present when not blocked");
        if resource.effective_availability != "available" {
            return Ok(ToolAuthorizationResponse {
                status: ToolAuthorizationStatus::Blocked,
                tool: resource,
                message: first_non_empty(&[
                    resource.unavailable_reason.as_str(),
                    "tool is not currently available",
                ]),
                ..ToolAuthorizationResponse::default()
            });
        }

        let approval_mode = if rule.exposure_mode == ExposureMode::ApprovalRequired {
            dope_sandbox::ApprovalMode::Ask
        } else {
            dope_sandbox::ApprovalMode::Allow
        };
        let consumer = self.build_tool_consumer_view(
            &server,
            &tool_name,
            &runtime_surface,
            &first_non_empty(&[input.requested_by.trim(), "mcp"]),
            approval_mode,
        )?;

        if rule.exposure_mode == ExposureMode::Allow {
            self.persist_consumer_view(&consumer)?;
            return Ok(ToolAuthorizationResponse {
                status: ToolAuthorizationStatus::Allowed,
                tool: resource,
                session_id: session_id(active.as_ref()),
                sandbox: Some(consumer),
                message: "tool use is allowed".to_string(),
                ..ToolAuthorizationResponse::default()
            });
        }

        let policy = self.inner.policy.as_ref().ok_or(McpError::PolicyNotConfigured)?;
        let approval_resource_id = format!("{server_id}:{tool_name}:{runtime_surface}");
        let requested_by = first_non_empty(&[input.requested_by.trim(), "mcp"]);
        if input.approval_id.trim().is_empty() {
            let (mut approval, mut decision) = policy
                .request_approval(dope_policy::RequestApprovalInput {
                    action: "tool_call.execute".to_string(),
                    resource_kind: RESOURCE_KIND_TOOL.to_string(),
                    resource_id: approval_resource_id,
                    reason: "MCP tool execution requires approval".to_string(),
                    requested_by: requested_by.clone(),
                    ..dope_policy::RequestApprovalInput::default()
                })
                .map_err(|e| McpError::Other(e.to_string()))?;
            approval.sandbox = consumer_view_map(&consumer);
            decision.sandbox = consumer_view_map(&consumer);
            let mut consumer = consumer;
            let record = consumer
                .policy_record
                .as_mut()
                .expect("consumer view always carries a policy record");
            record.approval_id = approval.approval_id.clone();
            record.decision_id = decision.decision_id.clone();
            record.decision = dope_sandbox::DecisionResolution::Ask;
            record.approval_status = dope_sandbox::DecisionApprovalStatus::Pending;
            record.status = dope_sandbox::PolicyRecordStatus::ApprovalPending;
            record.failure_class = dope_sandbox::ErrorClass::ApprovalRequired.as_str().to_string();
            self.persist_approval(&approval)?;
            self.persist_decision(&decision)?;
            self.persist_consumer_view(&consumer)?;
            self.publish_event(
                "policy",
                "policy.approval_requested",
                Resource {
                    kind: "approval".to_string(),
                    id: approval.approval_id.clone(),
                },
                approval_payload(&approval),
            )?;
            self.publish_event(
                "policy",
                "policy.decision_recorded",
                Resource {
                    kind: "decision".to_string(),
                    id: decision.decision_id.clone(),
                },
                decision_payload(&decision),
            )?;
            return Ok(ToolAuthorizationResponse {
                status: ToolAuthorizationStatus::Pending,
                tool: resource,
                session_id: session_id(active.as_ref()),
                message: "tool use requires approval".to_string(),
                approval: Some(approval),
                decision: Some(decision),
                sandbox: Some(consumer),
                ..ToolAuthorizationResponse::default()
            });
        }

        let approval = policy
            .get_approval(input.approval_id.trim())
            .ok_or(McpError::ApprovalNotFound)?;
        if approval.action != "tool_call.execute"
            || approval.resource_kind != RESOURCE_KIND_TOOL
            || approval.resource_id != approval_resource_id
        {
            return Err(McpError::ApprovalIDInvalid);
        }
        let mut consumer = consumer;
        consumer
            .policy_record
            .as_mut()
            .expect("consumer view always carries a policy record")
            .approval_id = approval.approval_id.clone();
        match approval.status {
            dope_policy::ApprovalStatus::Approved => {
                let record = consumer
                    .policy_record
                    .as_mut()
                    .expect("consumer view always carries a policy record");
                record.decision = dope_sandbox::DecisionResolution::Allow;
                record.approval_status = dope_sandbox::DecisionApprovalStatus::Approved;
                record.status = dope_sandbox::PolicyRecordStatus::PreflightAllowed;
                record.failure_class = String::new();
                self.persist_consumer_view(&consumer)?;
                Ok(ToolAuthorizationResponse {
                    status: ToolAuthorizationStatus::Allowed,
                    tool: resource,
                    session_id: session_id(active.as_ref()),
                    message: "tool use is allowed by approval".to_string(),
                    sandbox: Some(consumer),
                    ..ToolAuthorizationResponse::default()
                })
            }
            dope_policy::ApprovalStatus::Rejected => {
                let record = consumer
                    .policy_record
                    .as_mut()
                    .expect("consumer view always carries a policy record");
                record.decision = dope_sandbox::DecisionResolution::Deny;
                record.approval_status = dope_sandbox::DecisionApprovalStatus::Rejected;
                record.status = dope_sandbox::PolicyRecordStatus::Denied;
                record.failure_class = dope_sandbox::ErrorClass::ApprovalRejected.as_str().to_string();
                self.persist_consumer_view(&consumer)?;
                Ok(ToolAuthorizationResponse {
                    status: ToolAuthorizationStatus::Rejected,
                    tool: resource,
                    session_id: session_id(active.as_ref()),
                    message: "approval was rejected".to_string(),
                    approval: Some(approval),
                    sandbox: Some(consumer),
                    ..ToolAuthorizationResponse::default()
                })
            }
            dope_policy::ApprovalStatus::Pending => {
                let record = consumer
                    .policy_record
                    .as_mut()
                    .expect("consumer view always carries a policy record");
                record.decision = dope_sandbox::DecisionResolution::Ask;
                record.approval_status = dope_sandbox::DecisionApprovalStatus::Pending;
                record.status = dope_sandbox::PolicyRecordStatus::ApprovalPending;
                record.failure_class = dope_sandbox::ErrorClass::ApprovalRequired.as_str().to_string();
                self.persist_consumer_view(&consumer)?;
                Ok(ToolAuthorizationResponse {
                    status: ToolAuthorizationStatus::Pending,
                    tool: resource,
                    session_id: session_id(active.as_ref()),
                    message: "approval is still pending".to_string(),
                    approval: Some(approval),
                    sandbox: Some(consumer),
                    ..ToolAuthorizationResponse::default()
                })
            }
        }
    }

    /// Go `InstallCatalogEntry`.
    pub fn install_catalog_entry(
        &self,
        entry_id: &str,
        input: &CatalogInstallInput,
        method: InstallMethod,
    ) -> Result<CatalogInstallResult, McpError> {
        let entry = self.get_catalog_entry(entry_id).ok_or(McpError::ServerNotFound)?;
        let install_id = format!("mcp_install_{}", Utc::now().timestamp_nanos_opt().unwrap_or(0));
        let mut requested_payload = Map::new();
        requested_payload.insert("installId".to_string(), Value::String(install_id.clone()));
        requested_payload.insert("catalogEntryId".to_string(), Value::String(entry.id.clone()));
        requested_payload.insert("method".to_string(), Value::String(method.as_str().to_string()));
        requested_payload.insert(
            "environment".to_string(),
            Value::String(environment_scope(self.inner.cfg.environment)),
        );
        let requested_event = self.publish_audit_event(
            "mcp.catalog_install_requested",
            Resource {
                kind: "mcp_catalog_install".to_string(),
                id: install_id.clone(),
            },
            requested_payload,
        )?;

        let create_input = merge_catalog_install_input(&entry, input, method, self.inner.cfg.environment);
        let (install_availability, install_reason) = evaluate_catalog_install_spec_availability(
            &self.inner.cfg,
            &create_input,
            &entry.secret_requirements,
        );
        if install_availability != AvailabilityStatus::Ready {
            let mut result = CatalogInstallResult {
                install_id: install_id.clone(),
                status: "blocked".to_string(),
                catalog_entry_id: entry.id.clone(),
                server_id: create_input.server_id.clone(),
                availability_status: install_availability,
                availability_reason: install_reason.clone(),
                audit_event_ids: vec![requested_event.event_id.clone()],
                ..CatalogInstallResult::default()
            };
            let mut failed_payload = Map::new();
            failed_payload.insert("installId".to_string(), Value::String(install_id.clone()));
            failed_payload.insert("catalogEntryId".to_string(), Value::String(entry.id.clone()));
            failed_payload.insert("method".to_string(), Value::String(method.as_str().to_string()));
            failed_payload.insert("status".to_string(), Value::String(result.status.clone()));
            failed_payload.insert(
                "availabilityStatus".to_string(),
                Value::String(result.availability_status.as_str().to_string()),
            );
            failed_payload.insert(
                "availabilityReason".to_string(),
                Value::String(result.availability_reason.clone()),
            );
            if let Ok(failed_event) = self.publish_audit_event(
                "mcp.catalog_install_failed",
                Resource {
                    kind: "mcp_catalog_install".to_string(),
                    id: install_id,
                },
                failed_payload,
            ) {
                result.audit_event_ids.push(failed_event.event_id.clone());
            }
            return Ok(result);
        }

        if let Some(existing) = self.get_server(&create_input.server_id) {
            if let Some(reason) = catalog_install_conflict_reason(&existing, &entry.id) {
                let mut result = CatalogInstallResult {
                    install_id: install_id.clone(),
                    status: "blocked".to_string(),
                    catalog_entry_id: entry.id.clone(),
                    server_id: existing.server_id.clone(),
                    availability_status: AvailabilityStatus::Blocked,
                    availability_reason: reason.clone(),
                    audit_event_ids: vec![requested_event.event_id.clone()],
                    ..CatalogInstallResult::default()
                };
                let mut failed_payload = Map::new();
                failed_payload.insert("installId".to_string(), Value::String(install_id.clone()));
                failed_payload.insert("catalogEntryId".to_string(), Value::String(entry.id.clone()));
                failed_payload.insert("serverId".to_string(), Value::String(existing.server_id.clone()));
                failed_payload.insert("method".to_string(), Value::String(method.as_str().to_string()));
                failed_payload.insert("status".to_string(), Value::String(result.status.clone()));
                failed_payload.insert(
                    "availabilityStatus".to_string(),
                    Value::String(result.availability_status.as_str().to_string()),
                );
                failed_payload.insert(
                    "availabilityReason".to_string(),
                    Value::String(result.availability_reason.clone()),
                );
                if let Ok(failed_event) = self.publish_audit_event(
                    "mcp.catalog_install_failed",
                    Resource {
                        kind: "mcp_catalog_install".to_string(),
                        id: install_id,
                    },
                    failed_payload,
                ) {
                    result.audit_event_ids.push(failed_event.event_id.clone());
                }
                return Ok(result);
            }
        }

        let mut create_input = create_input;
        create_input.catalog_management = Some(catalog_management_for_create(
            &entry,
            &create_input,
            None,
            CatalogAction::Install,
            Utc::now(),
        ));
        let (resource, _) = match self.create_server(create_input.clone()) {
            Ok(created) => created,
            Err(err) => {
                let mut failed_payload = Map::new();
                failed_payload.insert("installId".to_string(), Value::String(install_id.clone()));
                failed_payload.insert("catalogEntryId".to_string(), Value::String(entry.id.clone()));
                failed_payload.insert("method".to_string(), Value::String(method.as_str().to_string()));
                failed_payload.insert("status".to_string(), Value::String("failed".to_string()));
                failed_payload.insert("reason".to_string(), Value::String(err.to_string()));
                let _ = self.publish_audit_event(
                    "mcp.catalog_install_failed",
                    Resource {
                        kind: "mcp_catalog_install".to_string(),
                        id: install_id,
                    },
                    failed_payload,
                );
                return Err(err);
            }
        };
        let mut result = CatalogInstallResult {
            install_id: install_id.clone(),
            status: "installed".to_string(),
            catalog_entry_id: entry.id.clone(),
            server_id: resource.server_id.clone(),
            availability_status: resource.availability_status,
            availability_reason: resource.availability_reason.clone(),
            audit_event_ids: vec![requested_event.event_id.clone()],
            server: Some(resource.clone()),
            ..CatalogInstallResult::default()
        };
        let mut completed_payload = Map::new();
        completed_payload.insert("installId".to_string(), Value::String(install_id.clone()));
        completed_payload.insert("catalogEntryId".to_string(), Value::String(entry.id.clone()));
        completed_payload.insert("serverId".to_string(), Value::String(resource.server_id.clone()));
        completed_payload.insert("method".to_string(), Value::String(method.as_str().to_string()));
        completed_payload.insert("status".to_string(), Value::String(result.status.clone()));
        completed_payload.insert(
            "availabilityStatus".to_string(),
            Value::String(result.availability_status.as_str().to_string()),
        );
        completed_payload.insert(
            "availabilityReason".to_string(),
            Value::String(result.availability_reason.clone()),
        );
        if let Ok(completed_event) = self.publish_audit_event(
            "mcp.catalog_install_completed",
            Resource {
                kind: "mcp_catalog_install".to_string(),
                id: install_id,
            },
            completed_payload,
        ) {
            result.audit_event_ids.push(completed_event.event_id.clone());
        }
        Ok(result)
    }

    /// Go `RefreshCatalogServer`.
    pub fn refresh_catalog_server(&self, server_id: &str) -> Result<CatalogLifecycleResult, McpError> {
        self.run_catalog_lifecycle_action(server_id, CatalogAction::Refresh)
    }

    /// Go `ReinstallCatalogServer`.
    pub fn reinstall_catalog_server(&self, server_id: &str) -> Result<CatalogLifecycleResult, McpError> {
        self.run_catalog_lifecycle_action(server_id, CatalogAction::Reinstall)
    }

    /// Go `UninstallCatalogServer`.
    pub fn uninstall_catalog_server(&self, server_id: &str) -> Result<CatalogLifecycleResult, McpError> {
        self.run_catalog_lifecycle_action(server_id, CatalogAction::Uninstall)
    }

    /// Go `RevalidateCatalogServer`.
    pub fn revalidate_catalog_server(&self, server_id: &str) -> Result<CatalogRevalidationResult, McpError> {
        let started_at = Instant::now();
        let server_id = server_id.trim().to_string();
        if server_id.is_empty() {
            return Err(McpError::ServerIDRequired);
        }
        let server = self.get_server(&server_id).ok_or(McpError::ServerNotFound)?;
        let mut result = CatalogRevalidationResult {
            action_id: format!("mcp_revalidate_{}", Utc::now().timestamp_nanos_opt().unwrap_or(0)),
            action: CatalogAction::Revalidate,
            server_id: server.server_id.clone(),
            catalog_entry_id: server.catalog_entry_id.clone(),
            ..CatalogRevalidationResult::default()
        };
        let mut requested_payload = Map::new();
        requested_payload.insert("actionId".to_string(), Value::String(result.action_id.clone()));
        requested_payload.insert("action".to_string(), Value::String(result.action.as_str().to_string()));
        requested_payload.insert("serverId".to_string(), Value::String(server.server_id.clone()));
        requested_payload.insert(
            "catalogEntryId".to_string(),
            Value::String(server.catalog_entry_id.clone()),
        );
        requested_payload.insert(
            "environment".to_string(),
            Value::String(environment_scope(self.inner.cfg.environment)),
        );
        let requested_event = self.publish_audit_event(
            "mcp.catalog_lifecycle_requested",
            Resource {
                kind: RESOURCE_KIND_SERVER.to_string(),
                id: server.server_id.clone(),
            },
            requested_payload,
        )?;
        result.audit_event_ids.push(requested_event.event_id.clone());

        if let Some(blocked) = self.catalog_target_block_result(&server) {
            return Ok(self.catalog_revalidation_blocked_result(&server, &result, &blocked, started_at));
        }
        if let Some(blocked) = self.catalog_revalidation_busy_block_result(&server)? {
            return Ok(self.catalog_revalidation_blocked_result(&server, &result, &blocked, started_at));
        }

        let management = self.build_catalog_management_locked(&server);
        let (issues, status, classification, reason) =
            self.collect_revalidation_issues(&server, management.as_ref());
        let checked_at = Utc::now();
        let mut server = server;
        server.catalog_management = management;
        if server.catalog_management.is_none() {
            server.catalog_management = Some(CatalogManagement::default());
        }
        if let Some(cm) = &mut server.catalog_management {
            cm.last_action = Some(CatalogAction::Revalidate);
            cm.last_action_status = Some(CatalogActionStatus::Completed);
            cm.last_action_failure_class = String::new();
            cm.last_action_reason = reason.clone();
            cm.last_action_at = Some(checked_at);
            cm.last_revalidation = Some(RevalidationSnapshot {
                checked_at,
                status,
                classification,
                reason: reason.clone(),
                issues: issues.clone(),
            });
        }
        self.set_server(&server);
        self.persist_server(&server)?;

        result.status = status;
        result.classification = classification;
        result.reason = reason.clone();
        result.issues = issues.clone();
        result.preflight_ms = started_at.elapsed().as_millis() as i64;
        if let Some(resource) = self.get_server_resource(&server.server_id) {
            result.server = Some(resource);
        }
        let mut completed_payload = Map::new();
        completed_payload.insert("actionId".to_string(), Value::String(result.action_id.clone()));
        completed_payload.insert("action".to_string(), Value::String(result.action.as_str().to_string()));
        completed_payload.insert("serverId".to_string(), Value::String(server.server_id.clone()));
        completed_payload.insert(
            "catalogEntryId".to_string(),
            Value::String(server.catalog_entry_id.clone()),
        );
        completed_payload.insert("status".to_string(), Value::String(result.status.as_str().to_string()));
        completed_payload.insert(
            "classification".to_string(),
            Value::String(result.classification.as_str().to_string()),
        );
        completed_payload.insert("reason".to_string(), Value::String(result.reason.clone()));
        completed_payload.insert("issues".to_string(), Value::Array(redacted_issues(&result.issues)));
        completed_payload.insert(
            "environment".to_string(),
            Value::String(environment_scope(self.inner.cfg.environment)),
        );
        if let Ok(completed_event) = self.publish_audit_event(
            "mcp.catalog_revalidation_completed",
            Resource {
                kind: RESOURCE_KIND_SERVER.to_string(),
                id: server.server_id.clone(),
            },
            completed_payload,
        ) {
            result.audit_event_ids.push(completed_event.event_id.clone());
        }
        Ok(result)
    }

    /// Go `CallTool`.
    pub fn call_tool(
        &self,
        server_id: &str,
        tool_name: &str,
        input: Value,
        authorization: &ToolAuthorizationResponse,
    ) -> Result<ToolInvocationResult, McpError> {
        let server_id = server_id.trim().to_string();
        let tool_name = tool_name.trim().to_string();
        if server_id.is_empty() {
            return Err(McpError::ServerIDRequired);
        }
        if tool_name.is_empty() {
            return Err(McpError::ToolNameRequired);
        }
        if authorization.status != ToolAuthorizationStatus::Allowed {
            return Ok(ToolInvocationResult {
                failure_class: "blocked".to_string(),
                error: first_non_empty(&[authorization.message.as_str(), "tool use is not allowed"]),
                ..ToolInvocationResult::default()
            });
        }
        let (active, server) = {
            let guard = self.inner.state.read();
            let active = guard.sessions.get(&server_id).cloned();
            let server = guard.servers.get(&server_id).cloned();
            (active, server)
        };
        let Some(server) = server else {
            // Go returns both the result and ErrServerNotFound; Rust surfaces the error.
            return Err(McpError::ServerNotFound);
        };
        let Some(active) = active else {
            return Ok(ToolInvocationResult {
                failure_class: "server_unhealthy".to_string(),
                error: "mcp server is not healthy".to_string(),
                ..ToolInvocationResult::default()
            });
        };
        let Some(session) = &active.session else {
            return Ok(ToolInvocationResult {
                failure_class: "server_unhealthy".to_string(),
                error: "mcp server is not healthy".to_string(),
                ..ToolInvocationResult::default()
            });
        };
        match session.call_tool(&tool_name, input) {
            Err(err) => Ok(ToolInvocationResult {
                session_id: active.session_id.clone(),
                failure_class: "transport_failed".to_string(),
                error: err,
                ..ToolInvocationResult::default()
            }),
            Ok(output) => {
                let redacted = self.redact_value(&server, Value::Object(output.clone()));
                let is_error = output
                    .get("isError")
                    .and_then(Value::as_bool)
                    .unwrap_or(false);
                if is_error {
                    Ok(ToolInvocationResult {
                        session_id: active.session_id.clone(),
                        output: Some(redacted),
                        failure_class: "remote_tool_error".to_string(),
                        error: first_non_empty(&[
                            string_from_map(&output, "message").as_str(),
                            "remote MCP tool returned an error",
                        ]),
                        ..ToolInvocationResult::default()
                    })
                } else {
                    Ok(ToolInvocationResult {
                        session_id: active.session_id.clone(),
                        output: Some(redacted),
                        ..ToolInvocationResult::default()
                    })
                }
            }
        }
    }

    /// Go `Start`: opens a transport session (stdio via the sandbox execution plane,
    /// streamable-http/websocket via the transport), discovers tools, and marks the
    /// server healthy. The concrete transports are deferred; with a default mux the
    /// transport-open step fails with `transport_runtime_failure`.
    pub fn start(&self, server_id: &str, requested_by: &str) -> Result<LifecycleResponse, McpError> {
        let started_at = Instant::now();
        let server_id = server_id.trim().to_string();
        if server_id.is_empty() {
            return Err(McpError::ServerIDRequired);
        }
        if self.inner.transport.is_none() {
            return Err(McpError::TransportNotConfigured);
        }
        let restore_request = is_restore_lifecycle_requester(requested_by);
        let reconnect_request = is_websocket_reconnect_requester(requested_by);

        let (server, mut state) = {
            let mut guard = self.inner.state.write();
            let server = match guard.servers.get(&server_id) {
                Some(server) => server.clone(),
                None => return Err(McpError::ServerNotFound),
            };
            if let Some(active) = guard.sessions.get(&server_id) {
                let resource = self.build_server_resource_locked(&guard, &server);
                return Ok(LifecycleResponse {
                    action: LifecycleAction::Start,
                    server: resource,
                    idempotent: true,
                    execution_id: active.execution_id.clone(),
                    preflight_ms: started_at.elapsed().as_millis() as i64,
                    ..LifecycleResponse::default()
                });
            }
            let mut state = guard.states.get(&server_id).cloned().unwrap_or_default();
            if !server.enabled {
                state.status = LifecycleStatus::Disabled;
                state.updated_at = Utc::now();
                guard.states.insert(server_id.clone(), state.clone());
                let resource = self.build_server_resource_locked(&guard, &server);
                let persisted = state.clone();
                drop(guard);
                let _ = self.persist_state(&persisted);
                return Ok(LifecycleResponse {
                    action: LifecycleAction::Start,
                    server: resource,
                    idempotent: true,
                    blocked: true,
                    blocked_reason: "server is disabled".to_string(),
                    preflight_ms: started_at.elapsed().as_millis() as i64,
                    ..LifecycleResponse::default()
                });
            }
            state.status = LifecycleStatus::Starting;
            state.health_reason = String::new();
            state.next_reconnect_at = None;
            if restore_request {
                state.last_recovery_at = Some(Utc::now());
                state.last_recovery_class = "restore_requested".to_string();
            } else if !reconnect_request {
                state.last_recovery_at = None;
                state.last_recovery_class = String::new();
            }
            state.updated_at = Utc::now();
            guard.states.insert(server_id.clone(), state.clone());
            (server, state)
        };
        if let Err(err) = self.persist_state(&state) {
            return Err(err);
        }

        let requested_by = first_non_empty(&[requested_by.trim(), "mcp"]);
        let consumer = match self.build_lifecycle_consumer_view(&server, &requested_by) {
            Ok(consumer) => consumer,
            Err(err) => {
                if restore_request {
                    self.record_restore_failure(
                        &server,
                        &mut state,
                        LifecycleStatus::Denied,
                        &err.to_string(),
                        "invalid_configuration",
                    );
                } else {
                    self.record_failure(
                        &server_id,
                        &mut state,
                        LifecycleStatus::Denied,
                        &err.to_string(),
                        "invalid_configuration",
                    );
                }
                let resource = self.get_server_resource(&server_id).unwrap_or_default();
                return Ok(LifecycleResponse {
                    action: LifecycleAction::Start,
                    server: resource,
                    failure_class: "invalid_configuration".to_string(),
                    blocked: true,
                    blocked_reason: err.to_string(),
                    preflight_ms: started_at.elapsed().as_millis() as i64,
                    ..LifecycleResponse::default()
                });
            }
        };

        let mut pipes = SessionPipes::default();
        let mut execution_id = String::new();
        let mut transport_server = server.clone();
        if server.transport_kind == TransportKind::Stdio {
            let sandboxes = match &self.inner.sandboxes {
                Some(sandboxes) => Arc::clone(sandboxes),
                None => return Err(McpError::SandboxManagerMissing),
            };
            let request = match self.build_execution_request(&server, &consumer, "") {
                Ok(request) => request,
                Err(err) => {
                    if restore_request {
                        self.record_restore_failure(
                            &server,
                            &mut state,
                            LifecycleStatus::Denied,
                            &err.to_string(),
                            "invalid_configuration",
                        );
                    } else {
                        self.record_failure(
                            &server_id,
                            &mut state,
                            LifecycleStatus::Denied,
                            &err.to_string(),
                            "invalid_configuration",
                        );
                    }
                    let resource = self.get_server_resource(&server_id).unwrap_or_default();
                    return Ok(LifecycleResponse {
                        action: LifecycleAction::Start,
                        server: resource,
                        failure_class: "invalid_configuration".to_string(),
                        blocked: true,
                        blocked_reason: err.to_string(),
                        preflight_ms: started_at.elapsed().as_millis() as i64,
                        ..LifecycleResponse::default()
                    });
                }
            };
            match sandboxes.start_attached_execution(&request) {
                Ok((_execution, Some(attached))) => {
                    execution_id = attached.execution.execution_id.clone();
                    pipes = SessionPipes {
                        stdin: attached.stdin,
                        stdout: attached.stdout,
                        stderr: attached.stderr,
                    };
                }
                Ok((execution, None)) => {
                    self.update_state_from_execution(&server_id, &mut state, &execution, false);
                    let resource = self.get_server_resource(&server_id).unwrap_or_default();
                    return Ok(LifecycleResponse {
                        action: LifecycleAction::Start,
                        server: resource,
                        execution_id: execution.execution_id.clone(),
                        failure_class: classify_execution_failure(&execution),
                        blocked: true,
                        blocked_reason: first_non_empty(&[
                            execution.result.error.as_str(),
                            execution.decision.explanation.as_str(),
                            state.health_reason.as_str(),
                        ]),
                        preflight_ms: started_at.elapsed().as_millis() as i64,
                        ..LifecycleResponse::default()
                    });
                }
                Err(err) => {
                    if restore_request {
                        self.record_restore_failure(
                            &server,
                            &mut state,
                            LifecycleStatus::Failed,
                            &err,
                            "launch_failed",
                        );
                    } else {
                        self.record_failure(&server_id, &mut state, LifecycleStatus::Failed, &err, "launch_failed");
                    }
                    let resource = self.get_server_resource(&server_id).unwrap_or_default();
                    return Ok(LifecycleResponse {
                        action: LifecycleAction::Start,
                        server: resource,
                        failure_class: "launch_failed".to_string(),
                        preflight_ms: started_at.elapsed().as_millis() as i64,
                        ..LifecycleResponse::default()
                    });
                }
            }
        }
        if server.transport_kind == TransportKind::Websocket {
            match self.resolve_websocket_headers(&server) {
                Ok(headers) => {
                    transport_server.resolved_websocket_headers = headers;
                }
                Err(err) => {
                    if restore_request {
                        self.record_restore_failure(
                            &server,
                            &mut state,
                            LifecycleStatus::Denied,
                            &err.to_string(),
                            "invalid_configuration",
                        );
                    } else {
                        self.record_failure(
                            &server_id,
                            &mut state,
                            LifecycleStatus::Denied,
                            &err.to_string(),
                            "invalid_configuration",
                        );
                    }
                    let resource = self.get_server_resource(&server_id).unwrap_or_default();
                    return Ok(LifecycleResponse {
                        action: LifecycleAction::Start,
                        server: resource,
                        failure_class: "invalid_configuration".to_string(),
                        blocked: true,
                        blocked_reason: err.to_string(),
                        preflight_ms: started_at.elapsed().as_millis() as i64,
                        ..LifecycleResponse::default()
                    });
                }
            }
        }

        let timeout = session_start_timeout();
        let session = match self
            .inner
            .transport
            .as_ref()
            .expect("transport defaults to a mux")
            .open(&transport_server, pipes, timeout)
        {
            Ok(session) => session,
            Err(err) => {
                if !execution_id.is_empty() {
                    if let Some(sandboxes) = &self.inner.sandboxes {
                        let _ = sandboxes.cancel_execution(&execution_id);
                    }
                }
                if restore_request {
                    self.record_restore_failure(
                        &server,
                        &mut state,
                        LifecycleStatus::Failed,
                        &err.to_string(),
                        "transport_runtime_failure",
                    );
                } else {
                    self.record_failure(
                        &server_id,
                        &mut state,
                        LifecycleStatus::Failed,
                        &err.to_string(),
                        "transport_runtime_failure",
                    );
                }
                let resource = self.get_server_resource(&server_id).unwrap_or_default();
                return Ok(LifecycleResponse {
                    action: LifecycleAction::Start,
                    server: resource,
                    execution_id: execution_id.clone(),
                    failure_class: "transport_runtime_failure".to_string(),
                    preflight_ms: started_at.elapsed().as_millis() as i64,
                    ..LifecycleResponse::default()
                });
            }
        };

        let tools = match session.list_tools(timeout) {
            Ok(tools) => tools,
            Err(err) => {
                let _ = session.close();
                if !execution_id.is_empty() {
                    if let Some(sandboxes) = &self.inner.sandboxes {
                        let _ = sandboxes.cancel_execution(&execution_id);
                    }
                }
                if restore_request {
                    self.record_restore_failure(
                        &server,
                        &mut state,
                        LifecycleStatus::Failed,
                        &err,
                        "transport_runtime_failure",
                    );
                } else {
                    self.record_failure(
                        &server_id,
                        &mut state,
                        LifecycleStatus::Failed,
                        &err,
                        "transport_runtime_failure",
                    );
                }
                let resource = self.get_server_resource(&server_id).unwrap_or_default();
                return Ok(LifecycleResponse {
                    action: LifecycleAction::Start,
                    server: resource,
                    execution_id: execution_id.clone(),
                    failure_class: "transport_runtime_failure".to_string(),
                    preflight_ms: started_at.elapsed().as_millis() as i64,
                    ..LifecycleResponse::default()
                });
            }
        };

        let now = Utc::now();
        let session_id = session.id();
        if state.last_started_at.is_some() {
            state.restart_count += 1;
        }
        state.status = LifecycleStatus::Healthy;
        state.last_execution_id = execution_id.clone();
        state.last_session_id = session_id.clone();
        state.last_started_at = Some(now);
        state.last_heartbeat_at = Some(now);
        state.health_reason = String::new();
        state.failure_count = 0;
        state.reconnect_attempt_count = 0;
        state.next_reconnect_at = None;
        if restore_request {
            state.last_recovery_at = Some(now);
            state.last_recovery_class = "restore_succeeded".to_string();
        }
        state.updated_at = now;

        let (resource, persisted_tools) = {
            let mut guard = self.inner.state.write();
            guard.states.insert(server_id.clone(), state.clone());
            guard.sessions.insert(
                server_id.clone(),
                SessionState {
                    session_id: session_id.clone(),
                    execution_id: execution_id.clone(),
                    session: Some(Arc::clone(&session)),
                    transport_kind: server.transport_kind,
                    stop_requested: false,
                    cancel_requested: false,
                },
            );
            {
                let tool_map = guard.tools.entry(server_id.clone()).or_default();
                for existing in tool_map.values_mut() {
                    existing.discovery_status = DiscoveryStatus::Stale;
                    existing.updated_at = now;
                }
                for mut tool in tools {
                    tool.server_id = server_id.clone();
                    tool.discovery_status = DiscoveryStatus::Discovered;
                    tool.updated_at = now;
                    tool.last_discovered_at = Some(now);
                    tool_map.insert(tool.tool_name.clone(), tool);
                }
            }
            let resource = self.build_server_resource_locked(&guard, &server);
            let persisted_tools = guard
                .tools
                .get(&server_id)
                .map(clone_tool_map)
                .unwrap_or_default();
            (resource, persisted_tools)
        };

        self.persist_state(&state)?;
        self.persist_tool_map(&server_id, &persisted_tools)?;
        let mut start_payload = Map::new();
        start_payload.insert("serverId".to_string(), Value::String(server_id.clone()));
        start_payload.insert("status".to_string(), Value::String(state.status.as_str().to_string()));
        start_payload.insert("executionId".to_string(), Value::String(execution_id.clone()));
        start_payload.insert("sessionId".to_string(), Value::String(session_id.clone()));
        start_payload.insert("toolCount".to_string(), Value::Number(persisted_tools.len().into()));
        start_payload.insert(
            "transportKind".to_string(),
            Value::String(server.transport_kind.as_str().to_string()),
        );
        self.publish_event(
            "mcp",
            "mcp.server_started",
            Resource {
                kind: RESOURCE_KIND_SERVER.to_string(),
                id: server_id.clone(),
            },
            start_payload,
        )?;
        self.publish_health_changed(&server_id, state.status, &state.health_reason)?;
        if restore_request {
            let mut restore_payload = Map::new();
            restore_payload.insert("serverId".to_string(), Value::String(server_id.clone()));
            restore_payload.insert(
                "transportKind".to_string(),
                Value::String(server.transport_kind.as_str().to_string()),
            );
            restore_payload.insert("sessionId".to_string(), Value::String(session_id.clone()));
            restore_payload.insert("toolCount".to_string(), Value::Number(persisted_tools.len().into()));
            self.publish_event(
                "mcp",
                "mcp.server_restore_completed",
                Resource {
                    kind: RESOURCE_KIND_SERVER.to_string(),
                    id: server_id.clone(),
                },
                restore_payload,
            )?;
        }

        let this = self.clone();
        let watcher_server_id = server_id.clone();
        let watcher_execution_id = execution_id.clone();
        std::thread::spawn(move || this.watch_session(&watcher_server_id, &watcher_execution_id, session));

        let resource = self.get_server_resource(&server_id).unwrap_or_default();
        Ok(LifecycleResponse {
            action: LifecycleAction::Start,
            server: resource,
            execution_id: execution_id.clone(),
            preflight_ms: started_at.elapsed().as_millis() as i64,
            idempotent: false,
            failure_class: String::new(),
            ..LifecycleResponse::default()
        })
    }

    /// Go `Stop`.
    pub fn stop(&self, server_id: &str) -> Result<LifecycleResponse, McpError> {
        self.stop_or_cancel(server_id, false)
    }

    /// Go `Cancel`.
    pub fn cancel(&self, server_id: &str) -> Result<LifecycleResponse, McpError> {
        self.stop_or_cancel(server_id, true)
    }

    /// Go `Restart`.
    pub fn restart(&self, server_id: &str, requested_by: &str) -> Result<LifecycleResponse, McpError> {
        match self.stop_or_cancel(server_id, false) {
            Ok(_) => {}
            Err(McpError::ServerNotFound) => {}
            Err(err) => return Err(err),
        }
        let mut response = self.start(server_id, requested_by)?;
        response.action = LifecycleAction::Restart;
        Ok(response)
    }

    // ---------------------------------------------------------------------------
    // Internal helpers
    // ---------------------------------------------------------------------------

    /// Go `watchSession`: processes one session termination (blocking on the session's
    /// done receiver), then reconciles state and schedules restarts/reconnects. Runs on
    /// a detached thread from `start`.
    pub fn watch_session(&self, server_id: &str, execution_id: &str, session: Arc<dyn Session>) {
        let done = session.done().recv().unwrap_or(Ok(()));
        let (active, server, state, stop_requested, cancel_requested, transport_kind) = {
            let mut guard = self.inner.state.write();
            let active = guard.sessions.get(server_id).cloned();
            if let Some(active_ref) = &active {
                if let Some(active_session) = &active_ref.session {
                    if Arc::ptr_eq(active_session, &session) {
                        guard.sessions.remove(server_id);
                    }
                }
            }
            let server = guard.servers.get(server_id).cloned();
            let state = guard.states.get(server_id).cloned().unwrap_or_default();
            let stop_requested = active.as_ref().is_some_and(|a| a.stop_requested);
            let cancel_requested = active.as_ref().is_some_and(|a| a.cancel_requested);
            let transport_kind = active
                .as_ref()
                .map(|a| a.transport_kind)
                .unwrap_or(TransportKind::Stdio);
            (active, server, state, stop_requested, cancel_requested, transport_kind)
        };
        let _ = active;
        let Some(server) = server else {
            return;
        };
        let mut state = state;
        if execution_id.is_empty() {
            if stop_requested || cancel_requested {
                let now = Utc::now();
                state.status = LifecycleStatus::Stopped;
                state.last_stopped_at = Some(now);
                state.updated_at = now;
                self.inner.state.write().states.insert(server_id.to_string(), state.clone());
                let _ = self.persist_state(&state);
                let _ = self.publish_health_changed(server_id, state.status, &state.health_reason);
                return;
            }
            if let Err(err) = &done {
                if transport_kind == TransportKind::Websocket && server.enabled && server.auto_restart {
                    self.schedule_websocket_reconnect(server_id, &state, Some(err));
                    return;
                }
                self.record_failure(
                    server_id,
                    &mut state,
                    LifecycleStatus::Failed,
                    err,
                    "transport_runtime_failure",
                );
            }
            if state.status == LifecycleStatus::Failed && server.enabled && server.auto_restart {
                self.schedule_restart(server_id, &state);
            }
            return;
        }

        if let Some(sandboxes) = &self.inner.sandboxes {
            if let Some(execution) = sandboxes.get_execution(execution_id) {
                self.update_state_from_execution(
                    server_id,
                    &mut state,
                    &execution,
                    stop_requested || cancel_requested,
                );
            } else if done.is_err() {
                self.record_failure(
                    server_id,
                    &mut state,
                    LifecycleStatus::Failed,
                    &crate::error_string(done.as_ref().err()),
                    "transport_runtime_failure",
                );
            }
        }
        if state.status == LifecycleStatus::Failed && server.enabled && server.auto_restart {
            self.schedule_restart(server_id, &state);
        }
    }

    /// Go `scheduleRestart`: marks the server backing-off and starts a detached timer
    /// thread that re-starts it after the backoff delay.
    fn schedule_restart(&self, server_id: &str, state: &ServerState) {
        let delay = mcp_backoff_delay(state.failure_count);
        let next = Utc::now() + chrono::Duration::from_std(delay).unwrap_or_default();
        let mut state = state.clone();
        state.status = LifecycleStatus::BackingOff;
        state.next_restart_at = Some(next);
        state.updated_at = Utc::now();
        self.inner.state.write().states.insert(server_id.to_string(), state.clone());
        let _ = self.persist_state(&state);
        let _ = self.publish_health_changed(server_id, state.status, &state.health_reason);

        let this = self.clone();
        let watcher_server_id = server_id.to_string();
        std::thread::spawn(move || {
            std::thread::sleep(delay);
            let _ = this.start(&watcher_server_id, "mcp.auto_restart");
        });
    }

    /// Go `scheduleWebsocketReconnect`: bounded daemon-managed reconnect with backoff.
    fn schedule_websocket_reconnect(&self, server_id: &str, state: &ServerState, cause: Option<&String>) {
        let now = Utc::now();
        let attempt = state.reconnect_attempt_count + 1;
        let reason = first_non_empty(&[
            crate::error_string(cause).as_str(),
            state.health_reason.as_str(),
            "websocket session disconnected",
        ]);
        if attempt > WEBSOCKET_RECONNECT_MAX_ATTEMPTS {
            let mut state = state.clone();
            state.status = LifecycleStatus::Failed;
            state.health_reason = reason.clone();
            state.last_recovery_at = Some(now);
            state.last_recovery_class = "reconnect_failed".to_string();
            state.next_reconnect_at = None;
            state.updated_at = now;
            self.inner.state.write().states.insert(server_id.to_string(), state.clone());
            let _ = self.persist_state(&state);
            let mut payload = Map::new();
            payload.insert("serverId".to_string(), Value::String(server_id.to_string()));
            payload.insert(
                "transportKind".to_string(),
                Value::String(TransportKind::Websocket.as_str().to_string()),
            );
            payload.insert(
                "attempt".to_string(),
                Value::Number(state.reconnect_attempt_count.into()),
            );
            payload.insert("reason".to_string(), Value::String(reason.clone()));
            payload.insert("failureClass".to_string(), Value::String("reconnect_exhausted".to_string()));
            let _ = self.publish_event(
                "mcp",
                "mcp.server_reconnect_failed",
                Resource {
                    kind: RESOURCE_KIND_SERVER.to_string(),
                    id: server_id.to_string(),
                },
                payload,
            );
            let _ = self.publish_health_changed(server_id, state.status, &state.health_reason);
            return;
        }

        let delay = mcp_backoff_delay(attempt);
        let next = now + chrono::Duration::from_std(delay).unwrap_or_default();
        let mut state = state.clone();
        state.status = LifecycleStatus::Degraded;
        state.health_reason = reason.clone();
        state.reconnect_attempt_count = attempt;
        state.last_recovery_at = Some(now);
        state.last_recovery_class = "reconnect_scheduled".to_string();
        state.next_reconnect_at = Some(next);
        state.updated_at = now;
        self.inner.state.write().states.insert(server_id.to_string(), state.clone());
        let _ = self.persist_state(&state);
        let mut payload = Map::new();
        payload.insert("serverId".to_string(), Value::String(server_id.to_string()));
        payload.insert(
            "transportKind".to_string(),
            Value::String(TransportKind::Websocket.as_str().to_string()),
        );
        payload.insert("attempt".to_string(), Value::Number(attempt.into()));
        payload.insert("reason".to_string(), Value::String(reason.clone()));
        payload.insert(
            "nextRetryAt".to_string(),
            Value::String(rfc3339_nano(next)),
        );
        let _ = self.publish_event(
            "mcp",
            "mcp.server_reconnect_scheduled",
            Resource {
                kind: RESOURCE_KIND_SERVER.to_string(),
                id: server_id.to_string(),
            },
            payload,
        );
        let _ = self.publish_health_changed(server_id, state.status, &state.health_reason);

        let this = self.clone();
        let watcher_server_id = server_id.to_string();
        let expected_attempt = attempt;
        std::thread::spawn(move || {
            std::thread::sleep(delay);
            let response = this
                .start(&watcher_server_id, "mcp.websocket_reconnect")
                .unwrap_or_default();
            if response.server.state.status == LifecycleStatus::Healthy {
                let recovered_at = Utc::now();
                let mut latest = {
                    let guard = this.inner.state.read();
                    guard.states.get(&watcher_server_id).cloned().unwrap_or_default()
                };
                latest.last_recovery_at = Some(recovered_at);
                latest.last_recovery_class = "reconnect_succeeded".to_string();
                latest.reconnect_attempt_count = 0;
                latest.next_reconnect_at = None;
                latest.updated_at = recovered_at;
                this.inner
                    .state
                    .write()
                    .states
                    .insert(watcher_server_id.clone(), latest.clone());
                let _ = this.persist_state(&latest);
                let mut payload = Map::new();
                payload.insert("serverId".to_string(), Value::String(watcher_server_id.clone()));
                payload.insert(
                    "transportKind".to_string(),
                    Value::String(TransportKind::Websocket.as_str().to_string()),
                );
                payload.insert("attempt".to_string(), Value::Number(expected_attempt.into()));
                payload.insert(
                    "sessionId".to_string(),
                    Value::String(latest.last_session_id.clone()),
                );
                let _ = this.publish_event(
                    "mcp",
                    "mcp.server_reconnect_completed",
                    Resource {
                        kind: RESOURCE_KIND_SERVER.to_string(),
                        id: watcher_server_id.clone(),
                    },
                    payload,
                );
                return;
            }
            let resource = this.get_server_resource(&watcher_server_id).unwrap_or_default();
            if !resource.server.enabled || !resource.server.auto_restart {
                return;
            }
            let cause = first_non_empty(&[
                response.blocked_reason.as_str(),
                resource.server.state.health_reason.as_str(),
                response.failure_class.as_str(),
                "websocket reconnect failed",
            ]);
            this.schedule_websocket_reconnect(&watcher_server_id, &resource.state, Some(&cause));
        });
    }
}
