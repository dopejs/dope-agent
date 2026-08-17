//! Memory-asset persistence (Roadmap 78, spec 058).
//!
//! The `memory_assets` table is tenant-partitioned; the full asset document
//! is the JSON column and the indexed columns are query projections. Rows
//! are written by the memory manager and restored at boot.

use rusqlite::{params, Row};

use crate::crud::{decode_json_field, now_rfc3339, null_string};
use crate::SQLiteStore;

/// Query filter for listing memory assets.
#[derive(Debug, Clone, Default)]
pub struct MemoryAssetFilter {
    pub tenant_id: String,
    pub layer: String,
    pub status: String,
    pub kind: String,
}

fn scan_asset(row: &Row) -> Result<dope_memory::MemoryAsset, String> {
    let document: String = row.get(0).map_err(|e| e.to_string())?;
    decode_json_field(&document)
}

impl SQLiteStore {
    pub fn upsert_memory_asset(&self, asset: &dope_memory::MemoryAsset) -> Result<(), String> {
        let document = serde_json::to_string(asset)
            .map_err(|e| format!("marshal memory asset {}: {e}", asset.asset_id))?;
        self.conn
            .execute(
                r#"INSERT INTO memory_assets (
                    asset_id, tenant_id, kind, layer, status, visibility, atom_type,
                    owner_kind, owner_id, version, supersedes_asset_id,
                    created_at, updated_at, document_json
                ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14)
                ON CONFLICT(asset_id) DO UPDATE SET
                    tenant_id = excluded.tenant_id,
                    kind = excluded.kind,
                    layer = excluded.layer,
                    status = excluded.status,
                    visibility = excluded.visibility,
                    atom_type = excluded.atom_type,
                    owner_kind = excluded.owner_kind,
                    owner_id = excluded.owner_id,
                    version = excluded.version,
                    supersedes_asset_id = excluded.supersedes_asset_id,
                    updated_at = excluded.updated_at,
                    document_json = excluded.document_json"#,
                params![
                    asset.asset_id,
                    null_string(&asset.tenant_id),
                    asset.kind.as_str(),
                    asset.layer.as_str(),
                    asset.status.as_str(),
                    asset.visibility.as_str(),
                    null_string(asset.atom_type.map(|a| a.as_str()).unwrap_or_default()),
                    asset.owner.kind.as_str(),
                    asset.owner.id,
                    asset.version,
                    null_string(&asset.supersedes_asset_id),
                    now_rfc3339(&asset.created_at),
                    now_rfc3339(&asset.updated_at),
                    document,
                ],
            )
            .map_err(|e| format!("upsert memory asset {}: {e}", asset.asset_id))?;
        Ok(())
    }

    pub fn get_memory_asset(
        &self,
        asset_id: &str,
    ) -> Result<Option<dope_memory::MemoryAsset>, String> {
        let mut stmt = self
            .conn
            .prepare("SELECT document_json FROM memory_assets WHERE asset_id = ?1")
            .map_err(|e| format!("get memory asset {asset_id}: {e}"))?;
        let mut rows = stmt.query(params![asset_id]).map_err(|e| e.to_string())?;
        let Some(row) = rows.next().map_err(|e| e.to_string())? else {
            return Ok(None);
        };
        scan_asset(row).map(Some)
    }

    pub fn list_memory_assets(
        &self,
        filter: &MemoryAssetFilter,
    ) -> Result<Vec<dope_memory::MemoryAsset>, String> {
        let mut sql = String::from(
            "SELECT document_json FROM memory_assets WHERE 1=1",
        );
        let mut args: Vec<String> = Vec::new();
        if !filter.tenant_id.trim().is_empty() {
            sql.push_str(&format!(" AND tenant_id = ?{}", args.len() + 1));
            args.push(filter.tenant_id.trim().to_string());
        }
        if !filter.layer.trim().is_empty() {
            sql.push_str(&format!(" AND layer = ?{}", args.len() + 1));
            args.push(filter.layer.trim().to_string());
        }
        if !filter.status.trim().is_empty() {
            sql.push_str(&format!(" AND status = ?{}", args.len() + 1));
            args.push(filter.status.trim().to_string());
        }
        if !filter.kind.trim().is_empty() {
            sql.push_str(&format!(" AND kind = ?{}", args.len() + 1));
            args.push(filter.kind.trim().to_string());
        }
        sql.push_str(" ORDER BY updated_at DESC, asset_id DESC");
        let mut stmt = self.conn.prepare(&sql).map_err(|e| format!("list memory assets: {e}"))?;
        let mut rows = stmt
            .query(rusqlite::params_from_iter(args.iter()))
            .map_err(|e| e.to_string())?;
        let mut items = Vec::new();
        while let Some(row) = rows.next().map_err(|e| e.to_string())? {
            items.push(scan_asset(row)?);
        }
        Ok(items)
    }

    /// Boot restore: every asset row (all tenants), oldest-first so
    /// supersede chains replay in order.
    pub fn list_all_memory_assets(&self) -> Result<Vec<dope_memory::MemoryAsset>, String> {
        let mut stmt = self
            .conn
            .prepare("SELECT document_json FROM memory_assets ORDER BY created_at ASC, asset_id ASC")
            .map_err(|e| format!("list all memory assets: {e}"))?;
        let mut rows = stmt.query([]).map_err(|e| e.to_string())?;
        let mut items = Vec::new();
        while let Some(row) = rows.next().map_err(|e| e.to_string())? {
            items.push(scan_asset(row)?);
        }
        Ok(items)
    }
}
