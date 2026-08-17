//! `/v1/retrieval/queries` — source-linked retrieval over the memory plane.
//!
//! The knowledge-retrieval query surface: the same fused ranking the
//! context plugin uses at `chat/pre-dispatch` (BM25 + recency + hashed
//! n-gram vector, RRF), exposed as an API so clients and agent tools can
//! recall memory on demand. Every hit carries its source links and
//! drill-down member ids — recalled results are evidence, never bare text.

use axum::body::Bytes;
use axum::extract::State;
use axum::routing::post;
use axum::Router;
use serde::{Deserialize, Serialize};

use crate::error::ApiError;
use crate::response::Json;
use crate::state::AppState;

#[derive(Debug, Clone, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
pub struct RetrievalQueryRequest {
    pub query: String,
    pub tenant_id: String,
    /// Result cap; 0 uses the default (5).
    pub limit: usize,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RetrievalHit {
    pub asset_id: String,
    pub layer: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub title: String,
    pub content: String,
    /// 1-based fused rank (RRF order).
    pub rank: usize,
    pub source_links: Vec<dope_memory::SourceLink>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub member_asset_ids: Vec<String>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RetrievalQueryResponse {
    pub hits: Vec<RetrievalHit>,
}

const DEFAULT_LIMIT: usize = 5;

/// POST /v1/retrieval/queries — fused recall over Ready L1 atoms
/// (private/team visibility, tenant-scoped; empty tenant = local scope).
#[allow(clippy::unused_async)]
pub async fn query(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<Json<RetrievalQueryResponse>, ApiError> {
    let request: RetrievalQueryRequest = super::decode_json_required(&body)?;
    if request.query.trim().is_empty() {
        return Err(ApiError::BadRequest("query is required".to_string()));
    }
    let Some(memory) = state.memory.as_deref() else {
        return Err(ApiError::internal("memory manager is not configured"));
    };
    let mut atoms = memory.list(
        &request.tenant_id,
        Some(dope_memory::MemoryLayer::L1),
        Some(dope_memory::AssetStatus::Ready),
    );
    atoms.retain(|asset| {
        matches!(
            asset.visibility,
            dope_memory::Visibility::Private | dope_memory::Visibility::Team
        )
    });
    // Newest first: the corpus index doubles as the recency rank.
    atoms.sort_by(|a, b| b.updated_at.cmp(&a.updated_at));
    let docs: Vec<dope_context::RetrievalDoc> = atoms
        .iter()
        .map(|asset| dope_context::RetrievalDoc {
            asset_id: asset.asset_id.clone(),
            title: asset.title.clone(),
            content: asset.content.clone(),
        })
        .collect();
    let embedder = dope_context::HashedNgramEmbedder::default();
    let limit = if request.limit == 0 { DEFAULT_LIMIT } else { request.limit };
    let hits = dope_context::retrieve_fused(&request.query, &docs, Some(&embedder))
        .into_iter()
        .take(limit)
        .enumerate()
        .map(|(position, idx)| {
            let asset = &atoms[idx];
            RetrievalHit {
                asset_id: asset.asset_id.clone(),
                layer: asset.layer.as_str().to_string(),
                title: asset.title.clone(),
                content: asset.content.clone(),
                rank: position + 1,
                source_links: asset.source_links.clone(),
                member_asset_ids: asset.member_asset_ids.clone(),
            }
        })
        .collect();
    Ok(Json(RetrievalQueryResponse { hits }))
}

pub fn router() -> Router<AppState> {
    Router::new().route("/v1/retrieval/queries", post(query))
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use axum::http::StatusCode;

    use super::super::tests_support::{request_json, test_state};

    fn seed_atom(manager: &dope_memory::Manager, content: &str) {
        manager
            .create(dope_memory::CreateAssetInput {
                kind: dope_memory::AssetKind::ChatMemory,
                layer: dope_memory::MemoryLayer::L1,
                owner: dope_memory::Actor {
                    kind: dope_memory::ActorKind::Operator,
                    id: "op".to_string(),
                },
                visibility: dope_memory::Visibility::Private,
                atom_type: Some(dope_memory::AtomType::Fact),
                content: content.to_string(),
                source_links: vec![dope_memory::SourceLink {
                    kind: dope_memory::SourceKind::Thread,
                    id: "thr_1".to_string(),
                    ..dope_memory::SourceLink::default()
                }],
                ..dope_memory::CreateAssetInput::default()
            })
            .expect("seed atom");
    }

    #[tokio::test]
    async fn query_returns_cited_hits_and_validates_input() {
        let mut state = test_state();
        let manager = Arc::new(dope_memory::Manager::new("test", None, None, None));
        seed_atom(&manager, "pnpm is the package manager for web projects");
        seed_atom(&manager, "lunch happens at noon");
        state.memory = Some(manager);

        let (status, json) = request_json(
            state.clone(),
            "POST",
            "/v1/retrieval/queries",
            Some(serde_json::json!({ "query": "which package manager" })),
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        let hits = json["hits"].as_array().expect("hits");
        assert!(!hits.is_empty());
        assert!(hits[0]["content"].as_str().unwrap().contains("pnpm"));
        assert_eq!(hits[0]["rank"], 1);
        assert_eq!(hits[0]["sourceLinks"][0]["id"], "thr_1", "citation intact");
        assert!(
            hits.iter().all(|h| !h["content"].as_str().unwrap().contains("lunch")),
            "unrelated atom not recalled"
        );

        let (status, _) = request_json(
            state,
            "POST",
            "/v1/retrieval/queries",
            Some(serde_json::json!({ "query": "  " })),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST);
    }
}
