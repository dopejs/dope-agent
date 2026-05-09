package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestTelegramSetupWizardPersistsValidCredentialEvenWhenAllowmentMissing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123:token/getMe" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"result": map[string]any{
				"id":       42,
				"username": "dope_test_bot",
			},
		})
	}))
	t.Cleanup(server.Close)

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	integration := newTelegramSetupWizardIntegration(sqliteStore, config.TelegramConnectorConfig{
		ConnectorID:   "telegram-main",
		DisplayName:   "Telegram Main",
		BotAPIBaseURL: server.URL,
	})
	session := setupwizard.SetupSession{
		SetupSessionID: "setup_telegram",
		TenantID:       "ten_telegram",
		TargetID:       setupwizard.TargetTelegramConnector,
		CreatedAt:      time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC),
	}
	input := setupwizard.SubmitSecretInput{SessionID: session.SetupSessionID, SecretRef: "TELEGRAM_BOT_TOKEN", Value: "123:token"}

	probe, err := integration.ProbeSubmittedSecret(context.Background(), session, input)
	if err != nil {
		t.Fatalf("ProbeSubmittedSecret returned error: %v", err)
	}
	if probe.State != setupwizard.StateActionRequired || probe.ReasonCode != setupwizard.ReasonTelegramAllowmentMissing {
		t.Fatalf("expected action-required allowment diagnostic, got %+v", probe)
	}
	session.State = probe.State
	session.ReasonCode = probe.ReasonCode
	if err := integration.RecordSubmittedSecretSetup(context.Background(), session, input); err != nil {
		t.Fatalf("RecordSubmittedSecretSetup returned error: %v", err)
	}
	stored, ok, err := sqliteStore.GetTelegramHostedSetup(context.Background(), "ten_telegram", "telegram-main")
	if err != nil || !ok {
		t.Fatalf("GetTelegramHostedSetup ok=%v err=%v", ok, err)
	}
	if stored.CredentialState != "valid" || stored.TerminalState != "action-required" || stored.AccountBinding == nil {
		t.Fatalf("expected valid credential with action-required allowment state, got %+v", stored)
	}
}

func TestSlackSetupWizardOAuthPersistsHostedSetup(t *testing.T) {
	t.Parallel()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	server := NewServer(Dependencies{
		Config: config.Config{
			Connectors: config.ConnectorConfig{
				Slack: config.SlackConnectorConfig{
					ConnectorID:        "slack-main",
					DisplayName:        "Slack Main",
					OAuthClientID:      "client-redacted",
					OAuthAPIBaseURL:    "https://slack.test",
					WorkspaceBindingID: "workspace_binding_redacted",
					WorkspaceID:        "workspace_redacted",
					AllowedChannelIDs:  []string{"channel_selected"},
					AllowedDMUserIDs:   []string{"user_allowed"},
				},
			},
		},
		Store: sqliteStore,
	})
	actor := setupWizardAPITenantContext("ten_slack_oauth", identity.PermissionSecretsManage, identity.PermissionIntegrationsManage, identity.PermissionCredentialsInspect)

	startReq := httptest.NewRequest(http.MethodPost, "/v1/setup/sessions", strings.NewReader(`{"targetId":"connector.slack","setupStyle":"oauth","source":"wizard"}`))
	startReq = startReq.WithContext(withTenantContext(startReq.Context(), actor))
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", startRec.Code, startRec.Body.String())
	}
	var startBody struct {
		Session setupwizard.SetupSession `json:"session"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startBody); err != nil {
		t.Fatalf("decode start: %v", err)
	}

	oauthReq := httptest.NewRequest(http.MethodPost, "/v1/setup/sessions/"+startBody.Session.SetupSessionID+"/oauth/start", strings.NewReader(`{"redirectRoute":"/setup/oauth/slack/callback"}`))
	oauthReq = oauthReq.WithContext(withTenantContext(oauthReq.Context(), actor))
	oauthRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(oauthRec, oauthReq)
	if oauthRec.Code != http.StatusOK {
		t.Fatalf("oauth start status=%d body=%s", oauthRec.Code, oauthRec.Body.String())
	}
	var oauthBody struct {
		AuthorizationURL string `json:"authorizationUrl"`
		State            string `json:"state"`
	}
	if err := json.Unmarshal(oauthRec.Body.Bytes(), &oauthBody); err != nil {
		t.Fatalf("decode oauth start: %v", err)
	}
	authorizationURL, err := url.Parse(oauthBody.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse authorizationUrl: %v", err)
	}
	if authorizationURL.Scheme != "https" || authorizationURL.Host != "slack.test" || authorizationURL.Path != "/oauth/v2/authorize" {
		t.Fatalf("expected Slack OAuth authorize URL, got %s", oauthBody.AuthorizationURL)
	}
	query := authorizationURL.Query()
	if query.Get("client_id") != "client-redacted" || query.Get("state") != oauthBody.State || !strings.Contains(query.Get("scope"), "chat:write") {
		t.Fatalf("unexpected Slack OAuth authorize query: %s", authorizationURL.RawQuery)
	}

	session, ok, err := sqliteStore.GetSetupSession(context.Background(), actor.TenantID, startBody.Session.SetupSessionID)
	if err != nil || !ok {
		t.Fatalf("GetSetupSession ok=%v err=%v", ok, err)
	}
	session.ResourceRefs = []setupwizard.ResourceRef{{Kind: "slack_route_policy_validation", ID: "slack-main/workspace_redacted"}}
	if err := sqliteStore.SaveSetupSession(context.Background(), session); err != nil {
		t.Fatalf("SaveSetupSession returned error: %v", err)
	}

	callbackReq := httptest.NewRequest(http.MethodPost, "/v1/setup/sessions/"+startBody.Session.SetupSessionID+"/oauth/callback", strings.NewReader(`{"state":"`+oauthBody.State+`","result":"completed","accountLabel":"Workspace Redacted"}`))
	callbackReq = callbackReq.WithContext(withTenantContext(callbackReq.Context(), actor))
	callbackRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("callback status=%d body=%s", callbackRec.Code, callbackRec.Body.String())
	}

	stored, ok, err := sqliteStore.GetSlackHostedSetup(context.Background(), actor.TenantID, "slack-main")
	if err != nil || !ok {
		t.Fatalf("GetSlackHostedSetup ok=%v err=%v", ok, err)
	}
	if stored.TerminalState != "ready" || !stored.DeliveryEligible || stored.WorkspaceBinding == nil || stored.RoutePolicy == nil {
		t.Fatalf("expected ready Slack hosted setup from OAuth wizard, got %+v", stored)
	}
}

func TestSlackSetupWizardOAuthCodeExchangeStoresBotTokenSecret(t *testing.T) {
	t.Parallel()

	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth.v2.access" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm returned error: %v", err)
		}
		if r.Form.Get("client_id") != "client-redacted" || r.Form.Get("client_secret") != "secret-redacted" || r.Form.Get("code") != "code-redacted" {
			t.Fatalf("unexpected oauth form: %s", r.Form.Encode())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":           true,
			"access_token": "xoxb-redacted-token",
			"scope":        "chat:write,channels:read",
			"bot_user_id":  "B123",
			"team": map[string]string{
				"id":   "workspace_oauth",
				"name": "OAuth Workspace",
			},
		})
	}))
	t.Cleanup(slackServer.Close)

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	backend, err := secrets.NewLocalBackend(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalBackend returned error: %v", err)
	}
	secretManager := secrets.NewManager(sqliteStore, backend)

	server := NewServer(Dependencies{
		Config: config.Config{
			Connectors: config.ConnectorConfig{
				Slack: config.SlackConnectorConfig{
					ConnectorID:        "slack-main",
					DisplayName:        "Slack Main",
					OAuthClientID:      "client-redacted",
					OAuthClientSecret:  "secret-redacted",
					OAuthAPIBaseURL:    slackServer.URL,
					BotTokenSecretRef:  "slack/slack-main/bot_token",
					WorkspaceBindingID: "workspace_binding_redacted",
					AllowedDMUserIDs:   []string{"user_allowed"},
				},
			},
		},
		Store:   sqliteStore,
		Secrets: secretManager,
	})
	actor := setupWizardAPITenantContext("ten_slack_oauth_exchange", identity.PermissionSecretsManage, identity.PermissionIntegrationsManage, identity.PermissionCredentialsInspect)

	startReq := httptest.NewRequest(http.MethodPost, "/v1/setup/sessions", strings.NewReader(`{"targetId":"connector.slack","setupStyle":"oauth","source":"wizard"}`))
	startReq = startReq.WithContext(withTenantContext(startReq.Context(), actor))
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", startRec.Code, startRec.Body.String())
	}
	var startBody struct {
		Session setupwizard.SetupSession `json:"session"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &startBody); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	oauthReq := httptest.NewRequest(http.MethodPost, "/v1/setup/sessions/"+startBody.Session.SetupSessionID+"/oauth/start", strings.NewReader(`{"redirectRoute":"/setup/oauth/slack/callback"}`))
	oauthReq = oauthReq.WithContext(withTenantContext(oauthReq.Context(), actor))
	oauthRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(oauthRec, oauthReq)
	var oauthBody struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(oauthRec.Body.Bytes(), &oauthBody); err != nil {
		t.Fatalf("decode oauth start: %v", err)
	}
	session, ok, err := sqliteStore.GetSetupSession(context.Background(), actor.TenantID, startBody.Session.SetupSessionID)
	if err != nil || !ok {
		t.Fatalf("GetSetupSession ok=%v err=%v", ok, err)
	}
	session.ResourceRefs = []setupwizard.ResourceRef{{Kind: "slack_route_policy_validation", ID: "slack-main/workspace_oauth"}}
	if err := sqliteStore.SaveSetupSession(context.Background(), session); err != nil {
		t.Fatalf("SaveSetupSession returned error: %v", err)
	}

	callbackReq := httptest.NewRequest(http.MethodPost, "/v1/setup/sessions/"+startBody.Session.SetupSessionID+"/oauth/callback", strings.NewReader(`{"state":"`+oauthBody.State+`","result":"completed","code":"code-redacted","redirectUri":"https://dope.test/setup/oauth/slack/callback"}`))
	callbackReq = callbackReq.WithContext(withTenantContext(callbackReq.Context(), actor))
	callbackRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("callback status=%d body=%s", callbackRec.Code, callbackRec.Body.String())
	}

	resolved, err := secretManager.Resolve(context.Background(), secrets.ResolveInput{TenantID: actor.TenantID, SecretRef: "slack/slack-main/bot_token"})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if resolved.Value != "xoxb-redacted-token" {
		t.Fatalf("unexpected stored token value %q", resolved.Value)
	}
	setup, ok, err := sqliteStore.GetSlackHostedSetup(context.Background(), actor.TenantID, "slack-main")
	if err != nil || !ok {
		t.Fatalf("GetSlackHostedSetup ok=%v err=%v", ok, err)
	}
	if setup.TerminalState != "ready" || setup.WorkspaceBinding == nil || setup.WorkspaceBinding.WorkspaceID != "workspace_oauth" {
		t.Fatalf("expected ready exchanged Slack setup, got %+v", setup)
	}
	if strings.Contains(callbackRec.Body.String(), "xoxb-redacted-token") {
		t.Fatalf("callback leaked token: %s", callbackRec.Body.String())
	}
}
