//! Slack Web API REST transport (port of transport_webapi.go): REST via ureq —
//! chat.postMessage / auth.test / conversations.info, bot-token resolution,
//! and the classified Web API error type.

use std::io::Read;
use std::sync::Arc;
use std::time::Duration;

use dope_connectors::{DiagnosticReasonCode, RedactionStatus};
use dope_imtypes::{OutboundReply, ReplyCapabilities, SentReply};
use serde::de::DeserializeOwned;

use crate::destinations::{
    ConversationType, RoutePolicy, RouteValidationState, normalize_route_policy,
};
use crate::error::SlackError;
use crate::readiness::WorkspaceBinding;
use crate::route::InboundEvent;
use crate::transport::Transport;
use crate::util::{first_non_empty, is_unset_time};

/// Slack Web API failure (Go `WebAPIError`): an HTTP status code and/or a
/// Slack error code, classified into a diagnostic reason via
/// [WebApiError::error_class].
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct WebApiError {
    pub status_code: u16,
    pub code: String,
    pub message: String,
}

impl std::fmt::Display for WebApiError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if !self.message.trim().is_empty() {
            return f.write_str(&self.message);
        }
        if !self.code.trim().is_empty() {
            return write!(f, "slack web api error: {}", self.code.trim());
        }
        if self.status_code > 0 {
            return write!(f, "slack web api status {}", self.status_code);
        }
        f.write_str("slack web api error")
    }
}

impl std::error::Error for WebApiError {}

impl WebApiError {
    /// Go `ErrorClass`: maps the Slack error code / HTTP status to a
    /// diagnostic reason code.
    #[must_use]
    pub fn error_class(&self) -> DiagnosticReasonCode {
        match self.code.trim() {
            "invalid_auth" | "not_authed" | "token_revoked" | "account_inactive" => {
                DiagnosticReasonCode::AuthMissing
            }
            "missing_scope" | "not_in_channel" | "channel_not_found" | "is_archived"
            | "restricted_action" => DiagnosticReasonCode::PermissionMissing,
            "ratelimited" | "rate_limited" => DiagnosticReasonCode::RateLimited,
            "network_failed" => DiagnosticReasonCode::NetworkFailed,
            _ => {
                if self.status_code == 429 {
                    DiagnosticReasonCode::RateLimited
                } else if self.status_code > 0 {
                    // Go maps both 5xx and any other non-2xx status to
                    // provider_unavailable.
                    DiagnosticReasonCode::ProviderUnavailable
                } else {
                    DiagnosticReasonCode::UnknownConnectorFailure
                }
            }
        }
    }
}

/// Resolves the connector's bot token on demand (Go `BotTokenProvider`).
pub trait BotTokenProvider: Send + Sync {
    fn bot_token(&self, connector_id: &str) -> Result<String, SlackError>;
}

/// Minimal HTTP client boundary for the Web API transport: posts a JSON body
/// with bearer auth and returns the raw HTTP status and body. Transport-level
/// failures surface as [WebApiError] with code `network_failed`.
pub trait SlackHttpClient: Send + Sync {
    fn post(
        &self,
        url: &str,
        token: &str,
        payload: &serde_json::Value,
    ) -> Result<(u16, Vec<u8>), WebApiError>;
}

/// Default [SlackHttpClient] backed by ureq (blocking, like Go's
/// http.Client with a 10s timeout).
#[derive(Clone, Debug)]
pub struct UreqHttpClient {
    timeout: Duration,
}

impl Default for UreqHttpClient {
    fn default() -> Self {
        UreqHttpClient {
            timeout: Duration::from_secs(10),
        }
    }
}

impl SlackHttpClient for UreqHttpClient {
    fn post(
        &self,
        url: &str,
        token: &str,
        payload: &serde_json::Value,
    ) -> Result<(u16, Vec<u8>), WebApiError> {
        let body = serde_json::to_vec(payload).map_err(|e| WebApiError {
            status_code: 0,
            code: "request_encode_failed".to_string(),
            message: format!("encode slack request body: {e}"),
        })?;
        let response = ureq::post(url)
            .set("Authorization", &format!("Bearer {token}"))
            .set("Content-Type", "application/json")
            .timeout(self.timeout)
            .send_bytes(&body);
        match response {
            Ok(response) => {
                let status = response.status();
                let mut buf = Vec::new();
                let _ = response.into_reader().read_to_end(&mut buf);
                Ok((status, buf))
            }
            Err(ureq::Error::Status(status, response)) => {
                let mut buf = Vec::new();
                let _ = response.into_reader().read_to_end(&mut buf);
                Ok((status, buf))
            }
            Err(ureq::Error::Transport(transport)) => Err(WebApiError {
                status_code: 0,
                code: "network_failed".to_string(),
                message: transport.to_string(),
            }),
        }
    }
}

/// Configuration for [WebApiTransport] (Go `WebAPITransportConfig`).
#[derive(Clone, Default)]
pub struct WebApiTransportConfig {
    pub connector_id: String,
    pub base_url: String,
    pub bot_token: String,
    pub token_provider: Option<Arc<dyn BotTokenProvider>>,
    pub http_client: Option<Arc<dyn SlackHttpClient>>,
}

/// REST transport over the Slack Web API (Go `WebAPITransport`).
pub struct WebApiTransport {
    connector_id: String,
    base_url: String,
    bot_token: String,
    token_provider: Option<Arc<dyn BotTokenProvider>>,
    http_client: Arc<dyn SlackHttpClient>,
}

impl WebApiTransport {
    /// Go `NewWebAPITransport`.
    #[must_use]
    pub fn new(cfg: WebApiTransportConfig) -> Self {
        let base_url = cfg.base_url.trim().trim_end_matches('/').to_string();
        let base_url = if base_url.is_empty() {
            "https://slack.com".to_string()
        } else {
            base_url
        };
        WebApiTransport {
            connector_id: cfg.connector_id.trim().to_string(),
            base_url,
            bot_token: cfg.bot_token.trim().to_string(),
            token_provider: cfg.token_provider,
            http_client: cfg
                .http_client
                .unwrap_or_else(|| Arc::new(UreqHttpClient::default())),
        }
    }

    /// Go `SendReply`: posts chat.postMessage (optionally threaded) and
    /// returns the Slack timestamp as the external message id.
    pub fn send_reply(&self, reply: &OutboundReply) -> Result<SentReply, SlackError> {
        if reply.channel_id.trim().is_empty() {
            return Err(SlackError::Message(
                "slack channel id is required".to_string(),
            ));
        }
        let mut payload = serde_json::Map::new();
        payload.insert(
            "channel".to_string(),
            serde_json::Value::String(reply.channel_id.trim().to_string()),
        );
        payload.insert(
            "text".to_string(),
            serde_json::Value::String(reply.content.trim().to_string()),
        );
        if !reply.reply_to_external_message_id.trim().is_empty() {
            payload.insert(
                "thread_ts".to_string(),
                serde_json::Value::String(reply.reply_to_external_message_id.trim().to_string()),
            );
        }
        #[derive(serde::Deserialize)]
        struct PostMessageResponse {
            #[serde(default)]
            ok: bool,
            #[serde(default)]
            error: String,
            #[serde(default)]
            ts: String,
        }
        let response: PostMessageResponse =
            self.post("chat.postMessage", &serde_json::Value::Object(payload))?;
        if !response.ok {
            return Err(SlackError::WebApi(WebApiError {
                status_code: 0,
                code: response.error,
                message: String::new(),
            }));
        }
        Ok(SentReply {
            external_message_id: response.ts.trim().to_string(),
        })
    }

    /// Go `ValidateInstallation`: auth.test against the resolved bot token,
    /// filling the workspace binding from the verified identity.
    pub fn validate_installation(&self, binding: &mut WorkspaceBinding) -> Result<(), SlackError> {
        #[derive(serde::Deserialize)]
        struct AuthTestResponse {
            #[serde(default)]
            ok: bool,
            #[serde(default)]
            error: String,
            #[serde(default)]
            team_id: String,
            #[serde(default)]
            team: String,
            #[serde(default)]
            bot_id: String,
            #[serde(default)]
            user_id: String,
        }
        let response: AuthTestResponse = self.post("auth.test", &serde_json::json!({}))?;
        if !response.ok {
            return Err(SlackError::WebApi(WebApiError {
                status_code: 0,
                code: response.error,
                message: String::new(),
            }));
        }
        binding.workspace_id = first_non_empty(&[&binding.workspace_id, &response.team_id]);
        binding.workspace_label = first_non_empty(&[&binding.workspace_label, &response.team]);
        binding.installation_id = first_non_empty(&[
            &binding.installation_id,
            &response.bot_id,
            &response.user_id,
        ]);
        binding.oauth_grant_state = "valid".to_string();
        binding.required_scope_state = "valid".to_string();
        if is_unset_time(&binding.validated_at) {
            binding.validated_at = chrono::Utc::now();
        }
        if binding.redaction_status == RedactionStatus::default() {
            binding.redaction_status = RedactionStatus::Redacted;
        }
        Ok(())
    }

    /// Go `ValidateRoutePolicy`: verifies every selected channel via
    /// conversations.info and stamps the policy valid.
    pub fn validate_route_policy(&self, policy: &RoutePolicy) -> Result<RoutePolicy, SlackError> {
        let mut policy = normalize_route_policy(policy.clone(), chrono::Utc::now());
        for channel in &policy.selected_channels {
            if channel.conversation_type != ConversationType::Channel
                || channel.conversation_id.trim().is_empty()
            {
                continue;
            }
            #[derive(serde::Deserialize)]
            struct ConversationsInfoResponse {
                #[serde(default)]
                ok: bool,
                #[serde(default)]
                error: String,
            }
            let response: ConversationsInfoResponse = self.post(
                "conversations.info",
                &serde_json::json!({ "channel": channel.conversation_id }),
            )?;
            if !response.ok {
                return Err(SlackError::WebApi(WebApiError {
                    status_code: 0,
                    code: response.error,
                    message: String::new(),
                }));
            }
        }
        policy.validation_state = RouteValidationState::Valid;
        Ok(policy)
    }

    /// Post one JSON Web API method (Go `post`): resolves the token, posts,
    /// and rejects non-2xx responses as classified errors.
    fn post<T: DeserializeOwned>(
        &self,
        method: &str,
        payload: &serde_json::Value,
    ) -> Result<T, SlackError> {
        let token = self.resolve_token()?;
        let url = format!("{}/api/{}", self.base_url, method.trim());
        let (status, body) = self.http_client.post(&url, &token, payload)?;
        if !(200..300).contains(&status) {
            return Err(SlackError::WebApi(WebApiError {
                status_code: status,
                code: String::new(),
                message: String::new(),
            }));
        }
        serde_json::from_slice(&body)
            .map_err(|e| SlackError::Message(format!("decode slack response: {e}")))
    }

    /// Go `resolveToken`: the configured bot token, else the token provider,
    /// else a not_authed error.
    fn resolve_token(&self) -> Result<String, SlackError> {
        if !self.bot_token.trim().is_empty() {
            return Ok(self.bot_token.trim().to_string());
        }
        if let Some(provider) = &self.token_provider {
            let token = provider.bot_token(&self.connector_id)?;
            if !token.trim().is_empty() {
                return Ok(token.trim().to_string());
            }
        }
        Err(SlackError::WebApi(WebApiError {
            status_code: 0,
            code: "not_authed".to_string(),
            message: String::new(),
        }))
    }
}

impl Transport for WebApiTransport {
    fn start<'a>(&self, _handle: Box<dyn Fn(InboundEvent) + 'a>) -> Result<(), SlackError> {
        Ok(())
    }

    fn send_reply(&self, reply: &OutboundReply) -> Result<SentReply, SlackError> {
        WebApiTransport::send_reply(self, reply)
    }

    fn reply_capabilities(&self) -> ReplyCapabilities {
        ReplyCapabilities {
            max_message_length: 40000,
            ..ReplyCapabilities::default()
        }
    }

    fn close(&self) -> Result<(), SlackError> {
        Ok(())
    }
}
