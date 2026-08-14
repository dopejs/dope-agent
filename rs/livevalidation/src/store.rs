//! Persistence abstraction (port of `store.go`). The Go store interface is
//! implemented by the daemon's store layer; in Rust `dope-store` (wave 5)
//! will implement this trait. The manager is persistence-optional: a missing
//! store degrades each method to the Go `m.store == nil` behavior.

use std::future::Future;
use std::pin::Pin;

use crate::error::LiveValidationError;
use crate::ledger::LedgerOutcome;
use crate::matrix::MatrixRow;
use crate::matrix::ToolClass;
use crate::types::AmbiguousCommit;
use crate::types::Attempt;
use crate::types::AttemptStatus;
use crate::types::Comparison;
use crate::types::ComparisonStatus;
use crate::types::FreshApproval;
use crate::types::KillSwitch;
use crate::types::KillSwitchScope;
use crate::types::ReconciliationResolution;
use crate::types::RetentionPolicy;
use crate::types::SideEffectLedgerEntry;
use crate::types::SideEffectScope;

/// Object-safe boxed future used by the store trait.
pub type BoxFuture<'a, T> = Pin<Box<dyn Future<Output = T> + Send + 'a>>;

/// Port of `AttemptFilter`. Empty fields match everything.
#[derive(Debug, Clone, Default)]
pub struct AttemptFilter {
    pub tenant_id: String,
    pub environment_scope: String,
    pub candidate_id: String,
    pub status: AttemptStatus,
    pub limit: i64,
}

/// Port of `LedgerFilter`.
#[derive(Debug, Clone, Default)]
pub struct LedgerFilter {
    pub tenant_id: String,
    pub validation_id: String,
    pub candidate_id: String,
    pub tool_class: ToolClass,
    pub outcome: LedgerOutcome,
    pub limit: i64,
}

/// Port of `KillSwitchFilter`.
#[derive(Debug, Clone, Default)]
pub struct KillSwitchFilter {
    pub tenant_id: String,
    pub scope: KillSwitchScope,
    pub enabled: Option<bool>,
    pub limit: i64,
}

/// Port of `ComparisonFilter`.
#[derive(Debug, Clone, Default)]
pub struct ComparisonFilter {
    pub tenant_id: String,
    pub validation_id: String,
    pub candidate_id: String,
    pub terminal_status: ComparisonStatus,
    pub limit: i64,
}

/// Port of the Go `Store` interface.
pub trait Store: Send + Sync {
    fn upsert_attempt(&self, item: Attempt) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn get_attempt(
        &self,
        tenant_id: &str,
        validation_id: &str,
    ) -> BoxFuture<'_, Result<Option<Attempt>, LiveValidationError>>;
    fn list_attempts(
        &self,
        filter: AttemptFilter,
    ) -> BoxFuture<'_, Result<Vec<Attempt>, LiveValidationError>>;
    fn upsert_scope(
        &self,
        item: SideEffectScope,
        tenant_id: &str,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn upsert_approval(
        &self,
        item: FreshApproval,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn append_ledger_entry(
        &self,
        item: SideEffectLedgerEntry,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn update_ledger_entry_outcome(
        &self,
        ledger_entry_id: &str,
        outcome: &LedgerOutcome,
        reason_code: &str,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn list_ledger_entries(
        &self,
        filter: LedgerFilter,
    ) -> BoxFuture<'_, Result<Vec<SideEffectLedgerEntry>, LiveValidationError>>;
    fn upsert_kill_switch(
        &self,
        item: KillSwitch,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn list_kill_switches(
        &self,
        filter: KillSwitchFilter,
    ) -> BoxFuture<'_, Result<Vec<KillSwitch>, LiveValidationError>>;
    fn upsert_support_matrix_snapshot(
        &self,
        tenant_id: &str,
        snapshot_id: &str,
        rows: Vec<MatrixRow>,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn save_ambiguous_commit(
        &self,
        item: AmbiguousCommit,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn save_reconciliation_resolution(
        &self,
        item: ReconciliationResolution,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn save_comparison(&self, item: Comparison) -> BoxFuture<'_, Result<(), LiveValidationError>>;
    fn list_comparisons(
        &self,
        filter: ComparisonFilter,
    ) -> BoxFuture<'_, Result<Vec<Comparison>, LiveValidationError>>;
    fn save_retention_policy(
        &self,
        item: RetentionPolicy,
    ) -> BoxFuture<'_, Result<(), LiveValidationError>>;
}
