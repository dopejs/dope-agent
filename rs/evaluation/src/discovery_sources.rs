//! Port of `daemon/internal/evaluation/discovery_sources.go`: reading
//! tenant-scoped discovery source records into source refs.

use crate::error::EvaluationError;
use crate::product_validation::{normalize_product_limit, validate_tenant_scoped_product_request};
use crate::types::{SourceKind, SourceRef};
use chrono::{DateTime, Utc};

/// Go `DiscoverySourceRecord`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct DiscoverySourceRecord {
    pub tenant_id: String,
    pub kind: SourceKind,
    pub id: String,
    pub observed_at: DateTime<Utc>,
    pub payload: serde_json::Map<String, serde_json::Value>,
}

/// Go `DiscoverySourceFilter`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct DiscoverySourceFilter {
    pub tenant_id: String,
    pub source_kinds: Vec<SourceKind>,
    pub window_start: DateTime<Utc>,
    pub window_end: DateTime<Utc>,
    pub limit: i64,
    pub cursor: String,
}

/// Go `DiscoverySourceReader` interface.
pub trait DiscoverySourceReader: Send + Sync {
    fn list_discovery_sources(
        &self,
        filter: &DiscoverySourceFilter,
    ) -> Result<(Vec<DiscoverySourceRecord>, String), EvaluationError>;
}

/// Go `ReadDiscoverySourceRefs`.
pub fn read_discovery_source_refs(
    reader: &dyn DiscoverySourceReader,
    filter: &DiscoverySourceFilter,
) -> Result<(Vec<SourceRef>, String), EvaluationError> {
    validate_tenant_scoped_product_request(&filter.tenant_id)?;
    let mut normalized = filter.clone();
    normalized.limit = normalize_product_limit(normalized.limit);
    let (records, next_cursor) = reader.list_discovery_sources(&normalized)?;
    let refs = collect_discovery_source_refs(&filter.tenant_id, &records)?;
    Ok((refs, next_cursor))
}

/// Go `CollectDiscoverySourceRefs`.
pub fn collect_discovery_source_refs(
    tenant_id: &str,
    records: &[DiscoverySourceRecord],
) -> Result<Vec<SourceRef>, EvaluationError> {
    validate_tenant_scoped_product_request(tenant_id)?;
    let mut refs = Vec::with_capacity(records.len());
    for record in records {
        if record.tenant_id.trim() != tenant_id.trim() {
            return Err(EvaluationError::ProductCrossTenantSource);
        }
        if record.id.trim().is_empty() {
            continue;
        }
        refs.push(SourceRef {
            kind: record.kind,
            id: record.id.clone(),
            route: discovery_source_route(record.kind, &record.id),
        });
    }
    Ok(refs)
}

/// Go `discoverySourceRoute`.
#[must_use]
pub fn discovery_source_route(kind: SourceKind, id: &str) -> String {
    match kind {
        SourceKind::Run => format!("/v1/runs/{id}"),
        SourceKind::Workflow => format!("/v1/workflows/{id}"),
        SourceKind::Fixture => format!("/v1/evaluation/fixtures/{id}"),
        SourceKind::ToolCall => format!("/v1/tool-calls/{id}"),
        SourceKind::LiveValidationLedger => format!("/v1/live-validations/ledger/{id}"),
        _ => String::new(),
    }
}
