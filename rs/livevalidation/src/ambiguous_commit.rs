//! Ambiguous-commit recording (port of `ambiguous_commit.go`).

use chrono::DateTime;
use chrono::Utc;

use crate::error::LiveValidationError;
use crate::ledger::LedgerOutcome;
use crate::manager::Manager;
use crate::types::AmbiguousCommit;
use crate::types::AmbiguousCommitCause;

impl Manager {
    /// Port of `Manager.RecordAmbiguousCommit`: stamps defaults, persists the
    /// record, and flips the linked ledger entry to operator-action-needed.
    pub async fn record_ambiguous_commit(
        &self,
        mut item: AmbiguousCommit,
    ) -> Result<AmbiguousCommit, LiveValidationError> {
        let now = self.now();
        if item.ambiguous_commit_id.is_empty() {
            item.ambiguous_commit_id = crate::manager::new_id("lv_ambiguous");
        }
        if item.cause.is_empty() {
            item.cause = AmbiguousCommitCause::from(AmbiguousCommitCause::OTHER);
        }
        item.automatic_retry_stopped = true;
        if item.created_at == DateTime::<Utc>::default() {
            item.created_at = now;
        }
        item.updated_at = now;
        if let Some(store) = self.store() {
            store.save_ambiguous_commit(item.clone()).await?;
        }
        if !item.ledger_entry_id.is_empty() {
            self.update_ledger_outcome(
                &item.ledger_entry_id,
                &LedgerOutcome::from(LedgerOutcome::OPERATOR_ACTION_NEEDED),
                "live_validation.ambiguous_commit",
            )
            .await?;
        }
        Ok(item)
    }
}
