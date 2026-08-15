//! Evidence retention policy (port of `retention.go`).

use chrono::DateTime;
use chrono::Utc;

use dope_identity::tenantctx;

use crate::error::LiveValidationError;
use crate::manager::Manager;
use crate::types::RetentionAppliesTo;
use crate::types::RetentionMode;
use crate::types::RetentionPolicy;

impl Manager {
    /// Port of `Manager.DefaultRetentionPolicy`.
    #[must_use]
    pub fn default_retention_policy(&self) -> RetentionPolicy {
        let now = self.now();
        let (tenant_id, created_by) = match tenantctx::from_context() {
            Some(ctx) => (ctx.tenant_id, ctx.principal_id),
            None => (String::new(), String::new()),
        };
        RetentionPolicy {
            policy_id: "live_validation_retention_default".to_string(),
            tenant_id,
            applies_to: RetentionAppliesTo::from(RetentionAppliesTo::ALL),
            mode: RetentionMode::from(RetentionMode::INDEFINITE),
            created_by_principal_id: created_by,
            created_at: now,
            ..RetentionPolicy::default()
        }
    }

    /// Port of `Manager.SaveRetentionPolicy`.
    pub async fn save_retention_policy(
        &self,
        mut item: RetentionPolicy,
    ) -> Result<RetentionPolicy, LiveValidationError> {
        if item.policy_id.is_empty() {
            item.policy_id = crate::manager::new_id("lv_retention");
        }
        if item.mode.is_empty() {
            item.mode = RetentionMode::from(RetentionMode::INDEFINITE);
        }
        if item.applies_to.is_empty() {
            item.applies_to = RetentionAppliesTo::from(RetentionAppliesTo::ALL);
        }
        if item.created_at == DateTime::<Utc>::default() {
            item.created_at = self.now();
        }
        if let Some(store) = self.store() {
            store.save_retention_policy(item.clone()).await?;
        }
        Ok(item)
    }
}

#[cfg(test)]
mod tests {

    use dope_identity::TenantContext;
    use dope_identity::tenantctx;

    use crate::testutil::manager_without_store;
    use crate::types::RetentionAppliesTo;
    use crate::types::RetentionMode;

    #[test]
    fn default_retention_policy_is_indefinite() {
        let manager = manager_without_store();
        let policy = tenantctx::with_context(
            TenantContext {
                tenant_id: "ten_1".to_string(),
                principal_id: "prn_1".to_string(),
                ..TenantContext::default()
            },
            || manager.default_retention_policy(),
        );
        assert_eq!(policy.mode, RetentionMode::from(RetentionMode::INDEFINITE));
        assert_eq!(
            policy.applies_to,
            RetentionAppliesTo::from(RetentionAppliesTo::ALL)
        );
        assert_eq!(policy.tenant_id, "ten_1");
    }
}
