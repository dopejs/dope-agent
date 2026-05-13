package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/checkpoints"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestAgentProfileAPIProjectsActiveProfileIntoRunAndSessionEvidence(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	bus := events.NewBus()
	tenant := identity.TenantContext{TenantID: "ten_api_profile_runtime", PrincipalID: "prn_admin", Permissions: []identity.Permission{identity.PermissionProfilesInspect, identity.PermissionProfilesManage}}
	viewer := identity.TenantContext{TenantID: tenant.TenantID, PrincipalID: "prn_viewer", Permissions: []identity.Permission{identity.PermissionCredentialsInspect}}
	inspector := identity.TenantContext{TenantID: tenant.TenantID, PrincipalID: "prn_inspector", Permissions: []identity.Permission{identity.PermissionCredentialsInspect, identity.PermissionProfilesInspect}}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/profiles", strings.NewReader(`{"displayName":"Runtime Agent","persona":{"tone":"direct"},"activate":true}`))
	createReq = createReq.WithContext(withTenantContext(createReq.Context(), tenant))
	createRec := httptest.NewRecorder()
	handleAgentProfileRoutes(sqliteStore, bus, createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create profile status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	runtimeManager := runtime.NewManager()
	sessionRouter := router.NewSessionRouter()
	runReq := httptest.NewRequest(http.MethodPost, "/v1/runs", strings.NewReader(`{"entrypoint":"operator","goal":"test profile projection"}`))
	runReq = runReq.WithContext(withTenantContext(runReq.Context(), viewer))
	runRec := httptest.NewRecorder()
	handleRuns(config.Config{Environment: config.EnvironmentTest}, sessionRouter, runtimeManager, bus, nil, nil, sqliteStore, checkpoints.NewManager(sqliteStore, runtimeManager), runRec, runReq)
	if runRec.Code != http.StatusCreated {
		t.Fatalf("create run status=%d body=%s", runRec.Code, runRec.Body.String())
	}
	var createdRun runtime.Run
	if err := json.Unmarshal(runRec.Body.Bytes(), &createdRun); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if createdRun.ActiveProfileProjection != nil {
		t.Fatalf("viewer without profiles.inspect must not see run active profile projection, got %+v", createdRun.ActiveProfileProjection)
	}
	sessionProjections, err := sqliteStore.ListRuntimeProfileProjections(runReq.Context(), tenant.TenantID, string(profiles.RuntimeResourceSession), createdRun.SessionID, "", 1)
	if err != nil {
		t.Fatalf("ListRuntimeProfileProjections returned error: %v", err)
	}
	if len(sessionProjections) != 1 {
		t.Fatalf("expected session projection linked to active profile, got %+v", sessionProjections)
	}
	sessionsReq := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	sessionsReq = sessionsReq.WithContext(withTenantContext(sessionsReq.Context(), viewer))
	sessionsRec := httptest.NewRecorder()
	handleSessions(sessionRouter, bus, sqliteStore, sessionsRec, sessionsReq)
	if sessionsRec.Code != http.StatusOK {
		t.Fatalf("sessions status=%d body=%s", sessionsRec.Code, sessionsRec.Body.String())
	}
	if strings.Contains(sessionsRec.Body.String(), `"activeProfileProjection"`) {
		t.Fatalf("viewer without profiles.inspect must not see session active profile projection, body=%s", sessionsRec.Body.String())
	}
	inspectorSessionsReq := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	inspectorSessionsReq = inspectorSessionsReq.WithContext(withTenantContext(inspectorSessionsReq.Context(), inspector))
	inspectorSessionsRec := httptest.NewRecorder()
	handleSessions(sessionRouter, bus, sqliteStore, inspectorSessionsRec, inspectorSessionsReq)
	if inspectorSessionsRec.Code != http.StatusOK {
		t.Fatalf("inspector sessions status=%d body=%s", inspectorSessionsRec.Code, inspectorSessionsRec.Body.String())
	}
	if !strings.Contains(inspectorSessionsRec.Body.String(), `"activeProfileProjection"`) {
		t.Fatalf("expected inspector session active profile projection, body=%s", inspectorSessionsRec.Body.String())
	}
	runProjections, err := sqliteStore.ListRuntimeProfileProjections(runReq.Context(), tenant.TenantID, string(profiles.RuntimeResourceRun), createdRun.RunID, "", 1)
	if err != nil {
		t.Fatalf("ListRuntimeProfileProjections returned error: %v", err)
	}
	if len(runProjections) != 1 {
		t.Fatalf("expected run projection to remain durable, got %+v", runProjections)
	}
	versions, err := sqliteStore.ListAgentProfileVersions(runReq.Context(), tenant.TenantID, runProjections[0].ProfileID, 10)
	if err != nil {
		t.Fatalf("ListAgentProfileVersions returned error: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("run activity without profiles.manage must not mutate profile versions, got %+v", versions)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+createdRun.RunID, nil)
	getReq = getReq.WithContext(withTenantContext(getReq.Context(), viewer))
	getRec := httptest.NewRecorder()
	handleRunByID(nil, runtimeManager, sqliteStore, getRec, getReq, createdRun.RunID)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get run status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	if strings.Contains(getRec.Body.String(), `"activeProfileProjection"`) {
		t.Fatalf("viewer without profiles.inspect must not see active profile projection in get response, body=%s", getRec.Body.String())
	}
	inspectorGetReq := httptest.NewRequest(http.MethodGet, "/v1/runs/"+createdRun.RunID, nil)
	inspectorGetReq = inspectorGetReq.WithContext(withTenantContext(inspectorGetReq.Context(), inspector))
	inspectorGetRec := httptest.NewRecorder()
	handleRunByID(nil, runtimeManager, sqliteStore, inspectorGetRec, inspectorGetReq, createdRun.RunID)
	if inspectorGetRec.Code != http.StatusOK {
		t.Fatalf("inspector get run status=%d body=%s", inspectorGetRec.Code, inspectorGetRec.Body.String())
	}
	if !strings.Contains(inspectorGetRec.Body.String(), `"activeProfileProjection"`) {
		t.Fatalf("expected inspector active profile projection in get response, body=%s", inspectorGetRec.Body.String())
	}
}
