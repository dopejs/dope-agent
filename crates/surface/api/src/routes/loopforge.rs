//! Loopforge integration routes.
//!
//! The daemon owns the last accepted project context so desktop clients can
//! reconnect without reconstructing state from a local process. The payload is
//! deliberately read-mostly and redacted; deterministic tool execution remains
//! in Loopforge.

use std::path::PathBuf;

use axum::extract::State;
use axum::routing::get;
use axum::{Json, Router};
use serde::{Deserialize, Serialize};

use crate::error::ApiError;
use crate::state::AppState;

pub const CONTEXT_SCHEMA_VERSION: &str = "game-project-context-v1";

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct LoopforgeProjectContext {
    pub schema_version: String,
    pub project_id: String,
    pub project_root: String,
    pub observed_revision: u64,
    pub stage: String,
    pub engine: Option<String>,
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub next_actions: Vec<String>,
    #[serde(default)]
    pub redactions: Vec<String>,
}

fn validate_context(context: &LoopforgeProjectContext) -> Result<(), ApiError> {
    if context.schema_version != CONTEXT_SCHEMA_VERSION {
        return Err(ApiError::BadRequest(format!(
            "unsupported Loopforge context schema: {}",
            context.schema_version
        )));
    }
    if context.project_id.trim().is_empty()
        || context.project_root.trim().is_empty()
        || context.stage.trim().is_empty()
    {
        return Err(ApiError::BadRequest(
            "project_id, project_root, and stage are required".to_string(),
        ));
    }
    Ok(())
}

fn context_path(state: &AppState) -> PathBuf {
    PathBuf::from(&state.config.data_dir).join("loopforge-project.json")
}

fn require_loopback(state: &AppState) -> Result<(), ApiError> {
    let bind_addr = state.config.bind_addr.trim();
    if bind_addr.starts_with("127.0.0.1:") || bind_addr.starts_with("[::1]:") {
        Ok(())
    } else {
        Err(ApiError::Forbidden(
            "Loopforge context routes require a loopback daemon bind address".to_string(),
        ))
    }
}

fn load_context(state: &AppState) -> Option<LoopforgeProjectContext> {
    let path = context_path(state);
    let bytes = std::fs::read(path).ok()?;
    serde_json::from_slice(&bytes).ok()
}

fn save_context(state: &AppState, context: &LoopforgeProjectContext) -> Result<(), ApiError> {
    let path = context_path(state);
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).map_err(|err| ApiError::internal(err.to_string()))?;
    }
    let temporary = path.with_extension("json.tmp");
    let bytes =
        serde_json::to_vec_pretty(context).map_err(|err| ApiError::internal(err.to_string()))?;
    std::fs::write(&temporary, bytes).map_err(|err| ApiError::internal(err.to_string()))?;
    std::fs::rename(temporary, path).map_err(|err| ApiError::internal(err.to_string()))
}

pub fn router() -> Router<AppState> {
    Router::new().route("/v1/loopforge/project", get(get_project).put(put_project))
}

async fn get_project(
    State(state): State<AppState>,
) -> Result<Json<LoopforgeProjectContext>, ApiError> {
    require_loopback(&state)?;
    let context = state
        .loopforge_context
        .lock()
        .clone()
        .or_else(|| load_context(&state))
        .ok_or_else(|| {
            ApiError::NotFound("Loopforge project context is not configured".to_string())
        })?;
    Ok(Json(context))
}

async fn put_project(
    State(state): State<AppState>,
    Json(context): Json<LoopforgeProjectContext>,
) -> Result<Json<LoopforgeProjectContext>, ApiError> {
    require_loopback(&state)?;
    validate_context(&context)?;
    save_context(&state, &context)?;
    *state.loopforge_context.lock() = Some(context.clone());
    Ok(Json(context))
}

/// Shared state constructor helper for route tests.
#[cfg(test)]
pub(crate) fn test_context() -> LoopforgeProjectContext {
    LoopforgeProjectContext {
        schema_version: CONTEXT_SCHEMA_VERSION.to_string(),
        project_id: "gameproj_test".to_string(),
        project_root: "/tmp/game".to_string(),
        observed_revision: 1,
        stage: "DISCOVERY".to_string(),
        engine: Some("godot".to_string()),
        capabilities: vec!["loopforge.status".to_string()],
        next_actions: vec!["hypothesis.create".to_string()],
        redactions: vec!["access_tokens".to_string()],
    }
}

#[cfg(test)]
mod tests {
    use axum::body::{to_bytes, Body};
    use axum::http::{Request, StatusCode};
    use tower::ServiceExt;

    use super::*;

    #[tokio::test]
    async fn project_context_round_trips_and_rejects_unknown_versions() {
        let state = crate::routes::tests_support::test_state();
        let app = super::router().with_state(state.clone());
        let payload = serde_json::to_value(test_context()).expect("payload");
        let response = app
            .clone()
            .oneshot(
                Request::builder()
                    .method("PUT")
                    .uri("/v1/loopforge/project")
                    .header("content-type", "application/json")
                    .body(Body::from(payload.to_string()))
                    .expect("request"),
            )
            .await
            .expect("response");
        assert_eq!(response.status(), StatusCode::OK);

        let response = app
            .oneshot(
                Request::builder()
                    .uri("/v1/loopforge/project")
                    .body(Body::empty())
                    .expect("request"),
            )
            .await
            .expect("response");
        assert_eq!(response.status(), StatusCode::OK);
        let body = to_bytes(response.into_body(), usize::MAX)
            .await
            .expect("body");
        let got: LoopforgeProjectContext = serde_json::from_slice(&body).expect("json");
        assert_eq!(got, test_context());

        let mut invalid = serde_json::to_value(test_context()).expect("payload");
        invalid["schema_version"] = serde_json::json!("game-project-context-v2");
        let response = super::router()
            .with_state(crate::routes::tests_support::test_state())
            .oneshot(
                Request::builder()
                    .method("PUT")
                    .uri("/v1/loopforge/project")
                    .header("content-type", "application/json")
                    .body(Body::from(invalid.to_string()))
                    .expect("request"),
            )
            .await
            .expect("response");
        assert_eq!(response.status(), StatusCode::BAD_REQUEST);
    }
}
