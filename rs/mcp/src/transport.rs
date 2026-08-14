//! Port of the MCP client transport layer (transport.go / streamable_http.go /
//! websocket.go).
//!
//! The transport/session traits, the JSON-RPC wire types, the stdio
//! content-length framing helper, the shared tools/list decoding, and the transport
//! mux are ported faithfully. The concrete transports are DEFERRED: the stdio transport
//! needs the sandbox `AttachedExecution` process pipes (not yet in dope-sandbox), the
//! streamable-http transport needs an HTTP client binding, and the websocket transport
//! needs a websocket client crate (not in the workspace). A `TransportMux` constructed
//! without concrete transports returns `ErrTransportUnavailable` for every kind,
//! mirroring the Go mux's behavior for an unavailable transport.

use std::io::{BufRead, Read, Write};
use std::sync::Arc;
use std::sync::mpsc::Receiver;
use std::time::Duration;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::types::{DiscoveryStatus, Server, Tool, TransportKind};
use crate::McpError;

/// MCP protocol version used for session initialization (Go hard-codes this).
pub const MCP_PROTOCOL_VERSION: &str = "2024-11-05";

/// Stdio subprocess pipes handed to a stdio transport (Go SessionPipes).
#[derive(Debug, Clone, Default)]
pub struct SessionPipes {
    pub stdin: Option<Box<dyn Write + Send>>,
    pub stdout: Option<Box<dyn Read + Send>>,
    pub stderr: Option<Box<dyn Read + Send>>,
}

impl SessionPipes {
    /// Whether both stdin and stdout are present (Go `stdioTransport.Open` rejects a
    /// session without them).
    #[must_use]
    pub fn has_stdio(&self) -> bool {
        self.stdin.is_some() && self.stdout.is_some()
    }
}

/// Opens an MCP session for a server (Go Transport interface).
pub trait Transport: Send + Sync {
    fn open(
        &self,
        server: &Server,
        pipes: SessionPipes,
        timeout: Duration,
    ) -> Result<Arc<dyn Session>, McpError>;
}

/// An open MCP client session (Go Session interface). `done` yields the terminal
/// result once (equivalent to Go `Done() <-chan error`; a disconnected channel is an
/// implicit nil/clean close).
pub trait Session: Send + Sync {
    fn id(&self) -> String;
    fn list_tools(&self, timeout: Duration) -> Result<Vec<Tool>, String>;
    fn call_tool(
        &self,
        tool_name: &str,
        input: Value,
    ) -> Result<serde_json::Map<String, Value>, String>;
    fn close(&self) -> Result<(), String>;
    fn done(&self) -> &Receiver<Result<(), String>>;
}

/// JSON-RPC request wire type (Go rpcRequest).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RpcRequest {
    pub jsonrpc: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub id: String,
    pub method: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub params: Option<Value>,
}

/// JSON-RPC response wire type (Go rpcResponse).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RpcResponse {
    pub jsonrpc: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub id: String,
    #[serde(default)]
    pub result: Option<Value>,
    #[serde(default)]
    pub error: Option<RpcError>,
}

/// JSON-RPC error body (Go rpcError).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RpcError {
    pub code: i64,
    pub message: String,
}

/// Go `stdioSession.initialize` params: protocol version, empty capabilities, and the
/// dope-daemon client info.
#[must_use]
pub fn initialize_params() -> Value {
    serde_json::json!({
        "protocolVersion": MCP_PROTOCOL_VERSION,
        "capabilities": {},
        "clientInfo": { "name": "dope-daemon", "version": "dev" },
    })
}

/// Go `schemaFingerprint`: sha256 hex of the JSON-serialized input schema (empty for
/// an empty schema).
#[must_use]
pub fn schema_fingerprint(value: &Value) -> String {
    let empty = match value {
        Value::Null => true,
        Value::Object(map) => map.is_empty(),
        _ => false,
    };
    if empty {
        return String::new();
    }
    let Ok(payload) = serde_json::to_string(value) else {
        return String::new();
    };
    let mut hasher = sha2::Sha256::new();
    hasher.update(payload.as_bytes());
    format!("{:x}", hasher.finalize())
}

/// Go `normalizeToolArguments`: nil input becomes an empty object.
#[must_use]
pub fn normalize_tool_arguments(input: Value) -> Value {
    if input.is_null() {
        serde_json::json!({})
    } else {
        input
    }
}

/// Decodes a `tools/list` response payload into discovered tools (Go payload struct in
/// `ListTools`).
pub fn decode_tools_list(raw: &Value, server_id: &str, now: DateTime<Utc>) -> Result<Vec<Tool>, String> {
    #[derive(Deserialize)]
    struct ToolsListTool {
        #[serde(default)]
        name: String,
        #[serde(default)]
        title: String,
        #[serde(default)]
        description: String,
        #[serde(default, rename = "inputSchema")]
        input_schema: Value,
    }
    #[derive(Deserialize)]
    struct ToolsListPayload {
        #[serde(default)]
        tools: Vec<ToolsListTool>,
    }
    let payload: ToolsListPayload =
        serde_json::from_value(raw.clone()).map_err(|e| format!("decode tools/list response: {e}"))?;
    let mut tools = Vec::with_capacity(payload.tools.len());
    for item in payload.tools {
        tools.push(Tool {
            server_id: server_id.to_string(),
            tool_name: item.name.trim().to_string(),
            title: item.title.trim().to_string(),
            description: item.description.trim().to_string(),
            schema_fingerprint: schema_fingerprint(&item.input_schema),
            discovery_status: DiscoveryStatus::Discovered,
            last_discovered_at: Some(now),
            updated_at: now,
            ..Tool::default()
        });
    }
    Ok(tools)
}

/// Go `readFramedMessage`: reads one LSP-style `Content-Length` framed message from
/// a stdio reader.
pub fn read_framed_message(reader: &mut impl BufRead) -> Result<Vec<u8>, String> {
    let mut length: i64 = -1;
    loop {
        let mut line = String::new();
        let read = reader.read_line(&mut line).map_err(|e| e.to_string())?;
        if read == 0 {
            return Err("read framed message: unexpected EOF".to_string());
        }
        let trimmed = line.trim_end_matches(['\r', '\n']).to_string();
        if trimmed.is_empty() {
            break;
        }
        if !trimmed.to_lowercase().starts_with("content-length:") {
            continue;
        }
        let value = if let Some(rest) = trimmed.strip_prefix("Content-Length:") {
            rest.trim().to_string()
        } else if let Some(rest) = trimmed.strip_prefix("content-length:") {
            rest.trim().to_string()
        } else {
            continue;
        };
        let parsed: i64 = value
            .parse()
            .map_err(|_| format!("parse content length: {value}"))?;
        length = parsed;
    }
    if length < 0 {
        return Err("missing content length header".to_string());
    }
    let mut payload = vec![0u8; length as usize];
    reader.read_exact(&mut payload).map_err(|e| e.to_string())?;
    Ok(payload)
}

/// Go `NewTransportMux` default: a mux whose concrete transports are not installed.
/// Without concrete transports every kind is unavailable (the concrete stdio /
/// streamable-http / websocket transports are deferred).
#[derive(Default)]
pub struct TransportMux {
    pub stdio: Option<Arc<dyn Transport>>,
    pub remote: Option<Arc<dyn Transport>>,
    pub websocket: Option<Arc<dyn Transport>>,
}

impl TransportMux {
    /// Go `NewTransportMux`. Unlike Go (which substitutes defaults), a `None`
    /// transport stays unavailable so the deferral is observable.
    #[must_use]
    pub fn new(
        stdio: Option<Arc<dyn Transport>>,
        remote: Option<Arc<dyn Transport>>,
        websocket: Option<Arc<dyn Transport>>,
    ) -> Self {
        TransportMux { stdio, remote, websocket }
    }
}

impl Transport for TransportMux {
    fn open(
        &self,
        server: &Server,
        pipes: SessionPipes,
        timeout: Duration,
    ) -> Result<Arc<dyn Session>, McpError> {
        let selected = match server.transport_kind {
            TransportKind::Stdio => self.stdio.as_ref(),
            TransportKind::StreamableHTTP => self.remote.as_ref(),
            TransportKind::Websocket => self.websocket.as_ref(),
        };
        match selected {
            Some(transport) => transport.open(server, pipes, timeout),
            None => Err(McpError::TransportUnavailable),
        }
    }
}
