package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleAgentProfileRoutes(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "profile store is not configured")
		return
	}
	if r.URL.Path == "/v1/profiles" {
		switch r.Method {
		case http.MethodGet:
			handleListAgentProfiles(sqliteStore, bus, w, r)
		case http.MethodPost:
			handleCreateAgentProfile(sqliteStore, bus, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/profiles/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	profileID := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			handleGetAgentProfile(sqliteStore, bus, profileID, w, r)
		case http.MethodPatch:
			handleUpdateAgentProfile(sqliteStore, bus, profileID, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch parts[1] {
	case "activate":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleActivateAgentProfile(sqliteStore, bus, profileID, w, r)
	case "versions":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleListAgentProfileVersions(sqliteStore, bus, profileID, w, r)
	case "rollback":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleRollbackAgentProfile(sqliteStore, bus, profileID, w, r)
	case "archive":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleRetireAgentProfile(sqliteStore, bus, profileID, profiles.StatusArchived, w, r)
	case "disable":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleRetireAgentProfile(sqliteStore, bus, profileID, profiles.StatusDisabled, w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func handleListAgentProfiles(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesInspect, "agent_profile.inspect_denied", "")
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := sqliteStore.ListAgentProfiles(r.Context(), tenantContext.TenantID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func handleCreateAgentProfile(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesManage, "agent_profile.create_denied", "")
	if !ok {
		return
	}
	var input profiles.MutationInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := sqliteStore.CreateAgentProfile(r.Context(), tenantContext, input)
	if err != nil {
		if publishErr := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: tenantContext.TenantID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "agent_profile.validation_failed", Outcome: "denied", ReasonCode: reasonCodeForProfileError(err), PermissionGate: string(identity.PermissionProfilesManage), RedactionStatus: profiles.RedactionRedacted}); publishErr != nil {
			writeError(w, http.StatusInternalServerError, publishErr.Error())
			return
		}
		writeProfileError(w, err)
		return
	}
	if err := publishProfileMutationEvents(r.Context(), sqliteStore, bus, result, tenantContext, "agent_profile.created", "succeeded", defaultAPIReason(input.ReasonCode, "user_created_profile")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func handleGetAgentProfile(sqliteStore *store.SQLiteStore, bus *events.Bus, profileID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesInspect, "agent_profile.inspect_denied", profileID)
	if !ok {
		return
	}
	detail, found, err := sqliteStore.GetAgentProfileDetail(r.Context(), tenantContext.TenantID, profileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "profile not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func handleUpdateAgentProfile(sqliteStore *store.SQLiteStore, bus *events.Bus, profileID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesManage, "agent_profile.update_denied", profileID)
	if !ok {
		return
	}
	var input profiles.MutationInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := sqliteStore.UpdateAgentProfile(r.Context(), tenantContext, profileID, input)
	if err != nil {
		if publishErr := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: tenantContext.TenantID, ProfileID: profileID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "agent_profile.validation_failed", Outcome: "denied", ReasonCode: reasonCodeForProfileError(err), PermissionGate: string(identity.PermissionProfilesManage), RedactionStatus: profiles.RedactionRedacted}); publishErr != nil {
			writeError(w, http.StatusInternalServerError, publishErr.Error())
			return
		}
		writeProfileError(w, err)
		return
	}
	if err := publishProfileMutationEvents(r.Context(), sqliteStore, bus, result, tenantContext, "agent_profile.updated", "succeeded", defaultAPIReason(input.ReasonCode, "user_updated_profile")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleActivateAgentProfile(sqliteStore *store.SQLiteStore, bus *events.Bus, profileID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesManage, "agent_profile.activate_denied", profileID)
	if !ok {
		return
	}
	var input profiles.ActivationInput
	if err := decodeJSONBody(r, &input); err != nil && !strings.Contains(err.Error(), "EOF") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	selection, err := sqliteStore.ActivateAgentProfile(r.Context(), tenantContext, profileID, input)
	if err != nil {
		if publishErr := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: tenantContext.TenantID, ProfileID: profileID, ProfileVersionID: input.ProfileVersionID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "agent_profile.activate_denied", Outcome: "denied", ReasonCode: reasonCodeForProfileError(err), PermissionGate: string(identity.PermissionProfilesManage), RedactionStatus: profiles.RedactionRedacted}); publishErr != nil {
			writeError(w, http.StatusInternalServerError, publishErr.Error())
			return
		}
		writeProfileError(w, err)
		return
	}
	if err := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: tenantContext.TenantID, ProfileID: selection.ProfileID, ProfileVersionID: selection.ProfileVersionID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "agent_profile.activated", Outcome: "succeeded", ReasonCode: defaultAPIReason(input.ReasonCode, "user_selected_default"), PermissionGate: string(identity.PermissionProfilesManage), AuditEventID: selection.AuditEventID, RedactionStatus: profiles.RedactionRedacted}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, selection)
}

func handleListAgentProfileVersions(sqliteStore *store.SQLiteStore, bus *events.Bus, profileID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesInspect, "agent_profile.inspect_denied", profileID)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := sqliteStore.ListAgentProfileVersions(r.Context(), tenantContext.TenantID, profileID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ListResponse[profiles.ProfileVersion]{Items: items})
}

func handleRollbackAgentProfile(sqliteStore *store.SQLiteStore, bus *events.Bus, profileID string, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesManage, "agent_profile.rollback_denied", profileID)
	if !ok {
		return
	}
	var input profiles.RollbackInput
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := sqliteStore.RollbackAgentProfile(r.Context(), tenantContext, profileID, input)
	if err != nil {
		if publishErr := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: tenantContext.TenantID, ProfileID: profileID, ProfileVersionID: input.SourceProfileVersionID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "agent_profile.rollback_denied", Outcome: "denied", ReasonCode: reasonCodeForProfileError(err), PermissionGate: string(identity.PermissionProfilesManage), RedactionStatus: profiles.RedactionRedacted}); publishErr != nil {
			writeError(w, http.StatusInternalServerError, publishErr.Error())
			return
		}
		writeProfileError(w, err)
		return
	}
	if err := publishProfileMutationEvents(r.Context(), sqliteStore, bus, result, tenantContext, "agent_profile.rolled_back", "succeeded", defaultAPIReason(input.ReasonCode, "operator_reverted_persona")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func handleRetireAgentProfile(sqliteStore *store.SQLiteStore, bus *events.Bus, profileID string, status profiles.Status, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := requireProfilePermission(sqliteStore, bus, w, r, identity.PermissionProfilesManage, "agent_profile.retirement_denied", profileID)
	if !ok {
		return
	}
	var input profiles.RetirementInput
	if err := decodeJSONBody(r, &input); err != nil && !strings.Contains(err.Error(), "EOF") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := sqliteStore.RetireAgentProfile(r.Context(), tenantContext, profileID, status, input)
	if err != nil {
		if publishErr := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: tenantContext.TenantID, ProfileID: profileID, ActorPrincipalID: tenantContext.PrincipalID, EventName: "agent_profile.retirement_denied", Outcome: "denied", ReasonCode: reasonCodeForProfileError(err), PermissionGate: string(identity.PermissionProfilesManage), RedactionStatus: profiles.RedactionRedacted}); publishErr != nil {
			writeError(w, http.StatusInternalServerError, publishErr.Error())
			return
		}
		writeProfileError(w, err)
		return
	}
	eventName := "agent_profile.archived"
	if status == profiles.StatusDisabled {
		eventName = "agent_profile.disabled"
	}
	if err := publishProfileMutationEvents(r.Context(), sqliteStore, bus, result, tenantContext, eventName, "succeeded", defaultAPIReason(input.ReasonCode, "operator_retired_profile")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.Selection.SelectionID != "" {
		if err := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: tenantContext.TenantID, ProfileID: result.Selection.ProfileID, ProfileVersionID: result.Selection.ProfileVersionID, ActorPrincipalID: "system", EventName: "agent_profile.safe_default_fallback", Outcome: "succeeded", ReasonCode: "current_default_retired", PermissionGate: string(identity.PermissionProfilesManage), AuditEventID: result.Selection.AuditEventID, RedactionStatus: profiles.RedactionRedacted}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func requireProfilePermission(sqliteStore *store.SQLiteStore, bus *events.Bus, w http.ResponseWriter, r *http.Request, permission identity.Permission, eventName, profileID string) (identity.TenantContext, bool) {
	tenantContext, err := RequirePermission(r.Context(), permission)
	if err != nil {
		if existing, ok := tenantContextFromContext(r.Context()); ok {
			if publishErr := publishProfileLifecycle(r.Context(), sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: existing.TenantID, ProfileID: profileID, ActorPrincipalID: existing.PrincipalID, EventName: eventName, Outcome: "denied", ReasonCode: "permission_denied", PermissionGate: string(permission), RedactionStatus: profiles.RedactionRedacted}); publishErr != nil {
				writeError(w, http.StatusInternalServerError, publishErr.Error())
				return identity.TenantContext{}, false
			}
		}
		writeTenantDenial(w, http.StatusForbidden)
		return identity.TenantContext{}, false
	}
	return tenantContext, true
}

func publishProfileMutationEvents(ctx context.Context, sqliteStore *store.SQLiteStore, bus *events.Bus, result profiles.MutationResult, actor identity.TenantContext, eventName, outcome, reasonCode string) error {
	if err := publishProfileLifecycle(ctx, sqliteStore, bus, events.AgentProfileLifecycleInput{TenantID: actor.TenantID, ProfileID: result.Profile.ProfileID, ProfileVersionID: result.Version.ProfileVersionID, ActorPrincipalID: actor.PrincipalID, EventName: eventName, Outcome: outcome, ReasonCode: reasonCode, PermissionGate: string(identity.PermissionProfilesManage), SafeSummary: profiles.SafeProfileSummary(result.Profile), AuditEventID: result.AuditEventID, RedactionStatus: profiles.RedactionRedacted}); err != nil {
		return err
	}
	if result.Version.ProfileVersionID != "" {
		if err := publishProfileEvent(ctx, sqliteStore, bus, events.AgentProfileVersionCreatedEvent(events.AgentProfileVersionInput{TenantID: actor.TenantID, ProfileID: result.Profile.ProfileID, ProfileVersionID: result.Version.ProfileVersionID, ChangeKind: result.Version.ChangeKind, VersionNumber: result.Version.VersionNumber, ReasonCode: reasonCode, RedactionStatus: profiles.RedactionRedacted})); err != nil {
			return err
		}
	}
	return nil
}

func publishProfileLifecycle(ctx context.Context, sqliteStore *store.SQLiteStore, bus *events.Bus, input events.AgentProfileLifecycleInput) error {
	return publishProfileEvent(ctx, sqliteStore, bus, events.AgentProfileLifecycleEvent(input))
}

func publishProfileEvent(ctx context.Context, sqliteStore *store.SQLiteStore, bus *events.Bus, event events.Event) error {
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

func writeProfileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrProfileNotFound):
		writeError(w, http.StatusNotFound, "profile not found")
	case errors.Is(err, profiles.ErrInvalidProfile), errors.Is(err, profiles.ErrScopedBindingDeferred):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, profiles.ErrProfileNotActivatable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, profiles.ErrExplicitActorRequired):
		writeTenantDenial(w, http.StatusForbidden)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func reasonCodeForProfileError(err error) string {
	switch {
	case errors.Is(err, profiles.ErrInvalidProfile):
		return profiles.ValidationReasonCode(err)
	case errors.Is(err, profiles.ErrScopedBindingDeferred):
		return "scoped_binding_deferred"
	case errors.Is(err, profiles.ErrProfileNotActivatable):
		return "profile_not_activatable"
	case errors.Is(err, store.ErrProfileNotFound):
		return "profile_not_found"
	case errors.Is(err, profiles.ErrExplicitActorRequired):
		return "explicit_actor_required"
	default:
		return "profile_operation_failed"
	}
}

func defaultAPIReason(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
