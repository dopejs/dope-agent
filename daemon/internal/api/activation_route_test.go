package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func TestActivationRouteShellRequiresTenantContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/activation", nil)
	rec := httptest.NewRecorder()

	handleActivation(nil, rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected tenant denial 403, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestActivationRouteShellMethods(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		method     string
		path       string
		handler    func(http.ResponseWriter, *http.Request)
		wantStatus int
		wantBody   string
	}{
		{
			name:       "get activation shell",
			method:     http.MethodGet,
			path:       "/v1/activation",
			handler:    func(w http.ResponseWriter, r *http.Request) { handleActivation(nil, w, r) },
			wantStatus: http.StatusNotImplemented,
			wantBody:   "activation_not_implemented",
		},
		{
			name:       "post activation shell",
			method:     http.MethodPost,
			path:       "/v1/activation",
			handler:    func(w http.ResponseWriter, r *http.Request) { handleActivation(nil, w, r) },
			wantStatus: http.StatusNotImplemented,
			wantBody:   "activation_not_implemented",
		},
		{
			name:       "reject activation patch",
			method:     http.MethodPatch,
			path:       "/v1/activation",
			handler:    func(w http.ResponseWriter, r *http.Request) { handleActivation(nil, w, r) },
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "test chat shell",
			method:     http.MethodPost,
			path:       "/v1/activation/test-chat",
			handler:    func(w http.ResponseWriter, r *http.Request) { handleActivationTestChat(nil, w, r) },
			wantStatus: http.StatusNotImplemented,
			wantBody:   "activation_not_implemented",
		},
		{
			name:       "reject test chat get",
			method:     http.MethodGet,
			path:       "/v1/activation/test-chat",
			handler:    func(w http.ResponseWriter, r *http.Request) { handleActivationTestChat(nil, w, r) },
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "diagnostics shell",
			method:     http.MethodGet,
			path:       "/v1/activation/diagnostics",
			handler:    func(w http.ResponseWriter, r *http.Request) { handleActivationDiagnostics(nil, w, r) },
			wantStatus: http.StatusNotImplemented,
			wantBody:   "activation_not_implemented",
		},
		{
			name:       "reject diagnostics post",
			method:     http.MethodPost,
			path:       "/v1/activation/diagnostics",
			handler:    func(w http.ResponseWriter, r *http.Request) { handleActivationDiagnostics(nil, w, r) },
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	tenantContext := identity.TenantContext{
		PrincipalID: "prn_1",
		TokenID:     "tok_1",
		TenantID:    "ten_personal",
		Role:        identity.RoleOwner,
		Permissions: identity.PermissionsForRole(identity.RoleOwner, identity.StatusActive),
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req = req.WithContext(withTenantContext(req.Context(), tenantContext))
			rec := httptest.NewRecorder()

			tc.handler(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.wantStatus, rec.Code, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantBody, rec.Body.String())
			}
		})
	}
}
