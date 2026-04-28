package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func r37TenantContext(t *testing.T, tenantID string, role identity.Role, permissions ...identity.Permission) identity.TenantContext {
	t.Helper()
	if len(permissions) == 0 {
		permissions = identity.PermissionsForRole(role, identity.StatusActive)
	}
	return identity.TenantContext{
		PrincipalID:  "principal_" + tenantID,
		TokenID:      "token_" + tenantID,
		TenantID:     tenantID,
		TenantSource: "test",
		Role:         role,
		Permissions:  append([]identity.Permission(nil), permissions...),
		ResolvedAt:   time.Now().UTC(),
	}
}

func r37TenantAdminContext(t *testing.T, tenantID string) identity.TenantContext {
	t.Helper()
	return r37TenantContext(t, tenantID, identity.RoleAdmin)
}

func r37CredentialOperatorContext(t *testing.T, tenantID string) identity.TenantContext {
	t.Helper()
	return r37TenantContext(t, tenantID, identity.RoleOperator, identity.PermissionCredentialsInspect)
}

func r37ViewerContext(t *testing.T, tenantID string) identity.TenantContext {
	t.Helper()
	return r37TenantContext(t, tenantID, identity.RoleViewer, identity.PermissionReadOnlyInspect)
}

func r37RequestWithTenantContext(t *testing.T, method, target string, tenantContext identity.TenantContext) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(withTenantContext(req.Context(), tenantContext))
}

func TestR37HostedCredentialStableDenials(t *testing.T) {
	cases := []struct {
		name             string
		request          *http.Request
		resourceTenantID string
		wantReason       string
	}{
		{
			name:             "missing tenant",
			request:          httptest.NewRequest(http.MethodGet, "/v1/tenant-secrets", nil),
			resourceTenantID: "",
			wantReason:       credentialDenialMissingTenant,
		},
		{
			name:             "missing permission",
			request:          r37RequestWithTenantContext(t, http.MethodGet, "/v1/tenant-secrets", r37ViewerContext(t, "ten_r37_a")),
			resourceTenantID: "ten_r37_a",
			wantReason:       credentialDenialMissingPermission,
		},
		{
			name:             "cross tenant",
			request:          r37RequestWithTenantContext(t, http.MethodGet, "/v1/tenant-secrets", r37CredentialOperatorContext(t, "ten_r37_a")),
			resourceTenantID: "ten_r37_b",
			wantReason:       credentialDenialCrossTenant,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, reason := requireHostedCredentialPermission(tc.request, credentialPermissionInspect, tc.resourceTenantID)
			if reason != tc.wantReason {
				t.Fatalf("reason=%q, want %q", reason, tc.wantReason)
			}
			rec := httptest.NewRecorder()
			writeCredentialDenial(rec, http.StatusForbidden, reason)
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode denial: %v", err)
			}
			if body["error"] != credentialDenialStableError || body["reasonCode"] != tc.wantReason {
				t.Fatalf("denial body=%v", body)
			}
		})
	}
}

func TestR37ViewerAndUnauthorizedOperatorCannotMutateCredentialResources(t *testing.T) {
	for _, tc := range []struct {
		name       string
		request    *http.Request
		permission identity.Permission
	}{
		{
			name:       "viewer secret mutation",
			request:    r37RequestWithTenantContext(t, http.MethodPost, "/v1/tenant-secrets", r37ViewerContext(t, "ten_r37_a")),
			permission: identity.PermissionSecretsManage,
		},
		{
			name:       "viewer integration mutation",
			request:    r37RequestWithTenantContext(t, http.MethodPost, "/v1/integrations", r37ViewerContext(t, "ten_r37_a")),
			permission: identity.PermissionIntegrationsManage,
		},
		{
			name:       "inspect operator cannot mutate provider auth",
			request:    r37RequestWithTenantContext(t, http.MethodPost, "/v1/providers/codex/auth/revoke", r37CredentialOperatorContext(t, "ten_r37_a")),
			permission: identity.PermissionIntegrationsManage,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, reason := requireHostedCredentialPermission(tc.request, tc.permission, "")
			if reason != credentialDenialMissingPermission {
				t.Fatalf("reason=%q, want %q", reason, credentialDenialMissingPermission)
			}
			rec := httptest.NewRecorder()
			writeCredentialDenial(rec, http.StatusForbidden, reason)
			if bytes.Contains(rec.Body.Bytes(), []byte("R37_FAKE_SECRET")) {
				t.Fatalf("denial leaked sentinel: %s", rec.Body.String())
			}
		})
	}
}

func TestR37TenantSecretCreateListMetadataDisableAPI(t *testing.T) {
	manager := r37SecretManager(t)
	admin := r37TenantAdminContext(t, "ten_r37_a")

	createBody := []byte(`{"secretRef":"shared-key","displayName":"Shared Key","value":"R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK","document":{"owner":"ops"}}`)
	createReq := r37RequestWithTenantContext(t, http.MethodPost, "/v1/tenant-secrets", admin)
	createReq.Body = ioNopCloser(createBody)
	createRec := httptest.NewRecorder()
	handleTenantSecrets(manager, createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	if bytes.Contains(createRec.Body.Bytes(), []byte("R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK")) {
		t.Fatalf("create response leaked secret value: %s", createRec.Body.String())
	}

	listReq := r37RequestWithTenantContext(t, http.MethodGet, "/v1/tenant-secrets", admin)
	listRec := httptest.NewRecorder()
	handleTenantSecrets(manager, listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}
	var list struct {
		Items []secrets.TenantSecret `json:"items"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].TenantID != "ten_r37_a" || list.Items[0].SecretRef != "shared-key" {
		t.Fatalf("unexpected list: %#v", list.Items)
	}

	patchReq := r37RequestWithTenantContext(t, http.MethodPatch, "/v1/tenant-secrets/shared-key", admin)
	patchReq.Body = ioNopCloser([]byte(`{"displayName":"Rotatable Key","document":{"owner":"secops"}}`))
	patchRec := httptest.NewRecorder()
	handleTenantSecretRoutes(manager, patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", patchRec.Code, patchRec.Body.String())
	}

	disableReq := r37RequestWithTenantContext(t, http.MethodPost, "/v1/tenant-secrets/shared-key/disable", admin)
	disableReq.Body = ioNopCloser([]byte(`{"disabledReason":"operator_request"}`))
	disableRec := httptest.NewRecorder()
	handleTenantSecretRoutes(manager, disableRec, disableReq)
	if disableRec.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disableRec.Code, disableRec.Body.String())
	}
}

func TestR37TenantSecretInspectGetIsRedactedAndPermissioned(t *testing.T) {
	manager := r37SecretManager(t)
	if _, err := manager.Create(httptest.NewRequest(http.MethodPost, "/", nil).Context(), secrets.CreateInput{
		TenantID:  "ten_r37_a",
		SecretRef: "inspect-key",
		Value:     "R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK",
	}); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	inspectReq := r37RequestWithTenantContext(t, http.MethodGet, "/v1/tenant-secrets/inspect-key", r37CredentialOperatorContext(t, "ten_r37_a"))
	inspectRec := httptest.NewRecorder()
	handleTenantSecretRoutes(manager, inspectRec, inspectReq)
	if inspectRec.Code != http.StatusOK {
		t.Fatalf("inspect status=%d body=%s", inspectRec.Code, inspectRec.Body.String())
	}
	if bytes.Contains(inspectRec.Body.Bytes(), []byte("R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK")) {
		t.Fatalf("inspect response leaked secret value: %s", inspectRec.Body.String())
	}

	viewerReq := r37RequestWithTenantContext(t, http.MethodGet, "/v1/tenant-secrets/inspect-key", r37ViewerContext(t, "ten_r37_a"))
	viewerRec := httptest.NewRecorder()
	handleTenantSecretRoutes(manager, viewerRec, viewerReq)
	if viewerRec.Code != http.StatusForbidden || !bytes.Contains(viewerRec.Body.Bytes(), []byte(credentialDenialMissingPermission)) {
		t.Fatalf("viewer status=%d body=%s", viewerRec.Code, viewerRec.Body.String())
	}
}

func TestR37ConnectorProjectionIncludesRedactedSecretSummary(t *testing.T) {
	connector := projectConnectorResource(connectors.Connector{
		TenantID:    "ten_r37_a",
		ConnectorID: "discord",
		Kind:        "discord",
		DisplayName: "Discord",
		Status:      connectors.StatusHealthy,
		SecretRefs:  []string{"discord/token"},
	})
	if len(connector.SecretSummary) != 1 {
		t.Fatalf("expected redacted secret summary, got %+v", connector.SecretSummary)
	}
	summary := connector.SecretSummary[0]
	if summary.SecretRef != "discord/token" || summary.RedactionRule == "" || summary.Resolution != secrets.ResolutionStatusUnavailable {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestR37SandboxExplainSecretScopeRequiresCredentialInspect(t *testing.T) {
	decision := sandbox.Decision{
		Consumer: &sandbox.ConsumerContractView{
			Declaration: &sandbox.ConsumerRequirementDeclaration{SecretRefs: []string{"sandbox/token"}},
			SecretScope: []sandbox.SecretScopeOutcome{{
				ConsumerKind:     sandbox.ConsumerKindManagedProvider,
				ConsumerID:       "codex",
				SecretRef:        "sandbox/token",
				EnvironmentScope: sandbox.SecretEnvironmentScopeTest,
				Resolution:       sandbox.SecretResolutionResolved,
			}},
		},
	}
	viewerReq := r37RequestWithTenantContext(t, http.MethodPost, "/v1/sandbox/explain", r37ViewerContext(t, "ten_r37_a"))
	redactSandboxDecisionCredentialInspection(viewerReq, &decision)
	if len(decision.Consumer.SecretScope) != 0 || len(decision.Consumer.Declaration.SecretRefs) != 0 {
		t.Fatalf("viewer saw secret scope: %+v", decision.Consumer)
	}

	operatorDecision := sandbox.Decision{
		Consumer: &sandbox.ConsumerContractView{
			Declaration: &sandbox.ConsumerRequirementDeclaration{SecretRefs: []string{"sandbox/token"}},
			SecretScope: []sandbox.SecretScopeOutcome{{
				ConsumerKind:     sandbox.ConsumerKindManagedProvider,
				ConsumerID:       "codex",
				SecretRef:        "sandbox/token",
				EnvironmentScope: sandbox.SecretEnvironmentScopeTest,
				Resolution:       sandbox.SecretResolutionResolved,
			}},
		},
	}
	operatorReq := r37RequestWithTenantContext(t, http.MethodPost, "/v1/sandbox/explain", r37CredentialOperatorContext(t, "ten_r37_a"))
	redactSandboxDecisionCredentialInspection(operatorReq, &operatorDecision)
	if len(operatorDecision.Consumer.SecretScope) != 1 || len(operatorDecision.Consumer.Declaration.SecretRefs) != 1 {
		t.Fatalf("operator secret scope redacted unexpectedly: %+v", operatorDecision.Consumer)
	}
}

func TestR37TenantSecretRotateAPIUsesNewActiveVersion(t *testing.T) {
	manager := r37SecretManager(t)
	admin := r37TenantAdminContext(t, "ten_r37_a")
	if _, err := manager.Create(httptest.NewRequest(http.MethodPost, "/", nil).Context(), secrets.CreateInput{
		TenantID: "ten_r37_a", SecretRef: "rotating-key", Value: "old-value",
	}); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	rotateReq := r37RequestWithTenantContext(t, http.MethodPost, "/v1/tenant-secrets/rotating-key/rotate", admin)
	rotateReq.Body = ioNopCloser([]byte(`{"value":"new-value"}`))
	rotateRec := httptest.NewRecorder()
	handleTenantSecretRoutes(manager, rotateRec, rotateReq)
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("rotate status=%d body=%s", rotateRec.Code, rotateRec.Body.String())
	}
	resolved, err := manager.Resolve(rotateReq.Context(), secrets.ResolveInput{TenantID: "ten_r37_a", SecretRef: "rotating-key"})
	if err != nil {
		t.Fatalf("resolve rotated secret: %v", err)
	}
	if resolved.Value != "new-value" {
		t.Fatalf("resolved value=%q, want new-value", resolved.Value)
	}
}

func r37SecretManager(t *testing.T) *secrets.Manager {
	t.Helper()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	backend, err := secrets.NewLocalBackend(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalBackend: %v", err)
	}
	return secrets.NewManager(sqliteStore, backend)
}

func ioNopCloser(data []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(data))
}
