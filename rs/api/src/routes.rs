//! Router assembly for the API surface.
//!
//! Port of the route registrations in Go `NewServer` (daemon/internal/api/
//! server.go). This foundation ships the unauthenticated introspection routes:
//! `/healthz`, `/version`, `/v1/system/info`. Route families attach their own
//! `Router`s (with `protected()` layers) in later waves.

use axum::extract::State;
use axum::routing::get;
use axum::Router;
use serde::Serialize;
use tower_http::trace::TraceLayer;

use crate::response::Json;
use crate::state::AppState;
use crate::types::{self, SystemInfoResponse};

/// `/healthz` payload (Go: `{"ok": true, "service": "dope"}`).
#[derive(Debug, Clone, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct HealthzResponse {
    pub ok: bool,
    pub service: &'static str,
}

/// `/version` payload (Go: `{"version": cfg.Version}`).
#[derive(Debug, Clone, PartialEq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct VersionResponse {
    pub version: String,
}

/// GET /healthz — liveness probe.
#[allow(clippy::unused_async)]
pub async fn healthz() -> Json<HealthzResponse> {
    Json(HealthzResponse { ok: true, service: "dope" })
}

/// GET /version — daemon version.
#[allow(clippy::unused_async)]
pub async fn version(State(state): State<AppState>) -> Json<VersionResponse> {
    Json(VersionResponse { version: state.config.version.clone() })
}

/// GET /v1/system/info — environment introspection (Go
/// `buildSystemInfoResponse`).
#[allow(clippy::unused_async)]
pub async fn system_info(State(state): State<AppState>) -> Json<SystemInfoResponse> {
    Json(types::build_system_info_response(&state.config))
}

/// Builds the API router with shared layers (tracing). Route families attach
/// their own routers and `protected()` route layers in later waves.
#[must_use]
pub fn router(state: AppState) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/version", get(version))
        .route("/v1/system/info", get(system_info))
        .layer(TraceLayer::new_for_http())
        .with_state(state)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    use axum::body::to_bytes;
    use axum::http::{Request, StatusCode};
    use parking_lot::Mutex;
    use tower::ServiceExt;
    use uuid::Uuid;

    fn test_config() -> dope_config::Config {
        dope_config::Config {
            environment: dope_config::Environment::Test,
            bind_addr: "127.0.0.1:19192".to_string(),
            data_dir: "/tmp/dope-api-test".to_string(),
            log_level: "info".to_string(),
            version: "0.1.0".to_string(),
            llm: dope_config::LlmConfig::default(),
            connectors: dope_config::ConnectorConfig {
                discord: dope_config::DiscordConnectorConfig { enabled: false, ..Default::default() },
                telegram: dope_config::TelegramConnectorConfig { enabled: false, ..Default::default() },
                slack: dope_config::SlackConnectorConfig { enabled: false, ..Default::default() },
                matrix: dope_config::MatrixConnectorConfig { enabled: false, ..Default::default() },
            },
        }
    }

    /// Builds an AppState with only the required core; managers stay None.
    fn test_state() -> AppState {
        let dir = std::env::temp_dir().join(format!("dope-api-routes-{}", Uuid::now_v7()));
        std::fs::create_dir_all(&dir).expect("mkdir");
        let store = Arc::new(Mutex::new(
            dope_store::SQLiteStore::new(dir.to_str().expect("path")).expect("store"),
        ));
        AppState::new(test_config(), Arc::new(dope_events::Bus::new()), store)
    }

    async fn get_json(uri: &str) -> (StatusCode, serde_json::Value) {
        let app = router(test_state());
        let request = Request::builder()
            .uri(uri)
            .body(axum::body::Body::empty())
            .expect("request");
        let response = app.oneshot(request).await.expect("oneshot");
        let status = response.status();
        let bytes = to_bytes(response.into_body(), usize::MAX).await.expect("body");
        let json = serde_json::from_slice(&bytes).expect("json body");
        (status, json)
    }

    #[tokio::test]
    async fn healthz_returns_ok() {
        let (status, json) = get_json("/healthz").await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json, serde_json::json!({ "ok": true, "service": "dope" }));
    }

    #[tokio::test]
    async fn version_returns_config_version() {
        let (status, json) = get_json("/version").await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json, serde_json::json!({ "version": "0.1.0" }));
    }

    #[tokio::test]
    async fn system_info_returns_environment_projection() {
        let (status, json) = get_json("/v1/system/info").await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["service"], "dope");
        assert_eq!(json["environment"], "test");
        assert_eq!(json["version"], "0.1.0");
        assert_eq!(json["bindAddr"], "127.0.0.1:19192");
        assert_eq!(json["dataDir"], "/tmp/dope-api-test");
        assert_eq!(json["logLevel"], "info");
    }

    #[tokio::test]
    async fn unknown_route_is_404() {
        // axum returns an empty 404 body; check the status only.
        let app = router(test_state());
        let request = Request::builder()
            .uri("/v1/does-not-exist")
            .body(axum::body::Body::empty())
            .expect("request");
        let response = app.oneshot(request).await.expect("oneshot");
        assert_eq!(response.status(), StatusCode::NOT_FOUND);
    }
}
