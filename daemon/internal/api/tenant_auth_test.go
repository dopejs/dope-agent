package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func TestAuthMeIncludesResolvedTenantContext(t *testing.T) {
	harness := newTenantAuthHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", harness.authHeader)
	rec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for auth me, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response AuthMeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode auth me response: %v", err)
	}
	if response.Token.TokenID != harness.token.TokenID {
		t.Fatalf("expected token %s, got %+v", harness.token.TokenID, response.Token)
	}
	if response.Principal.PrincipalID != harness.principal.PrincipalID {
		t.Fatalf("expected principal %s, got %+v", harness.principal.PrincipalID, response.Principal)
	}
	if response.DefaultTenant.TenantID != harness.defaultTenant.TenantID {
		t.Fatalf("expected default tenant %s, got %+v", harness.defaultTenant.TenantID, response.DefaultTenant)
	}
	if response.CurrentTenant.TenantID != harness.defaultTenant.TenantID || response.TenantContext.TenantID != harness.defaultTenant.TenantID {
		t.Fatalf("expected current/default tenant context, got %+v", response)
	}
	if len(response.AllowedTenants) != 1 || response.AllowedTenants[0].TenantID != harness.defaultTenant.TenantID {
		t.Fatalf("expected one allowed tenant, got %+v", response.AllowedTenants)
	}
	if len(response.TokenGrants) != 1 || response.TokenGrants[0].TenantID != harness.defaultTenant.TenantID {
		t.Fatalf("expected one token grant, got %+v", response.TokenGrants)
	}
	if !identity.HasPermission(response.Permissions, identity.PermissionTenantManage) {
		t.Fatalf("expected owner permissions in auth me, got %+v", response.Permissions)
	}
}

func TestTenantInspectionHonorsExplicitTenantSelectionAndStableDenial(t *testing.T) {
	harness := newTenantAuthHarness(t)

	allowedReq := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+harness.defaultTenant.TenantID, nil)
	allowedReq.Header.Set("Authorization", harness.authHeader)
	allowedReq.Header.Set("X-Dope-Tenant-ID", harness.defaultTenant.TenantID)
	allowedRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(allowedRec, allowedReq)
	if allowedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for allowed tenant, got %d body=%s", allowedRec.Code, allowedRec.Body.String())
	}
	var detail TenantDetailResponse
	if err := json.Unmarshal(allowedRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("failed to decode tenant detail: %v", err)
	}
	if detail.Tenant.TenantID != harness.defaultTenant.TenantID || detail.TenantContext.TenantSource != identity.TenantSourceExplicitHeader {
		t.Fatalf("expected explicit tenant detail, got %+v", detail)
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/v1/tenants/"+harness.otherTenant.TenantID, nil)
	deniedReq.Header.Set("Authorization", harness.authHeader)
	deniedReq.Header.Set("X-Dope-Tenant-ID", harness.otherTenant.TenantID)
	deniedRec := httptest.NewRecorder()
	harness.server.Handler().ServeHTTP(deniedRec, deniedReq)
	if deniedRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed tenant, got %d body=%s", deniedRec.Code, deniedRec.Body.String())
	}
	var denial identity.Denial
	if err := json.Unmarshal(deniedRec.Body.Bytes(), &denial); err != nil {
		t.Fatalf("failed to decode denial: %v", err)
	}
	if denial.ErrorCode != "tenant_access_denied" || denial.Error != "tenant access denied" {
		t.Fatalf("expected stable tenant denial, got %+v", denial)
	}
}

func TestAuthTokenLifecycleRoutesCoverIssueGrantRotateAndRevoke(t *testing.T) {
	harness := newTenantAuthHarness(t)

	createTenantRec := httptest.NewRecorder()
	createTenantReq := httptest.NewRequest(http.MethodPost, "/v1/tenants", strings.NewReader(`{"displayName":"Token Org","tenantKind":"organization"}`))
	createTenantReq.Header.Set("Authorization", harness.authHeader)
	harness.server.Handler().ServeHTTP(createTenantRec, createTenantReq)
	if createTenantRec.Code != http.StatusCreated {
		t.Fatalf("expected organization create 201, got %d body=%s", createTenantRec.Code, createTenantRec.Body.String())
	}
	var createdTenant struct {
		Tenant identity.Tenant `json:"tenant"`
	}
	if err := json.Unmarshal(createTenantRec.Body.Bytes(), &createdTenant); err != nil {
		t.Fatalf("decode tenant create: %v", err)
	}

	issueRec := httptest.NewRecorder()
	issueReq := httptest.NewRequest(http.MethodPost, "/v1/auth/tokens", strings.NewReader(`{"label":"automation","defaultTenantId":"`+harness.defaultTenant.TenantID+`","allowedTenantIds":["`+harness.defaultTenant.TenantID+`"]}`))
	issueReq.Header.Set("Authorization", harness.authHeader)
	harness.server.Handler().ServeHTTP(issueRec, issueReq)
	if issueRec.Code != http.StatusCreated {
		t.Fatalf("expected token issue 201, got %d body=%s", issueRec.Code, issueRec.Body.String())
	}
	var issued struct {
		Token       auth.AccessToken            `json:"token"`
		AccessToken string                      `json:"accessToken"`
		Grants      []identity.TokenTenantGrant `json:"grants"`
	}
	if err := json.Unmarshal(issueRec.Body.Bytes(), &issued); err != nil {
		t.Fatalf("decode issue token: %v", err)
	}
	if issued.AccessToken == "" || len(issued.Grants) != 1 || issued.Grants[0].TenantID != harness.defaultTenant.TenantID {
		t.Fatalf("unexpected issue response: %+v", issued)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/auth/tokens", nil)
	listReq.Header.Set("Authorization", harness.authHeader)
	harness.server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected token list 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}

	grantsRec := httptest.NewRecorder()
	grantsReq := httptest.NewRequest(http.MethodPatch, "/v1/auth/tokens/"+issued.Token.TokenID+"/tenant-grants", strings.NewReader(`{"defaultTenantId":"`+createdTenant.Tenant.TenantID+`","allowedTenantIds":["`+createdTenant.Tenant.TenantID+`"]}`))
	grantsReq.Header.Set("Authorization", harness.authHeader)
	grantsReq.Header.Set("X-Dope-Tenant-ID", createdTenant.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(grantsRec, grantsReq)
	if grantsRec.Code != http.StatusOK {
		t.Fatalf("expected grant update 200, got %d body=%s", grantsRec.Code, grantsRec.Body.String())
	}

	rotateRec := httptest.NewRecorder()
	rotateReq := httptest.NewRequest(http.MethodPost, "/v1/auth/tokens/"+issued.Token.TokenID+"/rotate", strings.NewReader(`{"reason":"scheduled"}`))
	rotateReq.Header.Set("Authorization", harness.authHeader)
	rotateReq.Header.Set("X-Dope-Tenant-ID", createdTenant.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("expected token rotate 200, got %d body=%s", rotateRec.Code, rotateRec.Body.String())
	}
	var rotated struct {
		OldToken    auth.AccessToken `json:"oldToken"`
		NewToken    auth.AccessToken `json:"newToken"`
		AccessToken string           `json:"accessToken"`
	}
	if err := json.Unmarshal(rotateRec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode rotate token: %v", err)
	}
	if rotated.OldToken.Status != string(identity.StatusRotated) || rotated.NewToken.RotatedFromTokenID != issued.Token.TokenID || rotated.AccessToken == "" {
		t.Fatalf("unexpected rotate response: %+v", rotated)
	}

	oldMeRec := httptest.NewRecorder()
	oldMeReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	oldMeReq.Header.Set("Authorization", "Bearer "+issued.AccessToken)
	harness.server.Handler().ServeHTTP(oldMeRec, oldMeReq)
	if oldMeRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected old rotated token to be unauthorized, got %d body=%s", oldMeRec.Code, oldMeRec.Body.String())
	}

	revokeRec := httptest.NewRecorder()
	revokeReq := httptest.NewRequest(http.MethodPost, "/v1/auth/tokens/"+rotated.NewToken.TokenID+"/revoke", strings.NewReader(`{"reason":"done"}`))
	revokeReq.Header.Set("Authorization", harness.authHeader)
	revokeReq.Header.Set("X-Dope-Tenant-ID", createdTenant.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected token revoke 200, got %d body=%s", revokeRec.Code, revokeRec.Body.String())
	}

	newMeRec := httptest.NewRecorder()
	newMeReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	newMeReq.Header.Set("Authorization", "Bearer "+rotated.AccessToken)
	newMeReq.Header.Set("X-Dope-Tenant-ID", createdTenant.Tenant.TenantID)
	harness.server.Handler().ServeHTTP(newMeRec, newMeReq)
	if newMeRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked replacement token to be unauthorized, got %d body=%s", newMeRec.Code, newMeRec.Body.String())
	}
}

func TestExpiredTokenDenialWritesTenantAudit(t *testing.T) {
	harness := newTenantAuthHarness(t)
	expiredAt := time.Now().UTC().Add(-time.Minute)
	token, secret, err := harness.authManager.IssueToken(auth.IssueTokenInput{
		PrincipalID:     harness.principal.PrincipalID,
		Label:           "expired",
		DefaultTenantID: harness.defaultTenant.TenantID,
		ExpiresAt:       &expiredAt,
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}
	if err := harness.store.UpsertAccessToken(httptest.NewRequest(http.MethodGet, "/", nil).Context(), token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}
	if err := harness.store.UpsertTokenTenantGrant(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.TokenTenantGrant{
		GrantID:              "grant_expired_token",
		TokenID:              token.TokenID,
		TenantID:             harness.defaultTenant.TenantID,
		IsDefault:            true,
		Status:               identity.StatusActive,
		CreatedAt:            harness.principal.CreatedAt,
		UpdatedAt:            harness.principal.UpdatedAt,
		GrantedByPrincipalID: harness.principal.PrincipalID,
	}); err != nil {
		t.Fatalf("UpsertTokenTenantGrant returned error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	harness.server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected expired token 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	audits, err := harness.store.ListTenantAuditEvents(httptest.NewRequest(http.MethodGet, "/", nil).Context(), identity.AuditEventFilter{TokenID: token.TokenID, EventKind: "tenant.token_expiry_denied"})
	if err != nil {
		t.Fatalf("ListTenantAuditEvents returned error: %v", err)
	}
	if len(audits) != 1 || audits[0].ReasonCode != "token_expired" {
		t.Fatalf("expected token expiry denial audit, got %+v", audits)
	}
}

type tenantAuthHarness struct {
	server        *Server
	store         *store.SQLiteStore
	authManager   *auth.Manager
	authHeader    string
	token         auth.AccessToken
	principal     identity.Principal
	defaultTenant identity.Tenant
	otherTenant   identity.Tenant
}

func newTenantAuthHarness(t *testing.T) tenantAuthHarness {
	t.Helper()

	sqliteStore, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "dope-data"))
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := sqliteStore.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	ctx := context.Background()
	now := time.Now().UTC()
	principal := identity.Principal{
		PrincipalID:     "prn_api_local",
		PrincipalKind:   identity.PrincipalKindLocalOperator,
		DisplayName:     "API local operator",
		Status:          identity.StatusActive,
		DefaultTenantID: "ten_api_default",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	defaultTenant := identity.Tenant{
		TenantID:                principal.DefaultTenantID,
		TenantKind:              identity.TenantKindPersonal,
		DisplayName:             "Default tenant",
		Status:                  identity.StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		CreatedByPrincipalID:    principal.PrincipalID,
		DefaultOwnerPrincipalID: principal.PrincipalID,
	}
	otherTenant := identity.Tenant{
		TenantID:                "ten_api_other",
		TenantKind:              identity.TenantKindOrganization,
		DisplayName:             "Other tenant",
		Status:                  identity.StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		CreatedByPrincipalID:    "prn_other",
		DefaultOwnerPrincipalID: "prn_other",
	}
	if err := sqliteStore.UpsertPrincipal(ctx, principal); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	if err := sqliteStore.UpsertTenant(ctx, defaultTenant); err != nil {
		t.Fatalf("UpsertTenant(default) returned error: %v", err)
	}
	if err := sqliteStore.UpsertTenant(ctx, otherTenant); err != nil {
		t.Fatalf("UpsertTenant(other) returned error: %v", err)
	}
	if err := sqliteStore.UpsertMembership(ctx, identity.Membership{
		MembershipID: "mem_api_owner",
		TenantID:     defaultTenant.TenantID,
		PrincipalID:  principal.PrincipalID,
		Role:         identity.RoleOwner,
		Status:       identity.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
		AcceptedAt:   &now,
	}); err != nil {
		t.Fatalf("UpsertMembership returned error: %v", err)
	}

	authManager := auth.NewManager()
	pairing, code, err := authManager.StartPairing(auth.StartPairingInput{Mode: auth.PairingModeLocal, Label: "tenant-api"})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	_, token, tokenSecret, err := authManager.CompletePairing(pairing.PairingID, auth.CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}
	token.PrincipalID = principal.PrincipalID
	token.DefaultTenantID = defaultTenant.TenantID
	token.Status = string(identity.StatusActive)
	authManager.Restore(nil, []auth.AccessToken{token})
	if err := sqliteStore.UpsertAccessToken(ctx, token); err != nil {
		t.Fatalf("UpsertAccessToken returned error: %v", err)
	}
	if err := sqliteStore.UpsertTokenTenantGrant(ctx, identity.TokenTenantGrant{
		GrantID:              "grant_api_default",
		TokenID:              token.TokenID,
		TenantID:             defaultTenant.TenantID,
		IsDefault:            true,
		Status:               identity.StatusActive,
		CreatedAt:            now,
		UpdatedAt:            now,
		GrantedByPrincipalID: principal.PrincipalID,
	}); err != nil {
		t.Fatalf("UpsertTokenTenantGrant returned error: %v", err)
	}

	identityManager := identity.NewManager(sqliteStore)
	server := NewServer(Dependencies{
		Config: config.Config{
			Environment: config.EnvironmentTest,
			BindAddr:    "127.0.0.1:19191",
			DataDir:     "~/.dope-test",
			LogLevel:    "error",
			Version:     "test",
		},
		Logger:   telemetry.New("error").Slog(),
		EventBus: events.NewBus(),
		Auth:     authManager,
		Identity: identityManager,
		Router:   router.NewSessionRouter(),
		Runtime:  runtime.NewManager(),
		Store:    sqliteStore,
	})

	return tenantAuthHarness{
		server:        server,
		store:         sqliteStore,
		authManager:   authManager,
		authHeader:    "Bearer " + tokenSecret,
		token:         token,
		principal:     principal,
		defaultTenant: defaultTenant,
		otherTenant:   otherTenant,
	}
}
