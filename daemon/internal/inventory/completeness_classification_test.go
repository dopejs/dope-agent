package inventory_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/inventory"
)

// Roadmap 35 (US3 / T084) — classification invariants.
//
// Asserts inventory rows obey the classification contract documented in the
// inventory header:
//
//   - rollback MUST be `backup_restore` for every row in this delivery.
//   - A `tenant_owned` row MUST NOT carry tenantIdSource `not_applicable`.
//   - A `tenant_owned` row MUST populate indexesAndUniqueness, isolationTests,
//     and storeAccess with substantive content (not the literal "none" or
//     "(none)") so reviewers can see the per-row commitment.
//
// These checks fire when a future PR weakens classification (for example by
// flipping a tenant_owned row to a global helper or omitting a per-tenant
// index commitment). The error messages name the offending row.
func TestInventoryClassificationInvariants(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	invPath := filepath.Join(repoRoot, "specs", "020-tenant-scoped-data-migration", "contracts", "schema-inventory.md")

	entries, err := inventory.LoadFromFile(invPath)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("inventory parsed empty (regression)")
	}

	for _, e := range entries {
		if !equalsIgnoreCase(e.Rollback, "backup_restore") {
			t.Errorf("%s: rollback must be backup_restore, got %q", e.Name, e.Rollback)
		}
		if e.Classification != inventory.ClassificationTenantOwned {
			continue
		}
		if equalsIgnoreCase(e.TenantIDSource, "not_applicable") {
			t.Errorf("%s: tenant_owned row must not carry tenantIdSource=not_applicable", e.Name)
		}
		for col, val := range map[string]string{
			"indexesAndUniqueness": e.IndexesAndUniqueness,
			"isolationTests":       e.IsolationTests,
			"storeAccess":          e.StoreAccess,
		} {
			if isPlaceholder(val) {
				t.Errorf("%s: tenant_owned row has placeholder %q in %s", e.Name, val, col)
			}
		}
	}
}

func equalsIgnoreCase(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), b)
}

// isPlaceholder reports whether a cell is semantically empty: literal "none"
// or "(none)" with optional surrounding whitespace.
func isPlaceholder(cell string) bool {
	c := strings.ToLower(strings.TrimSpace(cell))
	return c == "none" || c == "(none)"
}
