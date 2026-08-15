//! Minimal stdio MCP server used by the dope-mcp transport tests: reads
//! LSP-style `Content-Length` framed JSON-RPC requests from stdin and answers
//! `initialize`, `tools/list`, and `tools/call` (a single "echo" tool) on stdout.
//! Notifications (no id) are ignored. Exits when stdin closes.

use std::io::{BufReader, Write};

use dope_mcp::transport::{RpcRequest, RpcResponse, read_framed_message};
use serde_json::json;

fn main() {
    let stdin = std::io::stdin();
    let mut reader = BufReader::new(stdin.lock());
    let mut stdout = std::io::stdout();
    loop {
        let payload = match read_framed_message(&mut reader) {
            Ok(payload) => payload,
            Err(_) => break,
        };
        let request: RpcRequest = match serde_json::from_slice(&payload) {
            Ok(request) => request,
            Err(_) => continue,
        };
        if request.id.trim().is_empty() {
            continue;
        }
        let response = respond(&request);
        let body = match serde_json::to_vec(&response) {
            Ok(body) => body,
            Err(_) => continue,
        };
        let frame = format!("Content-Length: {}\r\n\r\n", body.len());
        if stdout
            .write_all(frame.as_bytes())
            .and_then(|_| stdout.write_all(&body))
            .and_then(|_| stdout.flush())
            .is_err()
        {
            break;
        }
    }
}

fn respond(request: &RpcRequest) -> RpcResponse {
    let id = request.id.clone();
    match request.method.as_str() {
        "initialize" => RpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: Some(json!({
                "protocolVersion": "2024-11-05",
                "capabilities": { "tools": { "listChanged": false } },
                "serverInfo": { "name": "fake-mcp-server", "version": "0.1.0" },
            })),
            error: None,
        },
        "tools/list" => RpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: Some(json!({
                "tools": [{
                    "name": "echo",
                    "title": "Echo",
                    "description": "Echoes the message argument back",
                    "inputSchema": {
                        "type": "object",
                        "properties": { "message": { "type": "string" } },
                        "required": ["message"],
                    },
                }],
            })),
            error: None,
        },
        "tools/call" => {
            let arguments = request
                .params
                .as_ref()
                .and_then(|params| params.get("arguments"))
                .cloned()
                .unwrap_or_else(|| json!({}));
            let text = arguments
                .get("message")
                .and_then(|value| value.as_str())
                .unwrap_or("")
                .to_string();
            RpcResponse {
                jsonrpc: "2.0".to_string(),
                id,
                result: Some(json!({
                    "content": [{ "type": "text", "text": text }],
                    "isError": false,
                })),
                error: None,
            }
        }
        _ => RpcResponse {
            jsonrpc: "2.0".to_string(),
            id,
            result: None,
            error: Some(dope_mcp::transport::RpcError {
                code: -32601,
                message: format!("method not found: {}", request.method),
            }),
        },
    }
}
