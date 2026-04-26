// Package tenancy centralizes tenant context resolution and tenant-aware
// store access for tenant-owned tables.
//
// Every tenant-owned read or write goes through this package. Helpers either
// resolve the acting tenant from the request context (via
// daemon/internal/tenantctx) or accept an explicit TenantID for background
// callers that already have a value but no context. Tenant-mismatch detection
// emits the audit event defined by Roadmap 35; missing-context calls return
// ErrTenantContextRequired.
package tenancy
