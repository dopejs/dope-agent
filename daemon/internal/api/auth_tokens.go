package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/auth"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func handleAuthTokens(authManager *auth.Manager, identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleAuthTokenList(sqliteStore, w, r)
	case http.MethodPost:
		handleAuthTokenCreate(authManager, identityManager, sqliteStore, w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleAuthTokenRoutes(authManager *auth.Manager, identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/auth/tokens/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || r.Method != http.MethodPost && r.Method != http.MethodPatch {
		http.NotFound(w, r)
		return
	}
	tokenID := parts[0]
	switch parts[1] {
	case "rotate":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleAuthTokenRotate(authManager, identityManager, sqliteStore, w, r, tokenID)
	case "revoke":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleAuthTokenRevoke(authManager, sqliteStore, w, r, tokenID)
	case "tenant-grants":
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleAuthTokenGrantUpdate(authManager, identityManager, sqliteStore, w, r, tokenID)
	default:
		http.NotFound(w, r)
	}
}

func handleAuthTokenList(sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	tokens, err := sqliteStore.ListAccessTokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	principalID := strings.TrimSpace(r.URL.Query().Get("principalId"))
	if principalID == "" || !identity.HasPermission(tenantContext.Permissions, identity.PermissionTenantManage) {
		principalID = tenantContext.PrincipalID
	}
	items := make([]auth.AccessToken, 0, len(tokens))
	for _, token := range tokens {
		if principalID != "" && token.PrincipalID != principalID {
			continue
		}
		if status != "" && token.Status != status {
			continue
		}
		items = append(items, token)
	}
	writeJSON(w, http.StatusOK, ListResponse[auth.AccessToken]{Items: items})
}

func handleAuthTokenCreate(authManager *auth.Manager, identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request) {
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionTenantManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	var request struct {
		Label            string   `json:"label"`
		ExpiresAt        string   `json:"expiresAt"`
		DefaultTenantID  string   `json:"defaultTenantId"`
		AllowedTenantIDs []string `json:"allowedTenantIds"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	expiresAt, err := parseOptionalTime(request.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := identityManager.ValidateTokenTenantGrants(r.Context(), tenantContext, tenantContext.TokenID, request.AllowedTenantIDs, request.DefaultTenantID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := appendTokenAudit(r, sqliteStore, "tenant.token_issued", request.DefaultTenantID, tenantContext.PrincipalID, "", "token_issued"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, secret, err := authManager.IssueToken(auth.IssueTokenInput{PrincipalID: tenantContext.PrincipalID, Label: request.Label, DefaultTenantID: request.DefaultTenantID, ExpiresAt: expiresAt})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.UpsertAccessToken(r.Context(), token); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	grants, err := identityManager.ReplaceTokenTenantGrants(r.Context(), tenantContext, token.TokenID, request.AllowedTenantIDs, request.DefaultTenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "accessToken": secret, "grants": grants})
}

func handleAuthTokenRotate(authManager *auth.Manager, identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, tokenID string) {
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionTenantManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	var request struct {
		ExpiresAt string `json:"expiresAt"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	expiresAt, err := parseOptionalTime(request.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	oldGrants, err := sqliteStore.ListTokenTenantGrants(r.Context(), tokenID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	allowedTenantIDs, defaultTenantID := activeGrantSet(oldGrants)
	if len(allowedTenantIDs) == 0 {
		writeError(w, http.StatusBadRequest, identity.ErrTokenGrantInvalid.Error())
		return
	}
	if err := appendTokenAudit(r, sqliteStore, "tenant.token_rotated", defaultTenantID, tenantContext.PrincipalID, tokenID, firstNonEmpty(request.Reason, "token_rotated")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	oldToken, newToken, secret, err := authManager.RotateToken(tokenID, auth.RotateTokenInput{ExpiresAt: expiresAt, Reason: request.Reason})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.UpsertAccessToken(r.Context(), oldToken); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := sqliteStore.UpsertAccessToken(r.Context(), newToken); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	grants, err := identityManager.ReplaceTokenTenantGrants(r.Context(), tenantContext, newToken.TokenID, allowedTenantIDs, defaultTenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"oldToken": oldToken, "newToken": newToken, "accessToken": secret, "grants": grants})
}

func handleAuthTokenRevoke(authManager *auth.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, tokenID string) {
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionTenantManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := appendTokenAudit(r, sqliteStore, "tenant.token_revoked", tenantContext.TenantID, tenantContext.PrincipalID, tokenID, firstNonEmpty(request.Reason, "token_revoked")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, err := authManager.RevokeToken(tokenID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sqliteStore.UpsertAccessToken(r.Context(), token); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

func handleAuthTokenGrantUpdate(authManager *auth.Manager, identityManager *identity.Manager, sqliteStore *store.SQLiteStore, w http.ResponseWriter, r *http.Request, tokenID string) {
	tenantContext, err := RequirePermission(r.Context(), identity.PermissionTenantManage)
	if err != nil {
		writeTenantDenial(w, http.StatusForbidden)
		return
	}
	var request struct {
		DefaultTenantID  string   `json:"defaultTenantId"`
		AllowedTenantIDs []string `json:"allowedTenantIds"`
		Reason           string   `json:"reason"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	grants, err := identityManager.ReplaceTokenTenantGrants(r.Context(), tenantContext, tokenID, request.AllowedTenantIDs, request.DefaultTenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, ok := authManager.GetToken(tokenID)
	if ok {
		token.DefaultTenantID = request.DefaultTenantID
		token.UpdatedAt = time.Now().UTC()
		authManager.UpdateToken(token)
		if err := sqliteStore.UpsertAccessToken(r.Context(), token); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokenId": tokenID, "grants": grants})
}

func appendTokenAudit(r *http.Request, sqliteStore *store.SQLiteStore, eventKind, tenantID, principalID, tokenID, reasonCode string) error {
	if sqliteStore == nil {
		return nil
	}
	_, err := sqliteStore.AppendTenantAuditEvent(r.Context(), identity.TenantAuditEvent{
		EventKind:   eventKind,
		TenantID:    tenantID,
		PrincipalID: principalID,
		TokenID:     tokenID,
		Outcome:     identity.AuditOutcomeSucceeded,
		ReasonCode:  reasonCode,
		CreatedAt:   time.Now().UTC(),
	})
	return err
}

func parseOptionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func activeGrantSet(grants []identity.TokenTenantGrant) ([]string, string) {
	tenantIDs := make([]string, 0, len(grants))
	defaultTenantID := ""
	for _, grant := range grants {
		if grant.Status != identity.StatusActive {
			continue
		}
		tenantIDs = append(tenantIDs, grant.TenantID)
		if grant.IsDefault {
			defaultTenantID = grant.TenantID
		}
	}
	if defaultTenantID == "" && len(tenantIDs) > 0 {
		defaultTenantID = tenantIDs[0]
	}
	return tenantIDs, defaultTenantID
}
