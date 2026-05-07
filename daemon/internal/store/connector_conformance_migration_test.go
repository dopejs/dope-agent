package store

import (
	"context"
	"testing"
)

func TestSQLiteStoreConnectorConformanceMigrationCreatesEvidenceTables(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	for _, table := range []string{
		"connector_conformance_results",
		"connector_diagnostic_states",
		"connector_diagnostic_redaction_failures",
		"connector_delivery_boundaries",
	} {
		if !sqliteTableExists(t, ctx, store, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	for _, column := range []string{
		"connector_account_id",
		"channel_or_conversation_id",
		"provider_message_id",
		"equivalent_rule_id",
		"foreground_outcome_status",
		"background_delivery_id",
		"delivery_boundary_kind",
	} {
		if !sqliteColumnExists(t, ctx, store, "connector_messages", column) {
			t.Fatalf("expected connector_messages.%s to exist", column)
		}
	}

	if !sqliteIndexExists(t, ctx, store, "idx_connector_messages_standard_identity_unique") {
		t.Fatalf("expected idx_connector_messages_standard_identity_unique to exist")
	}
}

func sqliteTableExists(t *testing.T, ctx context.Context, store *SQLiteStore, table string) bool {
	t.Helper()

	var name string
	err := store.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	return err == nil && name == table
}

func sqliteColumnExists(t *testing.T, ctx context.Context, store *SQLiteStore, table, column string) bool {
	t.Helper()

	rows, err := store.DB().QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s): %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info(%s): %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info(%s): %v", table, err)
	}
	return false
}

func sqliteIndexExists(t *testing.T, ctx context.Context, store *SQLiteStore, index string) bool {
	t.Helper()

	var name string
	err := store.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, index).Scan(&name)
	return err == nil && name == index
}
