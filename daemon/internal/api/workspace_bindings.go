package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// --- Workspaces ---

func handleWorkspaceRoutes(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "binding store is not configured")
		return
	}
	if r.URL.Path == "/v1/workspaces" {
		switch r.Method {
		case http.MethodGet:
			handleListWorkspaces(sqliteStore, w, r)
		case http.MethodPost:
			handleCreateWorkspace(sqliteStore, bus, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	workspaceID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/workspaces/"), "/")
	if workspaceID == "" || strings.Contains(workspaceID, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		handleGetWorkspace(sqliteStore, workspaceID, w, r)
	case http.MethodPatch:
		handleUpdateWorkspace(sqliteStore, bus, workspaceID, w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleListWorkspaces(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsInspect)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := sqliteStore.ListWorkspaces(r.Context(), tenantContext.TenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resources := make([]bindings.WorkspaceResource, 0, len(items))
	for _, ws := range items {
		resources = append(resources, bindings.ToWorkspaceResource(ws))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenantId": tenantContext.TenantID, "workspaces": resources})
}

func handleGetWorkspace(sqliteStore *store.SQLiteStore, workspaceID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsInspect)
	if !ok {
		return
	}
	ws, found, err := sqliteStore.GetWorkspace(r.Context(), tenantContext.TenantID, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeJSON(w, http.StatusOK, bindings.ToWorkspaceResource(ws))
}

func handleCreateWorkspace(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsManage)
	if !ok {
		return
	}
	var req bindings.CreateWorkspaceRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ws, auditID, err := sqliteStore.CreateWorkspace(r.Context(), tenantContext, req.DisplayName)
	if err != nil {
		writeBindingError(w, err)
		return
	}
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, WorkspaceID: ws.WorkspaceID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "workspace.created", Outcome: "succeeded", ReasonCode: "user_created_workspace", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Workspace created", AuditEventID: auditID}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, bindings.ToWorkspaceResource(ws))
}

func handleUpdateWorkspace(sqliteStore *store.SQLiteStore, bus *events.Bus, workspaceID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsManage)
	if !ok {
		return
	}
	var req bindings.UpdateWorkspaceRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ws, auditID, err := sqliteStore.UpdateWorkspaceStatus(r.Context(), tenantContext, workspaceID, req.Status)
	if err != nil {
		writeBindingError(w, err)
		return
	}
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, WorkspaceID: ws.WorkspaceID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "workspace." + string(ws.Status), Outcome: "succeeded", ReasonCode: "user_updated_workspace", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Workspace updated", AuditEventID: auditID}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bindings.ToWorkspaceResource(ws))
}

// --- Bindings ---

func handleBindingRoutes(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "binding store is not configured")
		return
	}
	if r.URL.Path == "/v1/bindings" {
		switch r.Method {
		case http.MethodGet:
			handleListBindings(sqliteStore, w, r)
		case http.MethodPost:
			handleCreateBinding(sqliteStore, bus, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/bindings/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	bindingID := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			handleGetBinding(sqliteStore, bindingID, w, r)
		case http.MethodPatch:
			handleUpdateBinding(sqliteStore, bus, bindingID, w, r)
		case http.MethodDelete:
			handleDeleteBinding(sqliteStore, bus, bindingID, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "repair" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleRepairBinding(sqliteStore, bus, bindingID, w, r)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

func handleListBindings(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsInspect)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := sqliteStore.ListBindingRules(r.Context(), tenantContext.TenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resources := make([]bindings.BindingResource, 0, len(items))
	for _, b := range items {
		resources = append(resources, bindings.ToBindingResource(b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenantId": tenantContext.TenantID, "bindings": resources})
}

func handleGetBinding(sqliteStore *store.SQLiteStore, bindingID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsInspect)
	if !ok {
		return
	}
	b, found, err := sqliteStore.GetBindingRule(r.Context(), tenantContext.TenantID, bindingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "binding not found")
		return
	}
	writeJSON(w, http.StatusOK, bindings.ToBindingResource(b))
}

func handleCreateBinding(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsManage)
	if !ok {
		return
	}
	var req bindings.CreateBindingRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rule, auditID, err := sqliteStore.CreateBindingRule(r.Context(), tenantContext, req)
	if err != nil {
		if publishErr := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "binding.validation_failed", Outcome: "denied", ReasonCode: reasonCodeForBindingError(err), PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Binding validation failed"}); publishErr != nil {
			writeError(w, http.StatusInternalServerError, publishErr.Error())
			return
		}
		writeBindingError(w, err)
		return
	}
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, BindingID: rule.BindingID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "binding.created", Outcome: "succeeded", ReasonCode: "user_created_binding", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Binding created", AuditEventID: auditID}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, bindings.ToBindingResource(rule))
}

func handleUpdateBinding(sqliteStore *store.SQLiteStore, bus *events.Bus, bindingID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsManage)
	if !ok {
		return
	}
	var req bindings.UpdateBindingRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rule, auditID, err := sqliteStore.UpdateBindingRule(r.Context(), tenantContext, bindingID, req)
	if err != nil {
		writeBindingError(w, err)
		return
	}
	eventName := "binding.updated"
	if rule.Status == bindings.BindingDisabled {
		eventName = "binding.disabled"
	}
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, BindingID: rule.BindingID, ActorPrincipalID: tenantContext.PrincipalID, EventName: eventName, Outcome: "succeeded", ReasonCode: "user_updated_binding", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Binding updated", AuditEventID: auditID}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bindings.ToBindingResource(rule))
}

func handleDeleteBinding(sqliteStore *store.SQLiteStore, bus *events.Bus, bindingID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsManage)
	if !ok {
		return
	}
	auditID, err := sqliteStore.RemoveBindingRule(r.Context(), tenantContext, bindingID)
	if err != nil {
		writeBindingError(w, err)
		return
	}
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, BindingID: bindingID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "binding.removed", Outcome: "succeeded", ReasonCode: "user_removed_binding", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Binding removed", AuditEventID: auditID}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleRepairBinding(sqliteStore *store.SQLiteStore, bus *events.Bus, bindingID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsManage)
	if !ok {
		return
	}
	rule, auditID, err := sqliteStore.RepairBindingRule(r.Context(), tenantContext, bindingID)
	if err != nil {
		writeBindingError(w, err)
		return
	}
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, BindingID: rule.BindingID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "binding.repaired", Outcome: "succeeded", ReasonCode: "user_repaired_binding", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Binding repair evaluated", AuditEventID: auditID}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bindings.ToBindingResource(rule))
}

// --- Capability visibility ---

func handleCapabilityVisibilityRoutes(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "binding store is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		handleListCapabilityVisibility(sqliteStore, w, r)
	case http.MethodPut:
		handleSetCapabilityVisibility(sqliteStore, bus, w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleListCapabilityVisibility(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsInspect)
	if !ok {
		return
	}
	scopeKind := bindings.VisibilityScopeKind(strings.TrimSpace(r.URL.Query().Get("scopeKind")))
	scopeRef := strings.TrimSpace(r.URL.Query().Get("scopeRef"))
	if scopeKind != bindings.VisibilityScopeProfile && scopeKind != bindings.VisibilityScopeWorkspace {
		writeError(w, http.StatusBadRequest, "scopeKind must be profile or workspace")
		return
	}
	items, err := sqliteStore.ListCapabilityVisibility(r.Context(), tenantContext.TenantID, scopeKind, scopeRef)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resources := make([]bindings.CapabilityVisibilityResource, 0, len(items))
	for _, p := range items {
		resources = append(resources, bindings.ToCapabilityVisibilityResource(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenantId": tenantContext.TenantID, "policies": resources})
}

func handleSetCapabilityVisibility(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireBindingPermission(sqliteStore, w, r, identity.PermissionBindingsManage)
	if !ok {
		return
	}
	var req bindings.SetVisibilityRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	policy, auditID, err := sqliteStore.SetCapabilityVisibility(r.Context(), tenantContext, req)
	if err != nil {
		writeBindingError(w, err)
		return
	}
	if err := publishBindingEvent(r.Context(), sqliteStore, bus, events.CapabilityVisibilityChangedEvent(events.CapabilityVisibilityChangedInput{TenantID: tenantContext.TenantID, ActorPrincipalID: tenantContext.PrincipalID, ScopeKind: policy.ScopeKind, ScopeRef: policy.ScopeRef, CapabilityID: policy.CapabilityID, Visibility: policy.Visibility, AuditEventID: auditID})); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bindings.ToCapabilityVisibilityResource(policy))
}

// --- helpers ---

func requireBindingPermission(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, permission identity.Permission) (identity.TenantContext, bool) {
	tenantContext, err := RequirePermission(r.Context(), permission)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return identity.TenantContext{}, false
	}
	return tenantContext, true
}

func publishBindingLifecycle(ctx context.Context, sqliteStore *store.SQLiteStore, bus *events.Bus, input events.BindingLifecycleInput) error {
	return publishBindingEvent(ctx, sqliteStore, bus, events.BindingLifecycleEvent(input))
}

func publishBindingEvent(ctx context.Context, sqliteStore *store.SQLiteStore, bus *events.Bus, event events.Event) error {
	if bus == nil {
		if sqliteStore == nil {
			return nil
		}
		prepared := ensureEventDefaults(event)
		if prepared.EnvironmentScope == "" {
			prepared.EnvironmentScope = events.EnvironmentScopeFromContext(ctx)
		}
		if prepared.TenantID != "" {
			_, err := sqliteStore.AppendEventForTenantRaw(ctx, prepared, prepared.TenantID)
			return err
		}
		_, err := sqliteStore.AppendEvent(ctx, prepared)
		return err
	}
	_, err := publishEvent(ctx, bus, sqliteStore, event)
	return err
}

func writeBindingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrBindingNotFound):
		writeError(w, http.StatusNotFound, "binding not found")
	case errors.Is(err, store.ErrWorkspaceNotFound):
		writeError(w, http.StatusNotFound, "workspace not found")
	case errors.Is(err, bindings.ErrInvalidBinding):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, bindings.ErrExplicitActorRequired):
		writeTenantDenial(w, http.StatusForbidden)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func reasonCodeForBindingError(err error) string {
	if errors.Is(err, bindings.ErrInvalidBinding) {
		return bindings.ValidationReasonCode(err)
	}
	if errors.Is(err, store.ErrBindingNotFound) {
		return "binding_not_found"
	}
	if errors.Is(err, bindings.ErrExplicitActorRequired) {
		return "explicit_actor_required"
	}
	return "binding_operation_failed"
}
