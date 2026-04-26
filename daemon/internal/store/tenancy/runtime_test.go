package tenancy

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestRuntimeTenantID_ReturnsResolvedID(t *testing.T) {
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_abc12345",
		PrincipalID: "prn_def67890",
	})
	if got := RuntimeTenantID(ctx); got != "ten_abc12345" {
		t.Fatalf("RuntimeTenantID = %q, want %q", got, "ten_abc12345")
	}
}

func TestRuntimeTenantID_ReturnsEmptyWhenMissing(t *testing.T) {
	if got := RuntimeTenantID(context.Background()); got != "" {
		t.Fatalf("RuntimeTenantID without ctx = %q, want empty (Pass A allows NULL writes)", got)
	}
}

func TestRuntimeTenantPredicate_EmptyTenantReturnsNoFragment(t *testing.T) {
	frag, args := RuntimeTenantPredicate("")
	if frag != "" || len(args) != 0 {
		t.Fatalf("empty tenant should produce no predicate, got frag=%q args=%v", frag, args)
	}
}

func TestRuntimeTenantPredicate_AllowsNullDuringPassA(t *testing.T) {
	frag, args := RuntimeTenantPredicate("ten_abc12345")
	want := "(tenant_id = ? OR tenant_id IS NULL)"
	if frag != want {
		t.Fatalf("predicate fragment = %q, want %q", frag, want)
	}
	if len(args) != 1 || args[0] != "ten_abc12345" {
		t.Fatalf("predicate args = %v, want [ten_abc12345]", args)
	}
}
