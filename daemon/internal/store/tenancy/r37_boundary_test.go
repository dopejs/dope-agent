package tenancy_test

// Roadmap 37 hosted credential boundary signature golden.
//
// Verifies FR-015 by enforcing two disjoint groups distinguished by inventory
// classification:
//
//   GROUP A (this roadmap adds tenant ownership):
//     secret_scope_bindings, provider_preferences, mcp_tool_exposure_rules.
//     For each: (i) the only column added is `tenant_id` plus the indexes
//     declared in the inventory; (ii) the new tenancy.<X> helpers expose
//     ONLY ownership wiring — no credential / OAuth / secret-reference /
//     redaction / authorization function is added or changed; (iii) the
//     credential-resolution outputs are preserved (verified externally by
//     the migration suite — Group A schema check here is the structural
//     guard).
//
//   GROUP B (owned by this roadmap):
//     provider_auth_states, mcp_servers, mcp_server_states, mcp_tools,
//     connectors. For each: (i) `tenant_id` is required for new
//     tenant-aware access paths; (ii) indexes and uniqueness must be scoped
//     by tenant; (iii) tenancy helpers may wire ownership, but credential
//     resolution, OAuth lifecycle, redaction, and authorization remain in
//     the R37 domain packages.
//
// Both groups share an exported-signature snapshot under
// `testdata/r37_boundary_signatures.golden` covering daemon/internal/secrets,
// providers, managedproviders, mcp, and connectors. Regenerate via
// `go test ./internal/store/tenancy/ -update -run TestR37BoundarySignaturesGolden`.
// Updates MUST be co-reviewed with the Roadmap 37 owner.

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

var updateGolden = flag.Bool("update", false, "rewrite the R37 boundary signature golden")

// Group A — R35 grants tenant ownership.
var groupATables = []string{
	"secret_scope_bindings",
	"provider_preferences",
	"mcp_tool_exposure_rules",
}

// Group B — R37 adds explicit tenant ownership.
var groupBTables = []string{
	"provider_auth_states",
	"mcp_servers",
	"mcp_server_states",
	"mcp_tools",
	"connectors",
}

// boundaryPackages enumerates the dirs whose exported surface is captured in
// the signature golden. Paths are relative to the daemon module root.
var boundaryPackages = []string{
	"internal/secrets",
	"internal/providers",
	"internal/managedproviders",
	"internal/mcp",
	"internal/connectors",
}

func TestR37GroupASchemaHasTenantID(t *testing.T) {
	cols := loadColumnsByTable(t)
	for _, table := range groupATables {
		if _, ok := cols[table]; !ok {
			t.Errorf("Group A table %s missing from schema", table)
			continue
		}
		if !hasColumn(cols[table], "tenant_id") {
			t.Errorf("Group A table %s MUST have tenant_id column (R35 ownership)", table)
		}
	}
}

func TestR37GroupBSchemaHasTenantID(t *testing.T) {
	cols := loadColumnsByTable(t)
	for _, table := range groupBTables {
		if _, ok := cols[table]; !ok {
			t.Errorf("Group B table %s missing from schema", table)
			continue
		}
		if !hasColumn(cols[table], "tenant_id") {
			t.Errorf("Group B table %s MUST have tenant_id column for R37 credential isolation", table)
		}
	}
}

func TestR37GroupBTenantHelpersArePresent(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	helperPath := filepath.Join(filepath.Dir(thisFile), "r37_resources.go")
	body, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("read R37 resource helpers: %v", err)
	}
	text := string(body)
	for _, table := range groupBTables {
		needle := `"` + table + `"`
		if !strings.Contains(text, needle) {
			t.Errorf("R37 resource helper missing Group B table %s", table)
		}
	}
}

func TestR37BoundarySignaturesGolden(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	daemonRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	goldenPath := filepath.Join(filepath.Dir(thisFile), "testdata", "r37_boundary_signatures.golden")

	got, err := collectExportedSignatures(daemonRoot, boundaryPackages)
	if err != nil {
		t.Fatalf("collect signatures: %v", err)
	}

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("R37 boundary signature drift detected.\nThis means a public symbol changed in one of: %v\nIf intentional and co-reviewed with Roadmap 37 owner, regenerate with:\n  go test ./internal/store/tenancy/ -update -run TestR37BoundarySignaturesGolden\n\nDIFF:\n%s", boundaryPackages, signatureDiff(string(want), string(got)))
	}
}

// loadColumnsByTable boots a fresh head-version SQLite store and returns
// the column set per table (lowercased names) for schema introspection.
func loadColumnsByTable(t *testing.T) map[string]map[string]struct{} {
	t.Helper()
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	rows, err := s.DB().QueryContext(context.Background(),
		`SELECT m.name AS table_name, p.name AS column_name FROM sqlite_master m
		 JOIN pragma_table_info(m.name) p ON 1=1
		 WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	defer rows.Close()
	out := map[string]map[string]struct{}{}
	for rows.Next() {
		var table, col string
		if err := rows.Scan(&table, &col); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, ok := out[table]; !ok {
			out[table] = map[string]struct{}{}
		}
		out[table][strings.ToLower(col)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	return out
}

func hasColumn(cols map[string]struct{}, name string) bool {
	_, ok := cols[strings.ToLower(name)]
	return ok
}

// collectExportedSignatures walks each package directory and returns a
// canonical text of every exported func / type / var / const declaration.
// Test files are excluded.
func collectExportedSignatures(daemonRoot string, pkgs []string) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("// AUTO-GENERATED by TestR37BoundarySignaturesGolden.\n")
	out.WriteString("// Regenerate with: go test ./internal/store/tenancy/ -update -run TestR37BoundarySignaturesGolden\n")
	out.WriteString("// Roadmap 37 owner MUST co-review changes to this file.\n\n")

	for _, rel := range pkgs {
		dir := filepath.Join(daemonRoot, rel)
		entries, err := os.ReadDir(dir)
		if err != nil {
			// Empty / not-yet-existing packages are recorded explicitly so
			// drift detection still fires when files are first added.
			fmt.Fprintf(&out, "## package %s\n(no .go files)\n\n", rel)
			continue
		}
		var goFiles []string
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			goFiles = append(goFiles, filepath.Join(dir, n))
		}
		sort.Strings(goFiles)

		fmt.Fprintf(&out, "## package %s\n", rel)
		if len(goFiles) == 0 {
			out.WriteString("(no non-test .go files)\n\n")
			continue
		}

		fset := token.NewFileSet()
		var sigs []string
		for _, gf := range goFiles {
			file, err := parser.ParseFile(fset, gf, nil, parser.ParseComments)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", gf, err)
			}
			sigs = append(sigs, extractExportedDecls(fset, file)...)
		}
		sort.Strings(sigs)
		for _, s := range sigs {
			out.WriteString(s)
			out.WriteString("\n")
		}
		out.WriteString("\n")
	}
	return out.Bytes(), nil
}

func extractExportedDecls(fset *token.FileSet, file *ast.File) []string {
	var out []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name == nil || !d.Name.IsExported() {
				continue
			}
			recv := ""
			if d.Recv != nil && len(d.Recv.List) > 0 {
				recv = "(" + nodeText(fset, d.Recv.List[0].Type) + ") "
			}
			sig := nodeText(fset, &ast.FuncType{Func: token.NoPos, TypeParams: d.Type.TypeParams, Params: d.Type.Params, Results: d.Type.Results})
			sig = strings.Replace(sig, "func", "", 1)
			out = append(out, "func "+recv+d.Name.Name+sig)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !s.Name.IsExported() {
						continue
					}
					out = append(out, "type "+s.Name.Name+" "+nodeText(fset, s.Type))
				case *ast.ValueSpec:
					for i, n := range s.Names {
						if !n.IsExported() {
							continue
						}
						kind := "var"
						if d.Tok == token.CONST {
							kind = "const"
						}
						typ := ""
						if s.Type != nil {
							typ = " " + nodeText(fset, s.Type)
						}
						val := ""
						if i < len(s.Values) && s.Values[i] != nil {
							val = " = " + nodeText(fset, s.Values[i])
						}
						out = append(out, kind+" "+n.Name+typ+val)
					}
				}
			}
		}
	}
	return out
}

func nodeText(fset *token.FileSet, n ast.Node) string {
	if n == nil {
		return ""
	}
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces, Tabwidth: 4}
	if err := cfg.Fprint(&buf, fset, n); err != nil {
		return fmt.Sprintf("<print-err: %v>", err)
	}
	return buf.String()
}

func signatureDiff(want, got string) string {
	wlines := strings.Split(want, "\n")
	glines := strings.Split(got, "\n")
	wset := map[string]struct{}{}
	gset := map[string]struct{}{}
	for _, l := range wlines {
		wset[l] = struct{}{}
	}
	for _, l := range glines {
		gset[l] = struct{}{}
	}
	var only struct {
		want []string
		got  []string
	}
	for l := range wset {
		if _, ok := gset[l]; !ok {
			only.want = append(only.want, l)
		}
	}
	for l := range gset {
		if _, ok := wset[l]; !ok {
			only.got = append(only.got, l)
		}
	}
	sort.Strings(only.want)
	sort.Strings(only.got)
	var out bytes.Buffer
	if len(only.want) > 0 {
		out.WriteString("REMOVED (in golden, not in current):\n")
		for _, l := range only.want {
			out.WriteString("  - " + l + "\n")
		}
	}
	if len(only.got) > 0 {
		out.WriteString("ADDED (in current, not in golden):\n")
		for _, l := range only.got {
			out.WriteString("  + " + l + "\n")
		}
	}
	return out.String()
}
