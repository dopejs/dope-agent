package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// ErrTenantOwnershipDenied is returned by the tenant-aware persist
// helpers (persistRun / persistSession / persistLLMDispatch) when the
// caller's resolved tenant tries to write a row owned by a different
// tenant. The handler maps this to a 404 + audit emission so the
// row's existence is not leaked across tenants.
var ErrTenantOwnershipDenied = errors.New("tenant ownership denied")

// Roadmap 35 (T040+) — request-scope tenant guards used by route
// handlers across every tenant_owned domain. The helpers in this file
// implement the contract:
//
//   - cross-tenant by-id GET: emit `audit.cross_tenant_access_denied`
//     and return 404 (existence is NEVER leaked across tenants);
//   - cross-tenant LIST: post-projection filter against tenant_id;
//   - cross-tenant write: refused with 404 + audit emit so write
//     attempts cannot be used as an oracle either.
//
// Pre-backfill rows whose tenant_id is still NULL are NOT considered
// cross-tenant by these helpers: they are treated as legacy, owned by
// no one, and are visible to every tenant context. The US2 backfill
// (T067 onwards) eliminates the NULL set; once T077a completes,
// `BindRowTenant` will collapse to a single tenant-aware INSERT and
// these guards will short-circuit on a strict equality check.

// resolveAuditEmitter returns the configured emitter, or constructs a
// default one wired against the daemon's event bus if none was injected.
func resolveAuditEmitter(deps Dependencies) *audit.Emitter {
	if deps.AuditEmitter != nil {
		return deps.AuditEmitter
	}
	return audit.NewEmitter(deps.EventBus, deps.Logger)
}

// guardResourceForTenant verifies that the resource keyed by
// (table, pkColumn, pkValue) is owned by the caller's resolved tenant.
// Behaviour:
//   - returns (true, nil) when the row is owned by the caller, OR is
//     pre-backfill (tenant_id IS NULL);
//   - writes 404 + emits audit and returns (false, nil) on cross-tenant;
//   - writes 500 + returns (false, err) on a lookup failure;
//   - returns (true, nil) silently when the request has NO tenant
//     context (Pass A allows legacy/anonymous traffic to keep working
//     during the rollout window — `protected()` already rejects
//     unauthenticated requests upstream when auth is configured).
func guardResourceForTenant(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	st *store.SQLiteStore,
	emitter *audit.Emitter,
	table, pkColumn, pkValue, resourceKind string,
) (bool, error) {
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok || tenantContext.TenantID == "" {
		return true, nil
	}
	if st == nil {
		return true, nil
	}
	owner, found, err := st.LookupRowTenant(ctx, table, pkColumn, pkValue)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return false, err
	}
	if !found {
		// Caller can decide whether to 404; the guard's job is only to
		// reject cross-tenant access. Returning true here lets the
		// downstream handler shape the response.
		return true, nil
	}
	if owner == "" || owner == tenantContext.TenantID {
		return true, nil
	}
	emitTenantBreach(ctx, emitter, surfaceFromRequest(r), resourceKind)
	writeError(w, http.StatusNotFound, "not found")
	return false, nil
}

// emitTenantBreach publishes the audit event for a denied cross-tenant
// access. Safe to call even when the emitter is nil (no-op).
func emitTenantBreach(ctx context.Context, emitter *audit.Emitter, surface, resourceKind string) {
	if emitter == nil {
		return
	}
	_ = emitter.Emit(ctx, surface, resourceKind)
}

// surfaceFromRequest builds the canonical `api:<METHOD route>` surface
// label used in audit emissions. Uses the URL path — query parameters
// and tenant headers are intentionally excluded.
func surfaceFromRequest(r *http.Request) string {
	if r == nil {
		return "api:unknown"
	}
	return "api:" + r.Method + " " + r.URL.Path
}

// withByIDTenantGuard wraps a /v1/<prefix>/ lambda so the {id} segment
// is verified against the caller's tenant before the inner handler
// runs. It applies ONLY when the path strips down to exactly `<id>` or
// `<id>/<sub>`; deeper paths bypass the guard so sub-route handlers
// (e.g. /v1/runs/{id}/cancel) can apply their own checks. The id is
// looked up against (table, pkColumn).
//
// This is the canonical glue used by route registration in NewServer
// to satisfy Roadmap 35 T040-T051: cross-tenant by-id requests get a
// 404 + audit emission BEFORE any domain handler runs.
func withByIDTenantGuard(
	st *store.SQLiteStore,
	emitter *audit.Emitter,
	prefix, table, pkColumn, resourceKind string,
	inner http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if len(path) > len(prefix) && path[:len(prefix)] == prefix {
			rest := path[len(prefix):]
			id := rest
			if idx := indexByte(rest, '/'); idx >= 0 {
				id = rest[:idx]
			}
			if id != "" {
				ok, err := guardResourceForTenant(r.Context(), w, r, st, emitter, table, pkColumn, id, resourceKind)
				if err != nil || !ok {
					return
				}
			}
		}
		inner(w, r)
	}
}

// indexByte mirrors strings.IndexByte without pulling the strings
// import in (kept package-local to make the guard helper file
// self-contained).
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// filterRunsByTenant scopes a list of runs to the caller's tenant.
// Returns the input unchanged when no tenant context is resolved.
// Pre-backfill rows whose tenant_id is still NULL stay visible to
// every tenant (legacy compatibility); post-backfill, every row has
// a tenant id and the filter is strict.
func filterRunsByTenant(ctx context.Context, st *store.SQLiteStore, runs []runtime.Run) ([]runtime.Run, error) {
	tc, ok := tenantContextFromContext(ctx)
	if !ok || tc.TenantID == "" {
		return runs, nil
	}
	return filterRowsByTenant(ctx, st, runs, "runs", "run_id", func(run runtime.Run) string { return run.RunID }, tc.TenantID)
}

// filterLLMDispatchesByTenant scopes a list of LLM dispatches to the
// caller's tenant. Mirrors filterRunsByTenant: pre-backfill rows with
// NULL tenant_id stay visible to every tenant; post-backfill, every row
// has a tenant id and the filter is strict. Returns the input unchanged
// when no tenant context is resolved (admin/maintenance paths).
//
// Roadmap 35 round-3 Finding #1: GET /v1/llm/dispatches previously
// returned every tenant's dispatches because the handler called
// store.ListLLMDispatches without scoping. This helper closes that gap.
func filterLLMDispatchesByTenant(ctx context.Context, st *store.SQLiteStore, dispatches []llm.Dispatch) ([]llm.Dispatch, error) {
	tc, ok := tenantContextFromContext(ctx)
	if !ok || tc.TenantID == "" {
		return dispatches, nil
	}
	return filterRowsByTenant(ctx, st, dispatches, "llm_dispatches", "dispatch_id", func(d llm.Dispatch) string { return d.DispatchID }, tc.TenantID)
}

// filterSessionsByTenant scopes a list of sessions to the caller's tenant.
func filterSessionsByTenant(ctx context.Context, st *store.SQLiteStore, sessions []router.Session) ([]router.Session, error) {
	tc, ok := tenantContextFromContext(ctx)
	if !ok || tc.TenantID == "" {
		return sessions, nil
	}
	return filterRowsByTenant(ctx, st, sessions, "sessions", "session_id", func(s router.Session) string { return s.SessionID }, tc.TenantID)
}

// filterRowsByTenant retains only rows whose tenant_id matches the
// caller's resolved tenant. Rows whose tenant_id is "" (pre-backfill)
// are treated as legacy and are ALSO retained to preserve compatibility
// during the staged rollout. The closure resolves the per-row tenant_id
// so callers don't need to hold a parallel slice.
//
// O(N) lookups; suitable for small bounded result sets (the daemon's
// typical list endpoint returns << 1000 rows). Pass B replaces this
// with a tenant-scoped SQL filter.
func filterRowsByTenant[T any](
	ctx context.Context,
	st *store.SQLiteStore,
	rows []T,
	table, pkColumn string,
	keyOf func(T) string,
	tenantID string,
) ([]T, error) {
	if tenantID == "" || st == nil || len(rows) == 0 {
		return rows, nil
	}
	out := make([]T, 0, len(rows))
	for _, row := range rows {
		owner, found, err := st.LookupRowTenant(ctx, table, pkColumn, keyOf(row))
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if owner == "" || owner == tenantID {
			out = append(out, row)
		}
	}
	return out, nil
}
