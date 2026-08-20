//! triage route family (port of daemon/internal/api/triage.go, Roadmap 65).
//!
//! Routes: GET/POST /v1/triage/policies, GET /v1/triage/policies/{policy_id},
//! POST /v1/triage/policies/{policy_id}/run. Triage evaluates explicit-rule
//! policies over caller-supplied message sets; it never scans a mailbox on its
//! own. Error mapping preserves Go writeTriageError: policy-not-found -> 404,
//! invalid rule/policy -> 400, everything else -> 500.

use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::{Deserialize, Serialize};

use kura_triage as triage;

use crate::error::ApiError;
use crate::state::AppState;

use super::{decode_json_or_default, decode_json_required};

/// Route family router. Unregistered methods answer 405 like the Go
/// MethodNotAllowed branches.
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        .route("/v1/triage/policies", get(list_policies).post(create_policy))
        .route("/v1/triage/policies/{policy_id}", get(get_policy))
        .route("/v1/triage/policies/{policy_id}/run", post(run_policy))
}

/// Go CreateTriagePolicyRequest.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct CreateTriagePolicyRequest {
    name: String,
    rules: Vec<triage::Rule>,
    default_classification: triage::Classification,
}

/// Go RunTriageRequest — the caller selects which messages to triage.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct RunTriageRequest {
    messages: Vec<triage::Message>,
}

#[derive(Debug, Serialize)]
struct TriagePolicyListResponse {
    items: Vec<triage::Policy>,
}

fn manager(state: &AppState) -> Result<&triage::Manager, ApiError> {
    state
        .triage
        .as_deref()
        .ok_or_else(|| ApiError::internal("triage manager is not configured"))
}

fn map_triage_error(err: triage::TriageError) -> ApiError {
    let message = err.to_string();
    match err {
        triage::TriageError::PolicyNotFound => ApiError::NotFound(message),
        triage::TriageError::InvalidRule | triage::TriageError::InvalidPolicy => {
            ApiError::BadRequest(message)
        }
    }
}

/// GET /v1/triage/policies (Go handleTriagePolicies GET branch).
async fn list_policies(
    State(state): State<AppState>,
) -> Result<Json<TriagePolicyListResponse>, ApiError> {
    let manager = manager(&state)?;
    Ok(Json(TriagePolicyListResponse { items: manager.list_policies() }))
}

/// POST /v1/triage/policies (Go handleTriagePolicies POST branch) — 201.
async fn create_policy(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<(StatusCode, Json<triage::Policy>), ApiError> {
    let request: CreateTriagePolicyRequest = decode_json_required(&body)?;
    let manager = manager(&state)?;
    let policy = manager
        .create_policy(request.name.trim(), request.rules, request.default_classification)
        .map_err(map_triage_error)?;
    Ok((StatusCode::CREATED, Json(policy)))
}

/// GET /v1/triage/policies/{policy_id} (Go handleTriagePolicyRoutes get).
async fn get_policy(
    State(state): State<AppState>,
    Path(policy_id): Path<String>,
) -> Result<Json<triage::Policy>, ApiError> {
    let manager = manager(&state)?;
    manager
        .get_policy(policy_id.trim())
        .map(Json)
        .ok_or_else(|| map_triage_error(triage::TriageError::PolicyNotFound))
}

/// POST /v1/triage/policies/{policy_id}/run (Go handleTriagePolicyRoutes run
/// branch) — 201; an empty body runs the policy over zero messages (Go
/// tolerates the EOF decode error).
async fn run_policy(
    State(state): State<AppState>,
    Path(policy_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, Json<triage::Run>), ApiError> {
    let request: RunTriageRequest = decode_json_or_default(&body)?;
    let manager = manager(&state)?;
    let run = manager
        .run(policy_id.trim(), &request.messages)
        .map_err(map_triage_error)?;
    Ok((StatusCode::CREATED, Json(run)))
}

#[cfg(test)]
mod tests {
    use super::super::tests_support::{request_json, test_state};
    use axum::http::StatusCode;
    use std::sync::Arc;

    fn state_with_manager() -> crate::state::AppState {
        let mut state = test_state();
        state.triage = Some(Arc::new(kura_triage::Manager::new("test")));
        state
    }

    #[tokio::test]
    async fn create_list_get_and_run_policy() {
        let state = state_with_manager();
        let (status, created) = request_json(
            state.clone(),
            "POST",
            "/v1/triage/policies",
            Some(serde_json::json!({
                "name": "vip",
                "rules": [],
                "defaultClassification": "urgent"
            })),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "{created}");
        let policy_id = created["policyId"].as_str().expect("policyId").to_string();

        let (status, listed) =
            request_json(state.clone(), "GET", "/v1/triage/policies", None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(listed["items"].as_array().expect("items").len(), 1);

        let (status, fetched) = request_json(
            state.clone(),
            "GET",
            &format!("/v1/triage/policies/{policy_id}"),
            None,
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(fetched["policyId"], policy_id.as_str());

        // Empty body is tolerated: the run covers zero messages.
        let (status, run) = request_json(
            state,
            "POST",
            &format!("/v1/triage/policies/{policy_id}/run"),
            None,
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "{run}");
        assert_eq!(run["policyId"], policy_id.as_str());
    }

    #[tokio::test]
    async fn missing_policy_is_404_and_missing_manager_is_500() {
        let (status, _) = request_json(
            state_with_manager(),
            "GET",
            "/v1/triage/policies/triage_policy_missing",
            None,
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND);

        let (status, body) =
            request_json(test_state(), "GET", "/v1/triage/policies", None).await;
        assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
        assert_eq!(body["error"], "triage manager is not configured");
    }
}
