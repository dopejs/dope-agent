// Package tenantctx is the shared carrier for the resolved tenant context.
//
// Roadmap 35 introduces tenant ownership for every persisted runtime / product /
// harness record. Both the API middleware (`daemon/internal/api`) that resolves
// tenant context from the request and the store-layer guards
// (`daemon/internal/store/tenancy`) that enforce tenant scoping on every query
// need to read and write the same context value. This package owns the carrier
// so the store-layer guards do not depend on api-private symbols.
package tenantctx

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

// ErrTenantContextRequired is returned by store-layer helpers when a
// tenant-owned read or write is attempted without a resolved tenant context.
// This is the single source of truth for the sentinel; the tenancy package
// re-exports it.
var ErrTenantContextRequired = errors.New("tenant context required")

type contextKey string

const tenantContextKey contextKey = "tenant_context"

// WithContext returns a derived context that carries the resolved tenant
// context value. It is called by the API middleware after the identity
// manager resolves the caller.
func WithContext(ctx context.Context, tc identity.TenantContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, tenantContextKey, tc)
}

// FromContext extracts the resolved tenant context if present. Callers that
// require a tenant value should use Require instead.
func FromContext(ctx context.Context) (identity.TenantContext, bool) {
	if ctx == nil {
		return identity.TenantContext{}, false
	}
	tc, ok := ctx.Value(tenantContextKey).(identity.TenantContext)
	if !ok {
		return identity.TenantContext{}, false
	}
	return tc, true
}

// Require returns the resolved TenantID or ErrTenantContextRequired. It is
// the canonical entry point for store-layer helpers that gate access on
// tenant context.
func Require(ctx context.Context) (string, error) {
	tc, ok := FromContext(ctx)
	if !ok || tc.TenantID == "" {
		return "", ErrTenantContextRequired
	}
	return tc.TenantID, nil
}
