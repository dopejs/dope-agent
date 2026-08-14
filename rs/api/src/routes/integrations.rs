//! integrations route family (port of daemon/internal/api/integrations.go +
//! integration_diagnostics.go + the `/v1/integrations*` registrations in
//! server.go).
//!
//! Surface (Go parity):
//! - `GET/POST /v1/integrations` — list (tenant-scoped when a tenant context
//!   is resolved) / create. Fully ported on dope-integrations Manager +
//!   dope-store integrations DAO + integration.registered event.
//! - `GET/DELETE /v1/integrations/{id}` — detail / disconnect. The by-id
//!   tenant guard (Go withByIDTenantGuard on integrations.integration_id) is
//!   applied inline. Fully ported (manager.disconnect + integration.disconnected).
//! - `POST /v1/integrations/{id}/readiness` — readiness report (Go
//!   handleIntegrationReadiness): manager.update_readiness + persist +
//!   integration.updated / integration.readiness_changed.
//! - `POST /v1/integrations/{id}/default` — canonical-default selection (Go
//!   handleIntegrationDefault): persists every sibling in the same binding
//!   group + integration.updated / integration.default_changed.
//! - `/v1/integrations/{id}/diagnostics` + `{id}/diagnostics/runs` — the
//!   diagnostics list/create dispatch (Go handleIntegrationDiagnostics). The
//!   tenant/permission gates and cross-tenant non-disclosure (404) are fully
//!   ported; the handlers then answer 501 because the dope-store diagnostic
//!   DAOs (SaveIntegrationDiagnosticResult / SaveIntegrationDiagnosticRun /
//!   LatestIntegrationDiagnosticResults) are not ported yet (migrations r42
//!   create the tables; the DAO layer is a follow-up).
//! - `/v1/integration-diagnostics/runs` + `/runs/{run_id}` — list/detail
//!   (Go handleIntegrationDiagnosticRuns). Gated + 501 (store DAOs missing).
//! - `/v1/integration-diagnostics/smoke` — smoke report (Go
//!   handleIntegrationDiagnosticSmoke): full tenant/permission/risky-probe
//!   gating, then 501 (needs SaveSmokeMatrixReport + dope-opsreadiness
//!   BuildIntegrationDiagnosticSmokeReport; both unported in this crate).
//! - `/v1/integration-diagnostics/retention/apply` — gated + 501 (store DAO
//!   ApplyExpiredDiagnosticRetentionRecords missing).
//! - `/v1/integration-diagnostics/reason-codes` — fully ported
//!   (default_diagnostic_reason_code_catalog()).
//! - `/v1/integrations/sync` + `/v1/integrations/{id}/adapter-rpc` —
//!   registered as 501 markers: the Go daemon has no HTTP handlers for these
//!   (adapter_rpc is an integrations.BackendKind used by the calendar/mail
//!   managers, not a REST surface; the manager-restore-from-store sync has no
//!   Go route either). No Go behavior exists to port, so they answer 501
//!   rather than inventing API surface.
//!
//! Go maps the nil-manager analogue to 500; a nil sqliteStore is impossible
//! here (AppState.store is required). Status codes / DTOs / validation mirror
//! the Go handlers: empty body -> 400, unknown integration -> 404, manager
//! validation failures -> 400, cross-tenant by-id access -> 404 (never
//! disclosed as 403).

use std::collections::HashMap;

use axum::body::Bytes;
use axum::extract::{Extension, Path, Query, State};
use axum::http::{Method, StatusCode, Uri};
use axum::routing::{get, post};
use axum::{Json as AxumJson, Router};
use chrono::{DateTime, Utc};

use dope_events as events;
use dope_identity::{can_inspect_credentials, has_permission, Permission};

use crate::error::ApiError;
use crate::middleware::{environment_scope_from_config, guard_resource_for_tenant, TenantContext};
use crate::response::Json;
use crate::state::AppState;
use crate::types::{
    CreateIntegrationRequest, IntegrationListResponse, ListResponse,
    ReportIntegrationReadinessRequest,
};

/// Route family router. Only the methods the Go handlers accept are
/// registered; axum answers the other methods with 405 (Go
/// w.WriteHeader(http.StatusMethodNotAllowed)).
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        // /v1/integrations (Go handleIntegrations).
        .route(
            "/v1/integrations",
            get(list_integrations).post(create_integration),
        )
        // /v1/integrations/ (Go handleIntegrationRoutes, wrapped in the
        // by-id tenant guard on integrations.integration_id).
        .route(
            "/v1/integrations/{integration_id}",
            get(get_integration).delete(disconnect_integration),
        )
        .route(
            "/v1/integrations/{integration_id}/readiness",
            post(update_integration_readiness),
        )
        .route(
            "/v1/integrations/{integration_id}/default",
            post(set_integration_default),
        )
        // /v1/integrations/{id}/diagnostics (Go handleIntegrationDiagnostics).
        .route(
            "/v1/integrations/{integration_id}/diagnostics",
            get(integration_diagnostic_list),
        )
        .route(
            "/v1/integrations/{integration_id}/diagnostics/runs",
            post(create_integration_diagnostic_run),
        )
        // /v1/integration-diagnostics/* (Go server.go registrations).
        .route(
            "/v1/integration-diagnostics/runs",
            get(list_integration_diagnostic_runs),
        )
        .route(
            "/v1/integration-diagnostics/runs/{run_id}",
            get(get_integration_diagnostic_run),
        )
        .route(
            "/v1/integration-diagnostics/smoke",
            post(run_integration_diagnostic_smoke),
        )
        .route(
            "/v1/integration-diagnostics/retention/apply",
            post(apply_integration_diagnostic_retention),
        )
        .route(
            "/v1/integration-diagnostics/reason-codes",
            get(integration_diagnostic_reason_codes),
        )
        // Rust-surface placeholders with no Go handler source (see module
        // docs): static segments win over the {integration_id} capture.
        .route("/v1/integrations/sync", post(integration_sync))
        .route(
            "/v1/integrations/{integration_id}/adapter-rpc",
            post(integration_adapter_rpc),
        )
}

// ---------------------------------------------------------------------------
// /v1/integrations — list / create (Go handleIntegrations)
// ---------------------------------------------------------------------------

/// GET /v1/integrations — full list, or the caller's tenant list when a
/// tenant context is resolved. Tenant reads additionally require
/// credentials.inspect or integrations.manage (Go
/// requireHostedCredentialReadAny).
async fn list_integrations(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<Json<IntegrationListResponse>, ApiError> {
    let manager = integrations_manager(&state)?;
    let items = match tenant {
        Some(tc) if !tc.0 .0.tenant_id.trim().is_empty() => {
            if !can_inspect_credentials(&tc.0 .0, &[Permission::IntegrationsManage]) {
                return Err(credential_denial());
            }
            manager.list_for_tenant(&tc.0 .0.tenant_id)
        }
        _ => manager.list(),
    };
    Ok(Json(IntegrationListResponse { items }))
}

/// POST /v1/integrations — create a resource (Go handleIntegrations POST
/// branch). Tenant mutations require integrations.manage; the resource is
/// persisted to the store and announced with integration.registered.
async fn create_integration(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<(StatusCode, Json<dope_integrations::Resource>), ApiError> {
    let manager = integrations_manager(&state)?;
    let mut tenant_id = String::new();
    if let Some(tc) = tenant.as_ref() {
        if !tc.0 .0.tenant_id.trim().is_empty() {
            require_permission(&tc.0 .0, Permission::IntegrationsManage)?;
            tenant_id = tc.0 .0.tenant_id.clone();
        }
    }
    let input: CreateIntegrationRequest = decode_json_body(&body)?;
    let item = manager
        .create(dope_integrations::CreateInput {
            tenant_id: tenant_id.clone(),
            integration_id: input.integration_id.clone(),
            domain_kind: input.domain_kind.clone(),
            display_name: input.display_name.clone(),
            account_binding: input.account_binding.clone(),
            backend_binding: dope_integrations::BackendBinding {
                backend_kind: input.backend_kind,
                backend_ref_id: input.backend_ref_id.clone(),
                backend_display_name: input.backend_display_name.clone(),
                source_kind: String::new(),
                supports_interactive_auth: false,
                // Go handleIntegrations: fake-local always supports probe
                // reads; otherwise the backend kind must support the domain.
                supports_probe_read: input.backend_kind
                    == dope_integrations::BackendKind::FakeLocal
                    || dope_integrations::backend_kind_supports_domain(
                        input.backend_kind,
                        &input.domain_kind,
                    ),
                supports_probe_mutation: input.backend_kind
                    == dope_integrations::BackendKind::FakeLocal,
            },
            canonical_default: input.canonical_default,
            environment_scope: environment_scope_from_config(&state.config),
        })
        .map_err(|err| ApiError::BadRequest(err.to_string()))?;
    persist_integration(&state, &item)?;
    publish_integration_event(
        &state,
        tenant.as_ref().map(|e| &e.0),
        "integration.registered",
        &item,
    )?;
    Ok((StatusCode::CREATED, Json(item)))
}

// ---------------------------------------------------------------------------
// /v1/integrations/{id} — detail / disconnect (Go handleIntegrationRoutes)
// ---------------------------------------------------------------------------

/// GET /v1/integrations/{integration_id} — one resource. Tenant reads use
/// get_for_tenant (cross-tenant rows 404, never disclosed).
async fn get_integration(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    method: Method,
    uri: Uri,
    Path(integration_id): Path<String>,
) -> Result<Json<dope_integrations::Resource>, ApiError> {
    let manager = integrations_manager(&state)?;
    guard_integration_resource(
        &state,
        &method,
        &uri,
        tenant.as_ref().map(|e| &e.0),
        &integration_id,
    )
    .await?;
    let item = match tenant {
        Some(tc) if !tc.0 .0.tenant_id.trim().is_empty() => {
            if !can_inspect_credentials(&tc.0 .0, &[Permission::IntegrationsManage]) {
                return Err(credential_denial());
            }
            manager.get_for_tenant(&integration_id, &tc.0 .0.tenant_id)
        }
        _ => manager.get(&integration_id),
    };
    item.ok_or_else(|| ApiError::NotFound("not found".to_string()))
        .map(Json)
}

/// DELETE /v1/integrations/{integration_id} — disconnect (Go
/// handleIntegrationDisconnect). The reason query param defaults to "operator
/// disconnected integration".
async fn disconnect_integration(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    method: Method,
    uri: Uri,
    Path(integration_id): Path<String>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<dope_integrations::Resource>, ApiError> {
    let manager = integrations_manager(&state)?;
    guard_integration_resource(
        &state,
        &method,
        &uri,
        tenant.as_ref().map(|e| &e.0),
        &integration_id,
    )
    .await?;
    if let Some(tc) = tenant.as_ref() {
        if !tc.0 .0.tenant_id.trim().is_empty() {
            require_permission(&tc.0 .0, Permission::IntegrationsManage)?;
        }
    }
    let reason = params
        .get("reason")
        .map(|raw| raw.trim().to_string())
        .filter(|trimmed| !trimmed.is_empty())
        .unwrap_or_else(|| "operator disconnected integration".to_string());
    let item = manager
        .disconnect(&integration_id, &reason)
        .map_err(map_integration_error)?;
    persist_integration(&state, &item)?;
    publish_integration_event(
        &state,
        tenant.as_ref().map(|e| &e.0),
        "integration.disconnected",
        &item,
    )?;
    Ok(Json(item))
}

// ---------------------------------------------------------------------------
// /v1/integrations/{id}/readiness (Go handleIntegrationReadiness)
// ---------------------------------------------------------------------------

/// POST /v1/integrations/{integration_id}/readiness — report readiness/
/// auth/health state. Mutations require integrations.manage and the resource
/// must belong to the caller's tenant (Go GetForTenant -> 404).
async fn update_integration_readiness(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    method: Method,
    uri: Uri,
    Path(integration_id): Path<String>,
    body: Bytes,
) -> Result<Json<dope_integrations::Resource>, ApiError> {
    let manager = integrations_manager(&state)?;
    guard_integration_resource(
        &state,
        &method,
        &uri,
        tenant.as_ref().map(|e| &e.0),
        &integration_id,
    )
    .await?;
    match tenant.as_ref() {
        Some(tc) if !tc.0 .0.tenant_id.trim().is_empty() => {
            require_permission(&tc.0 .0, Permission::IntegrationsManage)?;
            if manager
                .get_for_tenant(&integration_id, &tc.0 .0.tenant_id)
                .is_none()
            {
                return Err(ApiError::NotFound("not found".to_string()));
            }
        }
        _ => {
            if manager.get(&integration_id).is_none() {
                return Err(ApiError::NotFound("not found".to_string()));
            }
        }
    }
    let input: ReportIntegrationReadinessRequest = decode_json_body(&body)?;
    let item = manager
        .update_readiness(
            &integration_id,
            dope_integrations::UpdateReadinessInput {
                readiness_status: input.readiness_status,
                auth_state: input.auth_state.as_str().to_string(),
                health_state: input.health_state.as_str().to_string(),
                reason: input.reason.clone(),
                required_operator_action: input.required_operator_action.clone(),
                account_binding: Some(input.account_binding.clone()),
                secret_resolution: input.secret_resolution.clone(),
            },
        )
        .map_err(map_integration_error)?;
    persist_integration(&state, &item)?;
    publish_integration_event(
        &state,
        tenant.as_ref().map(|e| &e.0),
        "integration.updated",
        &item,
    )?;
    publish_integration_event(
        &state,
        tenant.as_ref().map(|e| &e.0),
        "integration.readiness_changed",
        &item,
    )?;
    Ok(Json(item))
}

// ---------------------------------------------------------------------------
// /v1/integrations/{id}/default (Go handleIntegrationDefault)
// ---------------------------------------------------------------------------

/// POST /v1/integrations/{integration_id}/default — promote the resource to
/// canonical default, demoting siblings in the same binding group (same
/// domain kind + environment scope + account key) and persisting every member
/// of the group.
async fn set_integration_default(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    method: Method,
    uri: Uri,
    Path(integration_id): Path<String>,
) -> Result<Json<dope_integrations::Resource>, ApiError> {
    let manager = integrations_manager(&state)?;
    guard_integration_resource(
        &state,
        &method,
        &uri,
        tenant.as_ref().map(|e| &e.0),
        &integration_id,
    )
    .await?;
    let tenant_id = match tenant.as_ref() {
        Some(tc) if !tc.0 .0.tenant_id.trim().is_empty() => {
            require_permission(&tc.0 .0, Permission::IntegrationsManage)?;
            if manager
                .get_for_tenant(&integration_id, &tc.0 .0.tenant_id)
                .is_none()
            {
                return Err(ApiError::NotFound("not found".to_string()));
            }
            tc.0 .0.tenant_id.clone()
        }
        _ => {
            if manager.get(&integration_id).is_none() {
                return Err(ApiError::NotFound("not found".to_string()));
            }
            String::new()
        }
    };
    let item = manager
        .set_canonical_default(&integration_id)
        .map_err(map_integration_error)?;
    // Go: persist every integration in the same binding group (the demotion
    // already happened inside the manager; persist the group so the store
    // matches the in-memory state).
    let to_persist = if tenant_id.is_empty() {
        manager.list()
    } else {
        manager.list_for_tenant(&tenant_id)
    };
    for integration in to_persist {
        if same_binding_group(&integration, &item) {
            persist_integration(&state, &integration)?;
        }
    }
    publish_integration_event(
        &state,
        tenant.as_ref().map(|e| &e.0),
        "integration.updated",
        &item,
    )?;
    publish_integration_event(
        &state,
        tenant.as_ref().map(|e| &e.0),
        "integration.default_changed",
        &item,
    )?;
    Ok(Json(item))
}

// ---------------------------------------------------------------------------
// Integration diagnostics (Go integration_diagnostics.go)
// ---------------------------------------------------------------------------

/// GET /v1/integrations/{integration_id}/diagnostics — latest diagnostic
/// state. Gates are fully ported (tenant context, diagnostics.read
/// permission, cross-tenant non-disclosure); the store result DAO is not
/// ported yet, so an allowed request answers 501.
async fn integration_diagnostic_list(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    method: Method,
    uri: Uri,
    Path(integration_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let manager = integrations_manager(&state)?;
    guard_integration_resource(
        &state,
        &method,
        &uri,
        tenant.as_ref().map(|e| &e.0),
        &integration_id,
    )
    .await?;
    let tc = require_tenant(tenant.as_ref().map(|e| &e.0))?;
    require_permission(tc, Permission::IntegrationDiagnosticsRead)?;
    // Go handleIntegrationDiagnosticList: GetForTenant before touching the
    // store — cross-tenant lookups 404 without disclosing the row.
    if manager
        .get_for_tenant(&integration_id, &tc.tenant_id)
        .is_none()
    {
        return Err(ApiError::NotFound("not found".to_string()));
    }
    Ok(not_implemented(
        "integration_diagnostic_store_not_ported",
        "integration diagnostic result DAOs are not yet ported",
    ))
}

/// POST /v1/integrations/{integration_id}/diagnostics/runs — start a
/// diagnostic run (Go handleCreateIntegrationDiagnosticRun). Gates ported
/// (diagnostics.run permission, cross-tenant 404); the run/result DAOs are
/// not ported yet, so an allowed request answers 501.
async fn create_integration_diagnostic_run(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    method: Method,
    uri: Uri,
    Path(integration_id): Path<String>,
    _body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let manager = integrations_manager(&state)?;
    guard_integration_resource(
        &state,
        &method,
        &uri,
        tenant.as_ref().map(|e| &e.0),
        &integration_id,
    )
    .await?;
    let tc = require_tenant(tenant.as_ref().map(|e| &e.0))?;
    require_permission(tc, Permission::IntegrationDiagnosticsRun)?;
    if manager
        .get_for_tenant(&integration_id, &tc.tenant_id)
        .is_none()
    {
        return Err(ApiError::NotFound("not found".to_string()));
    }
    Ok(not_implemented(
        "integration_diagnostic_store_not_ported",
        "integration diagnostic run DAOs are not yet ported",
    ))
}

/// GET /v1/integration-diagnostics/runs — diagnostic-run list (Go
/// handleIntegrationDiagnosticRuns list branch). Gated; the store
/// list/get run DAOs are not ported yet -> 501.
async fn list_integration_diagnostic_runs(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    _query: Query<HashMap<String, String>>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = &state;
    let tc = require_tenant(tenant.as_ref().map(|e| &e.0))?;
    require_permission(tc, Permission::IntegrationDiagnosticsRead)?;
    Ok(not_implemented(
        "integration_diagnostic_store_not_ported",
        "integration diagnostic run DAOs are not yet ported",
    ))
}

/// GET /v1/integration-diagnostics/runs/{run_id} — run detail (Go
/// handleIntegrationDiagnosticRuns detail branch). Gated; store DAO missing
/// -> 501.
async fn get_integration_diagnostic_run(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Path(_run_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = &state;
    let tc = require_tenant(tenant.as_ref().map(|e| &e.0))?;
    require_permission(tc, Permission::IntegrationDiagnosticsRead)?;
    Ok(not_implemented(
        "integration_diagnostic_store_not_ported",
        "integration diagnostic run DAOs are not yet ported",
    ))
}

/// POST /v1/integration-diagnostics/smoke — smoke report (Go
/// handleIntegrationDiagnosticSmoke). The tenant/permission gates and the
/// risky-probe gate are fully ported; the report needs
/// SaveSmokeMatrixReport + dope-opsreadiness (unported here) -> 501.
async fn run_integration_diagnostic_smoke(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = &state;
    let tc = require_tenant(tenant.as_ref().map(|e| &e.0))?;
    require_permission(tc, Permission::IntegrationDiagnosticsSmoke)?;
    let input: crate::types::CreateIntegrationDiagnosticSmokeRequest = decode_json_body(&body)?;
    // Go smokeRequestContainsRiskyProbe: any probe that is not read-only or
    // reversible additionally requires diagnostics.smoke.risky.
    if smoke_request_contains_risky_probe(&input) {
        require_permission(tc, Permission::IntegrationDiagnosticsSmokeRisky)?;
    }
    Ok(not_implemented(
        "integration_diagnostic_smoke_not_ported",
        "integration diagnostic smoke report persistence is not yet ported",
    ))
}

/// POST /v1/integration-diagnostics/retention/apply — apply expired retention
/// (Go handleIntegrationDiagnosticRetentionApply). Gated; the store DAO is
/// not ported yet -> 501.
async fn apply_integration_diagnostic_retention(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = &state;
    let tc = require_tenant(tenant.as_ref().map(|e| &e.0))?;
    require_permission(tc, Permission::IntegrationDiagnosticsRun)?;
    Ok(not_implemented(
        "integration_diagnostic_store_not_ported",
        "integration diagnostic retention DAO is not yet ported",
    ))
}

/// GET /v1/integration-diagnostics/reason-codes — the reason-code catalog
/// (Go handleIntegrationDiagnosticReasonCodes). Fully ported.
async fn integration_diagnostic_reason_codes(
) -> Result<Json<ListResponse<dope_integrations::DiagnosticReasonCodeDefinition>>, ApiError> {
    Ok(Json(ListResponse {
        items: dope_integrations::default_diagnostic_reason_code_catalog(),
    }))
}

// ---------------------------------------------------------------------------
// Placeholder surfaces without a Go handler (see module docs)
// ---------------------------------------------------------------------------

/// POST /v1/integrations/sync — no Go route exists (the Go daemon restores
/// the manager from the store at startup; there is no sync endpoint). 501
/// marker until a spec defines the surface.
async fn integration_sync(
    State(state): State<AppState>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = &state;
    Ok(not_implemented(
        "integration_sync_not_implemented",
        "integration store sync has no Go handler to port",
    ))
}

/// POST /v1/integrations/{integration_id}/adapter-rpc — `adapter_rpc` is an
/// integrations.BackendKind (docs/runtime/integration-adapter-plane.md) used
/// by the calendar/mail managers; the Go API surface has no REST handler for
/// it. 501 marker.
async fn integration_adapter_rpc(
    State(state): State<AppState>,
    Path(_integration_id): Path<String>,
) -> Result<(StatusCode, AxumJson<serde_json::Value>), ApiError> {
    let _ = &state;
    Ok(not_implemented(
        "integration_adapter_rpc_not_implemented",
        "adapter rpc has no Go HTTP handler to port",
    ))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Go handleIntegrations / handleIntegrationRoutes nil-manager branch: 500
/// with the stable message.
fn integrations_manager(state: &AppState) -> Result<&dope_integrations::Manager, ApiError> {
    state
        .integrations
        .as_deref()
        .ok_or_else(|| ApiError::Internal("integrations manager is not configured".to_string()))
}

/// Go withByIDTenantGuard for the integrations table.
async fn guard_integration_resource(
    state: &AppState,
    method: &Method,
    uri: &Uri,
    tenant: Option<&TenantContext>,
    integration_id: &str,
) -> Result<(), ApiError> {
    let surface = format!("api:{} {}", method.as_str(), uri.path());
    guard_resource_for_tenant(
        state,
        tenant,
        &surface,
        "integrations",
        "integration_id",
        integration_id,
        "integration",
    )
    .await
}

/// Go requireHostedCredentialPermission: resolved tenant + exact permission.
fn require_permission(
    tc: &dope_identity::TenantContext,
    permission: Permission,
) -> Result<(), ApiError> {
    if !has_permission(&tc.permissions, permission) {
        return Err(credential_denial());
    }
    Ok(())
}

/// Go tenant-context-required gate for the diagnostics handlers
/// (writeCredentialDenial(403, "tenant_context_missing")).
fn require_tenant(
    tenant: Option<&TenantContext>,
) -> Result<&dope_identity::TenantContext, ApiError> {
    let Some(tc) = tenant else {
        return Err(credential_denial());
    };
    if tc.0.tenant_id.trim().is_empty() {
        return Err(credential_denial());
    }
    Ok(&tc.0)
}

/// Go writeCredentialDenial: the stable error string is
/// `credential_access_denied` (the Go body also carries a reasonCode; the
/// existing route-family ports use the stable string alone).
fn credential_denial() -> ApiError {
    ApiError::Forbidden("credential_access_denied".to_string())
}

/// Go handleIntegrationDisconnect / readiness / default error mapping:
/// IntegrationNotFound -> 404, everything else -> 400.
fn map_integration_error(err: dope_integrations::IntegrationError) -> ApiError {
    match err {
        dope_integrations::IntegrationError::IntegrationNotFound => {
            ApiError::NotFound("not found".to_string())
        }
        other => ApiError::BadRequest(other.to_string()),
    }
}

/// Go handleIntegrationDefault binding group: same domain kind, environment
/// scope and account key.
fn same_binding_group(
    left: &dope_integrations::Resource,
    right: &dope_integrations::Resource,
) -> bool {
    left.domain_kind.trim() == right.domain_kind.trim()
        && left.environment_scope.trim() == right.environment_scope.trim()
        && account_key(left) == account_key(right)
}

fn account_key(item: &dope_integrations::Resource) -> String {
    item.account_binding
        .as_ref()
        .map(|binding| binding.account_key.trim().to_string())
        .unwrap_or_default()
}

/// Go persistIntegration: upsert the resource document (the Rust store writes
/// the tenant column as NULL until tenancy wiring; the document carries the
/// tenant id).
fn persist_integration(
    state: &AppState,
    item: &dope_integrations::Resource,
) -> Result<(), ApiError> {
    state
        .store
        .lock()
        .upsert_integration(item)
        .map_err(ApiError::from_store)
}

/// Go publishEvent for the integration category: binds environment scope +
/// tenant, persists (tenant-owned path), then publishes on the bus. The
/// integration category is not global, so a resolved tenant is bound.
fn publish_integration_event(
    state: &AppState,
    tenant: Option<&TenantContext>,
    name: &str,
    item: &dope_integrations::Resource,
) -> Result<(), ApiError> {
    let account_key = account_key(item);
    let payload = match name {
        "integration.registered" | "integration.updated" => serde_json::json!({
            "integrationId": item.integration_id,
            "domainKind": item.domain_kind,
            "displayName": item.display_name,
            "environmentScope": item.environment_scope,
            "readinessStatus": item.readiness_status,
            "canonicalDefault": item.canonical_default,
            "backendKind": item.backend_binding.backend_kind,
            "accountKey": account_key,
        }),
        "integration.readiness_changed" => serde_json::json!({
            "integrationId": item.integration_id,
            "readinessStatus": item.readiness_status,
            "authState": item.auth_state,
            "healthState": item.health_state,
            "reason": item.readiness_reason,
            "requiredOperatorAction": item.required_operator_action,
            "accountKey": account_key,
            "backendKind": item.backend_binding.backend_kind,
        }),
        "integration.disconnected" => serde_json::json!({
            "tenantId": item.tenant_id,
            "integrationId": item.integration_id,
            "readinessStatus": item.readiness_status,
            "authState": item.auth_state,
            "healthState": item.health_state,
            "disabledReason": item.disabled_reason,
        }),
        "integration.default_changed" => serde_json::json!({
            "integrationId": item.integration_id,
            "domainKind": item.domain_kind,
            "environmentScope": item.environment_scope,
            "accountKey": account_key,
            "canonicalDefault": item.canonical_default,
        }),
        _ => serde_json::json!({}),
    };
    publish_event(
        state,
        tenant,
        events::Event {
            category: "integration".to_string(),
            name: name.to_string(),
            resource: events::Resource {
                kind: "integration".to_string(),
                id: item.integration_id.clone(),
            },
            payload: payload.as_object().cloned().unwrap_or_default(),
            ..events::Event::default()
        },
    )
}

/// Go publishEvent (see calendar.rs for the shared shape): bind environment
/// scope + tenant, persist (tenant-owned or global path), then bus publish.
fn publish_event(
    state: &AppState,
    tenant: Option<&TenantContext>,
    event: events::Event,
) -> Result<(), ApiError> {
    let mut prepared = event;
    if prepared.environment_scope.is_empty() {
        prepared.environment_scope = environment_scope_from_config(&state.config);
    }
    if prepared.tenant_id.is_empty() {
        if let Some(tc) = tenant {
            if !tc.0.tenant_id.is_empty() && !events::is_global_category(&prepared.category) {
                prepared.tenant_id = tc.0.tenant_id.clone();
            }
        }
    }
    if prepared.event_id.is_empty() {
        prepared.event_id = new_event_id();
    }
    if prepared.occurred_at == DateTime::<Utc>::MIN_UTC {
        prepared.occurred_at = Utc::now();
    }
    let persisted = if prepared.tenant_id.is_empty() {
        state.store.lock().append_event(&prepared)
    } else {
        state
            .store
            .lock()
            .append_event_for_tenant_raw(&prepared, &prepared.tenant_id)
    }
    .map_err(ApiError::from_store)?;
    let _ = state.event_bus.publish(persisted);
    Ok(())
}

fn new_event_id() -> String {
    let hex = uuid::Uuid::new_v4().simple().to_string();
    format!("evt_{}", &hex[..16])
}

/// Go smokeRequestContainsRiskyProbe: any probe that is not read-only or
/// reversible makes the request "risky".
fn smoke_request_contains_risky_probe(
    input: &crate::types::CreateIntegrationDiagnosticSmokeRequest,
) -> bool {
    input
        .probes
        .iter()
        .any(|probe| !probe.read_only_or_reversible)
}

/// Go decodeJSONBody: an empty body maps to "request body is required" (400);
/// malformed JSON maps to the decoder error (400).
fn decode_json_body<T: serde::de::DeserializeOwned>(body: &Bytes) -> Result<T, ApiError> {
    if body.is_empty() {
        return Err(ApiError::BadRequest("request body is required".to_string()));
    }
    serde_json::from_slice(body).map_err(|err| ApiError::BadRequest(err.to_string()))
}

/// 501 `{error, code}` payload (resources.rs precedent for registered-but-
/// unported surfaces).
fn not_implemented(
    code: &'static str,
    message: &'static str,
) -> (StatusCode, AxumJson<serde_json::Value>) {
    (
        StatusCode::NOT_IMPLEMENTED,
        AxumJson(serde_json::json!({ "error": message, "code": code })),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    use std::sync::Arc;

    use axum::body::{to_bytes, Body};
    use axum::http::header::CONTENT_TYPE;
    use axum::http::Request;
    use parking_lot::Mutex;
    use tower::ServiceExt;
    use uuid::Uuid;

    fn test_config() -> dope_config::Config {
        dope_config::Config {
            environment: dope_config::Environment::Test,
            bind_addr: "127.0.0.1:19192".to_string(),
            data_dir: "/tmp/dope-api-integrations".to_string(),
            log_level: "info".to_string(),
            version: "0.1.0".to_string(),
            llm: dope_config::LlmConfig::default(),
            connectors: dope_config::ConnectorConfig {
                discord: dope_config::DiscordConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
                telegram: dope_config::TelegramConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
                slack: dope_config::SlackConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
                matrix: dope_config::MatrixConnectorConfig {
                    enabled: false,
                    ..Default::default()
                },
            },
        }
    }

    fn test_state() -> AppState {
        let dir = std::env::temp_dir().join(format!("dope-api-integrations-{}", Uuid::now_v7()));
        std::fs::create_dir_all(&dir).expect("mkdir");
        let store = Arc::new(Mutex::new(
            dope_store::SQLiteStore::new(dir.to_str().expect("path")).expect("store"),
        ));
        AppState::new(test_config(), Arc::new(dope_events::Bus::new()), store)
    }

    fn with_manager(mut state: AppState) -> AppState {
        state.integrations = Some(Arc::new(dope_integrations::Manager::new("test")));
        state
    }

    fn request(method: &str, uri: &str, body: Option<&str>) -> Request<Body> {
        let builder = Request::builder()
            .method(method)
            .uri(uri)
            .header(CONTENT_TYPE, "application/json");
        let req = match body {
            Some(payload) => builder
                .body(Body::from(payload.to_string()))
                .expect("request"),
            None => builder.body(Body::empty()).expect("request"),
        };
        req
    }

    fn tenant_request(
        method: &str,
        uri: &str,
        body: Option<&str>,
        tenant_id: &str,
        permissions: Vec<Permission>,
    ) -> Request<Body> {
        let mut req = request(method, uri, body);
        req.extensions_mut()
            .insert(TenantContext(dope_identity::TenantContext {
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
        let bytes = to_bytes(response.into_body(), usize::MAX)
            .await
            .expect("body");
        let json = if bytes.is_empty() {
            serde_json::Value::Null
        } else {
            serde_json::from_slice(&bytes).expect("json body")
        };
        (status, json)
    }

    fn create_fixture(state: &AppState, tenant_id: &str, id: &str, display: &str) {
        let manager = state.integrations.as_ref().expect("manager");
        manager
            .create(dope_integrations::CreateInput {
                tenant_id: tenant_id.to_string(),
                integration_id: id.to_string(),
                domain_kind: "calendar".to_string(),
                display_name: display.to_string(),
                backend_binding: dope_integrations::BackendBinding {
                    backend_kind: dope_integrations::BackendKind::FakeLocal,
                    ..dope_integrations::BackendBinding::default()
                },
                ..dope_integrations::CreateInput::default()
            })
            .expect("create fixture");
    }

    /// Port of the Go list/create/readiness/default/disconnect flows.
    #[tokio::test]
    async fn integration_crud_flow() {
        let state = with_manager(test_state());
        let app = router().with_state(state.clone());

        // Create -> 201 with the persisted resource.
        let (status, json) = send(
            &app,
            request(
                "POST",
                "/v1/integrations",
                Some(r#"{"integrationId":"integration_main","domainKind":"calendar","displayName":"Main Calendar","backendKind":"fake_local","accountBinding":{"accountKey":"acct_main","knownAfterAuth":false}}"#),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED);
        assert_eq!(json["integrationId"], "integration_main");
        assert_eq!(json["environmentScope"], "test");
        assert_eq!(json["readinessStatus"], "not_configured");
        assert_eq!(json["backendBinding"]["backendKind"], "fake_local");
        assert_eq!(json["backendBinding"]["supportsProbeRead"], true);

        // List -> contains the created resource.
        let (status, json) = send(&app, request("GET", "/v1/integrations", None)).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["items"].as_array().expect("items").len(), 1);
        assert_eq!(json["items"][0]["integrationId"], "integration_main");

        // Get by id -> 200.
        let (status, json) = send(
            &app,
            request("GET", "/v1/integrations/integration_main", None),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["integrationId"], "integration_main");

        // Readiness -> 200 with the updated projection.
        let (status, json) = send(
            &app,
            request(
                "POST",
                "/v1/integrations/integration_main/readiness",
                Some(r#"{"readinessStatus":"healthy","authState":"authorized","healthState":"healthy","reason":"all good","secretResolution":"resolved"}"#),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["readinessStatus"], "healthy");
        assert_eq!(json["authState"], "authorized");
        assert_eq!(json["healthState"], "healthy");

        // Default -> 200 canonical default.
        let (status, json) = send(
            &app,
            request("POST", "/v1/integrations/integration_main/default", None),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["canonicalDefault"], true);

        // Disconnect -> 200 unavailable + disabled reason.
        let (status, json) = send(
            &app,
            request(
                "DELETE",
                "/v1/integrations/integration_main?reason=rotate",
                None,
            ),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["readinessStatus"], "unavailable");
        assert_eq!(json["authState"], "revoked");
        assert_eq!(json["disabledReason"], "rotate");

        // Get after disconnect -> 200 with the disconnected state.
        let (status, json) = send(
            &app,
            request("GET", "/v1/integrations/integration_main", None),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["readinessStatus"], "unavailable");

        // Store round-trip: the persisted document is reloadable.
        let stored = state
            .store
            .lock()
            .list_integrations("test")
            .expect("list store");
        assert_eq!(stored.len(), 1);
        assert_eq!(stored[0].integration_id, "integration_main");
        assert_eq!(
            stored[0].readiness_status,
            dope_integrations::ReadinessStatus::Unavailable
        );
    }

    #[tokio::test]
    async fn create_validation_maps_to_400() {
        let state = with_manager(test_state());
        let app = router().with_state(state.clone());
        let (status, json) = send(
            &app,
            request(
                "POST",
                "/v1/integrations",
                Some(r#"{"domainKind":"calendar"}"#),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST);
        assert_eq!(json["code"], "bad_request");
    }

    #[tokio::test]
    async fn missing_manager_returns_500() {
        let state = test_state();
        let app = router().with_state(state);
        let (status, json) = send(&app, request("GET", "/v1/integrations", None)).await;
        assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
        assert!(json["error"]
            .as_str()
            .unwrap_or("")
            .contains("integrations manager is not configured"));
    }

    /// Port of the permission + cross-tenant non-disclosure shape: a tenant
    /// without integrations.manage is denied 403 on the list; a tenant with
    /// the permission never sees another tenant's integration (404).
    #[tokio::test]
    async fn tenant_list_denial_and_cross_tenant_non_disclosure() {
        let state = with_manager(test_state());
        create_fixture(&state, "ten_a", "integration_a", "Tenant A Calendar");
        create_fixture(&state, "ten_b", "integration_b", "Tenant B Secret Calendar");
        let app = router().with_state(state.clone());

        // No permission -> 403 credential denial.
        let (status, json) = send(
            &app,
            tenant_request("GET", "/v1/integrations", None, "ten_a", vec![]),
        )
        .await;
        assert_eq!(status, StatusCode::FORBIDDEN);
        assert_eq!(json["error"], "credential_access_denied");

        // With integrations.manage -> tenant-scoped list only.
        let (status, json) = send(
            &app,
            tenant_request(
                "GET",
                "/v1/integrations",
                None,
                "ten_a",
                vec![Permission::IntegrationsManage],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        let items = json["items"].as_array().expect("items");
        assert_eq!(items.len(), 1);
        assert_eq!(items[0]["integrationId"], "integration_a");

        // Cross-tenant by-id get -> 404, without leaking the resource.
        let (status, body) = send(
            &app,
            tenant_request(
                "GET",
                "/v1/integrations/integration_b",
                None,
                "ten_a",
                vec![Permission::IntegrationsManage],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND);
        let raw = body.to_string();
        assert!(!raw.contains("Tenant B Secret Calendar") && !raw.contains("integration_b"));
    }

    /// Port of TestIntegrationDiagnosticsAPIRequiresPermissionAndDoesNotDiscloseCrossTenantState.
    #[tokio::test]
    async fn diagnostics_denial_and_cross_tenant_non_disclosure() {
        let state = with_manager(test_state());
        create_fixture(&state, "ten_a", "integration_a", "Tenant A Calendar");
        create_fixture(&state, "ten_b", "integration_b", "Tenant B Secret Calendar");
        let app = router().with_state(state.clone());

        // Tenant context without diagnostics.read -> 403.
        let (status, json) = send(
            &app,
            tenant_request(
                "GET",
                "/v1/integrations/integration_a/diagnostics",
                None,
                "ten_a",
                vec![],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::FORBIDDEN);
        assert_eq!(json["error"], "credential_access_denied");

        // With the permission, a cross-tenant integration -> 404 (never 403).
        let (status, body) = send(
            &app,
            tenant_request(
                "GET",
                "/v1/integrations/integration_b/diagnostics",
                None,
                "ten_a",
                vec![Permission::IntegrationDiagnosticsRead],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND);
        let raw = body.to_string();
        assert!(!raw.contains("Tenant B Secret Calendar") && !raw.contains("integration_b"));
    }

    /// The diagnostics result/run DAOs are not ported to dope-store, so an
    /// allowed request answers 501 after the gates (resources.rs precedent).
    #[tokio::test]
    async fn diagnostics_allowed_request_answers_501() {
        let state = with_manager(test_state());
        create_fixture(&state, "ten_a", "integration_a", "Tenant A Calendar");
        let app = router().with_state(state.clone());

        let (status, json) = send(
            &app,
            tenant_request(
                "GET",
                "/v1/integrations/integration_a/diagnostics",
                None,
                "ten_a",
                vec![Permission::IntegrationDiagnosticsRead],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
        assert_eq!(json["code"], "integration_diagnostic_store_not_ported");

        let (status, _json) = send(
            &app,
            tenant_request(
                "GET",
                "/v1/integration-diagnostics/runs",
                None,
                "ten_a",
                vec![Permission::IntegrationDiagnosticsRead],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED);

        let (status, json) = send(
            &app,
            tenant_request(
                "POST",
                "/v1/integration-diagnostics/smoke",
                Some(r#"{"reportId":"smoke_1","integrationId":"integration_a","probes":[{"probeAction":"calendar.read","safeCredentialsAvailable":true,"tenantApprovalAvailable":true,"providerAvailable":true,"supported":true,"readOnlyOrReversible":true}]}"#),
                "ten_a",
                vec![Permission::IntegrationDiagnosticsSmoke],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
        assert_eq!(json["code"], "integration_diagnostic_smoke_not_ported");
    }

    /// Smoke with a risky (mutating) probe requires the extra
    /// diagnostics.smoke.risky permission (Go smokeRequestContainsRiskyProbe).
    #[tokio::test]
    async fn smoke_risky_probe_requires_risky_permission() {
        let state = with_manager(test_state());
        create_fixture(&state, "ten_a", "integration_a", "Tenant A Calendar");
        let app = router().with_state(state.clone());
        let body = r#"{"reportId":"smoke_1","integrationId":"integration_a","probes":[{"probeAction":"calendar.write","safeCredentialsAvailable":true,"tenantApprovalAvailable":true,"providerAvailable":true,"supported":true,"readOnlyOrReversible":false}]}"#;
        let (status, json) = send(
            &app,
            tenant_request(
                "POST",
                "/v1/integration-diagnostics/smoke",
                Some(body),
                "ten_a",
                vec![Permission::IntegrationDiagnosticsSmoke],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::FORBIDDEN);
        assert_eq!(json["error"], "credential_access_denied");

        let (status, json) = send(
            &app,
            tenant_request(
                "POST",
                "/v1/integration-diagnostics/smoke",
                Some(body),
                "ten_a",
                vec![
                    Permission::IntegrationDiagnosticsSmoke,
                    Permission::IntegrationDiagnosticsSmokeRisky,
                ],
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
        assert_eq!(json["code"], "integration_diagnostic_smoke_not_ported");
    }

    /// Port of Go handleIntegrationDiagnosticReasonCodes: the catalog returns
    /// non-empty items.
    #[tokio::test]
    async fn reason_codes_catalog() {
        let state = test_state();
        let app = router().with_state(state);
        let (status, json) = send(
            &app,
            request("GET", "/v1/integration-diagnostics/reason-codes", None),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        let items = json["items"].as_array().expect("items");
        assert!(!items.is_empty());
        assert!(items.iter().any(|item| item["reasonCode"] == "healthy"));
        assert!(items
            .iter()
            .any(|item| item["reasonCode"] == "scope_missing"));
    }

    /// Readiness on an unknown integration -> 404 (Go GetForTenant/Get).
    #[tokio::test]
    async fn readiness_unknown_integration_404() {
        let state = with_manager(test_state());
        let app = router().with_state(state.clone());
        let (status, _) = send(
            &app,
            request(
                "POST",
                "/v1/integrations/integration_missing/readiness",
                Some(r#"{"readinessStatus":"healthy"}"#),
            ),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND);
    }

    /// The placeholder surfaces answer 501 and never invent behavior.
    #[tokio::test]
    async fn placeholder_routes_answer_501() {
        let state = with_manager(test_state());
        let app = router().with_state(state.clone());
        let (status, json) = send(&app, request("POST", "/v1/integrations/sync", None)).await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
        assert_eq!(json["code"], "integration_sync_not_implemented");

        let (status, json) = send(
            &app,
            request("POST", "/v1/integrations/integration_a/adapter-rpc", None),
        )
        .await;
        assert_eq!(status, StatusCode::NOT_IMPLEMENTED);
        assert_eq!(json["code"], "integration_adapter_rpc_not_implemented");
    }
}
