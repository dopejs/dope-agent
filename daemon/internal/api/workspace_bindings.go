package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// writeJSONWithBindingProjection writes response and, for callers holding bindings.inspect,
// attaches the latest runtime binding evidence for (resourceKind, resourceID) as an additive
// `bindingProjection` field without altering the base response type (FR-013, SC-008, SC-012).
func writeJSONWithBindingProjection(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, resourceKind, resourceID string, response any) {
	if sqliteStore == nil {
		writeJSON(w, http.StatusOK, response)
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || !identity.HasPermission(tenantContext.Permissions, identity.PermissionBindingsInspect) {
		writeJSON(w, http.StatusOK, response)
		return
	}
	evidence, found, err := sqliteStore.LatestRuntimeBindingEvidence(r.Context(), tenantContext.TenantID, resourceKind, resourceID)
	if err != nil || !found {
		writeJSON(w, http.StatusOK, response)
		return
	}
	raw, err := json.Marshal(response)
	if err != nil {
		writeJSON(w, http.StatusOK, response)
		return
	}
	merged := map[string]any{}
	if err := json.Unmarshal(raw, &merged); err != nil {
		writeJSON(w, http.StatusOK, response)
		return
	}
	merged["bindingProjection"] = bindings.ToRuntimeEvidenceResource(evidence)
	writeJSON(w, http.StatusOK, merged)
}

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
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, BindingID: rule.BindingID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "binding.created", Outcome: "succeeded", ReasonCode: "user_created_binding", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Binding created", ResultingSelectionSummary: rule.ResultingSelectionSummary, AuditEventID: auditID}); err != nil {
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
	if err := publishBindingLifecycle(r.Context(), sqliteStore, bus, events.BindingLifecycleInput{TenantID: tenantContext.TenantID, BindingID: rule.BindingID, ActorPrincipalID: tenantContext.PrincipalID, EventName: eventName, Outcome: "succeeded", ReasonCode: "user_updated_binding", PermissionGate: string(identity.PermissionBindingsManage), SafeSummary: "Binding updated", PreviousSelectionSummary: rule.PreviousSelectionSummary, ResultingSelectionSummary: rule.ResultingSelectionSummary, AuditEventID: auditID}); err != nil {
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

// enforceRunCapabilityVisibility blocks execution of a hidden or disabled capability/skill
// requested through the runtime tool-call API (FR-016: hidden/disabled capabilities MUST NOT
// execute even when requested directly by a user, agent, client integration, connector
// payload, or replay).
//
// When the run has recorded binding evidence, enforcement uses that run's resolved profile +
// workspace (the binding that actually applied to the work, including any channel/account
// binding), so the execution gate matches the work-start decision. Otherwise (no run-scoped
// evidence, e.g. a direct API tool-call with no chat work-start) it falls back to the tenant
// default: the active profile plus the default workspace. Resolution failures fail closed
// (block), matching the chat enforcement and the spec's fail-closed posture. An empty tenant
// or absent active profile means no binding policy is in force, so execution is allowed.
func enforceRunCapabilityVisibility(ctx context.Context, sqliteStore *store.SQLiteStore, runID, capabilityID string) error {
	capabilityID = strings.TrimSpace(capabilityID)
	if sqliteStore == nil || capabilityID == "" {
		return nil
	}
	tenantContext, ok := tenantContextFromContext(ctx)
	if !ok || strings.TrimSpace(tenantContext.TenantID) == "" {
		return nil
	}
	tenantID := strings.TrimSpace(tenantContext.TenantID)

	var profileID, workspaceID string
	if runID = strings.TrimSpace(runID); runID != "" {
		ev, found, err := sqliteStore.LatestRuntimeBindingEvidence(ctx, tenantID, "run", runID)
		if err != nil {
			return err
		}
		if found {
			profileID, workspaceID = ev.SelectedProfileID, ev.SelectedWorkspaceID
		}
	}
	if profileID == "" && workspaceID == "" {
		profile, _, found, err := sqliteStore.ActiveAgentProfileSelection(ctx, tenantID)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		ws, err := sqliteStore.EnsureDefaultWorkspace(ctx, tenantID)
		if err != nil {
			return err
		}
		profileID, workspaceID = profile.ProfileID, ws.WorkspaceID
	}

	decision, err := sqliteStore.EffectiveCapabilityVisibility(ctx, tenantID, profileID, workspaceID, capabilityID, nil)
	if err != nil {
		return err
	}
	if err := bindings.EnforceExecutable(decision); err != nil {
		return fmt.Errorf("%w: %s", err, capabilityID)
	}
	return nil
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
