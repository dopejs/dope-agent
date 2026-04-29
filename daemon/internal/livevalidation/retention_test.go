package livevalidation

import (
	"context"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestDefaultRetentionPolicyIsIndefinite(t *testing.T) {
	manager := NewManager(Dependencies{Enabled: true, Clock: fixedClock})
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_1", PrincipalID: "prn_1"})
	policy := manager.DefaultRetentionPolicy(ctx)
	if policy.Mode != RetentionModeIndefinite || policy.AppliesTo != RetentionAppliesAll || policy.TenantID != "ten_1" {
		t.Fatalf("policy=%+v, want indefinite all tenant policy", policy)
	}
}
