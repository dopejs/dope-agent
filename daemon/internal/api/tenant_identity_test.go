package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestTenantPermissionsRouteReflectsRoleDerivedPermissions(t *testing.T) {
	harness := newTenantAuthHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+harness.defaultTenant.TenantID+"/permissions", nil)
	req.Header.Set("Authorization", harness.authHeader)
	rec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for tenant permissions, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response ListResponse[identity.PermissionEvaluation]
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode permission response: %v", err)
	}
	if len(response.Items) == 0 {
		t.Fatal("expected permission evaluations")
	}
	seen := map[identity.Permission]bool{}
	for _, item := range response.Items {
		if item.Allowed {
			seen[item.Permission] = true
		}
	}
	for _, permission := range identity.AllSensitivePermissions {
		if !seen[permission] {
			t.Fatalf("expected owner to have %s in %+v", permission, response.Items)
		}
	}
}

func TestTenantManagementRoutesCoverMembershipInvitationAndAudit(t *testing.T) {
	harness := newTenantAuthHarness(t)
	invitedHeader, invitedPrincipal := harness.issuePrincipalToken(t, "prn_invited_api", "Invited API")

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"displayName":"Acme","tenantKind":"organization"}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for tenant create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Tenant identity.Tenant `json:"tenant"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create tenant: %v", err)
	}
	if created.Tenant.TenantKind != identity.TenantKindOrganization {
		t.Fatalf("expected organization tenant, got %+v", created.Tenant)
	}

	inviteRec := httptest.NewRecorder()
	inviteReq := httptest.NewRequest(http.MethodPost, "/v1/tenants/"+created.Tenant.TenantID+"/invitations", strings.NewReader(`{"invitedPrincipalId":"`+invitedPrincipal.PrincipalID+`","role":"operator"}`))
	inviteReq.Header.Set("Authorization", harness.authHeader)
	inviteReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(inviteRec, inviteReq)
	if inviteRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for invitation create, got %d body=%s", inviteRec.Code, inviteRec.Body.String())
	}
	var invitationResponse struct {
		Invitation identity.TenantInvitation `json:"invitation"`
	}
	if err := json.Unmarshal(inviteRec.Body.Bytes(), &invitationResponse); err != nil {
		t.Fatalf("decode invitation: %v", err)
	}

	acceptRec := httptest.NewRecorder()
	acceptReq := httptest.NewRequest(http.MethodPost, "/v1/tenant-invitations/"+invitationResponse.Invitation.InvitationID+"/accept", nil)
	acceptReq.Header.Set("Authorization", invitedHeader)
	harness.server.Handler().ServeHTTP(acceptRec, acceptReq)
	if acceptRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for invitation accept, got %d body=%s", acceptRec.Code, acceptRec.Body.String())
	}
	var acceptResponse struct {
		Membership identity.Membership `json:"membership"`
	}
	if err := json.Unmarshal(acceptRec.Body.Bytes(), &acceptResponse); err != nil {
		t.Fatalf("decode accept response: %v", err)
	}
	if acceptResponse.Membership.Role != identity.RoleOperator {
		t.Fatalf("expected operator membership, got %+v", acceptResponse.Membership)
	}

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPatch, "/v1/tenants/"+created.Tenant.TenantID+"/memberships/"+acceptResponse.Membership.MembershipID, strings.NewReader(`{"role":"viewer"}`))
	updateReq.Header.Set("Authorization", harness.authHeader)
	updateReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for membership update, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	removeRec := httptest.NewRecorder()
	removeReq := httptest.NewRequest(http.MethodDelete, "/v1/tenants/"+created.Tenant.TenantID+"/memberships/"+acceptResponse.Membership.MembershipID, nil)
	removeReq.Header.Set("Authorization", harness.authHeader)
	removeReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(removeRec, removeReq)
	if removeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for membership remove, got %d body=%s", removeRec.Code, removeRec.Body.String())
	}

	principalRec := httptest.NewRecorder()
	principalReq := httptest.NewRequest(http.MethodPatch, "/v1/principals/"+invitedPrincipal.PrincipalID, strings.NewReader(`{"status":"disabled"}`))
	principalReq.Header.Set("Authorization", harness.authHeader)
	principalReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(principalRec, principalReq)
	if principalRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for principal update, got %d body=%s", principalRec.Code, principalRec.Body.String())
	}

	auditRec := httptest.NewRecorder()
	auditReq := httptest.NewRequest(http.MethodGet, "/v1/tenant-audit-events?tenantId="+created.Tenant.TenantID, nil)
	auditReq.Header.Set("Authorization", harness.authHeader)
	auditReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(auditRec, auditReq)
	if auditRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for audit list, got %d body=%s", auditRec.Code, auditRec.Body.String())
	}
}

func TestMembershipRoleUpdateLeavesAuditVisibleRoleChangeState(t *testing.T) {
	harness := newTenantAuthHarness(t)
	_, memberPrincipal := harness.issuePrincipalToken(t, "prn_role_member", "Role Member")

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"displayName":"Audit Org","tenantKind":"organization"}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for tenant create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Tenant identity.Tenant `json:"tenant"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode tenant create: %v", err)
	}

	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	member := identity.Membership{
		MembershipID: "mem_role_member",
		TenantID:     created.Tenant.TenantID,
		PrincipalID:  memberPrincipal.PrincipalID,
		Role:         identity.RoleOperator,
		Status:       identity.StatusActive,
		CreatedAt:    created.Tenant.CreatedAt,
		UpdatedAt:    created.Tenant.UpdatedAt,
	}
	if err := harness.store.UpsertMembership(ctx, member); err != nil {
		t.Fatalf("UpsertMembership returned error: %v", err)
	}

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPatch, "/v1/tenants/"+created.Tenant.TenantID+"/memberships/"+member.MembershipID, strings.NewReader(`{"role":"admin"}`))
	updateReq.Header.Set("Authorization", harness.authHeader)
	updateReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for membership update, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	audits, err := harness.store.ListTenantAuditEvents(ctx, identity.AuditEventFilter{TenantID: created.Tenant.TenantID, EventKind: "tenant.membership_role_updated", Limit: 10})
	if err != nil {
		t.Fatalf("ListTenantAuditEvents returned error: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one role update audit, got %+v", audits)
	}
	audit := audits[0]
	if audit.PrincipalID != harness.principal.PrincipalID || audit.TargetPrincipalID != memberPrincipal.PrincipalID || audit.TenantID != created.Tenant.TenantID {
		t.Fatalf("audit actor/tenant/target mismatch: %+v", audit)
	}
	if audit.CreatedAt.IsZero() {
		t.Fatalf("expected audit timestamp: %+v", audit)
	}
	if audit.Document["membershipId"] != member.MembershipID || audit.Document["oldRole"] != string(identity.RoleOperator) || audit.Document["newRole"] != string(identity.RoleAdmin) {
		t.Fatalf("expected audit document with membership and old/new role, got %+v", audit.Document)
	}
}

func TestMembershipRoleUpdateAndRemovalPreventLastOwnerLoss(t *testing.T) {
	harness := newTenantAuthHarness(t)

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"displayName":"Owner Guard Org","tenantKind":"organization"}`))
	createReq.Header.Set("Authorization", harness.authHeader)
	harness.server.Handler().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for tenant create, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		Tenant     identity.Tenant     `json:"tenant"`
		Membership identity.Membership `json:"membership"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode tenant create: %v", err)
	}

	downgradeRec := httptest.NewRecorder()
	downgradeReq := httptest.NewRequest(http.MethodPatch, "/v1/tenants/"+created.Tenant.TenantID+"/memberships/"+created.Membership.MembershipID, strings.NewReader(`{"role":"viewer"}`))
	downgradeReq.Header.Set("Authorization", harness.authHeader)
	downgradeReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(downgradeRec, downgradeReq)
	if downgradeRec.Code != http.StatusBadRequest || !strings.Contains(downgradeRec.Body.String(), identity.ErrOwnerInvariant.Error()) {
		t.Fatalf("expected last-owner downgrade denial, got %d body=%s", downgradeRec.Code, downgradeRec.Body.String())
	}

	removeRec := httptest.NewRecorder()
	removeReq := httptest.NewRequest(http.MethodDelete, "/v1/tenants/"+created.Tenant.TenantID+"/memberships/"+created.Membership.MembershipID, nil)
	removeReq.Header.Set("Authorization", harness.authHeader)
	removeReq.Header.Set("X-Dope-Tenant-ID", created.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(removeRec, removeReq)
	if removeRec.Code != http.StatusBadRequest || !strings.Contains(removeRec.Body.String(), identity.ErrOwnerInvariant.Error()) {
		t.Fatalf("expected last-owner removal denial, got %d body=%s", removeRec.Code, removeRec.Body.String())
	}

	memberships, err := harness.store.ListMemberships(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.MembershipFilter{TenantID: created.Tenant.TenantID, Status: identity.StatusActive, Role: identity.RoleOwner, Limit: 10})
	if err != nil {
		t.Fatalf("ListMemberships returned error: %v", err)
	}
	if len(memberships) != 1 || memberships[0].MembershipID != created.Membership.MembershipID {
		t.Fatalf("expected last owner to remain active, got %+v", memberships)
	}
}

func TestSensitiveTenantManagementPermissionOutcomes(t *testing.T) {
	harness := newTenantAuthHarness(t)

	viewerHeader, viewer := harness.issuePrincipalToken(t, "prn_viewer_api", "Viewer API")
	operatorHeader, operator := harness.issuePrincipalToken(t, "prn_operator_api", "Operator API")
	adminHeader, admin := harness.issuePrincipalToken(t, "prn_admin_api", "Admin API")
	disabledHeader, disabled := harness.issuePrincipalToken(t, "prn_disabled_api", "Disabled API")
	removedHeader, removed := harness.issuePrincipalToken(t, "prn_removed_api", "Removed API")
	revokedHeader, revoked := harness.issuePrincipalToken(t, "prn_revoked_api", "Revoked API")

	harness.setDefaultMembershipRole(t, operator.PrincipalID, identity.RoleOperator, identity.StatusActive)
	harness.setDefaultMembershipRole(t, admin.PrincipalID, identity.RoleAdmin, identity.StatusActive)
	harness.setDefaultMembershipRole(t, removed.PrincipalID, identity.RoleOwner, identity.StatusRemoved)
	disabled.Status = identity.StatusDisabled
	if err := harness.store.UpsertPrincipal(httptest.NewRequest(http.MethodGet, "/", nil).Context(), disabled); err != nil {
		t.Fatalf("UpsertPrincipal disabled returned error: %v", err)
	}
	harness.setTokenStatus(t, revoked.PrincipalID, string(identity.StatusRevoked))

	for _, tt := range []struct {
		name       string
		header     string
		wantStatus int
	}{
		{name: "owner", header: harness.authHeader, wantStatus: http.StatusCreated},
		{name: "admin", header: adminHeader, wantStatus: http.StatusCreated},
		{name: "operator", header: operatorHeader, wantStatus: http.StatusForbidden},
		{name: "viewer", header: viewerHeader, wantStatus: http.StatusForbidden},
		{name: "disabled principal", header: disabledHeader, wantStatus: http.StatusForbidden},
		{name: "removed membership", header: removedHeader, wantStatus: http.StatusForbidden},
		{name: "revoked token", header: revokedHeader, wantStatus: http.StatusUnauthorized},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"displayName":"`+tt.name+` org","tenantKind":"organization"}`))
		req.Header.Set("Authorization", tt.header)
		harness.server.Handler().ServeHTTP(rec, req)
		if rec.Code != tt.wantStatus {
			t.Fatalf("%s: expected %d, got %d body=%s", tt.name, tt.wantStatus, rec.Code, rec.Body.String())
		}
	}

	audits, err := harness.store.ListTenantAuditEvents(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.AuditEventFilter{TenantID: harness.defaultTenant.TenantID, PrincipalID: viewer.PrincipalID, Outcome: identity.AuditOutcomeDenied})
	if err != nil {
		t.Fatalf("ListTenantAuditEvents returned error: %v", err)
	}
	if len(audits) == 0 || !strings.Contains(audits[0].ReasonCode, string(identity.PermissionTenantManage)) {
		t.Fatalf("expected tenant.manage denial audit for viewer, got %+v", audits)
	}
}

func TestTenantListHandlesLowHundredsAllowedTenants(t *testing.T) {
	harness := newTenantAuthHarness(t)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	for idx := 0; idx < 220; idx++ {
		tenantID := "ten_bulk_" + strconv.Itoa(idx)
		tenant := identity.Tenant{
			TenantID:                tenantID,
			TenantKind:              identity.TenantKindOrganization,
			DisplayName:             "Bulk " + strconv.Itoa(idx),
			Status:                  identity.StatusActive,
			CreatedAt:               harness.principal.CreatedAt,
			UpdatedAt:               harness.principal.UpdatedAt,
			CreatedByPrincipalID:    harness.principal.PrincipalID,
			DefaultOwnerPrincipalID: harness.principal.PrincipalID,
		}
		if err := harness.store.UpsertTenant(ctx, tenant); err != nil {
			t.Fatalf("UpsertTenant returned error: %v", err)
		}
		if err := harness.store.UpsertMembership(ctx, identity.Membership{MembershipID: "mem_" + tenantID, TenantID: tenantID, PrincipalID: harness.principal.PrincipalID, Role: identity.RoleViewer, Status: identity.StatusActive, CreatedAt: harness.principal.CreatedAt, UpdatedAt: harness.principal.UpdatedAt}); err != nil {
			t.Fatalf("UpsertMembership returned error: %v", err)
		}
		if err := harness.store.UpsertTokenTenantGrant(ctx, identity.TokenTenantGrant{GrantID: "grant_" + tenantID, TokenID: harness.token.TokenID, TenantID: tenantID, Status: identity.StatusActive, CreatedAt: harness.principal.CreatedAt, UpdatedAt: harness.principal.UpdatedAt, GrantedByPrincipalID: harness.principal.PrincipalID}); err != nil {
			t.Fatalf("UpsertTokenTenantGrant returned error: %v", err)
		}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tenants", nil)
	req.Header.Set("Authorization", harness.authHeader)
	harness.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected tenant list 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response TenantListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode tenant list: %v", err)
	}
	if len(response.Items) != 221 {
		t.Fatalf("expected 221 allowed tenants including default, got %d", len(response.Items))
	}
}

func (h tenantAuthHarness) issuePrincipalToken(t *testing.T, principalID, displayName string) (string, identity.Principal) {
	t.Helper()
	principal := identity.Principal{
		PrincipalID:     principalID,
		PrincipalKind:   identity.PrincipalKindUser,
		DisplayName:     displayName,
		Status:          identity.StatusActive,
		DefaultTenantID: h.defaultTenant.TenantID,
		CreatedAt:       h.principal.CreatedAt,
		UpdatedAt:       h.principal.UpdatedAt,
	}
	if err := h.store.UpsertPrincipal(httptest.NewRequest(http.MethodGet, "/", nil).Context(), principal); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	pairing, code, err := h.authManager.StartPairing(auth.StartPairingInput{Mode: auth.PairingModeLocal, Label: displayName})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	_, token, secret, err := h.authManager.CompletePairing(pairing.PairingID, auth.CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}
	token.PrincipalID = principal.PrincipalID
	token.DefaultTenantID = h.defaultTenant.TenantID
	token.Status = string(identity.StatusActive)
	h.authManager.UpdateToken(token)
	if err := h.store.UpsertAccessToken(httptest.NewRequest(http.MethodGet, "/", nil).Context(), token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}
	if err := h.store.UpsertMembership(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.Membership{
		MembershipID: "mem_" + principalID,
		TenantID:     h.defaultTenant.TenantID,
		PrincipalID:  principal.PrincipalID,
		Role:         identity.RoleViewer,
		Status:       identity.StatusActive,
		CreatedAt:    h.principal.CreatedAt,
		UpdatedAt:    h.principal.UpdatedAt,
	}); err != nil {
		t.Fatalf("UpsertMembership returned error: %v", err)
	}
	if err := h.store.UpsertTokenTenantGrant(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.TokenTenantGrant{
		GrantID:              "grant_" + principalID,
		TokenID:              token.TokenID,
		TenantID:             h.defaultTenant.TenantID,
		IsDefault:            true,
		Status:               identity.StatusActive,
		CreatedAt:            h.principal.CreatedAt,
		UpdatedAt:            h.principal.UpdatedAt,
		GrantedByPrincipalID: h.principal.PrincipalID,
	}); err != nil {
		t.Fatalf("UpsertTokenTenantGrant returned error: %v", err)
	}
	return "Bearer " + secret, principal
}

func (h tenantAuthHarness) setDefaultMembershipRole(t *testing.T, principalID string, role identity.Role, status identity.LifecycleStatus) {
	t.Helper()
	membership := identity.Membership{
		MembershipID: "mem_" + principalID,
		TenantID:     h.defaultTenant.TenantID,
		PrincipalID:  principalID,
		Role:         role,
		Status:       status,
		CreatedAt:    h.principal.CreatedAt,
		UpdatedAt:    h.principal.UpdatedAt,
	}
	if err := h.store.UpsertMembership(httptest.NewRequest(http.MethodGet, "/", nil).Context(), membership); err != nil {
		t.Fatalf("UpsertMembership returned error: %v", err)
	}
}

func (h tenantAuthHarness) setTokenStatus(t *testing.T, principalID string, status string) {
	t.Helper()
	for _, token := range h.authManager.ListTokens() {
		if token.PrincipalID != principalID {
			continue
		}
		token.Status = status
		h.authManager.UpdateToken(token)
		if err := h.store.UpsertAccessToken(httptest.NewRequest(http.MethodGet, "/", nil).Context(), token); err != nil {
			t.Fatalf("UpsertAccessToken returned error: %v", err)
		}
		return
	}
	t.Fatalf("token for principal %s not found", principalID)
}
