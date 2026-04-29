package livevalidation

import (
	"context"
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestResolveReconciliationRequiresAuthority(t *testing.T) {
	manager := NewManager(Dependencies{Enabled: true, Store: &memoryStore{}, Clock: fixedClock})
	viewer := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_viewer",
		Role:        identity.RoleViewer,
		Permissions: identity.PermissionsForRole(identity.RoleViewer, identity.StatusActive),
	})
	if _, err := manager.ResolveReconciliation(viewer, ReconciliationResolution{AmbiguousCommitID: "amb_1", Resolution: ResolutionConfirmedCommitted, Reason: "checked"}); !errors.Is(err, ErrReconciliationPermissionDenied) {
		t.Fatalf("ResolveReconciliation err=%v, want permission denied", err)
	}
	admin := tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    "ten_1",
		PrincipalID: "prn_admin",
		Role:        identity.RoleAdmin,
		Permissions: identity.PermissionsForRole(identity.RoleAdmin, identity.StatusActive),
	})
	resolution, err := manager.ResolveReconciliation(admin, ReconciliationResolution{AmbiguousCommitID: "amb_1", Resolution: ResolutionConfirmedCommitted, Reason: "checked"})
	if err != nil {
		t.Fatalf("ResolveReconciliation returned error: %v", err)
	}
	if resolution.ResolvedBy != "prn_admin" || resolution.TenantID != "ten_1" {
		t.Fatalf("resolution=%+v, want admin tenant evidence", resolution)
	}
}
