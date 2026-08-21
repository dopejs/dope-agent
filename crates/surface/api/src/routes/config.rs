//! config route family (port of the /v1/config handler and
//! buildConfigResponse in Go daemon/internal/api).
//!
//! Route: GET /v1/config — the redacted configuration inspection projection:
//! effective environment, LLM provider settings (secrets replaced by
//! `configured` booleans and a redactedFields list), connector projections
//! with hosted-readiness, the MCP server/catalog/transport inventory, and the
//! sandbox backend capability profiles.

use axum::extract::State;
use axum::routing::get;
use axum::{Json, Router};
use serde::Serialize;

use kura_config as config;
use kura_mcp as mcp;
use kura_sandbox as sandbox;

use crate::state::AppState;

/// Route family router.
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new().route("/v1/config", get(get_config))
}

// ---------------------------------------------------------------------------
// Response DTOs (Go ConfigResponse and friends; json tags preserved)
// ---------------------------------------------------------------------------

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigResponse {
    environment: String,
    bind_addr: String,
    data_dir: String,
    config_file_path: String,
    log_level: String,
    version: String,
    llm: ConfigLlmResponse,
    connectors: ConfigConnectorsResponse,
    mcp: ConfigMcpResponse,
    sandbox: ConfigSandboxResponse,
    redacted_fields: Vec<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigLlmResponse {
    default_provider: String,
    default_model: String,
    default_timeout_ms: i64,
    default_max_retries: i64,
    #[serde(rename = "openaiCompatible")]
    openai_compatible: ConfigOpenAiCompatibleProviderResponse,
    claude: ConfigManagedCliProviderResponse,
    codex: ConfigManagedCliProviderResponse,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigOpenAiCompatibleProviderResponse {
    configured: bool,
    #[serde(rename = "baseURL")]
    base_url: String,
    model: String,
    timeout_ms: i64,
    stream_first_chunk_timeout_ms: i64,
    stream_idle_timeout_ms: i64,
    stream_max_duration_ms: i64,
    api_key_configured: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    api_key_env: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigManagedCliProviderResponse {
    configured: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    cli_path: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    default_model: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    work_dir: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    sandbox: Option<serde_json::Value>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigConnectorsResponse {
    discord: ConfigDiscordConnectorResponse,
    telegram: ConfigTelegramConnectorResponse,
    slack: ConfigSlackConnectorResponse,
    matrix: ConfigMatrixConnectorResponse,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigMcpResponse {
    servers: Vec<mcp::ServerResource>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    catalog: Vec<mcp::CatalogEntry>,
    transports: Vec<mcp::TransportCapability>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigSandboxResponse {
    backends: Vec<sandbox::BackendCapabilityProfile>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigDiscordConnectorResponse {
    enabled: bool,
    configured: bool,
    connector_id: String,
    display_name: String,
    delivery_mode: String,
    require_mention: bool,
    #[serde(rename = "respondInDM")]
    respond_in_dm: bool,
    allowed_guild_ids: Vec<String>,
    allowed_channel_ids: Vec<String>,
    bot_token_configured: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    bot_token_env: String,
    hosted_readiness: config::DiscordHostedReadinessProjection,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigTelegramConnectorResponse {
    enabled: bool,
    configured: bool,
    connector_id: String,
    display_name: String,
    bot_token_configured: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    bot_token_env: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    bot_username: String,
    allowed_user_ids: Vec<String>,
    allowed_direct_chat_ids: Vec<String>,
    allowed_group_ids: Vec<String>,
    hosted_readiness: config::TelegramHostedReadinessProjection,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigSlackConnectorResponse {
    enabled: bool,
    configured: bool,
    connector_id: String,
    display_name: String,
    #[serde(rename = "apiBaseURL", skip_serializing_if = "String::is_empty")]
    api_base_url: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    bot_token_secret_ref: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    workspace_binding_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    workspace_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    bot_user_id: String,
    allowed_channel_ids: Vec<String>,
    #[serde(rename = "allowedDMUserIds")]
    allowed_dm_user_ids: Vec<String>,
    #[serde(rename = "allowedDMUserGroups")]
    allowed_dm_user_groups: Vec<String>,
    hosted_readiness: config::SlackHostedReadinessProjection,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConfigMatrixConnectorResponse {
    enabled: bool,
    configured: bool,
    connector_id: String,
    display_name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    homeserver_url: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    homeserver_id: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    bot_user_id: String,
    bot_access_token_set: bool,
    #[serde(skip_serializing_if = "String::is_empty")]
    bot_access_token_env: String,
    selected_room_ids: Vec<String>,
    allowed_direct_user_ids: Vec<String>,
    configured_commands: Vec<String>,
    hosted_readiness: config::MatrixHostedReadinessProjection,
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

/// GET /v1/config (Go /v1/config handler + buildConfigResponse).
async fn get_config(State(state): State<AppState>) -> Json<ConfigResponse> {
    Json(build_config_response(&state))
}

fn build_config_response(state: &AppState) -> ConfigResponse {
    let cfg = &state.config;
    let mut redacted_fields = Vec::new();
    if !cfg.llm.openai_compatible.api_key.is_empty() {
        redacted_fields.push("llm.openaiCompatible.apiKey".to_string());
    }
    if !cfg.connectors.discord.bot_token.is_empty() {
        redacted_fields.push("connectors.discord.botToken".to_string());
    }
    if !cfg.connectors.telegram.bot_token.is_empty() {
        redacted_fields.push("connectors.telegram.botToken".to_string());
    }
    if !cfg.connectors.slack.oauth_client_secret.is_empty() {
        redacted_fields.push("connectors.slack.oauthClientSecret".to_string());
    }
    if !cfg.connectors.matrix.bot_access_token.is_empty() {
        redacted_fields.push("connectors.matrix.botAccessToken".to_string());
    }

    let default_timeout_ms = if cfg.llm.default_timeout_ms > 0 {
        cfg.llm.default_timeout_ms
    } else {
        30000
    };
    let openai_timeout_ms = if cfg.llm.openai_compatible.timeout_ms > 0 {
        cfg.llm.openai_compatible.timeout_ms
    } else {
        default_timeout_ms
    };
    let first_chunk_timeout_ms = if cfg.llm.openai_compatible.stream_first_chunk_timeout_ms > 0 {
        cfg.llm.openai_compatible.stream_first_chunk_timeout_ms
    } else {
        openai_timeout_ms
    };
    let idle_timeout_ms = if cfg.llm.openai_compatible.stream_idle_timeout_ms > 0 {
        cfg.llm.openai_compatible.stream_idle_timeout_ms
    } else {
        first_chunk_timeout_ms
    };
    let discord_delivery_mode = if cfg.connectors.discord.delivery_mode.is_empty() {
        "gateway".to_string()
    } else {
        cfg.connectors.discord.delivery_mode.clone()
    };

    let openai = &cfg.llm.openai_compatible;
    let discord = &cfg.connectors.discord;
    let telegram = &cfg.connectors.telegram;
    let slack = &cfg.connectors.slack;
    let matrix = &cfg.connectors.matrix;

    ConfigResponse {
        // Deployment shape, not an isolation scope: an embedded daemon
        // reports itself honestly rather than claiming to be a test daemon.
        environment: match cfg.environment {
            config::Environment::Prod => "prod".to_string(),
            config::Environment::Test => "test".to_string(),
            config::Environment::Embedded => "embedded".to_string(),
        },
        bind_addr: cfg.bind_addr.clone(),
        data_dir: cfg.data_dir.clone(),
        config_file_path: config::config_file_path(&cfg.data_dir)
            .to_string_lossy()
            .to_string(),
        log_level: cfg.log_level.clone(),
        version: cfg.version.clone(),
        llm: ConfigLlmResponse {
            default_provider: cfg.llm.default_provider.clone(),
            default_model: cfg.llm.default_model.clone(),
            default_timeout_ms,
            default_max_retries: cfg.llm.default_max_retries,
            openai_compatible: ConfigOpenAiCompatibleProviderResponse {
                configured: !openai.base_url.is_empty()
                    || !openai.api_key.is_empty()
                    || !openai.model.is_empty(),
                base_url: openai.base_url.clone(),
                model: openai.model.clone(),
                timeout_ms: openai_timeout_ms,
                stream_first_chunk_timeout_ms: first_chunk_timeout_ms,
                stream_idle_timeout_ms: idle_timeout_ms,
                stream_max_duration_ms: openai.stream_max_duration_ms,
                api_key_configured: !openai.api_key.is_empty(),
                api_key_env: openai.api_key_env.clone(),
            },
            claude: managed_cli_response("claude_managed", &cfg.llm.claude),
            codex: managed_cli_response("codex_managed", &cfg.llm.codex),
        },
        connectors: ConfigConnectorsResponse {
            discord: ConfigDiscordConnectorResponse {
                enabled: discord.enabled,
                configured: !discord.bot_token.is_empty(),
                connector_id: discord.connector_id.clone(),
                display_name: discord.display_name.clone(),
                delivery_mode: discord_delivery_mode,
                require_mention: discord.require_mention,
                respond_in_dm: discord.respond_in_dm,
                allowed_guild_ids: discord.allowed_guild_ids.clone(),
                allowed_channel_ids: discord.allowed_channel_ids.clone(),
                bot_token_configured: !discord.bot_token.is_empty(),
                bot_token_env: discord.bot_token_env.clone(),
                hosted_readiness: discord.project_hosted_readiness(""),
            },
            telegram: ConfigTelegramConnectorResponse {
                enabled: telegram.enabled,
                configured: !telegram.bot_token.is_empty(),
                connector_id: telegram.connector_id.clone(),
                display_name: telegram.display_name.clone(),
                bot_token_configured: !telegram.bot_token.is_empty(),
                bot_token_env: telegram.bot_token_env.clone(),
                bot_username: telegram.bot_username.clone(),
                allowed_user_ids: telegram.allowed_user_ids.clone(),
                allowed_direct_chat_ids: telegram.allowed_direct_chat_ids.clone(),
                allowed_group_ids: telegram.allowed_group_ids.clone(),
                hosted_readiness: telegram.project_hosted_readiness(""),
            },
            slack: ConfigSlackConnectorResponse {
                enabled: slack.enabled,
                configured: !slack.workspace_id.is_empty()
                    || !slack.allowed_channel_ids.is_empty()
                    || !slack.allowed_dm_user_ids.is_empty()
                    || !slack.allowed_dm_user_groups.is_empty(),
                connector_id: slack.connector_id.clone(),
                display_name: slack.display_name.clone(),
                api_base_url: slack.api_base_url.clone(),
                bot_token_secret_ref: slack.bot_token_secret_ref.clone(),
                workspace_binding_id: slack.workspace_binding_id.clone(),
                workspace_id: slack.workspace_id.clone(),
                bot_user_id: slack.bot_user_id.clone(),
                allowed_channel_ids: slack.allowed_channel_ids.clone(),
                allowed_dm_user_ids: slack.allowed_dm_user_ids.clone(),
                allowed_dm_user_groups: slack.allowed_dm_user_groups.clone(),
                hosted_readiness: slack.project_hosted_readiness(""),
            },
            matrix: ConfigMatrixConnectorResponse {
                enabled: matrix.enabled,
                configured: !matrix.homeserver_url.is_empty()
                    || !matrix.bot_access_token.is_empty()
                    || !matrix.selected_room_ids.is_empty()
                    || !matrix.allowed_direct_user_ids.is_empty(),
                connector_id: matrix.connector_id.clone(),
                display_name: matrix.display_name.clone(),
                homeserver_url: matrix.homeserver_url.clone(),
                homeserver_id: matrix.homeserver_id.clone(),
                bot_user_id: matrix.bot_user_id.clone(),
                bot_access_token_set: !matrix.bot_access_token.is_empty(),
                bot_access_token_env: matrix.bot_access_token_env.clone(),
                selected_room_ids: matrix.selected_room_ids.clone(),
                allowed_direct_user_ids: matrix.allowed_direct_user_ids.clone(),
                configured_commands: matrix.configured_commands.clone(),
                hosted_readiness: matrix.project_hosted_readiness(""),
            },
        },
        mcp: ConfigMcpResponse {
            servers: state
                .mcp
                .as_deref()
                .map(mcp::Manager::list_servers)
                .unwrap_or_default(),
            catalog: state
                .mcp
                .as_deref()
                .map(mcp::Manager::list_catalog)
                .unwrap_or_default(),
            transports: state
                .mcp
                .as_deref()
                .map(mcp::Manager::list_transport_capabilities)
                .unwrap_or_default(),
        },
        sandbox: ConfigSandboxResponse {
            backends: state
                .sandboxes
                .as_deref()
                .map(sandbox::Manager::backend_capabilities)
                .unwrap_or_default(),
        },
        redacted_fields,
    }
}

/// Go buildManagedProviderConfigSandbox: the declaration-only sandbox
/// contract view a managed CLI provider would run under for config
/// inspection.
fn managed_provider_config_sandbox(provider_id: &str, work_dir: &str) -> Option<serde_json::Value> {
    let work_dir = work_dir.trim();
    let read_roots: Vec<String> = if work_dir.is_empty() {
        Vec::new()
    } else {
        vec![work_dir.to_string()]
    };
    let view = sandbox::ConsumerContractView {
        declaration: Some(sandbox::ConsumerRequirementDeclaration {
            declaration_id: format!("managed_provider:{}:config", provider_id.trim()),
            consumer_kind: sandbox::ConsumerKind::ManagedProvider,
            consumer_id: provider_id.trim().to_string(),
            operation_kind: "config_inspect".to_string(),
            profile_id: sandbox::PROFILE_ID_SUBPROCESS_DEFAULT.to_string(),
            execution_mode: sandbox::ExecutionMode::DeclarationOnly,
            allowed_backend_kinds: vec![sandbox::BackendKind::Subprocess],
            read_roots,
            write_roots: Vec::new(),
            network_mode: Some(sandbox::NetworkMode::Deny),
            secret_refs: Vec::new(),
            approval_mode: Some(sandbox::ApprovalMode::Allow),
            required_enforcement_strength: "declared_only".to_string(),
            active: true,
            source: sandbox::Source::Builtin,
            ..sandbox::ConsumerRequirementDeclaration::default()
        }),
        ..sandbox::ConsumerContractView::default()
    };
    serde_json::to_value(&view).ok()
}

fn managed_cli_response(
    provider_id: &str,
    provider: &config::ManagedCliProviderConfig,
) -> ConfigManagedCliProviderResponse {
    ConfigManagedCliProviderResponse {
        configured: !provider.cli_path.is_empty()
            || !provider.default_model.is_empty()
            || (!provider.work_dir.is_empty() && provider.work_dir != "~"),
        cli_path: provider.cli_path.clone(),
        default_model: provider.default_model.clone(),
        work_dir: provider.work_dir.clone(),
        sandbox: managed_provider_config_sandbox(provider_id, &provider.work_dir),
    }
}

#[cfg(test)]
mod tests {
    use super::super::tests_support::{request_json, test_state};
    use axum::http::StatusCode;

    #[tokio::test]
    async fn config_projection_reports_environment_and_redactions() {
        let mut state = test_state();
        state.config.llm.openai_compatible.api_key = "sk-secret".to_string();
        state.config.llm.openai_compatible.api_key_env = "OPENAI_API_KEY".to_string();

        let (status, body) = request_json(state, "GET", "/v1/config", None).await;
        assert_eq!(status, StatusCode::OK, "{body}");
        assert_eq!(body["environment"], "test");
        assert_eq!(body["bindAddr"], "127.0.0.1:19192");
        assert_eq!(body["llm"]["openaiCompatible"]["apiKeyConfigured"], true);
        assert!(body["llm"]["openaiCompatible"]["apiKey"].is_null(), "{body}");
        assert!(body["redactedFields"]
            .as_array()
            .expect("redactedFields")
            .iter()
            .any(|field| field == "llm.openaiCompatible.apiKey"));
        assert_eq!(body["llm"]["defaultTimeoutMs"], 30000);
        assert_eq!(body["connectors"]["discord"]["deliveryMode"], "gateway");
        assert!(body["mcp"]["servers"].as_array().expect("servers").is_empty());
        assert!(body["sandbox"]["backends"].as_array().expect("backends").is_empty());
    }
}
