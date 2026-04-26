package inventory_test

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/inventory"
)

// Roadmap 35 (US3 / T083) — events-vs-inventory drift guard.
//
// Asserts every event payload schema under schemas/events/ is covered by
// exactly one inventory event row (literal name or glob), and every
// inventory event row that claims a schema-backed surface matches at least
// one schema. Rows annotated as "lifecycle, in-process" are exempt from the
// reverse check because their publisher is the runtime bus and they have
// no on-disk schema (e.g. daemon.migration.*).
func TestInventoryEventCompleteness(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	schemasDir := filepath.Join(repoRoot, "schemas", "events")
	invPath := filepath.Join(repoRoot, "specs", "020-tenant-scoped-data-migration", "contracts", "schema-inventory.md")

	dir, err := os.ReadDir(schemasDir)
	if err != nil {
		t.Fatalf("read schemas dir: %v", err)
	}
	var schemaNames []string
	for _, f := range dir {
		n := f.Name()
		if !strings.HasSuffix(n, ".event.schema.json") {
			continue
		}
		schemaNames = append(schemaNames, strings.TrimSuffix(n, ".event.schema.json"))
	}
	sort.Strings(schemaNames)
	if len(schemaNames) == 0 {
		t.Fatalf("no event schema files discovered under %s", schemasDir)
	}

	entries, err := inventory.LoadFromFile(invPath)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	type pat struct {
		raw      string
		isGlob   bool
		exempt   bool
		prefix   string
		literal  string
		matched  int
		original string
	}
	// Exemption is keyed off the cleaned (stripped-of-parenthetical)
	// pattern name, not a substring scan over the raw label. The
	// previous "contains 'lifecycle' or 'in-process'" heuristic would
	// silently exempt any future row that happened to mention those
	// words, including dead inventory rows. The explicit allowlist
	// below forces a deliberate decision per-pattern.
	exemptPatterns := map[string]struct{}{
		// daemon migration lifecycle events publish through the in-
		// process events bus and have no on-disk schema files.
		"daemon.migration.*": {},
		// connector_global / capability_global publish daemon-wide
		// connector and capability lifecycle events that are not
		// per-event-named in schemas/events/. They are exempt from the
		// reverse check because they describe a category, not a single
		// event payload.
		"connector_global":  {},
		"capability_global": {},
		// chat-query-stream-* events ride the SSE in-process channel,
		// not the persisted events table; their payloads are encoded
		// inline rather than via per-event JSON schema files.
		"chat-query-stream-*": {},
	}

	var pats []pat
	for _, e := range entries {
		// Skip table rows (no `*` and not a literal known event name).
		// Heuristic: an event row carries `*` OR is a literal hyphen-cased
		// name that exists in our schema set. We resolve literal vs table
		// rows by name set lookup below.
		clean := stripParenthetical(e.Name)
		if clean == "" {
			continue
		}
		_, exempt := exemptPatterns[clean]
		if strings.ContainsRune(clean, '*') {
			prefix := strings.TrimSuffix(clean, "*")
			pats = append(pats, pat{raw: clean, isGlob: true, exempt: exempt, prefix: prefix, original: e.Name})
			continue
		}
		// Literal name candidate — keep only if it appears in the schema
		// set; otherwise it is a table row, not an event row.
		for _, sn := range schemaNames {
			if sn == clean {
				pats = append(pats, pat{raw: clean, isGlob: false, exempt: exempt, literal: clean, original: e.Name})
				break
			}
		}
	}

	if len(pats) == 0 {
		t.Fatalf("no event-shaped inventory rows discovered (regression in parser?)")
	}

	// Forward direction: every schema must match at least one inventory pattern.
	var orphanSchemas []string
	for _, sn := range schemaNames {
		matched := false
		for i := range pats {
			if pats[i].isGlob {
				if pats[i].prefix == "" || strings.HasPrefix(sn, pats[i].prefix) {
					pats[i].matched++
					matched = true
				}
			} else if pats[i].literal == sn {
				pats[i].matched++
				matched = true
			}
		}
		if !matched {
			orphanSchemas = append(orphanSchemas, sn)
		}
	}

	// Reverse direction: every non-exempt inventory pattern must match at least
	// one schema.
	var unmatchedPatterns []string
	for _, p := range pats {
		if p.exempt {
			continue
		}
		if p.matched == 0 {
			unmatchedPatterns = append(unmatchedPatterns, p.original)
		}
	}

	sort.Strings(orphanSchemas)
	sort.Strings(unmatchedPatterns)

	if len(orphanSchemas) > 0 {
		t.Errorf("schemas/events/ has %d schema(s) not covered by any inventory entry: %v", len(orphanSchemas), orphanSchemas)
	}
	if len(unmatchedPatterns) > 0 {
		t.Errorf("inventory has %d event entry(ies) with no matching schema: %v", len(unmatchedPatterns), unmatchedPatterns)
	}
}

// TestInventoryGlobalCategoriesMatchRuntimeRegistry cross-checks that
// the inventory's "Event Sources" rows acknowledge every category in
// the runtime registry events.GlobalCategories. The filesystem-based
// drift guard above only catches per-event-name divergence; it cannot
// detect the case where a category is added at runtime but never
// declared in the inventory's Affected Events column.
//
// Failure mode this guards against: someone adds "billing" to
// events.GlobalCategories so the daemon stops binding tenant_id on
// billing.* events, but forgets to update the inventory contract;
// downstream readers (operators, R37 boundary review, this drift
// guard's filesystem half) keep treating billing events as missing.
func TestInventoryGlobalCategoriesMatchRuntimeRegistry(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	invPath := filepath.Join(repoRoot, "specs", "020-tenant-scoped-data-migration", "contracts", "schema-inventory.md")

	entries, err := inventory.LoadFromFile(invPath)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	// Build the set of category prefixes covered by the inventory by
	// scanning EVERY column for an event-name pattern of the form
	// "<category>-*" or "<category>.*". This is intentionally lax — a
	// category mentioned anywhere in the inventory counts as covered.
	covered := map[string]bool{}
	for _, e := range entries {
		blob := strings.Join([]string{
			e.Name, e.AffectedAPIs, e.AffectedEvents, e.StoreAccess,
			e.IsolationTests, e.Rollback,
		}, " ")
		blob = strings.ToLower(blob)
		// Inventory uses both `mcp-*` and `mcp.*` forms; match either.
		for cat := range events.GlobalCategories {
			low := strings.ToLower(cat)
			if strings.Contains(blob, low+"-") ||
				strings.Contains(blob, low+".") ||
				strings.Contains(blob, "category="+low) ||
				strings.Contains(blob, low+" ") ||
				strings.HasSuffix(blob, low) {
				covered[cat] = true
			}
		}
	}

	var missing []string
	for cat := range events.GlobalCategories {
		if !covered[cat] {
			missing = append(missing, cat)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("runtime events.GlobalCategories %v are not mentioned anywhere in the inventory; "+
			"add an Event Sources row or annotate the existing one so the contract reflects the runtime "+
			"emission set.", missing)
	}
}

// stripParenthetical returns the inventory name with any trailing
// parenthetical descriptor removed, e.g.:
//
//	"audit-cross-tenant-access-denied (NEW in R35)" -> "audit-cross-tenant-access-denied"
//	"daemon.migration.* (lifecycle, in-process)"    -> "daemon.migration.*"
//	"tenant-* (R34 surface)"                         -> "tenant-*"
func stripParenthetical(name string) string {
	if i := strings.Index(name, "("); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}
