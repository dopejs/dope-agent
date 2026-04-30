package tenancy_test

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestIntegrationDiagnosticsTenantAccessorBindsAndListsTenantRows(t *testing.T) {
	t.Parallel()

	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	accessor := tenancy.NewIntegrationDiagnostics(s, nil)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_diag", PrincipalID: "operator"})
	now := time.Now().UTC()
	result := integrations.DiagnosticResult{
		DiagnosticResultID: "diag_result_tenant_1",
		IntegrationID:      "integration_diag",
		DomainKind:         "calendar",
		ProviderKind:       "feishu_lark",
		Capability:         "calendar.read",
		Status:             integrations.DiagnosticStatusHealthy,
		ReasonCode:         integrations.ReasonHealthy,
		RemediationOwner:   integrations.RemediationOwnerNoneRequired,
		RetrySafety:        integrations.RetrySafetyNoActionNeeded,
		CheckedAt:          now,
		StaleAfter:         now.Add(integrations.DiagnosticStaleAfter),
		FreshnessState:     integrations.FreshnessStateFresh,
		RedactionStatus:    integrations.RedactionStatusRedacted,
		RetentionExpiresAt: now.Add(90 * 24 * time.Hour),
	}
	if err := accessor.SaveResultForTenant(ctx, result); err != nil {
		t.Fatalf("SaveResultForTenant: %v", err)
	}
	got, err := accessor.ListLatestForTenant(ctx, integrations.DiagnosticResultFilter{IntegrationID: "integration_diag"})
	if err != nil {
		t.Fatalf("ListLatestForTenant: %v", err)
	}
	if len(got) != 1 || got[0].TenantID != "ten_diag" {
		t.Fatalf("unexpected tenant diagnostic results: %+v", got)
	}
	if err := accessor.SaveResultForTenant(context.Background(), result); err != tenancy.ErrTenantContextRequired {
		t.Fatalf("SaveResultForTenant without tenant err=%v, want ErrTenantContextRequired", err)
	}
}
