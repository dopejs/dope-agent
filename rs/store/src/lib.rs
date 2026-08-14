//! Port of the daemon/internal/store core (the SQLite connection + schema migration
//! framework). The 55 schema versions and per-domain CRUD methods are ported incrementally;
//! this crate establishes the connection, pragma configuration, and migration runner.

use std::path::Path;

use chrono::SecondsFormat;
use rusqlite::{params, Connection};

mod crud;
mod migrations;

/// The production schema head. The full 55-version migration list is added incrementally.
pub const CURRENT_SCHEMA_VERSION: i64 = 55;

const DEFAULT_DATABASE_FILE: &str = "daemon.sqlite";

/// One schema migration: a monotonically increasing version plus the SQL statements applied in
/// order within a single transaction.
#[derive(Debug, Clone, Default)]
pub struct SchemaMigration {
    pub version: i64,
    pub name: String,
    pub statements: Vec<String>,
}

/// The ordered schema migration list (see migrations.rs).
#[must_use]
pub fn schema_migrations() -> Vec<SchemaMigration> {
    migrations::schema_migrations()
}

pub struct SQLiteStore {
    data_dir: String,
    db_path: String,
    conn: Connection,
}

impl SQLiteStore {
    pub fn new(data_dir: &str) -> Result<Self, String> {
        let resolved = resolve_data_dir(data_dir)?;
        std::fs::create_dir_all(&resolved).map_err(|e| format!("create data dir: {e}"))?;
        let db_path = Path::new(&resolved).join(DEFAULT_DATABASE_FILE);
        let db_path = db_path.to_string_lossy().to_string();
        let conn = Connection::open(&db_path).map_err(|e| format!("open sqlite db: {e}"))?;
        let store = SQLiteStore { data_dir: resolved, db_path, conn };
        store.configure()?;
        store.migrate()?;
        Ok(store)
    }

    #[must_use]
    pub fn data_dir(&self) -> &str {
        &self.data_dir
    }

    #[must_use]
    pub fn db_path(&self) -> &str {
        &self.db_path
    }

    /// Applies the SQLite pragmas used by the Go store.
    fn configure(&self) -> Result<(), String> {
        self.conn
            .execute_batch(
                "PRAGMA foreign_keys = ON;
                 PRAGMA journal_mode = WAL;
                 PRAGMA busy_timeout = 5000;",
            )
            .map_err(|e| format!("apply pragmas: {e}"))
    }

    /// Ensures the bookkeeping table and applies any migrations newer than the current version.
    fn migrate(&self) -> Result<(), String> {
        let tx = self.conn.unchecked_transaction().map_err(|e| format!("begin migration: {e}"))?;
        tx.execute_batch(
            "CREATE TABLE IF NOT EXISTS schema_migrations (
                version INTEGER PRIMARY KEY,
                name TEXT NOT NULL,
                applied_at TEXT NOT NULL
            );",
        )
        .map_err(|e| format!("ensure schema_migrations table: {e}"))?;

        let mut current = current_schema_version(&tx)?;
        if current > CURRENT_SCHEMA_VERSION {
            return Err(format!("database schema version {current} is newer than supported version {CURRENT_SCHEMA_VERSION}"));
        }

        for migration in schema_migrations() {
            if migration.version <= current {
                continue;
            }
            for statement in &migration.statements {
                tx.execute_batch(statement)
                    .map_err(|e| format!("apply schema migration {} ({}): {e}", migration.version, migration.name))?;
            }
            record_schema_migration(&tx, migration.version, &migration.name)?;
            current = migration.version;
        }

        tx.commit().map_err(|e| format!("commit migration transaction: {e}"))
    }
}

fn current_schema_version(conn: &Connection) -> Result<i64, String> {
    let version: Option<i64> = conn
        .query_row("SELECT MAX(version) FROM schema_migrations", [], |row| row.get(0))
        .map_err(|e| format!("load current schema version: {e}"))?;
    Ok(version.unwrap_or(0))
}

fn record_schema_migration(conn: &Connection, version: i64, name: &str) -> Result<(), String> {
    let applied_at = chrono::Utc::now().to_rfc3339_opts(SecondsFormat::Nanos, true);
    conn.execute(
        "INSERT INTO schema_migrations (version, name, applied_at)
         VALUES (?1, ?2, ?3)
         ON CONFLICT(version) DO UPDATE SET name = excluded.name, applied_at = excluded.applied_at",
        params![version, name, applied_at],
    )
    .map_err(|e| format!("record schema migration {version}: {e}"))?;
    Ok(())
}

fn resolve_data_dir(data_dir: &str) -> Result<String, String> {
    if data_dir.is_empty() {
        return Err("data dir is required".to_string());
    }
    if data_dir == "~" || data_dir.starts_with("~/") {
        let home = std::env::var("HOME").map_err(|_| "resolve user home: HOME is not set".to_string())?;
        if data_dir == "~" {
            return Ok(home);
        }
        return Ok(Path::new(&home).join(&data_dir[2..]).to_string_lossy().to_string());
    }
    Ok(data_dir.to_string())
}
