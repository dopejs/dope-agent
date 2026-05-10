package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	matrixconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/matrix"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestMatrixSupportInspectionProjectionIsPermissionSafeAndCurrent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	projection := projectMatrixSupportInspection(store.MatrixHostedSetupRecord{
		TenantID:        "ten_matrix",
		ConnectorID:     "matrix-main",
		TerminalState:   "action-required",
		RedactionStatus: "redacted",
	}, matrixconnector.MatrixConditionBlockedRoute, now, now.Add(90*time.Second))

	if projection.FreshnessState != "fresh" || projection.InspectionElapsedMs > int64((2*time.Minute).Milliseconds()) {
		t.Fatalf("expected within-2-minute fresh inspection, got %+v", projection)
	}
	if projection.LatestMatrixCondition != string(matrixconnector.MatrixConditionBlockedRoute) || projection.RedactionStatus != "redacted" {
		t.Fatalf("expected redacted Matrix condition projection, got %+v", projection)
	}
}

func TestMatrixDiagnosticsRouteIsTenantScopedPermissionGatedAndFresh(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	if _, _, err := supervisor.Register(connectors.RegisterInput{
		TenantID:    "ten_matrix_api",
		ConnectorID: "matrix-main",
		Kind:        "matrix",
		DisplayName: "Matrix Main",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	now := time.Now().UTC()
	diagnostic, err := connectors.ClassifyDiagnostic(connectors.DiagnosticInput{
		TenantID:          "ten_matrix_api",
		ConnectorID:       "matrix-main",
		ReasonCode:        connectors.DiagnosticBlockedRoute,
		EvidenceTimestamp: now.Add(-90 * time.Second),
		RedactionReliable: true,
		SafeEvidence:      map[string]string{"stage": "matrix_support_inspection"},
	})
	if err != nil {
		t.Fatalf("ClassifyDiagnostic returned error: %v", err)
	}
	if err := sqliteStore.SaveConnectorDiagnosticState(context.Background(), diagnostic); err != nil {
		t.Fatalf("SaveConnectorDiagnosticState returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/matrix-main/diagnostics", nil)
	req = req.WithContext(withTenantContext(req.Context(), identity.TenantContext{
		TenantID:    "ten_matrix_api",
		PrincipalID: "prn_matrix_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	rec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("diagnostics status=%d body=%s", rec.Code, rec.Body.String())
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/matrix-main/diagnostics", nil)
	deniedReq = deniedReq.WithContext(withTenantContext(deniedReq.Context(), identity.TenantContext{
		TenantID:    "ten_matrix_api",
		PrincipalID: "prn_matrix_denied",
	}))
	deniedRec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("permission denied status=%d body=%s, want 403", deniedRec.Code, deniedRec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/matrix-main/diagnostics", nil)
	otherReq = otherReq.WithContext(withTenantContext(otherReq.Context(), identity.TenantContext{
		TenantID:    "ten_other",
		PrincipalID: "prn_other",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	otherRec := httptest.NewRecorder()
	handleConnectorRoutes(supervisor, nil, nil, nil, sqliteStore, nil, otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant diagnostics status=%d body=%s, want 404", otherRec.Code, otherRec.Body.String())
	}
}
