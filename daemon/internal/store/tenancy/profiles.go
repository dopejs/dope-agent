package tenancy

import (
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

type ProfileAccessScope struct {
	TenantID    string
	Permissions []identity.Permission
}

func (scope ProfileAccessScope) Allows(profile profiles.AgentProfile, permission identity.Permission) bool {
	return scope.TenantID != "" && scope.TenantID == profile.TenantID && identity.HasPermission(scope.Permissions, permission)
}

func (scope ProfileAccessScope) CanInspect(profile profiles.AgentProfile) bool {
	return scope.Allows(profile, identity.PermissionProfilesInspect)
}

func (scope ProfileAccessScope) CanManage(profile profiles.AgentProfile) bool {
	return scope.Allows(profile, identity.PermissionProfilesManage)
}
