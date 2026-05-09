package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
	"github.com/dopejs/dope-agent/daemon/internal/store"
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
	Code         string                  `json:"code,omitempty"`
	RedirectURI  string                  `json:"redirectUri,omitempty"`
}

type setupDisableRequest struct {
	DisabledReason string `json:"disabledReason"`
}

type slackHostedSetupResource struct {
	TenantID           string                         `json:"tenantId,omitempty"`
	ConnectorID        string                         `json:"connectorId"`
	ConnectorKind      string                         `json:"connectorKind"`
	DisplayName        string                         `json:"displayName"`
	Status             string                         `json:"status"`
	TerminalState      string                         `json:"terminalState"`
	OAuthState         string                         `json:"oauthState"`
	RoutePolicyState   string                         `json:"routePolicyState"`
	DeliveryEligible   bool                           `json:"deliveryEligible"`
	WorkspaceBindingID string                         `json:"workspaceBindingId"`
	ReasonCode         string                         `json:"reasonCode,omitempty"`
	RedactionStatus    string                         `json:"redactionStatus"`
	CreatedAt          time.Time                      `json:"createdAt"`
	UpdatedAt          time.Time                      `json:"updatedAt"`
	ValidatedAt        time.Time                      `json:"validatedAt,omitempty"`
	RetentionExpiresAt time.Time                      `json:"retentionExpiresAt"`
	WorkspaceBinding   *slackWorkspaceBindingResource `json:"workspaceBinding,omitempty"`
	RoutePolicy        *slackRoutePolicyResource      `json:"routePolicy,omitempty"`
}

type slackWorkspaceBindingResource struct {
	WorkspaceID        string            `json:"workspaceId"`
	WorkspaceLabel     string            `json:"workspaceLabel,omitempty"`
	InstallationID     string            `json:"installationId"`
	OAuthGrantState    string            `json:"oauthGrantState"`
	RequiredScopeState string            `json:"requiredScopeState"`
	ValidatedAt        time.Time         `json:"validatedAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

type slackRoutePolicyResource struct {
	TenantID            string                               `json:"tenantId,omitempty"`
	ConnectorID         string                               `json:"connectorId"`
	WorkspaceBindingID  string                               `json:"workspaceBindingId"`
	SelectedChannels    []store.SlackConversationRouteRecord `json:"selectedChannels"`
	AllowedDMUsers      []string                             `json:"allowedDMUsers"`
	AllowedDMUserGroups []string                             `json:"allowedDMUserGroups"`
	MentionGate         string                               `json:"mentionGate"`
	ThreadReplyMode     string                               `json:"threadReplyMode"`
	ValidationState     string                               `json:"validationState"`
	ReasonCode          string                               `json:"reasonCode,omitempty"`
	ValidatedAt         time.Time                            `json:"validatedAt"`
	RedactionStatus     string                               `json:"redactionStatus"`
	SafeEvidence        map[string]string                    `json:"safeEvidence,omitempty"`
}

type slackSmokeEvidenceResource struct {
	SmokeEvidenceID    string            `json:"smokeEvidenceId"`
	TenantID           string            `json:"tenantId,omitempty"`
	ConnectorID        string            `json:"connectorId"`
	WorkspaceBindingID string            `json:"workspaceBindingId"`
	Status             string            `json:"status"`
	AuthorizationMode  string            `json:"authorizationMode"`
	Owner              string            `json:"owner"`
	Reason             string            `json:"reason"`
	RemainingRisk      string            `json:"remainingRisk,omitempty"`
	ValidatedAt        time.Time         `json:"validatedAt"`
	RetentionExpiresAt time.Time         `json:"retentionExpiresAt"`
	RedactionStatus    string            `json:"redactionStatus"`
	SafeEvidence       map[string]string `json:"safeEvidence,omitempty"`
}

func projectSlackHostedSetupResource(record store.SlackHostedSetupRecord) slackHostedSetupResource {
	resource := slackHostedSetupResource{
		TenantID:           record.TenantID,
		ConnectorID:        record.ConnectorID,
		ConnectorKind:      record.ConnectorKind,
		DisplayName:        record.DisplayName,
		Status:             record.Status,
		TerminalState:      record.TerminalState,
		OAuthState:         record.OAuthState,
		RoutePolicyState:   record.RoutePolicyState,
		DeliveryEligible:   record.DeliveryEligible,
		WorkspaceBindingID: record.WorkspaceBindingID,
		ReasonCode:         record.ReasonCode,
		RedactionStatus:    record.RedactionStatus,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
		ValidatedAt:        record.ValidatedAt,
		RetentionExpiresAt: record.RetentionExpiresAt,
	}
	if record.WorkspaceBinding != nil {
		resource.WorkspaceBinding = &slackWorkspaceBindingResource{
			WorkspaceID:        record.WorkspaceBinding.WorkspaceID,
			WorkspaceLabel:     record.WorkspaceBinding.WorkspaceLabel,
			InstallationID:     record.WorkspaceBinding.InstallationID,
			OAuthGrantState:    record.WorkspaceBinding.OAuthGrantState,
			RequiredScopeState: record.WorkspaceBinding.RequiredScopeState,
			ValidatedAt:        record.WorkspaceBinding.ValidatedAt,
			RedactionStatus:    record.WorkspaceBinding.RedactionStatus,
			SafeEvidence:       record.WorkspaceBinding.SafeEvidence,
		}
	}
	if record.RoutePolicy != nil {
		policy := projectSlackRoutePolicyResource(*record.RoutePolicy)
		resource.RoutePolicy = &policy
	}
	return resource
}

func projectSlackRoutePolicyResource(record store.SlackRoutePolicyRecord) slackRoutePolicyResource {
	return slackRoutePolicyResource{
		TenantID:            record.TenantID,
		ConnectorID:         record.ConnectorID,
		WorkspaceBindingID:  record.WorkspaceBindingID,
		SelectedChannels:    record.SelectedChannels,
		AllowedDMUsers:      record.AllowedDMUsers,
		AllowedDMUserGroups: record.AllowedDMUserGroups,
		MentionGate:         record.MentionGate,
		ThreadReplyMode:     record.ThreadReplyMode,
		ValidationState:     record.ValidationState,
		ReasonCode:          record.ReasonCode,
		ValidatedAt:         record.ValidatedAt,
		RedactionStatus:     record.RedactionStatus,
		SafeEvidence:        record.SafeEvidence,
	}
}

func projectSlackSmokeEvidenceResource(record store.SlackSmokeEvidenceRecord) slackSmokeEvidenceResource {
	return slackSmokeEvidenceResource{
		SmokeEvidenceID:    record.SmokeEvidenceID,
		TenantID:           record.TenantID,
		ConnectorID:        record.ConnectorID,
		WorkspaceBindingID: record.WorkspaceBindingID,
		Status:             record.Status,
		AuthorizationMode:  record.AuthorizationMode,
		Owner:              record.Owner,
		Reason:             record.Reason,
		RemainingRisk:      record.RemainingRisk,
		ValidatedAt:        record.ValidatedAt,
		RetentionExpiresAt: record.RetentionExpiresAt,
		RedactionStatus:    record.RedactionStatus,
		SafeEvidence:       record.SafeEvidence,
	}
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
			Code:          request.Code,
			RedirectURI:   request.RedirectURI,
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
