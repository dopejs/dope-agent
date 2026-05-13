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

func TestAgentProfileAPIRequiresPermissionsAndMutatesProfiles(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	bus := events.NewBus()
	admin := identity.TenantContext{TenantID: "ten_api_profiles", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesInspect, identity.PermissionProfilesManage}}
	viewer := identity.TenantContext{TenantID: "ten_api_profiles", PrincipalID: "prn_viewer", Permissions: []identity.Permission{identity.PermissionProfilesInspect}}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/profiles", strings.NewReader(`{"displayName":"Support","persona":{"tone":"direct"},"activate":true}`))
	createReq = createReq.WithContext(withTenantContext(createReq.Context(), admin))
	createRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Profile struct {
			ProfileID string `json:"profileId"`
		} `json:"profile"`
		Version struct {
			ProfileVersionID string `json:"profileVersionId"`
		} `json:"version"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/profiles", nil).WithContext(withTenantContext(createReq.Context(), viewer))
	listRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	updateDeniedReq := httptest.NewRequest(http.MethodPatch, "/v1/profiles/"+created.Profile.ProfileID, strings.NewReader(`{"displayName":"Denied"}`))
	updateDeniedReq = updateDeniedReq.WithContext(withTenantContext(updateDeniedReq.Context(), viewer))
	updateDeniedRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, updateDeniedRec, updateDeniedReq)
	if updateDeniedRec.Code != http.StatusForbidden {
		t.Fatalf("denied update status=%d body=%s", updateDeniedRec.Code, updateDeniedRec.Body.String())
	}
	invalidCreateReq := httptest.NewRequest(http.MethodPost, "/v1/profiles", strings.NewReader(`{"displayName":"Invalid","defaultProviderPreference":{"providerId":"bad provider"}}`))
	invalidCreateReq = invalidCreateReq.WithContext(withTenantContext(invalidCreateReq.Context(), admin))
	invalidCreateRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, invalidCreateRec, invalidCreateReq)
	if invalidCreateRec.Code != http.StatusBadRequest || !strings.Contains(invalidCreateRec.Body.String(), "provider_preference_malformed") {
		t.Fatalf("invalid create status=%d body=%s", invalidCreateRec.Code, invalidCreateRec.Body.String())
	}
	persistedEvents, err := sqliteStore.ListEventsForTenantRaw(createReq.Context(), admin.TenantID, events.Filter{Category: "agent_profile"})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw returned error: %v", err)
	}
	foundDenied := false
	foundValidation := false
	for _, event := range persistedEvents {
		if event.Name == "agent_profile.update_denied" && event.Payload["reasonCode"] == "permission_denied" {
			foundDenied = true
		}
		if event.Name == "agent_profile.validation_failed" && event.Payload["reasonCode"] == "provider_preference_malformed" {
			foundValidation = true
		}
	}
	if !foundDenied || !foundValidation {
		t.Fatalf("expected durable denied and validation profile events, got %+v", persistedEvents)
	}

	archiveReq := httptest.NewRequest(http.MethodPost, "/v1/profiles/"+created.Profile.ProfileID+"/archive", strings.NewReader(`{"reasonCode":"test_archive"}`))
	archiveReq = archiveReq.WithContext(withTenantContext(archiveReq.Context(), admin))
	archiveRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", archiveRec.Code, archiveRec.Body.String())
	}
	if got := bus.List(events.Filter{Category: "agent_profile", TenantOwnedTenantID: "ten_api_profiles"}); len(got) == 0 {
		t.Fatalf("expected profile lifecycle events")
	}
}

func TestAgentProfileAPIVersionsRollbackDisableAndOverlays(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	bus := events.NewBus()
	admin := identity.TenantContext{TenantID: "ten_api_profile_history", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesInspect, identity.PermissionProfilesManage}}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/profiles", strings.NewReader(`{"displayName":"Support","persona":{"tone":"direct"},"overlayReferences":[{"referenceKind":"prompt","referenceUri":"prompt://profiles/support","scope":"profile"}]}`))
	createReq = createReq.WithContext(withTenantContext(createReq.Context(), admin))
	createRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Profile struct {
			ProfileID string `json:"profileId"`
		} `json:"profile"`
		Version struct {
			ProfileVersionID string `json:"profileVersionId"`
		} `json:"version"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/v1/profiles/"+created.Profile.ProfileID, strings.NewReader(`{"displayName":"Support Updated","persona":{"tone":"calm"},"overlayReferences":[{"referenceKind":"config","referenceUri":"config://profiles/support","scope":"profile"}]}`))
	updateReq = updateReq.WithContext(withTenantContext(updateReq.Context(), admin))
	updateRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updateRec.Code, updateRec.Body.String())
	}

	removeOverlayReq := httptest.NewRequest(http.MethodPatch, "/v1/profiles/"+created.Profile.ProfileID, strings.NewReader(`{"displayName":"Support Without Overlay","persona":{"tone":"calm"}}`))
	removeOverlayReq = removeOverlayReq.WithContext(withTenantContext(removeOverlayReq.Context(), admin))
	removeOverlayRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, removeOverlayRec, removeOverlayReq)
	if removeOverlayRec.Code != http.StatusOK {
		t.Fatalf("remove overlay status=%d body=%s", removeOverlayRec.Code, removeOverlayRec.Body.String())
	}

	versionsReq := httptest.NewRequest(http.MethodGet, "/v1/profiles/"+created.Profile.ProfileID+"/versions", nil)
	versionsReq = versionsReq.WithContext(withTenantContext(versionsReq.Context(), admin))
	versionsRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, versionsRec, versionsReq)
	if versionsRec.Code != http.StatusOK {
		t.Fatalf("versions status=%d body=%s", versionsRec.Code, versionsRec.Body.String())
	}
	if strings.Count(versionsRec.Body.String(), `"changeKind":"updated"`) < 2 {
		t.Fatalf("expected updated version in history, body=%s", versionsRec.Body.String())
	}

	rollbackReq := httptest.NewRequest(http.MethodPost, "/v1/profiles/"+created.Profile.ProfileID+"/rollback", strings.NewReader(`{"sourceProfileVersionId":"`+created.Version.ProfileVersionID+`"}`))
	rollbackReq = rollbackReq.WithContext(withTenantContext(rollbackReq.Context(), admin))
	rollbackRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, rollbackRec, rollbackReq)
	if rollbackRec.Code != http.StatusOK {
		t.Fatalf("rollback status=%d body=%s", rollbackRec.Code, rollbackRec.Body.String())
	}
	if !strings.Contains(rollbackRec.Body.String(), `"changeKind":"rolled_back"`) {
		t.Fatalf("expected rolled back version, body=%s", rollbackRec.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/v1/profiles/"+created.Profile.ProfileID, nil)
	detailReq = detailReq.WithContext(withTenantContext(detailReq.Context(), admin))
	detailRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, detailRec, detailReq)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
	if !strings.Contains(detailRec.Body.String(), `"referenceKind":"prompt"`) || strings.Contains(detailRec.Body.String(), `"referenceKind":"config"`) {
		t.Fatalf("rollback should expose source overlay only, body=%s", detailRec.Body.String())
	}

	disableReq := httptest.NewRequest(http.MethodPost, "/v1/profiles/"+created.Profile.ProfileID+"/disable", strings.NewReader(`{}`))
	disableReq = disableReq.WithContext(withTenantContext(disableReq.Context(), admin))
	disableRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableRec.Code, disableRec.Body.String())
	}
	if !strings.Contains(disableRec.Body.String(), `"status":"disabled"`) {
		t.Fatalf("expected disabled profile body=%s", disableRec.Body.String())
	}
}

func TestAgentProfileAPIPersistsLifecycleEventsWithoutBus(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	admin := identity.TenantContext{TenantID: "ten_api_profiles_no_bus", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesInspect, identity.PermissionProfilesManage}}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/profiles", strings.NewReader(`{"displayName":"Support","persona":{"tone":"direct"},"activate":true}`))
	createReq = createReq.WithContext(withTenantContext(createReq.Context(), admin))
	createRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, nil, createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	persistedEvents, err := sqliteStore.ListEventsForTenantRaw(createReq.Context(), admin.TenantID, events.Filter{Category: "agent_profile"})
	if err != nil {
		t.Fatalf("ListEventsForTenantRaw returned error: %v", err)
	}
	foundCreated := false
	foundVersion := false
	for _, event := range persistedEvents {
		if event.Name == "agent_profile.created" {
			foundCreated = true
		}
		if event.Name == "agent_profile.version_created" {
			foundVersion = true
		}
	}
	if !foundCreated || !foundVersion {
		t.Fatalf("expected durable profile events without bus, got %+v", persistedEvents)
	}
}
