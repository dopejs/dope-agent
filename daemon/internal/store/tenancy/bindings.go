package tenancy

import (
	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

// BindingAccessScope guards tenant-scoped binding and workspace access by permission.
type BindingAccessScope struct {
	TenantID    string
	Permissions []identity.Permission
}

func (scope BindingAccessScope) allows(tenantID string, permission identity.Permission) bool {
	return scope.TenantID != "" && scope.TenantID == tenantID && identity.HasPermission(scope.Permissions, permission)
}

// CanInspectWorkspace reports whether the scope may read the workspace.
func (scope BindingAccessScope) CanInspectWorkspace(ws bindings.Workspace) bool {
	return scope.allows(ws.TenantID, identity.PermissionBindingsInspect)
}

// CanManageWorkspace reports whether the scope may mutate the workspace.
func (scope BindingAccessScope) CanManageWorkspace(ws bindings.Workspace) bool {
	return scope.allows(ws.TenantID, identity.PermissionBindingsManage)
}

// CanInspectBinding reports whether the scope may read the binding rule.
func (scope BindingAccessScope) CanInspectBinding(rule bindings.BindingRule) bool {
	return scope.allows(rule.TenantID, identity.PermissionBindingsInspect)
}

// CanManageBinding reports whether the scope may mutate the binding rule.
func (scope BindingAccessScope) CanManageBinding(rule bindings.BindingRule) bool {
	return scope.allows(rule.TenantID, identity.PermissionBindingsManage)
}

// CanInspectTenant reports whether the scope may inspect binding state for a tenant id
// (used before any resource is loaded, so denial never reveals existence — FR-007).
func (scope BindingAccessScope) CanInspectTenant(tenantID string) bool {
	return scope.allows(tenantID, identity.PermissionBindingsInspect)
}

// CanManageTenant reports whether the scope may manage binding state for a tenant id.
func (scope BindingAccessScope) CanManageTenant(tenantID string) bool {
	return scope.allows(tenantID, identity.PermissionBindingsManage)
}
