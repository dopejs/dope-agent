//! sandboxes route family (port of the /v1/sandboxes handlers in Go
//! daemon/internal/api/server.go, Roadmap 16).
//!
//! Routes: GET /v1/sandboxes/profiles, POST /v1/sandboxes/profiles/reload,
//! GET /v1/sandboxes/profiles/{profile_id}, GET/POST /v1/sandboxes/executions,
//! GET /v1/sandboxes/executions/{execution_id}, POST
//! /v1/sandboxes/executions/{execution_id}/cancel, POST /v1/sandboxes/explain.
//!
//! Error mapping preserves the Go handlers: nil manager -> 500, missing
//! command -> 400, unknown execution -> 404.
//!
//! Deliberately not ported (documented divergence): the Go explain handler's
//! credential-inspection redaction (redactSandboxDecisionCredentialInspection)
//! keys off the resolved tenant context, which the router only carries once
//! the protected() middleware attachment task in Roadmap 74 lands; Go skips
//! the redaction when no tenant context is resolved, which is the only state
//! the current router produces.

use axum::body::Bytes;
use axum::extract::{Extension, Path, State};
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::Serialize;

use kura_sandbox as sandbox;

use crate::error::ApiError;
use crate::middleware::AuthenticatedToken;
use crate::state::AppState;

use super::decode_json_required;

/// Route family router.
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        .route("/v1/sandboxes/profiles", get(list_profiles))
        .route("/v1/sandboxes/profiles/reload", post(reload_profiles))
        .route("/v1/sandboxes/profiles/{profile_id}", get(get_profile))
        .route("/v1/sandboxes/executions", get(list_executions).post(start_execution))
        .route("/v1/sandboxes/executions/{execution_id}", get(get_execution))
        .route("/v1/sandboxes/executions/{execution_id}/cancel", post(cancel_execution))
        .route("/v1/sandboxes/explain", post(explain))
}

#[derive(Debug, Serialize)]
struct ProfileListResponse {
    items: Vec<sandbox::Profile>,
}

#[derive(Debug, Serialize)]
struct ExecutionListResponse {
    items: Vec<sandbox::Execution>,
}

/// Go SandboxExplainResponse.
#[derive(Debug, Serialize)]
struct SandboxExplainResponse {
    decision: sandbox::Decision,
}

fn manager(state: &AppState) -> Result<&sandbox::Manager, ApiError> {
    state
        .sandboxes
        .as_deref()
        .ok_or_else(|| ApiError::internal("sandbox manager is not configured"))
}

/// Go currentActor: the authenticated token's label, or its token id (same
/// helper as the computer-use family).
fn current_actor(token: Option<&AuthenticatedToken>) -> String {
    let Some(token) = token else {
        return String::new();
    };
    if !token.0.label.trim().is_empty() {
        token.0.label.clone()
    } else {
        token.0.token_id.clone()
    }
}

fn map_sandbox_error(err: sandbox::SandboxError) -> ApiError {
    match err {
        sandbox::SandboxError::CommandRequired => ApiError::BadRequest(err.to_string()),
        sandbox::SandboxError::ExecutionNotFound => ApiError::NotFound("not found".to_string()),
        other => ApiError::Internal(other.to_string()),
    }
}

/// GET /v1/sandboxes/profiles (Go handleSandboxProfiles).
async fn list_profiles(
    State(state): State<AppState>,
) -> Result<Json<ProfileListResponse>, ApiError> {
    let manager = manager(&state)?;
    Ok(Json(ProfileListResponse { items: manager.list_profiles() }))
}

/// POST /v1/sandboxes/profiles/reload (Go handleSandboxProfileRoutes reload
/// branch).
async fn reload_profiles(
    State(state): State<AppState>,
) -> Result<Json<ProfileListResponse>, ApiError> {
    let manager = manager(&state)?;
    Ok(Json(ProfileListResponse { items: manager.reload() }))
}

/// GET /v1/sandboxes/profiles/{profile_id} (Go handleSandboxProfileRoutes get
/// branch).
async fn get_profile(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
) -> Result<Json<sandbox::Profile>, ApiError> {
    let manager = manager(&state)?;
    manager
        .get_profile(profile_id.trim())
        .map(Json)
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))
}

/// GET /v1/sandboxes/executions (Go handleSandboxExecutions GET branch).
async fn list_executions(
    State(state): State<AppState>,
) -> Result<Json<ExecutionListResponse>, ApiError> {
    let manager = manager(&state)?;
    Ok(Json(ExecutionListResponse { items: manager.list_executions() }))
}

/// POST /v1/sandboxes/executions (Go handleSandboxExecutions POST branch) —
/// 201 with the started execution.
async fn start_execution(
    State(state): State<AppState>,
    token: Option<Extension<AuthenticatedToken>>,
    body: Bytes,
) -> Result<(StatusCode, Json<sandbox::Execution>), ApiError> {
    let mut request: sandbox::ExecutionRequest = decode_json_required(&body)?;
    if request.requested_by.trim().is_empty() {
        request.requested_by = current_actor(token.as_ref().map(|extension| &extension.0));
    }
    let manager = manager(&state)?;
    let execution = manager.start_execution(request).map_err(map_sandbox_error)?;
    Ok((StatusCode::CREATED, Json(execution)))
}

/// GET /v1/sandboxes/executions/{execution_id} (Go
/// handleSandboxExecutionRoutes get branch).
async fn get_execution(
    State(state): State<AppState>,
    Path(execution_id): Path<String>,
) -> Result<Json<sandbox::Execution>, ApiError> {
    let manager = manager(&state)?;
    manager
        .get_execution(execution_id.trim())
        .map(Json)
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))
}

/// POST /v1/sandboxes/executions/{execution_id}/cancel (Go
/// handleSandboxExecutionRoutes cancel branch).
async fn cancel_execution(
    State(state): State<AppState>,
    Path(execution_id): Path<String>,
) -> Result<Json<sandbox::Execution>, ApiError> {
    let manager = manager(&state)?;
    let (execution, _) = manager
        .cancel_execution(execution_id.trim())
        .map_err(map_sandbox_error)?;
    Ok(Json(execution))
}

/// POST /v1/sandboxes/explain (Go handleSandboxExplain) — the sandbox
/// decision for a request without executing it.
async fn explain(
    State(state): State<AppState>,
    token: Option<Extension<AuthenticatedToken>>,
    body: Bytes,
) -> Result<Json<SandboxExplainResponse>, ApiError> {
    let mut request: sandbox::ExecutionRequest = decode_json_required(&body)?;
    if request.requested_by.trim().is_empty() {
        request.requested_by = current_actor(token.as_ref().map(|extension| &extension.0));
    }
    let manager = manager(&state)?;
    let decision = manager.explain(request).map_err(map_sandbox_error)?;
    Ok(Json(SandboxExplainResponse { decision }))
}

#[cfg(test)]
mod tests {
    use super::super::tests_support::{request_json, test_state};
    use axum::http::StatusCode;
    use std::sync::Arc;

    fn state_with_manager() -> crate::state::AppState {
        let mut state = test_state();
        let manager = kura_sandbox::Manager::new(
            state.config.clone(),
            None,
            kura_events::Bus::new(),
            kura_policy::Engine::new(),
        );
        state.sandboxes = Some(Arc::new(manager));
        state
    }

    #[tokio::test]
    async fn profiles_list_get_and_reload() {
        let state = state_with_manager();
        let (status, listed) =
            request_json(state.clone(), "GET", "/v1/sandboxes/profiles", None).await;
        assert_eq!(status, StatusCode::OK, "{listed}");
        let items = listed["items"].as_array().expect("items");
        assert!(!items.is_empty(), "{listed}");
        let profile_id = items[0]["profileId"].as_str().expect("profileId").to_string();

        let (status, fetched) = request_json(
            state.clone(),
            "GET",
            &format!("/v1/sandboxes/profiles/{profile_id}"),
            None,
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{fetched}");

        let (status, reloaded) =
            request_json(state.clone(), "POST", "/v1/sandboxes/profiles/reload", None).await;
        assert_eq!(status, StatusCode::OK, "{reloaded}");

        let (status, _) = request_json(
            state,
            "GET",
            "/v1/sandboxes/profiles/sandbox_profile_missing",
            None,
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND);
    }

    #[tokio::test]
    async fn explain_requires_a_command_and_missing_execution_is_404() {
        let state = state_with_manager();
        let (status, body) = request_json(
            state.clone(),
            "POST",
            "/v1/sandboxes/explain",
            Some(serde_json::json!({ "command": "" })),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "{body}");

        let (status, _) = request_json(
            state.clone(),
            "GET",
            "/v1/sandboxes/executions/sbx_exec_missing",
            None,
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND);

        let (status, listed) =
            request_json(state, "GET", "/v1/sandboxes/executions", None).await;
        assert_eq!(status, StatusCode::OK, "{listed}");
    }
}
