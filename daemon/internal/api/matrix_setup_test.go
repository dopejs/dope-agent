package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestProjectMatrixSetupKeepsSecretsOutOfAPIShape(t *testing.T) {
	t.Parallel()

	projection := projectMatrixSetup(store.MatrixHostedSetupRecord{
		TenantID:            "ten_matrix",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		TerminalState:       "action-required",
		BotCredentialState:  "invalid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "blocked",
		DeliveryEligible:    false,
		HomeserverBindingID: "matrix_hs_1",
		ReasonCode:          "bot_auth_invalid",
		RedactionStatus:     "redacted",
	})
	if projection.ConnectorKind != "matrix" || projection.DeliveryEligible {
		t.Fatalf("unexpected matrix setup projection: %+v", projection)
	}
	if projection.RedactionStatus != "redacted" || projection.ReasonCode == "" {
		t.Fatalf("expected redacted actionable projection, got %+v", projection)
	}
}

func TestMatrixSetupRouteIsTenantScopedAndRedacted(t *testing.T) {
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
	now := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	if err := sqliteStore.SaveMatrixHostedSetup(context.Background(), store.MatrixHostedSetupRecord{
		TenantID:            "ten_matrix_api",
		ConnectorID:         "matrix-main",
		ConnectorKind:       "matrix",
		DisplayName:         "Matrix Main",
		Status:              "degraded",
		TerminalState:       "action-required",
		BotCredentialState:  "valid",
		HomeserverState:     "reachable",
		RoutePolicyState:    "valid",
		DeliveryEligible:    false,
		HomeserverBindingID: "matrix_hs_1",
		ReasonCode:          "blocked_route",
		RedactionStatus:     "redacted",
		CreatedAt:           now,
		UpdatedAt:           now,
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		HomeserverBinding: &store.MatrixHomeserverBindingRecord{
			HomeserverBindingID:       "matrix_hs_1",
			HomeserverURL:             "https://matrix.example.org",
			HomeserverName:            "matrix.example.org",
			BotUserID:                 "@bot:example.org",
			AuthorizationState:        "valid",
			HomeserverCapabilityState: "valid",
			ValidatedAt:               now,
			RedactionStatus:           "redacted",
			SafeEvidence:              map[string]string{"source": "test"},
		},
	}); err != nil {
		t.Fatalf("SaveMatrixHostedSetup returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/connectors/matrix-main/matrix-setup", nil)
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
		t.Fatalf("matrix setup status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "" || rec.Body.String() == "null\n" {
		t.Fatalf("expected matrix setup projection body, got %q", rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode matrix setup projection: %v", err)
	}
	for _, field := range []string{"status", "createdAt", "updatedAt", "retentionExpiresAt"} {
		if _, ok := body[field]; !ok {
			t.Fatalf("matrix setup projection missing schema-required field %s: %s", field, rec.Body.String())
		}
	}
	if _, ok := body["homeserverBinding"]; !ok {
		t.Fatalf("matrix setup projection should include homeserver binding when stored: %s", rec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors/matrix-main/matrix-setup", nil)
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
		t.Fatalf("cross-tenant setup status=%d body=%s, want 404", otherRec.Code, otherRec.Body.String())
	}
}

func TestMatrixConnectorCreationAndListingAreTenantScoped(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	supervisor := connectors.NewSupervisor()
	tenantContext := identity.TenantContext{
		TenantID:    "ten_matrix_api",
		PrincipalID: "prn_matrix_api",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/connectors", strings.NewReader(`{"connectorId":"matrix-main","kind":"matrix","displayName":"Matrix Main"}`))
	createReq = createReq.WithContext(withTenantContext(createReq.Context(), tenantContext))
	createRec := httptest.NewRecorder()
	handleConnectors(supervisor, events.NewBus(), sqliteStore, createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("matrix connector create status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"kind":"matrix"`) || !strings.Contains(createRec.Body.String(), `"tenantId":"ten_matrix_api"`) {
		t.Fatalf("matrix connector create projection missing kind or tenant: %s", createRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/connectors", nil)
	listReq = listReq.WithContext(withTenantContext(listReq.Context(), tenantContext))
	listRec := httptest.NewRecorder()
	handleConnectors(supervisor, nil, sqliteStore, listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("matrix connector list status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"connectorId":"matrix-main"`) || !strings.Contains(listRec.Body.String(), `"kind":"matrix"`) {
		t.Fatalf("matrix connector list missing registered connector: %s", listRec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodGet, "/v1/connectors", nil)
	otherReq = otherReq.WithContext(withTenantContext(otherReq.Context(), identity.TenantContext{
		TenantID:    "ten_other",
		PrincipalID: "prn_other",
		Permissions: []identity.Permission{
			identity.PermissionConnectorsManage,
			identity.PermissionCredentialsInspect,
		},
	}))
	otherRec := httptest.NewRecorder()
	handleConnectors(supervisor, nil, sqliteStore, otherRec, otherReq)
	if otherRec.Code != http.StatusOK {
		t.Fatalf("cross-tenant connector list status=%d body=%s", otherRec.Code, otherRec.Body.String())
	}
	if strings.Contains(otherRec.Body.String(), "matrix-main") {
		t.Fatalf("cross-tenant connector list leaked Matrix connector: %s", otherRec.Body.String())
	}
}
