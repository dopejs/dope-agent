package reminders

import (
	"context"
	"errors"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

// errMissingTenantContext is returned when the follow-up refresh is
// invoked without a resolvable tenant binding. This used to silently
// fall back to a tenantless `Store.ListRuns` scan that violated FR-006;
// the caller MUST now plumb a tenant context through (or rely on the
// daemon's bootstrapped default-personal-tenant cache).
var errMissingTenantContext = errors.New("reminders: refreshFollowUpLink requires a tenant context")

func refreshFollowUpLink(ctx context.Context, sqliteStore *store.SQLiteStore, environmentScope string, link *FollowUpLink) (*FollowUpLink, error) {
	if sqliteStore == nil || link == nil {
		return cloneFollowUpLink(link), nil
	}
	out := cloneFollowUpLink(link)
	now := time.Now().UTC()
	out.LastCheckedAt = &now

	var stale bool
	switch out.LinkKind {
	case FollowUpLinkKindRun:
		// FR-006: cross-tenant existence MUST NOT leak. Resolve the
		// caller's tenant id from context, falling back to the cached
		// default personal tenant. A missing binding is treated as
		// "the source is not visible to the caller, so the link is
		// stale" — the same semantics the tenancy layer enforces for
		// reads of foreign tenant-owned rows.
		tenantID := resolveTenantID(ctx, sqliteStore)
		if tenantID == "" {
			stale = true
			break
		}
		exists, err := sqliteStore.RunExistsForTenant(ctx, out.SourceID, tenantID)
		if err != nil {
			return nil, err
		}
		stale = !exists
	case FollowUpLinkKindWorkflow:
		_, ok, err := sqliteStore.GetWorkflowByID(ctx, environmentScope, out.SourceID)
		if err != nil {
			return nil, err
		}
		stale = !ok
	case FollowUpLinkKindCalendarOperation:
		_, ok, err := sqliteStore.GetCalendarOperationByID(ctx, environmentScope, out.SourceID)
		if err != nil {
			return nil, err
		}
		stale = !ok
	case FollowUpLinkKindMailOperation:
		_, ok, err := sqliteStore.GetMailOperationByID(ctx, environmentScope, out.SourceID)
		if err != nil {
			return nil, err
		}
		stale = !ok
	default:
		stale = false
	}
	out.Stale = stale
	if stale && out.SourceDisplayState == "" {
		out.SourceDisplayState = "stale"
	}
	return out, nil
}

// resolveTenantID returns the caller's resolved tenant id, falling back
// to the bootstrapped default personal tenant. Returns "" only when
// neither is available (pre-bootstrap call paths). Callers treat ""
// as "no visible tenant" rather than as an error to keep the follow-up
// projection robust on the very first daemon boot.
func resolveTenantID(ctx context.Context, s *store.SQLiteStore) string {
	if tc, ok := tenantctx.FromContext(ctx); ok && tc.TenantID != "" {
		return tc.TenantID
	}
	if s == nil {
		return ""
	}
	switch v := s.ResolveDefaultTenantBinding(ctx).(type) {
	case string:
		return v
	default:
		return ""
	}
}

var _ = errMissingTenantContext // reserved for future strict callers
