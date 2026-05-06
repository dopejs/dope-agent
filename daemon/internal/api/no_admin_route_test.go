package api

// Roadmap 35 (US3 / T089b) — no admin cross-tenant route surface.
//
// CONVENTION (enforced by this test for the lifetime of Roadmap 35):
//
//   - The literal comment marker `// admin: cross-tenant` is FORBIDDEN
//     anywhere under daemon/internal/api/ and daemon/internal/store/. Any
//     handler that needs to bypass tenant scoping MUST wait for Roadmap 36
//     to introduce a sanctioned admin surface; until then the marker exists
//     ONLY in this test file (where it is permitted as documentation).
//   - Every route registered by NewServer in server.go MUST wrap its handler
//     in `protected(` (resolves a tenant context) or `withEnvironment(`
//     (unauthenticated environment resolution for pairing). The only
//     exemptions are health/version/system-info introspection endpoints,
//     enumerated below.
//
// The test scans server.go source rather than the runtime mux because Go's
// http.ServeMux exposes no public route enumeration; reading source keeps
// the assertion simple and survives runtime injection of more routes.
//
// When Roadmap 36 lands the admin surface, move the marker convention into
// the admin sub-package and update T090's runbook accordingly.

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const forbiddenAdminMarker = "// admin: " + "cross-tenant"

// allowedUnauthenticatedRoutes lists the literal first-argument string of
// each mux.HandleFunc call that is NOT required to wrap its handler in
// protected(). Health/version/system-info endpoints are the only fully
// unauthenticated sanctioned exemptions.
var allowedUnauthenticatedRoutes = map[string]struct{}{
	"/healthz":        {},
	"/version":        {},
	"/v1/system/info": {},
}

// allowedWithEnvironmentRoutes lists the routes that are permitted to use
// `withEnvironment(` instead of `protected(`. withEnvironment is NOT a
// tenant-aware wrapper — it only resolves the environment scope for
// pre-pairing requests where no tenant context exists yet. Treating it
// as tenant-equivalent (the previous behavior) would silently allow new
// admin-style routes to bypass tenant resolution. The pairing entry
// points are the only legitimate users.
var allowedWithEnvironmentRoutes = map[string]struct{}{
	"/v1/auth/pairings/start": {},
	"/v1/auth/pairings/":      {},
}

func TestNoAdminCrossTenantMarker(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	daemonRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	for _, sub := range []string{
		filepath.Join("internal", "api"),
		filepath.Join("internal", "store"),
	} {
		root := filepath.Join(daemonRoot, sub)
		offenders := scanForMarker(t, root, forbiddenAdminMarker)
		if len(offenders) > 0 {
			t.Errorf("forbidden marker %q found under %s in: %v", forbiddenAdminMarker, sub, offenders)
		}
	}
}

func TestAllRoutesGoThroughTenantContext(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	serverPath := filepath.Join(filepath.Dir(thisFile), "server.go")

	src, err := os.ReadFile(serverPath)
	if err != nil {
		t.Fatalf("read %s: %v", serverPath, err)
	}

	// Strict per-line match. The regex captures the path AND the literal
	// token immediately following the comma — must be `protected` or
	// `withEnvironment` (with `(` opening the wrapper call). This avoids
	// the prior loose 200-char substring scan that would have accepted
	// `mux.HandleFunc("/foo", func(w,r) { ...; protected(...) })` —
	// where `protected(` lives inside the handler body, not wrapping it.
	//
	// Format expected for protected routes:
	//   mux.HandleFunc("/v1/...", protected(func(w,r,...){...}))
	// Format expected for the pairing routes:
	//   mux.HandleFunc("/v1/auth/pairings/start", withEnvironment(func(w,r){...}))
	registerRE := regexp.MustCompile(`mux\.HandleFunc\("([^"]+)"\s*,\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	matches := registerRE.FindAllSubmatch(src, -1)
	if len(matches) == 0 {
		t.Fatalf("no mux.HandleFunc registrations matched (regex regression?)")
	}

	var (
		unprotected       []string
		misuseEnvironment []string
	)
	for _, m := range matches {
		path := string(m[1])
		wrapper := string(m[2])
		if _, ok := allowedUnauthenticatedRoutes[path]; ok {
			continue
		}
		switch wrapper {
		case "protected":
			// OK — tenant-aware wrapper.
		case "withEnvironment":
			// Only allowed for the explicit pre-pairing entry points.
			if _, ok := allowedWithEnvironmentRoutes[path]; !ok {
				misuseEnvironment = append(misuseEnvironment, path)
			}
		default:
			unprotected = append(unprotected, path+" (wrapped in "+wrapper+"())")
		}
	}
	if len(unprotected) > 0 {
		t.Errorf("the following routes are NOT wrapped in protected(): %v", unprotected)
	}
	if len(misuseEnvironment) > 0 {
		t.Errorf("the following routes use withEnvironment() but are not in the allow-list "+
			"(withEnvironment is NOT tenant-equivalent — it must be reserved for the pairing "+
			"entry points): %v", misuseEnvironment)
	}
}

// scanForMarker walks root and returns paths of *.go files containing
// the literal marker. The forbiddenAdminMarker constant in this file is
// itself excluded so the test definition does not register as an offender.
func scanForMarker(t *testing.T, root, marker string) []string {
	t.Helper()
	var hits []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Exempt this test file (it must reference the marker to enforce it).
		if strings.HasSuffix(path, "no_admin_route_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), marker) {
			hits = append(hits, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return hits
}
