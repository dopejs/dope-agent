package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleTenants(identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleCreateTenant(identityManager, w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "store is not configured")
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	token, ok := authenticatedToken(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, auth.ErrAuthRequired.Error())
		return
	}
	allowedTenants, err := allowedTenantsForToken(r.Context(), sqliteStore, token, tenantContext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, TenantListResponse{Items: allowedTenants})
}

func handleTenantRoutes(identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if sqliteStore == nil {
		writeError(w, http.StatusInternalServerError, "store is not configured")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/tenants/"), "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	tenantID := parts[0]
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID != tenantID {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	if len(parts) >= 2 && parts[1] == "memberships" {
		handleTenantMembershipRoutes(identityManager, sqliteStore, w, r, tenantID, parts[2:])
		return
	}
	if len(parts) >= 2 && parts[1] == "invitations" {
		handleTenantInvitationCollection(identityManager, sqliteStore, w, r, tenantID, tenantContext)
		return
	}
	if len(parts) == 2 && parts[1] == "permissions" {
		handleTenantPermissions(w, r, tenantContext)
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenant, ok, err := sqliteStore.GetTenant(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	tenant.CallerMembershipRole = tenantContext.Role
	tenant.CallerMembershipStatus = identity.StatusActive
	tenant.CallerPermissions = append([]identity.Permission(nil), tenantContext.Permissions...)
	writeJSON(w, http.StatusOK, TenantDetailResponse{
		Tenant:        tenant,
		TenantContext: tenantContext,
	})
}

func handleCreateTenant(identityManager *identity.Manager, w http.ResponseWriter, r *http.Request) {
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionTenantManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	var request struct {
		DisplayName string `json:"displayName"`
		TenantKind  string `json:"tenantKind"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.TenantKind != "" && request.TenantKind != string(identity.TenantKindOrganization) {
		writeError(w, http.StatusBadRequest, identity.ErrTenantInvalid.Error())
		return
	}
	tenant, membership, err := identityManager.CreateOrganizationTenant(r.Context(), tenantContext, request.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tenant": tenant, "membership": membership})
}

func handleTenantMembershipRoutes(identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, tenantID string, parts []string) {
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionTenantManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	if len(parts) == 0 && r.Method == http.MethodGet {
		items, err := sqliteStore.ListMemberships(r.Context(), identity.MembershipFilter{TenantID: tenantID, Limit: 500})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[identity.Membership]{Items: items})
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var request struct {
			Role identity.Role `json:"role"`
		}
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		membership, err := identityManager.UpdateMembershipRole(r.Context(), tenantContext, tenantID, parts[0], request.Role)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"membership": membership})
	case http.MethodDelete:
		membership, err := identityManager.RemoveMembership(r.Context(), tenantContext, tenantID, parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"membership": membership})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleTenantInvitationCollection(identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, tenantID string, tenantContext identity.TenantContext) {
	switch r.Method {
	case http.MethodGet:
		items, err := sqliteStore.ListTenantInvitations(r.Context(), identity.InvitationFilter{TenantID: tenantID, Limit: 500})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[identity.TenantInvitation]{Items: items})
	case http.MethodPost:
		if _, err := RequirePermission(r.Context(), identity.PermissionTenantManage); err != nil {
			writeTenantDenial(w, http.StatusForbidden)
			return
		}
		var request struct {
			InvitedPrincipalID string        `json:"invitedPrincipalId"`
			Role               identity.Role `json:"role"`
		}
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		invitation, err := identityManager.CreateInvitation(r.Context(), tenantContext, identity.CreateInvitationInput{TenantID: tenantID, InvitedPrincipalID: request.InvitedPrincipalID, Role: request.Role})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"invitation": invitation})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleTenantInvitations(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	items, err := sqliteStore.ListTenantInvitations(r.Context(), identity.InvitationFilter{PrincipalID: tenantContext.PrincipalID, Limit: 500})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ListResponse[identity.TenantInvitation]{Items: items})
}

func handleTenantInvitationRoutes(identityManager *identity.Manager, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/tenant-invitations/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "accept":
		membership, err := identityManager.AcceptInvitation(r.Context(), tenantContext.PrincipalID, parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"membership": membership})
	case "reject":
		invitation, err := identityManager.DecideInvitation(r.Context(), tenantContext.PrincipalID, parts[0], identity.StatusRejected)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"invitation": invitation})
	default:
		http.NotFound(w, r)
	}
}

func handlePrincipals(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	filter := identity.PrincipalFilter{TenantID: tenantContext.TenantID, Limit: 500}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionTenantManage) {
		filter = identity.PrincipalFilter{Limit: 500}
	}
	items, err := sqliteStore.ListPrincipals(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !identity.HasPermission(tenantContext.Permissions, identity.PermissionTenantManage) {
		self := make([]identity.Principal, 0, 1)
		for _, item := range items {
			if item.PrincipalID == tenantContext.PrincipalID {
				self = append(self, item)
			}
		}
		items = self
	}
	writeJSON(w, http.StatusOK, ListResponse[identity.Principal]{Items: items})
}

func handlePrincipalRoutes(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionTenantManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	principalID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/principals/"), "/")
	principal, ok, err := sqliteStore.GetPrincipal(r.Context(), principalID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	var request struct {
		Status identity.LifecycleStatus `json:"status"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	principal.Status = request.Status
	if principal.Status == "" {
		principal.Status = identity.StatusActive
	}
	principal.UpdatedAt = tenantContext.ResolvedAt
	if err := sqliteStore.UpsertPrincipal(r.Context(), principal); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := sqliteStore.AppendTenantAuditEvent(r.Context(), identity.TenantAuditEvent{EventKind: "tenant.principal_lifecycle_updated", TenantID: tenantContext.TenantID, PrincipalID: tenantContext.PrincipalID, TargetPrincipalID: principal.PrincipalID, Outcome: identity.AuditOutcomeSucceeded, ReasonCode: "principal_lifecycle_updated"}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"principal": principal})
}

func handleTenantAuditEvents(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenantId"))
	if tenantID == "" {
		tenantID = tenantContext.TenantID
	}
	if tenantID != tenantContext.TenantID {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	items, err := sqliteStore.ListTenantAuditEvents(r.Context(), identity.AuditEventFilter{TenantID: tenantID, Limit: 500})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ListResponse[identity.TenantAuditEvent]{Items: items})
}

func handleTenantPermissions(w http.ResponseWriter, r *http.Request, tenantContext identity.TenantContext) {
	items := make([]identity.PermissionEvaluation, 0, len(identity.AllSensitivePermissions)+1)
	for _, permission := range append(identity.AllSensitivePermissions, identity.PermissionReadOnlyInspect) {
		items = append(items, identity.EvaluatePermission(tenantContext, permission))
	}
	writeJSON(w, http.StatusOK, ListResponse[identity.PermissionEvaluation]{Items: items})
}

func buildAuthMeResponse(ctx context.Context, sqliteStore *store.SQLiteStore, token auth.AccessToken, tenantContext identity.TenantContext) (AuthMeResponse, error) {
	principal, ok, err := sqliteStore.GetPrincipal(ctx, tenantContext.PrincipalID)
	if err != nil {
		return AuthMeResponse{}, err
	}
	if !ok {
		return AuthMeResponse{}, identity.ErrTenantAccessDenied
	}
	defaultTenant, ok, err := sqliteStore.GetTenant(ctx, principal.DefaultTenantID)
	if err != nil {
		return AuthMeResponse{}, err
	}
	if !ok {
		return AuthMeResponse{}, identity.ErrTenantAccessDenied
	}
	currentTenant, ok, err := sqliteStore.GetTenant(ctx, tenantContext.TenantID)
	if err != nil {
		return AuthMeResponse{}, err
	}
	if !ok {
		return AuthMeResponse{}, identity.ErrTenantAccessDenied
	}
	allowedTenants, err := allowedTenantsForToken(ctx, sqliteStore, token, tenantContext)
	if err != nil {
		return AuthMeResponse{}, err
	}
	tokenGrants, err := sqliteStore.ListTokenTenantGrants(ctx, token.TokenID)
	if err != nil {
		return AuthMeResponse{}, err
	}
	defaultTenant.DefaultForCurrentPrincipal = defaultTenant.TenantID == principal.DefaultTenantID
	defaultTenant.DefaultForCurrentToken = defaultTenant.TenantID == token.DefaultTenantID
	currentTenant.CallerMembershipRole = tenantContext.Role
	currentTenant.CallerMembershipStatus = identity.StatusActive
	currentTenant.CallerPermissions = append([]identity.Permission(nil), tenantContext.Permissions...)
	return AuthMeResponse{
		Token:          token,
		Principal:      principal,
		DefaultTenant:  defaultTenant,
		CurrentTenant:  currentTenant,
		AllowedTenants: allowedTenants,
		TokenGrants:    tokenGrants,
		Permissions:    append([]identity.Permission(nil), tenantContext.Permissions...),
		TenantContext:  tenantContext,
	}, nil
}

func allowedTenantsForToken(ctx context.Context, sqliteStore *store.SQLiteStore, token auth.AccessToken, tenantContext identity.TenantContext) ([]identity.Tenant, error) {
	grants, err := sqliteStore.ListTokenTenantGrants(ctx, token.TokenID)
	if err != nil {
		return nil, err
	}
	grantedTenantIDs := map[string]identity.TokenTenantGrant{}
	for _, grant := range grants {
		if grant.Status == identity.StatusActive {
			grantedTenantIDs[grant.TenantID] = grant
		}
	}
	memberships, err := sqliteStore.ListMemberships(ctx, identity.MembershipFilter{Status: identity.StatusActive, Limit: 500})
	if err != nil {
		return nil, err
	}
	items := make([]identity.Tenant, 0)
	for _, membership := range memberships {
		if membership.PrincipalID != tenantContext.PrincipalID || membership.Status != identity.StatusActive {
			continue
		}
		grant, ok := grantedTenantIDs[membership.TenantID]
		if !ok {
			continue
		}
		tenant, ok, err := sqliteStore.GetTenant(ctx, membership.TenantID)
		if err != nil {
			return nil, err
		}
		if !ok || tenant.Status != identity.StatusActive {
			continue
		}
		tenant.CallerMembershipRole = membership.Role
		tenant.CallerMembershipStatus = membership.Status
		tenant.CallerPermissions = identity.PermissionsForRole(membership.Role, membership.Status)
		tenant.DefaultForCurrentPrincipal = tenant.TenantID == tenantContext.TenantID && tenant.TenantID == token.DefaultTenantID
		tenant.DefaultForCurrentToken = grant.IsDefault || tenant.TenantID == token.DefaultTenantID
		items = append(items, tenant)
	}
	return items, nil
}
