//! ProfileAccessScope guards tenant-scoped agent-profile access by permission.
//! Port of daemon/internal/store/tenancy/profiles.go.

use kura_identity::{Permission, has_permission};

/// Tenant-scoped permission scope for agent profiles.
#[derive(Debug, Clone, Default)]
pub struct ProfileAccessScope {
    pub tenant_id: String,
    pub permissions: Vec<Permission>,
}

impl ProfileAccessScope {
    fn allows(&self, profile: &kura_profiles::AgentProfile, permission: Permission) -> bool {
        !self.tenant_id.is_empty()
            && self.tenant_id == profile.tenant_id
            && has_permission(&self.permissions, permission)
    }

    /// Whether the scope may read the profile.
    #[must_use]
    pub fn can_inspect(&self, profile: &kura_profiles::AgentProfile) -> bool {
        self.allows(profile, Permission::ProfilesInspect)
    }

    /// Whether the scope may mutate the profile.
    #[must_use]
    pub fn can_manage(&self, profile: &kura_profiles::AgentProfile) -> bool {
        self.allows(profile, Permission::ProfilesManage)
    }
}
