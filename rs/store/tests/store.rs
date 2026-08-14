use std::path::Path;

use dope_store::{schema_migrations, SQLiteStore, CURRENT_SCHEMA_VERSION};

fn temp_dir(name: &str) -> String {
    let dir = std::env::temp_dir().join(format!("dope_store_{name}_{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    dir.to_string_lossy().to_string()
}

#[test]
fn opens_store_and_creates_schema_migrations_table() {
    let dir = temp_dir("open");
    let store = SQLiteStore::new(&dir).unwrap();
    assert_eq!(store.data_dir(), dir);
    assert!(Path::new(store.db_path()).exists());

    // The bookkeeping table is created even with an empty migration list.
    let version: i64 = store_conn_query(store.db_path(), "SELECT COUNT(1) FROM schema_migrations");
    assert_eq!(version, 0);
}

#[test]
fn empty_migration_list_is_noop() {
    assert!(schema_migrations().is_empty());
    assert_eq!(CURRENT_SCHEMA_VERSION, 55);
}

fn store_conn_query(db_path: &str, query: &str) -> i64 {
    let conn = rusqlite::Connection::open(db_path).unwrap();
    conn.query_row(query, [], |row| row.get(0)).unwrap()
}
