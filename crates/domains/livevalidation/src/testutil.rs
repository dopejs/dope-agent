//! Test helpers shared across the crate's tests.

use std::sync::Arc;

use chrono::DateTime;
use chrono::Utc;
use kura_identity::LifecycleStatus;
use kura_identity::Role;
use kura_identity::TenantContext;
use kura_identity::permissions_for_role;
use parking_lot::Mutex;

use crate::error::LiveValidationError;
use crate::ledger::LedgerOutcome;
use crate::ledger::validate_ledger_transition;
use crate::manager::Clock;
use crate::manager::Dependencies;
use crate::manager::Manager;
use crate::matrix::MatrixRow;
use crate::store::AttemptFilter;
use crate::store::BoxFuture;
use crate::store::ComparisonFilter;
use crate::store::KillSwitchFilter;
use crate::store::LedgerFilter;
use crate::store::Store;
use crate::types::Attempt;
use crate::types::Comparison;
use crate::types::FreshApproval;
use crate::types::KillSwitch;
use crate::types::ReconciliationResolution;
use crate::types::RetentionPolicy;
use crate::types::SideEffectLedgerEntry;
use crate::types::SideEffectScope;

/// Fixed timestamp used by tests: 2026-04-29T10:00:00Z.
pub(crate) fn fixed_clock() -> DateTime<Utc> {
    DateTime::<Utc>::from_timestamp_secs(1_777_456_800).expect("fixed clock")
}

pub(crate) fn operator_context() -> TenantContext {
    TenantContext {
        tenant_id: "ten_1".to_string(),
        principal_id: "prn_operator".to_string(),
        role: Some(Role::Operator),
        permissions: permissions_for_role(Role::Operator, LifecycleStatus::Active),
        ..TenantContext::default()
    }
}

pub(crate) fn admin_context() -> TenantContext {
    TenantContext {
        tenant_id: "ten_1".to_string(),
        principal_id: "prn_admin".to_string(),
        role: Some(Role::Admin),
        permissions: permissions_for_role(Role::Admin, LifecycleStatus::Active),
        ..TenantContext::default()
    }
}

pub(crate) fn viewer_context() -> TenantContext {
    TenantContext {
        tenant_id: "ten_1".to_string(),
        principal_id: "prn_viewer".to_string(),
        role: Some(Role::Viewer),
        permissions: permissions_for_role(Role::Viewer, LifecycleStatus::Active),
        ..TenantContext::default()
    }
}

/// In-memory state backing [`MemStore`].
#[derive(Default)]
pub(crate) struct MemState {
    pub attempts: Vec<Attempt>,
    pub ledger: Vec<SideEffectLedgerEntry>,
    pub kill_switches: Vec<KillSwitch>,
    pub comparisons: Vec<Comparison>,
}

/// A stateful, in-memory [`Store`] for tests (port of the Go `memoryStore`).
#[derive(Default)]
pub(crate) struct MemStore {
    pub state: Mutex<MemState>,
}

impl Store for MemStore {
    fn upsert_attempt(&self, item: Attempt) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        let mut state = self.state.lock();
        if let Some(existing) = state
            .attempts
            .iter_mut()
            .find(|a| a.validation_id == item.validation_id)
        {
            *existing = item;
        } else {
            state.attempts.push(item);
        }
        Box::pin(async { Ok(()) })
    }

    fn get_attempt(
        &self,
        tenant_id: &str,
        validation_id: &str,
    ) -> BoxFuture<'_, Result<Option<Attempt>, LiveValidationError>> {
        let result = {
            let state = self.state.lock();
            state
                .attempts
                .iter()
                .find(|a| {
                    a.validation_id == validation_id
                        && (tenant_id.is_empty() || a.tenant_id == tenant_id)
                })
                .cloned()
        };
        Box::pin(async move { Ok(result) })
    }

    fn list_attempts(
        &self,
        _filter: AttemptFilter,
    ) -> BoxFuture<'_, Result<Vec<Attempt>, LiveValidationError>> {
        let result = self.state.lock().attempts.clone();
        Box::pin(async move { Ok(result) })
    }

    fn upsert_scope(
        &self,
        _item: SideEffectScope,
        _tenant_id: &str,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        Box::pin(async { Ok(()) })
    }

    fn upsert_approval(
        &self,
        _item: FreshApproval,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        Box::pin(async { Ok(()) })
    }

    fn append_ledger_entry(
        &self,
        item: SideEffectLedgerEntry,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        self.state.lock().ledger.push(item);
        Box::pin(async { Ok(()) })
    }

    fn update_ledger_entry_outcome(
        &self,
        ledger_entry_id: &str,
        outcome: &LedgerOutcome,
        reason_code: &str,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        let result = {
            let mut state = self.state.lock();
            if let Some(entry) = state
                .ledger
                .iter_mut()
                .find(|e| e.ledger_entry_id == ledger_entry_id)
            {
                match validate_ledger_transition(&entry.outcome, outcome) {
                    Ok(()) => {
                        entry.outcome = outcome.clone();
                        entry.reason_code = reason_code.to_string();
                        Ok(())
                    }
                    Err(err) => Err(err),
                }
            } else {
                Ok(())
            }
        };
        Box::pin(async move { result })
    }

    fn list_ledger_entries(
        &self,
        _filter: LedgerFilter,
    ) -> BoxFuture<'_, Result<Vec<SideEffectLedgerEntry>, LiveValidationError>> {
        let result = self.state.lock().ledger.clone();
        Box::pin(async move { Ok(result) })
    }

    fn upsert_kill_switch(
        &self,
        item: KillSwitch,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        self.state.lock().kill_switches.push(item);
        Box::pin(async { Ok(()) })
    }

    fn list_kill_switches(
        &self,
        filter: KillSwitchFilter,
    ) -> BoxFuture<'_, Result<Vec<KillSwitch>, LiveValidationError>> {
        let result = {
            let state = self.state.lock();
            state
                .kill_switches
                .iter()
                .filter(|item| filter.enabled.is_none_or(|enabled| item.enabled == enabled))
                .cloned()
                .collect()
        };
        Box::pin(async move { Ok(result) })
    }

    fn upsert_support_matrix_snapshot(
        &self,
        _tenant_id: &str,
        _snapshot_id: &str,
        _rows: Vec<MatrixRow>,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        Box::pin(async { Ok(()) })
    }

    fn save_ambiguous_commit(
        &self,
        _item: crate::types::AmbiguousCommit,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        Box::pin(async { Ok(()) })
    }

    fn save_reconciliation_resolution(
        &self,
        _item: ReconciliationResolution,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        Box::pin(async { Ok(()) })
    }

    fn save_comparison(&self, item: Comparison) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        self.state.lock().comparisons.push(item);
        Box::pin(async { Ok(()) })
    }

    fn list_comparisons(
        &self,
        _filter: ComparisonFilter,
    ) -> BoxFuture<'_, Result<Vec<Comparison>, LiveValidationError>> {
        let result = self.state.lock().comparisons.clone();
        Box::pin(async move { Ok(result) })
    }

    fn save_retention_policy(
        &self,
        _item: RetentionPolicy,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>> {
        Box::pin(async { Ok(()) })
    }
}

/// Builds an enabled manager backed by `store` and the fixed clock.
pub(crate) fn manager_with_store(store: Arc<MemStore>) -> Manager {
    let clock: Clock = Arc::new(fixed_clock);
    Manager::new(Dependencies {
        environment_scope: "test".to_string(),
        store: Some(store),
        enabled: true,
        billing: None,
        hosted_billing: false,
        clock: Some(clock),
        ledger_event_sink: None,
        candidate_tool_class_resolver: None,
    })
}

/// Builds an enabled manager with the fixed clock and no store.
pub(crate) fn manager_without_store() -> Manager {
    let clock: Clock = Arc::new(fixed_clock);
    Manager::new(Dependencies {
        environment_scope: "test".to_string(),
        store: None,
        enabled: true,
        billing: None,
        hosted_billing: false,
        clock: Some(clock),
        ledger_event_sink: None,
        candidate_tool_class_resolver: None,
    })
}
