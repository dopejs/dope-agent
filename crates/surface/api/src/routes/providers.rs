//! providers route family (port of the /v1/providers handlers in Go
//! daemon/internal/api/server.go, Roadmaps 9/10).
//!
//! Routes: GET /v1/providers, GET /v1/providers/{provider_id}, GET
//! /v1/providers/{provider_id}/auth, POST
//! /v1/providers/{provider_id}/auth/{start|complete|refresh|revoke}, GET
//! /v1/providers/{provider_id}/models, POST
//! /v1/providers/{provider_id}/default-model, GET/POST
//! /v1/providers/{provider_id}/checks, and GET
//! /v1/providers/{provider_id}/checks/{check_id}.
//!
//! Deliberately not ported (documented divergence, pending the Roadmap 74
//! tenant-context work): the hosted credential-permission gates and
//! per-tenant managed-auth variants — Go uses the local (non-tenant) paths
//! when no tenant context is resolved, which is the only state the current
//! router produces. Go's RunCheck builds a failed Check record for run
//! errors; the Rust manager surfaces the error instead, so the handler
//! synthesizes the failed record before persisting it.

use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json, Router};
use chrono::Utc;
use serde::{Deserialize, Serialize};

use dope_events as events;
use dope_providers as providers;

use crate::error::ApiError;
use crate::middleware::environment_scope_from_config;
use crate::state::AppState;

use super::{decode_json_or_default, decode_json_required};

/// Route family router.
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        .route("/v1/providers", get(list_providers))
        .route("/v1/providers/{provider_id}", get(get_provider))
        .route("/v1/providers/{provider_id}/auth", get(get_auth_state))
        .route("/v1/providers/{provider_id}/auth/{action}", post(auth_action))
        .route("/v1/providers/{provider_id}/models", get(list_models))
        .route("/v1/providers/{provider_id}/default-model", post(set_default_model))
        .route("/v1/providers/{provider_id}/checks", get(list_checks).post(run_check))
        .route("/v1/providers/{provider_id}/checks/{check_id}", get(get_check))
}

#[derive(Debug, Serialize)]
struct ProviderListResponse {
    items: Vec<providers::Profile>,
}

#[derive(Debug, Serialize)]
struct ProviderAuthStateResponse {
    auth: providers::AuthState,
}

#[derive(Debug, Serialize)]
struct ProviderModelListResponse {
    items: Vec<providers::Model>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ProviderDefaultModelResponse {
    provider_id: String,
    default_model: String,
    updated_at: chrono::DateTime<Utc>,
}

#[derive(Debug, Serialize)]
struct ProviderCheckListResponse {
    items: Vec<providers::Check>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct ProviderDefaultModelRequest {
    model: String,
}

fn manager(state: &AppState) -> Result<&providers::Manager, ApiError> {
    state
        .providers
        .as_deref()
        .ok_or_else(|| ApiError::internal("provider manager is not configured"))
}

/// Go llmPrepareStatusCode over ProvidersError: model/auth validation is 400,
/// the rest is 500.
fn map_providers_error(err: &providers::ProvidersError) -> ApiError {
    match err {
        providers::ProvidersError::ModelNotSupported { .. }
        | providers::ProvidersError::ManagedAuthUnsupported
        | providers::ProvidersError::Prepare(_) => ApiError::BadRequest(err.to_string()),
        _ => ApiError::Internal(err.to_string()),
    }
}

fn publish_provider_event(
    state: &AppState,
    name: &str,
    resource_kind: &str,
    resource_id: &str,
    payload: serde_json::Map<String, serde_json::Value>,
) -> Result<(), ApiError> {
    let event = events::Event {
        category: "provider".to_string(),
        name: name.to_string(),
        environment_scope: environment_scope_from_config(&state.config),
        resource: events::Resource {
            kind: resource_kind.to_string(),
            id: resource_id.to_string(),
        },
        payload,
        ..events::Event::default()
    };
    let stored = state
        .store
        .lock()
        .append_event(&event)
        .map_err(ApiError::from_store)?;
    state.event_bus.publish(stored);
    Ok(())
}

fn json_value<T: Serialize>(value: &T) -> serde_json::Value {
    serde_json::to_value(value).unwrap_or(serde_json::Value::Null)
}

/// Go publishProviderAuthEvent payload.
fn auth_event_payload(auth: &providers::AuthState) -> serde_json::Map<String, serde_json::Value> {
    let mut payload = serde_json::Map::new();
    payload.insert("tenantId".to_string(), serde_json::json!(auth.tenant_id));
    payload.insert("providerId".to_string(), serde_json::json!(auth.provider_id));
    payload.insert("family".to_string(), json_value(&auth.family));
    payload.insert("authMode".to_string(), json_value(&auth.auth_mode));
    payload.insert("status".to_string(), json_value(&auth.status));
    payload.insert("cliAvailable".to_string(), serde_json::json!(auth.cli_available));
    payload.insert("accountLabel".to_string(), serde_json::json!(auth.account_label));
    payload.insert("accountId".to_string(), serde_json::json!(auth.account_id));
    payload.insert("plan".to_string(), serde_json::json!(auth.plan));
    payload.insert("authMethod".to_string(), serde_json::json!(auth.auth_method));
    payload.insert("lastError".to_string(), serde_json::json!(auth.last_error));
    if !auth.metadata.is_empty() {
        payload.insert("metadata".to_string(), json_value(&auth.metadata));
    }
    if auth.sandbox.is_some() {
        payload.insert("sandbox".to_string(), json_value(&auth.sandbox));
    }
    payload
}

/// Go publishProviderCheckEvent payload.
fn check_event_payload(check: &providers::Check) -> serde_json::Map<String, serde_json::Value> {
    let mut payload = serde_json::Map::new();
    payload.insert("providerId".to_string(), serde_json::json!(check.provider_id));
    payload.insert("family".to_string(), json_value(&check.family));
    payload.insert("authMode".to_string(), json_value(&check.auth_mode));
    payload.insert("status".to_string(), json_value(&check.status));
    payload.insert("model".to_string(), serde_json::json!(check.model));
    payload.insert("endpoint".to_string(), serde_json::json!(check.endpoint));
    payload.insert("usage".to_string(), json_value(&check.usage));
    if !check.error_class.is_empty() {
        payload.insert("errorClass".to_string(), serde_json::json!(check.error_class));
    }
    if !check.error_code.is_empty() {
        payload.insert("errorCode".to_string(), serde_json::json!(check.error_code));
    }
    if !check.error_message.is_empty() {
        payload.insert("errorMessage".to_string(), serde_json::json!(check.error_message));
    }
    payload
}

/// GET /v1/providers (Go handleProviders).
async fn list_providers(
    State(state): State<AppState>,
) -> Result<Json<ProviderListResponse>, ApiError> {
    let manager = manager(&state)?;
    Ok(Json(ProviderListResponse { items: manager.list_profiles() }))
}

/// GET /v1/providers/{provider_id} (Go handleProviderRoutes profile branch).
async fn get_provider(
    State(state): State<AppState>,
    Path(provider_id): Path<String>,
) -> Result<Json<providers::Profile>, ApiError> {
    let manager = manager(&state)?;
    manager
        .get_profile(provider_id.trim())
        .map(Json)
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))
}

/// GET /v1/providers/{provider_id}/auth (Go auth-state branch).
async fn get_auth_state(
    State(state): State<AppState>,
    Path(provider_id): Path<String>,
) -> Result<Json<ProviderAuthStateResponse>, ApiError> {
    let manager = manager(&state)?;
    let provider_id = provider_id.trim();
    if manager.get_profile(provider_id).is_none() {
        return Err(ApiError::NotFound("not found".to_string()));
    }
    manager
        .get_auth_state(provider_id)
        .map(|auth| Json(ProviderAuthStateResponse { auth }))
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))
}

/// POST /v1/providers/{provider_id}/auth/{start|complete|refresh|revoke}
/// (Go managed-auth branch): run the managed flow, persist the auth state
/// and model list, and publish the provider.auth_* event.
async fn auth_action(
    State(state): State<AppState>,
    Path((provider_id, action)): Path<(String, String)>,
) -> Result<Json<ProviderAuthStateResponse>, ApiError> {
    let manager = manager(&state)?;
    let provider_id = provider_id.trim().to_string();
    if manager.get_profile(&provider_id).is_none() {
        return Err(ApiError::NotFound("not found".to_string()));
    }
    let (result, event_name) = match action.as_str() {
        "start" => (manager.start_managed_auth(&provider_id).await, "provider.auth_started"),
        "complete" => {
            (manager.complete_managed_auth(&provider_id).await, "provider.auth_completed")
        }
        "refresh" => {
            (manager.refresh_managed_auth(&provider_id).await, "provider.auth_refreshed")
        }
        "revoke" => (manager.revoke_managed_auth(&provider_id).await, "provider.auth_revoked"),
        _ => return Err(ApiError::NotFound("not found".to_string())),
    };
    let (auth, models) = result.map_err(|err| map_providers_error(&err))?;

    // Go persistManagedProviderState (local path).
    {
        let store = state.store.lock();
        store
            .upsert_provider_auth_state(&auth)
            .map_err(ApiError::from_store)?;
        store
            .replace_provider_models(&auth.provider_id, &models)
            .map_err(ApiError::from_store)?;
    }
    publish_provider_event(
        &state,
        event_name,
        "provider_auth",
        &auth.provider_id,
        auth_event_payload(&auth),
    )?;
    Ok(Json(ProviderAuthStateResponse { auth }))
}

/// GET /v1/providers/{provider_id}/models (Go models branch).
async fn list_models(
    State(state): State<AppState>,
    Path(provider_id): Path<String>,
) -> Result<Json<ProviderModelListResponse>, ApiError> {
    let manager = manager(&state)?;
    let provider_id = provider_id.trim();
    if manager.get_profile(provider_id).is_none() {
        return Err(ApiError::NotFound("not found".to_string()));
    }
    manager
        .list_models(provider_id)
        .map(|items| Json(ProviderModelListResponse { items }))
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))
}

/// POST /v1/providers/{provider_id}/default-model (Go default-model branch).
async fn set_default_model(
    State(state): State<AppState>,
    Path(provider_id): Path<String>,
    body: Bytes,
) -> Result<Json<ProviderDefaultModelResponse>, ApiError> {
    let request: ProviderDefaultModelRequest = decode_json_required(&body)?;
    let manager = manager(&state)?;
    let provider_id = provider_id.trim();
    if manager.get_profile(provider_id).is_none() {
        return Err(ApiError::NotFound("not found".to_string()));
    }
    let preference = manager
        .set_default_model(provider_id, request.model.trim())
        .map_err(|err| map_providers_error(&err))?;
    state
        .store
        .lock()
        .upsert_provider_preference(&preference)
        .map_err(ApiError::from_store)?;

    let mut payload = serde_json::Map::new();
    payload.insert("providerId".to_string(), serde_json::json!(preference.provider_id));
    payload.insert("defaultModel".to_string(), serde_json::json!(preference.default_model));
    publish_provider_event(
        &state,
        "provider.default_model_changed",
        "provider_preference",
        &preference.provider_id,
        payload,
    )?;
    Ok(Json(ProviderDefaultModelResponse {
        provider_id: preference.provider_id,
        default_model: preference.default_model,
        updated_at: preference.updated_at,
    }))
}

/// GET /v1/providers/{provider_id}/checks (Go checks list branch).
async fn list_checks(
    State(state): State<AppState>,
    Path(provider_id): Path<String>,
) -> Result<Json<ProviderCheckListResponse>, ApiError> {
    let manager = manager(&state)?;
    let provider_id = provider_id.trim();
    if manager.get_profile(provider_id).is_none() {
        return Err(ApiError::NotFound("not found".to_string()));
    }
    let items = state
        .store
        .lock()
        .list_provider_checks(provider_id)
        .map_err(ApiError::from_store)?;
    Ok(Json(ProviderCheckListResponse { items }))
}

/// POST /v1/providers/{provider_id}/checks (Go checks run branch) — 201 with
/// the persisted check, passed or failed.
async fn run_check(
    State(state): State<AppState>,
    Path(provider_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, Json<providers::Check>), ApiError> {
    let input: providers::CheckInput = decode_json_or_default(&body)?;
    let manager = manager(&state)?;
    let provider_id = provider_id.trim().to_string();
    let Some(profile) = manager.get_profile(&provider_id) else {
        return Err(ApiError::NotFound("not found".to_string()));
    };

    let check_id = providers::new_check_id();
    let (check, event_name) = match manager.run_check(&provider_id, &check_id, input).await {
        Ok(check) => (check, "provider.check_completed"),
        // Go RunCheck returns a failed Check alongside the error; the Rust
        // manager surfaces only the error, so synthesize the failed record.
        Err(err) => (
            providers::Check {
                check_id,
                provider_id: profile.provider_id.clone(),
                family: profile.family,
                auth_mode: profile.auth_mode,
                status: providers::CheckStatus::Failed,
                error_class: "check_failed".to_string(),
                error_message: err.to_string(),
                created_at: Utc::now(),
                completed_at: Utc::now(),
                ..providers::Check::default()
            },
            "provider.check_failed",
        ),
    };
    state
        .store
        .lock()
        .upsert_provider_check(&check)
        .map_err(ApiError::from_store)?;
    publish_provider_event(
        &state,
        event_name,
        "provider_check",
        &check.provider_id,
        check_event_payload(&check),
    )?;
    Ok((StatusCode::CREATED, Json(check)))
}

/// GET /v1/providers/{provider_id}/checks/{check_id} (Go check detail).
async fn get_check(
    State(state): State<AppState>,
    Path((provider_id, check_id)): Path<(String, String)>,
) -> Result<Json<providers::Check>, ApiError> {
    manager(&state)?;
    let check = state
        .store
        .lock()
        .get_provider_check(provider_id.trim(), check_id.trim())
        .map_err(ApiError::from_store)?;
    check
        .map(Json)
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))
}

#[cfg(test)]
mod tests {
    use super::super::tests_support::{request_json, test_state};
    use axum::http::StatusCode;
    use std::sync::Arc;

    fn state_with_manager() -> crate::state::AppState {
        let mut state = test_state();
        let manager = dope_providers::new_manager(state.config.llm.clone(), None, Vec::new());
        state.providers = Some(Arc::new(manager));
        state
    }

    #[tokio::test]
    async fn list_get_models_and_default_model() {
        let state = state_with_manager();
        let (status, listed) = request_json(state.clone(), "GET", "/v1/providers", None).await;
        assert_eq!(status, StatusCode::OK, "{listed}");
        let items = listed["items"].as_array().expect("items");
        assert!(!items.is_empty(), "{listed}");
        let provider_id = items[0]["providerId"].as_str().expect("providerId").to_string();

        let (status, fetched) =
            request_json(state.clone(), "GET", &format!("/v1/providers/{provider_id}"), None).await;
        assert_eq!(status, StatusCode::OK, "{fetched}");

        let (status, models) = request_json(
            state.clone(),
            "GET",
            &format!("/v1/providers/{provider_id}/models"),
            None,
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{models}");

        let (status, _) =
            request_json(state, "GET", "/v1/providers/provider_missing", None).await;
        assert_eq!(status, StatusCode::NOT_FOUND);
    }
}
