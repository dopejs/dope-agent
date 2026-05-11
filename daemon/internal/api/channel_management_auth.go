package api

import (
	"net/http"

	"github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func requireChannelManagementPermission(r *http.Request, sqliteStore *store.SQLiteStore, connectorID string, permission identity.Permission, action string) (identity.TenantContext, bool) {
	tenantContext, ok := tenantContextFromContext(r.Context())
	if !ok || tenantContext.TenantID == "" {
		return identity.TenantContext{}, false
	}
	if identity.HasPermission(tenantContext.Permissions, permission) {
		return tenantContext, true
	}
	_, _ = recordChannelManagementAudit(r, sqliteStore, connectors.ConnectorAuditRecord{
		TenantID:        tenantContext.TenantID,
		ConnectorID:     connectorID,
		PrincipalID:     tenantContext.PrincipalID,
		Action:          action,
		PermissionGate:  string(permission),
		Outcome:         "denied",
		ReasonCode:      "permission_missing",
		RedactionStatus: connectors.RedactionStatusRedacted,
	})
	return identity.TenantContext{}, false
}

func requireChannelManagementPermissions(r *http.Request, sqliteStore *store.SQLiteStore, connectorID string, action string, permissions ...identity.Permission) (identity.TenantContext, bool) {
	var tenantContext identity.TenantContext
	for _, permission := range permissions {
		var ok bool
		tenantContext, ok = requireChannelManagementPermission(r, sqliteStore, connectorID, permission, action)
		if !ok {
			return identity.TenantContext{}, false
		}
	}
	return tenantContext, true
}

func writeChannelManagementDenial(w http.ResponseWriter) {
	writeCredentialDenial(w, http.StatusForbidden, "permission_missing")
}
