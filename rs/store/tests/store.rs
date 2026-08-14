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

    // All ported migrations are applied on open, up to the head of the ported list.
    let applied: i64 = store_conn_query(store.db_path(), "SELECT MAX(version) FROM schema_migrations");
    assert_eq!(applied, schema_migrations().last().unwrap().version);
}

#[test]
fn migrations_are_ordered_and_start_at_baseline() {
    let migrations = schema_migrations();
    assert!(migrations.len() >= 1);
    assert_eq!(migrations[0].version, 1);
    assert_eq!(migrations[0].name, "baseline");
    for pair in migrations.windows(2) {
        assert!(pair[0].version < pair[1].version);
    }
    assert_eq!(CURRENT_SCHEMA_VERSION, 55);
}

fn store_conn_query(db_path: &str, query: &str) -> i64 {
    let conn = rusqlite::Connection::open(db_path).unwrap();
    conn.query_row(query, [], |row| row.get(0)).unwrap()
}
