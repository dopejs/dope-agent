//! Generic JSON-document store backing the Roadmap 65-71 in-memory managers (triage, routine,
//! webhook, catalog, execprofile, evidence). Ported from `daemon/internal/store/manager_documents.go`.

use chrono::{DateTime, Utc};
use rusqlite::{params, Row};

use crate::crud::{now_rfc3339, null_string, parse_rfc3339};
use crate::SQLiteStore;

/// One manager document row keyed by (doc_kind, doc_id).
#[derive(Debug, Clone, Default, PartialEq)]
pub struct ManagerDocument {
    pub doc_kind: String,
    pub doc_id: String,
    pub environment_scope: String,
    pub tenant_id: String,
    pub document_json: String,
    pub updated_at: DateTime<Utc>,
}

fn scan_manager_document(row: &Row) -> Result<ManagerDocument, String> {
    let doc_kind: String = row.get(0).map_err(|e| e.to_string())?;
    let doc_id: String = row.get(1).map_err(|e| e.to_string())?;
    let environment_scope: Option<String> = row.get(2).map_err(|e| e.to_string())?;
    let tenant_id: Option<String> = row.get(3).map_err(|e| e.to_string())?;
    let document_json: String = row.get(4).map_err(|e| e.to_string())?;
    let updated_at: String = row.get(5).map_err(|e| e.to_string())?;

    Ok(ManagerDocument {
        doc_kind,
        doc_id,
        environment_scope: environment_scope.unwrap_or_default(),
        tenant_id: tenant_id.unwrap_or_default(),
        document_json,
        updated_at: parse_rfc3339(&updated_at)?,
    })
}

impl SQLiteStore {
    pub fn put_manager_document(&self, doc: &ManagerDocument) -> Result<(), String> {
        self.conn
            .execute(
                r#"INSERT INTO manager_documents (doc_kind, doc_id, environment_scope, tenant_id, document_json, updated_at)
                VALUES (?1, ?2, ?3, ?4, ?5, ?6)
                ON CONFLICT(doc_kind, doc_id) DO UPDATE SET
                    environment_scope = excluded.environment_scope,
                    tenant_id = excluded.tenant_id,
                    document_json = excluded.document_json,
                    updated_at = excluded.updated_at"#,
                params![
                    doc.doc_kind,
                    doc.doc_id,
                    null_string(&doc.environment_scope),
                    null_string(&doc.tenant_id),
                    doc.document_json,
                    now_rfc3339(&doc.updated_at),
                ],
            )
            .map_err(|e| format!("put manager document {}/{}: {e}", doc.doc_kind, doc.doc_id))?;
        Ok(())
    }

    pub fn delete_manager_document(&self, doc_kind: &str, doc_id: &str) -> Result<(), String> {
        self.conn
            .execute(
                "DELETE FROM manager_documents WHERE doc_kind = ?1 AND doc_id = ?2",
                params![doc_kind, doc_id],
            )
            .map_err(|e| format!("delete manager document {doc_kind}/{doc_id}: {e}"))?;
        Ok(())
    }

    pub fn list_manager_documents(&self, doc_kind: &str) -> Result<Vec<ManagerDocument>, String> {
        let mut stmt = self
            .conn
            .prepare(
                r#"SELECT doc_kind, doc_id, environment_scope, tenant_id, document_json, updated_at
                FROM manager_documents
                WHERE doc_kind = ?1
                ORDER BY doc_id"#,
            )
            .map_err(|e| format!("list manager documents for {doc_kind}: {e}"))?;
        let mut rows = stmt.query(params![doc_kind]).map_err(|e| e.to_string())?;
        let mut items = Vec::new();
        while let Some(row) = rows.next().map_err(|e| e.to_string())? {
            items.push(scan_manager_document(row)?);
        }
        Ok(items)
    }
}
