//! Setup permission checks (port of `permissions.go`).

use dope_identity::{Permission, TenantContext, has_permission};

use crate::types::SetupError;

pub const PERMISSION_SECRETS_MANAGE: &str = "secrets.manage";
pub const PERMISSION_INTEGRATIONS_MANAGE: &str = "integrations.manage";

#[must_use]
pub fn can_mutate_setup(tc: &TenantContext) -> bool {
    !tc.tenant_id.is_empty()
        && !tc.principal_id.is_empty()
        && has_permission(&tc.permissions, Permission::SecretsManage)
        && has_permission(&tc.permissions, Permission::IntegrationsManage)
}

#[must_use]
pub fn can_inspect_setup(tc: &TenantContext) -> bool {
    !tc.tenant_id.is_empty()
        && !tc.principal_id.is_empty()
        && has_permission(&tc.permissions, Permission::CredentialsInspect)
}

pub fn require_mutation(tc: &TenantContext) -> Result<(), SetupError> {
    if !can_mutate_setup(tc) {
        return Err(SetupError::PermissionDenied);
    }
    Ok(())
}

pub fn require_inspection(tc: &TenantContext) -> Result<(), SetupError> {
    if !can_inspect_setup(tc) {
        return Err(SetupError::PermissionDenied);
    }
    Ok(())
}
