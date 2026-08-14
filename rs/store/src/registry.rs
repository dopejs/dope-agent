//! SQLite CRUD for the registry-style tables: sessions (router), capabilities (supervisor),
//! LLM dispatches, provider records, and policy approvals/decisions. Ported from
//! `daemon/internal/store/store.go` tenantless write paths.

use rusqlite::{params, Row};

use crate::crud::{
    enum_str, now_rfc3339, null_string, opt_time_string, parse_enum, parse_opt_rfc3339,
    parse_rfc3339,
};
use crate::SQLiteStore;

fn scan_session(row: &Row) -> Result<dope_router::Session, String> {
    let session_id: String = row.get(0).map_err(|e| e.to_string())?;
    let kind: String = row.get(1).map_err(|e| e.to_string())?;
    let status: String = row.get(2).map_err(|e| e.to_string())?;
    let channel: String = row.get(3).map_err(|e| e.to_string())?;
    let account_id: Option<String> = row.get(4).map_err(|e| e.to_string())?;
    let peer_id: String = row.get(5).map_err(|e| e.to_string())?;
    let thread_id: Option<String> = row.get(6).map_err(|e| e.to_string())?;
    let routing_key: String = row.get(7).map_err(|e| e.to_string())?;
    let generation: i64 = row.get(8).map_err(|e| e.to_string())?;
    let created_at: String = row.get(9).map_err(|e| e.to_string())?;
    let updated_at: String = row.get(10).map_err(|e| e.to_string())?;
    let last_active_at: String = row.get(11).map_err(|e| e.to_string())?;
    let last_reset_at: Option<String> = row.get(12).map_err(|e| e.to_string())?;

    Ok(dope_router::Session {
        session_id,
        kind: parse_enum(&kind)?,
        status: parse_enum(&status)?,
        channel,
        account_id: account_id.unwrap_or_default(),
        peer_id,
        thread_id: thread_id.unwrap_or_default(),
        routing_key,
        generation,
        created_at: parse_rfc3339(&created_at)?,
        updated_at: parse_rfc3339(&updated_at)?,
        last_active_at: parse_rfc3339(&last_active_at)?,
        last_reset_at: parse_opt_rfc3339(last_reset_at)?,
        active_profile_projection: None,
    })
}

fn scan_capability(row: &Row) -> Result<dope_capabilities::Capability, String> {
    let capability_id: String = row.get(0).map_err(|e| e.to_string())?;
    let kind: String = row.get(1).map_err(|e| e.to_string())?;
    let display_name: String = row.get(2).map_err(|e| e.to_string())?;
    let status: String = row.get(3).map_err(|e| e.to_string())?;
    let failure_count: i64 = row.get(4).map_err(|e| e.to_string())?;
    let restart_count: i64 = row.get(5).map_err(|e| e.to_string())?;
    let backoff_seconds: i64 = row.get(6).map_err(|e| e.to_string())?;
    let next_restart_at: Option<String> = row.get(7).map_err(|e| e.to_string())?;
    let last_restart_at: Option<String> = row.get(8).map_err(|e| e.to_string())?;
    let last_heartbeat_at: Option<String> = row.get(9).map_err(|e| e.to_string())?;
    let last_failure_reason: Option<String> = row.get(10).map_err(|e| e.to_string())?;
    let created_at: String = row.get(11).map_err(|e| e.to_string())?;
    let updated_at: String = row.get(12).map_err(|e| e.to_string())?;

    Ok(dope_capabilities::Capability {
        capability_id,
        kind,
        display_name,
        status: parse_enum(&status)?,
        failure_count,
        restart_count,
        backoff_seconds,
        next_restart_at: parse_opt_rfc3339(next_restart_at)?,
        last_restart_at: parse_opt_rfc3339(last_restart_at)?,
        last_heartbeat_at: parse_opt_rfc3339(last_heartbeat_at)?,
        last_failure_reason: last_failure_reason.unwrap_or_default(),
        created_at: parse_rfc3339(&created_at)?,
        updated_at: parse_rfc3339(&updated_at)?,
    })
}

impl SQLiteStore {
    pub fn upsert_session(&self, session: &dope_router::Session) -> Result<(), String> {
        self.conn
            .execute(
                r#"INSERT INTO sessions (
                    session_id, kind, status, channel, account_id, peer_id, thread_id, routing_key,
                    generation, created_at, updated_at, last_active_at, last_reset_at
                ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)
                ON CONFLICT(session_id) DO UPDATE SET
                    kind = excluded.kind,
                    status = excluded.status,
                    channel = excluded.channel,
                    account_id = excluded.account_id,
                    peer_id = excluded.peer_id,
                    thread_id = excluded.thread_id,
                    routing_key = excluded.routing_key,
                    generation = excluded.generation,
                    created_at = excluded.created_at,
                    updated_at = excluded.updated_at,
                    last_active_at = excluded.last_active_at,
                    last_reset_at = excluded.last_reset_at"#,
                params![
                    session.session_id,
                    enum_str(&session.kind),
                    enum_str(&session.status),
                    session.channel,
                    null_string(&session.account_id),
                    session.peer_id,
                    null_string(&session.thread_id),
                    session.routing_key,
                    session.generation,
                    now_rfc3339(&session.created_at),
                    now_rfc3339(&session.updated_at),
                    now_rfc3339(&session.last_active_at),
                    opt_time_string(&session.last_reset_at),
                ],
            )
            .map_err(|e| format!("upsert session {}: {e}", session.session_id))?;
        Ok(())
    }

    pub fn list_sessions(&self) -> Result<Vec<dope_router::Session>, String> {
        let mut stmt = self
            .conn
            .prepare(
                r#"SELECT session_id, kind, status, channel, account_id, peer_id, thread_id, routing_key,
                    generation, created_at, updated_at, last_active_at, last_reset_at
                FROM sessions
                ORDER BY created_at ASC, session_id ASC"#,
            )
            .map_err(|e| format!("list sessions: {e}"))?;
        let mut rows = stmt.query([]).map_err(|e| e.to_string())?;
        let mut items = Vec::new();
        while let Some(row) = rows.next().map_err(|e| e.to_string())? {
            items.push(scan_session(row)?);
        }
        Ok(items)
    }

    pub fn upsert_capability(&self, capability: &dope_capabilities::Capability) -> Result<(), String> {
        self.conn
            .execute(
                r#"INSERT INTO capabilities (
                    capability_id, kind, display_name, status, failure_count, restart_count,
                    backoff_seconds, next_restart_at, last_restart_at, last_heartbeat_at,
                    last_failure_reason, created_at, updated_at
                ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13)
                ON CONFLICT(capability_id) DO UPDATE SET
                    kind = excluded.kind,
                    display_name = excluded.display_name,
                    status = excluded.status,
                    failure_count = excluded.failure_count,
                    restart_count = excluded.restart_count,
                    backoff_seconds = excluded.backoff_seconds,
                    next_restart_at = excluded.next_restart_at,
                    last_restart_at = excluded.last_restart_at,
                    last_heartbeat_at = excluded.last_heartbeat_at,
                    last_failure_reason = excluded.last_failure_reason,
                    created_at = excluded.created_at,
                    updated_at = excluded.updated_at"#,
                params![
                    capability.capability_id,
                    capability.kind,
                    capability.display_name,
                    enum_str(&capability.status),
                    capability.failure_count,
                    capability.restart_count,
                    capability.backoff_seconds,
                    opt_time_string(&capability.next_restart_at),
                    opt_time_string(&capability.last_restart_at),
                    opt_time_string(&capability.last_heartbeat_at),
                    null_string(&capability.last_failure_reason),
                    now_rfc3339(&capability.created_at),
                    now_rfc3339(&capability.updated_at),
                ],
            )
            .map_err(|e| format!("upsert capability {}: {e}", capability.capability_id))?;
        Ok(())
    }

    pub fn list_capabilities(&self) -> Result<Vec<dope_capabilities::Capability>, String> {
        let mut stmt = self
            .conn
            .prepare(
                r#"SELECT capability_id, kind, display_name, status, failure_count, restart_count,
                    backoff_seconds, next_restart_at, last_restart_at, last_heartbeat_at,
                    last_failure_reason, created_at, updated_at
                FROM capabilities
                ORDER BY created_at ASC, capability_id ASC"#,
            )
            .map_err(|e| format!("list capabilities: {e}"))?;
        let mut rows = stmt.query([]).map_err(|e| e.to_string())?;
        let mut items = Vec::new();
        while let Some(row) = rows.next().map_err(|e| e.to_string())? {
            items.push(scan_capability(row)?);
        }
        Ok(items)
    }
}
