package store

import (
	"context"
	"testing"
)

func TestR37HostedCredentialSchema(t *testing.T) {
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	columns := r37ColumnsByTable(t, s)
	for _, table := range []string{"tenant_secrets", "tenant_secret_versions"} {
		if _, ok := columns[table]; !ok {
			t.Fatalf("expected %s table", table)
		}
	}
	for _, column := range []string{"secret_id", "tenant_id", "secret_ref", "status", "active_version_id", "document_json"} {
		if !r37HasColumn(columns["tenant_secrets"], column) {
			t.Errorf("tenant_secrets missing %s", column)
		}
	}
	for _, column := range []string{"secret_version_id", "secret_id", "tenant_id", "secret_ref", "version_number", "status", "value_backend_ref"} {
		if !r37HasColumn(columns["tenant_secret_versions"], column) {
			t.Errorf("tenant_secret_versions missing %s", column)
		}
	}
	for _, table := range []string{"provider_auth_states", "connectors", "mcp_servers", "mcp_server_states", "mcp_tools"} {
		if !r37HasColumn(columns[table], "tenant_id") {
			t.Errorf("%s missing tenant_id", table)
		}
	}
}

func r37ColumnsByTable(t *testing.T, s *SQLiteStore) map[string]map[string]struct{} {
	t.Helper()
	rows, err := s.DB().QueryContext(context.Background(),
		`SELECT m.name AS table_name, p.name AS column_name FROM sqlite_master m
		 JOIN pragma_table_info(m.name) p ON 1=1
		 WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatalf("introspect schema: %v", err)
	}
	defer rows.Close()
	out := map[string]map[string]struct{}{}
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			t.Fatalf("scan schema row: %v", err)
		}
		if _, ok := out[table]; !ok {
			out[table] = map[string]struct{}{}
		}
		out[table][column] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("schema rows: %v", err)
	}
	return out
}

func r37HasColumn(columns map[string]struct{}, column string) bool {
	_, ok := columns[column]
	return ok
}
