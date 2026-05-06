package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/activation"
	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	storepkg "github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestActivationAPIPostCreatesAndReusesPersonalTenantWithoutDeveloperSetup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := storepkg.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = sqliteStore.Close()
	})
	if err := sqliteStore.UpsertPrincipal(ctx, identity.Principal{
		PrincipalID:   "prn_hosted_api",
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Hosted API User",
		Status:        identity.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	service := activation.NewService(activation.Dependencies{
		StateStore:       sqliteStore,
		Identity:         sqliteStore,
		Audit:            sqliteStore,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	first := exerciseActivationPost(t, service, auth.AccessToken{TokenID: "tok_hosted_api", PrincipalID: "prn_hosted_api", Status: string(identity.StatusActive)}, `{"source":"signup"}`)
	second := exerciseActivationPost(t, service, auth.AccessToken{TokenID: "tok_hosted_api", PrincipalID: "prn_hosted_api", Status: string(identity.StatusActive)}, `{"source":"returning_user"}`)
	current := exerciseActivationGet(t, service, auth.AccessToken{TokenID: "tok_hosted_api", PrincipalID: "prn_hosted_api", Status: string(identity.StatusActive)}, identity.TenantContext{
		PrincipalID: "prn_hosted_api",
		TokenID:     "tok_hosted_api",
		TenantID:    first.Activation.TenantID,
		Role:        identity.RoleOwner,
		Permissions: identity.PermissionsForRole(identity.RoleOwner, identity.StatusActive),
	})

	if first.Activation.TenantID == "" || first.Activation.TenantID != second.Activation.TenantID {
		t.Fatalf("expected stable tenant id, first=%q second=%q", first.Activation.TenantID, second.Activation.TenantID)
	}
	if current.Activation.ActivationID != first.Activation.ActivationID {
		t.Fatalf("GET activation should return persisted activation %q, got %q", first.Activation.ActivationID, current.Activation.ActivationID)
	}
	if first.Activation.Status != activation.StatusActive || second.Activation.Status != activation.StatusActive {
		t.Fatalf("expected active activation states, first=%#v second=%#v", first.Activation, second.Activation)
	}
	tenants, err := sqliteStore.ListTenants(ctx, identity.TenantFilter{TenantKind: identity.TenantKindPersonal, Status: identity.StatusActive, Limit: 100})
	if err != nil {
		t.Fatalf("ListTenants returned error: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("expected one personal tenant, got %d: %#v", len(tenants), tenants)
	}
}

func TestActivationAPIPostInviteAcceptanceDoesNotRequireOrganizationOnboarding(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := storepkg.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = sqliteStore.Close()
	})
	if err := sqliteStore.UpsertPrincipal(ctx, identity.Principal{
		PrincipalID:   "prn_invited_api",
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Invited API User",
		Status:        identity.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	if err := sqliteStore.UpsertTenant(ctx, identity.Tenant{
		TenantID:                "ten_org_invite",
		TenantKind:              identity.TenantKindOrganization,
		DisplayName:             "Inviting Org",
		Status:                  identity.StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		DefaultOwnerPrincipalID: "prn_invited_api",
	}); err != nil {
		t.Fatalf("UpsertTenant org returned error: %v", err)
	}
	service := activation.NewService(activation.Dependencies{
		StateStore:       sqliteStore,
		Identity:         sqliteStore,
		Audit:            sqliteStore,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})

	response := exerciseActivationPost(t, service, auth.AccessToken{TokenID: "tok_invited_api", PrincipalID: "prn_invited_api", Status: string(identity.StatusActive)}, `{"source":"invite_acceptance"}`)
	if response.Activation.Status != activation.StatusActive {
		t.Fatalf("expected active activation, got %#v", response.Activation)
	}
	if response.Activation.TenantID == "ten_org_invite" {
		t.Fatalf("activation must resolve a personal tenant, got organization tenant %q", response.Activation.TenantID)
	}
}

func TestActivationAPIPostDisabledPrincipalReturnsStableDeniedReason(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := storepkg.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = sqliteStore.Close()
	})
	if err := sqliteStore.UpsertPrincipal(ctx, identity.Principal{
		PrincipalID:   "prn_disabled_api",
		PrincipalKind: identity.PrincipalKindUser,
		DisplayName:   "Disabled API User",
		Status:        identity.StatusDisabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	service := activation.NewService(activation.Dependencies{
		StateStore:       sqliteStore,
		Identity:         sqliteStore,
		Audit:            sqliteStore,
		Now:              func() time.Time { return now },
		EnvironmentScope: "test",
		Hosted:           true,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/activation", strings.NewReader(`{"source":"signup"}`))
	req = req.WithContext(withAuthenticatedToken(req.Context(), auth.AccessToken{TokenID: "tok_disabled_api", PrincipalID: "prn_disabled_api", Status: string(identity.StatusActive)}))
	rec := httptest.NewRecorder()

	handleActivation(service, rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload["code"] != string(activation.ReasonPrincipalDisabled) {
		t.Fatalf("expected code %q, got %#v", activation.ReasonPrincipalDisabled, payload)
	}
}

func TestActivationAPITestChatCompletesMetadataOnlyFirstAction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := storepkg.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	if err := sqliteStore.UpsertPrincipal(ctx, identity.Principal{PrincipalID: "prn_test_chat_api", PrincipalKind: identity.PrincipalKindUser, DisplayName: "Hosted API User", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	chatRunner := &apiActivationChatRunner{result: activation.TestChatResult{
		DispatchID:   "dispatch_activation",
		Status:       activation.TestChatStatusCompleted,
		Provider:     "test",
		Model:        "test-chat",
		Usage:        map[string]any{"inputTokens": 1, "reply": "must drop"},
		FinishReason: "stop",
		CompletedAt:  now,
	}}
	service := activation.NewService(activation.Dependencies{StateStore: sqliteStore, Identity: sqliteStore, Chat: chatRunner, Audit: sqliteStore, Now: func() time.Time { return now }, EnvironmentScope: "test", Hosted: true})
	started := exerciseActivationPost(t, service, auth.AccessToken{TokenID: "tok_test_chat_api", PrincipalID: "prn_test_chat_api", Status: string(identity.StatusActive)}, `{"source":"signup"}`)

	response, body := exerciseActivationTestChat(t, service, auth.AccessToken{TokenID: "tok_test_chat_api", PrincipalID: "prn_test_chat_api", Status: string(identity.StatusActive)}, identity.TenantContext{
		PrincipalID: "prn_test_chat_api",
		TokenID:     "tok_test_chat_api",
		TenantID:    started.Activation.TenantID,
		Role:        identity.RoleOwner,
		Permissions: identity.PermissionsForRole(identity.RoleOwner, identity.StatusActive),
	}, `{"message":"Do not persist this activation prompt."}`)

	if response.Activation.Status != activation.StatusFirstActionCompleted || response.TestChat.DispatchID != "dispatch_activation" {
		t.Fatalf("expected completed activation test chat, got %#v", response)
	}
	if strings.Contains(string(body), "Do not persist this activation prompt") || strings.Contains(string(body), "must drop") {
		t.Fatalf("test chat response retained forbidden message content: %s", string(body))
	}
	if chatRunner.last.TenantID != started.Activation.TenantID || chatRunner.last.Message == "" {
		t.Fatalf("expected chat runner to execute under active tenant, got %#v", chatRunner.last)
	}
}

func TestActivationAPITestChatReadinessFailureAndTenantMismatchUseStableReasons(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	sqliteStore, err := storepkg.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	if err := sqliteStore.UpsertPrincipal(ctx, identity.Principal{PrincipalID: "prn_blocked_chat_api", PrincipalKind: identity.PrincipalKindUser, DisplayName: "Hosted API User", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("UpsertPrincipal returned error: %v", err)
	}
	service := activation.NewService(activation.Dependencies{
		StateStore: sqliteStore,
		Identity:   sqliteStore,
		Billing:    apiFailingBillingProjector{},
		Chat:       &apiActivationChatRunner{},
		Audit:      sqliteStore,
		Now:        func() time.Time { return now },
		Hosted:     true,
	})
	started := exerciseActivationPost(t, service, auth.AccessToken{TokenID: "tok_blocked_chat_api", PrincipalID: "prn_blocked_chat_api", Status: string(identity.StatusActive)}, `{"source":"signup"}`)
	blockedPayload := exerciseActivationTestChatError(t, service, auth.AccessToken{TokenID: "tok_blocked_chat_api", PrincipalID: "prn_blocked_chat_api", Status: string(identity.StatusActive)}, identity.TenantContext{
		PrincipalID: "prn_blocked_chat_api",
		TokenID:     "tok_blocked_chat_api",
		TenantID:    started.Activation.TenantID,
		Role:        identity.RoleOwner,
		Permissions: identity.PermissionsForRole(identity.RoleOwner, identity.StatusActive),
	}, `{"message":"safe"}`)
	if blockedPayload["reasonCode"] != string(activation.ReasonQuotaBaselineUnavailable) {
		t.Fatalf("expected quota blocker reason, got %#v", blockedPayload)
	}

	mismatchPayload := exerciseActivationTestChatError(t, service, auth.AccessToken{TokenID: "tok_blocked_chat_api", PrincipalID: "prn_blocked_chat_api", Status: string(identity.StatusActive)}, identity.TenantContext{
		PrincipalID: "prn_blocked_chat_api",
		TokenID:     "tok_blocked_chat_api",
		TenantID:    "ten_other",
		Role:        identity.RoleOwner,
		Permissions: identity.PermissionsForRole(identity.RoleOwner, identity.StatusActive),
	}, `{"message":"safe"}`)
	if mismatchPayload["reasonCode"] != string(activation.ReasonTenantAccessRevoked) {
		t.Fatalf("expected tenant mismatch reason, got %#v", mismatchPayload)
	}
}

type activationAPIResponse struct {
	Activation activation.State `json:"activation"`
}

type activationAPITestChatResponse struct {
	Activation activation.State            `json:"activation"`
	TestChat   activation.TestChatMetadata `json:"testChat"`
}

func exerciseActivationPost(t *testing.T, service *activation.Service, token auth.AccessToken, body string) activationAPIResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/activation", strings.NewReader(body))
	req = req.WithContext(withAuthenticatedToken(req.Context(), token))
	rec := httptest.NewRecorder()

	handleActivation(service, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response activationAPIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode activation response: %v", err)
	}
	return response
}

func exerciseActivationTestChat(t *testing.T, service *activation.Service, token auth.AccessToken, tenantContext identity.TenantContext, body string) (activationAPITestChatResponse, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/activation/test-chat", strings.NewReader(body))
	req = req.WithContext(withAuthenticatedToken(req.Context(), token))
	req = req.WithContext(withTenantContext(req.Context(), tenantContext))
	rec := httptest.NewRecorder()

	handleActivationTestChat(service, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response activationAPITestChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode activation test chat response: %v", err)
	}
	return response, rec.Body.Bytes()
}

func exerciseActivationTestChatError(t *testing.T, service *activation.Service, token auth.AccessToken, tenantContext identity.TenantContext, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/activation/test-chat", strings.NewReader(body))
	req = req.WithContext(withAuthenticatedToken(req.Context(), token))
	req = req.WithContext(withTenantContext(req.Context(), tenantContext))
	rec := httptest.NewRecorder()

	handleActivationTestChat(service, rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode activation test chat error: %v", err)
	}
	return payload
}

type apiActivationChatRunner struct {
	result activation.TestChatResult
	err    error
	last   activation.TestChatInput
}

func (r *apiActivationChatRunner) RunActivationTestChat(_ context.Context, input activation.TestChatInput) (activation.TestChatResult, error) {
	r.last = input
	return r.result, r.err
}

type apiFailingBillingProjector struct{}

func (apiFailingBillingProjector) UsageSummary(context.Context, string, bool) (billing.UsageSummary, error) {
	return billing.UsageSummary{}, billing.ErrQuotaStateUnavailable
}

func exerciseActivationGet(t *testing.T, service *activation.Service, token auth.AccessToken, tenantContext identity.TenantContext) activationAPIResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/activation", nil)
	req = req.WithContext(withAuthenticatedToken(req.Context(), token))
	req = req.WithContext(withTenantContext(req.Context(), tenantContext))
	rec := httptest.NewRecorder()

	handleActivation(service, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response activationAPIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode activation get response: %v", err)
	}
	return response
}
