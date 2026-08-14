//! Tenant-aware accessor for the approvals + decisions pair.
//! Port of daemon/internal/store/tenancy/approvals.go.

use crate::{emit_denial, require, TenancyError};

/// Tenant-aware accessor for approvals and decisions.
pub struct Approvals {
    store: crate::SQLiteStore,
    emitter: Option<dope_audit::Emitter>,
}

impl Approvals {
    #[must_use]
    pub fn new(store: crate::SQLiteStore, emitter: Option<dope_audit::Emitter>) -> Self {
        Approvals { store, emitter }
    }

    fn emit(&self, surface: &str, resource_kind: &str) {
        emit_denial(&self.emitter, surface, resource_kind);
    }

    pub fn list_approvals_for_tenant(&self) -> Result<Vec<dope_policy::Approval>, TenancyError> {
        let tenant_id = require()?;
        self.store.list_approvals_for_tenant_raw(&tenant_id).map_err(TenancyError::from)
    }

    pub fn upsert_approval_for_tenant(&self, approval: &dope_policy::Approval) -> Result<(), TenancyError> {
        let tenant_id = require()?;
        self.store.upsert_approval(approval).map_err(TenancyError::from)?;
        match self.store.bind_row_tenant("approvals", "approval_id", &approval.approval_id, &tenant_id) {
            Err(e) if crate::SQLiteStore::is_cross_tenant_row(&e) => {
                self.emit("store:UpsertApprovalForTenant", "approval");
                Err(TenancyError::CrossTenantWrite)
            }
            other => other.map_err(TenancyError::from),
        }
    }

    pub fn list_decisions_for_tenant(&self) -> Result<Vec<dope_policy::Decision>, TenancyError> {
        let tenant_id = require()?;
        self.store.list_decisions_for_tenant_raw(&tenant_id).map_err(TenancyError::from)
    }

    pub fn upsert_decision_for_tenant(&self, decision: &dope_policy::Decision) -> Result<(), TenancyError> {
        let tenant_id = require()?;
        self.store.upsert_decision(decision).map_err(TenancyError::from)?;
        match self.store.bind_row_tenant("decisions", "decision_id", &decision.decision_id, &tenant_id) {
            Err(e) if crate::SQLiteStore::is_cross_tenant_row(&e) => {
                self.emit("store:UpsertDecisionForTenant", "decision");
                Err(TenancyError::CrossTenantWrite)
            }
            other => other.map_err(TenancyError::from),
        }
    }
}
