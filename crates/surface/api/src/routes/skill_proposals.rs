//! `/v1/skills/proposals` — agent-managed skills (design root: skills are
//! memory assets in the 058 envelope, kind=skill).
//!
//! The proposal lifecycle **rides the memory plane's governance wholesale**:
//! a proposal is a `kind=skill`, `layer=l1` memory asset — the L1 validator
//! forces motivating evidence (source links), and the default write policy
//! forces agent-authored writes into `Pending`, which lands the proposal in
//! the existing memory review queue. Approval is the existing
//! `/v1/memory/assets/{id}/approve`. What this family adds is the
//! **publication bridge**: only an approved (Ready) proposal can publish,
//! and publication is the sole path into the skills registry — it writes
//! `<data_dir>/skills/<id>/SKILL.md`, registers a catalog item (kind=skill,
//! Community trust), and reloads the registry. Unpublished proposals are
//! never loadable (the registry only scans the skills dir), which is the
//! runtime guard the design fixes.

use axum::body::Bytes;
use axum::extract::{Extension, Path, State};
use axum::routing::{get, post};
use axum::Router;
use chrono::Utc;
use serde::{Deserialize, Serialize};

use kura_memory as memory;

use crate::error::ApiError;
use crate::middleware::TenantContext;
use crate::response::Json;
use crate::state::AppState;

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct SkillProposalRequest {
    pub name: String,
    pub description: String,
    /// The skill instruction body (SKILL.md content below the frontmatter).
    pub body: String,
    pub tenant_id: String,
    /// Motivating evidence (conversation/run provenance). Required — the L1
    /// validator rejects proposals without it.
    pub evidence_links: Vec<memory::SourceLink>,
    /// Defaults to an agent actor; operator-authored proposals may say so.
    pub proposed_by: Option<memory::Actor>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SkillProposalResponse {
    pub asset: memory::MemoryAsset,
    pub decision: memory::WriteDecision,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SkillPublishResponse {
    pub skill_id: String,
    pub catalog_item_id: String,
    pub version: String,
}

/// Lowercase-kebab skill id from the proposal name (mirrors the registry's
/// normalization closely enough for directory naming).
fn skill_dir_id(name: &str) -> String {
    let id: String = name
        .trim()
        .to_lowercase()
        .chars()
        .map(|c| if c.is_alphanumeric() { c } else { '-' })
        .collect();
    id.trim_matches('-').to_string()
}

/// POST /v1/skills/proposals — draft a skill as a governed memory asset.
#[allow(clippy::unused_async)]
pub async fn propose(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
    body: Bytes,
) -> Result<Json<SkillProposalResponse>, ApiError> {
    let request: SkillProposalRequest = super::decode_json_required(&body)?;
    if request.name.trim().is_empty() || request.body.trim().is_empty() {
        return Err(ApiError::BadRequest("name and body are required".to_string()));
    }
    if skill_dir_id(&request.name).is_empty() {
        return Err(ApiError::BadRequest("name yields an empty skill id".to_string()));
    }
    let manager = state
        .memory
        .as_deref()
        .ok_or_else(|| ApiError::internal("memory manager is not configured"))?;
    let tenant_id = super::memory::route_tenant(&tenant, &request.tenant_id);
    let owner = request.proposed_by.clone().unwrap_or(memory::Actor {
        kind: memory::ActorKind::Agent,
        id: "agent".to_string(),
    });
    // The asset content IS the SKILL.md the publish step writes verbatim.
    let content = format!(
        "---\nname: {}\ndescription: {}\n---\n\n{}\n",
        request.name.trim(),
        request.description.trim(),
        request.body.trim()
    );
    let (asset, decision) = manager
        .create(memory::CreateAssetInput {
            kind: memory::AssetKind::Skill,
            layer: memory::MemoryLayer::L1,
            tenant_id,
            owner,
            visibility: memory::Visibility::Private,
            atom_type: Some(memory::AtomType::Reference),
            title: request.name.trim().to_string(),
            content,
            source_links: request.evidence_links,
            ..memory::CreateAssetInput::default()
        })
        .map_err(|err| ApiError::BadRequest(err.to_string()))?;
    super::memory::persist_capture(&state, &asset);
    Ok(Json(SkillProposalResponse { asset, decision }))
}

/// GET /v1/skills/proposals — every skill-kind asset for the tenant.
#[allow(clippy::unused_async)]
pub async fn list(
    State(state): State<AppState>,
    tenant: Option<Extension<TenantContext>>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let manager = state
        .memory
        .as_deref()
        .ok_or_else(|| ApiError::internal("memory manager is not configured"))?;
    let tenant_id = super::memory::route_tenant(&tenant, "");
    let mut items = manager.list(&tenant_id, Some(memory::MemoryLayer::L1), None);
    items.retain(|asset| asset.kind == memory::AssetKind::Skill);
    Ok(Json(serde_json::json!({ "items": items })))
}

/// POST /v1/skills/proposals/{asset_id}/publish — the only path from an
/// approved proposal into the runtime: skills dir + catalog + registry.
#[allow(clippy::unused_async)]
pub async fn publish(
    State(state): State<AppState>,
    Path(asset_id): Path<String>,
) -> Result<Json<SkillPublishResponse>, ApiError> {
    let manager = state
        .memory
        .as_deref()
        .ok_or_else(|| ApiError::internal("memory manager is not configured"))?;
    let Some(asset) = manager.get(&asset_id) else {
        return Err(ApiError::NotFound("skill proposal not found".to_string()));
    };
    if asset.kind != memory::AssetKind::Skill {
        return Err(ApiError::BadRequest("asset is not a skill proposal".to_string()));
    }
    if asset.status != memory::AssetStatus::Ready {
        return Err(ApiError::Conflict(format!(
            "proposal is {}; only approved (ready) proposals publish",
            asset.status.as_str()
        )));
    }
    let skills = state
        .skills
        .as_deref()
        .ok_or_else(|| ApiError::internal("skills registry is not configured"))?;
    let catalog = state
        .catalog
        .as_deref()
        .ok_or_else(|| ApiError::internal("catalog manager is not configured"))?;

    // Directory name is kebab; the registry's runtime id is the normalized
    // (lowercased) frontmatter name — the response carries the registry id.
    let dir_id = skill_dir_id(&asset.title);
    let skill_id = asset.title.trim().to_lowercase();
    let dir = std::path::Path::new(&state.config.data_dir)
        .join("skills")
        .join(&dir_id);
    std::fs::create_dir_all(&dir)
        .and_then(|()| std::fs::write(dir.join("SKILL.md"), asset.content.as_bytes()))
        .map_err(|err| ApiError::internal(&format!("write skill bundle: {err}")))?;

    let version = format!("{}.0.0", asset.version.max(1));
    let item = catalog
        .register_item(kura_catalog::CatalogItem {
            item_id: String::new(),
            kind: kura_catalog::ItemKind::Skill,
            name: asset.title.clone(),
            trust_tier: kura_catalog::TrustTier::Community,
            permissions: Vec::new(),
            versions: vec![kura_catalog::Version {
                version: version.clone(),
                source: format!("memory:{}", asset.asset_id),
                checksum: String::new(),
                requirements: Vec::new(),
                published_at: Utc::now(),
            }],
            created_at: Utc::now(),
            updated_at: Utc::now(),
        })
        .map_err(|err| ApiError::internal(&format!("catalog register: {err}")))?;
    skills
        .reload()
        .map_err(|err| ApiError::internal(&format!("skills reload: {err}")))?;

    // Audit event: the publication decision with its evidence chain.
    let mut payload = serde_json::Map::new();
    payload.insert("assetId".to_string(), serde_json::json!(asset.asset_id));
    payload.insert("skillId".to_string(), serde_json::json!(skill_id));
    payload.insert("catalogItemId".to_string(), serde_json::json!(item.item_id));
    payload.insert("version".to_string(), serde_json::json!(version));
    let event = kura_events::Event {
        category: "skill".to_string(),
        name: "skill.proposal_published".to_string(),
        resource: kura_events::Resource {
            kind: "skill".to_string(),
            id: skill_id.clone(),
        },
        payload,
        ..kura_events::Event::default()
    };
    let event = state.store.lock().append_event(&event).unwrap_or(event);
    state.event_bus.publish(event);

    Ok(Json(SkillPublishResponse {
        skill_id,
        catalog_item_id: item.item_id,
        version,
    }))
}

pub fn router() -> Router<AppState> {
    Router::new()
        .route("/v1/skills/proposals", post(propose).get(list))
        .route("/v1/skills/proposals/{asset_id}/publish", post(publish))
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use axum::http::StatusCode;

    use super::super::tests_support::{request_json, test_state};

    fn skill_state() -> crate::state::AppState {
        let mut state = test_state();
        let dir = std::env::temp_dir()
            .join(format!("kura-skillprop-{}", uuid::Uuid::now_v7()));
        std::fs::create_dir_all(&dir).expect("mkdir");
        state.config.data_dir = dir.to_string_lossy().into_owned();
        state.memory = Some(Arc::new(kura_memory::Manager::new("test", None, None, None)));
        state.skills = Some(Arc::new(
            kura_skills::Registry::with_roots(
                &dir.join("home").to_string_lossy(),
                &state.config.data_dir,
            )
            .expect("registry"),
        ));
        state.catalog = Some(Arc::new(kura_catalog::Manager::new("test", None, None)));
        state
    }

    #[tokio::test]
    async fn propose_review_publish_lifecycle_with_runtime_guard() {
        let state = skill_state();

        // Agent-authored proposal: forced Pending by the write policy.
        let (status, json) = request_json(
            state.clone(),
            "POST",
            "/v1/skills/proposals",
            Some(serde_json::json!({
                "name": "Deploy Checklist",
                "description": "run before every deploy",
                "body": "1. run tests\n2. check CI",
                "evidenceLinks": [{ "kind": "thread", "id": "thr_evidence" }]
            })),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{json}");
        assert!(
            json["decision"]["require_approval"].is_object(),
            "agent write forced into review: {json}"
        );
        let asset_id = json["asset"]["assetId"].as_str().expect("asset id").to_string();

        // Runtime guard: pending proposals are not loadable and not
        // publishable.
        assert!(
            state.skills.as_deref().unwrap().get("deploy checklist").is_none(),
            "pending proposal must not be loadable"
        );
        let (status, _) = request_json(
            state.clone(),
            "POST",
            &format!("/v1/skills/proposals/{asset_id}/publish"),
            Some(serde_json::json!({})),
        )
        .await;
        assert_eq!(status, StatusCode::CONFLICT, "unapproved publish refused");

        // Approve through the existing memory review, then publish.
        let (status, _) = request_json(
            state.clone(),
            "POST",
            &format!("/v1/memory/assets/{asset_id}/approve"),
            Some(serde_json::json!({ "actor": { "kind": "operator", "id": "op" } })),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        let (status, json) = request_json(
            state.clone(),
            "POST",
            &format!("/v1/skills/proposals/{asset_id}/publish"),
            Some(serde_json::json!({})),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{json}");
        assert_eq!(json["skillId"], "deploy checklist");
        assert!(!json["catalogItemId"].as_str().unwrap().is_empty());

        // Published: loadable from the registry with provenance intact in
        // the catalog version source.
        let skill = state
            .skills
            .as_deref()
            .unwrap()
            .get("deploy checklist")
            .expect("published skill loads");
        assert!(skill.body.contains("run tests"));
        let items = state.catalog.as_deref().unwrap().list_items();
        assert!(items.iter().any(|item| item
            .versions
            .iter()
            .any(|v| v.source == format!("memory:{asset_id}"))));

        // Listed as a proposal asset.
        let (status, json) =
            request_json(state, "GET", "/v1/skills/proposals", None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(json["items"][0]["kind"], "skill");
    }
}
