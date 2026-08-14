//! Ambiguous-commit reconciliation (port of `reconciliation.go`).

use chrono::DateTime;
use chrono::Utc;

use dope_identity::can_resolve_live_validation_reconciliation;
use dope_identity::tenantctx;

use crate::error::LiveValidationError;
use crate::manager::Manager;
use crate::types::ReconciliationResolution;

impl Manager {
    /// Port of `Manager.ResolveReconciliation`.
    pub async fn resolve_reconciliation(
        &self,
        mut item: ReconciliationResolution,
    ) -> Result<ReconciliationResolution, LiveValidationError> {
        let Some(tenant_context) = tenantctx::from_context() else {
            return Err(LiveValidationError::ReconciliationPermissionDenied);
        };
        if !can_resolve_live_validation_reconciliation(&tenant_context) {
            return Err(LiveValidationError::ReconciliationPermissionDenied);
        }
        let now = self.now();
        if item.reconciliation_id.is_empty() {
            item.reconciliation_id = crate::manager::new_id("lv_reconcile");
        }
        if item.tenant_id.is_empty() {
            item.tenant_id = tenant_context.tenant_id.clone();
        }
        if item.resolved_by.is_empty() {
            item.resolved_by = tenant_context.principal_id.clone();
        }
        if item.resolved_at == DateTime::<Utc>::default() {
            item.resolved_at = now;
        }
        if let Some(store) = self.store() {
            store.save_reconciliation_resolution(item.clone()).await?;
        }
        Ok(item)
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Arc;

    use dope_identity::tenantctx;

    use crate::error::LiveValidationError;
    use crate::testutil::MemStore;
    use crate::testutil::admin_context;
    use crate::testutil::manager_with_store;
    use crate::testutil::viewer_context;
    use crate::types::ReconciliationResolution;
    use crate::types::ReconciliationResolutionValue;

    #[tokio::test]
    async fn resolve_reconciliation_requires_authority() {
        let manager = manager_with_store(Arc::new(MemStore::default()));

        let viewer_result = tenantctx::scope(viewer_context(), async {
            manager
                .resolve_reconciliation(ReconciliationResolution {
                    ambiguous_commit_id: "amb_1".to_string(),
                    resolution: ReconciliationResolutionValue::from(
                        ReconciliationResolutionValue::CONFIRMED_COMMITTED,
                    ),
                    reason: "checked".to_string(),
                    ..ReconciliationResolution::default()
                })
                .await
        })
        .await;
        assert!(matches!(
            viewer_result,
            Err(LiveValidationError::ReconciliationPermissionDenied)
        ));

        let resolution = tenantctx::scope(admin_context(), async {
            manager
                .resolve_reconciliation(ReconciliationResolution {
                    ambiguous_commit_id: "amb_1".to_string(),
                    resolution: ReconciliationResolutionValue::from(
                        ReconciliationResolutionValue::CONFIRMED_COMMITTED,
                    ),
                    reason: "checked".to_string(),
                    ..ReconciliationResolution::default()
                })
                .await
        })
        .await
        .expect("resolve reconciliation");
        assert_eq!(resolution.resolved_by, "prn_admin");
        assert_eq!(resolution.tenant_id, "ten_1");
    }
}
