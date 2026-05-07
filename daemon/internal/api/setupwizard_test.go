package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestSetupWizardAPICoversProofTargetLifecycleAndDenials(t *testing.T) {
	service := setupwizard.NewService(setupwizard.ServiceDependencies{Store: setupwizard.NewMemoryStore()})
	actor := setupWizardAPITenantContext("ten_setup_api", identity.PermissionSecretsManage, identity.PermissionIntegrationsManage, identity.PermissionCredentialsInspect)

	targetsRec := exerciseSetupWizardRoute(service, actor, http.MethodGet, "/v1/setup/targets", "")
	if targetsRec.Code != http.StatusOK {
		t.Fatalf("targets status=%d body=%s", targetsRec.Code, targetsRec.Body.String())
	}
	var targets struct {
		Items []setupwizard.SetupTarget `json:"items"`
	}
	if err := json.Unmarshal(targetsRec.Body.Bytes(), &targets); err != nil {
		t.Fatalf("decode targets: %v", err)
	}
	if len(targets.Items) < 2 {
		t.Fatalf("expected proof targets, got %+v", targets.Items)
	}
	foundDiscord := false
	for _, target := range targets.Items {
		if target.TargetID == setupwizard.TargetDiscordConnector && target.TargetKind == setupwizard.TargetKindConnector {
			foundDiscord = true
		}
	}
	if !foundDiscord {
		t.Fatalf("expected Discord connector setup target, got %+v", targets.Items)
	}

	startRec := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions", `{"targetId":"provider.openai_compatible","setupStyle":"submitted_secret","source":"wizard"}`)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", startRec.Code, startRec.Body.String())
	}
	var startBody struct {
		Session setupwizard.SetupSession `json:"session"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startBody); err != nil {
		t.Fatalf("decode start: %v", err)
	}

	submitRec := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions/"+startBody.Session.SetupSessionID+"/submit-secret", `{"secretRef":"OPENAI_COMPATIBLE_API_KEY","value":"R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK","displayName":"OpenAI-compatible API key"}`)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", submitRec.Code, submitRec.Body.String())
	}
	if bytes.Contains(submitRec.Body.Bytes(), []byte("R46_FAKE_OPENAI_COMPATIBLE_KEY_DO_NOT_LEAK")) {
		t.Fatalf("submit response leaked secret: %s", submitRec.Body.String())
	}

	oauthStart := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions", `{"targetId":"integration.feishu_lark","setupStyle":"oauth","source":"wizard"}`)
	if oauthStart.Code != http.StatusCreated {
		t.Fatalf("oauth session status=%d body=%s", oauthStart.Code, oauthStart.Body.String())
	}
	var oauthBody struct {
		Session setupwizard.SetupSession `json:"session"`
	}
	if err := json.Unmarshal(oauthStart.Body.Bytes(), &oauthBody); err != nil {
		t.Fatalf("decode oauth start: %v", err)
	}
	oauthState := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions/"+oauthBody.Session.SetupSessionID+"/oauth/start", `{"redirectRoute":"/callback"}`)
	if oauthState.Code != http.StatusOK {
		t.Fatalf("oauth state status=%d body=%s", oauthState.Code, oauthState.Body.String())
	}
	var stateBody struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(oauthState.Body.Bytes(), &stateBody); err != nil {
		t.Fatalf("decode oauth state: %v", err)
	}
	callback := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions/"+oauthBody.Session.SetupSessionID+"/oauth/callback", `{"state":"`+stateBody.State+`","result":"denied"}`)
	if callback.Code != http.StatusOK || !bytes.Contains(callback.Body.Bytes(), []byte(`"state":"action_required"`)) {
		t.Fatalf("callback status=%d body=%s", callback.Code, callback.Body.String())
	}

	discordStart := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions", `{"targetId":"connector.discord","setupStyle":"submitted_secret","source":"wizard"}`)
	if discordStart.Code != http.StatusCreated {
		t.Fatalf("discord session status=%d body=%s", discordStart.Code, discordStart.Body.String())
	}
	var discordBody struct {
		Session setupwizard.SetupSession `json:"session"`
	}
	if err := json.Unmarshal(discordStart.Body.Bytes(), &discordBody); err != nil {
		t.Fatalf("decode discord start: %v", err)
	}
	discordSubmit := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions/"+discordBody.Session.SetupSessionID+"/submit-secret", `{"secretRef":"DISCORD_BOT_TOKEN","value":"R49_FAKE_DISCORD_BOT_TOKEN_DO_NOT_LEAK","displayName":"Discord bot token"}`)
	if discordSubmit.Code != http.StatusOK {
		t.Fatalf("discord submit status=%d body=%s", discordSubmit.Code, discordSubmit.Body.String())
	}
	if !bytes.Contains(discordSubmit.Body.Bytes(), []byte(`"state":"degraded"`)) || !bytes.Contains(discordSubmit.Body.Bytes(), []byte(setupwizard.ReasonDiscordDestinationMissing)) {
		t.Fatalf("expected Discord setup to save degraded until explicit destinations validate, got %s", discordSubmit.Body.String())
	}
	if bytes.Contains(discordSubmit.Body.Bytes(), []byte("R49_FAKE_DISCORD_BOT_TOKEN_DO_NOT_LEAK")) {
		t.Fatalf("discord setup leaked token: %s", discordSubmit.Body.String())
	}

	denied := setupWizardAPITenantContext("ten_setup_api", identity.PermissionSecretsManage, identity.PermissionIntegrationsManage)
	denialRec := exerciseSetupWizardRoute(service, denied, http.MethodGet, "/v1/setup/targets", "")
	if denialRec.Code != http.StatusForbidden || bytes.Contains(denialRec.Body.Bytes(), []byte("OPENAI_COMPATIBLE_API_KEY")) {
		t.Fatalf("inspection denial status=%d body=%s", denialRec.Code, denialRec.Body.String())
	}
}

func TestIntegrationSetupGateBlocksFeishuDependentUse(t *testing.T) {
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	defer sqliteStore.Close()
	ctx := withTenantContext(context.Background(), setupWizardAPITenantContext("ten_lark_gate", identity.PermissionSecretsManage, identity.PermissionIntegrationsManage, identity.PermissionCredentialsInspect))
	if err := sqliteStore.SaveSetupSession(ctx, setupwizard.SetupSession{
		SetupSessionID:   "setup_lark_blocked",
		TenantID:         "ten_lark_gate",
		TargetID:         setupwizard.TargetFeishuLark,
		TargetKind:       setupwizard.TargetKindIntegration,
		SetupStyle:       setupwizard.SetupStyleOAuth,
		State:            setupwizard.StateActionRequired,
		ReasonCode:       setupwizard.ReasonScopeMissing,
		Retryable:        true,
		RemediationOwner: setupwizard.OwnerTenantAdmin,
		SafeUseMode:      setupwizard.SafeUseBlocked,
		RedactionStatus:  setupwizard.RedactionRedacted,
	}); err != nil {
		t.Fatalf("SaveSetupSession returned error: %v", err)
	}

	if err := enforceIntegrationSetupGate(ctx, sqliteStore, setupwizard.TargetFeishuLark, "metadata_read"); !errors.Is(err, integrations.ErrProbeBlocked) {
		t.Fatalf("enforceIntegrationSetupGate error=%v, want ErrProbeBlocked", err)
	}
}

func exerciseSetupWizardRoute(service *setupwizard.Service, tc identity.TenantContext, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req = req.WithContext(withTenantContext(req.Context(), tc))
	rec := httptest.NewRecorder()
	if path == "/v1/setup/targets" {
		handleSetupTargets(service, rec, req)
		return rec
	}
	if path == "/v1/setup/sessions" {
		handleSetupSessions(service, rec, req)
		return rec
	}
	handleSetupSessionRoutes(service, rec, req)
	return rec
}

func setupWizardAPITenantContext(tenantID string, permissions ...identity.Permission) identity.TenantContext {
	return identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: "prn_" + tenantID,
		Permissions: permissions,
	}
}
