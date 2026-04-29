package store

import (
	"context"
	"testing"
)

func TestLiveValidationSchemaTablesAndIndexes(t *testing.T) {
	t.Parallel()

	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	tables := map[string][]string{
		"live_validation_attempts":                   {"validation_id", "tenant_id", "candidate_id", "environment_scope", "status", "document_json"},
		"live_validation_scopes":                     {"scope_id", "validation_id", "tenant_id", "approval_mode", "document_json"},
		"live_validation_approvals":                  {"approval_id", "validation_id", "tenant_id", "approval_target", "tool_class", "status", "document_json"},
		"live_validation_support_matrix_snapshots":   {"snapshot_id", "tenant_id", "version", "created_at", "document_json"},
		"live_validation_ledger_entries":             {"ledger_entry_id", "validation_id", "tenant_id", "tool_class", "safety_class", "outcome", "document_json"},
		"live_validation_kill_switches":              {"kill_switch_id", "scope", "tenant_id", "enabled", "document_json"},
		"live_validation_ambiguous_commits":          {"ambiguous_commit_id", "ledger_entry_id", "validation_id", "tenant_id", "cause", "document_json"},
		"live_validation_reconciliation_resolutions": {"reconciliation_id", "ambiguous_commit_id", "tenant_id", "resolved_by", "resolution", "document_json"},
		"live_validation_comparisons":                {"comparison_id", "validation_id", "tenant_id", "candidate_id", "terminal_status", "document_json"},
		"live_validation_retention_policies":         {"policy_id", "tenant_id", "applies_to", "retention_mode", "document_json"},
	}
	for table, columns := range tables {
		got := loadStoreColumns(t, s, ctx, table)
		for _, column := range columns {
			if !got[column] {
				t.Fatalf("table %s missing column %s", table, column)
			}
		}
	}

	indexes := []string{
		"idx_live_validation_attempts_tenant_status",
		"idx_live_validation_attempts_tenant_candidate",
		"idx_live_validation_ledger_validation",
		"idx_live_validation_ledger_outcome",
		"idx_live_validation_kill_switches_enabled",
		"idx_live_validation_comparisons_validation",
		"idx_live_validation_retention_tenant",
	}
	for _, indexName := range indexes {
		if !storeIndexExists(t, s, ctx, indexName) {
			t.Fatalf("missing live validation index %s", indexName)
		}
	}
}

func loadStoreColumns(t *testing.T, s *SQLiteStore, ctx context.Context, table string) map[string]bool {
	t.Helper()
	rows, err := s.DB().QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column for %s: %v", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("columns rows for %s: %v", table, err)
	}
	return columns
}

func storeIndexExists(t *testing.T, s *SQLiteStore, ctx context.Context, indexName string) bool {
	t.Helper()
	var count int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, indexName).Scan(&count); err != nil {
		t.Fatalf("query index %s: %v", indexName, err)
	}
	return count == 1
}
