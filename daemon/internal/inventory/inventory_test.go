package inventory

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseGoodFixture(t *testing.T) {
	src := strings.NewReader(`# header
some prose
| name | classification | tenantIdSource | migrationAction | affectedAPIs | affectedEvents | storeAccess | indexesAndUniqueness | isolationTests | rollback |
|------|----------------|----------------|-----------------|--------------|----------------|-------------|----------------------|----------------|----------|
| runs | tenant_owned | backfilled default personal tenant | add_column_backfill, index_rebuild | run routes | run-* | tenancy.runs | (tenant_id, created_at DESC) | runs domain | backup_restore |
| schema_migrations | global | not_applicable | leave_global | none | none | unchanged | none | none | backup_restore |
`)
	entries, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "runs" || entries[0].Classification != ClassificationTenantOwned {
		t.Fatalf("entry[0] mismatch: %+v", entries[0])
	}
	if entries[1].Classification != ClassificationGlobal {
		t.Fatalf("entry[1] classification: %q", entries[1].Classification)
	}
}

func TestParseRejectsEmptyCell(t *testing.T) {
	src := strings.NewReader(`| name | classification | tenantIdSource | migrationAction | affectedAPIs | affectedEvents | storeAccess | indexesAndUniqueness | isolationTests | rollback |
|---|---|---|---|---|---|---|---|---|---|
| runs | tenant_owned |  | add_column_backfill | r | e | s | i | t | backup_restore |
`)
	if _, err := Load(src); err == nil {
		t.Fatalf("expected error for empty cell")
	}
}

func TestParseRejectsUnknownClassification(t *testing.T) {
	src := strings.NewReader(`| name | classification | tenantIdSource | migrationAction | affectedAPIs | affectedEvents | storeAccess | indexesAndUniqueness | isolationTests | rollback |
|---|---|---|---|---|---|---|---|---|---|
| runs | unowned | x | add_column_backfill | r | e | s | i | t | backup_restore |
`)
	if _, err := Load(src); err == nil {
		t.Fatalf("expected error for unknown classification")
	}
}

func TestParseRejectsUnknownMigrationAction(t *testing.T) {
	src := strings.NewReader(`| name | classification | tenantIdSource | migrationAction | affectedAPIs | affectedEvents | storeAccess | indexesAndUniqueness | isolationTests | rollback |
|---|---|---|---|---|---|---|---|---|---|
| runs | tenant_owned | x | recreate_world | r | e | s | i | t | backup_restore |
`)
	if _, err := Load(src); err == nil {
		t.Fatalf("expected error for unknown migration action")
	}
}

func TestRealInventoryParses(t *testing.T) {
	// Locate the checked-in inventory relative to the repo root via this
	// file's path. Avoids hard-coding worktree-specific absolute paths.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(repoRoot, "specs", "020-tenant-scoped-data-migration", "contracts", "schema-inventory.md")

	entries, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile %s: %v", path, err)
	}
	if len(entries) < 30 {
		t.Fatalf("expected at least 30 inventory entries, got %d", len(entries))
	}

	// Spot check that real table names are present, not placeholders.
	byName := ByName(entries)
	for _, expected := range []string{"runs", "events", "schedule_dispatch_attempts", "evaluation_regression_fixtures", "computer_use_actions"} {
		if _, ok := byName[expected]; !ok {
			t.Fatalf("expected inventory entry for %q to be present", expected)
		}
	}
}
