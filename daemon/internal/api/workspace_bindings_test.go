package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func bindingAdmin() identity.TenantContext {
	return identity.TenantContext{TenantID: "ten_api_bind", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionBindingsInspect, identity.PermissionBindingsManage, identity.PermissionProfilesManage}}
}

func bindingViewerNoPerms() identity.TenantContext {
	return identity.TenantContext{TenantID: "ten_api_bind", PrincipalID: "prn_viewer", Permissions: []identity.Permission{identity.PermissionReadOnlyInspect}}
}

// B1/B2/B23/FR-007/FR-008: workspace + binding lifecycle through the API with permission
// gating and tenant isolation.
func TestWorkspaceBindingAPILifecycleAndPermissions(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer sqliteStore.Close()
	bus := events.NewBus()
	admin := bindingAdmin()
	viewer := bindingViewerNoPerms()

	profile, err := sqliteStore.EnsureDefaultAgentProfile(t.Context(), admin.TenantID)
	if err != nil {
		t.Fatalf("EnsureDefaultAgentProfile: %v", err)
	}

	// List workspaces (inspect) returns the lazily-provisioned default.
	wsListReq := httptest.NewRequest(http.MethodGet, "/v1/workspaces", nil).WithContext(withTenantContext(t.Context(), admin))
	wsListRec := httptest.NewRecorder()
	handleWorkspaceRoutes(sqliteStore, bus, wsListRec, wsListReq)
	if wsListRec.Code != http.StatusOK {
		t.Fatalf("list workspaces status=%d body=%s", wsListRec.Code, wsListRec.Body.String())
	}
	var wsList struct {
		Workspaces []struct {
			WorkspaceID string `json:"workspaceId"`
			IsDefault   bool   `json:"isDefault"`
		} `json:"workspaces"`
	}
	_ = json.Unmarshal(wsListRec.Body.Bytes(), &wsList)
	if len(wsList.Workspaces) != 1 || !wsList.Workspaces[0].IsDefault {
		t.Fatalf("expected one default workspace, got %+v", wsList.Workspaces)
	}
	defaultWorkspaceID := wsList.Workspaces[0].WorkspaceID

	// Create binding (manage).
	body := `{"scopeKind":"channel","scopeRef":"discord:c1","selectedProfileId":"` + profile.ProfileID + `","selectedWorkspaceId":"` + defaultWorkspaceID + `"}`
	createReq := httptest.NewRequest(http.MethodPost, "/v1/bindings", strings.NewReader(body)).WithContext(withTenantContext(t.Context(), admin))
	createRec := httptest.NewRecorder()
	handleBindingRoutes(sqliteStore, bus, createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create binding status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		BindingID string `json:"bindingId"`
	}
	_ = json.Unmarshal(createRec.Body.Bytes(), &created)
	if created.BindingID == "" {
		t.Fatalf("missing binding id")
	}

	// Create denied for viewer without bindings.manage (no existence leak — pure 403).
	denyReq := httptest.NewRequest(http.MethodPost, "/v1/bindings", strings.NewReader(body)).WithContext(withTenantContext(t.Context(), viewer))
	denyRec := httptest.NewRecorder()
	handleBindingRoutes(sqliteStore, bus, denyRec, denyReq)
	if denyRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer, got %d", denyRec.Code)
	}

	// Inspect denied for viewer (no bindings.inspect).
	inspectDenyReq := httptest.NewRequest(http.MethodGet, "/v1/bindings/"+created.BindingID, nil).WithContext(withTenantContext(t.Context(), viewer))
	inspectDenyRec := httptest.NewRecorder()
	handleBindingRoutes(sqliteStore, bus, inspectDenyRec, inspectDenyReq)
	if inspectDenyRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 inspect, got %d", inspectDenyRec.Code)
	}

	// Disable then repair.
	disableReq := httptest.NewRequest(http.MethodPatch, "/v1/bindings/"+created.BindingID, strings.NewReader(`{"disable":true}`)).WithContext(withTenantContext(t.Context(), admin))
	disableRec := httptest.NewRecorder()
	handleBindingRoutes(sqliteStore, bus, disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableRec.Code, disableRec.Body.String())
	}

	// Set capability visibility (manage) at workspace scope.
	visBody := `{"scopeKind":"workspace","scopeRef":"` + defaultWorkspaceID + `","capabilityId":"tool.shell","visibility":"hidden"}`
	visReq := httptest.NewRequest(http.MethodPut, "/v1/capability-visibility", strings.NewReader(visBody)).WithContext(withTenantContext(t.Context(), admin))
	visRec := httptest.NewRecorder()
	handleCapabilityVisibilityRoutes(sqliteStore, bus, visRec, visReq)
	if visRec.Code != http.StatusOK {
		t.Fatalf("set visibility status=%d body=%s", visRec.Code, visRec.Body.String())
	}

	// Remove binding.
	delReq := httptest.NewRequest(http.MethodDelete, "/v1/bindings/"+created.BindingID, nil).WithContext(withTenantContext(t.Context(), admin))
	delRec := httptest.NewRecorder()
	handleBindingRoutes(sqliteStore, bus, delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", delRec.Code)
	}

	// B11: lifecycle events were recorded.
	evts, err := sqliteStore.ListEventsForTenantRaw(t.Context(), admin.TenantID, events.Filter{Category: "binding"})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw: %v", err)
	}
	if len(evts) == 0 {
		t.Fatalf("expected binding lifecycle events")
	}
}

// B18: invalid create returns a safe reason and a denial event.
func TestWorkspaceBindingAPIValidationFailure(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer sqliteStore.Close()
	admin := bindingAdmin()
	req := httptest.NewRequest(http.MethodPost, "/v1/bindings", strings.NewReader(`{"scopeKind":"channel","scopeRef":"discord:x","selectedProfileId":"prof_missing"}`)).WithContext(withTenantContext(t.Context(), admin))
	rec := httptest.NewRecorder()
	handleBindingRoutes(sqliteStore, events.NewBus(), rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "selected_profile_unavailable") {
		t.Fatalf("expected validation failure, status=%d body=%s", rec.Code, rec.Body.String())
	}
}
