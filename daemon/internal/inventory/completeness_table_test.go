package inventory_test

import (
	"context"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/inventory"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Roadmap 35 (US3 / T082) — schema-vs-inventory drift guard.
//
// Spins up a fresh SQLite store at the head schema version and asserts that
// the set of persisted user tables matches the set of `name` cells in the
// inventory's table sections. Drift in either direction fails: a new table
// added without an inventory entry, or an inventory entry that points to a
// non-existent table.
func TestInventoryTableCompleteness(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	rows, err := s.DB().QueryContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()

	live := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		// Skip internal SQLite virtual / shadow names that may show up
		// during testing (e.g., FTS shadow tables). None are present in
		// the head schema today, but the filter keeps the test stable
		// if FTS is added later for global, non-tenant search indexes.
		if strings.HasPrefix(name, "sqlite_") {
			continue
		}
		live[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	invPath := filepath.Join(repoRoot, "specs", "020-tenant-scoped-data-migration", "contracts", "schema-inventory.md")
	entries, err := inventory.LoadFromFile(invPath)
	if err != nil {
		t.Fatalf("LoadFromFile %s: %v", invPath, err)
	}

	inv := map[string]inventory.Entry{}
	for _, e := range entries {
		// Event-source rows live in the same table; identify them by the
		// presence of a glob (`*`), e.g., `run-*`, `mcp-*`. Those are
		// covered by T083, not by the table-completeness check.
		if strings.ContainsRune(e.Name, '*') {
			continue
		}
		// Skip header/composite labels that explicitly mark themselves.
		if strings.Contains(e.Name, "(") {
			continue
		}
		inv[e.Name] = e
	}

	var missingFromInventory, missingFromSchema []string
	for name := range live {
		if _, ok := inv[name]; !ok {
			missingFromInventory = append(missingFromInventory, name)
		}
	}
	for name := range inv {
		if _, ok := live[name]; !ok {
			missingFromSchema = append(missingFromSchema, name)
		}
	}
	sort.Strings(missingFromInventory)
	sort.Strings(missingFromSchema)

	if len(missingFromInventory) > 0 {
		t.Errorf("schema has %d table(s) absent from inventory: %v", len(missingFromInventory), missingFromInventory)
	}
	if len(missingFromSchema) > 0 {
		t.Errorf("inventory has %d table name(s) absent from schema: %v", len(missingFromSchema), missingFromSchema)
	}
}
