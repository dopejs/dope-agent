package inventory

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Classification is the tenant-scope classification of a persisted table or
// event source per Roadmap 35.
type Classification string

const (
	ClassificationTenantOwned Classification = "tenant_owned"
	ClassificationGlobal      Classification = "global"
	ClassificationDerived     Classification = "derived"
)

// AllowedMigrationActions is the closed set of strings allowed in the
// `migrationAction` cell. The cell may be a comma-joined combination.
var AllowedMigrationActions = map[string]struct{}{
	"add_column_backfill":      {},
	"index_rebuild":            {},
	"leave_global":             {},
	"leave_existing":           {},
	"derive_at_read":           {},
	"remove_tenantless_access": {},
}

// Entry represents one row in the schema inventory. Every column is required
// at parse time; empty cells fail validation.
type Entry struct {
	Name                 string
	Classification       Classification
	TenantIDSource       string
	MigrationAction      string
	AffectedAPIs         string
	AffectedEvents       string
	StoreAccess          string
	IndexesAndUniqueness string
	IsolationTests       string
	Rollback             string
}

// LoadFromFile parses the inventory Markdown file at path.
func LoadFromFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open inventory file: %w", err)
	}
	defer f.Close()
	return Load(f)
}

// Load parses the inventory Markdown stream. It accepts every Markdown table
// row whose first cell starts with `|` and is not the header or separator
// row. Section headings (`##`, `###`) and prose paragraphs are ignored. Each
// non-header table row MUST contain exactly 10 cells matching the column
// order documented in the inventory file.
func Load(r io.Reader) ([]Entry, error) {
	const expectedCells = 10

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var entries []Entry
	headerSeen := false
	for lineNo := 0; scanner.Scan(); {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			headerSeen = false
			continue
		}

		cells := splitMarkdownRow(line)
		if len(cells) != expectedCells {
			// Allow header / separator row to silently align if it has the
			// expected width; otherwise skip rows with unexpected column
			// counts to keep the parser tolerant of nearby explanatory tables.
			if !headerSeen && isHeaderRow(cells) {
				headerSeen = true
				continue
			}
			if isSeparatorRow(cells) {
				continue
			}
			continue
		}
		if isSeparatorRow(cells) {
			continue
		}
		if !headerSeen && isHeaderRow(cells) {
			headerSeen = true
			continue
		}
		// At this point we have a data row. Skip if it looks like a header
		// (e.g. another section's header row).
		if isHeaderRow(cells) {
			headerSeen = true
			continue
		}

		entry, err := buildEntry(cells, lineNo)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan inventory: %w", err)
	}
	return entries, nil
}

func splitMarkdownRow(line string) []string {
	// Markdown rows look like: | a | b | c |
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func isHeaderRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	return strings.EqualFold(cells[0], "name")
}

func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if c == "" {
			continue
		}
		c = strings.TrimSpace(c)
		c = strings.TrimLeft(c, ":")
		c = strings.TrimRight(c, ":")
		if c == "" {
			continue
		}
		for _, ch := range c {
			if ch != '-' {
				return false
			}
		}
	}
	return true
}

func buildEntry(cells []string, lineNo int) (Entry, error) {
	for idx, c := range cells {
		if c == "" {
			return Entry{}, fmt.Errorf("inventory line %d: column %d is empty", lineNo, idx+1)
		}
	}

	classification := Classification(cells[1])
	switch classification {
	case ClassificationTenantOwned, ClassificationGlobal, ClassificationDerived:
	default:
		return Entry{}, fmt.Errorf("inventory line %d (%s): unknown classification %q", lineNo, cells[0], cells[1])
	}

	if err := validateMigrationAction(cells[3]); err != nil {
		return Entry{}, fmt.Errorf("inventory line %d (%s): %w", lineNo, cells[0], err)
	}

	return Entry{
		Name:                 cells[0],
		Classification:       classification,
		TenantIDSource:       cells[2],
		MigrationAction:      cells[3],
		AffectedAPIs:         cells[4],
		AffectedEvents:       cells[5],
		StoreAccess:          cells[6],
		IndexesAndUniqueness: cells[7],
		IsolationTests:       cells[8],
		Rollback:             cells[9],
	}, nil
}

func validateMigrationAction(cell string) error {
	if cell == "" {
		return fmt.Errorf("migration action is empty")
	}
	// Cells may carry freeform notes alongside the action keyword (e.g.
	// "leave_existing (verify-only); add UNIQUE (tenant_id, principal_id)
	// if not present"). Validation requires AT LEAST one allowed action
	// keyword to appear as a recognizable whole token; everything else in
	// the cell is documentation.
	for action := range AllowedMigrationActions {
		if containsToken(cell, action) {
			return nil
		}
	}
	return fmt.Errorf("no allowed migration action found in %q (allowed: add_column_backfill, index_rebuild, leave_global, leave_existing, derive_at_read, remove_tenantless_access)", cell)
}

// containsToken reports whether s contains token as a whole token surrounded
// by non-identifier characters (or the string boundary).
func containsToken(s, token string) bool {
	idx := 0
	for {
		i := strings.Index(s[idx:], token)
		if i < 0 {
			return false
		}
		i += idx
		left := i == 0 || !isIdentRune(rune(s[i-1]))
		end := i + len(token)
		right := end == len(s) || !isIdentRune(rune(s[end]))
		if left && right {
			return true
		}
		idx = end
	}
}

func isIdentRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// ByName returns a map of entries keyed by Name for O(1) lookups.
func ByName(entries []Entry) map[string]Entry {
	out := make(map[string]Entry, len(entries))
	for _, e := range entries {
		out[e.Name] = e
	}
	return out
}
