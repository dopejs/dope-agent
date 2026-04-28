package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

const (
	credentialDenialMissingTenant      = "credential_denied:missing_tenant"
	credentialDenialMissingPermission  = "credential_denied:missing_permission"
	credentialDenialCrossTenant        = "credential_denied:cross_tenant"
	credentialDenialStableError        = "credential_access_denied"
	credentialPermissionInspect        = identity.PermissionCredentialsInspect
	credentialPermissionManageSecrets  = identity.PermissionSecretsManage
	credentialPermissionManageProvider = identity.PermissionIntegrationsManage
)

func requireHostedCredentialPermission(r *http.Request, permission identity.Permission, resourceTenantID string) (identity.TenantContext, string) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		return identity.TenantContext{}, credentialDenialMissingTenant
	}
	if resourceTenantID != "" && resourceTenantID != tenantContext.TenantID {
		return identity.TenantContext{}, credentialDenialCrossTenant
	}
	if err := identity.RequirePermission(tenantContext, permission); err != nil {
		return identity.TenantContext{}, credentialDenialMissingPermission
	}
	return tenantContext, ""
}

func writeCredentialDenial(w http.ResponseWriter, status int, reasonCode string) {
	writeJSON(w, status, map[string]any{
		"error":      credentialDenialStableError,
		"reasonCode": reasonCode,
	})
}

type createTenantSecretRequest struct {
	SecretRef   string         `json:"secretRef"`
	DisplayName string         `json:"displayName,omitempty"`
	Value       string         `json:"value"`
	Document    map[string]any `json:"document,omitempty"`
}

type updateTenantSecretRequest struct {
	DisplayName *string        `json:"displayName,omitempty"`
	Document    map[string]any `json:"document,omitempty"`
}

type rotateTenantSecretRequest struct {
	Value string `json:"value"`
}

type disableTenantSecretRequest struct {
	DisabledReason string `json:"disabledReason"`
}

func handleTenantSecrets(manager *secrets.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "tenant secret manager is not configured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		tenantContext, reason := requireHostedCredentialRead(r)
		if reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		items, err := manager.List(r.Context(), tenantContext.TenantID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ListResponse[secrets.TenantSecret]{Items: items})
	case http.MethodPost:
		tenantContext, reason := requireHostedCredentialPermission(r, credentialPermissionManageSecrets, "")
		if reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		var request createTenantSecretRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		secret, err := manager.Create(r.Context(), secrets.CreateInput{
			TenantID:    tenantContext.TenantID,
			SecretRef:   request.SecretRef,
			DisplayName: request.DisplayName,
			Value:       request.Value,
			Document:    request.Document,
		})
		if err != nil {
			writeTenantSecretError(w, err)
			return
		}
		if err := recordCredentialAudit(r.Context(), audit.CredentialAuditInput{
			TenantID:     tenantContext.TenantID,
			PrincipalID:  tenantContext.PrincipalID,
			ResourceKind: secrets.ResourceKindTenantSecret,
			ResourceID:   secret.SecretID,
			Action:       secrets.AuditActionSecretCreate,
			SecretRef:    secret.SecretRef,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"secret": secret})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleTenantSecretRoutes(manager *secrets.Manager, w http.ResponseWriter, r *http.Request) {
	if manager == nil {
		writeError(w, http.StatusInternalServerError, "tenant secret manager is not configured")
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/tenant-secrets/"), "/")
	if path == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	action := ""
	switch {
	case strings.HasSuffix(path, "/rotate"):
		action = "rotate"
		path = strings.TrimSuffix(path, "/rotate")
	case strings.HasSuffix(path, "/disable"):
		action = "disable"
		path = strings.TrimSuffix(path, "/disable")
	}
	secretRef, err := urlPathUnescape(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch {
	case r.Method == http.MethodGet && action == "":
		tenantContext, reason := requireHostedCredentialRead(r)
		if reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		secret, err := manager.Get(r.Context(), tenantContext.TenantID, secretRef)
		if err != nil {
			writeTenantSecretError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"secret": secret})
	case r.Method == http.MethodPatch && action == "":
		tenantContext, reason := requireHostedCredentialPermission(r, credentialPermissionManageSecrets, "")
		if reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		var request updateTenantSecretRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		secret, err := manager.UpdateMetadata(r.Context(), secrets.UpdateMetadataInput{
			TenantID:    tenantContext.TenantID,
			SecretRef:   secretRef,
			DisplayName: request.DisplayName,
			Document:    request.Document,
		})
		if err != nil {
			writeTenantSecretError(w, err)
			return
		}
		if err := recordCredentialAudit(r.Context(), audit.CredentialAuditInput{
			TenantID:     tenantContext.TenantID,
			PrincipalID:  tenantContext.PrincipalID,
			ResourceKind: secrets.ResourceKindTenantSecret,
			ResourceID:   secret.SecretID,
			Action:       secrets.AuditActionSecretUpdate,
			SecretRef:    secret.SecretRef,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"secret": secret})
	case r.Method == http.MethodPost && action == "rotate":
		tenantContext, reason := requireHostedCredentialPermission(r, credentialPermissionManageSecrets, "")
		if reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		var request rotateTenantSecretRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		secret, err := manager.Rotate(r.Context(), secrets.RotateInput{TenantID: tenantContext.TenantID, SecretRef: secretRef, Value: request.Value})
		if err != nil {
			writeTenantSecretError(w, err)
			return
		}
		if err := recordCredentialAudit(r.Context(), audit.CredentialAuditInput{
			TenantID:        tenantContext.TenantID,
			PrincipalID:     tenantContext.PrincipalID,
			ResourceKind:    secrets.ResourceKindSecretVersion,
			ResourceID:      secret.SecretID,
			Action:          secrets.AuditActionSecretRotate,
			SecretRef:       secret.SecretRef,
			SecretVersionID: secret.ActiveVersionID,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"secret": secret})
	case r.Method == http.MethodPost && action == "disable":
		tenantContext, reason := requireHostedCredentialPermission(r, credentialPermissionManageSecrets, "")
		if reason != "" {
			writeCredentialDenial(w, http.StatusForbidden, reason)
			return
		}
		var request disableTenantSecretRequest
		if err := decodeJSONBody(r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		secret, err := manager.Disable(r.Context(), secrets.DisableInput{TenantID: tenantContext.TenantID, SecretRef: secretRef, DisabledReason: request.DisabledReason})
		if err != nil {
			writeTenantSecretError(w, err)
			return
		}
		if err := recordCredentialAudit(r.Context(), audit.CredentialAuditInput{
			TenantID:     tenantContext.TenantID,
			PrincipalID:  tenantContext.PrincipalID,
			ResourceKind: secrets.ResourceKindTenantSecret,
			ResourceID:   secret.SecretID,
			Action:       secrets.AuditActionSecretDisable,
			SecretRef:    secret.SecretRef,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"secret": secret})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func recordCredentialAudit(ctx context.Context, input audit.CredentialAuditInput) error {
	sqliteStore, ok := tenantAuditStoreFromContext(ctx)
	if !ok || sqliteStore == nil {
		return nil
	}
	if input.TenantID == "" {
		if tenantContext, ok := tenantContextFromContext(ctx); ok {
			input.TenantID = tenantContext.TenantID
			input.PrincipalID = firstNonEmpty(input.PrincipalID, tenantContext.PrincipalID)
		}
	}
	_, err := sqliteStore.AppendTenantAuditEvent(ctx, audit.BuildCredentialAuditEvent(input))
	return err
}

func requireHostedCredentialRead(r *http.Request) (identity.TenantContext, string) {
	return requireHostedCredentialReadAny(r, credentialPermissionManageSecrets)
}

func requireHostedCredentialReadAny(r *http.Request, managePermissions ...identity.Permission) (identity.TenantContext, string) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		return identity.TenantContext{}, credentialDenialMissingTenant
	}
	if !identity.CanInspectCredentials(tenantContext, managePermissions...) {
		return identity.TenantContext{}, credentialDenialMissingPermission
	}
	return tenantContext, ""
}

func writeTenantSecretError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, secrets.ErrTenantRequired), errors.Is(err, secrets.ErrSecretRefRequired), errors.Is(err, secrets.ErrSecretValueRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, secrets.ErrSecretNotFound), errors.Is(err, secrets.ErrSecretVersionNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, secrets.ErrSecretDisabled), errors.Is(err, secrets.ErrCrossTenantSecret):
		writeCredentialDenial(w, http.StatusForbidden, credentialDenialCrossTenant)
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func urlPathUnescape(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", secrets.ErrSecretRefRequired
	}
	// net/http has already decoded most path escapes before dispatch, but
	// path-style refs can still arrive from direct handler tests.
	return strings.ReplaceAll(value, "%2F", "/"), nil
}
