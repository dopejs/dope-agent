package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
)

type setupStartRequest struct {
	TargetID   string                 `json:"targetId"`
	SetupStyle setupwizard.SetupStyle `json:"setupStyle"`
	Source     string                 `json:"source"`
}

type setupSecretSubmitRequest struct {
	SecretRef    string                    `json:"secretRef"`
	Value        string                    `json:"value"`
	DisplayName  string                    `json:"displayName,omitempty"`
	ResourceRefs []setupwizard.ResourceRef `json:"resourceRefs,omitempty"`
}

type setupOAuthStartRequest struct {
	RedirectRoute string `json:"redirectRoute"`
}

type setupOAuthCallbackRequest struct {
	State        string                  `json:"state"`
	Result       setupwizard.OAuthResult `json:"result"`
	AccountLabel string                  `json:"accountLabel,omitempty"`
}

type setupDisableRequest struct {
	DisabledReason string `json:"disabledReason"`
}

func handleSetupTargets(service *setupwizard.Service, w http.ResponseWriter, r *http.Request) {
	if service == nil {
		writeError(w, http.StatusInternalServerError, "setup wizard service is not configured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeSetupError(w, setupwizard.ErrTenantRequired)
		return
	}
	targets, err := service.ListTargets(r.Context(), setupwizard.ListTargetsInput{TenantContext: tenantContext})
	if err != nil {
		writeSetupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ListResponse[setupwizard.SetupTarget]{Items: targets})
}

func handleSetupSessions(service *setupwizard.Service, w http.ResponseWriter, r *http.Request) {
	if service == nil {
		writeError(w, http.StatusInternalServerError, "setup wizard service is not configured")
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeSetupError(w, setupwizard.ErrTenantRequired)
		return
	}
	switch r.Method {
	case http.MethodGet:
		sessions, err := service.ListSessions(r.Context(), tenantContext)
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[setupwizard.SetupSession]{Items: sessions})
	case http.MethodPost:
		var request setupStartRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		session, err := service.Start(r.Context(), setupwizard.StartInput{
			TenantContext: tenantContext,
			TargetID:      request.TargetID,
			SetupStyle:    request.SetupStyle,
			Source:        request.Source,
		})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"session": session})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleSetupSessionRoutes(service *setupwizard.Service, w http.ResponseWriter, r *http.Request) {
	if service == nil {
		writeError(w, http.StatusInternalServerError, "setup wizard service is not configured")
		return
	}
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeSetupError(w, setupwizard.ErrTenantRequired)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/setup/sessions/"), "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	sessionID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = strings.Join(parts[1:], "/")
	}
	switch {
	case r.Method == http.MethodGet && action == "":
		session, err := service.Get(r.Context(), tenantContext, sessionID)
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case r.Method == http.MethodPost && action == "submit-secret":
		var request setupSecretSubmitRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		session, err := service.SubmitSecret(r.Context(), setupwizard.SubmitSecretInput{
			TenantContext: tenantContext,
			SessionID:     sessionID,
			SecretRef:     request.SecretRef,
			Value:         request.Value,
			DisplayName:   request.DisplayName,
			ResourceRefs:  request.ResourceRefs,
		})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case r.Method == http.MethodPost && action == "oauth/start":
		var request setupOAuthStartRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := service.StartOAuth(r.Context(), setupwizard.OAuthStartInput{
			TenantContext: tenantContext,
			SessionID:     sessionID,
			RedirectRoute: request.RedirectRoute,
		})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": result.Session, "authorizationUrl": result.AuthorizationURL, "state": result.StateRef})
	case r.Method == http.MethodPost && action == "oauth/callback":
		var request setupOAuthCallbackRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		session, err := service.CompleteOAuth(r.Context(), setupwizard.OAuthCallbackInput{
			TenantContext: tenantContext,
			SessionID:     sessionID,
			State:         request.State,
			Result:        request.Result,
			AccountLabel:  request.AccountLabel,
		})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case r.Method == http.MethodPost && action == "retry":
		session, err := service.Retry(r.Context(), setupwizard.ReplaceInput{TenantContext: tenantContext, SessionID: sessionID})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case r.Method == http.MethodPost && action == "replace":
		session, err := service.Replace(r.Context(), setupwizard.ReplaceInput{TenantContext: tenantContext, SessionID: sessionID})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case r.Method == http.MethodPost && action == "cancel":
		session, err := service.Cancel(r.Context(), setupwizard.ReplaceInput{TenantContext: tenantContext, SessionID: sessionID})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case r.Method == http.MethodPost && action == "disable":
		var request setupDisableRequest
		if r.Body != nil && r.ContentLength != 0 {
			if err := decodeJSONBody(r, &request); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		session, err := service.Disable(r.Context(), setupwizard.DisableInput{
			TenantContext:  tenantContext,
			SessionID:      sessionID,
			DisabledReason: request.DisabledReason,
		})
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"session": session})
	case r.Method == http.MethodGet && action == "diagnostics":
		diagnostics, err := service.Diagnostics(r.Context(), tenantContext, sessionID)
		if err != nil {
			writeSetupError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[setupwizard.SetupDiagnostic]{Items: diagnostics})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func writeSetupError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "setup_failed:unexpected"
	stage := "unknown"
	retryable := false
	owner := string(setupwizard.OwnerOperator)
	switch {
	case errors.Is(err, setupwizard.ErrPermissionDenied):
		status = http.StatusForbidden
		code = "setup_denied:missing_permission"
		stage = "permission"
		owner = string(setupwizard.OwnerTenantAdmin)
	case errors.Is(err, setupwizard.ErrTenantRequired):
		status = http.StatusForbidden
		code = "setup_denied:tenant_access"
		stage = "tenant_access"
		owner = string(setupwizard.OwnerTenantAdmin)
	case errors.Is(err, setupwizard.ErrUnsupportedTarget):
		status = http.StatusBadRequest
		code = "setup_blocked:unsupported_target"
		stage = "target"
	case errors.Is(err, setupwizard.ErrSessionNotFound):
		status = http.StatusNotFound
		code = "setup_denied:tenant_access"
		stage = "tenant_access"
	case errors.Is(err, setupwizard.ErrSecretRefRequired), errors.Is(err, setupwizard.ErrSecretValueRequired), errors.Is(err, setupwizard.ErrTargetRequired), errors.Is(err, setupwizard.ErrOAuthStateRequired):
		status = http.StatusBadRequest
		code = "setup_action_required:credential_missing"
		stage = "input"
		retryable = true
		owner = string(setupwizard.OwnerProductUser)
	}
	writeJSON(w, status, map[string]any{
		"error":            "setup permission denied",
		"code":             code,
		"reasonCode":       code,
		"stage":            stage,
		"retryable":        retryable,
		"remediationOwner": owner,
	})
}
