package setupwizard

import "github.com/dopejs/dope-agent/daemon/internal/identity"

const (
	PermissionSecretsManage      = string(identity.PermissionSecretsManage)
	PermissionIntegrationsManage = string(identity.PermissionIntegrationsManage)
	PermissionCredentialsInspect = string(identity.PermissionCredentialsInspect)
)

func CanMutateSetup(tc identity.TenantContext) bool {
	return tc.TenantID != "" &&
		tc.PrincipalID != "" &&
		identity.HasPermission(tc.Permissions, identity.PermissionSecretsManage) &&
		identity.HasPermission(tc.Permissions, identity.PermissionIntegrationsManage)
}

func CanInspectSetup(tc identity.TenantContext) bool {
	return tc.TenantID != "" &&
		tc.PrincipalID != "" &&
		identity.HasPermission(tc.Permissions, identity.PermissionCredentialsInspect)
}

func RequireMutation(tc identity.TenantContext) error {
	if !CanMutateSetup(tc) {
		return ErrPermissionDenied
	}
	return nil
}

func RequireInspection(tc identity.TenantContext) error {
	if !CanInspectSetup(tc) {
		return ErrPermissionDenied
	}
	return nil
}
