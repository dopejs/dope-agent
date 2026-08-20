//! Behavioral tests for the Web API REST transport (port of
//! transport_webapi_test.go): threaded replies, error classification, network
//! failures, and installation validation against an injected HTTP client.

use std::sync::Arc;

use kura_connectors::DiagnosticReasonCode;
use kura_imtypes::OutboundReply;
use kura_slack::diagnostics::diagnostic_reason_for_error;
use kura_slack::readiness::WorkspaceBinding;
use kura_slack::transport::Transport;
use kura_slack::transport_webapi::{
    SlackHttpClient, WebApiError, WebApiTransport, WebApiTransportConfig,
};

/// Handler signature for the injected HTTP client (Go's
/// roundTripFunc/http.Client injection point).
type Handler =
    dyn Fn(&str, &str, &serde_json::Value) -> Result<(u16, Vec<u8>), WebApiError> + Send + Sync;

/// Test double HTTP client driven by a handler closure.
struct FakeHttpClient {
    handler: Box<Handler>,
}

impl SlackHttpClient for FakeHttpClient {
    fn post(
        &self,
        url: &str,
        token: &str,
        payload: &serde_json::Value,
    ) -> Result<(u16, Vec<u8>), WebApiError> {
        (self.handler)(url, token, payload)
    }
}

fn transport_with(
    handler: impl Fn(&str, &str, &serde_json::Value) -> Result<(u16, Vec<u8>), WebApiError>
    + Send
    + Sync
    + 'static,
) -> WebApiTransport {
    WebApiTransport::new(WebApiTransportConfig {
        connector_id: "slack-main".to_string(),
        base_url: "https://slack.test".to_string(),
        bot_token: "xoxb-redacted".to_string(),
        http_client: Some(Arc::new(FakeHttpClient {
            handler: Box::new(handler),
        })),
        ..WebApiTransportConfig::default()
    })
}

#[test]
fn web_api_transport_sends_threaded_reply() {
    let transport = transport_with(|url, auth, payload| {
        assert!(
            url.ends_with("/api/chat.postMessage"),
            "unexpected path {url}"
        );
        // The client receives the resolved token and adds the Bearer prefix
        // itself (matching Go's post: Authorization: Bearer <token>).
        assert_eq!(auth, "xoxb-redacted", "unexpected authorization token");
        let body = payload.as_object().expect("payload must be an object");
        assert_eq!(body.get("channel").and_then(|v| v.as_str()), Some("C123"));
        assert_eq!(body.get("text").and_then(|v| v.as_str()), Some("hello"));
        assert_eq!(
            body.get("thread_ts").and_then(|v| v.as_str()),
            Some("171.0001")
        );
        Ok((
            200,
            serde_json::to_vec(&serde_json::json!({"ok": true, "ts": "171.0002"})).expect("encode"),
        ))
    });
    let sent = transport
        .send_reply(&OutboundReply {
            connector_id: "slack-main".to_string(),
            channel_id: "C123".to_string(),
            content: "hello".to_string(),
            reply_to_external_message_id: "171.0001".to_string(),
        })
        .expect("send reply");
    assert_eq!(
        sent.external_message_id, "171.0002",
        "expected Slack ts as external message id"
    );
}

#[test]
fn web_api_transport_classifies_provider_errors() {
    let transport = transport_with(|_, _, _| {
        Ok((
            200,
            serde_json::to_vec(&serde_json::json!({"ok": false, "error": "missing_scope"}))
                .expect("encode"),
        ))
    });
    let err = transport
        .send_reply(&OutboundReply {
            connector_id: "slack-main".to_string(),
            channel_id: "C123".to_string(),
            content: "hello".to_string(),
            ..OutboundReply::default()
        })
        .expect_err("expected Slack API error");
    assert_eq!(
        diagnostic_reason_for_error(&err),
        DiagnosticReasonCode::PermissionMissing,
        "got {err}"
    );
}

#[test]
fn web_api_transport_classifies_network_failure() {
    let transport = transport_with(|_, _, _| {
        Err(WebApiError {
            status_code: 0,
            code: "network_failed".to_string(),
            message: "connection reset by peer".to_string(),
        })
    });
    let err = transport
        .send_reply(&OutboundReply {
            connector_id: "slack-main".to_string(),
            channel_id: "C123".to_string(),
            content: "hello".to_string(),
            ..OutboundReply::default()
        })
        .expect_err("expected Slack network error");
    assert_eq!(
        diagnostic_reason_for_error(&err),
        DiagnosticReasonCode::NetworkFailed,
        "got {err}"
    );
}

#[test]
fn web_api_transport_validates_installation() {
    let transport = transport_with(|url, _, _| {
        assert!(url.ends_with("/api/auth.test"), "unexpected path {url}");
        Ok((
            200,
            serde_json::to_vec(&serde_json::json!({
                "ok": true,
                "team_id": "T123",
                "team": "Test Workspace",
                "bot_id": "B123",
                "user_id": "Ubot",
            }))
            .expect("encode"),
        ))
    });
    let mut binding = WorkspaceBinding {
        tenant_id: "ten_slack".to_string(),
        connector_id: "slack-main".to_string(),
        ..WorkspaceBinding::default()
    };
    transport
        .validate_installation(&mut binding)
        .expect("validate installation");
    assert_eq!(binding.workspace_id, "T123");
    assert_eq!(binding.workspace_label, "Test Workspace");
    assert_eq!(binding.installation_id, "B123");
    assert_eq!(binding.oauth_grant_state, "valid");
    assert_eq!(binding.required_scope_state, "valid");
}

#[test]
fn web_api_transport_reply_capabilities_are_final_only() {
    let transport = WebApiTransport::new(WebApiTransportConfig {
        connector_id: "slack-main".to_string(),
        base_url: "https://slack.test".to_string(),
        bot_token: "xoxb-redacted".to_string(),
        ..WebApiTransportConfig::default()
    });
    let caps = transport.reply_capabilities();
    assert_eq!(caps.max_message_length, 40000);
    assert!(!caps.supports_streaming, "slack replies are final-only");
    assert!(!caps.supports_thinking);
    assert!(transport.start(Box::new(|_| {})).is_ok());
    assert!(transport.close().is_ok());
}
