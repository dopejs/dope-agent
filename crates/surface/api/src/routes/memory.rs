//! memory route family (Roadmap 78, spec 058 — the memory plane foundation).
//!
//! Routes: GET/POST /v1/memory/assets, GET /v1/memory/assets/{asset_id},
//! GET /v1/memory/assets/{asset_id}/drilldown,
//! POST /v1/memory/assets/{asset_id}/{approve|reject|revoke|visibility},
//! POST /v1/memory/capture (L0 episode reference + turn bookkeeping),
//! POST /v1/memory/consolidate (manual consolidation trigger).
//!
//! Every mutation persists through the store DAO, publishes the matching
//! memory.* event, and re-renders the white-box Markdown projection for
//! ready L2/L3 assets under `<data_dir>/memory/`. A resolved tenant
//! context overrides any caller-supplied tenant.

use std::path::PathBuf;

use axum::body::Bytes;
use axum::extract::{Extension, Path, Query, State};
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::{Deserialize, Serialize};

use dope_events as events;
use dope_memory as memory;

use crate::error::ApiError;
use crate::middleware::{environment_scope_from_config, TenantContext};
use crate::state::AppState;

use super::{decode_json_or_default, decode_json_required};

/// Route family router.
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        .route("/v1/memory/assets", get(list_assets).post(create_asset))
        .route("/v1/memory/assets/{asset_id}", get(get_asset))
        .route("/v1/memory/assets/{asset_id}/drilldown", get(drilldown))
        .route("/v1/memory/assets/{asset_id}/approve", post(approve_asset))
        .route("/v1/memory/assets/{asset_id}/reject", post(reject_asset))
        .route("/v1/memory/assets/{asset_id}/revoke", post(revoke_asset))
        .route("/v1/memory/assets/{asset_id}/visibility", post(set_visibility))
        .route("/v1/memory/capture", post(capture))
        .route("/v1/memory/consolidate", post(consolidate))
}

#[derive(Debug, Serialize)]
struct AssetListResponse {
    items: Vec<memory::MemoryAsset>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct AssetDecisionResponse {
    asset: memory::MemoryAsset,
    decision: memory::WriteDecision,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct ListQuery {
    tenant_id: String,
    layer: String,
    status: String,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct ReviewRequest {
    actor: memory::Actor,
    reason: String,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct VisibilityRequest {
    visibility: Option<memory::Visibility>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct CaptureRequest {
    tenant_id: String,
    owner: memory::Actor,
    title: String,
    source_links: Vec<memory::SourceLink>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct ConsolidateRequest {
    tenant_id: String,
    trigger: String,
    window: Vec<memory::L0Item>,
}

fn manager(state: &AppState) -> Result<&memory::Manager, ApiError> {
    state
        .memory
        .as_deref()
        .ok_or_else(|| ApiError::internal("memory manager is not configured"))
}

fn map_memory_error(err: memory::MemoryError) -> ApiError {
    let message = err.to_string();
    match err {
        memory::MemoryError::AssetNotFound => ApiError::NotFound(message),
        memory::MemoryError::Rejected(_) => ApiError::Forbidden(message),
        memory::MemoryError::NotPending
        | memory::MemoryError::NotActive
        | memory::MemoryError::InvalidVisibilityChange => ApiError::Conflict(message),
        _ => ApiError::BadRequest(message),
    }
}

fn context_tenant(tenant: &Option<Extension<TenantContext>>, fallback: &str) -> String {
    if let Some(tc) = tenant.as_ref().map(|extension| &extension.0.0) {
        if !tc.tenant_id.trim().is_empty() {
            return tc.tenant_id.trim().to_string();
        }
    }
    fallback.trim().to_string()
}

fn persist_asset(state: &AppState, asset: &memory::MemoryAsset) -> Result<(), ApiError> {
    state
        .store
        .lock()
        .upsert_memory_asset(asset)
        .map_err(ApiError::from_store)
}

fn publish_memory_event(
    state: &AppState,
    name: &str,
    asset: &memory::MemoryAsset,
) -> Result<(), ApiError> {
    let mut payload = serde_json::Map::new();
    payload.insert("tenantId".to_string(), serde_json::json!(asset.tenant_id));
    payload.insert("kind".to_string(), serde_json::json!(asset.kind.as_str()));
    payload.insert("layer".to_string(), serde_json::json!(asset.layer.as_str()));
    payload.insert("status".to_string(), serde_json::json!(asset.status.as_str()));
    payload.insert("visibility".to_string(), serde_json::json!(asset.visibility.as_str()));
    payload.insert("version".to_string(), serde_json::json!(asset.version));
    if !asset.supersedes_asset_id.is_empty() {
        payload.insert(
            "supersedesAssetId".to_string(),
            serde_json::json!(asset.supersedes_asset_id),
        );
    }
    let event = events::Event {
        category: "memory".to_string(),
        name: name.to_string(),
        environment_scope: environment_scope_from_config(&state.config),
        resource: events::Resource {
            kind: "memory_asset".to_string(),
            id: asset.asset_id.clone(),
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

/// Writes the white-box Markdown projection for ready L2/L3 assets.
fn project_markdown(state: &AppState, asset: &memory::MemoryAsset) {
    if asset.status != memory::AssetStatus::Ready
        || !matches!(asset.layer, memory::MemoryLayer::L2 | memory::MemoryLayer::L3)
    {
        return;
    }
    let Ok(manager) = manager(state) else { return };
    let tenant_dir = if asset.tenant_id.is_empty() { "local" } else { &asset.tenant_id };
    let dir = PathBuf::from(&state.config.data_dir).join("memory").join(tenant_dir);
    if std::fs::create_dir_all(&dir).is_err() {
        return;
    }
    let _ = std::fs::write(
        dir.join(format!("{}.md", asset.asset_id)),
        manager.render_markdown(asset),
    );
}

fn finish_mutation(
    state: &AppState,
    event_name: &str,
    asset: &memory::MemoryAsset,
) -> Result<(), ApiError> {
    persist_asset(state, asset)?;
    publish_memory_event(state, event_name, asset)?;
    project_markdown(state, asset);
    Ok(())
}


// ---------------------------------------------------------------------------
// Reusable write-path helpers (spec 058 phase 2): the capture hooks in the
// chat/connector/workflow families and the app scheduler tick call these.
// ---------------------------------------------------------------------------

/// Persists + publishes + projects one manager-created asset (the
/// full-content capture path for context refs, which must not go through
/// capture_l0's excerpt truncation).
pub fn persist_capture(state: &AppState, asset: &memory::MemoryAsset) {
    if let Err(err) = finish_mutation(state, "memory.asset_written", asset) {
        eprintln!("memory: persist capture failed: {err:?}");
    }
}

/// Fire-and-forget L0 capture + turn bookkeeping. Content is a bounded
/// excerpt (truth stays behind the source links). Returns Some((asset id,
/// extraction due)) on success; failures log and return None — capture
/// never fails the originating request.
pub fn capture_l0(
    state: &AppState,
    tenant_id: &str,
    owner: memory::Actor,
    role: &str,
    text: &str,
    source_links: Vec<memory::SourceLink>,
) -> Option<(String, bool)> {
    let manager = state.memory.as_deref()?;
    let excerpt: String = text.chars().take(2000).collect();
    let result = manager.create(memory::CreateAssetInput {
        kind: memory::AssetKind::ChatMemory,
        layer: memory::MemoryLayer::L0Ref,
        tenant_id: tenant_id.trim().to_string(),
        owner,
        visibility: memory::Visibility::Private,
        title: role.trim().to_string(),
        content: excerpt,
        source_links,
        ..memory::CreateAssetInput::default()
    });
    match result {
        Ok((asset, _)) => {
            if let Err(err) = finish_mutation(state, "memory.asset_written", &asset) {
                eprintln!("memory: persist capture failed: {err:?}");
            }
            let due = manager.record_turn(tenant_id.trim(), chrono::Utc::now());
            Some((asset.asset_id, due))
        }
        Err(err) => {
            eprintln!("memory: capture failed: {err}");
            None
        }
    }
}

/// Runs one consolidation pass: builds the pending L0 window when none is
/// supplied, refines through the Consolidator, persists + publishes every
/// written asset, and records the run event.
pub fn execute_consolidation(
    state: &AppState,
    tenant_id: &str,
    trigger: &str,
    window: Option<Vec<memory::L0Item>>,
) -> Result<memory::ConsolidationRun, ApiError> {
    let manager = manager(state)?;
    let tenant_id = tenant_id.trim();
    let window = match window {
        Some(window) if !window.is_empty() => window,
        _ => manager.pending_l0_window(tenant_id),
    };
    let (run, written) = manager.consolidate(tenant_id, trigger, &window);
    for asset in &written {
        finish_mutation(state, "memory.asset_written", asset)?;
    }
    let mut payload = serde_json::Map::new();
    payload.insert("tenantId".to_string(), serde_json::json!(run.tenant_id));
    payload.insert("trigger".to_string(), serde_json::json!(run.trigger));
    payload.insert("extractedL1".to_string(), serde_json::json!(run.extracted_l1));
    payload.insert("aggregatedL2".to_string(), serde_json::json!(run.aggregated_l2));
    payload.insert("distilledL3".to_string(), serde_json::json!(run.distilled_l3));
    if !run.error.is_empty() {
        payload.insert("error".to_string(), serde_json::json!(run.error));
    }
    let event = events::Event {
        category: "memory".to_string(),
        name: "memory.consolidation_run".to_string(),
        environment_scope: environment_scope_from_config(&state.config),
        resource: events::Resource {
            kind: "memory_consolidation".to_string(),
            id: run.run_id.clone(),
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
    Ok(run)
}

/// The 60s scheduler tick: idle-triggered consolidation for every tenant
/// with bookkeeping, plus the retention sweep. Errors log; the tick never
/// aborts.
pub fn memory_tick(state: &AppState) {
    let Some(manager) = state.memory.as_deref() else { return };
    let now = chrono::Utc::now();
    for tenant in manager.tenants_with_bookkeeping() {
        if manager.idle_due(&tenant, now) {
            if let Err(err) = execute_consolidation(state, &tenant, "idle", None) {
                eprintln!("memory: idle consolidation for {tenant} failed: {err:?}");
            }
        }
    }
    for expired in manager.sweep_retention(now) {
        if let Err(err) = finish_mutation(state, "memory.asset_expired", &expired) {
            eprintln!("memory: retention persist failed: {err:?}");
        }
    }
}

/// GET /v1/memory/assets.
async fn list_assets(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    Query(query): Query<ListQuery>,
) -> Result<Json<AssetListResponse>, ApiError> {
    let manager = manager(&state)?;
    let tenant_id = context_tenant(&tenant, &query.tenant_id);
    let layer = if query.layer.trim().is_empty() {
        None
    } else {
        serde_json::from_value(serde_json::json!(query.layer.trim())).ok()
    };
    let status = if query.status.trim().is_empty() {
        None
    } else {
        serde_json::from_value(serde_json::json!(query.status.trim())).ok()
    };
    Ok(Json(AssetListResponse { items: manager.list(&tenant_id, layer, status) }))
}

/// POST /v1/memory/assets — a policy-gated write; 201 with the stored asset
/// and the policy decision.
async fn create_asset(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<(StatusCode, Json<AssetDecisionResponse>), ApiError> {
    let mut input: memory::CreateAssetInput = decode_json_required(&body)?;
    input.tenant_id = context_tenant(&tenant, &input.tenant_id);
    let manager = manager(&state)?;
    let (asset, decision) = manager.create(input).map_err(map_memory_error)?;
    finish_mutation(&state, "memory.asset_written", &asset)?;
    if !asset.supersedes_asset_id.is_empty() && asset.status == memory::AssetStatus::Ready {
        if let Some(previous) = manager.get(&asset.supersedes_asset_id) {
            finish_mutation(&state, "memory.asset_superseded", &previous)?;
        }
    }
    Ok((StatusCode::CREATED, Json(AssetDecisionResponse { asset, decision })))
}

/// GET /v1/memory/assets/{asset_id}.
async fn get_asset(
    State(state): State<AppState>,
    Path(asset_id): Path<String>,
) -> Result<Json<memory::MemoryAsset>, ApiError> {
    let manager = manager(&state)?;
    manager
        .get(asset_id.trim())
        .map(Json)
        .ok_or_else(|| ApiError::NotFound("not found".to_string()))
}

/// GET /v1/memory/assets/{asset_id}/drilldown — the deterministic path down
/// to L1 source links (the L0 citation).
async fn drilldown(
    State(state): State<AppState>,
    Path(asset_id): Path<String>,
) -> Result<Json<memory::DrilldownNode>, ApiError> {
    let manager = manager(&state)?;
    manager.drilldown(asset_id.trim()).map(Json).map_err(map_memory_error)
}

/// POST /v1/memory/assets/{asset_id}/approve.
async fn approve_asset(
    State(state): State<AppState>,
    Path(asset_id): Path<String>,
    body: Bytes,
) -> Result<Json<memory::MemoryAsset>, ApiError> {
    let request: ReviewRequest = decode_json_or_default(&body)?;
    let manager = manager(&state)?;
    let (asset, superseded) = manager
        .approve(asset_id.trim(), &request.actor)
        .map_err(map_memory_error)?;
    finish_mutation(&state, "memory.asset_written", &asset)?;
    if let Some(previous) = superseded {
        finish_mutation(&state, "memory.asset_superseded", &previous)?;
    }
    Ok(Json(asset))
}

/// POST /v1/memory/assets/{asset_id}/reject.
async fn reject_asset(
    State(state): State<AppState>,
    Path(asset_id): Path<String>,
    body: Bytes,
) -> Result<Json<memory::MemoryAsset>, ApiError> {
    let request: ReviewRequest = decode_json_or_default(&body)?;
    let manager = manager(&state)?;
    let asset = manager
        .reject(asset_id.trim(), &request.reason)
        .map_err(map_memory_error)?;
    finish_mutation(&state, "memory.asset_revoked", &asset)?;
    Ok(Json(asset))
}

/// POST /v1/memory/assets/{asset_id}/revoke — reversibility: tombstone.
async fn revoke_asset(
    State(state): State<AppState>,
    Path(asset_id): Path<String>,
    body: Bytes,
) -> Result<Json<memory::MemoryAsset>, ApiError> {
    let request: ReviewRequest = decode_json_or_default(&body)?;
    let manager = manager(&state)?;
    let asset = manager
        .revoke(asset_id.trim(), &request.reason)
        .map_err(map_memory_error)?;
    finish_mutation(&state, "memory.asset_revoked", &asset)?;
    Ok(Json(asset))
}

/// POST /v1/memory/assets/{asset_id}/visibility — narrowing applies
/// immediately; widening runs the policy gate.
async fn set_visibility(
    State(state): State<AppState>,
    Path(asset_id): Path<String>,
    body: Bytes,
) -> Result<Json<AssetDecisionResponse>, ApiError> {
    let request: VisibilityRequest = decode_json_required(&body)?;
    let visibility = request
        .visibility
        .ok_or_else(|| ApiError::BadRequest("visibility is required".to_string()))?;
    let manager = manager(&state)?;
    let (asset, decision) = manager
        .set_visibility(asset_id.trim(), visibility)
        .map_err(map_memory_error)?;
    finish_mutation(&state, "memory.asset_written", &asset)?;
    Ok(Json(AssetDecisionResponse { asset, decision }))
}

/// POST /v1/memory/capture — records an L0 episode reference and the turn
/// bookkeeping; responds with whether an extraction pass is now due.
async fn capture(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<(StatusCode, Json<serde_json::Value>), ApiError> {
    let request: CaptureRequest = decode_json_required(&body)?;
    let manager = manager(&state)?;
    let tenant_id = context_tenant(&tenant, &request.tenant_id);
    let (asset, _) = manager
        .create(memory::CreateAssetInput {
            kind: memory::AssetKind::ChatMemory,
            layer: memory::MemoryLayer::L0Ref,
            tenant_id: tenant_id.clone(),
            owner: request.owner,
            visibility: memory::Visibility::Private,
            title: request.title,
            content: String::new(),
            source_links: request.source_links,
            ..memory::CreateAssetInput::default()
        })
        .map_err(map_memory_error)?;
    finish_mutation(&state, "memory.asset_written", &asset)?;
    let extraction_due = manager.record_turn(&tenant_id, chrono::Utc::now());
    Ok((
        StatusCode::CREATED,
        Json(serde_json::json!({
            "asset": asset,
            "extractionDue": extraction_due,
        })),
    ))
}

/// POST /v1/memory/consolidate — the manual consolidation trigger; the L0
/// window defaults to the tenant's pending captures.
async fn consolidate(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<Json<memory::ConsolidationRun>, ApiError> {
    let request: ConsolidateRequest = decode_json_or_default(&body)?;
    let tenant_id = context_tenant(&tenant, &request.tenant_id);
    let trigger = if request.trigger.trim().is_empty() { "manual" } else { request.trigger.trim() };
    let window = if request.window.is_empty() { None } else { Some(request.window) };
    let run = execute_consolidation(&state, &tenant_id, trigger, window)?;
    Ok(Json(run))
}

#[cfg(test)]
mod tests {
    use super::super::tests_support::{request_json, test_state};
    use axum::http::StatusCode;
    use std::sync::Arc;

    fn state_with_manager() -> crate::state::AppState {
        let mut state = test_state();
        state.memory = Some(Arc::new(dope_memory::Manager::new("test", None, None, None)));
        state
    }

    fn atom_body(content: &str) -> serde_json::Value {
        serde_json::json!({
            "kind": "chat_memory",
            "layer": "l1",
            "owner": { "kind": "operator", "id": "op_1" },
            "atomType": "preference",
            "title": "reply language",
            "content": content,
            "sourceLinks": [{ "kind": "thread", "id": "thr_1", "excerpt": "用中文回复" }]
        })
    }

    #[tokio::test]
    async fn create_list_drilldown_and_revoke_atom() {
        let state = state_with_manager();
        let (status, created) = request_json(
            state.clone(),
            "POST",
            "/v1/memory/assets",
            Some(atom_body("prefers Chinese replies")),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "{created}");
        assert_eq!(created["decision"], "accept", "{created}");
        assert_eq!(created["asset"]["status"], "ready");
        let atom_id = created["asset"]["assetId"].as_str().expect("assetId").to_string();

        // L2 scenario over the atom; drill-down resolves to the atom's
        // source links.
        let (status, scenario) = request_json(
            state.clone(),
            "POST",
            "/v1/memory/assets",
            Some(serde_json::json!({
                "kind": "chat_memory",
                "layer": "l2",
                "owner": { "kind": "operator", "id": "op_1" },
                "title": "communication preferences",
                "content": "The user prefers Chinese replies.",
                "memberAssetIds": [atom_id]
            })),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "{scenario}");
        let scenario_id = scenario["asset"]["assetId"].as_str().expect("assetId").to_string();

        let (status, tree) = request_json(
            state.clone(),
            "GET",
            &format!("/v1/memory/assets/{scenario_id}/drilldown"),
            None,
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{tree}");
        assert_eq!(tree["members"][0]["asset"]["assetId"], atom_id.as_str());
        assert_eq!(
            tree["members"][0]["asset"]["sourceLinks"][0]["id"],
            "thr_1",
            "{tree}"
        );

        // The white-box Markdown projection exists on disk.
        let path = std::path::PathBuf::from(&state.config.data_dir)
            .join("memory")
            .join("local")
            .join(format!("{scenario_id}.md"));
        assert!(path.exists(), "markdown projection missing at {path:?}");

        let (status, listed) =
            request_json(state.clone(), "GET", "/v1/memory/assets?layer=l1", None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(listed["items"].as_array().expect("items").len(), 1);

        let (status, revoked) = request_json(
            state,
            "POST",
            &format!("/v1/memory/assets/{atom_id}/revoke"),
            Some(serde_json::json!({ "reason": "user asked to forget" })),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{revoked}");
        assert_eq!(revoked["status"], "revoked");
    }

    #[tokio::test]
    async fn agent_writes_require_approval_and_unattributed_writes_reject() {
        let state = state_with_manager();
        let mut body = atom_body("agent noticed a deadline");
        body["owner"] = serde_json::json!({ "kind": "agent", "id": "agent_main" });
        let (status, pending) =
            request_json(state.clone(), "POST", "/v1/memory/assets", Some(body)).await;
        assert_eq!(status, StatusCode::CREATED, "{pending}");
        assert_eq!(pending["asset"]["status"], "pending");
        let asset_id = pending["asset"]["assetId"].as_str().expect("assetId").to_string();

        let (status, approved) = request_json(
            state.clone(),
            "POST",
            &format!("/v1/memory/assets/{asset_id}/approve"),
            Some(serde_json::json!({ "actor": { "kind": "operator", "id": "op_1" } })),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{approved}");
        assert_eq!(approved["status"], "ready");

        // No source links -> rejected at the boundary.
        let mut unattributed = atom_body("floating claim");
        unattributed["sourceLinks"] = serde_json::json!([]);
        let (status, err) =
            request_json(state, "POST", "/v1/memory/assets", Some(unattributed)).await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "{err}");
    }

    #[tokio::test]
    async fn capture_and_manual_consolidation_record_runs() {
        let state = state_with_manager();
        let (status, captured) = request_json(
            state.clone(),
            "POST",
            "/v1/memory/capture",
            Some(serde_json::json!({
                "owner": { "kind": "system", "id": "chat" },
                "title": "session turn",
                "sourceLinks": [{ "kind": "message", "id": "msg_1" }]
            })),
        )
        .await;
        assert_eq!(status, StatusCode::CREATED, "{captured}");
        // Warm-up doubling: the first turn is immediately extraction-due.
        assert_eq!(captured["extractionDue"], true, "{captured}");

        let (status, run) = request_json(
            state,
            "POST",
            "/v1/memory/consolidate",
            Some(serde_json::json!({ "trigger": "manual" })),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{run}");
        assert_eq!(run["trigger"], "manual");
        // The Noop consolidator records the run with zero drafts.
        assert_eq!(run["extractedL1"], 0);
    }
}
