package tenantctx

import (
	"context"
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestRequireWithoutContext(t *testing.T) {
	if _, err := Require(context.Background()); !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("expected ErrTenantContextRequired, got %v", err)
	}
}

func TestRequireWithEmptyTenantID(t *testing.T) {
	ctx := WithContext(context.Background(), identity.TenantContext{PrincipalID: "prn_abc"})
	if _, err := Require(ctx); !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("expected ErrTenantContextRequired, got %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	tc := identity.TenantContext{PrincipalID: "prn_a", TenantID: "ten_a", TokenID: "tok_a"}
	ctx := WithContext(context.Background(), tc)

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatalf("FromContext returned !ok")
	}
	if got.PrincipalID != tc.PrincipalID || got.TenantID != tc.TenantID || got.TokenID != tc.TokenID {
		t.Fatalf("FromContext mismatch: got %+v want %+v", got, tc)
	}

	tenantID, err := Require(ctx)
	if err != nil {
		t.Fatalf("Require returned err: %v", err)
	}
	if tenantID != "ten_a" {
		t.Fatalf("Require returned %q, want ten_a", tenantID)
	}
}

func TestNilContext(t *testing.T) {
	if _, ok := FromContext(nil); ok {
		t.Fatalf("FromContext(nil) should return !ok")
	}
	if _, err := Require(nil); !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("Require(nil) expected ErrTenantContextRequired, got %v", err)
	}
}

func TestIsolation(t *testing.T) {
	a := WithContext(context.Background(), identity.TenantContext{TenantID: "ten_a"})
	b := WithContext(context.Background(), identity.TenantContext{TenantID: "ten_b"})

	idA, _ := Require(a)
	idB, _ := Require(b)
	if idA != "ten_a" || idB != "ten_b" {
		t.Fatalf("contexts leaked: a=%q b=%q", idA, idB)
	}
}
