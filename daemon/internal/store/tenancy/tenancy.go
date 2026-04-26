package tenancy

import (
	"context"

	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

// ErrTenantContextRequired is returned when a tenant-owned read or write is
// attempted without a resolved tenant context. It is a re-export of the
// tenantctx sentinel so callers in this package don't need to import both.
var ErrTenantContextRequired = tenantctx.ErrTenantContextRequired

// TenantID is the resolved tenant identifier carried in request context.
// Aliased to string to keep call sites lightweight.
type TenantID = string

// Require returns the resolved TenantID from the context, or
// ErrTenantContextRequired if none is present. Every tenant-owned store
// helper that derives the acting tenant from context MUST call Require
// first and reject on error.
func Require(ctx context.Context) (TenantID, error) {
	return tenantctx.Require(ctx)
}

// Must returns the resolved TenantID and panics if missing. It is intended
// only for code paths where missing context is a programmer error (e.g. an
// internal helper invoked directly after a successful Require check by a
// route handler). Production code should prefer Require + error return.
func Must(ctx context.Context) TenantID {
	tenantID, err := Require(ctx)
	if err != nil {
		panic(err)
	}
	return tenantID
}
