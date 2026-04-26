package tenancy_test

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestRequireMissingContext(t *testing.T) {
	if _, err := tenancy.Require(context.Background()); !errors.Is(err, tenancy.ErrTenantContextRequired) {
		t.Fatalf("expected ErrTenantContextRequired, got %v", err)
	}
}

func TestRequireResolvedContext(t *testing.T) {
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_a", PrincipalID: "prn_a"})
	got, err := tenancy.Require(ctx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "ten_a" {
		t.Fatalf("Require returned %q, want ten_a", got)
	}
}

func TestRequireConcurrentIsolation(t *testing.T) {
	a := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_a"})
	b := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_b"})

	idA, _ := tenancy.Require(a)
	idB, _ := tenancy.Require(b)
	if idA != "ten_a" || idB != "ten_b" {
		t.Fatalf("contexts leaked: a=%q b=%q", idA, idB)
	}
}

func TestMustPanicsOnMissingContext(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("Must should panic without tenant context")
		}
	}()
	_ = tenancy.Must(context.Background())
}

// TestNoApiImport asserts the tenancy package never depends on
// daemon/internal/api. The tenancy guards must be reusable from background
// callers that have no request context, and importing api would create an
// import cycle through the auth/middleware layer.
func TestNoApiImport(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}",
		"github.com/dopejs/dope-agent/daemon/internal/store/tenancy").CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "github.com/dopejs/dope-agent/daemon/internal/api" {
			t.Fatalf("tenancy package must not depend on daemon/internal/api")
		}
	}
}
