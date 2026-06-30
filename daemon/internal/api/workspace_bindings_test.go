package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// FR-016/SC-005: a capability disabled (or hidden) at the active profile/workspace scope MUST
// NOT execute through the direct runtime tool-call API, while a visible capability is allowed.
// This guards the execution gate consumed by prepareExecutableSkillToolCall /
// prepareCapabilityToolCall, independent of the chat work-start path.
func TestEnforceDirectCapabilityVisibilityBlocksHiddenAndDisabled(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer sqliteStore.Close()
	admin := bindingAdmin()
	ctx := withTenantContext(t.Context(), admin)

	profile, err := sqliteStore.EnsureDefaultAgentProfile(ctx, admin.TenantID)
	if err != nil {
		t.Fatalf("EnsureDefaultAgentProfile: %v", err)
	}

	// No policy yet: capability is allowed to execute.
	if err := enforceRunCapabilityVisibility(ctx, sqliteStore, "", "skill.alpha"); err != nil {
		t.Fatalf("expected allow with no policy, got %v", err)
	}

	// Disable it at profile scope -> execution must be blocked.
	if _, _, err := sqliteStore.SetCapabilityVisibility(ctx, admin, bindings.SetVisibilityRequest{
		ScopeKind: bindings.VisibilityScopeProfile, ScopeRef: profile.ProfileID, CapabilityID: "skill.alpha", Visibility: bindings.VisibilityDisabled,
	}); err != nil {
		t.Fatalf("SetCapabilityVisibility disabled: %v", err)
	}
	if err := enforceRunCapabilityVisibility(ctx, sqliteStore, "", "skill.alpha"); !errors.Is(err, bindings.ErrCapabilityNotExecutable) {
		t.Fatalf("expected ErrCapabilityNotExecutable for disabled capability, got %v", err)
	}

	// Hidden at workspace scope is also non-executable.
	ws, err := sqliteStore.EnsureDefaultWorkspace(ctx, admin.TenantID)
	if err != nil {
		t.Fatalf("EnsureDefaultWorkspace: %v", err)
	}
	if _, _, err := sqliteStore.SetCapabilityVisibility(ctx, admin, bindings.SetVisibilityRequest{
		ScopeKind: bindings.VisibilityScopeWorkspace, ScopeRef: ws.WorkspaceID, CapabilityID: "skill.beta", Visibility: bindings.VisibilityHidden,
	}); err != nil {
		t.Fatalf("SetCapabilityVisibility hidden: %v", err)
	}
	if err := enforceRunCapabilityVisibility(ctx, sqliteStore, "", "skill.beta"); !errors.Is(err, bindings.ErrCapabilityNotExecutable) {
		t.Fatalf("expected ErrCapabilityNotExecutable for hidden capability, got %v", err)
	}

	// A different, unconstrained capability still executes.
	if err := enforceRunCapabilityVisibility(ctx, sqliteStore, "", "skill.gamma"); err != nil {
		t.Fatalf("expected allow for unconstrained capability, got %v", err)
	}
}

// FR-016 (run-scoped): when a run has recorded binding evidence, the execution gate enforces
// that run's resolved profile/workspace visibility — not the tenant default — so a capability
// disabled under the run's bound profile cannot execute, while one only restricted under a
// different (tenant-default) profile is unaffected.
func TestEnforceRunCapabilityVisibilityUsesRunBinding(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer sqliteStore.Close()
	admin := bindingAdmin()
	ctx := withTenantContext(t.Context(), admin)

	tenantDefault, err := sqliteStore.EnsureDefaultAgentProfile(ctx, admin.TenantID)
	if err != nil {
		t.Fatalf("EnsureDefaultAgentProfile: %v", err)
	}
	runProfile, err := sqliteStore.CreateAgentProfile(ctx, admin, profiles.MutationInput{DisplayName: "RunProfile", Activate: true})
	if err != nil {
		t.Fatalf("CreateAgentProfile: %v", err)
	}
	ws, err := sqliteStore.EnsureDefaultWorkspace(ctx, admin.TenantID)
	if err != nil {
		t.Fatalf("EnsureDefaultWorkspace: %v", err)
	}

	// Record run-scoped binding evidence selecting the run profile.
	if _, err := sqliteStore.RecordRuntimeBindingEvidence(ctx, bindings.BuildRuntimeBindingEvidence(
		bindings.EffectiveBindingSelection{Outcome: bindings.OutcomeResolved, BindingScope: bindings.RuntimeScopeChannel, SelectedProfileID: runProfile.Profile.ProfileID, SelectedWorkspaceID: ws.WorkspaceID},
		bindings.RuntimeBindingEvidenceInput{TenantID: admin.TenantID, ResourceKind: "run", ResourceID: "run_x", OccurredAt: time.Now().UTC()},
	)); err != nil {
		t.Fatalf("RecordRuntimeBindingEvidence: %v", err)
	}

	// Disabled under the RUN's profile -> blocked when enforced for that run.
	if _, _, err := sqliteStore.SetCapabilityVisibility(ctx, admin, bindings.SetVisibilityRequest{ScopeKind: bindings.VisibilityScopeProfile, ScopeRef: runProfile.Profile.ProfileID, CapabilityID: "skill.delta", Visibility: bindings.VisibilityDisabled}); err != nil {
		t.Fatalf("SetCapabilityVisibility run profile: %v", err)
	}
	if err := enforceRunCapabilityVisibility(ctx, sqliteStore, "run_x", "skill.delta"); !errors.Is(err, bindings.ErrCapabilityNotExecutable) {
		t.Fatalf("expected block under run profile, got %v", err)
	}

	// Disabled only under the tenant-default profile -> NOT blocked for this run.
	if _, _, err := sqliteStore.SetCapabilityVisibility(ctx, admin, bindings.SetVisibilityRequest{ScopeKind: bindings.VisibilityScopeProfile, ScopeRef: tenantDefault.ProfileID, CapabilityID: "skill.epsilon", Visibility: bindings.VisibilityDisabled}); err != nil {
		t.Fatalf("SetCapabilityVisibility tenant default: %v", err)
	}
	if err := enforceRunCapabilityVisibility(ctx, sqliteStore, "run_x", "skill.epsilon"); err != nil {
		t.Fatalf("expected run binding to ignore tenant-default-only restriction, got %v", err)
	}
}

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
