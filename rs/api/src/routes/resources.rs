//! resources route family (port of daemon/internal/api/thread_lifecycle.go,
//! thread_handoff.go, agent_profiles.go, workspace_bindings.go).
//!
//! The Go surface mounts these route families under /v1:
//! - `/v1/threads` + `/v1/threads/` — thread list, detail, lifecycle actions
//!   (reset/archive/reopen), handoff creation, continuity-preview detail
//! - `/v1/profiles` + `/v1/profiles/` — agent profile CRUD + lifecycle
//! - `/v1/workspaces` + `/v1/workspaces/` — workspace records
//! - `/v1/bindings` + `/v1/bindings/` — binding rules (+ repair)
//! - `/v1/capability-visibility` — capability visibility policy
//!
//! Port status:
//! - The thread list/detail/lifecycle surface is fully ported on the dope-store
//!   thread DAOs (rs/store/src/thread_persistence.rs), the dope-threads domain
//!   types and the dope-events thread event builders. Status codes, DTOs,
//!   validation (empty body -> 400, unknown action -> 404, missing row -> 404,
//!   transition conflicts -> 409) and the tenant-scoped permission gates
//!   (credentials.inspect for reads, connectors.manage for mutations) mirror the
//!   Go handlers.
//! - The thread detail binding projection (writeThreadDetailWithBindingProjection,
//!   FR-013) is skipped: it needs `LatestRuntimeBindingEvidence` + the
//!   bindings resource projection, neither ported. The base response matches Go
//!   for callers without bindings.inspect.
//! - The following surfaces are registered but answer 501 because the store
//!   DAOs they need have not been ported (reported, not duplicated):
//!   * POST /v1/threads/{id}/handoffs — needs store save_handoff_link /
//!     save_handoff_source_references / list_continuity_turns (plus the
//!     active-profile projection DAOs); dope-threads already supplies
//!     validate_handoff / build_handoff_source_references / resolve_conversation_shape.
//!   * GET /v1/threads/{id}/continuity-previews/{preview_id} — needs store
//!     get_continuity_preview_detail.
//!   * /v1/profiles* — needs store ListAgentProfiles / CreateAgentProfile /
//!     GetAgentProfileDetail / UpdateAgentProfile / ActivateAgentProfile /
//!     ListAgentProfileVersions / RollbackAgentProfile / RetireAgentProfile
//!     (dope-profiles supplies the domain types and mutation policy).
//!   * /v1/workspaces* — needs store ListWorkspaces / GetWorkspace /
//!     CreateWorkspace / UpdateWorkspaceStatus.
//!   * /v1/bindings* — needs store ListBindingRules / GetBindingRule /
//!     CreateBindingRule / UpdateBindingRule / RemoveBindingRule / RepairBindingRule.
//!   * /v1/capability-visibility — needs store ListCapabilityVisibility /
//!     SetCapabilityVisibility.
//!
//! The 501 shape follows the existing activation.rs precedent (and Go's own
//! handleThreadHandoffNotImplemented / evaluation-route stubs); Go maps the
//! nil-store analogue to 500, so a 500 would be misleading here — the store
//! exists, its DAOs are simply not ported.

use axum::body::Bytes;
use axum::extract::{Extension, Path, Query, State};
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json as AxumJson, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use dope_events as events;
use dope_identity::{has_permission, Permission};

use crate::error::ApiError;
use crate::middleware::TenantContext;
use crate::response::Json;
use crate::state::AppState;

/// Route family router. Only the methods the Go handlers accept are
/// registered; axum answers the other methods with 405 (Go
/// w.WriteHeader(http.StatusMethodNotAllowed)).
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        // Threads
        .route("/v1/threads", get(list_threads))
        .route("/v1/threads/{thread_id}", get(thread_detail))
        .route(
            "/v1/threads/{thread_id}/handoffs",
            post(thread_handoff_create),
        )
        .route(
            "/v1/threads/{thread_id}/continuity-previews/{preview_id}",
            get(thread_continuity_preview_detail),
        )
        .route("/v1/threads/{thread_id}/reset", post(thread_reset))
        .route("/v1/threads/{thread_id}/archive", post(thread_archive))
        .route("/v1/threads/{thread_id}/reopen", post(thread_reopen))
        // Agent profiles
        .route("/v1/profiles", get(list_profiles).post(create_profile))
        .route(
            "/v1/profiles/{profile_id}",
            get(get_profile).patch(update_profile),
        )
        .route("/v1/profiles/{profile_id}/activate", post(activate_profile))
        .route(
            "/v1/profiles/{profile_id}/versions",
            get(list_profile_versions),
        )
        .route("/v1/profiles/{profile_id}/rollback", post(rollback_profile))
        .route("/v1/profiles/{profile_id}/archive", post(archive_profile))
        .route("/v1/profiles/{profile_id}/disable", post(disable_profile))
        // Workspaces / bindings / capability visibility
        .route(
            "/v1/workspaces",
            get(list_workspaces).post(create_workspace),
        )
        .route(
            "/v1/workspaces/{workspace_id}",
            get(get_workspace).patch(update_workspace),
        )
        .route(
            "/v1/bindings",
            get(list_bindings).post(create_binding),
        )
        .route(
            "/v1/bindings/{binding_id}",
            get(get_binding).patch(update_binding).delete(delete_binding),
        )
        .route("/v1/bindings/{binding_id}/repair", post(repair_binding))
        .route(
            "/v1/capability-visibility",
            get(list_capability_visibility).put(set_capability_visibility),
        )
}

// ---------------------------------------------------------------------------
// Request/response DTOs (local ports of the Go api-package types)
// ---------------------------------------------------------------------------

/// Go handleThreadList query params (limit/cursor/state/sourceKind).
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ThreadListQuery {
    #[serde(default)]
    limit: Option<String>,
    #[serde(default)]
    cursor: Option<String>,
    #[serde(default)]
    state: Option<String>,
    #[serde(default)]
    source_kind: Option<String>,
}

/// Go `threadLifecycleActionRequest` (Go decodes note but never uses it;
/// kept for wire parity).
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ThreadLifecycleActionRequest {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    reason_code: String,
    #[allow(dead_code)]
    #[serde(default, skip_serializing_if = "String::is_empty")]
    note: String,
}

/// Go `threadLifecycleActionResponse`.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct ThreadLifecycleActionResponse {
    thread_id: String,
    lifecycle_state: dope_threads::LifecycleState,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    previous_session_segment_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    current_session_segment_id: String,
    audit_event_id: String,
    changed_at: DateTime<Utc>,
    action: dope_threads::LifecycleActionKind,
    available_actions: Vec<dope_threads::LifecycleActionKind>,
}

// ---------------------------------------------------------------------------
// Threads: list / detail / lifecycle actions
// ---------------------------------------------------------------------------

/// GET /v1/threads — tenant-scoped thread list with limit/cursor/state/
/// sourceKind filters (Go handleThreadList).
async fn list_threads(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(query): Query<ThreadListQuery>,
) -> Result<Json<dope_threads::ThreadListResponse>, ApiError> {
    let tc = require_thread_permission(tenant.as_ref().map(|e| &e.0), Permission::CredentialsInspect)?;
    // Go parseThreadLifecycleLimit: unparseable/zero limits default to 20 (the
    // store applies that default).
    let limit = query
        .limit
        .as_deref()
        .and_then(|raw| raw.parse::<i64>().ok())
        .unwrap_or(0);
    let store_query = dope_store::ThreadListQuery {
        tenant_id: tc.tenant_id.clone(),
        limit,
        cursor: query.cursor.unwrap_or_default(),
        state_filter: query.state.unwrap_or_default(),
        source_filter: query.source_kind.unwrap_or_default(),
    };
    let response = state
        .store
        .lock()
        .list_threads_for_tenant(&store_query)
        .map_err(ApiError::from_store)?;
    Ok(Json(response))
}

/// GET /v1/threads/{thread_id} — full operator detail view (Go
/// handleThreadDetail). The additive bindingProjection is skipped (store
/// LatestRuntimeBindingEvidence not ported); the base response matches Go for
/// callers without bindings.inspect.
async fn thread_detail(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Path(thread_id): Path<String>,
) -> Result<Json<dope_threads::ThreadDetailResponse>, ApiError> {
    let tc = require_thread_permission(tenant.as_ref().map(|e| &e.0), Permission::CredentialsInspect)?;
    let mut response = state
        .store
        .lock()
        .get_thread_detail_for_tenant(&tc.tenant_id, &thread_id)
        .map_err(ApiError::from_store)?
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))?;
    // Go canInspectProfileRuntime: without profiles.inspect the active-profile
    // projections are stripped from the detail and its handoff links.
    if !can_inspect_profile_runtime(tenant.as_ref().map(|e| &e.0)) {
        response.active_profile_projection = None;
        for link in &mut response.handoff_links {
            link.active_profile_projection = None;
        }
    }
    Ok(Json(response))
}

/// POST /v1/threads/{thread_id}/reset — reset lifecycle mutation.
async fn thread_reset(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Path(thread_id): Path<String>,
    body: Bytes,
) -> Result<Json<ThreadLifecycleActionResponse>, ApiError> {
    thread_lifecycle_action(state, tenant, thread_id, dope_threads::LifecycleActionKind::Reset, body).await
}

/// POST /v1/threads/{thread_id}/archive — archive lifecycle mutation.
async fn thread_archive(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Path(thread_id): Path<String>,
    body: Bytes,
) -> Result<Json<ThreadLifecycleActionResponse>, ApiError> {
    thread_lifecycle_action(state, tenant, thread_id, dope_threads::LifecycleActionKind::Archive, body).await
}

/// POST /v1/threads/{thread_id}/reopen — reopen lifecycle mutation.
async fn thread_reopen(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Path(thread_id): Path<String>,
    body: Bytes,
) -> Result<Json<ThreadLifecycleActionResponse>, ApiError> {
    thread_lifecycle_action(state, tenant, thread_id, dope_threads::LifecycleActionKind::Reopen, body).await
}

/// Shared body of the three lifecycle mutations (Go
/// handleThreadLifecycleAction): apply the mutation with audit evidence,
/// publish the lifecycle event (plus the scoped-reset evidence event for
/// resets), and answer the threadLifecycleActionResponse.
async fn thread_lifecycle_action(
    state: AppState,
    tenant: Option<Extension<TenantContext>>,
    thread_id: String,
    kind: dope_threads::LifecycleActionKind,
    body: Bytes,
) -> Result<Json<ThreadLifecycleActionResponse>, ApiError> {
    let tc = require_thread_permission(tenant.as_ref().map(|e| &e.0), Permission::ConnectorsManage)?;
    // Go decodeJSONBody: empty body -> 400 "request body is required".
    let input: ThreadLifecycleActionRequest = decode_json_body(&body)?;
    let now = Utc::now();
    let audit_event_id = format!(
        "audit_thread_{}_{}",
        lifecycle_action_kind_str(kind),
        now.timestamp_nanos_opt().unwrap_or_default()
    );
    let mutation_input = dope_threads::LifecycleMutationInput {
        actor_principal_id: tc.principal_id.clone(),
        reason_code: coalesce_reason(&input.reason_code, lifecycle_action_kind_str(kind)),
        audit_event_id: audit_event_id.clone(),
        now: Some(now),
        new_segment_id: String::new(),
    };
    let result = state
        .store
        .lock()
        .apply_thread_lifecycle_action(&tc.tenant_id, &thread_id, kind, &mutation_input)
        .map_err(|message| {
            // Go: ThreadAuditFailedClosedEvent for audit-evidence and
            // concurrent-mutation failures, before the error response.
            if is_audit_failed_closed(&message) {
                let _ = publish_thread_event(
                    &state,
                    &tc.tenant_id,
                    events::thread_audit_failed_closed_event(&tc.tenant_id, &thread_id, &message),
                );
            }
            map_thread_lifecycle_error(message)
        })?;
    let Some(result) = result else {
        return Err(ApiError::NotFound("not found".to_string()));
    };
    publish_thread_event(&state, &tc.tenant_id, events::thread_lifecycle_event(result.action.clone()))?;
    if kind == dope_threads::LifecycleActionKind::Reset {
        // Go: ListResetEventsForThread(limit 1) -> ThreadScopedResetEvidenceEvent.
        let reset = state
            .store
            .lock()
            .list_reset_events_for_thread(&tc.tenant_id, &thread_id, 1)
            .map_err(ApiError::from_store)?
            .into_iter()
            .next();
        if let Some(reset) = reset {
            publish_thread_event(&state, &tc.tenant_id, events::thread_scoped_reset_evidence_event(reset))?;
        }
    }
    Ok(Json(ThreadLifecycleActionResponse {
        thread_id: result.thread.thread_id.clone(),
        lifecycle_state: result.thread.lifecycle_state,
        previous_session_segment_id: result.action.prior_session_segment_id.clone(),
        current_session_segment_id: result.thread.current_session_segment_id.clone(),
        audit_event_id: result.action.audit_event_id.clone(),
        changed_at: result.action.completed_at,
        action: kind,
        available_actions: dope_threads::available_actions(result.thread.lifecycle_state),
    }))
}

/// POST /v1/threads/{thread_id}/handoffs — thread handoff creation. The
/// persistence DAOs (save_handoff_link / save_handoff_source_references /
/// list_continuity_turns, plus the active-profile projection DAOs) are not
/// ported to dope-store, so this answers 501 after the connectors.manage gate
/// (Go handleThreadHandoffNotImplemented precedent).
async fn thread_handoff_create(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Path(thread_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = require_thread_permission(tenant.as_ref().map(|e| &e.0), Permission::ConnectorsManage)?;
    let _ = (state, thread_id, body);
    Ok(not_implemented(
        "thread_handoff_not_implemented",
        "thread handoff creation is not enabled yet",
    ))
}

/// GET /v1/threads/{thread_id}/continuity-previews/{preview_id} — continuity
/// preview detail. The store get_continuity_preview_detail DAO is not ported,
/// so this answers 501 after the credentials.inspect gate.
async fn thread_continuity_preview_detail(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Path((thread_id, preview_id)): Path<(String, String)>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = require_thread_permission(tenant.as_ref().map(|e| &e.0), Permission::CredentialsInspect)?;
    let _ = (state, thread_id, preview_id);
    Ok(not_implemented(
        "thread_continuity_preview_not_implemented",
        "thread continuity preview detail is not yet ported",
    ))
}

// ---------------------------------------------------------------------------
// Agent profiles (registered; DAOs not ported -> 501)
// ---------------------------------------------------------------------------

async fn list_profiles(
    State(state): State<AppState>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = state;
    Ok(profile_store_not_configured())
}

async fn create_profile(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, body);
    Ok(profile_store_not_configured())
}

async fn get_profile(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, profile_id);
    Ok(profile_store_not_configured())
}

async fn update_profile(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, profile_id, body);
    Ok(profile_store_not_configured())
}

async fn activate_profile(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, profile_id, body);
    Ok(profile_store_not_configured())
}

async fn list_profile_versions(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, profile_id);
    Ok(profile_store_not_configured())
}

async fn rollback_profile(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, profile_id, body);
    Ok(profile_store_not_configured())
}

async fn archive_profile(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, profile_id, body);
    Ok(profile_store_not_configured())
}

async fn disable_profile(
    State(state): State<AppState>,
    Path(profile_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, profile_id, body);
    Ok(profile_store_not_configured())
}

// ---------------------------------------------------------------------------
// Workspaces / bindings / capability visibility (registered; DAOs not ported
// -> 501)
// ---------------------------------------------------------------------------

async fn list_workspaces(
    State(state): State<AppState>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = state;
    Ok(binding_store_not_configured())
}

async fn create_workspace(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, body);
    Ok(binding_store_not_configured())
}

async fn get_workspace(
    State(state): State<AppState>,
    Path(workspace_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, workspace_id);
    Ok(binding_store_not_configured())
}

async fn update_workspace(
    State(state): State<AppState>,
    Path(workspace_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, workspace_id, body);
    Ok(binding_store_not_configured())
}

async fn list_bindings(
    State(state): State<AppState>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = state;
    Ok(binding_store_not_configured())
}

async fn create_binding(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, body);
    Ok(binding_store_not_configured())
}

async fn get_binding(
    State(state): State<AppState>,
    Path(binding_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, binding_id);
    Ok(binding_store_not_configured())
}

async fn update_binding(
    State(state): State<AppState>,
    Path(binding_id): Path<String>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, binding_id, body);
    Ok(binding_store_not_configured())
}

async fn delete_binding(
    State(state): State<AppState>,
    Path(binding_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, binding_id);
    Ok(binding_store_not_configured())
}

async fn repair_binding(
    State(state): State<AppState>,
    Path(binding_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, binding_id);
    Ok(binding_store_not_configured())
}

async fn list_capability_visibility(
    State(state): State<AppState>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = state;
    Ok(binding_store_not_configured())
}

async fn set_capability_visibility(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = (state, body);
    Ok(binding_store_not_configured())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Go requireThreadPermission: the caller needs a resolved tenant context with
/// the given permission; failures answer the stable credential denial (403).
fn require_thread_permission(
    tenant: Option<&TenantContext>,
    permission: Permission,
) -> Result<&dope_identity::TenantContext, ApiError> {
    let Some(tc) = tenant else {
        return Err(credential_denial());
    };
    if tc.0.tenant_id.trim().is_empty() || !has_permission(&tc.0.permissions, permission) {
        return Err(credential_denial());
    }
    Ok(&tc.0)
}

/// Go writeCredentialDenial(403, "permission_missing"): the stable error
/// string is credential_access_denied.
fn credential_denial() -> ApiError {
    ApiError::Forbidden("credential_access_denied".to_string())
}

/// Go canInspectProfileRuntime: profiles.inspect grants the runtime projection
/// visibility on thread detail.
fn can_inspect_profile_runtime(tenant: Option<&TenantContext>) -> bool {
    match tenant {
        Some(tc) => has_permission(&tc.0.permissions, Permission::ProfilesInspect),
        None => false,
    }
}

/// Go coalesceReason.
fn coalesce_reason(value: &str, fallback: &str) -> String {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        fallback.to_string()
    } else {
        trimmed.to_string()
    }
}

/// Wire string for a lifecycle action kind (Go's `string(kind)`).
fn lifecycle_action_kind_str(kind: dope_threads::LifecycleActionKind) -> &'static str {
    match kind {
        dope_threads::LifecycleActionKind::Reset => "reset",
        dope_threads::LifecycleActionKind::Archive => "archive",
        dope_threads::LifecycleActionKind::Reopen => "reopen",
    }
}

/// Go handleThreadLifecycleMutationError: audit-evidence failures surface as
/// 500 with the stable message; transition/conflict/reopen-eligibility
/// failures surface as 409 with the error text; everything else is 500.
fn map_thread_lifecycle_error(message: String) -> ApiError {
    if message == dope_threads::ThreadsError::AuditEvidenceRequired.to_string() {
        ApiError::Internal("thread lifecycle audit evidence is required".to_string())
    } else if message == dope_threads::ThreadsError::LifecycleTransitionNotAllowed.to_string()
        || message == dope_threads::ThreadsError::LifecycleMutationConflict.to_string()
        || message == dope_threads::ThreadsError::LifecycleReopenNotEligible.to_string()
    {
        ApiError::Conflict(message)
    } else {
        ApiError::from_store(message)
    }
}

/// Go: ThreadAuditFailedClosedEvent is published for audit-evidence and
/// concurrent-mutation failures.
fn is_audit_failed_closed(message: &str) -> bool {
    message == dope_threads::ThreadsError::AuditEvidenceRequired.to_string()
        || message == dope_threads::ThreadsError::LifecycleMutationConflict.to_string()
}

/// Go publishEvent (legacy store-append path then bus publish). The thread
/// event builders carry the tenant id; the environment scope comes from the
/// daemon config.
fn publish_thread_event(state: &AppState, _tenant_id: &str, event: events::Event) -> Result<(), ApiError> {
    let mut event = event;
    if event.environment_scope.is_empty() {
        event.environment_scope = crate::middleware::environment_scope_from_config(&state.config);
    }
    let stored = state
        .store
        .lock()
        .append_event(&event)
        .map_err(ApiError::from_store)?;
    state.event_bus.publish(stored);
    Ok(())
}

/// Go decodeJSONBody: an empty body maps to "request body is required" (400);
/// malformed JSON maps to the decoder error (400).
fn decode_json_body<T: serde::de::DeserializeOwned>(body: &Bytes) -> Result<T, ApiError> {
    if body.is_empty() {
        return Err(ApiError::BadRequest("request body is required".to_string()));
    }
    serde_json::from_slice(body).map_err(|err| ApiError::BadRequest(err.to_string()))
}

/// 501 `{error, code}` payload (Go writeActivationNotImplemented precedent).
fn not_implemented(code: &'static str, message: &'static str) -> (StatusCode, AxumJson<serde_json::Value>) {
    (
        StatusCode::NOT_IMPLEMENTED,
        AxumJson(serde_json::json!({ "error": message, "code": code })),
    )
}

/// 501 for the agent-profile family: the profile DAOs are not ported to
/// dope-store (Go maps the nil-store analogue to 500 "profile store is not
/// configured"; 501 is the honest marker for the missing DAOs).
fn profile_store_not_configured() -> (StatusCode, AxumJson<serde_json::Value>) {
    not_implemented(
        "profile_store_not_configured",
        "agent profile store DAOs are not yet ported",
    )
}

/// 501 for the workspace/binding/capability-visibility families (Go nil-store
/// analogue: 500 "binding store is not configured").
fn binding_store_not_configured() -> (StatusCode, AxumJson<serde_json::Value>) {
    not_implemented(
        "binding_store_not_configured",
        "workspace/binding store DAOs are not yet ported",
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    use std::sync::Arc;

    use axum::body::{to_bytes, Body};
    use axum::http::Request;
    use axum::http::header::CONTENT_TYPE;
    use chrono::{Duration, TimeZone};
    use dope_identity::Permission;
    use dope_threads as threads;
    use parking_lot::Mutex;
    use tower::ServiceExt;
    use uuid::Uuid;

    fn test_config() -> dope_config::Config {
        dope_config::Config {
            environment: dope_config::Environment::Test,
            bind_addr: "127.0.0.1:19192".to_string(),
            data_dir: "/tmp/dope-api-resources".to_string(),
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

    fn test_state() -> AppState {
        let dir = std::env::temp_dir().join(format!("dope-api-resources-{}", Uuid::now_v7()));
        std::fs::create_dir_all(&dir).expect("mkdir");
        let store = Arc::new(Mutex::new(
            dope_store::SQLiteStore::new(dir.to_str().expect("path")).expect("store"),
        ));
        AppState::new(test_config(), Arc::new(dope_events::Bus::new()), store)
    }

    fn request(method: &str, uri: &str, body: Option<&str>) -> Request<Body> {
        let builder = Request::builder()
            .method(method)
            .uri(uri)
            .header(CONTENT_TYPE, "application/json");
        let req = match body {
            Some(payload) => builder.body(Body::from(payload.to_string())).expect("request"),
            None => builder.body(Body::empty()).expect("request"),
        };
        req
    }

    /// Builds a request with a resolved tenant context extension (the 
    /// protected() middleware installs this once auth is wired; tests inject it
    /// directly, matching reminders.rs).
    fn tenant_request(
        method: &str,
        uri: &str,
        body: Option<&str>,
        tenant_id: &str,
        permissions: Vec<Permission>,
    ) -> Request<Body> {
        let mut req = request(method, uri, body);
        req.extensions_mut().insert(TenantContext(dope_identity::TenantContext {
            tenant_id: tenant_id.to_string(),
            principal_id: format!("prn_{tenant_id}"),
            permissions,
            ..Default::default()
        }));
        req
    }

    async fn send(app: &axum::Router, req: Request<Body>) -> (StatusCode, serde_json::Value) {
        let response = app.clone().oneshot(req).await.expect("oneshot");
        let status = response.status();
        let bytes = to_bytes(response.into_body(), usize::MAX).await.expect("body");
        // axum's default 404 (route miss) has an empty body; ApiError responses
        // carry the {code,message,error} envelope.
        let json = if bytes.is_empty() {
            serde_json::Value::Null
        } else {
            serde_json::from_slice(&bytes).expect("json body")
        };
        (status, json)
    }

    fn seed_thread(state: &AppState, thread: &threads::Thread) {
        state.store.lock().upsert_thread(thread).expect("upsert thread");
    }

    fn thread(
        id: &str,
        tenant: &str,
        lifecycle: threads::LifecycleState,
        source: threads::SourceKind,
        summary: &str,
        last_activity: chrono::DateTime<chrono::Utc>,
        created: chrono::DateTime<chrono::Utc>,
    ) -> threads::Thread {
        threads::Thread {
            thread_id: id.to_string(),
            tenant_id: tenant.to_string(),
            lifecycle_state: lifecycle,
            current_session_segment_id: format!("seg_{id}"),
            source_kind: source,
            source_summary: summary.to_string(),
            last_activity_at: last_activity,
            created_at: created,
            updated_at: last_activity,
            retention_expires_at: Some(created + Duration::days(90)),
            redaction_status: threads::RedactionStatus::Redacted,
        }
    }

    // Port of TestThreadLifecycleListDetailPaginationAndDenial.
    #[tokio::test]
    async fn thread_list_detail_pagination_and_denial() {
        let state = test_state();
        // Base the fixture on the real clock (minus a small offset) so the
        // 90-day retention expiries the store derives are still in the future.
        let now = chrono::Utc::now() - Duration::minutes(5);
        seed_thread(&state, &thread("thr_active", "ten_threads", threads::LifecycleState::Active, threads::SourceKind::Channel, "Slack Main / #support", now + Duration::minutes(1), now));
        seed_thread(&state, &thread("thr_archived", "ten_threads", threads::LifecycleState::Archived, threads::SourceKind::Workflow, "Workflow", now + Duration::minutes(2), now));
        seed_thread(&state, &thread("thr_other", "ten_other", threads::LifecycleState::Active, threads::SourceKind::Channel, "Other", now + Duration::minutes(3), now));
        {
            let store = state.store.lock();
            store
                .save_thread_source_linkage(&threads::SourceLinkage {
                    source_linkage_id: "src_active".to_string(),
                    thread_id: "thr_active".to_string(),
                    tenant_id: "ten_threads".to_string(),
                    source_kind: threads::SourceKind::Channel,
                    connector_id: "slack-main".to_string(),
                    connector_kind: "slack".to_string(),
                    source_account_id: "workspace_redacted".to_string(),
                    source_conversation_id: "channel_redacted".to_string(),
                    source_message_id: "msg_redacted".to_string(),
                    routing_outcome: threads::RoutingOutcome::Accepted,
                    current: true,
                    linked_at: Some(now),
                    retention_expires_at: None,
                    redaction_status: threads::RedactionStatus::Redacted,
                })
                .expect("save source linkage");
            store
                .save_thread_runtime_projection(&threads::RuntimeProjection {
                    runtime_projection_id: "rtp_run_active".to_string(),
                    thread_id: "thr_active".to_string(),
                    tenant_id: "ten_threads".to_string(),
                    session_segment_id: "seg_thr_active".to_string(),
                    resource_kind: threads::RuntimeResourceKind::Run,
                    resource_id: "run_1".to_string(),
                    status: "completed".to_string(),
                    reason_code: "accepted".to_string(),
                    occurred_at: now,
                    route: "/v1/runs/run_1".to_string(),
                    safe_summary: "Assistant run completed".to_string(),
                    retention_expires_at: None,
                    redaction_status: threads::RedactionStatus::Redacted,
                })
                .expect("save runtime projection");
            let shape = threads::resolve_conversation_shape(&threads::ConversationShapeResolutionInput {
                tenant_id: "ten_threads".to_string(),
                thread_id: "thr_active".to_string(),
                session_segment_id: "seg_thr_active".to_string(),
                source_kind: threads::SourceKind::Channel,
                connector_id: "slack-main".to_string(),
                connector_kind: "slack".to_string(),
                source_account_id: "workspace_redacted".to_string(),
                source_conversation_id: "channel_redacted".to_string(),
                source_conversation_summary: "Slack Main / #support".to_string(),
                claimed_shape: Some(threads::ConversationShape::Room),
                now: Some(now),
            });
            store.save_conversation_shape_evidence(&shape).expect("save shape");
            store
                .save_participation_decision(&threads::ParticipationDecision {
                    participation_decision_id: String::new(),
                    tenant_id: "ten_threads".to_string(),
                    thread_id: "thr_active".to_string(),
                    session_segment_id: "seg_thr_active".to_string(),
                    connector_id: "slack-main".to_string(),
                    connector_kind: "slack".to_string(),
                    source_account_id: "workspace_redacted".to_string(),
                    source_conversation_id: "channel_redacted".to_string(),
                    source_message_id: "msg_redacted".to_string(),
                    conversation_shape: threads::ConversationShape::Room,
                    policy_id: String::new(),
                    mention_status: threads::MentionStatus::Missing,
                    allowlist_status: threads::AllowlistStatus::Eligible,
                    decision: threads::ParticipationDecisionValue::Ignored,
                    reason_code: threads::GROUP_ROOM_REASON_MISSING_QUALIFYING_MENTION.to_string(),
                    created_assistant_work: false,
                    occurred_at: Some(now),
                    retention_expires_at: None,
                    redaction_status: threads::RedactionStatus::Redacted,
                    safe_summary: "Room message ignored by participation policy".to_string(),
                })
                .expect("save participation decision");
        }
        let app = crate::routes::router(state.clone());

        // First page: limit 1 -> active thread (archived sorts last).
        let req = tenant_request("GET", "/v1/threads?limit=1", None, "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "list body: {json}");
        assert_eq!(json["tenantId"], "ten_threads");
        assert_eq!(json["items"].as_array().map(|v| v.len()), Some(1));
        assert_eq!(json["items"][0]["threadId"], "thr_active");
        assert_ne!(json["page"]["nextCursor"], "");

        // Second page via cursor.
        let cursor = json["page"]["nextCursor"].as_str().unwrap();
        let req = tenant_request("GET", &format!("/v1/threads?limit=1&cursor={cursor}"), None, "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "next page body: {json}");
        assert_eq!(json["items"][0]["threadId"], "thr_archived");

        // State + source-kind filters.
        let req = tenant_request("GET", "/v1/threads?state=archived&sourceKind=workflow", None, "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "filtered body: {json}");
        assert_eq!(json["items"].as_array().map(|v| v.len()), Some(1));
        assert_eq!(json["items"][0]["threadId"], "thr_archived");

        // Detail with the full operator trace.
        let req = tenant_request("GET", "/v1/threads/thr_active", None, "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "detail body: {json}");
        assert_eq!(json["thread"]["threadId"], "thr_active");
        assert_eq!(json["thread"]["tenantId"], "ten_threads");
        assert_eq!(json["sourceLinkages"].as_array().map(|v| v.len()), Some(1));
        assert_eq!(json["sourceLinkages"][0]["routingOutcome"], "accepted");
        assert_eq!(json["runtimeProjections"].as_array().map(|v| v.len()), Some(1));
        assert_eq!(json["runtimeProjections"][0]["resourceKind"], "run");
        assert_eq!(json["conversationShape"]["shape"], "room");
        assert_eq!(json["participationDecisions"].as_array().map(|v| v.len()), Some(1));
        assert_eq!(json["participationDecisions"][0]["reasonCode"], "missing_qualifying_mention");
        let raw = serde_json::to_string(&json).expect("marshal detail");
        for forbidden in ["semanticSummary", "recalledMemory", "contextPacking", "autonomousPruning"] {
            assert!(!raw.contains(forbidden), "detail leaked {forbidden}: {raw}");
        }

        // Denial without the inspect permission.
        let req = tenant_request("GET", "/v1/threads", None, "ten_threads", vec![]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::FORBIDDEN, "denied list body: {json}");
        assert_eq!(json["error"], "credential_access_denied");

        let req = tenant_request("GET", "/v1/threads/thr_active", None, "ten_threads", vec![]);
        let (status, _) = send(&app, req).await;
        assert_eq!(status, StatusCode::FORBIDDEN);

        // A tenant with no threads gets an empty page (not an error).
        let req = tenant_request("GET", "/v1/threads", None, "ten_empty", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "empty body: {json}");
        assert_eq!(json["tenantId"], "ten_empty");
        assert_eq!(json["items"].as_array().map(|v| v.len()), Some(0));

        // Missing thread -> 404.
        let req = tenant_request("GET", "/v1/threads/thr_missing", None, "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, _) = send(&app, req).await;
        assert_eq!(status, StatusCode::NOT_FOUND);
    }

    // Port of TestThreadLifecycleMutationsRequireManagePermissionAndPersistAudit.
    #[tokio::test]
    async fn thread_lifecycle_mutations_require_manage_permission_and_persist_audit() {
        let state = test_state();
        let now = chrono::Utc.with_ymd_and_hms(2026, 5, 11, 10, 0, 0).unwrap();
        seed_thread(&state, &thread("thr_mutate", "ten_threads", threads::LifecycleState::Active, threads::SourceKind::Channel, "Slack", now, now));
        {
            let store = state.store.lock();
            store
                .upsert_thread_session_segment(&threads::SessionSegment {
                    session_segment_id: "seg_thr_mutate".to_string(),
                    thread_id: "thr_mutate".to_string(),
                    tenant_id: "ten_threads".to_string(),
                    session_id: "sess_1".to_string(),
                    generation: 1,
                    state: "active".to_string(),
                    started_at: now,
                    ended_at: None,
                    last_active_at: now,
                    reset_from_session_segment_id: String::new(),
                    partial_evidence: false,
                })
                .expect("upsert segment");
            store
                .save_thread_source_linkage(&threads::SourceLinkage {
                    source_linkage_id: "src_mutate_current".to_string(),
                    thread_id: "thr_mutate".to_string(),
                    tenant_id: "ten_threads".to_string(),
                    source_kind: threads::SourceKind::Channel,
                    connector_id: "slack-main".to_string(),
                    connector_kind: "slack".to_string(),
                    source_account_id: "acct_redacted".to_string(),
                    source_conversation_id: "conv_redacted".to_string(),
                    source_message_id: String::new(),
                    routing_outcome: threads::RoutingOutcome::Accepted,
                    current: true,
                    linked_at: Some(now),
                    retention_expires_at: Some(now + Duration::days(90)),
                    redaction_status: threads::RedactionStatus::Redacted,
                })
                .expect("save source linkage");
            let shape = threads::resolve_conversation_shape(&threads::ConversationShapeResolutionInput {
                tenant_id: "ten_threads".to_string(),
                thread_id: "thr_mutate".to_string(),
                session_segment_id: "seg_thr_mutate".to_string(),
                source_kind: threads::SourceKind::Channel,
                connector_id: "slack-main".to_string(),
                connector_kind: "slack".to_string(),
                source_account_id: "acct_redacted".to_string(),
                source_conversation_id: "conv_redacted".to_string(),
                source_conversation_summary: "Slack / #support".to_string(),
                claimed_shape: Some(threads::ConversationShape::Room),
                now: Some(now),
            });
            store.save_conversation_shape_evidence(&shape).expect("save shape");
        }
        let app = crate::routes::router(state.clone());

        // Inspect-only callers cannot mutate.
        let req = tenant_request("POST", "/v1/threads/thr_mutate/archive", Some("{}"), "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::FORBIDDEN, "denied archive body: {json}");

        // Archive with connectors.manage.
        let req = tenant_request("POST", "/v1/threads/thr_mutate/archive", Some(r#"{"reasonCode":"operator_archive"}"#), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "archive body: {json}");
        assert_eq!(json["lifecycleState"], "archived");
        assert_eq!(json["action"], "archive");
        assert_ne!(json["auditEventId"], "");

        // Reopen an archived thread.
        let req = tenant_request("POST", "/v1/threads/thr_mutate/reopen", Some("{}"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "reopen body: {json}");
        assert_eq!(json["lifecycleState"], "reopened");

        // Reset publishes the scoped reset evidence event.
        let req = tenant_request("POST", "/v1/threads/thr_mutate/reset", Some("{}"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK, "reset body: {json}");
        assert_eq!(json["lifecycleState"], "reset");

        // The mutation trail persisted: 2 segments, 3 lifecycle actions, 1
        // scoped reset event.
        let detail = state
            .store
            .lock()
            .get_thread_detail_for_tenant("ten_threads", "thr_mutate")
            .expect("detail")
            .expect("found");
        assert_eq!(detail.thread.lifecycle_state, threads::LifecycleState::Reset);
        assert_eq!(detail.session_segments.len(), 2);
        assert_eq!(detail.lifecycle_actions.len(), 3);
        assert_eq!(detail.reset_events.len(), 1);
        assert_eq!(detail.reset_events[0].conversation_shape, threads::ConversationShape::Room);
        assert_eq!(detail.reset_events[0].permission_gate, "connectors.manage");

        let thread_events = state.event_bus.list(&dope_events::Filter {
            category: "thread".to_string(),
            ..Default::default()
        });
        assert!(
            thread_events
                .iter()
                .any(|event| event.name == dope_events::THREAD_RESET_SCOPED_NAME
                    && event.payload.get("conversationShape").and_then(|v| v.as_str()) == Some("room")),
            "expected thread.reset_scoped event with room shape, got {thread_events:?}"
        );
    }

    #[tokio::test]
    async fn thread_lifecycle_validation_404_and_conflict() {
        let state = test_state();
        let now = chrono::Utc.with_ymd_and_hms(2026, 5, 11, 10, 0, 0).unwrap();
        seed_thread(&state, &thread("thr_validate", "ten_threads", threads::LifecycleState::Active, threads::SourceKind::Channel, "Slack", now, now));
        seed_thread(&state, &thread("thr_other", "ten_other", threads::LifecycleState::Active, threads::SourceKind::Channel, "Other", now, now));
        {
            let store = state.store.lock();
            store
                .upsert_thread_session_segment(&threads::SessionSegment {
                    session_segment_id: "seg_thr_validate".to_string(),
                    thread_id: "thr_validate".to_string(),
                    tenant_id: "ten_threads".to_string(),
                    session_id: String::new(),
                    generation: 1,
                    state: "active".to_string(),
                    started_at: now,
                    ended_at: None,
                    last_active_at: now,
                    reset_from_session_segment_id: String::new(),
                    partial_evidence: false,
                })
                .expect("upsert segment");
        }
        let app = crate::routes::router(state.clone());

        // Empty body -> 400 "request body is required".
        let req = tenant_request("POST", "/v1/threads/thr_validate/archive", None, "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "empty body: {json}");
        assert_eq!(json["message"], "request body is required");

        // Malformed body -> 400.
        let req = tenant_request("POST", "/v1/threads/thr_validate/archive", Some("{not json"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, _) = send(&app, req).await;
        assert_eq!(status, StatusCode::BAD_REQUEST);

        // Unknown action segment -> 404 (axum route miss).
        let req = tenant_request("POST", "/v1/threads/thr_validate/frobnicate", Some("{}"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, _) = send(&app, req).await;
        assert_eq!(status, StatusCode::NOT_FOUND);

        // Missing thread -> 404.
        let req = tenant_request("POST", "/v1/threads/thr_missing/archive", Some("{}"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::NOT_FOUND, "missing body: {json}");

        // Cross-tenant mutation is scoped: ten_threads cannot touch ten_other's
        // thread (404, never leaked as 403/200).
        let req = tenant_request("POST", "/v1/threads/thr_other/archive", Some("{}"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, _) = send(&app, req).await;
        assert_eq!(status, StatusCode::NOT_FOUND);

        // Archiving an archived thread -> 409 lifecycle transition not allowed.
        let req = tenant_request("POST", "/v1/threads/thr_validate/archive", Some("{}"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, _) = send(&app, req).await;
        assert_eq!(status, StatusCode::OK);
        let req = tenant_request("POST", "/v1/threads/thr_validate/archive", Some("{}"), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::CONFLICT, "conflict body: {json}");
        assert_eq!(json["message"], "lifecycle transition is not allowed");
    }

    #[tokio::test]
    async fn handoff_and_continuity_gate_permission_then_501() {
        let state = test_state();
        let app = crate::routes::router(state.clone());

        // Without the manage permission the gate answers 403 before the 501.
        let req = tenant_request("POST", "/v1/threads/thr_1/handoffs", Some(r#"{"destination":{"surface":"web"}}"#), "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::FORBIDDEN, "handoff denied body: {json}");

        let req = tenant_request("GET", "/v1/threads/thr_1/continuity-previews/pv_1", None, "ten_threads", vec![]);
        let (status, _) = send(&app, req).await;
        assert_eq!(status, StatusCode::FORBIDDEN);

        // With the permission the unported surfaces answer 501.
        let req = tenant_request("POST", "/v1/threads/thr_1/handoffs", Some(r#"{"destination":{"surface":"web"}}"#), "ten_threads", vec![Permission::ConnectorsManage]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED, "handoff body: {json}");
        assert_eq!(json["code"], "thread_handoff_not_implemented");

        let req = tenant_request("GET", "/v1/threads/thr_1/continuity-previews/pv_1", None, "ten_threads", vec![Permission::CredentialsInspect]);
        let (status, json) = send(&app, req).await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED, "preview body: {json}");
        assert_eq!(json["code"], "thread_continuity_preview_not_implemented");
    }

    #[tokio::test]
    async fn unported_profile_binding_and_visibility_families_answer_501() {
        let state = test_state();
        let app = crate::routes::router(state.clone());

        // Agent profiles.
        for (method, uri, body) in [
            ("GET", "/v1/profiles", None),
            ("POST", "/v1/profiles", Some(r#"{"displayName":"Agent"}"#)),
            ("GET", "/v1/profiles/prof_1", None),
            ("PATCH", "/v1/profiles/prof_1", Some(r#"{"displayName":"Renamed"}"#)),
            ("POST", "/v1/profiles/prof_1/activate", Some("{}")),
            ("GET", "/v1/profiles/prof_1/versions", None),
            ("POST", "/v1/profiles/prof_1/rollback", Some("{}")),
            ("POST", "/v1/profiles/prof_1/archive", Some("{}")),
            ("POST", "/v1/profiles/prof_1/disable", Some("{}")),
        ] {
            let (status, json) = send(&app, request(method, uri, body)).await;
            assert_eq!(status, StatusCode::NOT_IMPLEMENTED, "{method} {uri}: {json}");
            assert_eq!(json["code"], "profile_store_not_configured", "{method} {uri}: {json}");
        }

        // Workspaces.
        for (method, uri, body) in [
            ("GET", "/v1/workspaces", None),
            ("POST", "/v1/workspaces", Some(r#"{"displayName":"Default"}"#)),
            ("GET", "/v1/workspaces/ws_1", None),
            ("PATCH", "/v1/workspaces/ws_1", Some(r#"{"status":"active"}"#)),
        ] {
            let (status, json) = send(&app, request(method, uri, body)).await;
            assert_eq!(status, StatusCode::NOT_IMPLEMENTED, "{method} {uri}: {json}");
            assert_eq!(json["code"], "binding_store_not_configured", "{method} {uri}: {json}");
        }

        // Bindings.
        for (method, uri, body) in [
            ("GET", "/v1/bindings", None),
            ("POST", "/v1/bindings", Some(r#"{"workspaceId":"ws_1"}"#)),
            ("GET", "/v1/bindings/b_1", None),
            ("PATCH", "/v1/bindings/b_1", Some(r#"{"status":"disabled"}"#)),
            ("DELETE", "/v1/bindings/b_1", None),
            ("POST", "/v1/bindings/b_1/repair", None),
        ] {
            let (status, json) = send(&app, request(method, uri, body)).await;
            assert_eq!(status, StatusCode::NOT_IMPLEMENTED, "{method} {uri}: {json}");
            assert_eq!(json["code"], "binding_store_not_configured", "{method} {uri}: {json}");
        }

        // Capability visibility.
        for (method, uri, body) in [
            ("GET", "/v1/capability-visibility?scopeKind=profile&scopeRef=prof_1", None),
            ("PUT", "/v1/capability-visibility", Some(r#"{"scopeKind":"profile","scopeRef":"prof_1","capabilityId":"cap_1","visibility":"visible"}"#)),
        ] {
            let (status, json) = send(&app, request(method, uri, body)).await;
            assert_eq!(status, StatusCode::NOT_IMPLEMENTED, "{method} {uri}: {json}");
            assert_eq!(json["code"], "binding_store_not_configured", "{method} {uri}: {json}");
        }
    }
}

