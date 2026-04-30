package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestEvaluationProductDashboardAPIRequiresReadPermissionAndListsTenantProjections(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_dashboard_api", PrincipalID: "prn_dashboard"})
	if err := sqliteStore.SaveDashboardProjection(ctx, evaluation.DashboardProjection{
		ProjectionID:         "projection_dashboard_api",
		TenantID:             "ten_dashboard_api",
		WindowStart:          now.Add(-time.Hour),
		WindowEnd:            now,
		CampaignStatusCounts: map[string]int{"completed": 1},
		DriftSummary:         map[string]int{"total": 2},
		GeneratedAt:          now,
	}); err != nil {
		t.Fatalf("SaveDashboardProjection: %v", err)
	}

	denied := httptest.NewRequest(http.MethodGet, "/v1/evaluation/dashboard", nil).WithContext(ctx)
	deniedRec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, deniedRec, denied)
	if deniedRec.Code != http.StatusForbidden || !strings.Contains(deniedRec.Body.String(), "evaluation.dashboard.read") {
		t.Fatalf("denied status=%d body=%s", deniedRec.Code, deniedRec.Body.String())
	}

	allowedCtx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_dashboard_api", PrincipalID: "prn_dashboard", Permissions: []identity.Permission{identity.PermissionEvaluationDashboardRead}})
	req := httptest.NewRequest(http.MethodGet, "/v1/evaluation/dashboard?limit=1", nil).WithContext(allowedCtx)
	rec := httptest.NewRecorder()
	handleEvaluationRoutes(nil, nil, nil, sqliteStore, rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "projection_dashboard_api") || !strings.Contains(rec.Body.String(), `"total":2`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
